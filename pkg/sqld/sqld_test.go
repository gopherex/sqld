package sqld_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yaroher/sqld/pkg/config"
	"github.com/yaroher/sqld/pkg/sqld"
)

// writeTempMigrations creates a temp dir with a single 0001.sql migration file
// containing the provided SQL, and returns the directory path.
func writeTempMigrations(t *testing.T, sql string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "0001.sql")
	if err := os.WriteFile(p, []byte(sql), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}
	return dir
}

func TestCollect(t *testing.T) {
	migDir := writeTempMigrations(t, `
		CREATE TABLE users(id bigint primary key, email text not null);
		CREATE TABLE orders(id bigint primary key, user_id bigint not null references users(id));
	`)
	cfg := &config.Config{
		Engine:     config.EnginePostgreSQL,
		Migrations: []config.MigrationSource{{Dir: migDir}},
	}
	cat, err := sqld.Collect(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.GetSchemas()) != 1 || len(cat.GetSchemas()[0].GetTables()) != 2 {
		t.Fatalf("schemas/tables wrong: %+v", cat.GetSchemas())
	}
	if len(cat.GetRelationships()) == 0 {
		t.Fatal("expected derived relationships")
	}
}

func TestCollectAll(t *testing.T) {
	migDir := writeTempMigrations(t, "CREATE TABLE users(id bigint primary key, email text not null);")
	cfg := &config.Config{
		Engine:     config.EnginePostgreSQL,
		Migrations: []config.MigrationSource{{Dir: migDir}},
		Queries: []config.Source{
			{Inline: "-- name: GetUser :one\nSELECT id, email FROM users WHERE id = $1;\n"},
		},
	}
	r, err := sqld.CollectAll(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Queries) != 1 || r.Queries[0].GetName() != "GetUser" {
		t.Fatalf("queries=%+v", r.Queries)
	}
	if len(r.Queries[0].GetColumns()) != 2 {
		t.Fatalf("query columns=%d", len(r.Queries[0].GetColumns()))
	}
}
