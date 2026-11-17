package postgres

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPackingQueueOrderAndOrphans(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL habilita PostgreSQL")
	}
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	cfg.MaxConns = 1 // las tablas temporales viven en una sola conexión
	db, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err = db.Exec(ctx, queueTables+` CREATE TEMP TABLE order_items (LIKE public.order_items INCLUDING DEFAULTS);
 INSERT INTO users(id,email,name,role,active) VALUES (1,'yo@x.cl','Yo','empaque',true),(2,'otro@x.cl','Otro','empaque',true),(3,'baja@x.cl','Baja','empaque',false);
 INSERT INTO orders(id,company_id,external_ref,channel,courier,status,priority,claimed_by,created_at,updated_at) VALUES
 (9300001,1,'a','manual','t','picked',false,NULL,now()-interval '1 hour',now()-interval '9 hours'),
 (9300002,1,'b','manual','t','picked',false,NULL,now()-interval '5 hours',now()),
 (9300003,1,'c','manual','t','picked',true,NULL,now(),now()),
 (9300004,1,'d','manual','t','packing',false,1,now()-interval '2 hours',now()),
 (9300005,1,'e','manual','t','packing',false,2,now(),now()),
 (9300006,1,'f','manual','t','packing',false,3,now()-interval '3 hours',now()),
 (9300007,1,'g','manual','t','validated',false,NULL,now(),now());`); err != nil {
		t.Fatal(err)
	}
	queue := func(includeDisabled bool) []int64 {
		t.Helper()
		orders, err := NewFulfillmentRepository(db).PackingQueue(ctx, nil, 1, includeDisabled)
		if err != nil {
			t.Fatal(err)
		}
		ids := make([]int64, 0, len(orders))
		for _, o := range orders {
			ids = append(ids, o.ID)
		}
		return ids
	}
	// Prioridad primero y luego lo más antiguo por created_at; updated_at no influye.
	if got, want := queue(false), []int64{9300003, 9300002, 9300004, 9300001}; !slices.Equal(got, want) {
		t.Fatalf("perfil Empaque: %v, esperado %v", got, want)
	}
	if got, want := queue(true), []int64{9300003, 9300002, 9300006, 9300004, 9300001}; !slices.Equal(got, want) {
		t.Fatalf("Jefe/Admin: %v, esperado %v", got, want)
	}
}
