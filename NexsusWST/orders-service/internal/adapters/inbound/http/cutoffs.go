package http

import (
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/httpx"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
	"net/http"
)

func (h *Handler) cutoffs(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	data, err := h.svc.Cutoffs(r.Context(), id)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, 200, map[string]any{"data": data})
}
func (h *Handler) saveCutoff(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Name       *string `json:"name"`
		CutoffTime string  `json:"cutoffTime"`
		Couriers   []struct {
			Code        string `json:"code"`
			ArrivalTime string `json:"arrivalTime"`
		} `json:"couriers"`
	}
	if !decode(w, r, &body) {
		return
	}
	c := ports.Cutoff{Name: body.Name, CutoffTime: body.CutoffTime}
	if r.Method == "PUT" {
		n, ok := pathInt(w, r, "id")
		if !ok {
			return
		}
		c.ID = n
	}
	for _, v := range body.Couriers {
		c.Couriers = append(c.Couriers, ports.CutoffCourier{Code: v.Code, ArrivalTime: v.ArrivalTime})
	}
	id, _ := tenant.From(r.Context())
	out, err := h.svc.SaveCutoff(r.Context(), id, c)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	status := 200
	if r.Method == "POST" {
		status = 201
	}
	httpx.JSON(w, status, out)
}
func (h *Handler) deleteCutoff(w http.ResponseWriter, r *http.Request) {
	n, ok := pathInt(w, r, "id")
	if !ok {
		return
	}
	id, _ := tenant.From(r.Context())
	if err := h.svc.DeleteCutoff(r.Context(), id, n); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(204)
}
