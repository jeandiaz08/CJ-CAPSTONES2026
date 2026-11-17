package workers

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestClaimJobsOrder(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL habilita PostgreSQL")
	}
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if _, err = tx.Exec(ctx, `CREATE TEMP TABLE job_outbox (LIKE public.job_outbox INCLUDING DEFAULTS) ON COMMIT DROP;
 INSERT INTO job_outbox(id,type,company_id,payload,priority,next_cutoff,created_at) VALUES
 ('sin-corte','assign_wave',1,'{}',true,NULL,now()-interval '5 hours'),
 ('corte-11','assign_wave',1,'{}',true,'11:00',now()-interval '4 hours'),
 ('corte-08-reciente','assign_wave',1,'{}',false,'08:00',now()-interval '1 hour'),
 ('corte-08-antigua','assign_wave',1,'{}',false,'08:00',now()-interval '3 hours'),
 ('corte-08-prioritaria','assign_wave',1,'{}',true,'08:00',now()),
 ('fuera-prioritaria','assign_wave',1,'{}',true,'00:00',now()),
 ('webhook','process_webhook',1,'{}',false,NULL,now());
 INSERT INTO job_outbox(id,type,company_id,payload,published_at) VALUES ('publicado','assign_wave',1,'{}',now());`); err != nil {
		t.Fatal(err)
	}
	rows, err := tx.Query(ctx, claimJobsSQL, 10)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for rows.Next() {
		var id, typ, payload string
		var company, order int64
		if err := rows.Scan(&id, &typ, &company, &order, &payload); err != nil {
			t.Fatal(err)
		}
		got = append(got, id)
	}
	rows.Close()
	want := []string{"webhook", "fuera-prioritaria", "corte-08-prioritaria", "corte-08-antigua", "corte-08-reciente", "corte-11", "sin-corte"}
	if !slices.Equal(got, want) {
		t.Fatalf("orden:\n%v\nesperado:\n%v", got, want)
	}
}
