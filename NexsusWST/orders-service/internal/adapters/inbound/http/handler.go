// Package http - adaptador de ENTRADA de Órdenes (REST + WebSocket).
// Traefik antepone /api/orders. El contrato completo está en
// orders-service/API.md. Todas las rutas exigen token de Keycloak y aplican
// el alcance de empresas del perfil (RF-03).
package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/redis/go-redis/v9"

	"github.com/nexuswst/orders-service/internal/adapters/outbound/redisbus"
	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/platform/httpserver"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/httpx"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

type Handler struct {
	svc       *application.Service
	rdb       *redis.Client
	profiles  tenant.ProfileStore
	log       *slog.Logger
	wsOrigins []string
}

func NewHandler(svc *application.Service, rdb *redis.Client, profiles tenant.ProfileStore, log *slog.Logger, wsOrigins []string) *Handler {
	return &Handler{svc: svc, rdb: rdb, profiles: profiles, log: log, wsOrigins: wsOrigins}
}

func (h *Handler) Register(mux *http.ServeMux, auth, wsAuth func(http.Handler) http.Handler) {
	r := func(pattern string, f http.HandlerFunc) { mux.Handle(pattern, auth(f)) }

	r("GET /v1/meta", h.meta)

	// Órdenes: ingreso, consulta, exportación (CU-01, CU-08)
	r("GET /v1/orders", h.list)
	r("GET /v1/orders/summary", h.summary)
	r("GET /v1/orders/export", h.export)
	r("POST /v1/orders", h.create)
	r("POST /v1/orders/import", h.importOrders)
	r("GET /v1/orders/import/template", h.importTemplate)
	r("GET /v1/orders/{id}", h.get)
	r("PATCH /v1/orders/{id}", h.edit)
	r("POST /v1/orders/{id}/cancel", h.cancel)
	r("POST /v1/orders/{id}/recover", h.recoverVoided)
	r("POST /v1/orders/{id}/incident", h.reportIncident)
	r("POST /v1/orders/{id}/resolve", h.resolve)
	r("POST /v1/orders/{id}/priority", h.priority)
	r("POST /v1/orders/{id}/simulate-picked", h.simulatePicked)
	r("GET /v1/orders/{id}/tracking", h.tracking)
	r("GET /v1/orders/{id}/documents", h.documents)
	r("POST /v1/orders/{id}/documents", h.uploadDocument)
	r("GET /v1/orders/{id}/documents/{docId}/download", h.downloadDocument)
	r("POST /v1/orders/{id}/label", h.label)

	// Empaque (CU-04)
	r("GET /v1/packing/queue", h.packingQueue)
	r("POST /v1/orders/{id}/packing/take", h.takePacking)
	r("POST /v1/orders/{id}/packing/release", h.releasePacking)
	r("PUT /v1/orders/{id}/packages", h.setPackages)
	r("POST /v1/orders/{id}/packing/confirm", h.confirmPacking)

	// Olas, reglas y couriers (CU-02)
	r("GET /v1/waves", h.waves)
	r("POST /v1/waves/assign", h.assignWave)
	r("POST /v1/waves/run-rules", h.runRules)
	r("GET /v1/wave-rules", h.listRules)
	r("POST /v1/wave-rules", h.saveRule)
	r("PUT /v1/wave-rules/{id}", h.saveRule)
	r("DELETE /v1/wave-rules/{id}", h.deleteRule)
	r("POST /v1/wave-rules/reorder", h.reorderRules)
	r("GET /v1/couriers/settings", h.listCourierSettings)
	r("PUT /v1/couriers/{code}/settings", h.saveCourierSettings)

	// Retiro y entrega al courier (CU-05)
	r("GET /v1/dispatch/couriers", h.readyByCourier)
	r("GET /v1/dispatch/manifests", h.manifests)
	r("POST /v1/dispatch/manifests", h.createManifest)
	r("GET /v1/dispatch/manifests/{id}", h.manifest)
	r("POST /v1/dispatch/manifests/{id}/scan", h.scan)
	r("POST /v1/dispatch/manifests/{id}/close", h.closeManifest)

	// Integraciones (RF-05) y avisos (RF-27)
	r("GET /v1/integrations", h.listIntegrations)
	r("POST /v1/integrations", h.createIntegration)
	r("POST /v1/integrations/{id}/rotate-secret", h.rotateSecret)
	r("POST /v1/integrations/{id}/enable", h.enableIntegration)
	r("POST /v1/integrations/{id}/disable", h.disableIntegration)
	r("GET /v1/notifications", h.notifications)
	r("POST /v1/notifications/read", h.readNotifications)

	mux.Handle("GET /ws", wsAuth(http.HandlerFunc(h.ws)))
}

