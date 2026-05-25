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

func tempProject(t *testing.T) *config.Config {
	t.Helper()
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "schema", "001.sql"), `
		CREATE TABLE users(id bigint primary key, email text not null);
		CREATE TABLE orders(id bigint primary key, user_id bigint not null references users(id));
	`)
	writeFile(t, filepath.Join(root, "queries", "q.sql"),
		"-- name: GetUser :one\nSELECT id, email FROM users WHERE id = $1;\n")
	return &config.Config{
		Engine: config.EnginePostgreSQL,
		SQL: []config.SQLSource{
			{Dir: filepath.Join(root, "schema"), Kind: config.SQLSchema},
			{Dir: filepath.Join(root, "queries"), Kind: config.SQLQuery},
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
