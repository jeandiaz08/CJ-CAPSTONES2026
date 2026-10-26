package workers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"

	"github.com/nexuswst/orders-service/internal/application"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/platform/config"
)

const (
	maxAttempts  = 5
	reclaimAfter = 60 * time.Second // mensajes sin confirmar se reintentan tras este tiempo
)

// Consumer lee de un grupo de Redis Streams y ejecuta los trabajos.
// Regla: procesar -> persistir resultado -> recién entonces XACK.
type Consumer struct {
	db  *pgxpool.Pool
	rdb *redis.Client
	svc *application.Service
	cfg config.Worker
	log *slog.Logger
}

func NewConsumer(db *pgxpool.Pool, rdb *redis.Client, svc *application.Service, cfg config.Worker, log *slog.Logger) *Consumer {
	return &Consumer{db: db, rdb: rdb, svc: svc, cfg: cfg, log: log.With("component", "consumer")}
}

func (c *Consumer) Run(ctx context.Context) {
	err := c.rdb.XGroupCreateMkStream(ctx, c.cfg.Stream, c.cfg.Group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		c.log.Error("no se pudo crear el grupo de consumidores", "err", err)
		return
	}

	msgs := make(chan redis.XMessage)
	var wg sync.WaitGroup
	for i := 0; i < c.cfg.Concurrency; i++ { // concurrencia limitada
		wg.Add(1)
		go func() {
			defer wg.Done()
			for m := range msgs {
				c.handle(context.WithoutCancel(ctx), m) // termina el trabajo en curso al apagar
			}
		}()
	}
	defer func() { close(msgs); wg.Wait(); c.log.Info("consumidor detenido") }()

	c.log.Info("consumidor iniciado", "stream", c.cfg.Stream, "group", c.cfg.Group, "consumer", c.cfg.Consumer)
	for ctx.Err() == nil {
		// 1) Recuperar mensajes de consumidores caídos / reintentos
		claimed, _, err := c.rdb.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream: c.cfg.Stream, Group: c.cfg.Group, Consumer: c.cfg.Consumer,
			MinIdle: reclaimAfter, Start: "0-0", Count: 10,
		}).Result()
		if err == nil {
			for _, m := range claimed {
				select {
				case msgs <- m:
				case <-ctx.Done():
					return
				}
			}
		}
		// 2) Leer mensajes nuevos (bloquea hasta 5s)
		res, err := c.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
			Group: c.cfg.Group, Consumer: c.cfg.Consumer,
			Streams: []string{c.cfg.Stream, ">"}, Count: 10, Block: 5 * time.Second,
		}).Result()
		if errors.Is(err, redis.Nil) || ctx.Err() != nil {
			continue
		}
		if err != nil {
			c.log.Warn("XREADGROUP", "err", err)
			time.Sleep(2 * time.Second)
			continue
		}
		for _, s := range res {
			for _, m := range s.Messages {
				select {
				case msgs <- m:
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func (c *Consumer) handle(ctx context.Context, m redis.XMessage) {
	jobID, _ := m.Values["job_id"].(string)
	typ, _ := m.Values["type"].(string)
	orderID, _ := strconv.ParseInt(str(m.Values["order_id"]), 10, 64)
	var payload map[string]any
	_ = json.Unmarshal([]byte(str(m.Values["payload"])), &payload)
	log := c.log.With("job_id", jobID, "type", typ, "order_id", orderID)

	// Idempotencia: si ya se completó (mensaje repetido), solo se confirma.
	var done bool
	var attempts int
	if err := c.db.QueryRow(ctx, `SELECT completed_at IS NOT NULL OR failed_at IS NOT NULL, attempts FROM job_outbox WHERE id=$1`, jobID).Scan(&done, &attempts); err != nil {
		log.Warn("trabajo desconocido; se confirma para no bloquear", "err", err)
		c.ack(ctx, m.ID)
		return
	}
	if done {
		c.ack(ctx, m.ID)
		return
	}

	var err error
	switch typ {
	case application.JobAssignWave:
		err = c.assignWave(ctx, orderID, str(payload["wave_id"]))
	case application.JobProcessWebhook:
		inboxID, _ := payload["inbox_id"].(float64)
		err = c.svc.ProcessInbox(ctx, int64(inboxID))
	default:
		err = errors.New("tipo de trabajo no soportado")
	}

	if err == nil {
		_, _ = c.db.Exec(ctx, `UPDATE job_outbox SET completed_at=now(), attempts=attempts+1 WHERE id=$1`, jobID)
		c.ack(ctx, m.ID)
		log.Info("trabajo completado")
		return
	}

	attempts++
	if attempts >= maxAttempts {
		// Agotado: la orden pasa a Incidencia con reejecución autorizada (DAS §5.4)
		_, _ = c.db.Exec(ctx, `UPDATE job_outbox SET failed_at=now(), attempts=$2, last_error=$3 WHERE id=$1`, jobID, attempts, err.Error())
		if orderID > 0 {
			c.toIncident(ctx, orderID, "Invas no pudo asignar la ola tras "+strconv.Itoa(attempts)+" intentos: "+err.Error()+". Usa «Reintentar» al resolver.")
		}
		c.ack(ctx, m.ID)
		log.Error("trabajo fallido definitivamente", "err", err, "attempts", attempts)
		return
	}
	_, _ = c.db.Exec(ctx, `UPDATE job_outbox SET attempts=$2, last_error=$3 WHERE id=$1`, jobID, attempts, err.Error())
	log.Warn("trabajo falló; se reintentará", "err", err, "attempts", attempts)
	// Sin XACK: queda pendiente y XAUTOCLAIM lo retoma tras reclaimAfter.
}

func (c *Consumer) assignWave(ctx context.Context, orderID int64, waveID string) error {
	o, err := c.svc.Orders.Get(ctx, nil, orderID)
	if err != nil {
		return err
	}
	if o.Status != domain.StatusValidated || o.WaveID != waveID {
		return nil // ya avanzó, se canceló o cambió de ola: nada que hacer
	}
	ref, err := c.svc.WMS.AssignToWave(ctx, o, waveID)
	if err != nil {
		return err
	}
	o, err = c.svc.Orders.CompleteWaveAssignment(ctx, orderID, ref)
	if err == nil {
		c.svc.Notifier.OrderChanged(ctx, o)
	}
	return err
}

// toIncident: agotados los reintentos, la orden pasa a Incidencia (vuelve al
// estado previo al resolverse, con reejecución autorizada).
func (c *Consumer) toIncident(ctx context.Context, orderID int64, reason string) {
	c.svc.SystemIncident(ctx, orderID, domain.IncidentInvas, reason, "system:worker")
}

func (c *Consumer) ack(ctx context.Context, id string) {
	if err := c.rdb.XAck(ctx, c.cfg.Stream, c.cfg.Group, id).Err(); err != nil {
		c.log.Warn("XACK", "err", err, "msg_id", id)
	}
}

func str(v any) string { s, _ := v.(string); return s }