// ---------------------------------------------------------------------------
// DTO
// ---------------------------------------------------------------------------

type orderDTO struct {
	ID            int64         `json:"id"`
	CompanyID     int64         `json:"companyId"`
	ExternalRef   string        `json:"externalRef"`
	Channel       string        `json:"channel"`
	Source        string        `json:"source"`
	Courier       string        `json:"courier"`
	CourierCode   string        `json:"courierCode,omitempty"`
	RecipientName string        `json:"recipientName"`
	RecipientPh   string        `json:"recipientPhone,omitempty"`
	RecipientMail string        `json:"recipientEmail,omitempty"`
	AddressLine   string        `json:"addressLine"`
	Commune       string        `json:"commune"`
	RegionCode    string        `json:"regionCode,omitempty"`
	DispatchType  string        `json:"dispatchType"`
	Warehouse     string        `json:"warehouse"`
	CustomerNotes string        `json:"customerNotes,omitempty"`
	Status        string        `json:"status"`
	StatusLabel   string        `json:"statusLabel"`
	PrevStatus    string        `json:"prevStatus,omitempty"`
	IncidentKind  string        `json:"incidentKind,omitempty"`
	IncidentLabel string        `json:"incidentLabel,omitempty"`
	Priority      bool          `json:"priority"`
	WaveID        string        `json:"waveId,omitempty"`
	WMSRef        string        `json:"wmsRef,omitempty"`
	Note          string        `json:"note,omitempty"`
	LastError     string        `json:"lastError,omitempty"`
	Attempts      int           `json:"validationAttempts"`
	ClaimedBy     *int64        `json:"claimedBy,omitempty"`
	ClaimedByName string        `json:"claimedByName,omitempty"`
	Editable      bool          `json:"editable"`
	Version       int           `json:"version"`
	Items         []domain.Item `json:"items"`
	CreatedAt     time.Time     `json:"createdAt"`
	UpdatedAt     time.Time     `json:"updatedAt"`
	HandedOverAt  *time.Time    `json:"handedOverAt,omitempty"`
	CancelledAt   *time.Time    `json:"cancelledAt,omitempty"`
}

func toDTO(o domain.Order) orderDTO {
	items := o.Items
	if items == nil {
		items = []domain.Item{}
	}
	return orderDTO{ID: o.ID, CompanyID: o.CompanyID, ExternalRef: o.ExternalRef, Channel: o.Channel, Source: o.Source,
		Courier: o.Courier, CourierCode: o.CourierCode, RecipientName: o.RecipientName, RecipientPh: o.RecipientPhone,
		RecipientMail: o.RecipientEmail, AddressLine: o.AddressLine, Commune: o.Commune, RegionCode: o.RegionCode,
		DispatchType: o.DispatchType, Warehouse: o.Warehouse, CustomerNotes: o.CustomerNotes,
		Status: string(o.Status), StatusLabel: domain.Labels[o.Status], PrevStatus: string(o.PrevStatus),
		IncidentKind: o.IncidentKind, IncidentLabel: domain.IncidentLabels[o.IncidentKind],
		Priority: o.Priority, WaveID: o.WaveID, WMSRef: o.WMSRef, Note: o.Note, LastError: o.LastError,
		Attempts: o.ValidationAttempts, ClaimedBy: o.ClaimedBy, ClaimedByName: o.ClaimedByName, Editable: o.Editable(),
		Version: o.Version, Items: items, CreatedAt: o.CreatedAt, UpdatedAt: o.UpdatedAt,
		HandedOverAt: o.HandedOverAt, CancelledAt: o.CancelledAt}
}

func toDTOs(list []domain.Order) []orderDTO {
	out := make([]orderDTO, 0, len(list))
	for _, o := range list {
		out = append(out, toDTO(o))
	}
	return out
}

// ---------------------------------------------------------------------------
// Utilidades
// ---------------------------------------------------------------------------

