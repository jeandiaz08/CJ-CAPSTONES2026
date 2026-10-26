// Package postgres implementa los repositorios de Órdenes (PA-02).
// Dueño de las tablas de Órdenes; LEE en solo lectura companies, products,
// users, couriers y communes del Core (DA-12).
package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

type Repository struct{ db *pgxpool.Pool }

func NewRepository(db *pgxpool.Pool) *Repository { return &Repository{db: db} }

var _ ports.OrderRepository = (*Repository)(nil)

// La toma de empaque de un usuario deshabilitado se ignora (vuelve a la bandeja).
const fromOrders = `orders o LEFT JOIN users cu ON cu.id = o.claimed_by AND cu.active`

const cols = `o.id, o.company_id, o.external_ref, o.channel, COALESCE(o.courier,''), COALESCE(o.courier_code,''),
	COALESCE(o.recipient_name,''), COALESCE(o.recipient_phone,''), COALESCE(o.recipient_email,''),
	COALESCE(o.address_line,''), COALESCE(o.commune,''), COALESCE(o.region_code,''),
	o.dispatch_type, o.warehouse, COALESCE(o.customer_notes,''), o.source,
	o.status, COALESCE(o.prev_status,''), COALESCE(o.incident_kind,''),
	o.priority, COALESCE(o.wave_id,''), COALESCE(o.wms_ref,''), o.version, COALESCE(o.note,''),
	o.validation_attempts, o.next_retry_at, COALESCE(o.last_error,''),
	o.claimed_by, COALESCE(cu.name,''), o.claimed_at, o.created_by, o.import_id,
	o.created_at, o.updated_at, o.handed_over_at, o.cancelled_at`

func scan(row pgx.Row) (domain.Order, error) {
	var o domain.Order
	var st, prev string
	err := row.Scan(&o.ID, &o.CompanyID, &o.ExternalRef, &o.Channel, &o.Courier, &o.CourierCode,
		&o.RecipientName, &o.RecipientPhone, &o.RecipientEmail,
		&o.AddressLine, &o.Commune, &o.RegionCode,
		&o.DispatchType, &o.Warehouse, &o.CustomerNotes, &o.Source,
		&st, &prev, &o.IncidentKind,
		&o.Priority, &o.WaveID, &o.WMSRef, &o.Version, &o.Note,
		&o.ValidationAttempts, &o.NextRetryAt, &o.LastError,
		&o.ClaimedBy, &o.ClaimedByName, &o.ClaimedAt, &o.CreatedBy, &o.ImportID,
		&o.CreatedAt, &o.UpdatedAt, &o.HandedOverAt, &o.CancelledAt)
	o.Status, o.PrevStatus = domain.Status(st), domain.Status(prev)
	return o, err
}

// querier: pool o transacción
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func getOne(ctx context.Context, q querier, id int64, lock bool) (domain.Order, error) {
	sql := `SELECT ` + cols + ` FROM ` + fromOrders + ` WHERE o.id=$1`
	if lock {
		sql += ` FOR UPDATE OF o`
	}
	o, err := scan(q.QueryRow(ctx, sql, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrNotFound
	}
	if err != nil {
		return o, err
	}
	return o, loadItems(ctx, q, &o)
}

func loadItems(ctx context.Context, q querier, o *domain.Order) error {
	rows, err := q.Query(ctx, `SELECT sku, quantity FROM order_items WHERE order_id=$1 ORDER BY id`, o.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	o.Items = []domain.Item{}
	for rows.Next() {
		var it domain.Item
		if err := rows.Scan(&it.SKU, &it.Quantity); err != nil {
			return err
		}
		o.Items = append(o.Items, it)
	}
	return rows.Err()
}

// loadItemsBulk carga las líneas de varias órdenes en una sola consulta.
func loadItemsBulk(ctx context.Context, q querier, orders []domain.Order) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]int64, len(orders))
	pos := map[int64]int{}
	for i, o := range orders {
		ids[i], pos[o.ID] = o.ID, i
		orders[i].Items = []domain.Item{}
	}
	rows, err := q.Query(ctx, `SELECT order_id, sku, quantity FROM order_items WHERE order_id = ANY($1) ORDER BY id`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var oid int64
		var it domain.Item
		if err := rows.Scan(&oid, &it.SKU, &it.Quantity); err != nil {
			return err
		}
		orders[pos[oid]].Items = append(orders[pos[oid]].Items, it)
	}
	return rows.Err()
}

