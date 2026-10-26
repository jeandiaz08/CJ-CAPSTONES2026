package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
	"github.com/nexuswst/orders-service/internal/shared/tenant"
)

// Notices - avisos de negocio (RF-27, H-15): quedan guardados, se envían en
// vivo por WebSocket y, cuando corresponde al cliente, también por correo.
// Nunca incluyen datos de otra empresa: cada aviso pertenece a una empresa.
type Notices struct {
	repo     ports.NotificationRepository
	notifier ports.Notifier
	mailer   ports.Mailer
	catalog  ports.CatalogReader
	log      *slog.Logger
}

// OnStatus genera el aviso que corresponde al nuevo estado.
func (n *Notices) OnStatus(ctx context.Context, o domain.Order, reason string) {
	if n == nil || n.repo == nil {
		return
	}
	ref := fmt.Sprintf("#%d (%s)", o.ID, o.ExternalRef)
	switch o.Status {
	case domain.StatusIncident:
		label := domain.IncidentLabels[o.IncidentKind]
		if label == "" {
			label = "Incidencia"
		}
		n.add(ctx, o, "all", "incident", "Orden "+ref+" en Incidencia", fmt.Sprintf("%s. %s", label, reason), false)
	case domain.StatusVoided:
		n.add(ctx, o, "all", "voided", "Orden "+ref+" Anulada por falta de stock",
			"La orden quedó Anulada porque el stock disponible no alcanza. "+reason+
				" Seguirá visible y se puede reintentar cuando haya reposición.", true)
	case domain.StatusCancelled:
		n.add(ctx, o, "all", "cancelled", "Orden "+ref+" Cancelada", reason, false)
	}
}

func (n *Notices) add(ctx context.Context, o domain.Order, audience, kind, title, body string, email bool) {
	oid := o.ID
	saved, err := n.repo.Add(ctx, ports.Notification{CompanyID: o.CompanyID, Audience: audience, OrderID: &oid, Kind: kind, Title: title, Body: body})
	if err != nil {
		n.log.Warn("no se pudo guardar el aviso", "err", err, "order_id", o.ID)
		return
	}
	n.notifier.Notify(ctx, saved)
	if !email || n.mailer == nil {
		return
	}
	to, err := n.catalog.ClientEmails(ctx, o.CompanyID)
	if err != nil || len(to) == 0 {
		return
	}
	go func() { // el correo no retrasa la operación
		if err := n.mailer.Send(context.WithoutCancel(ctx), to, "[NexusWST] "+title, body+"\n\nRevisa la orden en NexusWST."); err != nil {
			n.log.Warn("no se pudo enviar el aviso por correo", "err", err, "order_id", o.ID)
		}
	}()
}

// ---- consulta desde la interfaz -------------------------------------------

func (s *Service) ListNotifications(ctx context.Context, id tenant.Identity, scope []int64, onlyUnread bool, limit int) ([]ports.Notification, int, error) {
	if limit <= 0 || limit > 100 {
		limit = 30
	}
	internal := !id.HasRole(tenant.RoleCliente)
	return s.Notifications.List(ctx, scope, internal, id.UserID, onlyUnread, limit)
}

func (s *Service) MarkNotificationsRead(ctx context.Context, id tenant.Identity, ids []int64) error {
	return s.Notifications.MarkRead(ctx, ids, id.UserID)
}