func scope(w http.ResponseWriter, r *http.Request) (tenant.Identity, []int64, bool) {
	id, _ := tenant.From(r.Context())
	ids, _, err := id.Scope(r)
	if err != nil {
		httpx.Error(w, http.StatusForbidden, "forbidden", err.Error())
		return id, nil, false
	}
	return id, ids, true
}

func pathInt(w http.ResponseWriter, r *http.Request, name string) (int64, bool) {
	v, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	if err != nil || v <= 0 {
		httpx.Error(w, http.StatusBadRequest, "bad_request", name+" inválido")
		return 0, false
	}
	return v, true
}

func decode(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := httpx.Decode(r, dst); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "JSON inválido: "+err.Error())
		return false
	}
	return true
}

func (h *Handler) writeErr(w http.ResponseWriter, r *http.Request, err error) {
	var ve domain.ValidationErrors
	var op application.OpError
	msg := err.Error()
	if errors.As(err, &op) {
		msg = op.Msg
	}
	switch {
	case errors.As(err, &ve):
		httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "validation_error", "message": "Revisa los datos marcados", "fields": ve})
	case errors.Is(err, domain.ErrNotFound):
		httpx.Error(w, http.StatusNotFound, "not_found", "No encontrado")
	case errors.Is(err, domain.ErrForbidden), errors.Is(err, tenant.ErrForbiddenCompany):
		if op.Msg == "" {
			msg = domain.ErrForbidden.Error()
		}
		httpx.Error(w, http.StatusForbidden, "forbidden", msg)
	case errors.Is(err, domain.ErrNotEditable):
		httpx.Error(w, http.StatusConflict, "not_editable", msg)
	case errors.Is(err, domain.ErrInvalidTransition):
		httpx.Error(w, http.StatusConflict, "invalid_transition", msg)
	case errors.Is(err, domain.ErrConflict):
		httpx.Error(w, http.StatusConflict, "conflict", msg)
	case errors.Is(err, domain.ErrDuplicate):
		httpx.Error(w, http.StatusConflict, "duplicate", msg)
	case errors.Is(err, domain.ErrInvalid):
		httpx.Error(w, http.StatusUnprocessableEntity, "invalid", strings.TrimPrefix(msg, "orden inválida: "))
	default:
		ref := httpserver.RequestIDFrom(r.Context())
		h.log.Error("error no controlado", "err", err, "request_id", ref, "path", r.URL.Path)
		httpx.JSON(w, http.StatusInternalServerError, map[string]string{"error": "internal_error",
			"message": "Error interno. Informa esta referencia de diagnóstico: " + ref, "ref": ref})
	}
}

func parseDate(v string, endOfDay bool) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	loc, _ := time.LoadLocation("America/Santiago")
	if loc == nil {
		loc = time.Local
	}
	t, err := time.ParseInLocation("2006-01-02", v, loc)
	if err != nil {
		return nil, err
	}
	if endOfDay {
		t = t.AddDate(0, 0, 1)
	}
	return &t, nil
}

func splitInts(v string) []int64 {
	var out []int64
	for _, p := range strings.Split(v, ",") {
		if n, err := strconv.ParseInt(strings.TrimSpace(p), 10, 64); err == nil && n > 0 {
			out = append(out, n)
		}
	}
	return out
}

func filterFrom(r *http.Request, ids []int64) (ports.ListFilter, error) {
	q := r.URL.Query()
	f := ports.ListFilter{CompanyIDs: ids, View: q.Get("view"), Search: q.Get("q"), Courier: q.Get("courier"),
		Channel: q.Get("channel"), OnlyPriority: q.Get("priority") == "true", WithoutWave: q.Get("without_wave") == "true",
		IncidentKind: q.Get("incident_kind"), Sort: q.Get("sort"), IDs: splitInts(q.Get("ids"))}
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

// ---------------------------------------------------------------------------
// Meta (catálogos de la interfaz)
// ---------------------------------------------------------------------------

func (h *Handler) meta(w http.ResponseWriter, r *http.Request) {
	statuses := make([]map[string]string, 0, len(domain.AllStatuses))
	for _, s := range domain.AllStatuses {
		statuses = append(statuses, map[string]string{"code": string(s), "label": domain.Labels[s]})
	}
	kinds := make([]map[string]string, 0)
	for _, k := range domain.IncidentKinds {
		kinds = append(kinds, map[string]string{"code": k, "label": domain.IncidentLabels[k]})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"statuses": statuses, "incidentKinds": kinds,
		"dispatchTypes": domain.DispatchTypes, "importColumns": application.ImportColumns,
		"documentKinds":    []string{"factura", "boleta", "guia_despacho", "etiqueta", "otro"},
		"channelProviders": application.ChannelProviders, "courierProviders": application.CourierProviders})
}

