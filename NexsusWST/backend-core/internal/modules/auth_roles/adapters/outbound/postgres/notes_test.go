package postgres

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/nexuswst/backend-core/internal/modules/auth_roles/domain"
)

func TestNotesOwnershipAndPartialUpdates(t *testing.T) {
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
	r := NewRepository(db)
	rows, err := db.Query(ctx, `SELECT id FROM users ORDER BY id LIMIT 2`)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		rows.Scan(&id)
		ids = append(ids, id)
	}
	rows.Close()
	if len(ids) < 2 {
		t.Fatal("requiere dos perfiles locales")
	}
	body := "Prueba temporal de aislamiento"
	date := "2026-11-02"
	n, err := r.CreateNote(ctx, ids[0], domain.NoteChanges{Body: &body, DueDate: &date, DueDateSet: true})
	if err != nil {
		t.Fatal(err)
	}
	defer db.Exec(ctx, `DELETE FROM user_notes WHERE id=$1`, n.ID)
	yes := true
	n, err = r.UpdateNote(ctx, ids[0], n.ID, domain.NoteChanges{Done: &yes})
	if err != nil || !n.Done || n.DoneAt == nil || n.DueDate == nil || *n.DueDate != date {
		t.Fatalf("done update: %+v %v", n, err)
	}
	n, err = r.UpdateNote(ctx, ids[0], n.ID, domain.NoteChanges{DueDateSet: true})
	if err != nil || n.DueDate != nil || n.Body != body || !n.Done {
		t.Fatalf("date null: %+v %v", n, err)
	}
	no := false
	n, err = r.UpdateNote(ctx, ids[0], n.ID, domain.NoteChanges{Done: &no})
	if err != nil || n.DoneAt != nil || n.Done {
		t.Fatalf("uncheck %+v %v", n, err)
	}
	if _, err = r.UpdateNote(ctx, ids[1], n.ID, domain.NoteChanges{Body: &body}); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("update ajena", err)
	}
	if err = r.DeleteNote(ctx, ids[1], n.ID); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("delete ajena", err)
	}
	list, err := r.ListNotes(ctx, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range list {
		if v.ID == n.ID {
			t.Fatal("nota ajena expuesta")
		}
	}
	if err = r.DeleteNote(ctx, ids[0], n.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = r.CreateNote(ctx, ids[0], domain.NoteChanges{Body: ptr(strings.Repeat("x", 141))}); err == nil {
		t.Fatal("DB aceptó >140")
	}
}

func ptr(s string) *string { return &s }
