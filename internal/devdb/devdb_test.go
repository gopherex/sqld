package devdb

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestDevDBApply(t *testing.T) {
	ctx := context.Background()
	d, err := Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer d.Close()

	if err := d.Apply(ctx, "CREATE TABLE t(id int);"); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// verify via a fresh connection
	conn, err := pgx.Connect(ctx, d.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	var n int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM information_schema.tables WHERE table_name='t'").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Fatalf("table t not found (n=%d)", n)
	}
}

func TestOpenWithURL(t *testing.T) {
	d, err := Open(context.Background(), "postgres://x/y")
	if err != nil {
		t.Fatal(err)
	}
	defer d.Close()
	if d.URL() != "postgres://x/y" {
		t.Fatalf("url=%q", d.URL())
	}
}
