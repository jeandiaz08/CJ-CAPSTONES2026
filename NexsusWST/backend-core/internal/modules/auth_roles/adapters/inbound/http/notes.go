package http

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
	"github.com/nexuswst/backend-core/internal/shared/httpx"
	"github.com/nexuswst/backend-core/internal/shared/tenant"
)

type noteBody struct {
	Body    json.RawMessage `json:"body"`
	DueDate json.RawMessage `json:"dueDate"`
	Done    json.RawMessage `json:"done"`
}

func decodeNote(r *http.Request) (domain.NoteChanges, error) {
	var b noteBody
	var c domain.NoteChanges
	if err := httpx.Decode(r, &b); err != nil {
		return c, err
	}
	for _, f := range []struct {
		raw      json.RawMessage
		target   any
		nullable bool
	}{
		{b.Body, &c.Body, false}, {b.DueDate, &c.DueDate, true}, {b.Done, &c.Done, false},
	} {
		if len(f.raw) == 0 {
			continue
		}
		if bytes.Equal(bytes.TrimSpace(f.raw), []byte("null")) && !f.nullable {
			return c, domain.ErrInvalid
		}
		if err := json.Unmarshal(f.raw, f.target); err != nil {
			return c, err
		}
	}
	c.DueDateSet = len(b.DueDate) > 0
	return c, nil
}

func (h *Handler) listNotes(w http.ResponseWriter, r *http.Request) {
	id, _ := tenant.From(r.Context())
	out, err := h.svc.ListNotes(r.Context(), id.UserID)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (h *Handler) createNote(w http.ResponseWriter, r *http.Request) {
	c, err := decodeNote(r)
	if err != nil {
		httpx.Error(w, 400, "bad_request", "JSON de nota inválido")
		return
	}
	id, _ := tenant.From(r.Context())
	out, err := h.svc.CreateNote(r.Context(), id.UserID, c)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (h *Handler) updateNote(w http.ResponseWriter, r *http.Request) {
	noteID, ok := pathID(w, r)
	if !ok {
		return
	}
	c, err := decodeNote(r)
	if err != nil {
		httpx.Error(w, 400, "bad_request", "JSON de nota inválido")
		return
	}
	id, _ := tenant.From(r.Context())
	out, err := h.svc.UpdateNote(r.Context(), id.UserID, noteID, c)
	if err != nil {
		writeErr(w, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (h *Handler) deleteNote(w http.ResponseWriter, r *http.Request) {
	noteID, ok := pathID(w, r)
	if !ok {
		return
	}
	id, _ := tenant.From(r.Context())
	if err := h.svc.DeleteNote(r.Context(), id.UserID, noteID); err != nil {
		writeErr(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