// ---------------------------------------------------------------------------
// Órdenes
// ---------------------------------------------------------------------------

func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	_, ids, ok := scope(w, r)
	if !ok {
		return
	}
	f, err := filterFrom(r, ids)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	items, total, err := h.svc.List(r.Context(), f)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": toDTOs(items), "total": total, "limit": limit, "offset": f.Offset})
}

func (h *Handler) summary(w http.ResponseWriter, r *http.Request) {
	_, ids, ok := scope(w, r)
	if !ok {
		return
	}
	s, err := h.svc.Summary(r.Context(), ids)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": s})
}

// export - Excel (por defecto) o CSV del listado filtrado o de los ?ids= seleccionados.
func (h *Handler) export(w http.ResponseWriter, r *http.Request) {
	_, ids, ok := scope(w, r)
	if !ok {
		return
	}
	f, err := filterFrom(r, ids)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	format := r.URL.Query().Get("format")
	var buf bytes.Buffer
	if _, err := h.svc.Export(r.Context(), f, format, &buf); err != nil {
		h.writeErr(w, r, err)
		return
	}
	name := "ordenes-" + time.Now().Format("20060102-1504")
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

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	var d domain.Draft
	if !decode(w, r, &d) {
		return
	}
	o, err := h.svc.Create(r.Context(), id, d)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, toDTO(o))
}

func (h *Handler) importOrders(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	r.Body = http.MaxBytesReader(w, r.Body, application.MaxImportBytes+1<<20)
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Envía el archivo como multipart/form-data en el campo «file» (máx. 5 MB)")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Falta el archivo (campo «file»)")
		return
	}
	defer file.Close()
	companyID, _ := strconv.ParseInt(r.FormValue("companyId"), 10, 64)
	if companyID == 0 {
		companyID, _ = strconv.ParseInt(r.URL.Query().Get("company_id"), 10, 64)
	}
	res, err := h.svc.ImportOrders(r.Context(), id, companyID, hdr.Filename, file)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	code := http.StatusCreated
	if res.FailedRows > 0 {
		code = http.StatusMultiStatus // 207: parte creada, parte con errores
	}
	httpx.JSON(w, code, res)
}

