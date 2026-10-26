// Package redisbus - avisos de pantalla vía Redis Pub/Sub (PA-05).
// Pub/Sub NO guarda trabajo crítico: si un aviso se pierde, el navegador
// vuelve a consultar la API al reconectar.
package redisbus

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

// Un canal por empresa: una sesión solo se suscribe a SUS empresas.
func Channel(companyID int64) string { return fmt.Sprintf("nexus:events:orders:company:%d", companyID) }

// AllPattern - personal interno con acceso a todas las empresas.
const AllPattern = "nexus:events:orders:company:*"

type Notifier struct {
	rdb *redis.Client
	log *slog.Logger
}

func NewNotifier(rdb *redis.Client, log *slog.Logger) *Notifier { return &Notifier{rdb: rdb, log: log} }

func (n *Notifier) OrderChanged(ctx context.Context, o domain.Order) {
	msg, _ := json.Marshal(map[string]any{
		"type": "order.changed", "orderId": o.ID, "companyId": o.CompanyID,
		"status": o.Status, "version": o.Version, "priority": o.Priority,
	})
	if err := n.rdb.Publish(ctx, Channel(o.CompanyID), msg).Err(); err != nil {
		n.log.Warn("no se pudo publicar aviso", "err", err, "order_id", o.ID)
	}
}

// Notify publica un aviso de negocio (RF-27) en el canal de su empresa.
// El WebSocket filtra por audiencia (personal interno / cliente).
func (n *Notifier) Notify(ctx context.Context, x ports.Notification) {
	msg, _ := json.Marshal(map[string]any{"type": "notification", "audience": x.Audience, "notification": x,
		"companyId": x.CompanyID, "orderId": x.OrderID})
	if err := n.rdb.Publish(ctx, Channel(x.CompanyID), msg).Err(); err != nil {
		n.log.Warn("no se pudo publicar el aviso", "err", err, "notification_id", x.ID)
	}
}
