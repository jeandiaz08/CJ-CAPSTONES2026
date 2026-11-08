package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nexuswst/backend-core/internal/shared/httpx"
)

// H-15-T04 (SCRUM-186): toda respuesta de error trae la referencia de
// diagnóstico y un 500 deja su causa en el registro con esa misma referencia.
func stack(buf *bytes.Buffer, h http.HandlerFunc) http.Handler {
	log := slog.New(slog.NewJSONHandler(buf, nil))
	return Chain(h, RequestID, Recoverer(log), Logger(log))
}

func decode(t *testing.T, rec *httptest.ResponseRecorder) map[string]string {
	t.Helper()
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("cuerpo no es JSON: %q", rec.Body.String())
	}
	return body
}

func TestInternalErrorCarriesReferenceAndLogsCause(t *testing.T) {
	var buf bytes.Buffer
	h := stack(&buf, func(w http.ResponseWriter, r *http.Request) {
		httpx.Internal(w, errors.New("pq: conexión rechazada"), "No se pudo listar")
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/v1/products", nil))

	ref := rec.Header().Get("X-Request-Id")
	body := decode(t, rec)
	if rec.Code != 500 || ref == "" || body["ref"] != ref || body["error"] != "internal_error" {
		t.Fatalf("respuesta %d %v (ref %q)", rec.Code, body, ref)
	}
	if !strings.Contains(body["message"], "No se pudo listar") || !strings.Contains(body["message"], ref) {
		t.Fatalf("el mensaje debe pedir informar la referencia: %q", body["message"])
	}
	if strings.Contains(body["message"], "pq:") {
		t.Fatalf("no debe exponer el detalle técnico: %q", body["message"])
	}
	logs := buf.String()
	if !strings.Contains(logs, `"level":"ERROR"`) || !strings.Contains(logs, `"err":"pq: conexión rechazada"`) || !strings.Contains(logs, `"request_id":"`+ref+`"`) {
		t.Fatalf("el registro debe tener nivel ERROR, la causa y la referencia: %s", logs)
	}
}

func TestClientErrorsCarryReferenceToo(t *testing.T) {
	var buf bytes.Buffer
	h := stack(&buf, func(w http.ResponseWriter, r *http.Request) {
		httpx.Error(w, http.StatusUnprocessableEntity, "invalid", "SKU inválido")
	})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/v1/products", nil))
	if body := decode(t, rec); body["ref"] == "" || body["ref"] != rec.Header().Get("X-Request-Id") {
		t.Fatalf("422 sin referencia: %v", body)
	}
	if strings.Contains(buf.String(), `"level":"ERROR"`) {
		t.Fatalf("un 4xx no es un error del servidor: %s", buf.String())
	}
}

func TestRequestIDFromTraefikIsKeptAndBadOnesReplaced(t *testing.T) {
	var buf bytes.Buffer
	h := stack(&buf, func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(204) })
	for in, keep := range map[string]bool{
		"3f2a9c1e-traefik.01":          true,
		"línea\nfalsa":                 false,
		strings.Repeat("a", 65):        false,
		`x" ,"level":"INFO","msg":"ok`: false,
	} {
		req := httptest.NewRequest(http.MethodGet, "/x", nil)
		req.Header.Set("X-Request-Id", in)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		got := rec.Header().Get("X-Request-Id")
		if keep != (got == in) || got == "" || !validRequestID(got) {
			t.Errorf("entrada %q -> %q (conservar=%v)", in, got, keep)
		}
	}
}

func TestPanicAnswersJSONWithReference(t *testing.T) {
	var buf bytes.Buffer
	h := stack(&buf, func(w http.ResponseWriter, r *http.Request) { panic("nil map") })
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/x", nil))
	body := decode(t, rec)
	ref := rec.Header().Get("X-Request-Id")
	if rec.Code != 500 || body["ref"] != ref || !strings.Contains(buf.String(), `"request_id":"`+ref+`"`) {
		t.Fatalf("pánico: %d %v / %s", rec.Code, body, buf.String())
	}
}
