package application

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
)

// SecretBox cifra los secretos de integraciones con AES-256-GCM (RF-05,
// RNF-SEG-02): en la base nunca quedan en claro. La clave (32 bytes en hex)
// viene de INTEGRATIONS_KEY y no se sube al repositorio.
type SecretBox struct{ aead cipher.AEAD }

func NewSecretBox(hexKey string) (SecretBox, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return SecretBox{}, errors.New("INTEGRATIONS_KEY debe tener 64 caracteres hex (32 bytes)")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return SecretBox{}, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return SecretBox{}, err
	}
	return SecretBox{aead: aead}, nil
}

func (b SecretBox) Seal(plain string) (string, error) {
	if b.aead == nil {
		return "", errors.New("cifrado de integraciones no configurado (INTEGRATIONS_KEY)")
	}
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b.aead.Seal(nonce, nonce, []byte(plain), nil)), nil
}

func (b SecretBox) Open(enc string) (string, error) {
	if b.aead == nil {
		return "", errors.New("cifrado de integraciones no configurado (INTEGRATIONS_KEY)")
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil || len(raw) < b.aead.NonceSize() {
		return "", errors.New("secreto cifrado inválido")
	}
	n := b.aead.NonceSize()
	plain, err := b.aead.Open(nil, raw[:n], raw[n:], nil)
	if err != nil {
		return "", fmt.Errorf("no se pudo descifrar el secreto: %w", err)
	}
	return string(plain), nil
}

// NewSecret genera un secreto aleatorio para una integración.
func NewSecret() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// VerifySignature acepta la firma HMAC-SHA256 del cuerpo (X-Nexus-Signature,
// hex, opcionalmente con prefijo "sha256=") o, para proveedores que no firman
// (p. ej. Mercado Libre), el secreto como token. Comparación en tiempo constante.
func VerifySignature(secret string, body []byte, signature, token string) bool {
	if signature != "" {
		if len(signature) > 7 && signature[:7] == "sha256=" {
			signature = signature[7:]
		}
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(body)
		want := hex.EncodeToString(mac.Sum(nil))
		return hmac.Equal([]byte(want), []byte(signature))
	}
	if token != "" {
		return subtle.ConstantTimeCompare([]byte(secret), []byte(token)) == 1
	}
	return false
}
