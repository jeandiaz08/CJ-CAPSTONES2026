package auth

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func signRS256(t *testing.T, key *rsa.PrivateKey, kid string, claims map[string]any) string {
	t.Helper()
	h, _ := json.Marshal(map[string]string{"alg": "RS256", "typ": "JWT", "kid": kid})
	p, _ := json.Marshal(claims)
	signing := b64.EncodeToString(h) + "." + b64.EncodeToString(p)
	sum := sha256.Sum256([]byte(signing))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatal(err)
	}
	return signing + "." + b64.EncodeToString(sig)
}

func baseClaims(now time.Time) map[string]any {
	return map[string]any{
		"sub": "1f0c-uuid", "iss": "http://kc/realms/nexuswst", "aud": []string{"nexuswst", "account"},
		"iat": now.Unix(), "exp": now.Add(10 * time.Minute).Unix(), "email": "a@b.cl",
		"realm_access": map[string]any{"roles": []string{"default-roles", "jefe_operaciones"}},
	}
}

func TestVerifyRS256(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	now := time.Now()
	v := Verifier{Keys: StaticKeys{"k1": &key.PublicKey}, Issuer: "http://kc/realms/nexuswst", Audience: "nexuswst"}
	ctx := context.Background()

	c, err := v.Verify(ctx, signRS256(t, key, "k1", baseClaims(now)))
	if err != nil {
		t.Fatalf("token válido rechazado: %v", err)
	}
	if c.Subject != "1f0c-uuid" || c.BaseRole("admin", "jefe_operaciones") != "jefe_operaciones" {
		t.Fatalf("claims mal leídos: %+v", c)
	}

	// aud como texto simple también se acepta
	cl := baseClaims(now)
	cl["aud"] = "nexuswst"
	if _, err := v.Verify(ctx, signRS256(t, key, "k1", cl)); err != nil {
		t.Fatalf("aud texto rechazado: %v", err)
	}

	cases := map[string]string{
		"firma de otra clave": signRS256(t, other, "k1", baseClaims(now)),
		"kid desconocido":     signRS256(t, key, "k2", baseClaims(now)),
	}
	cl = baseClaims(now)
	cl["iss"] = "http://otro/realms/x"
	cases["emisor incorrecto"] = signRS256(t, key, "k1", cl)
	cl = baseClaims(now)
	cl["aud"] = "account"
	cases["destinatario incorrecto"] = signRS256(t, key, "k1", cl)
	cl = baseClaims(now)
	cl["sub"] = ""
	cases["sin sujeto"] = signRS256(t, key, "k1", cl)
	for name, tok := range cases {
		if _, err := v.Verify(ctx, tok); err == nil {
			t.Errorf("%s: aceptado", name)
		}
	}

	// Manipulado: se cambia el payload sin volver a firmar
	tok := signRS256(t, key, "k1", baseClaims(now))
	cl = baseClaims(now)
	cl["sub"] = "otro"
	p, _ := json.Marshal(cl)
	parts := splitToken(tok)
	if _, err := v.Verify(ctx, parts[0]+"."+b64.EncodeToString(p)+"."+parts[2]); err == nil {
		t.Error("payload manipulado aceptado")
	}

	// "alg": "none" y HS256 se rechazan
	for _, alg := range []string{"none", "HS256"} {
		h, _ := json.Marshal(map[string]string{"alg": alg, "kid": "k1"})
		if _, err := v.Verify(ctx, b64.EncodeToString(h)+"."+parts[1]+"."); err == nil {
			t.Errorf("alg %s aceptado", alg)
		}
	}

	// Vencido (10 min de token)
	v.Now = func() time.Time { return now.Add(11 * time.Minute) }
	if _, err := v.Verify(ctx, tok); !errors.Is(err, ErrExpiredToken) {
		t.Fatalf("esperaba vencido, obtuve %v", err)
	}
}

func TestVerifyEdDSA(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	h, _ := json.Marshal(map[string]string{"alg": "EdDSA", "kid": "ed"})
	p, _ := json.Marshal(baseClaims(time.Now()))
	signing := b64.EncodeToString(h) + "." + b64.EncodeToString(p)
	tok := signing + "." + b64.EncodeToString(ed25519.Sign(priv, []byte(signing)))
	v := Verifier{Keys: StaticKeys{"ed": pub}, Issuer: "http://kc/realms/nexuswst", Audience: "nexuswst"}
	if _, err := v.Verify(context.Background(), tok); err != nil {
		t.Fatal(err)
	}
}

func TestParseJWKSIgnoresEncryptionKeys(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	n := b64.EncodeToString(key.N.Bytes())
	keys, err := ParseJWKS([]jwk{
		{Kid: "sig", Kty: "RSA", Use: "sig", N: n, E: "AQAB"},
		{Kid: "enc", Kty: "RSA", Use: "enc", N: n, E: "AQAB"},
	})
	if err != nil || len(keys) != 1 || keys["sig"] == nil {
		t.Fatalf("esperaba solo la clave de firma: %v %v", keys, err)
	}
}

func splitToken(t string) []string {
	out := make([]string, 0, 3)
	start := 0
	for i := 0; i < len(t); i++ {
		if t[i] == '.' {
			out = append(out, t[start:i])
			start = i + 1
		}
	}
	return append(out, t[start:])
}
