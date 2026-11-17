package http

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/httpx"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// activity - GET /v1/activity (Tarea 06, Fase 2; cambio de Jean) → {items, total, limit, offset}.
// Filtros: module, from, to, actor, role, kind, q, company_id, limit, offset.
func (x *HistoryHandler) activity(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	f, err := activityFilter(r, id)
	if err != nil {
		x.h.writeErr(w, r, err)
		return
	}
	items, total, err := x.activityLog.List(r.Context(), id, f)
	if err != nil {
		x.h.writeErr(w, r, err)
		return
	}
	limit := f.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"items": items, "total": total, "limit": limit, "offset": f.Offset})
}

// activityExport - GET /v1/activity/export?module=…: Excel con lo filtrado del apartado y sus columnas.
func (x *HistoryHandler) activityExport(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	f, err := activityFilter(r, id)
	if err != nil {
		x.h.writeErr(w, r, err)
		return
	}
	var buf bytes.Buffer
	if err := x.activityLog.Export(r.Context(), id, f, &buf); err != nil {
		x.h.writeErr(w, r, err)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", `attachment; filename="registro-acciones-`+f.Module+"-"+time.Now().Format("20060102-1504")+`.xlsx"`)
	_, _ = w.Write(buf.Bytes())
}

func activityFilter(r *http.Request, id tenant.Identity) (ports.ActivityFilter, error) {
	q := r.URL.Query()
	f := ports.ActivityFilter{Module: q.Get("module"), Actor: q.Get("actor"), Role: q.Get("role"), Kind: q.Get("kind"), Search: q.Get("q")}
	// company_id filtra solo esa empresa; sin él, el alcance del usuario (más lo que no tiene empresa).
	if raw := q.Get("company_id"); raw != "" {
		cid, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || !id.CanAccess(cid) {
			return f, tenant.ErrForbiddenCompany
		}
		f.CompanyID = &cid
	} else if !id.AllCompanies {
		f.CompanyIDs = append([]int64{}, id.Companies...)
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
