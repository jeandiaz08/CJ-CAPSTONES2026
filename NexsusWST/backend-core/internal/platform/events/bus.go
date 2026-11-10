// Package events publica avisos de pantalla (campana) en el mismo canal de Redis que usa Órdenes
// (PA-05): un canal por empresa. El WebSocket de Órdenes los reenvía a quien tenga esa empresa y
// filtra por audiencia (los avisos «internal» no llegan al Cliente). Pub/Sub no guarda nada: si un
// aviso se pierde, la pantalla vuelve a consultar la API.
package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"
)

// Channel - mismo formato que orders-service/internal/adapters/outbound/redisbus.
func Channel(companyID int64) string { return fmt.Sprintf("nexus:events:orders:company:%d", companyID) }

// Notice es el aviso de la campana. Audience: internal | client | all.
type Notice struct {
	ID        string `json:"id"`
	CompanyID int64  `json:"companyId"`
	Audience  string `json:"audience"`
	Kind      string `json:"kind"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	RequestID int64  `json:"requestId,omitempty"`
	// UserID: si viene, el aviso es solo para ese usuario (la pantalla lo filtra; Tarea 08: respuesta a
	// una solicitud interna, que va por el canal interno).
	UserID    int64     `json:"userId,omitempty"`
	CreatedAt time.Time `json:"createdAt"`
}

type Bus struct {
	rdb *redis.Client
	log *slog.Logger
}

func New(rdb *redis.Client, log *slog.Logger) *Bus { return &Bus{rdb: rdb, log: log} }

// Notify publica el aviso; si Redis falla, solo queda en el log (no bloquea la acción).
func (b *Bus) Notify(ctx context.Context, n Notice) {
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	msg, _ := json.Marshal(map[string]any{"type": "notification", "audience": n.Audience, "companyId": n.CompanyID, "notification": n})
	if err := b.rdb.Publish(context.WithoutCancel(ctx), Channel(n.CompanyID), msg).Err(); err != nil {
		b.log.Warn("no se pudo publicar el aviso", "kind", n.Kind, "err", err)
	}
}
