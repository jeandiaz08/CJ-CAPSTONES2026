// Package httpx contiene utilidades HTTP realmente comunes (respuestas JSON).
package httpx

import (
	"encoding/json"
	"net/http"
)

func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// Error responde {error, message, ref}. ref es la referencia de diagnóstico
// (X-Request-Id) que también queda en el registro del servidor (H-15-T04).
func Error(w http.ResponseWriter, status int, code, msg string) {
	body := map[string]string{"error": code, "message": msg}
	if ref := w.Header().Get("X-Request-Id"); ref != "" {
		body["ref"] = ref
	}
	JSON(w, status, body)
}

// Internal responde 500 sin exponer el detalle técnico al usuario: el mensaje
// pide informar la referencia y la causa (err) queda en el registro del
// servidor con esa misma referencia (la escribe el middleware Logger).
func Internal(w http.ResponseWriter, err error, msg string) {
	recordError(w, err)
	ref := w.Header().Get("X-Request-Id")
	if ref != "" {
		msg += ". Informa esta referencia de diagnóstico: " + ref
	}
	Error(w, http.StatusInternalServerError, "internal_error", msg)
}

// ErrorRecorder lo implementa el ResponseWriter del middleware Logger.
type ErrorRecorder interface{ RecordError(error) }

func recordError(w http.ResponseWriter, err error) {
	for w != nil {
		if r, ok := w.(ErrorRecorder); ok {
			r.RecordError(err)
			return
		}
		u, ok := w.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			return
		}
		w = u.Unwrap()
	}
}

func Decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(nil, r.Body, 1<<20))
	dec.DisallowUnknownFields()
	return dec.Decode(dst)
}