func replaceItems(ctx context.Context, q querier, orderID int64, items []domain.Item) error {
	if _, err := q.Exec(ctx, `DELETE FROM order_items WHERE order_id=$1`, orderID); err != nil {
		return err
	}
	for _, it := range items {
		if _, err := q.Exec(ctx, `INSERT INTO order_items (order_id, sku, quantity) VALUES ($1,$2,$3)`, orderID, it.SKU, it.Quantity); err != nil {
			return err
		}
	}
	return nil
}

func addEvent(ctx context.Context, q querier, orderID int64, typ, ev, from, to, actor, reason string) error {
	_, err := q.Exec(ctx, `
		INSERT INTO order_events (order_id, event_type, event, from_status, to_status, actor, reason)
		VALUES ($1,$2,$3,NULLIF($4,''),NULLIF($5,''),$6,NULLIF($7,''))`, orderID, typ, ev, from, to, actor, reason)
	return err
}

func insertJobs(ctx context.Context, q querier, jobs []ports.Job) error {
	for _, j := range jobs {
		payload, _ := json.Marshal(j.Payload)
		if _, err := q.Exec(ctx, `
			INSERT INTO job_outbox (id, type, company_id, order_id, priority, payload)
			VALUES ($1,$2,$3,NULLIF($4,0),$5,$6) ON CONFLICT (id) DO NOTHING`, j.ID, j.Type, j.CompanyID, j.OrderID, j.Priority, payload); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) Create(ctx context.Context, o domain.Order, actor string, createdBy *int64) (domain.Order, error) {
	var out domain.Order
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var id int64
		if err := tx.QueryRow(ctx, `
			INSERT INTO orders (company_id, external_ref, channel, courier, status, version, next_retry_at,
				recipient_name, recipient_phone, recipient_email, address_line, commune, region_code,
				dispatch_type, warehouse, customer_notes, source, created_by, import_id)
			VALUES ($1,$2,$3,NULLIF($4,''),$5,1,now(), NULLIF($6,''),NULLIF($7,''),NULLIF($8,''),NULLIF($9,''),NULLIF($10,''),NULLIF($11,''),
				$12,$13,NULLIF($14,''),$15,$16,$17)
			RETURNING id`, o.CompanyID, o.ExternalRef, o.Channel, o.Courier, string(o.Status),
			o.RecipientName, o.RecipientPhone, o.RecipientEmail, o.AddressLine, o.Commune, o.RegionCode,
			o.DispatchType, o.Warehouse, o.CustomerNotes, o.Source, createdBy, o.ImportID).Scan(&id); err != nil {
			return err
		}
		if err := replaceItems(ctx, tx, id, o.Items); err != nil {
			return err
		}
		origin := map[string]string{"manual": "Ingreso manual", "bulk": "Carga masiva", "webhook": "Integración " + o.Channel}[o.Source]
		if err := addEvent(ctx, tx, id, "created", "create", "", string(o.Status), actor, origin); err != nil {
			return err
		}
		var err error
		out, err = getOne(ctx, tx, id, false)
		return err
	})
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" { // RF-13 duplicados
		return domain.Order{}, domain.ErrDuplicate
	}
	return out, err
}

