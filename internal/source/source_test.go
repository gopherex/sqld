package source

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherex/sqld/pkg/config"
)

func TestResolveQueryDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.sql"), []byte("-- name: GetA :one\nSELECT 1;"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.sql"), []byte("-- name: GetB :one\nSELECT 2;"), 0o644)

	cfg := &config.Config{Queries: []config.Source{{
		Dir: dir,
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
	for _, u := range units {
		if u.Kind != KindQuery {
			t.Fatalf("expected KindQuery, got %v", u.Kind)
		}
	}
}

func TestResolveMigrationsOrdered(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "0002_b.sql"), []byte("ALTER TABLE a ADD c int;"), 0o644)
	os.WriteFile(filepath.Join(dir, "0001_a.sql"), []byte("CREATE TABLE a(id int);"), 0o644)
	cfg := &config.Config{Migrations: []config.MigrationSource{{Dir: dir}}}
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

func TestResolveSchemaDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "schema.sql"), []byte("CREATE TABLE things(id bigint PRIMARY KEY);"), 0o644)

	cfg := &config.Config{Schema: []config.Source{{Dir: dir}}}
	units, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 1 {
		t.Fatalf("want 1 schema unit, got %d", len(units))
	}
	if units[0].Kind != KindSchema {
		t.Fatalf("expected KindSchema, got %v", units[0].Kind)
	}
}

func TestResolveSchemaFile(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "schema.sql")
	os.WriteFile(p, []byte("CREATE TABLE widgets(id bigint PRIMARY KEY);"), 0o644)

	cfg := &config.Config{Schema: []config.Source{{File: p}}}
	units, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 1 || units[0].Kind != KindSchema {
		t.Fatalf("schema file unit: %+v", units)
	}
}

func TestResolveSchemaInline(t *testing.T) {
	sql := "CREATE TABLE foo(id bigint);"
	cfg := &config.Config{Schema: []config.Source{{Inline: sql}}}
	units, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 1 || units[0].Kind != KindSchema || units[0].SQL != sql {
		t.Fatalf("schema inline unit: %+v", units)
	}
}

func TestSplitMigration(t *testing.T) {
	up, down := splitMigration("-- sqld:up\nCREATE TABLE a();\n-- sqld:down\nDROP TABLE a;\n")
	if !strings.Contains(up, "CREATE TABLE a") || strings.Contains(up, "DROP TABLE") {
		t.Fatalf("up=%q", up)
	}
	if !strings.Contains(down, "DROP TABLE a") {
		t.Fatalf("down=%q", down)
	}
	up2, down2 := splitMigration("CREATE TABLE b();")
	if !strings.Contains(up2, "CREATE TABLE b") || down2 != "" {
		t.Fatalf("no-marker: up=%q down=%q", up2, down2)
	}
}

func TestResolveMigrationUpDown(t *testing.T) {
	dir := t.TempDir()
	content := "-- sqld:up\nCREATE TABLE c(id int);\n-- sqld:down\nDROP TABLE c;\n"
	os.WriteFile(filepath.Join(dir, "0001_c.sql"), []byte(content), 0o644)

	cfg := &config.Config{Migrations: []config.MigrationSource{{Dir: dir}}}
	units, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 1 {
		t.Fatalf("want 1 unit, got %d", len(units))
	}
	u := units[0]
	if !strings.Contains(u.SQL, "CREATE TABLE c") || strings.Contains(u.SQL, "DROP TABLE") {
		t.Fatalf("SQL (up) unexpected: %q", u.SQL)
	}
	if !strings.Contains(u.DownSQL, "DROP TABLE c") {
		t.Fatalf("DownSQL unexpected: %q", u.DownSQL)
	}
}
