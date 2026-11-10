package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
)

// Core 000012 (Tarea 08): con company_id nulo, el índice uq_profile_requests_pending (NULLS NOT
// DISTINCT) rechaza una segunda solicitud interna pendiente para el mismo perfil y tipo. Sin
// NULLS NOT DISTINCT, dos NULL nunca serían iguales y pasarían las dos.
func TestInternalPendingRequestIsUnique(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL habilita PostgreSQL")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	var requester, target int64
	if err := db.QueryRow(ctx, `SELECT a.id, b.id FROM users a, users b
		WHERE a.role = 'admin' AND b.role IN ('jefe_operaciones', 'empaque') ORDER BY a.id, b.id LIMIT 1`).Scan(&requester, &target); err != nil {
		t.Skip("requiere un Administrador y un perfil interno locales")
	}
	r := NewRequests(db)
	var ids []int64
	defer func() {
		_, _ = db.Exec(ctx, `DELETE FROM user_audit WHERE request_id = ANY($1)`, ids)
		_, _ = db.Exec(ctx, `DELETE FROM profile_requests WHERE id = ANY($1)`, ids)
	}()
	// Deja libre el par (perfil, desactivación) por si quedó una pendiente de otra corrida.
	_, _ = db.Exec(ctx, `UPDATE profile_requests SET status = 'rejected', reviewed_by = $1, reviewed_at = now(), rejection_reason = 'prueba'
		WHERE scope = 'internal' AND status = 'pending' AND kind = 'deactivation' AND target_user_id = $2`, requester, target)

	d := domain.RequestDraft{Scope: domain.ProfileScopeInternal, Kind: domain.RequestDeactivation, TargetUserID: target, Name: "Prueba", Reason: "Prueba de índice"}
	id, err := r.Create(ctx, 0, requester, d)
	if err != nil {
		t.Fatal(err)
	}
	ids = append(ids, id)
	got, err := r.Get(ctx, id)
	if err != nil || got.Scope != domain.ProfileScopeInternal || got.CompanyID != 0 {
		t.Fatalf("solicitud interna: %+v %v", got, err)
	}
	if id2, err := r.Create(ctx, 0, requester, d); !errors.Is(err, domain.ErrRequestPending) {
		if err == nil {
			ids = append(ids, id2)
		}
		t.Fatalf("segunda pendiente interna para el mismo perfil y tipo: %v, quería ErrRequestPending", err)
	}
	// Perfil nuevo sin correo: se identifica por el usuario; distinto usuario sí pasa.
	nd := domain.RequestDraft{Scope: domain.ProfileScopeInternal, Kind: domain.RequestNewProfile, Name: "Prueba", Username: "prueba.indice.1",
		Role: "empaque", Permissions: []string{"despacho"}, Reason: "Prueba de índice"}
	id, err = r.Create(ctx, 0, requester, nd)
	if err != nil {
		t.Fatal(err)
	}
	ids = append(ids, id)
	if _, err := r.Create(ctx, 0, requester, nd); !errors.Is(err, domain.ErrRequestPending) {
		t.Fatalf("segunda pendiente del mismo usuario: %v", err)
	}
	nd.Username = "prueba.indice.2"
	id, err = r.Create(ctx, 0, requester, nd)
	if err != nil {
		t.Fatalf("otro usuario: %v", err)
	}
	ids = append(ids, id)
}
