// Package config lee la configuración desde variables de entorno (12-factor).
// Cambiar credenciales o direcciones NO requiere tocar reglas de negocio
// (RNF-POR-01).
package config

import (
	"fmt"
	"os"
	"time"
)

type Config struct {
	ServiceName     string
	Env             string
	HTTPPort        string
	DatabaseURL     string
	RedisAddr       string
	RedisPassword   string
	CacheDefaultTTL time.Duration
	LogLevel        string

	// Keycloak (DAS v1.1 §4.5, DA-11, DA-15)
	KeycloakInternalURL  string // red privada, p. ej. http://keycloak:8080/auth
	KeycloakRealm        string
	KeycloakIssuer       string // "iss" del token = URL pública del realm
	KeycloakAudience     string // "aud" exigido en el token
	KeycloakCoreClientID string // cuenta de servicio (Admin API)
	KeycloakCoreSecret   string
	KeycloakWebClientID  string
	AppPublicURL         string
}

func Load() (Config, error) {
	ttl, err := time.ParseDuration(getenv("CACHE_DEFAULT_TTL", "12h"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_DEFAULT_TTL inválido: %w", err)
	}
	c := Config{
		ServiceName:     getenv("SERVICE_NAME", "core"),
		Env:             getenv("APP_ENV", "development"),
		HTTPPort:        getenv("HTTP_PORT", "8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisAddr:       getenv("REDIS_ADDR", "redis:6379"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		CacheDefaultTTL: ttl,
		LogLevel:        getenv("LOG_LEVEL", "info"),

		KeycloakInternalURL:  getenv("KEYCLOAK_INTERNAL_URL", "http://keycloak:8080/auth"),
		KeycloakRealm:        getenv("KEYCLOAK_REALM", "nexuswst"),
		KeycloakAudience:     getenv("KEYCLOAK_AUDIENCE", "nexuswst"),
		KeycloakCoreClientID: getenv("KEYCLOAK_CORE_CLIENT_ID", "nexus-core"),
		KeycloakCoreSecret:   os.Getenv("KEYCLOAK_CORE_CLIENT_SECRET"),
		KeycloakWebClientID:  getenv("KEYCLOAK_WEB_CLIENT_ID", "nexus-web"),
		AppPublicURL:         getenv("APP_PUBLIC_URL", "http://localhost"),
	}
	c.KeycloakIssuer = getenv("KEYCLOAK_ISSUER", c.AppPublicURL+"/auth/realms/"+c.KeycloakRealm)
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL es obligatorio")
	}
	if c.KeycloakCoreSecret == "" {
		return Config{}, fmt.Errorf("KEYCLOAK_CORE_CLIENT_SECRET es obligatorio (Admin API de Keycloak)")
	}
	return c, nil
}

func (c Config) IsDevelopment() bool { return c.Env == "development" }

// JWKSURL es la dirección interna de las claves públicas de Keycloak.
func (c Config) JWKSURL() string {
	return c.KeycloakInternalURL + "/realms/" + c.KeycloakRealm + "/protocol/openid-connect/certs"
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
