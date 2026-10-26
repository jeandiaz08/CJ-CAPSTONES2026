package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

// IntegrationRepository - integraciones por cliente, bandeja de webhooks y
// seguimiento del courier (RF-05, RF-08, RF-25).
type IntegrationRepository struct{ db *pgxpool.Pool }

func NewIntegrationRepository(db *pgxpool.Pool) *IntegrationRepository {
	return &IntegrationRepository{db: db}
}

var _ ports.IntegrationRepository = (*IntegrationRepository)(nil)

const integCols = `id, company_id, provider, kind, channel, COALESCE(default_courier,''), active, secret_enc, created_by, created_at`

func scanInteg(row pgx.Row) (ports.Integration, error) {
	var i ports.Integration
	err := row.Scan(&i.ID, &i.CompanyID, &i.Provider, &i.Kind, &i.Channel, &i.DefaultCourier, &i.Active, &i.SecretEnc, &i.CreatedBy, &i.CreatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return i, domain.ErrNotFound
	}
	return i, err
}

func (r *IntegrationRepository) List(ctx context.Context, ids []int64) ([]ports.Integration, error) {
	rows, err := r.db.Query(ctx, `SELECT `+integCols+` FROM integrations WHERE ($1::bigint[] IS NULL OR company_id = ANY($1)) ORDER BY id`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ports.Integration{}
	for rows.Next() {
		i, err := scanInteg(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, i)
	}
	return out, rows.Err()
}

func (r *IntegrationRepository) Get(ctx context.Context, id int64) (ports.Integration, error) {
	return scanInteg(r.db.QueryRow(ctx, `SELECT `+integCols+` FROM integrations WHERE id=$1`, id))
}

func (r *IntegrationRepository) Save(ctx context.Context, i ports.Integration) (ports.Integration, error) {
	if i.ID == 0 {
		return scanInteg(r.db.QueryRow(ctx, `INSERT INTO integrations (company_id, provider, kind, channel, default_courier, secret_enc, active, created_by)
			VALUES ($1,$2,$3,$4,NULLIF($5,''),$6,$7,$8) RETURNING `+integCols,
			i.CompanyID, i.Provider, i.Kind, i.Channel, i.DefaultCourier, i.SecretEnc, i.Active, i.CreatedBy))
	}
	return scanInteg(r.db.QueryRow(ctx, `UPDATE integrations SET default_courier=NULLIF($2,''), secret_enc=$3, updated_at=now()
		WHERE id=$1 RETURNING `+integCols, i.ID, i.DefaultCourier, i.SecretEnc))
}

func (r *IntegrationRepository) SetActive(ctx context.Context, id int64, active bool) error {
	tag, err := r.db.Exec(ctx, `UPDATE integrations SET active=$2, updated_at=now() WHERE id=$1`, id, active)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

// Receive registra el aviso y encola su procesamiento en la MISMA transacción
// (bandeja transaccional). Un reenvío del canal también queda registrado.
func (r *IntegrationRepository) Receive(ctx context.Context, provider string, integrationID int64, externalID, payload string) (int64, bool, error) {
	var id int64
	var dup bool
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if externalID != "" {
			if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM webhook_inbox WHERE integration_id=$1 AND external_id=$2)`,
				integrationID, externalID).Scan(&dup); err != nil {
				return err
			}
		}
		var company int64
		if err := tx.QueryRow(ctx, `SELECT company_id FROM integrations WHERE id=$1`, integrationID).Scan(&company); err != nil {
			return err
		}
		if err := tx.QueryRow(ctx, `INSERT INTO webhook_inbox (provider, payload, integration_id, external_id)
			VALUES ($1,$2,$3,NULLIF($4,'')) RETURNING id`, provider, payload, integrationID, externalID).Scan(&id); err != nil {
			return err
		}
		return insertJobs(ctx, tx, []ports.Job{{ID: fmt.Sprintf("process_webhook:%d", id), Type: "process_webhook",
			CompanyID: company, Payload: map[string]any{"inbox_id": id}}})
	})
	return id, dup, err
}

func (r *IntegrationRepository) InboxItem(ctx context.Context, id int64) (ports.InboxItem, error) {
	var it ports.InboxItem
	err := r.db.QueryRow(ctx, `SELECT id, provider, COALESCE(integration_id,0), payload, attempts FROM webhook_inbox WHERE id=$1`, id).
		Scan(&it.ID, &it.Provider, &it.IntegrationID, &it.Payload, &it.Attempts)
	if errors.Is(err, pgx.ErrNoRows) {
		return it, domain.ErrNotFound
	}
	return it, err
}

func (r *IntegrationRepository) FinishInbox(ctx context.Context, id int64, status, errMsg string, orderID *int64) error {
	_, err := r.db.Exec(ctx, `UPDATE webhook_inbox SET status=$2, error=NULLIF($3,''), order_id=$4, processed_at=now(), attempts=attempts+1 WHERE id=$1`,
		id, status, errMsg, orderID)
	return err
}

func (r *IntegrationRepository) AddTracking(ctx context.Context, e ports.TrackingEvent) (bool, error) {
	tag, err := r.db.Exec(ctx, `INSERT INTO tracking_events (order_id, provider, event_id, status_raw, description, occurred_at)
		VALUES ($1,$2,$3,$4,NULLIF($5,''),$6) ON CONFLICT (provider, event_id) DO NOTHING`,
		e.OrderID, e.Provider, e.EventID, e.StatusRaw, e.Description, e.OccurredAt)
	return tag.RowsAffected() == 1, err
}

func (r *IntegrationRepository) Tracking(ctx context.Context, orderID int64) ([]ports.TrackingEvent, error) {
	rows, err := r.db.Query(ctx, `SELECT id, order_id, provider, event_id, status_raw, COALESCE(description,''), occurred_at, received_at
		FROM tracking_events WHERE order_id=$1 ORDER BY occurred_at, id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ports.TrackingEvent{}
	for rows.Next() {
		var e ports.TrackingEvent
		if err := rows.Scan(&e.ID, &e.OrderID, &e.Provider, &e.EventID, &e.StatusRaw, &e.Description, &e.OccurredAt, &e.ReceivedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *IntegrationRepository) FindOrderByRef(ctx context.Context, companyID int64, ref string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `SELECT id FROM orders WHERE company_id=$1 AND external_ref=$2 ORDER BY id LIMIT 1`, companyID, ref).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	return id, err
}

func (r *IntegrationRepository) FindOrderByCourierCode(ctx context.Context, courier, code string) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `SELECT id FROM orders WHERE courier=$1 AND courier_code=$2 ORDER BY id DESC LIMIT 1`, courier, code).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, domain.ErrNotFound
	}
	return id, err
}

