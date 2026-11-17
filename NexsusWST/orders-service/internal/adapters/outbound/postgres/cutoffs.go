package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

func (r *RuleRepository) Cutoffs(ctx context.Context) ([]ports.Cutoff, error) {
	rows, err := r.db.Query(ctx, `SELECT c.id,row_number() OVER(ORDER BY c.cutoff_time,c.id),c.name,to_char(c.cutoff_time,'HH24:MI'),COALESCE((SELECT json_agg(json_build_object('code',cc.courier,'name',co.name,'arrivalTime',to_char(cc.arrival_time,'HH24:MI')) ORDER BY co.name,cc.courier) FROM cutoff_couriers cc JOIN couriers co ON co.code=cc.courier WHERE cc.cutoff_id=c.id),'[]') FROM cutoffs c WHERE c.active ORDER BY c.cutoff_time,c.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ports.Cutoff{}
	for rows.Next() {
		var c ports.Cutoff
		var raw []byte
		if err := rows.Scan(&c.ID, &c.Number, &c.Name, &c.CutoffTime, &raw); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(raw, &c.Couriers); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (r *RuleRepository) SaveCutoff(ctx context.Context, c ports.Cutoff, actor string) (ports.Cutoff, error) {
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		var deadline string
		if err := tx.QueryRow(ctx, `SELECT value FROM ops_settings WHERE key='assignment_deadline' FOR UPDATE`).Scan(&deadline); err != nil {
			return err
		}
		if c.CutoffTime > deadline {
			return domain.ErrInvalid
		}
		if c.ID == 0 {
			if err := tx.QueryRow(ctx, `INSERT INTO cutoffs(name,cutoff_time,updated_by) VALUES($1,$2::time,$3) RETURNING id`, c.Name, c.CutoffTime, actor).Scan(&c.ID); err != nil {
				return err
			}
		} else {
			tag, err := tx.Exec(ctx, `UPDATE cutoffs SET name=$2,cutoff_time=$3::time,updated_by=$4,updated_at=now() WHERE id=$1 AND active`, c.ID, c.Name, c.CutoffTime, actor)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return domain.ErrNotFound
			}
		}
		if _, err := tx.Exec(ctx, `DELETE FROM cutoff_couriers WHERE cutoff_id=$1`, c.ID); err != nil {
			return err
		}
		for _, v := range c.Couriers {
			tag, err := tx.Exec(ctx, `INSERT INTO cutoff_couriers(cutoff_id,courier,arrival_time) SELECT $1,code,$3::time FROM couriers WHERE code=$2 AND active`, c.ID, v.Code, v.ArrivalTime)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 0 {
				return domain.ErrInvalid
			}
		}
		return nil
	})
	var pe *pgconn.PgError
	if errors.As(err, &pe) && pe.Code == "23505" {
		return c, domain.ErrCutoffTimeTaken
	}
	if err != nil {
		return c, err
	}
	all, err := r.Cutoffs(ctx)
	for _, v := range all {
		if v.ID == c.ID {
			return v, err
		}
	}
	return c, err
}
func (r *RuleRepository) DeleteCutoff(ctx context.Context, id int64, actor string) error {
	tag, err := r.db.Exec(ctx, `UPDATE cutoffs SET active=false,updated_by=$2,updated_at=now() WHERE id=$1 AND active`, id, actor)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return err
}
