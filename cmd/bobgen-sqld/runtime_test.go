// Runtime symbiosis proof for the bobgen PoC.
//
// The compile-level PoC (commit 40e6f66) proved bob can be forced to emit
// sqld's canonical Go type (pocshared.AccountStatus) for an enum column. This
// test proves the same type WORKS at runtime across both halves of the
// ORM⊕sqlc symbiosis, sharing ONE connection pool:
//
//   - WRITE path  : bob's generated models.Accounts.Insert(...) ORM query,
//                   executed over a *pgxpool.Pool via bob's native pgx executor.
//   - READ path   : a raw pgx SELECT on the SAME pool, scanning the enum column
//                   straight into a pocshared.AccountStatus (this mimics the row
//                   field code sqld-gen-go emits for an enum column).
//
// If the value written by bob round-trips back through a sqld-style raw read as
// the SAME Go type, runtime symbiosis is proven.
//
// Docker is required; the test skips gracefully when it is unavailable (matching
// the repo's other integration tests, e.g. internal/introspect, internal/devdb).
package main

import (
	"context"
	"testing"

	"github.com/aarondl/opt/omit"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stephenafamo/bob"
	bobpgx "github.com/stephenafamo/bob/drivers/pgx"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/im"

	"github.com/yaroher/sqld/cmd/bobgen-sqld/out/models"
	"github.com/yaroher/sqld/cmd/bobgen-sqld/pocshared"
	"github.com/yaroher/sqld/internal/devdb"
)

// ddl matches cmd/bobgen-sqld/testdata/schema.sql (the schema the generated
// models package was built from). The accounts model scans all six columns, so
// the table must carry every one of them including the nullable nickname.
const ddl = `
CREATE TYPE account_status AS ENUM ('active','suspended','closed');
CREATE TABLE accounts (
  id          bigserial PRIMARY KEY,
  email       text NOT NULL,
  status      account_status NOT NULL,
  is_verified boolean NOT NULL DEFAULT false,
  created_at  timestamptz NOT NULL DEFAULT now(),
  nickname    text
);`

// TestRuntimeSymbiosis is the proof: bob (ORM write) and a sqld-style raw read
// share ONE *pgxpool.Pool and ONE canonical Go type (pocshared.AccountStatus),
// and a value round-trips through both.
func TestRuntimeSymbiosis(t *testing.T) {
	ctx := context.Background()

	// Start an ephemeral Postgres (skip if Docker is unavailable, like the
	// repo's other integration tests).
	d, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer d.Close()

	// ONE pool, shared by both bob's ORM write path and the sqld-style raw read.
	pool, err := pgxpool.New(ctx, d.URL())
	if err != nil {
		t.Fatalf("open pgxpool: %v", err)
	}
	defer pool.Close()

	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatalf("apply ddl: %v", err)
	}

	// bob's NATIVE pgx executor: wrap the *pgxpool.Pool directly. No
	// database/sql, no pgx/stdlib — bobpgx.Pool implements bob.Executor over
	// the very same pool the raw read uses.
	exec := bobpgx.NewPool(pool)

	// ---- WRITE PATH (bob ORM) -------------------------------------------
	// Build the insert setter using sqld's canonical type. Status is typed as
	// omit.Val[pocshared.AccountStatus] in the generated model — bob emits OUR
	// type, and we feed it OUR const.
	setter := &models.AccountSetter{
		Email:  omit.From("alice@example.com"),
		Status: omit.From(pocshared.AccountStatusActive),
	}

	// models.Accounts.Insert(setter) auto-appends RETURNING all columns and
	// .One scans the result back into a *models.Account (whose Status field is
	// also pocshared.AccountStatus). Executed over the shared pool via bob.
	inserted, err := models.Accounts.Insert(setter).One(ctx, exec)
	if err != nil {
		t.Fatalf("bob insert: %v", err)
	}
	if inserted.ID == 0 {
		t.Fatalf("expected non-zero id from bob insert, got %d", inserted.ID)
	}
	// bob already round-tripped its own type on the write path (RETURNING).
	if inserted.Status != pocshared.AccountStatusActive {
		t.Fatalf("bob insert RETURNING: status=%q want %q", inserted.Status, pocshared.AccountStatusActive)
	}

	// ---- READ PATH (sqld-style raw pgx) ---------------------------------
	// This mirrors what sqld-gen-go emits for an enum column: a raw pgx query
	// scanning the enum text directly into the canonical Go type. SAME pool.
	var got pocshared.AccountStatus
	if err := pool.QueryRow(ctx,
		"SELECT status FROM accounts WHERE id = $1", inserted.ID,
	).Scan(&got); err != nil {
		t.Fatalf("sqld-style raw read: %v", err)
	}

	// ---- THE PROOF ------------------------------------------------------
	// Value written by bob (ORM), read back by sqld-style raw pgx, one pool,
	// one Go type. They are equal.
	if got != pocshared.AccountStatusActive {
		t.Fatalf("round-trip mismatch: read %q, want %q", got, pocshared.AccountStatusActive)
	}
	t.Logf("runtime symbiosis proven: bob wrote pocshared.AccountStatus(%q); "+
		"sqld-style raw read on the SAME pgxpool read back %q via bob's native pgx executor",
		inserted.Status, got)
}

