package example

import (
	"context"
	"testing"

	"github.com/aarondl/opt/null"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherex/sqld/example/gen/db"
	"github.com/gopherex/sqld/pkg/devdb"
)

// TestOptModeCompositeScanGlue exercises the exact runtime pattern sqld-gen-go's
// nullMode:opt emits for a nullable composite result column: scan into a
// *Composite temp, then wrap with null.FromPtr. This proves the glue handles
// both the present and NULL cases (pgx can't carry a non-null composite through
// null.Val's sql.Scanner path, which is why the temp pointer is used). Docker-gated.
func TestOptModeCompositeScanGlue(t *testing.T) {
	ctx := context.Background()

	dev, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer dev.Close()

	if err := dev.Apply(ctx, `CREATE TYPE address AS (street text, city text, zip text);`); err != nil {
		t.Fatalf("create type: %v", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, mustConfig(t, dev.URL()))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// Present value: scan into the *Composite temp, wrap with null.FromPtr.
	var tmp *db.AppAddress
	if err := pool.QueryRow(ctx, `SELECT row('Main St', 'Springfield', '12345')::address`).Scan(&tmp); err != nil {
		t.Fatalf("composite scan (non-null): %v", err)
	}
	got := null.FromPtr(tmp)
	if !got.IsValue() {
		t.Fatal("expected set value")
	}
	if v := got.GetOrZero(); v.Street != "Main St" || v.City != "Springfield" || v.Zip != "12345" {
		t.Fatalf("composite = %+v", v)
	}

	// NULL: pgx sets the pointer to nil; null.FromPtr yields an unset null.Val.
	tmp = nil
	if err := pool.QueryRow(ctx, `SELECT null::address`).Scan(&tmp); err != nil {
		t.Fatalf("composite scan (null): %v", err)
	}
	if null.FromPtr(tmp).IsValue() {
		t.Fatal("expected unset value for NULL composite")
	}
}

func mustConfig(t *testing.T, url string) *pgxpool.Config {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		t.Fatal(err)
	}
	// Register the public "address" composite per connection so *db.AppAddress
	// (a pgtype CompositeIndexScanner) can decode it.
	cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error {
		dt, err := c.LoadType(ctx, "address")
		if err != nil {
			return err
		}
		c.TypeMap().RegisterType(dt)
		return nil
	}
	return cfg
}
