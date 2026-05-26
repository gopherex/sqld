package migrate

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yaroher/sqld/internal/devdb"
)

func newDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	d, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	pool, err := pgxpool.New(ctx, d.URL())
	if err != nil {
		_ = d.Close()
		t.Fatal(err)
	}
	return pool, func() { pool.Close(); _ = d.Close() }
}

func testMigrations() []Migration {
	return []Migration{
		{Version: "0001", Name: "a", UpSQL: "CREATE TABLE a(id int);", DownSQL: "DROP TABLE a;", Checksum: "c1"},
		{Version: "0002", Name: "b", UpSQL: "CREATE TABLE b(id int);", DownSQL: "DROP TABLE b;", Checksum: "c2"},
	}
}

func tableExists(t *testing.T, pool *pgxpool.Pool, name string) bool {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(),
		"SELECT count(*) FROM information_schema.tables WHERE table_name=$1", name).Scan(&n); err != nil {
		t.Fatalf("count table %s: %v", name, err)
	}
	return n > 0
}

func TestUpDownStatus(t *testing.T) {
	pool, done := newDB(t)
	defer done()
	ctx := context.Background()

	m := New(pool, testMigrations())

	if err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
	st, err := m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Applied) != 2 || len(st.Pending) != 0 {
		t.Fatalf("after up: %+v", st)
	}
	if !tableExists(t, pool, "a") || !tableExists(t, pool, "b") {
		t.Fatalf("tables a/b should exist after up")
	}

	if err := m.Down(ctx, 1); err != nil {
		t.Fatal(err)
	}
	st, err = m.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Applied) != 1 || len(st.Pending) != 1 {
		t.Fatalf("after down: %+v", st)
	}
	if st.Applied[0].Version != "0001" || st.Pending[0].Version != "0002" {
		t.Fatalf("after down wrong sets: applied=%+v pending=%+v", st.Applied, st.Pending)
	}
	// b dropped, a remains.
	if tableExists(t, pool, "b") {
		t.Fatalf("b not dropped")
	}
	if !tableExists(t, pool, "a") {
		t.Fatalf("a should still exist")
	}
}

func TestIdempotentUp(t *testing.T) {
	pool, done := newDB(t)
	defer done()
	ctx := context.Background()

	m := New(pool, testMigrations())
	if err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
	// A second Up with nothing pending must be a no-op.
	if err := m.Up(ctx); err != nil {
		t.Fatalf("second Up: %v", err)
	}
	st, _ := m.Status(ctx)
	if len(st.Applied) != 2 || len(st.Pending) != 0 {
		t.Fatalf("after double up: %+v", st)
	}
}

func TestTo(t *testing.T) {
	pool, done := newDB(t)
	defer done()
	ctx := context.Background()

	m := New(pool, testMigrations())

	// Up to 0001 only.
	if err := m.To(ctx, "0001"); err != nil {
		t.Fatal(err)
	}
	st, _ := m.Status(ctx)
	if len(st.Applied) != 1 || st.Applied[0].Version != "0001" {
		t.Fatalf("To(0001): %+v", st)
	}
	if !tableExists(t, pool, "a") || tableExists(t, pool, "b") {
		t.Fatalf("To(0001): expected a only")
	}

	// Forward to 0002.
	if err := m.To(ctx, "0002"); err != nil {
		t.Fatal(err)
	}
	st, _ = m.Status(ctx)
	if len(st.Applied) != 2 {
		t.Fatalf("To(0002): %+v", st)
	}

	// Back down to before any migration ("0000").
	if err := m.To(ctx, "0000"); err != nil {
		t.Fatal(err)
	}
	st, _ = m.Status(ctx)
	if len(st.Applied) != 0 || len(st.Pending) != 2 {
		t.Fatalf("To(0000): %+v", st)
	}
	if tableExists(t, pool, "a") || tableExists(t, pool, "b") {
		t.Fatalf("To(0000): all tables should be gone")
	}
}

func TestDrift(t *testing.T) {
	pool, done := newDB(t)
	defer done()
	ctx := context.Background()

	m := New(pool, testMigrations())
	if err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}

	// Reload with a changed checksum for 0001 to simulate a modified file.
	migs := testMigrations()
	migs[0].Checksum = "changed"
	m2 := New(pool, migs)

	st, err := m2.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Drift) != 1 || st.Drift[0] != "0001" {
		t.Fatalf("expected drift on 0001, got %+v", st.Drift)
	}
}

func TestDownNoDownSQL(t *testing.T) {
	pool, done := newDB(t)
	defer done()
	ctx := context.Background()

	migs := []Migration{
		{Version: "0001", Name: "a", UpSQL: "CREATE TABLE a(id int);", DownSQL: "", Checksum: "c1"},
	}
	m := New(pool, migs)
	if err := m.Up(ctx); err != nil {
		t.Fatal(err)
	}
	if err := m.Down(ctx, 1); err == nil {
		t.Fatal("expected error reverting migration with no down SQL")
	}
}
