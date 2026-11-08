package http

import (
	"bytes"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/httpx"
)

// HistoryHandler - «Registro histórico» y «Registro de acciones» (Tarea 06, cambio de Jean; ver
// backend_pendientes «Cambios de backend hechos por Jean»). Va en un archivo
// propio para no tocar handler.go; se monta con una línea en cmd/api/main.go.
type HistoryHandler struct {
	h           *Handler // writeErr
	registry    *application.Registry
	activityLog *application.Activity
}

func NewHistoryHandler(registry *application.Registry, activity *application.Activity, log *slog.Logger) *HistoryHandler {
	return &HistoryHandler{h: &Handler{log: log}, registry: registry, activityLog: activity}
}

func (x *HistoryHandler) Register(mux *http.ServeMux, auth func(http.Handler) http.Handler) {
	mux.Handle("GET /v1/orders/registry", auth(http.HandlerFunc(x.list)))
	mux.Handle("GET /v1/orders/registry/export", auth(http.HandlerFunc(x.export)))
	mux.Handle("GET /v1/activity", auth(http.HandlerFunc(x.activity)))
}

type registryDTO struct {
	ID                  int64             `json:"id"`
	ExternalRef         string            `json:"externalRef"`
	CompanyID           int64             `json:"companyId"`
	CompanyName         string            `json:"companyName"`
	Courier             string            `json:"courier"`
	CourierName         string            `json:"courierName"`
	Status              string            `json:"status"`
	StatusLabel         string            `json:"statusLabel"`
	PreviousStatus      string            `json:"previousStatus,omitempty"`
	PreviousStatusLabel string            `json:"previousStatusLabel,omitempty"`
	ClosedAt            time.Time         `json:"closedAt"`
	ClosedBy            application.Actor `json:"closedBy"`
	Reason              string            `json:"reason"`
	Motive              string            `json:"motive"`
	Stale               bool              `json:"stale"` // entró por más de 1 año sin movimiento
}

func registryFilter(r *http.Request, ids []int64) (ports.RegistryFilter, error) {
	q := r.URL.Query()
	f := ports.RegistryFilter{CompanyIDs: ids, Search: q.Get("q"), Courier: q.Get("courier"), IDs: splitInts(q.Get("ids"))}
	if s := q.Get("status"); s != "" {
		f.Statuses = strings.Split(s, ",")
	}
	var err error
	if f.From, err = parseDate(q.Get("from"), false); err != nil {
		return f, fmt.Errorf("%w: fecha «desde» inválida (AAAA-MM-DD)", domain.ErrInvalid)
	}
	if f.To, err = parseDate(q.Get("to"), true); err != nil {
		return f, fmt.Errorf("%w: fecha «hasta» inválida (AAAA-MM-DD)", domain.ErrInvalid)
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	f.Offset, _ = strconv.Atoi(q.Get("offset"))
	return f, nil
}

// list - GET /v1/orders/registry → {data, total, limit, offset}.
func (x *HistoryHandler) list(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	f, err := registryFilter(r, ids)
	if err != nil {
		x.h.writeErr(w, r, err)
		return
	}
	rows, total, err := x.registry.List(r.Context(), id, f)
	if err != nil {
		x.h.writeErr(w, r, err)
		return
	}
	data := make([]registryDTO, 0, len(rows))
	for _, e := range rows {
		data = append(data, registryDTO{ID: e.OrderID, ExternalRef: e.ExternalRef, CompanyID: e.CompanyID, CompanyName: e.CompanyName,
			Courier: e.Courier, CourierName: e.CourierName, Status: e.Status, StatusLabel: domain.Labels[domain.Status(e.Status)],
			PreviousStatus: e.PreviousStatus, PreviousStatusLabel: domain.Labels[domain.Status(e.PreviousStatus)],
			ClosedAt: e.ClosedAt, ClosedBy: e.ClosedBy, Reason: e.Reason, Motive: e.Motive, Stale: e.Stale})
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 20
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": data, "total": total, "limit": limit, "offset": f.Offset})
}

// export - GET /v1/orders/registry/export: Excel (o ?format=csv) con lo filtrado o con ?ids=.
func (x *HistoryHandler) export(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	f, err := registryFilter(r, ids)
	if err != nil {
		x.h.writeErr(w, r, err)
		return
	}
	format := r.URL.Query().Get("format")
	var buf bytes.Buffer
	if err := x.registry.Export(r.Context(), id, f, format, &buf); err != nil {
		x.h.writeErr(w, r, err)
		return
	}
	name := "registro-ordenes-" + time.Now().Format("20060102-1504")
	if format == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		name += ".csv"
	} else {
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		name += ".xlsx"
	}
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = w.Write(buf.Bytes())
}
