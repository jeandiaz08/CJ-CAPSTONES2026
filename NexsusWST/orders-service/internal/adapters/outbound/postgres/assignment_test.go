package postgres

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestAssignmentSituationSQL(t *testing.T) {
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
	_, err = tx.Exec(ctx, `CREATE TEMP TABLE orders (LIKE public.orders INCLUDING DEFAULTS) ON COMMIT DROP;
 CREATE TEMP TABLE users (LIKE public.users INCLUDING DEFAULTS) ON COMMIT DROP;
 CREATE TEMP TABLE courier_settings (LIKE public.courier_settings INCLUDING DEFAULTS) ON COMMIT DROP;
 CREATE TEMP TABLE ops_settings (LIKE public.ops_settings INCLUDING DEFAULTS) ON COMMIT DROP;
 CREATE TEMP TABLE job_outbox (LIKE public.job_outbox INCLUDING DEFAULTS) ON COMMIT DROP;
 CREATE TEMP TABLE cutoffs(id bigint,cutoff_time time,active boolean) ON COMMIT DROP; CREATE TEMP TABLE cutoff_couriers(cutoff_id bigint,courier text) ON COMMIT DROP; INSERT INTO cutoffs VALUES(1,'06:00',true),(2,'08:00',true),(3,'23:00',false); INSERT INTO cutoff_couriers VALUES(1,'test'),(2,'test'),(3,'test');
 INSERT INTO ops_settings(key,value) VALUES('assignment_deadline','12:00');`)
	if err != nil {
		t.Fatal(err)
	}
	for i, tc := range []struct {
		name, status, prev, kind, job, oldJob, created string
		checked, wave                                  bool
		want                                           string
	}{
		{"queued with wave", "validated", "", "", "pending", "failed", "09:00", true, true, "en_cola"},
		{"exhausted", "incident", "validated", "invas", "failed", "pending", "09:00", true, true, "invas_agotado"},
		{"cut before no rule", "validated", "", "", "", "", "09:00", true, false, "fuera_corte"},
		{"no rule", "validated", "", "", "", "", "07:00", true, false, "sin_regla"},
		{"normal", "validated", "", "", "", "", "07:00", false, false, ""},
		{"outside universe", "pending", "", "", "pending", "", "09:00", true, false, ""},
		{"other incident", "incident", "validated", "datos", "failed", "", "09:00", true, false, ""},
		{"completed latest", "validated", "", "", "completed", "failed", "07:00", true, true, ""},
		{"deadline without courier cutoff", "validated", "", "", "", "", "13:00", false, false, ""},
		{"yesterday", "validated", "", "", "", "", "yesterday", false, false, ""},
		{"after first before last", "validated", "", "", "", "", "07:30", false, false, ""},
		{"after deadline", "validated", "", "", "", "", "13:00", false, false, "fuera_corte"},
		{"cut equality", "validated", "", "", "", "", "08:00", false, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			oid := int64(i + 1)
			courier := "test"
			if tc.name == "deadline without courier cutoff" {
				courier = "unset"
			}
			created := `((CURRENT_TIMESTAMP AT TIME ZONE 'America/Santiago')::date + $7::time) AT TIME ZONE 'America/Santiago'`
			clock := tc.created
			if clock == "yesterday" {
				created = `(((CURRENT_TIMESTAMP AT TIME ZONE 'America/Santiago')::date - 1) + $7::time) AT TIME ZONE 'America/Santiago'`
				clock = "09:00"
			}
			_, err := tx.Exec(ctx, `INSERT INTO orders(id,company_id,external_ref,channel,status,prev_status,incident_kind,created_at,rules_checked_at,wave_id,courier)
 VALUES($1::bigint,1,($1::bigint)::text,'manual',$2,NULLIF($3,''),NULLIF($4,''),`+created+`,CASE WHEN $5 THEN now() END,CASE WHEN $6 THEN 'OLA' END,$8)`, oid, tc.status, tc.prev, tc.kind, tc.checked, tc.wave, clock, courier)
			if err != nil {
				t.Fatal(err)
			}
			for n, job := range []string{tc.oldJob, tc.job} {
				if job == "" {
					continue
				}
				_, err = tx.Exec(ctx, `INSERT INTO job_outbox(id,type,company_id,order_id,payload,created_at,failed_at,completed_at)
 VALUES($1,'assign_wave',1,$2,'{}',now()+$3::int*interval '1 second',CASE WHEN $4='failed' THEN now() END,CASE WHEN $4='completed' THEN now() END)`, tc.name+job, oid, n, job)
				if err != nil {
					t.Fatal(err)
				}
			}
			var got *string
			err = tx.QueryRow(ctx, `SELECT `+assignmentSituation+` FROM `+fromOrders+` WHERE o.id=$1`, oid).Scan(&got)
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if got != nil {
					t.Fatalf("want null got %s", *got)
				}
			} else if got == nil || *got != tc.want {
				t.Fatalf("want %s got %v", tc.want, got)
			}
		})
	}
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM orders o WHERE `+assignmentUniverse).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 11 {
		t.Fatalf("universe count %d", count)
	}
}
