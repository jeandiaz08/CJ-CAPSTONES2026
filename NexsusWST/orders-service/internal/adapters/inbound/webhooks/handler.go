// Package webhooks - avisos de canales y couriers (DA-14, H-06, H-14).
//
//	POST /webhooks/{provider}/{integrationId}
//
// NO usan JWT: cada integración se autentica con su propio secreto:
//   - firma HMAC-SHA256 del cuerpo en la cabecera X-Nexus-Signature (hex), o
//   - para proveedores que no firman (Mercado Libre), el secreto como token
//     en X-Nexus-Token o ?token=.
//
// Se responde 200 de inmediato (Mercado Libre lo exige) y el aviso queda en
// webhook_inbox; el Worker lo procesa por la cola (bandeja transaccional).
// Una solicitud sin secreto o firma válidos se rechaza y no crea nada.
package webhooks

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/shared/httpx"
)

type Handler struct {
	svc *application.Service
	log *slog.Logger
}

func NewHandler(svc *application.Service, log *slog.Logger) *Handler {
	return &Handler{svc: svc, log: log.With("component", "webhooks")}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("POST /webhooks/{provider}/{id}", h.receive)
}

func (h *Handler) receive(w http.ResponseWriter, r *http.Request) {
	provider := strings.ToLower(r.PathValue("provider"))
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		httpx.Error(w, http.StatusNotFound, "not_found", "integración no encontrada")
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "cuerpo ilegible")
		return
	}
	token := r.Header.Get("X-Nexus-Token")
	if token == "" {
		token = r.URL.Query().Get("token")
	}
	inboxID, err := h.svc.ReceiveWebhook(r.Context(), provider, id, body, r.Header.Get("X-Nexus-Signature"), token)
	if errors.Is(err, application.ErrWebhookAuth) {
		h.log.Warn("webhook rechazado: secreto o firma inválidos", "provider", provider, "integration", id, "ip", r.RemoteAddr)
		httpx.Error(w, http.StatusUnauthorized, "unauthorized", "firma o secreto inválidos")
		return
	}
	if err != nil {
		h.log.Error("no se pudo registrar el webhook", "err", err)
		httpx.Error(w, http.StatusInternalServerError, "internal_error", "no se pudo registrar el aviso")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"received": inboxID})
}
