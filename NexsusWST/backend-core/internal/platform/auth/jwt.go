// Package auth verifica el token de acceso (JWT) emitido por Keycloak.
//
// DAS v1.1 §4.5 / DA-11 / DA-15: Keycloak firma con su clave PRIVADA
// (asimétrica, RS256 o EdDSA) y cada servicio verifica localmente con las
// claves PÚBLICAS que Keycloak publica (JWKS), guardadas en caché. Se
// comprueban algoritmo, identificador de clave, firma, emisor, destinatario y
// vigencia. Traefik no participa. Solo biblioteca estándar de Go.
//
// El token SOLO identifica a la persona: las empresas, permisos y si el
// perfil está activo se leen en el servidor (paquete tenant).
package auth

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalidToken = errors.New("token inválido")
	ErrExpiredToken = errors.New("token vencido")
)

// Claims relevantes del access token de Keycloak.
type Claims struct {
	Subject           string   `json:"sub"` // id del usuario en Keycloak (UUID)
	Email             string   `json:"email"`
	EmailVerified     bool     `json:"email_verified"`
	Name              string   `json:"name"`
	PreferredUsername string   `json:"preferred_username"`
	Issuer            string   `json:"iss"`
	Audience          Audience `json:"aud"`
	AuthorizedParty   string   `json:"azp"`
	IssuedAt          int64    `json:"iat"`
	NotBefore         int64    `json:"nbf"`
	ExpiresAt         int64    `json:"exp"`
	SessionID         string   `json:"sid"`
	RealmAccess       struct {
		Roles []string `json:"roles"`
	} `json:"realm_access"`
}

// Audience acepta "aud" como texto o como lista (Keycloak usa ambas formas).
type Audience []string

func (a *Audience) UnmarshalJSON(b []byte) error {
	var one string
	if json.Unmarshal(b, &one) == nil {
		*a = Audience{one}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		return err
	}
	*a = many
	return nil
}

func (a Audience) Contains(s string) bool {
	for _, x := range a {
		if x == s {
			return true
		}
	}
	return false
}

// KeySource entrega la clave pública para un "kid" (ver JWKS).
type KeySource interface {
	Key(ctx context.Context, kid string) (crypto.PublicKey, error)
}

type header struct {
	Alg string `json:"alg"`
	Typ string `json:"typ"`
	Kid string `json:"kid"`
}

var b64 = base64.RawURLEncoding

// Verifier comprueba firma, algoritmo, emisor, destinatario y vigencia.
type Verifier struct {
	Keys     KeySource
	Issuer   string // p. ej. http://localhost/auth/realms/nexuswst
	Audience string // "nexuswst" (lo agrega el mapper del cliente nexus-web)
	Leeway   time.Duration
	Now      func() time.Time
}

func (v Verifier) Verify(ctx context.Context, token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrInvalidToken
	}
	hb, err := b64.DecodeString(parts[0])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var h header
	if json.Unmarshal(hb, &h) != nil || h.Kid == "" {
		return Claims{}, ErrInvalidToken
	}
	// Lista cerrada de algoritmos: se rechazan "none", HS256 y cualquier otro.
	if h.Alg != "RS256" && h.Alg != "EdDSA" {
		return Claims{}, ErrInvalidToken
	}
	sig, err := b64.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	key, err := v.Keys.Key(ctx, h.Kid)
	if err != nil {
		return Claims{}, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	signed := []byte(parts[0] + "." + parts[1])
	switch k := key.(type) {
	case *rsa.PublicKey:
		if h.Alg != "RS256" {
			return Claims{}, ErrInvalidToken
		}
		sum := sha256.Sum256(signed)
		if rsa.VerifyPKCS1v15(k, crypto.SHA256, sum[:], sig) != nil {
			return Claims{}, ErrInvalidToken
		}
	case ed25519.PublicKey:
		if h.Alg != "EdDSA" || !ed25519.Verify(k, signed, sig) {
			return Claims{}, ErrInvalidToken
		}
	default:
		return Claims{}, ErrInvalidToken
	}

	pb, err := b64.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrInvalidToken
	}
	var c Claims
	if json.Unmarshal(pb, &c) != nil {
		return Claims{}, ErrInvalidToken
	}
	if c.Subject == "" || c.Issuer != v.Issuer || !c.Audience.Contains(v.Audience) {
		return Claims{}, ErrInvalidToken
	}
	now := time.Now
	if v.Now != nil {
		now = v.Now
	}
	t := now()
	if c.ExpiresAt == 0 || t.After(time.Unix(c.ExpiresAt, 0).Add(v.Leeway)) {
		return Claims{}, ErrExpiredToken
	}
	if c.NotBefore != 0 && t.Add(v.Leeway).Before(time.Unix(c.NotBefore, 0)) {
		return Claims{}, ErrInvalidToken
	}
	return c, nil
}

// BaseRole devuelve el primer rol base de NexusWST presente en el token
// (solo se usa para crear el perfil en el primer ingreso).
func (c Claims) BaseRole(known ...string) string {
	for _, r := range c.RealmAccess.Roles {
		for _, k := range known {
			if r == k {
				return r
			}
		}
	}
	return ""
}
