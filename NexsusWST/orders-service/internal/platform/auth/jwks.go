package auth

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rsa"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// JWKS descarga y guarda en caché las claves públicas de Keycloak.
//
//   - Se descargan al arrancar (sin bloquear si Keycloak aún no responde).
//   - Si llega un "kid" desconocido (Keycloak rotó la clave) se vuelve a
//     descargar, como máximo una vez cada MinRefresh, para no saturar a
//     Keycloak con tokens falsos.
//   - Se refrescan cada MaxAge aunque no haya rotación.
//   - Si Keycloak cae, se siguen usando las claves en caché (DAS §4.5:
//     las sesiones vigentes siguen funcionando hasta que venza su token).
type JWKS struct {
	URL        string
	Client     *http.Client
	MinRefresh time.Duration
	MaxAge     time.Duration
	Log        *slog.Logger

	mu        sync.RWMutex
	keys      map[string]crypto.PublicKey
	fetchedAt time.Time
	triedAt   time.Time
	fetchMu   sync.Mutex
}

func NewJWKS(url string, log *slog.Logger) *JWKS {
	return &JWKS{
		URL: url, Client: &http.Client{Timeout: 5 * time.Second},
		MinRefresh: 30 * time.Second, MaxAge: time.Hour, Log: log,
		keys: map[string]crypto.PublicKey{},
	}
}

var ErrUnknownKey = errors.New("clave de firma desconocida")

func (j *JWKS) Key(ctx context.Context, kid string) (crypto.PublicKey, error) {
	j.mu.RLock()
	k, ok := j.keys[kid]
	stale := time.Since(j.fetchedAt) > j.MaxAge
	j.mu.RUnlock()
	if ok && !stale {
		return k, nil
	}
	if err := j.refresh(ctx); err != nil && !ok {
		return nil, err
	}
	j.mu.RLock()
	defer j.mu.RUnlock()
	if k, ok := j.keys[kid]; ok {
		return k, nil
	}
	return nil, ErrUnknownKey
}

// Warmup intenta la primera descarga; si falla solo se registra.
func (j *JWKS) Warmup(ctx context.Context) {
	if err := j.refresh(ctx); err != nil && j.Log != nil {
		j.Log.Warn("no se pudieron descargar las claves de Keycloak (se reintenta al recibir un token)", "url", j.URL, "err", err)
	}
}

func (j *JWKS) refresh(ctx context.Context) error {
	j.fetchMu.Lock()
	defer j.fetchMu.Unlock()
	j.mu.RLock()
	recent := time.Since(j.triedAt) < j.MinRefresh
	j.mu.RUnlock()
	if recent { // ya se intentó hace poco (con o sin éxito): no insistir
		return ErrUnknownKey
	}
	j.mu.Lock()
	j.triedAt = time.Now()
	j.mu.Unlock()

	keys, err := j.fetch(ctx)
	if err != nil {
		return err
	}
	j.mu.Lock()
	j.keys, j.fetchedAt = keys, time.Now()
	j.mu.Unlock()
	return nil
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	Crv string `json:"crv"`
	N   string `json:"n"`
	E   string `json:"e"`
	X   string `json:"x"`
}

func (j *JWKS) fetch(ctx context.Context) (map[string]crypto.PublicKey, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.URL, nil)
	if err != nil {
		return nil, err
	}
	res, err := j.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("JWKS: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("JWKS: HTTP %d", res.StatusCode)
	}
	var doc struct {
		Keys []jwk `json:"keys"`
	}
	if err := json.NewDecoder(res.Body).Decode(&doc); err != nil {
		return nil, fmt.Errorf("JWKS: %w", err)
	}
	return ParseJWKS(doc.Keys)
}

// ParseJWKS convierte las claves de firma soportadas (RSA y Ed25519).
func ParseJWKS(list []jwk) (map[string]crypto.PublicKey, error) {
	out := map[string]crypto.PublicKey{}
	for _, k := range list {
		if k.Use != "" && k.Use != "sig" {
			continue // Keycloak también publica claves de cifrado
		}
		switch k.Kty {
		case "RSA":
			n, err1 := b64.DecodeString(k.N)
			e, err2 := b64.DecodeString(k.E)
			if err1 != nil || err2 != nil || len(n) < 256 { // mínimo 2048 bits
				continue
			}
			out[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}
		case "OKP":
			x, err := b64.DecodeString(k.X)
			if err != nil || k.Crv != "Ed25519" || len(x) != ed25519.PublicKeySize {
				continue
			}
			out[k.Kid] = ed25519.PublicKey(x)
		}
	}
	if len(out) == 0 {
		return nil, errors.New("JWKS sin claves de firma utilizables")
	}
	return out, nil
}

// StaticKeys sirve para pruebas.
type StaticKeys map[string]crypto.PublicKey

func (s StaticKeys) Key(_ context.Context, kid string) (crypto.PublicKey, error) {
	if k, ok := s[kid]; ok {
		return k, nil
	}
	return nil, ErrUnknownKey
}
