package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/nexuswst/orders-service/internal/ports"
)

// Registro de acciones (Tarea 06, Fase 2; cambio de Jean). Lee la vista activity_log
// (migración 000009); no escribe nada.
type ActivityRepo struct{ db querier }

// NewActivity recibe el pool (o una transacción en las pruebas).
func NewActivity(db querier) *ActivityRepo { return &ActivityRepo{db: db} }

var _ ports.ActivityReader = (*ActivityRepo)(nil)

func (r *ActivityRepo) Activity(ctx context.Context, f ports.ActivityFilter) ([]ports.ActivityEntry, int, error) {
	where := []string{"a.occurred_at >= $1", "($2::bigint[] IS NULL OR a.company_id IS NULL OR a.company_id = ANY($2))"}
	args := []any{f.Since, f.CompanyIDs}
	add := func(cond string, v any) {
		args = append(args, v)
		where = append(where, strings.ReplaceAll(cond, "?", fmt.Sprintf("$%d", len(args))))
	}
	if f.CompanyID != nil {
		add("a.company_id = ?", *f.CompanyID)
	}
	if f.Module != "" {
		add("a.module = ?", f.Module)
	}
	if a := strings.TrimSpace(f.Actor); a != "" {
		if id, err := strconv.ParseInt(a, 10, 64); err == nil {
			add("a.actor_id = ?", id)
		} else {
			add("a.actor_name ILIKE '%' || ? || '%'", a)
		}
	}
	if f.Role != "" {
		add("a.actor_role = ?", f.Role)
	}
	if f.Kind != "" {
		add("a.actor_kind = ?", f.Kind)
	}
	if q := strings.TrimSpace(f.Search); strings.HasPrefix(q, "#") {
		// «#12»: el manifiesto 12 (creación, cierre y las órdenes despachadas igual en él).
		add("(a.entity_label = ? OR a.details->>'manifestId' = substr(?, 2))", q)
	} else if q != "" {
		add("(a.entity_id = ? OR a.entity_label ILIKE '%' || ? || '%')", q)
	}
	if f.From != nil {
		add("a.occurred_at >= ?", *f.From)
	}
	if f.To != nil {
		add("a.occurred_at < ?", *f.To)
	}
	w := strings.Join(where, " AND ")

	var total int
	if err := r.db.QueryRow(ctx, `SELECT count(*) FROM activity_log a WHERE `+w, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	args = append(args, f.Limit, f.Offset)
	rows, err := r.db.Query(ctx, `
		SELECT a.id, a.occurred_at, a.actor_id, a.actor_name, COALESCE(a.actor_role, ''), a.actor_kind, a.module, a.action,
		       a.entity_type, a.entity_id, COALESCE(a.entity_label, ''), a.company_id, COALESCE(c.name, ''), COALESCE(cr.name, ''),
		       COALESCE(a.warehouse, ''), COALESCE(a.reason, ''), COALESCE(a.details, '{}'::jsonb)
		FROM activity_log a
		LEFT JOIN companies c ON c.id = a.company_id
		LEFT JOIN couriers cr ON cr.code = a.details->>'courier'
		WHERE `+w+`
		ORDER BY a.occurred_at DESC, a.id DESC
		LIMIT $`+fmt.Sprint(len(args)-1)+` OFFSET $`+fmt.Sprint(len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := []ports.ActivityEntry{}
	for rows.Next() {
		var e ports.ActivityEntry
		var details []byte
		if err := rows.Scan(&e.ID, &e.OccurredAt, &e.ActorID, &e.ActorName, &e.ActorRole, &e.ActorKind, &e.Module, &e.Action,
			&e.EntityType, &e.EntityID, &e.EntityLabel, &e.CompanyID, &e.CompanyName, &e.CourierName, &e.Warehouse, &e.Reason, &details); err != nil {
			return nil, 0, err
		}
		if err := json.Unmarshal(details, &e.Details); err != nil {
			return nil, 0, err
		}
		out = append(out, e)
	}
	return out, total, rows.Err()
}
