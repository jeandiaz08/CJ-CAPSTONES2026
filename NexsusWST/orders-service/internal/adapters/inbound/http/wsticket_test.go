package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

type ticketProfiles map[string]tenant.Profile

func (p ticketProfiles) ProfileByKeycloakID(_ context.Context, id string) (tenant.Profile, error) {
	if profile, ok := p[id]; ok {
		return profile, nil
	}
	return tenant.Profile{}, tenant.ErrProfileNotFound
}

func ticketServer(t *testing.T, profiles ticketProfiles) (*httptest.Server, *redis.Client) {
	t.Helper()
	url := os.Getenv("TEST_REDIS_URL")
	if url == "" {
		t.Skip("TEST_REDIS_URL habilita Redis")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		t.Fatal(err)
	}
	rdb := redis.NewClient(opts)
	t.Cleanup(func() { _ = rdb.Close() })
	h := NewHandler(nil, rdb, profiles, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	mux := http.NewServeMux()
	// Identidad fija: el token ya se validó en el middleware real.
	auth := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Authorization") == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: tenant.Profile{KeycloakID: "kc-1", Active: true}, SessionID: "s", TokenExpiresAt: 999})))
		})
	}
	mux.Handle("POST /v1/ws-ticket", auth(http.HandlerFunc(h.issueWSTicket)))
	mux.Handle("GET /probe", h.requireWSTicket(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, _ := tenant.From(r.Context())
		_ = json.NewEncoder(w).Encode(map[string]any{"user": id.UserID, "expires": id.TokenExpiresAt})
	})))
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, rdb
}

func newTicket(t *testing.T, srv *httptest.Server) string {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/ws-ticket", nil)
	req.Header.Set("Authorization", "Bearer x")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK || res.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("emisión: %d, Cache-Control %q", res.StatusCode, res.Header.Get("Cache-Control"))
	}
	var body struct {
		Ticket    string `json:"ticket"`
		ExpiresIn int    `json:"expiresIn"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if len(body.Ticket) < 40 || body.ExpiresIn != 30 {
		t.Fatalf("ticket %d caracteres, vigencia %d", len(body.Ticket), body.ExpiresIn)
	}
	return body.Ticket
}

func probe(t *testing.T, srv *httptest.Server, query string) int {
	t.Helper()
	res, err := http.Get(srv.URL + "/probe?" + query)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	return res.StatusCode
}

func TestWSTicketIsSingleUseAndExpires(t *testing.T) {
	srv, rdb := ticketServer(t, ticketProfiles{"kc-1": {UserID: 7, KeycloakID: "kc-1", Active: true}})
	ticket := newTicket(t, srv)
	ttl, err := rdb.TTL(context.Background(), wsTicketKey(ticket)).Result()
	if err != nil || ttl <= 0 || ttl > 30*time.Second {
		t.Fatalf("vigencia en Redis: %v %v", ttl, err)
	}
	if n, _ := rdb.Exists(context.Background(), "nexus:ws-ticket:"+ticket).Result(); n != 0 {
		t.Fatal("el ticket no debe guardarse en claro")
	}
	if got := probe(t, srv, "ticket="+ticket); got != http.StatusOK {
		t.Fatalf("primer uso: %d", got)
	}
	if got := probe(t, srv, "ticket="+ticket); got != http.StatusUnauthorized {
		t.Fatalf("segundo uso: %d", got)
	}
	expired := newTicket(t, srv)
	if err := rdb.Del(context.Background(), wsTicketKey(expired)).Err(); err != nil {
		t.Fatal(err)
	}
	if got := probe(t, srv, "ticket="+expired); got != http.StatusUnauthorized {
		t.Fatalf("vencido: %d", got)
	}
}

func TestWSHandshakeRejectsAccessToken(t *testing.T) {
	srv, _ := ticketServer(t, ticketProfiles{"kc-1": {KeycloakID: "kc-1", Active: true}})
	for _, query := range []string{"", "access_token=cualquiera", "ticket=inventado"} {
		if got := probe(t, srv, query); got != http.StatusUnauthorized {
			t.Fatalf("%q: %d", query, got)
		}
	}
}

func TestWSTicketChecksProfileAgain(t *testing.T) {
	profiles := ticketProfiles{"kc-1": {KeycloakID: "kc-1", Active: true}}
	srv, _ := ticketServer(t, profiles)
	ticket := newTicket(t, srv)
	profiles["kc-1"] = tenant.Profile{KeycloakID: "kc-1", Active: false}
	if got := probe(t, srv, "ticket="+ticket); got != http.StatusUnauthorized {
		t.Fatalf("deshabilitado: %d", got)
	}
	ticket = newTicket(t, srv)
	delete(profiles, "kc-1")
	if got := probe(t, srv, "ticket="+ticket); got != http.StatusForbidden {
		t.Fatalf("sin perfil: %d", got)
	}
}

func TestWSTicketRedisDown(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", MaxRetries: -1, DialTimeout: 50 * time.Millisecond})
	defer rdb.Close()
	h := NewHandler(nil, rdb, ticketProfiles{}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	rec := httptest.NewRecorder()
	h.requireWSTicket(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error(errors.New("no debía pasar")) })).
		ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ws?ticket=x", strings.NewReader("")))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("Redis caído: %d", rec.Code)
	}
}
