package postgres

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CatalogReader lee tablas del Core en SOLO LECTURA (DA-12): products,
// companies, couriers, communes y users. Órdenes no llama al Core ni usa su
// caché de Redis; así sigue operando si el Core está caído.
type CatalogReader struct {
	db *pgxpool.Pool

	mu         sync.Mutex
	communes   []string
	communesAt time.Time
}

func NewCatalogReader(db *pgxpool.Pool) *CatalogReader { return &CatalogReader{db: db} }

func (p *CatalogReader) MissingSKUs(ctx context.Context, companyID int64, skus []string) ([]string, error) {
	rows, err := p.db.Query(ctx, `
		SELECT s FROM unnest($2::text[]) AS s
		WHERE NOT EXISTS (SELECT 1 FROM products WHERE company_id=$1 AND sku=s AND active)`, companyID, skus)
	if err != nil {
		return nil, err
	}
	return collect[string](rows)
}

func (p *CatalogReader) KitSKUs(ctx context.Context, companyID int64, skus []string) ([]string, error) {
	rows, err := p.db.Query(ctx, `SELECT sku FROM products WHERE company_id=$1 AND sku = ANY($2) AND type='kit'`, companyID, skus)
	if err != nil {
		return nil, err
	}
	return collect[string](rows)
}

func (p *CatalogReader) CompanyActive(ctx context.Context, companyID int64) (bool, error) {
	var ok bool
	err := p.db.QueryRow(ctx, `SELECT active FROM companies WHERE id=$1`, companyID).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

func (p *CatalogReader) CompanyName(ctx context.Context, companyID int64) (string, error) {
	var name string
	err := p.db.QueryRow(ctx, `SELECT name FROM companies WHERE id=$1`, companyID).Scan(&name)
	return name, err
}

func (p *CatalogReader) CourierActive(ctx context.Context, code string) (bool, error) {
	var ok bool
	err := p.db.QueryRow(ctx, `SELECT active FROM couriers WHERE code=$1`, code).Scan(&ok)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	return ok, err
}

// Communes: catálogo poco cambiante; se guarda 10 minutos en memoria.
func (p *CatalogReader) Communes(ctx context.Context) ([]string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.communes != nil && time.Since(p.communesAt) < 10*time.Minute {
		return p.communes, nil
	}
	rows, err := p.db.Query(ctx, `SELECT name FROM communes WHERE active ORDER BY name`)
	if err != nil {
		return nil, err
	}
	list, err := collect[string](rows)
	if err != nil {
		return nil, err
	}
	p.communes, p.communesAt = list, time.Now()
	return list, nil
}

// ClientEmails - usuarios cliente activos de la empresa (avisos por correo).
func (p *CatalogReader) ClientEmails(ctx context.Context, companyID int64) ([]string, error) {
	rows, err := p.db.Query(ctx, `
		SELECT u.email::text FROM users u JOIN user_companies uc ON uc.user_id=u.id
		WHERE uc.company_id=$1 AND u.role='cliente' AND u.active`, companyID)
	if err != nil {
		return nil, err
	}
	return collect[string](rows)
}

func collect[T any](rows pgx.Rows) ([]T, error) {
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var v T
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