// TestRuntimeSymbiosisComposite is the STRETCH: a custom Postgres COMPOSITE
// type that REQUIRES per-connection registration, round-tripped through both
// bob (write, via bob's query builder + native pgx executor) and raw pgx
// (read), using ONE shared canonical Go type (pocshared.GeoPoint) and ONE pool.
//
// Unlike the string-kind enum (which needs no registration), a composite codec
// must be loaded onto every pooled connection. We do that in
// pgxpool.Config.AfterConnect — the same hook a real sqld+bob app would use so
// both halves see the registered type on whatever connection they acquire.
func TestRuntimeSymbiosisComposite(t *testing.T) {
	ctx := context.Background()

	d, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer d.Close()

	// The composite type must exist before AfterConnect runs (LoadType reads it
	// from the catalog), so create it on a throwaway connection first.
	const compDDL = `
CREATE TYPE geo_point AS (lat double precision, lng double precision);
CREATE TABLE places (
  id   bigint PRIMARY KEY,
  spot geo_point NOT NULL
);`
	if err := d.Apply(ctx, compDDL); err != nil {
		t.Fatalf("apply composite ddl: %v", err)
	}

	// ONE pool whose AfterConnect registers the geo_point codec on every conn.
	cfg, err := pgxpool.ParseConfig(d.URL())
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		dt, err := conn.LoadType(ctx, "geo_point")
		if err != nil {
			return err
		}
		conn.TypeMap().RegisterType(dt)
		return nil
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("open pgxpool: %v", err)
	}
	defer pool.Close()

	exec := bobpgx.NewPool(pool)

	// ---- WRITE PATH (bob query builder over the shared pool) ------------
	// The shared canonical Go type, passed as a bob arg. bob renders $N
	// placeholders and hands the pocshared.GeoPoint value to pgx, whose
	// registered composite codec encodes it.
	want := pocshared.GeoPoint{Lat: 51.5074, Lng: -0.1278}
	insertQ := psql.Insert(
		im.Into("places", "id", "spot"),
		im.Values(psql.Arg(int64(1)), psql.Arg(want)),
	)
	if _, err := bob.Exec(ctx, exec, insertQ); err != nil {
		t.Fatalf("bob composite insert: %v", err)
	}

	// ---- READ PATH (sqld-style raw pgx) ---------------------------------
	// Scan the composite straight into the SAME canonical Go type.
	var gotPoint pocshared.GeoPoint
	if err := pool.QueryRow(ctx,
		"SELECT spot FROM places WHERE id = $1", int64(1),
	).Scan(&gotPoint); err != nil {
		t.Fatalf("sqld-style raw composite read: %v", err)
	}

	// ---- THE PROOF ------------------------------------------------------
	if gotPoint != want {
		t.Fatalf("composite round-trip mismatch: read %+v, want %+v", gotPoint, want)
	}
	t.Logf("composite symbiosis proven: bob wrote pocshared.GeoPoint(%+v); "+
		"sqld-style raw read on the SAME pgxpool read back %+v "+
		"(geo_point codec registered via pgxpool AfterConnect)", want, gotPoint)
}
