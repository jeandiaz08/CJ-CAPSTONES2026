package application

import (
	"context"
	"log/slog"
	"testing"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

type savedNotices struct {
	ports.NotificationRepository
	saved []ports.Notification
}

func (s *savedNotices) Add(_ context.Context, n ports.Notification) (ports.Notification, error) {
	s.saved = append(s.saved, n)
	return n, nil
}

type noticeSink struct{}

func (noticeSink) OrderChanged(context.Context, domain.Order) {}
func (noticeSink) Notify(context.Context, ports.Notification) {}

// Campana: el título usa el número de la orden (sin el id interno) y el cuerpo no repite el tipo
// de incidencia cuando el motivo ya lo trae.
func TestIncidentNoticeText(t *testing.T) {
	cases := []struct{ name, kind, reason, body string }{
		{"motivo con el tipo (ReportIncident)", domain.IncidentOther, "Otro: Producto dañado en bodega", "Otro: Producto dañado en bodega"},
		{"motivo sin el tipo (worker)", domain.IncidentInvas, "Sin respuesta tras 5 intentos", "Invas no responde. Sin respuesta tras 5 intentos"},
		{"sin motivo", domain.IncidentOther, "", "Otro."},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := &savedNotices{}
			n := &Notices{repo: repo, notifier: noticeSink{}, log: slog.Default()}
			n.OnStatus(context.Background(), domain.Order{ID: 4511, ExternalRef: "20006", Status: domain.StatusIncident, IncidentKind: c.kind}, c.reason)
			if len(repo.saved) != 1 {
				t.Fatalf("avisos: %d", len(repo.saved))
			}
			if got := repo.saved[0].Title; got != "Orden #20006 en Incidencia" {
				t.Errorf("título: %q", got)
			}
			if got := repo.saved[0].Body; got != c.body {
				t.Errorf("cuerpo: %q, quería %q", got, c.body)
			}
		})
	}
}

// Sin número externo se usa el id, para que el aviso nunca quede sin referencia.
func TestOrderRefFallback(t *testing.T) {
	if got := orderRef(domain.Order{ID: 7}); got != "#7" {
		t.Fatalf("orderRef: %q", got)
	}
}