func (r *Repository) Get(ctx context.Context, ids []int64, id int64) (domain.Order, error) {
	o, err := scan(r.db.QueryRow(ctx, `SELECT `+cols+` FROM `+fromOrders+`
		WHERE ($1::bigint[] IS NULL OR o.company_id = ANY($1)) AND o.id=$2`, ids, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Order{}, domain.ErrNotFound // no revela si existe en otra empresa
	}
	if err != nil {
		return o, err
	}
	return o, loadItems(ctx, r.db, &o)
}

func (r *Repository) List(ctx context.Context, f ports.ListFilter) ([]domain.Order, int, error) {
	where := []string{"($1::bigint[] IS NULL OR o.company_id = ANY($1))"}
	args := []any{f.CompanyIDs}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if len(f.Statuses) > 0 {
		add("o.status = ANY(?::text[])", f.Statuses)
	}
	switch f.View {
	case ports.ViewRegistry:
		where = append(where, "o.status IN ('handed_over','cancelled')")
	case ports.ViewAll:
	default:
		if len(f.Statuses) == 0 || !contains(f.Statuses, "cancelled") {
			where = append(where, "o.status <> 'cancelled'") // Cancelada: oculta por defecto
		}
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		add(`(o.id::text = ? OR o.external_ref ILIKE '%' || ? || '%' OR o.recipient_name ILIKE '%' || ? || '%' OR o.courier_code = ?)`, q)
	}
	if f.Courier != "" {
		add("o.courier = ?", strings.ToLower(f.Courier))
	}
	if f.Channel != "" {
		add("o.channel = ?", strings.ToLower(f.Channel))
	}
	if f.From != nil {
		add("o.created_at >= ?", *f.From)
	}
	if f.To != nil {
		add("o.created_at < ?", *f.To)
	}
	if f.OnlyPriority {
		where = append(where, "o.priority")
	}
	if f.WithoutWave {
		where = append(where, "o.wave_id IS NULL")
	}
	if f.IncidentKind != "" {
		add("o.incident_kind = ?", f.IncidentKind)
	}
	if len(f.IDs) > 0 {
		add("o.id = ANY(?::bigint[])", f.IDs)
	}
	w := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM orders o WHERE `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	order := "o.priority DESC, o.id DESC"
	switch f.Sort {
	case "oldest":
		order = "o.id ASC"
	case "recent":
		order = "o.id DESC"
	case "updated":
		order = "o.updated_at DESC"
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, fmt.Sprintf(`SELECT %s FROM %s WHERE %s ORDER BY %s LIMIT $%d OFFSET $%d`,
		cols, fromOrders, w, order, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	out := make([]domain.Order, 0)
	for rows.Next() {
		o, err := scan(rows)
		if err != nil {
			rows.Close()
			return nil, 0, err
		}
		out = append(out, o)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return out, total, loadItemsBulk(ctx, r.db, out)
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

func (r *Repository) Summary(ctx context.Context, ids []int64) (map[domain.Status]int, error) {
	rows, err := r.db.Query(ctx, `SELECT status, count(*) FROM orders
		WHERE ($1::bigint[] IS NULL OR company_id = ANY($1)) GROUP BY status`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[domain.Status]int{}
	for rows.Next() {
		var s string
		var n int
		if err := rows.Scan(&s, &n); err != nil {
			return nil, err
		}
		out[domain.Status(s)] = n
	}
	return out, rows.Err()
}

func (r *Repository) Events(ctx context.Context, orderID int64) ([]ports.EventRecord, error) {
	rows, err := r.db.Query(ctx, `
		SELECT id, event_type, event, COALESCE(from_status,''), COALESCE(to_status,''), actor, COALESCE(reason,''), occurred_at
		FROM order_events WHERE order_id=$1 ORDER BY id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ports.EventRecord, 0)
	for rows.Next() {
		var e ports.EventRecord
		if err := rows.Scan(&e.ID, &e.Type, &e.Event, &e.FromStatus, &e.ToStatus, &e.Actor, &e.Reason, &e.OccurredAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

func (r *Repository) AddEvent(ctx context.Context, orderID int64, typ, ev, actor, reason string) error {
	return addEvent(ctx, r.db, orderID, typ, ev, "", "", actor, reason)
}

func (r *Repository) Transition(ctx context.Context, o domain.Order, from domain.Status, ev domain.Event, actor, reason string, opts ports.TransitionOpts) (domain.Order, error) {
	var out domain.Order
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var err error
		if out, err = transitionTx(ctx, tx, o, from, ev, actor, reason, opts.ClearWave); err != nil {
			return err
		}
		if opts.ReleaseCommitted {
			if err := releaseCommitted(ctx, tx, o); err != nil {
				return err
			}
		}
		return insertJobs(ctx, tx, opts.Jobs)
	})
	return out, err
}

// transitionTx guarda estado y campos de control con verificación de versión.
func transitionTx(ctx context.Context, tx pgx.Tx, o domain.Order, from domain.Status, ev domain.Event, actor, reason string, clearWave bool) (domain.Order, error) {
	nextRetry := "NULL"
	if o.Status == domain.StatusPending || o.Status == domain.StatusValidating { // vuelve a la cola del validador
		nextRetry = "now()"
	}
	tag, err := tx.Exec(ctx, `
		UPDATE orders SET status=$1, prev_status=NULLIF($2,''), incident_kind=NULLIF($3,''),
			note=CASE WHEN $1 IN ('incident','voided','cancelled') THEN NULLIF($4,'') ELSE NULL END,
			validation_attempts=$5, next_retry_at=`+nextRetry+`,
			last_error=CASE WHEN $1 IN ('pending','validating') THEN last_error ELSE NULL END,
			claimed_by=$6, claimed_at=$7,
			wave_id=CASE WHEN $10 THEN NULL ELSE wave_id END,
			rules_checked_at=CASE WHEN $1='validated' THEN NULL ELSE rules_checked_at END,
			handed_over_at=CASE WHEN $1='handed_over' THEN now() ELSE handed_over_at END,
			cancelled_at=CASE WHEN $1='cancelled' THEN now() ELSE cancelled_at END,
			version=version+1, updated_at=now()
		WHERE id=$8 AND version=$9`,
		string(o.Status), string(o.PrevStatus), o.IncidentKind, reason, o.ValidationAttempts,
		o.ClaimedBy, o.ClaimedAt, o.ID, o.Version, clearWave)
	if err != nil {
		return domain.Order{}, err
	}
	if tag.RowsAffected() == 0 {
		return domain.Order{}, domain.ErrConflict
	}
	if err := addEvent(ctx, tx, o.ID, "status_change", string(ev), string(from), string(o.Status), actor, reason); err != nil {
		return domain.Order{}, err
	}
	return getOne(ctx, tx, o.ID, false)
}

func (r *Repository) SaveEdit(ctx context.Context, before, after domain.Order, diff []string, actor string, ev domain.Event, opts ports.TransitionOpts) (domain.Order, error) {
	var out domain.Order
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE orders SET courier=NULLIF($2,''), courier_code=NULLIF($3,''), recipient_name=NULLIF($4,''),
				recipient_phone=NULLIF($5,''), recipient_email=NULLIF($6,''), address_line=NULLIF($7,''),
				commune=NULLIF($8,''), region_code=NULLIF($9,''), dispatch_type=$10, customer_notes=NULLIF($11,''),
				prev_status=NULLIF($12,''), validation_attempts=$13, rules_checked_at=NULL,
				version=version+1, updated_at=now()
			WHERE id=$1 AND version=$14`,
			before.ID, after.Courier, after.CourierCode, after.RecipientName, after.RecipientPhone, after.RecipientEmail,
			after.AddressLine, after.Commune, after.RegionCode, after.DispatchType, after.CustomerNotes,
			string(after.PrevStatus), after.ValidationAttempts, before.Version)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		if err := replaceItems(ctx, tx, before.ID, after.Items); err != nil {
			return err
		}
		if err := addEvent(ctx, tx, before.ID, "edit", "edit", "", "", actor, strings.Join(diff, "; ")); err != nil {
			return err
		}
		if opts.ReleaseCommitted {
			if err := releaseCommitted(ctx, tx, before); err != nil {
				return err
			}
		}
		if ev != "" {
			after.Version = before.Version + 1
			if out, err = transitionTx(ctx, tx, after, before.Status, ev, actor, "Edición: se valida de nuevo", opts.ClearWave); err != nil {
				return err
			}
			return nil
		}
		out, err = getOne(ctx, tx, before.ID, false)
		return err
	})
	return out, err
}

func (r *Repository) SetPriority(ctx context.Context, o domain.Order, priority bool, actor, reason string) (domain.Order, error) {
	var out domain.Order
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `UPDATE orders SET priority=$1, version=version+1, updated_at=now() WHERE id=$2 AND version=$3`,
			priority, o.ID, o.Version)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return domain.ErrConflict
		}
		// Si ya tiene trabajos encolados, suben de prioridad también
		if _, err := tx.Exec(ctx, `UPDATE job_outbox SET priority=$1 WHERE order_id=$2 AND published_at IS NULL`, priority, o.ID); err != nil {
			return err
		}
		ev := "unprioritize"
		if priority {
			ev = "prioritize"
		}
		if err := addEvent(ctx, tx, o.ID, "priority", ev, "", "", actor, reason); err != nil {
			return err
		}
		out, err = getOne(ctx, tx, o.ID, false)
		return err
	})
	return out, err
}

func (r *Repository) AssignWave(ctx context.Context, ids []int64, orderIDs []int64, waveID, actor, reason string) ([]int64, error) {
	assigned := []int64{}
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE orders SET wave_id=$1, version=version+1, updated_at=now()
			WHERE id = ANY($2) AND status='validated' AND wave_id IS NULL
			  AND ($3::bigint[] IS NULL OR company_id = ANY($3))
			RETURNING id, company_id, priority`, waveID, orderIDs, ids)
		if err != nil {
			return err
		}
		type row struct {
			id, company int64
			prio        bool
		}
		var list []row
		for rows.Next() {
			var x row
			if err := rows.Scan(&x.id, &x.company, &x.prio); err != nil {
				rows.Close()
				return err
			}
			list = append(list, x)
		}
		rows.Close()
		for _, x := range list {
			assigned = append(assigned, x.id)
			if err := addEvent(ctx, tx, x.id, "wave_requested", "assign_wave", "", "", actor, reason); err != nil {
				return err
			}
			if err := insertJobs(ctx, tx, []ports.Job{{
				ID: fmt.Sprintf("assign_wave:%d:%s", x.id, waveID), Type: "assign_wave",
				CompanyID: x.company, OrderID: x.id, Priority: x.prio, Payload: map[string]any{"wave_id": waveID},
			}}); err != nil {
				return err
			}
		}
		return nil
	})
	return assigned, err
}

