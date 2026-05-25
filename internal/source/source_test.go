package source

import (
	"os"
	"path/filepath"
	"testing"

	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
)

func TestResolveSchemaDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE a(id int);"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.sql"), []byte("CREATE TABLE b(id int);"), 0o644)

	cfg := &configv1.Config{Sql: []*configv1.SqlSource{{
		Source: &configv1.SqlSource_Dir{Dir: dir},
		Kind:   configv1.SqlKind_SQL_KIND_SCHEMA,
	}}}
	units, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 {
		t.Fatalf("want 2 units, got %d", len(units))
	}
	if filepath.Base(units[0].Path) != "a.sql" {
		t.Fatalf("order: %s", units[0].Path)
	}
}

func TestResolveMigrationsOrdered(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "0002_b.sql"), []byte("ALTER TABLE a ADD c int;"), 0o644)
	os.WriteFile(filepath.Join(dir, "0001_a.sql"), []byte("CREATE TABLE a(id int);"), 0o644)
	cfg := &configv1.Config{Migrations: []*configv1.MigrationSource{{Dir: dir}}}
	units, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 || units[0].Version != "0001_a" || units[1].Version != "0002_b" {
		t.Fatalf("migration order/version: %+v", units)
	}
	if units[0].Kind != KindMigrationUp {
		t.Fatalf("kind: %v", units[0].Kind)
	}
}
