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

	// Keycloak: Órdenes solo VERIFICA el token con las claves públicas (DA-11)
	KeycloakInternalURL string
	KeycloakRealm       string
	KeycloakIssuer      string
	KeycloakAudience    string
	AppPublicURL        string
}

func Load() (Config, error) {
	ttl, err := time.ParseDuration(getenv("CACHE_DEFAULT_TTL", "12h"))
	if err != nil {
		return Config{}, fmt.Errorf("CACHE_DEFAULT_TTL inválido: %w", err)
	}
	c := Config{
		ServiceName:     getenv("SERVICE_NAME", "orders"),
		Env:             getenv("APP_ENV", "development"),
		HTTPPort:        getenv("HTTP_PORT", "8081"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		RedisAddr:       getenv("REDIS_ADDR", "redis:6379"),
		RedisPassword:   os.Getenv("REDIS_PASSWORD"),
		CacheDefaultTTL: ttl,
		LogLevel:        getenv("LOG_LEVEL", "info"),

		KeycloakInternalURL: getenv("KEYCLOAK_INTERNAL_URL", "http://keycloak:8080/auth"),
		KeycloakRealm:       getenv("KEYCLOAK_REALM", "nexuswst"),
		KeycloakAudience:    getenv("KEYCLOAK_AUDIENCE", "nexuswst"),
		AppPublicURL:        getenv("APP_PUBLIC_URL", "http://localhost"),
	}
	c.KeycloakIssuer = getenv("KEYCLOAK_ISSUER", c.AppPublicURL+"/auth/realms/"+c.KeycloakRealm)
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL es obligatorio")
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

// Worker agrupa la configuración del consumidor de Redis Streams.
type Worker struct {
	Stream      string
	Group       string
	Consumer    string
	Concurrency int
	Batch       int // envío a Invas en lotes pequeños (RF-16)
}

func LoadWorker() Worker {
	host, _ := os.Hostname()
	n := 2
	if v := os.Getenv("WORKER_CONCURRENCY"); v != "" {
		fmt.Sscanf(v, "%d", &n)
	}
	return Worker{
		Stream:      getenv("ORDERS_STREAM", "nexus:jobs:orders"),
		Group:       getenv("ORDERS_CONSUMER_GROUP", "orders-workers"),
		Consumer:    getenv("WORKER_NAME", host),
		Concurrency: n,
		Batch:       5,
	}
}
