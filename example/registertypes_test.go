package main

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherex/sqld/example/gen/db"
	"github.com/gopherex/sqld/pkg/devdb"
)

// TestRegisterTypesHstoreRoundTrip applies the full example schema and wires
// db.RegisterTypes into the pool's AfterConnect. It proves the hstore fix: pgx's
// Conn.LoadType cannot load a non-array base type, so RegisterTypes used to fail
// with "load type hstore: array element OID not registered" the moment any
// connection opened. Now RegisterTypes (hstore + ltree-less + enums + composites
// + ranges) succeeds, and an hstore value round-trips. Docker-gated.
func TestRegisterTypesHstoreRoundTrip(t *testing.T) {
	ctx := context.Background()

	dev, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer dev.Close()

	if err := dev.Apply(ctx, "CREATE EXTENSION IF NOT EXISTS hstore; CREATE EXTENSION IF NOT EXISTS ltree;"); err != nil {
		t.Fatalf("create extensions: %v", err)
	}
	schema, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := dev.Apply(ctx, string(schema)); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	cfg, err := pgxpool.ParseConfig(dev.URL())
	if err != nil {
		t.Fatal(err)
	}
	// The fix under test: this must NOT error when the first connection opens.
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		return db.RegisterTypes(ctx, c)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// Forces a connection → AfterConnect → RegisterTypes; also round-trips hstore.
	var h pgtype.Hstore
	if err := pool.QueryRow(ctx, `SELECT 'a=>1, b=>2'::hstore`).Scan(&h); err != nil {
		t.Fatalf("hstore scan (RegisterTypes failed in AfterConnect?): %v", err)
	}
	if h["a"] == nil || *h["a"] != "1" || h["b"] == nil || *h["b"] != "2" {
		t.Fatalf("hstore round-trip = %v; want a=>1, b=>2", h)
	}
}
