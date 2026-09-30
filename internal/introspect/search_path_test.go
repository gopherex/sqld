package introspect

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/gopherex/sqld/pkg/devdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestIntrospectSearchPathScoped(t *testing.T) {
	ctx := context.Background()
	dev, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer dev.Close()
	if err := dev.Apply(ctx, `CREATE SCHEMA "Mixed Case"; CREATE TABLE "Mixed Case".items(id bigserial); CREATE VIEW "Mixed Case".feed AS SELECT * FROM "Mixed Case".items`); err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, dev.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, `SET search_path = "Mixed Case", public`); err != nil {
		t.Fatal(err)
	}
	check := func(t *testing.T, db DBTX) {
		t.Helper()
		var before, after string
		if err := db.QueryRow(ctx, `SHOW search_path`).Scan(&before); err != nil {
			t.Fatal(err)
		}
		cat, err := Introspect(ctx, db, []string{"Mixed Case"})
		if err != nil {
			t.Fatal(err)
		}
		if err := db.QueryRow(ctx, `SHOW search_path`).Scan(&after); err != nil {
			t.Fatal(err)
		}
		if after != before {
			t.Fatalf("search_path leaked: %q -> %q", before, after)
		}
		schema := findSchema(t, cat, "Mixed Case")
		if sql := schema.GetViews()[0].GetQuery().GetRawSql(); !strings.Contains(sql, `"Mixed Case".items`) {
			t.Fatalf("unqualified view: %s", sql)
		}
		if sql := schema.GetTables()[0].GetColumns()[0].GetDefaultExpr().GetRawSql(); !strings.Contains(sql, `'"Mixed Case".items_id_seq'::regclass`) {
			t.Fatalf("unqualified default: %s", sql)
		}
	}
	t.Run("connection", func(t *testing.T) { check(t, conn) })
	t.Run("error_restores_session", func(t *testing.T) {
		_, err := Introspect(ctx, failingIntrospectDB{DBTX: conn}, nil)
		if !errors.Is(err, errIntrospectQuery) {
			t.Fatalf("expected injected error, got %v", err)
		}
		var path string
		if err := conn.QueryRow(ctx, `SHOW search_path`).Scan(&path); err != nil {
			t.Fatal(err)
		}
		if path != `"Mixed Case", public` {
			t.Fatalf("search_path leaked on error: %s", path)
		}
	})
	t.Run("transaction", func(t *testing.T) {
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer tx.Rollback(ctx)
		if _, err := tx.Exec(ctx, `CREATE TABLE "Mixed Case".uncommitted(id int)`); err != nil {
			t.Fatal(err)
		}
		check(t, tx)
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT to_regclass('"Mixed Case".uncommitted') IS NOT NULL`).Scan(&exists); err != nil || !exists {
			t.Fatalf("caller transaction lost: %v", err)
		}
	})
	t.Run("pool", func(t *testing.T) {
		cfg, err := pgxpool.ParseConfig(dev.URL())
		if err != nil {
			t.Fatal(err)
		}
		cfg.MaxConns = 1
		cfg.ConnConfig.RuntimeParams["search_path"] = `"Mixed Case", public`
		pool, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		check(t, pool)
	})
}

var errIntrospectQuery = errors.New("injected introspection query failure")

type failingIntrospectDB struct{ DBTX }

func (d failingIntrospectDB) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := d.DBTX.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return failingIntrospectTx{Tx: tx}, nil
}

type failingIntrospectTx struct{ pgx.Tx }

func (tx failingIntrospectTx) Query(context.Context, string, ...any) (pgx.Rows, error) {
	return nil, errIntrospectQuery
}
