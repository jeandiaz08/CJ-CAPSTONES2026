package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type ctxKey string

const requestIDKey ctxKey = "request_id"

// Chain aplica los middlewares en el orden indicado.
func Chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// RequestID asigna un identificador de operación para correlacionar logs
// entre Traefik, Core y Órdenes (DAS §7.3 "diagnóstico").
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if !validRequestID(id) { // la de Traefik se respeta; una ajena o rara se reemplaza
			id = ""
		}
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
	})
}

// validRequestID evita que una cabecera externa ensucie el registro (saltos de
// línea, textos largos): 1 a 64 caracteres [A-Za-z0-9._-].
func validRequestID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, c := range id {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

func RequestIDFrom(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	err    error // causa de un 500, registrada por httpx.Internal
}

// RecordError guarda la causa técnica de un error para el registro.
func (s *statusRecorder) RecordError(err error) { s.err = err }

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap permite a http.ResponseController (y WebSocket) acceder al writer real.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			if r.URL.Path == "/health" {
				return
			}
			attrs := []any{
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"dur_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFrom(r.Context()),
			}
			if rec.status >= 500 || rec.err != nil {
				if rec.err != nil {
					attrs = append(attrs, "err", rec.err.Error())
				}
				log.Error("http", attrs...)
				return
			}
			log.Info("http", attrs...)
		})
	}
}

func Recoverer(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					ref := RequestIDFrom(r.Context())
					log.Error("panic", "err", rec, "request_id", ref, "stack", string(debug.Stack()))
					w.Header().Set("Content-Type", "application/json; charset=utf-8")
					w.WriteHeader(http.StatusInternalServerError)
					_, _ = w.Write([]byte(`{"error":"internal_error","message":"Error inesperado. Informa esta referencia de diagnóstico: ` + ref + `","ref":"` + ref + `"}`))
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}
