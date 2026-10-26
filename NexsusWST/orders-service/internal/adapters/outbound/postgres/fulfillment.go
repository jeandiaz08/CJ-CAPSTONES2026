package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

// FulfillmentRepository - bultos, documentos, cola de empaque, manifiestos y
// cargas masivas (RF-09, RF-18 a RF-24).
type FulfillmentRepository struct{ db *pgxpool.Pool }

func NewFulfillmentRepository(db *pgxpool.Pool) *FulfillmentRepository {
	return &FulfillmentRepository{db: db}
}

var _ ports.FulfillmentRepository = (*FulfillmentRepository)(nil)

// ---------------- Bultos ----------------

func (r *FulfillmentRepository) Packages(ctx context.Context, orderID int64) ([]domain.Package, error) {
	rows, err := r.db.Query(ctx, `SELECT id, code, weight_kg::float8, length_cm::float8, width_cm::float8, height_cm::float8
		FROM order_packages WHERE order_id=$1 ORDER BY id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []domain.Package{}
	for rows.Next() {
		var p domain.Package
		if err := rows.Scan(&p.ID, &p.Code, &p.WeightKg, &p.LengthCm, &p.WidthCm, &p.HeightCm); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// ReplacePackages vuelve a registrar los bultos; el código es NX-<orden>-<n>.
func (r *FulfillmentRepository) ReplacePackages(ctx context.Context, orderID int64, p []domain.Package, actor string, userID int64) ([]domain.Package, error) {
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `DELETE FROM order_packages WHERE order_id=$1`, orderID); err != nil {
			return err
		}
		for i, x := range p {
			if _, err := tx.Exec(ctx, `INSERT INTO order_packages (order_id, code, weight_kg, length_cm, width_cm, height_cm, created_by)
				VALUES ($1,$2,$3,$4,$5,$6,$7)`, orderID, fmt.Sprintf("NX-%d-%d", orderID, i+1), x.WeightKg, x.LengthCm, x.WidthCm, x.HeightCm, userID); err != nil {
				return err
			}
		}
		return addEvent(ctx, tx, orderID, "packages", "register", "", "", actor, fmt.Sprintf("%d bulto(s) registrados", len(p)))
	})
	if err != nil {
		return nil, err
	}
	return r.Packages(ctx, orderID)
}

// PackingQueue: Preparadas + «En empaque» tomadas por el usuario o por
// alguien que hoy está deshabilitado (vuelven a la bandeja).
func (r *FulfillmentRepository) PackingQueue(ctx context.Context, ids []int64, userID int64) ([]domain.Order, error) {
	rows, err := r.db.Query(ctx, `SELECT `+cols+` FROM `+fromOrders+`
		WHERE ($1::bigint[] IS NULL OR o.company_id = ANY($1))
		  AND (o.status='picked' OR (o.status='packing' AND (o.claimed_by=$2 OR cu.id IS NULL)))
		ORDER BY o.status DESC, o.priority DESC, o.updated_at LIMIT 300`, ids, userID)
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
	if out == nil {
		out = []domain.Order{}
	}
	return out, loadItemsBulk(ctx, r.db, out)
}

// ---------------- Documentos ----------------

const docCols = `id, order_id, company_id, kind, filename, content_type, size_bytes, sha256, storage_key, uploaded_by, uploaded_at`

func scanDoc(row pgx.Row) (ports.Document, error) {
	var d ports.Document
	err := row.Scan(&d.ID, &d.OrderID, &d.CompanyID, &d.Kind, &d.Filename, &d.ContentType, &d.SizeBytes, &d.SHA256, &d.StorageKey, &d.UploadedBy, &d.UploadedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, domain.ErrNotFound
	}
	return d, err
}

func (r *FulfillmentRepository) AddDocument(ctx context.Context, d ports.Document) (ports.Document, error) {
	return scanDoc(r.db.QueryRow(ctx, `INSERT INTO order_documents (order_id, company_id, kind, filename, content_type, size_bytes, sha256, storage_key, uploaded_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+docCols,
		d.OrderID, d.CompanyID, d.Kind, d.Filename, d.ContentType, d.SizeBytes, d.SHA256, d.StorageKey, d.UploadedBy))
}

