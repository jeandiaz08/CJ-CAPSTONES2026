// Punto de entrada del Backend Core (monolito modular).
// Aquí SOLO se compone la aplicación: conexiones, adaptadores, casos de uso y
// rutas de cada módulo. Para agregar un módulo: créalo en internal/modules/ y
// regístralo en la sección "Composición de módulos".
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/nexuswst/backend-core/internal/platform/auth"
	"github.com/nexuswst/backend-core/internal/platform/cache"
	"github.com/nexuswst/backend-core/internal/platform/config"
	"github.com/nexuswst/backend-core/internal/platform/database"
	"github.com/nexuswst/backend-core/internal/platform/httpserver"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"

	authhttp "github.com/nexuswst/backend-core/internal/modules/auth_roles/adapters/inbound/http"
	authkc "github.com/nexuswst/backend-core/internal/modules/auth_roles/adapters/outbound/keycloak"
	authpg "github.com/nexuswst/backend-core/internal/modules/auth_roles/adapters/outbound/postgres"
	authapp "github.com/nexuswst/backend-core/internal/modules/auth_roles/application"

	catalogshttp "github.com/nexuswst/backend-core/internal/modules/catalogs/adapters/inbound/http"
	catalogspg "github.com/nexuswst/backend-core/internal/modules/catalogs/adapters/outbound/postgres"
	catalogsredis "github.com/nexuswst/backend-core/internal/modules/catalogs/adapters/outbound/rediscache"
	catalogsapp "github.com/nexuswst/backend-core/internal/modules/catalogs/application"

	clientshttp "github.com/nexuswst/backend-core/internal/modules/clients/adapters/inbound/http"
	clientspg "github.com/nexuswst/backend-core/internal/modules/clients/adapters/outbound/postgres"
	clientsapp "github.com/nexuswst/backend-core/internal/modules/clients/application"

	productshttp "github.com/nexuswst/backend-core/internal/modules/products/adapters/inbound/http"
	productspg "github.com/nexuswst/backend-core/internal/modules/products/adapters/outbound/postgres"
	productsapp "github.com/nexuswst/backend-core/internal/modules/products/application"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "chequeo de salud para Docker")
	flag.Parse()
	if *healthcheck {
		os.Exit(httpserver.Healthcheck(envOr("HTTP_PORT", "8080")))
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuración", "err", err)
		os.Exit(1)
	}
	log := newLogger(cfg)
	ctx := context.Background()

	db, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("postgres", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	rdb, err := cache.Connect(ctx, cfg.RedisAddr, cfg.RedisPassword)
	if err != nil {
		log.Error("redis", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()

	// ---- Token de Keycloak: verificación local con claves públicas en caché ----
	jwks := auth.NewJWKS(cfg.JWKSURL(), log)
	go jwks.Warmup(ctx)
	verifier := auth.Verifier{Keys: jwks, Issuer: cfg.KeycloakIssuer, Audience: cfg.KeycloakAudience, Leeway: 30 * time.Second}
	profiles := tenant.PGProfiles{DB: db}

	// ---- Composición de módulos -----------------------------------------------
	idp := authkc.New(authkc.Config{BaseURL: cfg.KeycloakInternalURL, Realm: cfg.KeycloakRealm,
		ClientID: cfg.KeycloakCoreClientID, ClientSecret: cfg.KeycloakCoreSecret,
		AppURL: cfg.AppPublicURL, WebClientID: cfg.KeycloakWebClientID})
	authSvc := authapp.NewService(authpg.NewRepository(db), idp, profiles, log)
	// El Core crea el perfil en el primer ingreso (es dueño de la tabla users).
	requireAuth := tenant.Middleware(verifier, profiles, authSvc, false)
	clientsSvc := clientsapp.NewService(clientspg.NewRepository(db))
	productsSvc := productsapp.NewService(productspg.NewRepository(db))
	catalogsSvc := catalogsapp.NewService(catalogspg.NewSource(db), catalogsredis.New(rdb, cfg.Env), cfg.CacheDefaultTTL, log)

	// Precarga de catálogos al iniciar (RF-29). No bloquea el arranque.
	// TODO (PV-13): programar también la precarga al inicio de jornada.
	go func() {
		pctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := catalogsSvc.Preload(pctx); err != nil {
			log.Warn("precarga de catálogos falló", "err", err)
		}
	}()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": cfg.ServiceName})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		c, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status := map[string]string{"postgres": "ok", "redis": "ok"}
		code := http.StatusOK
		if err := db.Ping(c); err != nil {
			status["postgres"], code = "down", http.StatusServiceUnavailable
		}
		if err := rdb.Ping(c).Err(); err != nil {
			status["redis"] = "down" // caché caída != servicio caído (DAS §7.3)
		}
		httpx.JSON(w, code, status)
	})

	authhttp.NewHandler(authSvc).Register(mux, requireAuth)
	clientshttp.NewHandler(clientsSvc).Register(mux, requireAuth)
	productshttp.NewHandler(productsSvc).Register(mux, requireAuth)
	catalogshttp.NewHandler(catalogsSvc).Register(mux, requireAuth)
	// TODO: inventory, reception, billing -> mismo patrón que products.

	handler := httpserver.Chain(mux,
		httpserver.RequestID,
		httpserver.Recoverer(log),
		httpserver.Logger(log),
	)

	if err := httpserver.Run(":"+cfg.HTTPPort, handler, log); err != nil {
		log.Error("servidor", "err", err)
		os.Exit(1)
	}
}

func newLogger(cfg config.Config) *slog.Logger {
	level := slog.LevelInfo
	if cfg.LogLevel == "debug" {
		level = slog.LevelDebug
	}
	var h slog.Handler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	if cfg.IsDevelopment() {
		h = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: level})
	}
	return slog.New(h).With("service", cfg.ServiceName)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
