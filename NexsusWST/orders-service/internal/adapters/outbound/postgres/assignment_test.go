package postgres

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

// El SQL usa el reloj de la base de datos: las horas extremas (00:00 ya pasó, 23:59 aún no)
// dan el mismo resultado todo el día, salvo en esos minutos.
func skipNearMidnight(t *testing.T) {
	t.Helper()
	loc, err := time.LoadLocation("America/Santiago")
	if err != nil {
		t.Fatal(err)
	}
	if now := time.Now().In(loc).Format("15:04"); now < "00:02" || now > "23:57" {
		t.Skip("cerca de medianoche")
	}
}

const queueTables = `CREATE TEMP TABLE orders (LIKE public.orders INCLUDING DEFAULTS);
 CREATE TEMP TABLE users (LIKE public.users INCLUDING DEFAULTS);
 CREATE TEMP TABLE ops_settings (LIKE public.ops_settings INCLUDING DEFAULTS);
 CREATE TEMP TABLE job_outbox (LIKE public.job_outbox INCLUDING DEFAULTS);
 CREATE TEMP TABLE cutoffs (LIKE public.cutoffs INCLUDING DEFAULTS);
 CREATE TEMP TABLE cutoff_couriers (LIKE public.cutoff_couriers INCLUDING DEFAULTS);
 INSERT INTO cutoffs(id,cutoff_time,updated_by) VALUES (1,'00:00','t'),(2,'23:58','t'),(3,'23:59','t');
 INSERT INTO cutoff_couriers(cutoff_id,courier,arrival_time) VALUES (1,'pasado','00:30'),(2,'temprano','23:59'),(3,'tarde','23:59');`

func setSchedule(ctx context.Context, t *testing.T, q querier, s domain.OperationSchedule) {
	t.Helper()
	days := ""
	for i, day := range s.OperationDays {
		if i > 0 {
			days += ","
		}
		days += day
	}
	flag := "false"
	if s.PriorityOverridesDeadline {
		flag = "true"
	}
	if _, err := q.Exec(ctx, `DELETE FROM ops_settings`); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"assignment_deadline": s.AssignmentDeadline, "operation_start": s.OperationStart, "operation_days": days, "priority_overrides_deadline": flag} {
		if _, err := q.Exec(ctx, `INSERT INTO ops_settings(key,value) VALUES($1,$2)`, key, value); err != nil {
			t.Fatal(err)
		}
	}
}

var openAllDay = domain.OperationSchedule{AssignmentDeadline: "23:59", OperationStart: "00:00", OperationDays: domain.WeekDays}

func TestAssignmentSituationSQL(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL habilita PostgreSQL")
	}
	skipNearMidnight(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if _, err = db.Exec(ctx, queueTables); err != nil {
		t.Fatal(err)
	}
	afterDeadline := openAllDay
	afterDeadline.AssignmentDeadline = "00:00"
	overriding := afterDeadline
	overriding.PriorityOverridesDeadline = true
	beforeStart := openAllDay
	beforeStart.OperationStart = "23:59"
	noDays := openAllDay
	noDays.OperationDays = []string{"xxx"}
	for i, tc := range []struct {
		name, status, prev, kind, job, oldJob, courier string
		checked, wave, priority                        bool
		schedule                                       domain.OperationSchedule
		want                                           string
	}{
		{name: "en cola con ola", status: "validated", job: "pending", oldJob: "failed", courier: "tarde", checked: true, wave: true, schedule: openAllDay, want: "en_cola"},
		{name: "agotó reintentos", status: "incident", prev: "validated", kind: "invas", job: "failed", oldJob: "pending", courier: "tarde", checked: true, wave: true, schedule: openAllDay, want: "invas_agotado"},
		{name: "sin regla", status: "validated", courier: "tarde", checked: true, schedule: openAllDay, want: "sin_regla"},
		{name: "normal", status: "validated", courier: "tarde", schedule: openAllDay},
		{name: "fuera del universo", status: "pending", job: "pending", courier: "pasado", checked: true, schedule: openAllDay},
		{name: "otra incidencia", status: "incident", prev: "validated", kind: "datos", job: "failed", courier: "tarde", checked: true, schedule: openAllDay},
		{name: "último trabajo completado", status: "validated", job: "completed", oldJob: "failed", courier: "tarde", checked: true, wave: true, schedule: openAllDay},
		{name: "pasó el último corte de su courier", status: "validated", courier: "pasado", schedule: openAllDay, want: "fuera_corte"},
		{name: "fuera de corte manda sobre sin regla", status: "validated", courier: "pasado", checked: true, schedule: openAllDay, want: "fuera_corte"},
		{name: "prioritaria fuera de corte no queda retenida", status: "validated", courier: "pasado", priority: true, schedule: openAllDay},
		{name: "courier sin cortes no queda retenido", status: "validated", courier: "sin_cortes", schedule: openAllDay},
		{name: "después de la hora límite", status: "validated", courier: "tarde", schedule: afterDeadline, want: "fuera_corte"},
		{name: "la prioridad no salta la hora límite", status: "validated", courier: "tarde", priority: true, schedule: afterDeadline, want: "fuera_corte"},
		{name: "con el parámetro, sí la salta", status: "validated", courier: "tarde", priority: true, schedule: overriding},
		{name: "antes del inicio de operación", status: "validated", courier: "tarde", schedule: beforeStart, want: "fuera_corte"},
		{name: "día sin operación", status: "validated", courier: "tarde", priority: true, schedule: noDays, want: "fuera_corte"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			setSchedule(ctx, t, db, tc.schedule)
			oid := int64(i + 1)
			_, err := db.Exec(ctx, `INSERT INTO orders(id,company_id,external_ref,channel,status,prev_status,incident_kind,rules_checked_at,wave_id,courier,priority)
 VALUES($1::bigint,1,($1::bigint)::text,'manual',$2,NULLIF($3,''),NULLIF($4,''),CASE WHEN $5 THEN now() END,CASE WHEN $6 THEN 'OLA' END,$7,$8)`, oid, tc.status, tc.prev, tc.kind, tc.checked, tc.wave, tc.courier, tc.priority)
			if err != nil {
				t.Fatal(err)
			}
			for n, job := range []string{tc.oldJob, tc.job} {
				if job == "" {
					continue
				}
				_, err = db.Exec(ctx, `INSERT INTO job_outbox(id,type,company_id,order_id,payload,created_at,failed_at,completed_at)
 VALUES($1,'assign_wave',1,$2,'{}',now()+$3::int*interval '1 second',CASE WHEN $4='failed' THEN now() END,CASE WHEN $4='completed' THEN now() END)`, tc.name+job, oid, n, job)
				if err != nil {
					t.Fatal(err)
				}
			}
			var got *string
			if err = db.QueryRow(ctx, `SELECT `+assignmentSituation+` FROM `+fromOrders+` WHERE o.id=$1`, oid).Scan(&got); err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if got != nil {
					t.Fatalf("esperaba null, llegó %s", *got)
				}
			} else if got == nil || *got != tc.want {
				t.Fatalf("esperaba %s, llegó %v", tc.want, got)
			}
		})
	}
}