func (r *Repository) RetryFailedJobs(ctx context.Context, orderID int64) (int, error) {
	tag, err := r.db.Exec(ctx, `
		UPDATE job_outbox SET failed_at=NULL, published_at=NULL, attempts=0, last_error=NULL
		WHERE order_id=$1 AND failed_at IS NOT NULL`, orderID)
	return int(tag.RowsAffected()), err
}

// ---------------- Validación automática ----------------

func (r *Repository) ClaimDueValidations(ctx context.Context, limit int, lease time.Duration) ([]domain.Order, error) {
	var out []domain.Order
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `
			UPDATE orders o SET next_retry_at = now() + $2::interval
			WHERE o.id IN (
				SELECT id FROM orders
				WHERE status IN ('pending','validating') AND next_retry_at <= now()
				ORDER BY priority DESC, id
				LIMIT $1
				FOR UPDATE SKIP LOCKED)
			RETURNING o.id`, limit, fmt.Sprintf("%d seconds", int(lease.Seconds())))
		if err != nil {
			return err
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			o, err := getOne(ctx, tx, id, false)
			if err != nil {
				return err
			}
			out = append(out, o)
		}
		return nil
	})
	return out, err
}

func (r *Repository) ScheduleValidationRetry(ctx context.Context, orderID int64, attempts int, next time.Time, lastErr string) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET validation_attempts=$2, next_retry_at=$3, last_error=$4, updated_at=now() WHERE id=$1`,
		orderID, attempts, next, lastErr)
	return err
}

func (r *Repository) FinishValidation(ctx context.Context, orderID int64, invasStock map[string]int) (domain.Order, error) {
	var out domain.Order
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		o, err := getOne(ctx, tx, orderID, true)
		if err != nil {
			return err
		}
		if o.Status != domain.StatusValidating {
			out = o
			return nil // otra operación ya la movió
		}
		// Bloquea las filas de comprometido de estos SKU (orden fijo -> sin deadlocks)
		items := sortedItems(o.Items)
		for _, it := range items {
			if _, err := tx.Exec(ctx, `INSERT INTO stock_committed (company_id, sku, quantity) VALUES ($1,$2,0)
				ON CONFLICT DO NOTHING`, o.CompanyID, it.SKU); err != nil {
				return err
			}
		}
		ok := true
		var shortage string
		for _, it := range items {
			var committed int
			if err := tx.QueryRow(ctx, `SELECT quantity FROM stock_committed WHERE company_id=$1 AND sku=$2 FOR UPDATE`,
				o.CompanyID, it.SKU).Scan(&committed); err != nil {
				return err
			}
			if invasStock[it.SKU]-committed < it.Quantity {
				ok = false
				shortage = fmt.Sprintf("Sin stock para %s: disponible %d (Invas %d − comprometido %d) < pedido %d",
					it.SKU, max(invasStock[it.SKU]-committed, 0), invasStock[it.SKU], committed, it.Quantity)
				break
			}
		}
		ev, reason := domain.EvVoid, shortage
		if ok {
			ev, reason = domain.EvValidate, ""
			for _, it := range o.Items {
				if _, err := tx.Exec(ctx, `UPDATE stock_committed SET quantity=quantity+$3, updated_at=now()
					WHERE company_id=$1 AND sku=$2`, o.CompanyID, it.SKU, it.Quantity); err != nil {
					return err
				}
			}
		}
		from, err := o.Apply(ev)
		if err != nil {
			return err
		}
		o.ValidationAttempts = 0
		out, err = transitionTx(ctx, tx, o, from, ev, "system:validador", reason, false)
		return err
	})
	return out, err
}

// CompleteWaveAssignment: Invas aceptó la ola -> descuenta su stock, así que
// Nexus resta el comprometido y la orden pasa a En preparación (DAS §4.3).
func (r *Repository) CompleteWaveAssignment(ctx context.Context, orderID int64, wmsRef string) (domain.Order, error) {
	var out domain.Order
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		o, err := getOne(ctx, tx, orderID, true)
		if err != nil {
			return err
		}
		if o.Status != domain.StatusValidated {
			out = o
			return nil // idempotente: un mensaje repetido no la mueve dos veces
		}
		if err := releaseCommitted(ctx, tx, o); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `UPDATE orders SET wms_ref=$2 WHERE id=$1`, o.ID, wmsRef); err != nil {
			return err
		}
		from, err := o.Apply(domain.EvWaveConfirmed)
		if err != nil {
			return err
		}
		out, err = transitionTx(ctx, tx, o, from, domain.EvWaveConfirmed, "system:invas", "OS "+wmsRef, false)
		return err
	})
	return out, err
}

func (r *Repository) OrdersInStatus(ctx context.Context, status domain.Status, limit int) ([]domain.Order, error) {
	rows, err := r.db.Query(ctx, `SELECT `+cols+` FROM `+fromOrders+` WHERE o.status=$1 ORDER BY o.priority DESC, o.updated_at LIMIT $2`, string(status), limit)
	if err != nil {
		return nil, err
	}
	var out []domain.Order
	for rows.Next() {
		o, err := scan(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, o)
	}
	rows.Close()
	return out, loadItemsBulk(ctx, r.db, out)
}

func (r *Repository) RuleCandidates(ctx context.Context, limit int) ([]domain.Order, error) {
	rows, err := r.db.Query(ctx, `SELECT `+cols+` FROM `+fromOrders+`
		WHERE o.status='validated' AND o.wave_id IS NULL AND o.rules_checked_at IS NULL
		ORDER BY o.priority DESC, o.id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	var out []domain.Order
	for rows.Next() {
		o, err := scan(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		out = append(out, o)
	}
	rows.Close()
	return out, rows.Err()
}

func (r *Repository) MarkRulesChecked(ctx context.Context, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	_, err := r.db.Exec(ctx, `UPDATE orders SET rules_checked_at=now() WHERE id = ANY($1)`, ids)
	return err
}

func releaseCommitted(ctx context.Context, q querier, o domain.Order) error {
	if len(o.Items) == 0 {
		if err := loadItems(ctx, q, &o); err != nil {
			return err
		}
	}
	for _, it := range o.Items {
		if _, err := q.Exec(ctx, `UPDATE stock_committed SET quantity=GREATEST(quantity-$3,0), updated_at=now()
			WHERE company_id=$1 AND sku=$2`, o.CompanyID, it.SKU, it.Quantity); err != nil {
			return err
		}
	}
	return nil
}

func sortedItems(items []domain.Item) []domain.Item {
	out := append([]domain.Item(nil), items...)
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].SKU < out[j-1].SKU; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