// ---------------- Avisos ----------------

type NotificationRepository struct{ db *pgxpool.Pool }

func NewNotificationRepository(db *pgxpool.Pool) *NotificationRepository {
	return &NotificationRepository{db: db}
}

func (r *NotificationRepository) Add(ctx context.Context, n ports.Notification) (ports.Notification, error) {
	err := r.db.QueryRow(ctx, `INSERT INTO notifications (company_id, audience, order_id, kind, title, body)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at`, n.CompanyID, n.Audience, n.OrderID, n.Kind, n.Title, n.Body).
		Scan(&n.ID, &n.CreatedAt)
	return n, err
}

// List: el personal interno ve avisos «internal» y «all»; el cliente, «client» y «all».
func (r *NotificationRepository) List(ctx context.Context, ids []int64, internal bool, userID int64, onlyUnread bool, limit int) ([]ports.Notification, int, error) {
	aud := []string{"all", "client"}
	if internal {
		aud = []string{"all", "internal"}
	}
	const where = `($1::bigint[] IS NULL OR n.company_id = ANY($1)) AND n.audience = ANY($2)`
	var unread int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM notifications n WHERE `+where+`
		AND NOT EXISTS (SELECT 1 FROM notification_reads x WHERE x.notification_id=n.id AND x.user_id=$3)`, ids, aud, userID).Scan(&unread); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.Query(ctx, `SELECT n.id, n.company_id, n.audience, n.order_id, n.kind, n.title, n.body, n.created_at,
			EXISTS (SELECT 1 FROM notification_reads x WHERE x.notification_id=n.id AND x.user_id=$3)
		FROM notifications n WHERE `+where+`
		AND (NOT $4 OR NOT EXISTS (SELECT 1 FROM notification_reads x WHERE x.notification_id=n.id AND x.user_id=$3))
		ORDER BY n.id DESC LIMIT $5`, ids, aud, userID, onlyUnread, limit)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []ports.Notification{}
	for rows.Next() {
		var n ports.Notification
		if err := rows.Scan(&n.ID, &n.CompanyID, &n.Audience, &n.OrderID, &n.Kind, &n.Title, &n.Body, &n.CreatedAt, &n.Read); err != nil {
			return nil, 0, err
		}
		out = append(out, n)
	}
	return out, unread, rows.Err()
}

func (r *NotificationRepository) MarkRead(ctx context.Context, ids []int64, userID int64) error {
	_, err := r.db.Exec(ctx, `INSERT INTO notification_reads (notification_id, user_id)
		SELECT id, $2 FROM notifications WHERE id = ANY($1) ON CONFLICT DO NOTHING`, ids, userID)
	return err
}
