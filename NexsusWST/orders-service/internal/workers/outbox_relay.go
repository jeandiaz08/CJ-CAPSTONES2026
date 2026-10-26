// Package workers - coordinación técnica de trabajos asíncronos (PA-06).
// El Worker es parte de Órdenes (mismo código, base y reglas). No confundir
// con la tarea humana de picking.
package workers

import (
	"context"
	"log/slog"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

// OutboxRelay publica en Redis Streams las intenciones guardadas en
// PostgreSQL (bandeja transaccional), en lotes pequeños y PRIMERO las
// órdenes prioritarias («Enviar primero a la cola», RF-15).
type OutboxRelay struct {
	db     *pgxpool.Pool
	rdb    *redis.Client
	stream string
	batch  int
	log    *slog.Logger
}

func NewOutboxRelay(db *pgxpool.Pool, rdb *redis.Client, stream string, batch int, log *slog.Logger) *OutboxRelay {
	return &OutboxRelay{db: db, rdb: rdb, stream: stream, batch: batch, log: log}
}

func (r *OutboxRelay) Run(ctx context.Context) {
	t := time.NewTicker(time.Second)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if n, err := r.publishBatch(ctx); err != nil {
				r.log.Warn("outbox relay", "err", err)
			} else if n > 0 {
				r.log.Debug("outbox publicado", "jobs", n)
			}
		}
	}
}

func (r *OutboxRelay) publishBatch(ctx context.Context) (int, error) {
	count := 0
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			SELECT id, type, company_id, COALESCE(order_id,0), payload::text FROM job_outbox
			WHERE published_at IS NULL
			ORDER BY priority DESC, created_at
			LIMIT $1
			FOR UPDATE SKIP LOCKED`, r.batch)
		if err != nil {
			return err
		}
		type job struct {
			id, typ, payload string
			company, orderID int64
		}
		var jobs []job
		for rows.Next() {
			var j job
			if err := rows.Scan(&j.id, &j.typ, &j.company, &j.orderID, &j.payload); err != nil {
				rows.Close()
				return err
			}
			jobs = append(jobs, j)
		}
		rows.Close()

		for _, j := range jobs {
			if err := r.rdb.XAdd(ctx, &redis.XAddArgs{
				Stream: r.stream,
				Values: map[string]any{
					"job_id": j.id, "type": j.typ, "payload": j.payload,
					"company_id": strconv.FormatInt(j.company, 10),
					"order_id":   strconv.FormatInt(j.orderID, 10),
				},
			}).Err(); err != nil {
				return err
			}
			if _, err := tx.Exec(ctx, `UPDATE job_outbox SET published_at=now() WHERE id=$1`, j.id); err != nil {
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}
