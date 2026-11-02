package postgres

import (
	"context"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nexuswst/orders-service/internal/ports"
	"os"
	"testing"
)

func TestPackageCountAndManifest(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL required")
	}
	ctx := context.Background()
	db, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	orders, total, err := NewRepository(db).List(ctx, ports.ListFilter{View: ports.ViewAll, Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	if total == 0 {
		t.Fatal("requires local seed")
	}
	tray, _, err := NewRepository(db).List(ctx, ports.ListFilter{Limit: 200})
	if err != nil {
		t.Fatal(err)
	}
	for _, o := range tray {
		if o.Status == "handed_over" || o.Status == "cancelled" {
			t.Fatal("estado final en bandeja")
		}
	}
	sawEmpty, sawManifest := false, false
	for _, o := range orders {
		p, err := NewFulfillmentRepository(db).Packages(ctx, o.ID)
		if err != nil {
			t.Fatal(err)
		}
		if o.PackagesCount != len(p) {
			t.Fatalf("order %d: count=%d packages=%d", o.ID, o.PackagesCount, len(p))
		}
		if len(p) == 0 {
			sawEmpty = true
		}
		for _, v := range p {
			var expected *int64
			err = db.QueryRow(ctx, `SELECT COALESCE((SELECT manifest_id FROM manifest_scans WHERE package_id=$1 ORDER BY scanned_at DESC,manifest_id DESC LIMIT 1),(SELECT manifest_id FROM manifest_lines WHERE order_id=$2 AND status IN ('included','pending_packages','delivered') ORDER BY manifest_id DESC LIMIT 1))`, v.ID, o.ID).Scan(&expected)
			if err != nil {
				t.Fatal(err)
			}
			if (expected == nil) != (v.ManifestID == nil) || expected != nil && *expected != *v.ManifestID {
				t.Fatalf("package %d manifest mismatch", v.ID)
			}
			if expected != nil {
				sawManifest = true
			}
		}
	}
	if !sawEmpty || !sawManifest {
		t.Fatal("seed must include empty packages and manifests")
	}
}
