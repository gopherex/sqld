package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/yaroher/sqld/pkg/config"
)

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeMigrations writes a single 0001.sql migration file into a temp dir and
// returns the directory path.
func writeMigrations(t *testing.T, sql string) string {
	t.Helper()
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "0001.sql"), sql)
	return dir
}

func tempProject(t *testing.T) *config.Config {
	t.Helper()
	migDir := writeMigrations(t, `
		CREATE TABLE users(id bigint primary key, email text not null);
		CREATE TABLE orders(id bigint primary key, user_id bigint not null references users(id));
	`)
	return &config.Config{
		Engine:     config.EnginePostgreSQL,
		Migrations: []config.MigrationSource{{Dir: migDir}},
		Queries: []config.Source{
			{Inline: "-- name: GetUser :one\nSELECT id, email FROM users WHERE id = $1;\n"},
		},
	}
}

func TestCollectCatalog(t *testing.T) {
	cat, err := Collect(tempProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.GetSchemas()) != 1 || len(cat.GetSchemas()[0].GetTables()) != 2 {
		t.Fatalf("schemas/tables wrong: %+v", cat.GetSchemas())
	}
	if len(cat.GetRelationships()) == 0 {
		t.Fatal("no relationships derived")
	}
}

func TestGatherQueries(t *testing.T) {
	r, err := Gather(tempProject(t))
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

// tempProjectWithSchema builds a Config that uses a declarative schema source
// (KindSchema) instead of migrations as the catalog source of truth.
func tempProjectWithSchema(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Engine: config.EnginePostgreSQL,
		Schema: []config.Source{
			{Inline: `
				CREATE TABLE users(id bigint primary key, email text not null);
				CREATE TABLE orders(id bigint primary key, user_id bigint not null references users(id));
			`},
		},
		Queries: []config.Source{
			{Inline: "-- name: GetUser :one\nSELECT id, email FROM users WHERE id = $1;\n"},
		},
	}
}

func TestCollectCatalogFromSchema(t *testing.T) {
	cat, err := Collect(tempProjectWithSchema(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.GetSchemas()) != 1 || len(cat.GetSchemas()[0].GetTables()) != 2 {
		t.Fatalf("schemas/tables wrong: %+v", cat.GetSchemas())
	}
	if len(cat.GetRelationships()) == 0 {
		t.Fatal("no relationships derived from schema source")
	}
}

func TestGatherSchemaSourcePreferredOverMigrations(t *testing.T) {
	// When schema sources are present, catalog is built from them.
	// Migrations are still resolved into the Migration list.
	migDir := writeMigrations(t, "CREATE TABLE legacy(id bigint primary key);")
	cfg := &config.Config{
		Engine: config.EnginePostgreSQL,
		Schema: []config.Source{
			{Inline: "CREATE TABLE canonical(id bigint primary key);"},
		},
		Migrations: []config.MigrationSource{{Dir: migDir}},
	}
	r, err := Gather(cfg)
	if err != nil {
		t.Fatal(err)
	}
	// Catalog must contain "canonical" (from schema), not "legacy" (from migrations).
	if len(r.Catalog.GetSchemas()) != 1 {
		t.Fatalf("expected 1 schema, got %d", len(r.Catalog.GetSchemas()))
	}
	tables := r.Catalog.GetSchemas()[0].GetTables()
	if len(tables) != 1 || tables[0].GetName().GetName() != "canonical" {
		t.Fatalf("catalog table wrong: %+v", tables)
	}
	// Migrations list must still be populated.
	if len(r.Migrations) != 1 {
		t.Fatalf("expected 1 migration, got %d", len(r.Migrations))
	}
}