func (h *Handler) importTemplate(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("format") == "csv" {
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", `attachment; filename="plantilla-ordenes.csv"`)
		_, _ = io.WriteString(w, "\xef\xbb\xbf"+strings.Join(application.ImportColumns, ";")+"\n"+
			"PED-1001;chilexpress;domicilio;Ana Pérez;+56 9 1234 5678;ana@correo.cl;Av. Irarrázaval 1234;Ñuñoa;RM;;SKU-001;2\n")
		return
	}
	var buf bytes.Buffer
	if err := application.ImportTemplate(&buf); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="plantilla-ordenes.xlsx"`)
	_, _ = w.Write(buf.Bytes())
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	_, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	d, err := h.svc.Get(r.Context(), ids, oid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"order": toDTO(d.Order), "events": d.Events,
		"packages": d.Packages, "documents": d.Documents,
		"tracking": map[string]any{"available": len(d.Tracking) > 0, "events": d.Tracking}})
}

func (h *Handler) edit(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		domain.Changes
		Reason string `json:"reason"`
	}
	if !decode(w, r, &body) {
		return
	}
	o, err := h.svc.Edit(r.Context(), id, ids, oid, body.Changes, body.Reason)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(o))
}

type reasonBody struct {
	Reason string `json:"reason"`
	Note   string `json:"note"`
}

func (b reasonBody) text() string {
	if b.Reason != "" {
		return b.Reason
	}
	return b.Note
}

// orderAction ejecuta una acción simple sobre la orden y responde la orden resultante.
func (h *Handler) orderAction(w http.ResponseWriter, r *http.Request, f func(id tenant.Identity, scope []int64, oid int64, body reasonBody) (domain.Order, error)) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var body reasonBody
	if r.ContentLength != 0 && !decode(w, r, &body) {
		return
	}
	o, err := f(id, ids, oid, body)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(o))
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	h.orderAction(w, r, func(id tenant.Identity, s []int64, oid int64, b reasonBody) (domain.Order, error) {
		return h.svc.Cancel(r.Context(), id, s, oid, b.text())
	})
}

func (h *Handler) recoverVoided(w http.ResponseWriter, r *http.Request) {
	h.orderAction(w, r, func(id tenant.Identity, s []int64, oid int64, b reasonBody) (domain.Order, error) {
		return h.svc.RecoverVoided(r.Context(), id, s, oid, b.text())
	})
}

func (h *Handler) simulatePicked(w http.ResponseWriter, r *http.Request) {
	h.orderAction(w, r, func(id tenant.Identity, s []int64, oid int64, _ reasonBody) (domain.Order, error) {
		return h.svc.SimulatePicked(r.Context(), id, s, oid)
	})
}

func (h *Handler) reportIncident(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Kind   string `json:"kind"`
		Reason string `json:"reason"`
	}
	if !decode(w, r, &body) {
		return
	}
	o, err := h.svc.ReportIncident(r.Context(), id, ids, oid, body.Kind, body.Reason)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(o))
}

func (h *Handler) resolve(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Action string `json:"action"`
		Note   string `json:"note"`
	}
	if !decode(w, r, &body) {
		return
	}
	if body.Action == "" {
		body.Action = application.ResolveResume
	}
	o, err := h.svc.Resolve(r.Context(), id, ids, oid, body.Action, body.Note)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(o))
}

func (h *Handler) priority(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Priority bool   `json:"priority"`
		Reason   string `json:"reason"`
	}
	if !decode(w, r, &body) {
		return
	}
	o, err := h.svc.SetPriority(r.Context(), id, ids, oid, body.Priority, body.Reason)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, toDTO(o))
}

func (h *Handler) tracking(w http.ResponseWriter, r *http.Request) {
	_, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	ev, err := h.svc.Tracking(r.Context(), ids, oid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	msg := ""
	if len(ev) == 0 {
		msg = "El courier aún no informa seguimiento para esta orden."
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"available": len(ev) > 0, "events": ev, "message": msg})
}

// ---------------------------------------------------------------------------
// Documentos y etiquetas
// ---------------------------------------------------------------------------

func (h *Handler) documents(w http.ResponseWriter, r *http.Request) {
	_, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	d, err := h.svc.Get(r.Context(), ids, oid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": d.Documents})
}

func (h *Handler) uploadDocument(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, application.MaxDocumentBytes+1<<20)
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		httpx.Error(w, http.StatusUnprocessableEntity, "invalid", "Envía el archivo en el campo «file» (máx. 10 MB)")
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		httpx.Error(w, http.StatusBadRequest, "bad_request", "Falta el archivo (campo «file»)")
		return
	}
	defer file.Close()
	doc, err := h.svc.AddDocument(r.Context(), id, ids, oid, r.FormValue("kind"), hdr.Filename, file)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, doc)
}

func (h *Handler) downloadDocument(w http.ResponseWriter, r *http.Request) {
	_, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	did, ok := pathInt(w, r, "docId")
	if !ok {
		return
	}
	d, rc, err := h.svc.OpenDocument(r.Context(), ids, oid, did)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", d.ContentType)
	w.Header().Set("Content-Disposition", "attachment; filename*=UTF-8''"+url.PathEscape(d.Filename))
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(d.SizeBytes, 10))
	_, _ = io.Copy(w, rc)
}

func (h *Handler) label(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	doc, err := h.svc.PrintLabel(r.Context(), id, ids, oid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{"document": doc,
		"downloadPath": fmt.Sprintf("/v1/orders/%d/documents/%d/download", oid, doc.ID)})
}

// ---------------------------------------------------------------------------
// Empaque
// ---------------------------------------------------------------------------

func (h *Handler) packingQueue(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	list, err := h.svc.PackingQueue(r.Context(), id, ids)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": toDTOs(list)})
}

func (h *Handler) takePacking(w http.ResponseWriter, r *http.Request) {
	h.orderAction(w, r, func(id tenant.Identity, s []int64, oid int64, _ reasonBody) (domain.Order, error) {
		return h.svc.TakeForPacking(r.Context(), id, s, oid)
	})
}

func (h *Handler) releasePacking(w http.ResponseWriter, r *http.Request) {
	h.orderAction(w, r, func(id tenant.Identity, s []int64, oid int64, b reasonBody) (domain.Order, error) {
		return h.svc.ReleasePacking(r.Context(), id, s, oid, b.text())
	})
}

func (h *Handler) setPackages(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Packages []domain.Package `json:"packages"`
	}
	if !decode(w, r, &body) {
		return
	}
	p, err := h.svc.SetPackages(r.Context(), id, ids, oid, body.Packages)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": p})
}

func (h *Handler) confirmPacking(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	oid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Items   []domain.Item `json:"items"`
		Version int           `json:"version"`
	}
	if !decode(w, r, &body) {
		return
	}
	res, err := h.svc.ConfirmPacking(r.Context(), id, ids, oid, body.Items, body.Version)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"order": toDTO(res.Order), "discrepancies": res.Discrepancies})
}

// ---------------------------------------------------------------------------
// Olas, reglas y couriers
// ---------------------------------------------------------------------------

func (h *Handler) waves(w http.ResponseWriter, r *http.Request) {
	list, err := h.svc.Waves(r.Context())
	if err != nil {
		httpx.Error(w, http.StatusBadGateway, "wms_unavailable", "No se pudieron obtener las olas de Invas")
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) assignWave(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	var body struct {
		OrderIDs []int64 `json:"orderIds"`
		WaveID   string  `json:"waveId"`
	}
	if !decode(w, r, &body) {
		return
	}
	assigned, err := h.svc.AssignWave(r.Context(), id, ids, body.OrderIDs, body.WaveID)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusAccepted, map[string]any{"queued": assigned,
		"skipped": len(body.OrderIDs) - len(assigned)}) // no Validadas o ya con ola
}

func (h *Handler) runRules(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	n, err := h.svc.RunRulesNow(r.Context(), id)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"assigned": n})
}

func (h *Handler) listRules(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	list, err := h.svc.ListRules(r.Context(), id)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) saveRule(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	var body ports.WaveRule
	if !decode(w, r, &body) {
		return
	}
	body.ID = 0
	if raw := r.PathValue("id"); raw != "" {
		rid, ok := pathInt(w, r, "id")
		if !ok {
			return
		}
		body.ID = rid
	}
	saved, err := h.svc.SaveRule(r.Context(), id, body)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	code := http.StatusOK
	if r.Method == http.MethodPost {
		code = http.StatusCreated
	}
	httpx.JSON(w, code, saved)
}

func (h *Handler) deleteRule(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	rid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteRule(r.Context(), id, rid); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) reorderRules(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := h.svc.ReorderRules(r.Context(), id, body.IDs); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) listCourierSettings(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	list, err := h.svc.ListCourierSettings(r.Context(), id)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) saveCourierSettings(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	var body ports.CourierSettings
	if !decode(w, r, &body) {
		return
	}
	body.Courier = r.PathValue("code")
	if err := h.svc.SaveCourierSettings(r.Context(), id, body); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Retiro
// ---------------------------------------------------------------------------

func (h *Handler) readyByCourier(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ReadyByCourier(r.Context(), id, ids)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) manifests(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	list, err := h.svc.Manifests(r.Context(), id, r.URL.Query().Get("status"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) createManifest(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	var body struct {
		Courier  string  `json:"courier"`
		OrderIDs []int64 `json:"orderIds"`
	}
	if !decode(w, r, &body) {
		return
	}
	res, err := h.svc.CreateManifest(r.Context(), id, ids, body.Courier, body.OrderIDs)
	if err != nil {
		if len(res.Excluded) > 0 {
			httpx.JSON(w, http.StatusUnprocessableEntity, map[string]any{"error": "invalid", "message": err.Error(), "excluded": res.Excluded})
			return
		}
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handler) manifest(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	mid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	m, err := h.svc.Manifest(r.Context(), id, mid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, m)
}

func (h *Handler) scan(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	mid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		Code string `json:"code"`
	}
	if !decode(w, r, &body) {
		return
	}
	line, err := h.svc.ScanPackage(r.Context(), id, mid, body.Code)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, line)
}

func (h *Handler) closeManifest(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	mid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	var body struct {
		PickerName   string `json:"pickerName"`
		PickerID     string `json:"pickerId"`
		VehiclePlate string `json:"vehiclePlate"`
		Notes        string `json:"notes"`
	}
	if !decode(w, r, &body) {
		return
	}
	res, err := h.svc.CloseManifest(r.Context(), id, mid, body.PickerName, body.PickerID, body.VehiclePlate, body.Notes)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

// ---------------------------------------------------------------------------
// Integraciones y avisos
// ---------------------------------------------------------------------------

func (h *Handler) listIntegrations(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListIntegrations(r.Context(), id, ids)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": list})
}

func (h *Handler) createIntegration(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	var body ports.Integration
	if !decode(w, r, &body) {
		return
	}
	res, err := h.svc.CreateIntegration(r.Context(), id, body)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, res)
}

func (h *Handler) rotateSecret(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	iid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	res, err := h.svc.RotateIntegrationSecret(r.Context(), id, ids, iid)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, res)
}

func (h *Handler) enableIntegration(w http.ResponseWriter, r *http.Request) {
	h.setIntegration(w, r, true)
}
func (h *Handler) disableIntegration(w http.ResponseWriter, r *http.Request) {
	h.setIntegration(w, r, false)
}

func (h *Handler) setIntegration(w http.ResponseWriter, r *http.Request, active bool) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	iid, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	if err := h.svc.SetIntegrationActive(r.Context(), id, ids, iid, active); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) notifications(w http.ResponseWriter, r *http.Request) {
	id, ids, ok := scope(w, r)
	if !ok {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	list, unread, err := h.svc.ListNotifications(r.Context(), id, ids, r.URL.Query().Get("unread") == "true", limit)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": list, "unread": unread})
}

func (h *Handler) readNotifications(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	var body struct {
		IDs []int64 `json:"ids"`
	}
	if !decode(w, r, &body) {
		return
	}
	if err := h.svc.MarkNotificationsRead(r.Context(), id, body.IDs); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// WebSocket (RF-26): avisos SOLO de las empresas autorizadas (DAS PA-05)
// ---------------------------------------------------------------------------

func (h *Handler) ws(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{OriginPatterns: h.wsOrigins})
	if err != nil {
		return
	}
	ctx := conn.CloseRead(r.Context()) // el cliente solo escucha
	var sub *redis.PubSub
	if id.AllCompanies {
		sub = h.rdb.PSubscribe(ctx, redisbus.AllPattern)
	} else {
		chans := make([]string, 0, len(id.Companies))
		for _, c := range id.Companies {
			chans = append(chans, redisbus.Channel(c))
		}
		if len(chans) == 0 {
			conn.Close(websocket.StatusPolicyViolation, "sin empresas autorizadas")
			return
		}
		sub = h.rdb.Subscribe(ctx, chans...)
	}
	defer sub.Close()
	isClient := id.HasRole(tenant.RoleCliente)

	// La conexión no sobrevive al token (10 min): al vencer se cierra y la
	// interfaz se reconecta con el token renovado. Además se relee el perfil
	// cada 30 s: un usuario deshabilitado deja de recibir avisos de inmediato.
	expire := time.NewTimer(time.Until(time.Unix(id.TokenExpiresAt, 0)))
	defer expire.Stop()
	ping := time.NewTicker(25 * time.Second)
	defer ping.Stop()
	recheck := time.NewTicker(30 * time.Second)
	defer recheck.Stop()
	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return
		case <-expire.C:
			conn.Close(websocket.StatusPolicyViolation, "token vencido")
			return
		case <-ping.C:
			if err := conn.Ping(ctx); err != nil {
				return
			}
		case <-recheck.C:
			p, err := h.profiles.ProfileByKeycloakID(ctx, id.KeycloakID)
			if err != nil || !p.Active || !sameScope(p, id.Profile) {
				conn.Close(websocket.StatusPolicyViolation, "acceso cambiado: vuelve a conectar")
				return
			}
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if isClient && internalOnly(msg.Payload) {
				continue // los avisos internos no llegan al cliente
			}
			wctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := conn.Write(wctx, websocket.MessageText, []byte(msg.Payload))
			cancel()
			if err != nil {
				return
			}
		}
	}
}

func internalOnly(payload string) bool {
	var m struct {
		Audience string `json:"audience"`
	}
	_ = json.Unmarshal([]byte(payload), &m)
	return m.Audience == "internal"
}

// sameScope indica si el alcance de empresas no cambió desde que se abrió la conexión.
func sameScope(now, then tenant.Profile) bool {
	if now.AllCompanies != then.AllCompanies || len(now.Companies) != len(then.Companies) {
		return false
	}
	for i := range now.Companies {
		if now.Companies[i] != then.Companies[i] {
			return false
		}
	}
	return true
}
