package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

type RuleRepository struct{ db *pgxpool.Pool }

func NewRuleRepository(db *pgxpool.Pool) *RuleRepository { return &RuleRepository{db: db} }

var _ ports.RuleRepository = (*RuleRepository)(nil)

const ruleCols = `id, name, position, active, COALESCE(courier,''), company_id, COALESCE(channel,''),
	COALESCE(dispatch_type,''), COALESCE(warehouse,''), wave_code, updated_by, updated_at`

func scanRule(row pgx.Row) (ports.WaveRule, error) {
	var r ports.WaveRule
	err := row.Scan(&r.ID, &r.Name, &r.Position, &r.Active, &r.Courier, &r.CompanyID, &r.Channel,
		&r.DispatchType, &r.Warehouse, &r.WaveCode, &r.UpdatedBy, &r.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, domain.ErrNotFound
	}
	return r, err
}

func (r *RuleRepository) List(ctx context.Context, onlyActive bool) ([]ports.WaveRule, error) {
	rows, err := r.db.Query(ctx, `SELECT `+ruleCols+` FROM wave_rules WHERE (NOT $1 OR active) ORDER BY position, id`, onlyActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ports.WaveRule{}
	for rows.Next() {
		x, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, x)
	}
	return out, rows.Err()
}

func (r *RuleRepository) Get(ctx context.Context, id int64) (ports.WaveRule, error) {
	return scanRule(r.db.QueryRow(ctx, `SELECT `+ruleCols+` FROM wave_rules WHERE id=$1`, id))
}

func (r *RuleRepository) Save(ctx context.Context, x ports.WaveRule, actor string) (ports.WaveRule, error) {
	if x.ID == 0 {
		return scanRule(r.db.QueryRow(ctx, `
			INSERT INTO wave_rules (name, position, active, courier, company_id, channel, dispatch_type, warehouse, wave_code, created_by, updated_by)
			VALUES ($1, COALESCE(NULLIF($2,0), (SELECT COALESCE(max(position),0)+1 FROM wave_rules)), $3, NULLIF($4,''), $5,
				NULLIF($6,''), NULLIF($7,''), NULLIF($8,''), $9, $10, $10)
			RETURNING `+ruleCols, x.Name, x.Position, x.Active, x.Courier, x.CompanyID, x.Channel, x.DispatchType, x.Warehouse, x.WaveCode, actor))
	}
	return scanRule(r.db.QueryRow(ctx, `
		UPDATE wave_rules SET name=$2, position=COALESCE(NULLIF($3,0), position), active=$4, courier=NULLIF($5,''), company_id=$6,
			channel=NULLIF($7,''), dispatch_type=NULLIF($8,''), warehouse=NULLIF($9,''), wave_code=$10, updated_by=$11, updated_at=now()
		WHERE id=$1 RETURNING `+ruleCols, x.ID, x.Name, x.Position, x.Active, x.Courier, x.CompanyID, x.Channel, x.DispatchType, x.Warehouse, x.WaveCode, actor))
}

func (r *RuleRepository) Delete(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM wave_rules WHERE id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}

// Reorder fija la posición según el orden recibido (1, 2, 3...).
func (r *RuleRepository) Reorder(ctx context.Context, ids []int64, actor string) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		for i, id := range ids {
			tag, err := tx.Exec(ctx, `UPDATE wave_rules SET position=$2, updated_by=$3, updated_at=now() WHERE id=$1`, id, i+1, actor)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return domain.ErrNotFound
			}
		}
		return nil
	})
}

func (r *RuleRepository) ResetChecks(ctx context.Context) error {
	_, err := r.db.Exec(ctx, `UPDATE orders SET rules_checked_at=NULL WHERE status='validated' AND wave_id IS NULL AND rules_checked_at IS NOT NULL`)
	return err
}

// CourierSettings - sin configuración, el courier cubre todo y no limita bultos.
func (r *RuleRepository) CourierSettings(ctx context.Context, courier string) (ports.CourierSettings, error) {
	cs := ports.CourierSettings{Courier: courier, CoversAll: true, Communes: []string{}}
	err := r.db.QueryRow(ctx, `SELECT COALESCE(to_char(arrival_time,'HH24:MI'),''), max_packages, covers_all FROM courier_settings WHERE courier=$1`, courier).
		Scan(&cs.ArrivalTime, &cs.MaxPackages, &cs.CoversAll)
	if errors.Is(err, pgx.ErrNoRows) {
		return cs, nil
	}
	if err != nil {
		return cs, err
	}
	if !cs.CoversAll {
		rows, err := r.db.Query(ctx, `SELECT commune FROM courier_coverage WHERE courier=$1 ORDER BY commune`, courier)
		if err != nil {
			return cs, err
		}
		if cs.Communes, err = collect[string](rows); err != nil {
			return cs, err
		}
	}
	return cs, nil
}

func (r *RuleRepository) ListCourierSettings(ctx context.Context) ([]ports.CourierSettings, error) {
	rows, err := r.db.Query(ctx, `SELECT c.code FROM couriers c WHERE c.active ORDER BY c.name`)
	if err != nil {
		return nil, err
	}
	codes, err := collect[string](rows)
	if err != nil {
		return nil, err
	}
	out := make([]ports.CourierSettings, 0, len(codes))
	for _, c := range codes {
		cs, err := r.CourierSettings(ctx, c)
		if err != nil {
			return nil, err
		}
		out = append(out, cs)
	}
	return out, nil
}

func (r *RuleRepository) SaveCourierSettings(ctx context.Context, s ports.CourierSettings, actor string) error {
	return pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `
			INSERT INTO courier_settings (courier, arrival_time, max_packages, covers_all, updated_by, updated_at)
			VALUES ($1, NULLIF($2,'')::time, $3, $4, $5, now())
			ON CONFLICT (courier) DO UPDATE SET arrival_time=EXCLUDED.arrival_time, max_packages=EXCLUDED.max_packages,
				covers_all=EXCLUDED.covers_all, updated_by=EXCLUDED.updated_by, updated_at=now()`,
			s.Courier, s.ArrivalTime, s.MaxPackages, s.CoversAll, actor); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM courier_coverage WHERE courier=$1`, s.Courier); err != nil {
			return err
		}
		for _, c := range s.Communes {
			if _, err := tx.Exec(ctx, `INSERT INTO courier_coverage (courier, commune) VALUES ($1,$2)`, s.Courier, c); err != nil {
				return err
			}
		}
		return nil
	})
}
