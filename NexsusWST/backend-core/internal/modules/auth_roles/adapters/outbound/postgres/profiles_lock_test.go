package postgres

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// WithManagerLock (Tarea 08): dos cambios simultáneos sobre gestores corren uno tras otro, y el
// candado se suelta aunque fn falle.
func TestWithManagerLockSerializes(t *testing.T) {
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
	r := NewProfiles(db)

	var mu sync.Mutex
	events := []string{}
	mark := func(e string) { mu.Lock(); events = append(events, e); mu.Unlock() }
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = r.WithManagerLock(ctx, func(context.Context) error {
			mark("a-in")
			time.Sleep(150 * time.Millisecond)
			mark("a-out")
			return nil
		})
	}()
	time.Sleep(40 * time.Millisecond) // A ya tomó el candado
	go func() {
		defer wg.Done()
		_ = r.WithManagerLock(ctx, func(context.Context) error { mark("b-in"); return nil })
	}()
	wg.Wait()
	if len(events) != 3 || events[0] != "a-in" || events[1] != "a-out" || events[2] != "b-in" {
		t.Fatalf("el segundo debía esperar al primero: %v", events)
	}

	// Si fn falla, devuelve su error y el candado queda libre.
	boom := errors.New("falla")
	if err := r.WithManagerLock(ctx, func(context.Context) error { return boom }); !errors.Is(err, boom) {
		t.Fatalf("error de fn: %v", err)
	}
	done := make(chan error, 1)
	go func() { done <- r.WithManagerLock(ctx, func(context.Context) error { return nil }) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("el candado no se soltó tras una falla")
	}
}
