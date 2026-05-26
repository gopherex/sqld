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

// TestExternalReporting verifies External() distinguishes a --dev-url DevDB
// (external, reused) from an ephemeral container.
func TestExternalReporting(t *testing.T) {
	ext, err := Open(context.Background(), "postgres://x/y")
	if err != nil {
		t.Fatal(err)
	}
	defer ext.Close()
	if !ext.External() {
		t.Fatalf("a --dev-url DevDB should report External()=true")
	}
}

// TestReset (I1 support) verifies Reset clears user schemas/objects so an
// external dev DB can be reused for independent introspection states.
func TestReset(t *testing.T) {
	ctx := context.Background()
	d, err := Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer d.Close()

	if d.External() {
		t.Fatalf("an ephemeral container should report External()=false")
	}

	if err := d.Apply(ctx, "CREATE SCHEMA s; CREATE TABLE s.a(id int); CREATE TABLE public.b(id int);"); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := d.Reset(ctx); err != nil {
		t.Fatalf("reset: %v", err)
	}

	conn, err := pgx.Connect(ctx, d.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)

	var tables int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables
		 WHERE table_schema NOT IN ('pg_catalog','information_schema')`).Scan(&tables); err != nil {
		t.Fatal(err)
	}
	if tables != 0 {
		t.Fatalf("after reset, expected 0 user tables, got %d", tables)
	}
	var schemas int
	if err := conn.QueryRow(ctx,
		`SELECT count(*) FROM pg_namespace WHERE nspname='s'`).Scan(&schemas); err != nil {
		t.Fatal(err)
	}
	if schemas != 0 {
		t.Fatalf("after reset, schema s should be dropped")
	}
	// public must still exist and be writable.
	if _, err := conn.Exec(ctx, "CREATE TABLE public.c(id int);"); err != nil {
		t.Fatalf("public schema not usable after reset: %v", err)
	}
}
