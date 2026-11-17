package http

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/nexuswst/orders-service/internal/shared/httpx"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// Ticket del WebSocket: el navegador no puede enviar cabeceras al abrirlo y la URL no
// debe llevar el token. Es opaco, sirve una sola vez y vence pronto.
const wsTicketTTL = 30 * time.Second

type wsTicketData struct {
	KeycloakID     string `json:"keycloakId"`
	SessionID      string `json:"sessionId"`
	TokenExpiresAt int64  `json:"tokenExpiresAt"`
}

// En Redis se guarda el hash: leer Redis no entrega tickets utilizables.
func wsTicketKey(ticket string) string {
	sum := sha256.Sum256([]byte(ticket))
	return "nexus:ws-ticket:" + hex.EncodeToString(sum[:])
}

// issueWSTicket entrega un ticket a quien ya se autenticó con su token.
func (h *Handler) issueWSTicket(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		h.writeErr(w, r, err)
		return
	}
	ticket := base64.RawURLEncoding.EncodeToString(raw)
	data, err := json.Marshal(wsTicketData{KeycloakID: id.KeycloakID, SessionID: id.SessionID, TokenExpiresAt: id.TokenExpiresAt})
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	if err := h.rdb.Set(r.Context(), wsTicketKey(ticket), data, wsTicketTTL).Err(); err != nil {
		httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "no se pudo preparar la conexión en vivo")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	httpx.JSON(w, http.StatusOK, map[string]any{"ticket": ticket, "expiresIn": int(wsTicketTTL.Seconds())})
}

// requireWSTicket consume el ticket (GETDEL: una sola vez) y vuelve a leer el perfil, por si
// cambió desde que se pidió.
func (h *Handler) requireWSTicket(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ticket := r.URL.Query().Get("ticket")
		if ticket == "" {
			httpx.Error(w, http.StatusUnauthorized, "invalid_ticket", "falta el ticket de conexión")
			return
		}
		value, err := h.rdb.GetDel(r.Context(), wsTicketKey(ticket)).Result()
		if errors.Is(err, redis.Nil) {
			httpx.Error(w, http.StatusUnauthorized, "invalid_ticket", "el ticket de conexión no es válido o ya venció")
			return
		}
		if err != nil {
			httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "no se pudo verificar el ticket de conexión")
			return
		}
		var data wsTicketData
		if err := json.Unmarshal([]byte(value), &data); err != nil {
			httpx.Error(w, http.StatusUnauthorized, "invalid_ticket", "el ticket de conexión no es válido o ya venció")
			return
		}
		p, err := h.profiles.ProfileByKeycloakID(r.Context(), data.KeycloakID)
		switch {
		case errors.Is(err, tenant.ErrProfileNotFound):
			httpx.Error(w, http.StatusForbidden, "profile_missing", "tu usuario no tiene perfil en NexusWST")
			return
		case err != nil:
			httpx.Error(w, http.StatusServiceUnavailable, "unavailable", "no se pudo verificar tu perfil")
			return
		case !p.Active:
			httpx.Error(w, http.StatusUnauthorized, "user_disabled", "tu usuario está deshabilitado")
			return
		}
		next.ServeHTTP(w, r.WithContext(tenant.With(r.Context(), tenant.Identity{Profile: p, SessionID: data.SessionID, TokenExpiresAt: data.TokenExpiresAt})))
	})
}