// La pantalla no debe contradecir al motor: el SQL y domain.OperationSchedule.Decide
// responden lo mismo para cada combinación de horario, courier y prioridad.
func TestRetainedSQLMatchesEngine(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL habilita PostgreSQL")
	}
	skipNearMidnight(t)
	ctx := context.Background()
	db, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	if _, err = db.Exec(ctx, queueTables); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO orders(id,company_id,external_ref,channel,status,courier,priority) VALUES
 (1,1,'a','manual','validated','pasado',false),(2,1,'b','manual','validated','pasado',true),
 (3,1,'c','manual','validated','tarde',false),(4,1,'d','manual','validated','tarde',true),
 (5,1,'e','manual','validated','sin_cortes',false),(6,1,'f','manual','validated','sin_cortes',true)`); err != nil {
		t.Fatal(err)
	}
	cutoffs := map[string][]string{"pasado": {"00:00"}, "tarde": {"23:59"}, "temprano": {"23:58"}}
	schedules := []domain.OperationSchedule{openAllDay}
	for _, start := range []string{"00:00", "23:59"} {
		for _, deadline := range []string{"00:00", "23:59"} {
			for _, days := range [][]string{domain.WeekDays, {"xxx"}} {
				for _, flag := range []bool{false, true} {
					schedules = append(schedules, domain.OperationSchedule{AssignmentDeadline: deadline, OperationStart: start, OperationDays: days, PriorityOverridesDeadline: flag})
				}
			}
		}
	}
	for _, s := range schedules {
		setSchedule(ctx, t, db, s)
		rows, err := db.Query(ctx, `SELECT o.courier, o.priority, `+assignmentRetained+` FROM `+fromOrders+` ORDER BY o.id`)
		if err != nil {
			t.Fatal(err)
		}
		type row struct {
			courier  string
			priority bool
			retained *bool
		}
		var list []row
		for rows.Next() {
			var r row
			if err := rows.Scan(&r.courier, &r.priority, &r.retained); err != nil {
				t.Fatal(err)
			}
			list = append(list, r)
		}
		rows.Close()
		for _, r := range list {
			engine := !s.Decide(time.Now(), cutoffs[r.courier], r.priority).Assign
			screen := r.retained != nil && *r.retained
			if engine != screen {
				t.Errorf("horario %+v, courier %s, prioridad %v: motor retiene=%v, pantalla=%v", s, r.courier, r.priority, engine, screen)
			}
		}
	}
}

func TestAssignmentViewShowsTheQueueOrder(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL habilita PostgreSQL")
	}
	skipNearMidnight(t)
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
	if _, err = db.Exec(ctx, queueTables); err != nil {
		t.Fatal(err)
	}
	setSchedule(ctx, t, db, openAllDay)
	if _, err = db.Exec(ctx, `INSERT INTO orders(id,company_id,external_ref,channel,status,courier,priority,created_at) VALUES
 (9200001,1,'a','manual','validated','tarde',false,now()-interval '2 hours'),
 (9200002,1,'b','manual','validated','temprano',false,now()-interval '1 hour'),
 (9200003,1,'c','manual','validated','tarde',true,now()),
 (9200004,1,'d','manual','validated','pasado',true,now()),
 (9200005,1,'e','manual','validated','sin_cortes',false,now()-interval '9 hours'),
 (9200006,1,'f','manual','validated','temprano',false,now()-interval '3 hours')`); err != nil {
		t.Fatal(err)
	}
	orders, _, err := NewRepository(db).List(ctx, ports.ListFilter{View: ports.ViewAssignment, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	got := make([]int64, 0, len(orders))
	for _, o := range orders {
		got = append(got, o.ID)
	}
	// Prioritaria fuera de corte, corte 23:58 por antigüedad, corte 23:59 con la prioritaria primero, sin corte al final.
	want := []int64{9200004, 9200006, 9200002, 9200003, 9200001, 9200005}
	if !slices.Equal(got, want) {
		t.Fatalf("orden %v, esperado %v", got, want)
	}
}
