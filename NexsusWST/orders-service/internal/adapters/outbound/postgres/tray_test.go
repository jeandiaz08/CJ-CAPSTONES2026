package postgres

import (
	"context"
	"os"
	"slices"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexuswst/orders-service/internal/domain"
	"github.com/nexuswst/orders-service/internal/ports"
)

// Bandeja (CU-06/H-09): en curso siempre, Entregadas hasta 365 días, nunca Canceladas.
func TestTrayRegistryAndFilters(t *testing.T) {
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
	if _, err = db.Exec(ctx, `CREATE TEMP TABLE orders (LIKE public.orders INCLUDING DEFAULTS);
 CREATE TEMP TABLE users (LIKE public.users INCLUDING DEFAULTS);
 CREATE TEMP TABLE ops_settings (LIKE public.ops_settings INCLUDING DEFAULTS);
 CREATE TEMP TABLE job_outbox (LIKE public.job_outbox INCLUDING DEFAULTS);
 CREATE TEMP TABLE cutoffs (LIKE public.cutoffs INCLUDING DEFAULTS);
 CREATE TEMP TABLE cutoff_couriers (LIKE public.cutoff_couriers INCLUDING DEFAULTS);
 INSERT INTO ops_settings(key,value) VALUES('assignment_deadline','12:00');`); err != nil {
		t.Fatal(err)
	}
	const (
		pending int64 = 9000001 + iota
		delivered10
		delivered364
		delivered366
		cancelled
		voided
		deliveredNoDate
		queuedBulk
		validated
	)
	for _, o := range []struct {
		id               int64
		company          int
		status, source   string
		deliveredDaysAgo *int
		priority         bool
	}{
		{pending, 2, "pending", "manual", nil, false},
		{delivered10, 1, "handed_over", "manual", days(10), false},
		{delivered364, 1, "handed_over", "manual", days(364), false},
		{delivered366, 1, "handed_over", "manual", days(366), false},
		{cancelled, 1, "cancelled", "manual", nil, false},
		{voided, 1, "voided", "webhook", nil, false},
		{deliveredNoDate, 1, "handed_over", "manual", nil, false},
		{queuedBulk, 1, "validated", "bulk", nil, true},
		{validated, 1, "validated", "manual", nil, false},
	} {
		if _, err = db.Exec(ctx, `INSERT INTO orders(id,company_id,external_ref,channel,courier,status,source,priority,handed_over_at)
 VALUES($1::bigint,$2::bigint,($1::bigint)::text,'manual','test',$3,$4,$5,now()-make_interval(days=>$6::int))`,
			o.id, o.company, o.status, o.source, o.priority, o.deliveredDaysAgo); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = db.Exec(ctx, `INSERT INTO job_outbox(id,type,company_id,order_id,payload) VALUES('tray-job','assign_wave',1,$1,'{}')`, queuedBulk); err != nil {
		t.Fatal(err)
	}

	repo := NewRepository(db)
	for _, tc := range []struct {
		name   string
		filter ports.ListFilter
		want   []int64
	}{
		{"bandeja: en curso y Entregadas de hasta 365 días, reciente primero", ports.ListFilter{},
			[]int64{validated, queuedBulk, deliveredNoDate, voided, delivered364, delivered10, pending}},
		{"registro: Canceladas y Entregadas de más de 365 días", ports.ListFilter{View: ports.ViewRegistry},
			[]int64{cancelled, delivered366}},
		{"la bandeja no muestra Canceladas ni filtrando por estado", ports.ListFilter{Statuses: []string{"cancelled"}}, []int64{}},
		{"bandeja filtrada por Entregada", ports.ListFilter{Statuses: []string{"handed_over"}},
			[]int64{deliveredNoDate, delivered364, delivered10}},
		{"filtro de origen", ports.ListFilter{Source: "bulk"}, []int64{queuedBulk}},
		{"filtro de situación con total correcto", ports.ListFilter{Situation: "en_cola"}, []int64{queuedBulk}},
		{"Asignación mantiene la prioridad primero", ports.ListFilter{View: ports.ViewAssignment}, []int64{queuedBulk, validated}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.filter.Limit = 200
			orders, total, err := repo.List(ctx, tc.filter)
			if err != nil {
				t.Fatal(err)
			}
			got := make([]int64, 0, len(orders))
			for _, o := range orders {
				got = append(got, o.ID)
			}
			if !slices.Equal(got, tc.want) || total != len(tc.want) {
				t.Fatalf("got %v (total %d), want %v", got, total, tc.want)
			}
		})
	}

	s, err := repo.Summary(ctx, []int64{2})
	if err != nil {
		t.Fatal(err)
	}
	if s[domain.StatusPending] != 1 || s[domain.StatusValidated] != 0 {
		t.Fatalf("resumen por empresa: %v", s)
	}
}

func days(n int) *int { return &n }
