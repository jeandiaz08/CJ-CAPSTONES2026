package http

import (
	"net/http"

	"github.com/nexuswst/orders-service/internal/shared/httpx"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

type assignmentDeadlineBody struct {
	AssignmentDeadline string `json:"assignmentDeadline"`
}

func (h *Handler) assignmentDeadline(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	value, err := h.svc.AssignmentDeadline(r.Context(), id)
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": assignmentDeadlineBody{value}})
}

func (h *Handler) saveAssignmentDeadline(w http.ResponseWriter, r *http.Request) {
	var b assignmentDeadlineBody
	if !decode(w, r, &b) {
		return
	}
	id, _ := tenant.From(r.Context())
	if err := h.svc.SaveAssignmentDeadline(r.Context(), id, b.AssignmentDeadline); err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