func (r *FulfillmentRepository) Documents(ctx context.Context, orderID int64) ([]ports.Document, error) {
	rows, err := r.db.Query(ctx, `SELECT `+docCols+` FROM order_documents WHERE order_id=$1 ORDER BY id`, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ports.Document{}
	for rows.Next() {
		d, err := scanDoc(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func (r *FulfillmentRepository) Document(ctx context.Context, orderID, docID int64) (ports.Document, error) {
	return scanDoc(r.db.QueryRow(ctx, `SELECT `+docCols+` FROM order_documents WHERE order_id=$1 AND id=$2`, orderID, docID))
}

// ---------------- Retiro y manifiestos ----------------

func (r *FulfillmentRepository) ReadyByCourier(ctx context.Context, ids []int64) ([]ports.CourierQueue, error) {
	rows, err := r.db.Query(ctx, `
		SELECT COALESCE(o.courier,'(sin courier)'), count(DISTINCT o.id), count(p.id)
		FROM orders o LEFT JOIN order_packages p ON p.order_id=o.id
		WHERE o.status='ready_pickup' AND ($1::bigint[] IS NULL OR o.company_id = ANY($1))
		GROUP BY 1 ORDER BY 1`, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ports.CourierQueue{}
	for rows.Next() {
		var q ports.CourierQueue
		if err := rows.Scan(&q.Courier, &q.Orders, &q.Packages); err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

func (r *FulfillmentRepository) CreateManifest(ctx context.Context, courier string, orderIDs []int64, actor string) (ports.Manifest, error) {
	var id int64
	err := pgx.BeginFunc(ctx, r.db, func(tx pgx.Tx) error {
		if err := tx.QueryRow(ctx, `INSERT INTO manifests (courier, created_by) VALUES ($1,$2) RETURNING id`, courier, actor).Scan(&id); err != nil {
			return err
		}
		added := 0
		for _, oid := range orderIDs {
			// Una orden en otro manifiesto abierto no se repite.
			tag, err := tx.Exec(ctx, `INSERT INTO manifest_lines (manifest_id, order_id) SELECT $1, $2
				WHERE NOT EXISTS (SELECT 1 FROM manifest_lines WHERE order_id=$2 AND status='included')`, id, oid)
			if err != nil {
				return err
			}
			if tag.RowsAffected() == 1 {
				added++
				if err := addEvent(ctx, tx, oid, "manifest", "add", "", "", actor, fmt.Sprintf("Agregada al manifiesto #%d", id)); err != nil {
					return err
				}
			}
		}
		if added == 0 {
			return fmt.Errorf("%w: todas las órdenes ya están en un manifiesto abierto", domain.ErrConflict)
		}
		return nil
	})
	if err != nil {
		return ports.Manifest{}, err
	}
	return r.Manifest(ctx, id)
}

func (r *FulfillmentRepository) Manifest(ctx context.Context, id int64) (ports.Manifest, error) {
	var m ports.Manifest
	err := r.db.QueryRow(ctx, `SELECT id, courier, status, created_by, created_at, COALESCE(closed_by,''), closed_at,
		COALESCE(picker_name,''), COALESCE(picker_id,''), COALESCE(vehicle_plate,''), COALESCE(notes,'')
		FROM manifests WHERE id=$1`, id).Scan(&m.ID, &m.Courier, &m.Status, &m.CreatedBy, &m.CreatedAt, &m.ClosedBy, &m.ClosedAt,
		&m.PickerName, &m.PickerID, &m.VehiclePlate, &m.Notes)
	if errors.Is(err, pgx.ErrNoRows) {
		return m, domain.ErrNotFound
	}
	if err != nil {
		return m, err
	}
	rows, err := r.db.Query(ctx, `
		SELECT l.order_id, o.company_id, o.external_ref, COALESCE(o.recipient_name,''), COALESCE(o.address_line,''),
		       COALESCE(o.commune,''), l.status, COALESCE(l.moved_to,'')
		FROM manifest_lines l JOIN orders o ON o.id=l.order_id WHERE l.manifest_id=$1 ORDER BY l.order_id`, id)
	if err != nil {
		return m, err
	}
	m.Lines = []ports.ManifestLine{}
	for rows.Next() {
		var l ports.ManifestLine
		if err := rows.Scan(&l.OrderID, &l.CompanyID, &l.ExternalRef, &l.RecipientName, &l.AddressLine, &l.Commune, &l.Status, &l.MovedTo); err != nil {
			rows.Close()
			return m, err
		}
		m.Lines = append(m.Lines, l)
	}
	rows.Close()
	for i := range m.Lines {
		if m.Lines[i].Packages, err = r.bundles(ctx, id, m.Lines[i].OrderID); err != nil {
			return m, err
		}
	}
	return m, nil
}

func (r *FulfillmentRepository) bundles(ctx context.Context, manifestID, orderID int64) ([]ports.ManifestBundle, error) {
	rows, err := r.db.Query(ctx, `
		SELECT p.id, p.code, EXISTS (SELECT 1 FROM manifest_scans s WHERE s.manifest_id=$1 AND s.package_id=p.id)
		FROM order_packages p WHERE p.order_id=$2 ORDER BY p.id`, manifestID, orderID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ports.ManifestBundle{}
	for rows.Next() {
		var b ports.ManifestBundle
		if err := rows.Scan(&b.ID, &b.Code, &b.Scanned); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

func (r *FulfillmentRepository) Manifests(ctx context.Context, status string, limit int) ([]ports.Manifest, error) {
	rows, err := r.db.Query(ctx, `SELECT id FROM manifests WHERE ($1='' OR status=$1) ORDER BY id DESC LIMIT $2`, status, limit)
	if err != nil {
		return nil, err
	}
	ids, err := collect[int64](rows)
	if err != nil {
		return nil, err
	}
	out := make([]ports.Manifest, 0, len(ids))
	for _, id := range ids {
		m, err := r.Manifest(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, nil
}

func (r *FulfillmentRepository) ScanPackage(ctx context.Context, manifestID int64, code, actor string) (ports.ManifestLine, error) {
	var pkgID, orderID int64
	var open bool
	err := r.db.QueryRow(ctx, `
		SELECT p.id, p.order_id, m.status='open'
		FROM order_packages p
		JOIN manifest_lines l ON l.order_id=p.order_id AND l.manifest_id=$1 AND l.status='included'
		JOIN manifests m ON m.id=l.manifest_id
		WHERE p.code=$2`, manifestID, code).Scan(&pkgID, &orderID, &open)
	if errors.Is(err, pgx.ErrNoRows) {
		return ports.ManifestLine{}, fmt.Errorf("%w: el bulto %s no pertenece a este manifiesto", domain.ErrInvalid, code)
	}
	if err != nil {
		return ports.ManifestLine{}, err
	}
	if !open {
		return ports.ManifestLine{}, fmt.Errorf("%w: el manifiesto ya está cerrado", domain.ErrConflict)
	}
	if _, err := r.db.Exec(ctx, `INSERT INTO manifest_scans (manifest_id, package_id, scanned_by) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`,
		manifestID, pkgID, actor); err != nil {
		return ports.ManifestLine{}, err
	}
	m, err := r.Manifest(ctx, manifestID)
	if err != nil {
		return ports.ManifestLine{}, err
	}
	for _, l := range m.Lines {
		if l.OrderID == orderID {
			return l, nil
		}
	}
	return ports.ManifestLine{}, domain.ErrNotFound
}

func (r *FulfillmentRepository) SetLineStatus(ctx context.Context, manifestID, orderID int64, status, movedTo string) error {
	_, err := r.db.Exec(ctx, `UPDATE manifest_lines SET status=$3, moved_to=NULLIF($4,'') WHERE manifest_id=$1 AND order_id=$2`,
		manifestID, orderID, status, movedTo)
	return err
}

func (r *FulfillmentRepository) OpenManifestOf(ctx context.Context, orderID int64) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `SELECT l.manifest_id FROM manifest_lines l JOIN manifests m ON m.id=l.manifest_id
		WHERE l.order_id=$1 AND l.status='included' AND m.status='open'`, orderID).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, nil
	}
	return id, err
}

func (r *FulfillmentRepository) CloseManifest(ctx context.Context, m ports.Manifest, actor string) error {
	tag, err := r.db.Exec(ctx, `UPDATE manifests SET status='closed', closed_by=$2, closed_at=now(), picker_name=$3,
		picker_id=NULLIF($4,''), vehicle_plate=NULLIF($5,''), notes=NULLIF($6,'') WHERE id=$1 AND status='open'`,
		m.ID, actor, m.PickerName, m.PickerID, m.VehiclePlate, m.Notes)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrConflict
	}
	return err
}

// ---------------- Cargas masivas ----------------

func (r *FulfillmentRepository) SaveImport(ctx context.Context, companyID int64, filename string, total, created int, errs []ports.RowError, actor string) (int64, error) {
	raw, _ := json.Marshal(errs)
	failed := map[int]bool{}
	for _, e := range errs {
		failed[e.Row] = true
	}
	var id int64
	err := r.db.QueryRow(ctx, `INSERT INTO order_imports (company_id, filename, total_rows, created, failed, errors, created_by)
		VALUES ($1,$2,$3,$4,$5,$6,$7) RETURNING id`, companyID, filename, total, created, len(failed), raw, actor).Scan(&id)
	return id, err
}
