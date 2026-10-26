// Punto de entrada del Orders Service: API REST/WS, webhooks y Worker
// (validación automática + consumidor de Redis Streams) en el mismo proceso
// (DA-06). Órdenes NUNCA llama al Core (DA-12): verifica el token de Keycloak
// con sus claves públicas y lee Clientes/Productos directo de PostgreSQL.
package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	ordershttp "github.com/nexuswst/orders-service/internal/adapters/inbound/http"
	"github.com/nexuswst/orders-service/internal/adapters/inbound/webhooks"
	"github.com/nexuswst/orders-service/internal/adapters/outbound/courier"
	"github.com/nexuswst/orders-service/internal/adapters/outbound/mailer"
	"github.com/nexuswst/orders-service/internal/adapters/outbound/postgres"
	"github.com/nexuswst/orders-service/internal/adapters/outbound/redisbus"
	"github.com/nexuswst/orders-service/internal/adapters/outbound/storage"
	"github.com/nexuswst/orders-service/internal/adapters/outbound/wms"
	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/platform/auth"
	"github.com/nexuswst/orders-service/internal/platform/cache"
	"github.com/nexuswst/orders-service/internal/platform/config"
	"github.com/nexuswst/orders-service/internal/platform/database"
	"github.com/nexuswst/orders-service/internal/platform/httpserver"
	"github.com/nexuswst/orders-service/internal/shared/httpx"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
	"github.com/nexuswst/orders-service/internal/workers"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "chequeo de salud para Docker")
	flag.Parse()
	if *healthcheck {
		os.Exit(httpserver.Healthcheck(envOr("HTTP_PORT", "8081")))
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

	// Token de Keycloak verificado localmente; perfil leído de PostgreSQL en
	// cada petición (sin llamar al Core: DA-12). Órdenes no crea perfiles.
	jwks := auth.NewJWKS(cfg.JWKSURL(), log)
	go jwks.Warmup(ctx)
	verifier := auth.Verifier{Keys: jwks, Issuer: cfg.KeycloakIssuer, Audience: cfg.KeycloakAudience, Leeway: 30 * time.Second}
	profiles := tenant.PGProfiles{DB: db}
	requireAuth := tenant.Middleware(verifier, profiles, nil, false)
	requireAuthWS := tenant.Middleware(verifier, profiles, nil, true)

	// ---- Composición --------------------------------------------------------
	files, err := storage.NewLocal(envOr("ATTACHMENTS_DIR", "/data/attachments"))
	if err != nil {
		log.Error("almacenamiento de adjuntos", "err", err)
		os.Exit(1)
	}
	secrets, err := application.NewSecretBox(os.Getenv("INTEGRATIONS_KEY"))
	if err != nil {
		// Sin clave no se pueden crear ni verificar integraciones; el resto funciona.
		log.Warn("integraciones deshabilitadas", "err", err)
	}
	simCourier := courier.NewSimulated() // [SIMULADO] hasta habilitar cada courier/canal (PV-08)
	svc := application.NewService(application.Deps{
		Orders:        postgres.NewRepository(db),
		Rules:         postgres.NewRuleRepository(db),
		Fulfillment:   postgres.NewFulfillmentRepository(db),
		Integrations:  postgres.NewIntegrationRepository(db),
		Notifications: postgres.NewNotificationRepository(db),
		Catalog:       postgres.NewCatalogReader(db),
		Notifier:      redisbus.NewNotifier(rdb, log),
		Mailer: mailer.New(mailer.Config{Host: os.Getenv("SMTP_HOST"), Port: envOr("SMTP_PORT", "25"),
			From: envOr("SMTP_FROM", "no-reply@westorage.cl"), User: os.Getenv("SMTP_USER"),
			Password: os.Getenv("SMTP_PASSWORD"), StartTLS: os.Getenv("SMTP_STARTTLS") == "true"}, log),
		Files:    files,
		WMS:      wms.NewSimulated(log), // TODO PV-03/04: adaptador real de Invas
		Couriers: simCourier,
		Channels: simCourier,
		Secrets:  secrets,
		Log:      log,
		DevMode:  cfg.IsDevelopment(),
	})
	validator := application.NewValidator(svc)
	rules := application.NewRulesEngine(svc)
	picking := application.NewPickingSync(svc)

	// ---- Worker (forma parte de Órdenes) --------------------------------------
	wcfg := config.LoadWorker()
	bgCtx, stopBg := context.WithCancel(context.Background())
	var bg sync.WaitGroup
	bg.Add(5)
	go func() { defer bg.Done(); workers.RunValidationLoop(bgCtx, validator, 2*time.Second) }()
	go func() {
		defer bg.Done()
		workers.RunEvery(bgCtx, 5*time.Second, func(c context.Context) {
			if _, err := rules.RunOnce(c); err != nil {
				log.Warn("motor de reglas de ola", "err", err)
			}
		})
	}()
	go func() { defer bg.Done(); workers.RunEvery(bgCtx, 10*time.Second, picking.RunOnce) }()
	go func() {
		defer bg.Done()
		workers.NewOutboxRelay(db, rdb, wcfg.Stream, wcfg.Batch, log).Run(bgCtx)
	}()
	go func() {
		defer bg.Done()
		workers.NewConsumer(db, rdb, svc, wcfg, log).Run(bgCtx)
	}()

	// ---- HTTP ---------------------------------------------------------------
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, _ *http.Request) {
		httpx.JSON(w, http.StatusOK, map[string]string{"status": "ok", "service": cfg.ServiceName})
	})
	mux.HandleFunc("GET /ready", func(w http.ResponseWriter, r *http.Request) {
		c, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		status, code := map[string]string{"postgres": "ok", "redis": "ok"}, http.StatusOK
		if err := db.Ping(c); err != nil {
			status["postgres"], code = "down", http.StatusServiceUnavailable
		}
		if err := rdb.Ping(c).Err(); err != nil {
			status["redis"], code = "down", http.StatusServiceUnavailable // Órdenes necesita Streams
		}
		httpx.JSON(w, code, status)
	})

	origins := strings.Split(envOr("WS_ALLOWED_ORIGINS", envOr("APP_DOMAIN", "localhost")), ",")
	ordershttp.NewHandler(svc, rdb, profiles, log, origins).Register(mux, requireAuth, requireAuthWS)
	webhooks.NewHandler(svc, log).Register(mux)

	handler := httpserver.Chain(mux,
		httpserver.RequestID,
		httpserver.Recoverer(log),
		httpserver.Logger(log),
	)

	err = httpserver.Run(":"+cfg.HTTPPort, handler, log, func(context.Context) {
		// Primero se deja de tomar trabajos y se espera el trabajo en curso.
		stopBg()
		bg.Wait()
	})
	if err != nil {
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
