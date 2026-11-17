package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexuswst/orders-service/internal/ports"
)

// Registro histórico (Tarea 06, cambio de Jean; ver backend_pendientes «Cambios
// de backend hechos por Jean»).

// lastStatusChange - fecha del último cambio de estado de la orden o. Para las
// Entregadas manda handed_over_at; sin eventos, la fecha de creación.
const lastStatusChange = `COALESCE(CASE WHEN o.status = 'handed_over' THEN o.handed_over_at END,
	(SELECT max(e.occurred_at) FROM order_events e
	  WHERE e.order_id = o.id AND e.to_status IS NOT NULL AND e.to_status IS DISTINCT FROM e.from_status),
	o.created_at)`

// inRegistry - qué órdenes van al registro: las Canceladas de inmediato y
// cualquier otra cuando pasa 1 año desde su último cambio de estado. La bandeja
// (List) usa NOT inRegistry, así ninguna orden queda en los dos lados ni en ninguno.
const inRegistry = `(o.status = 'cancelled' OR ` + lastStatusChange + ` < now() - interval '1 year')`

type RegistryRepo struct{ db *pgxpool.Pool }

func NewRegistry(db *pgxpool.Pool) *RegistryRepo { return &RegistryRepo{db: db} }

var _ ports.RegistryReader = (*RegistryRepo)(nil)

func (r *RegistryRepo) Registry(ctx context.Context, f ports.RegistryFilter) ([]ports.RegistryEntry, int, error) {
	where := []string{inRegistry, "($1::bigint[] IS NULL OR o.company_id = ANY($1))"}
	args := []any{f.CompanyIDs}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		add(`(o.id::text = ? OR o.external_ref ILIKE '%' || ? || '%')`, q)
	}
	if len(f.Statuses) > 0 {
		add("o.status = ANY(?::text[])", f.Statuses)
	}
	if f.Courier != "" {
		add("o.courier = ?", strings.ToLower(f.Courier))
	}
	if len(f.IDs) > 0 {
		add("o.id = ANY(?::bigint[])", f.IDs)
	}
	if f.From != nil {
		add(lastStatusChange+" >= ?", *f.From)
	}
	if f.To != nil {
		add(lastStatusChange+" < ?", *f.To)
	}
	w := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM orders o WHERE `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, `
		SELECT o.id, o.external_ref, o.company_id, COALESCE(co.name, ''), COALESCE(o.courier, ''), COALESCE(cr.name, ''),
		       o.status, COALESCE(ce.from_status, ''), `+lastStatusChange+` AS closed_at,
		       COALESCE(ce.actor, ''), COALESCE(u.role, ''), COALESCE(NULLIF(ce.reason, ''), o.note, ''), o.status <> 'cancelled'
		FROM orders o
		LEFT JOIN companies co ON co.id = o.company_id
		LEFT JOIN couriers cr ON cr.code = o.courier
		LEFT JOIN LATERAL (
			SELECT e.actor, e.reason, e.from_status FROM order_events e
			WHERE e.order_id = o.id AND e.to_status IS NOT NULL AND e.to_status IS DISTINCT FROM e.from_status
			ORDER BY e.occurred_at DESC, e.id DESC LIMIT 1) ce ON true
		LEFT JOIN users u ON u.id = substring(ce.actor from '^user:([0-9]+)')::bigint
		WHERE `+w+`
		ORDER BY closed_at DESC, o.id DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []ports.RegistryEntry{}
	for rows.Next() {
		var e ports.RegistryEntry
		if err := rows.Scan(&e.OrderID, &e.ExternalRef, &e.CompanyID, &e.CompanyName, &e.Courier, &e.CourierName,
			&e.Status, &e.PreviousStatus, &e.ClosedAt, &e.Actor, &e.ActorRole, &e.Reason, &e.Stale); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}
