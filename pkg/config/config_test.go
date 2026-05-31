package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gopherex/sqld/pkg/config"
)

const basicYAML = `
version: "1"
engine: postgresql
options:
  defaultSchema: public
schema:
  - file: ./schema.sql
queries:
  - dir: ./queries
migrations:
  - dir: ./migrations
plugins:
  - name: go
    binary: ./bin/sqld-gen-go
    out: ./gen/db
    options:
      package: db
`

func writeTempConfig(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "sqld.yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp config: %v", err)
	}
	return p
}

func TestLoadBasic(t *testing.T) {
	p := writeTempConfig(t, basicYAML)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Engine != config.EnginePostgreSQL {
		t.Errorf("engine = %q; want %q", cfg.Engine, config.EnginePostgreSQL)
	}
	if cfg.Options.DefaultSchema != "public" {
		t.Errorf("defaultSchema = %q; want %q", cfg.Options.DefaultSchema, "public")
	}
	if len(cfg.Schema) != 1 {
		t.Fatalf("len(schema) = %d; want 1", len(cfg.Schema))
	}
	if cfg.Schema[0].File == "" {
		t.Error("schema[0].File should be set")
	}
	if len(cfg.Queries) != 1 {
		t.Fatalf("len(queries) = %d; want 1", len(cfg.Queries))
	}
	if cfg.Queries[0].Dir == "" {
		t.Error("queries[0].Dir should be set")
	}
	if len(cfg.Migrations) != 1 {
		t.Fatalf("len(migrations) = %d; want 1", len(cfg.Migrations))
	}
	if len(cfg.Plugins) != 1 || cfg.Plugins[0].Name != "go" {
		t.Fatalf("plugins = %+v", cfg.Plugins)
	}
	pkg, ok := cfg.Plugins[0].Options["package"]
	if !ok || pkg != "db" {
		t.Errorf("plugin options[package] = %v; want %q", pkg, "db")
	}
	// Enabled nil => IsEnabled() == true
	if !cfg.Plugins[0].IsEnabled() {
		t.Error("plugin should be enabled by default")
	}
}

func TestValidateDefaults(t *testing.T) {
	// Engine and defaultSchema defaults when omitted.
	y := `
migrations:
  - dir: ./migrations
`
	p := writeTempConfig(t, y)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Engine != config.EnginePostgreSQL {
		t.Errorf("default engine = %q; want %q", cfg.Engine, config.EnginePostgreSQL)
	}
	if cfg.Options.DefaultSchema != "public" {
		t.Errorf("default defaultSchema = %q; want %q", cfg.Options.DefaultSchema, "public")
	}
	// Glob default for dir query source.
	y2 := `
queries:
  - dir: ./queries
`
	p2 := writeTempConfig(t, y2)
	cfg2, err := config.Load(p2)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg2.Queries[0].Glob != "*.sql" {
		t.Errorf("default glob = %q; want %q", cfg2.Queries[0].Glob, "*.sql")
	}
}

func TestValidateErrorBadEngine(t *testing.T) {
	y := `
engine: mysql
queries:
  - inline: "SELECT 1;"
`
	p := writeTempConfig(t, y)
	_, err := config.Load(p)
	if err == nil {
		t.Fatal("expected error for bad engine")
	}
}

func TestValidateErrorTwoSources(t *testing.T) {
	y := `
queries:
  - file: ./a.sql
    dir: ./queries
`
	p := writeTempConfig(t, y)
	_, err := config.Load(p)
	if err == nil {
		t.Fatal("expected error for two sources set")
	}
}

func TestValidateErrorMissingOut(t *testing.T) {
	y := `
queries:
  - inline: "SELECT 1;"
plugins:
  - name: go
    binary: ./bin/sqld-gen-go
`
	p := writeTempConfig(t, y)
	_, err := config.Load(p)
	if err == nil {
		t.Fatal("expected error for missing out")
	}
}

func TestValidateErrorMissingMigrationDir(t *testing.T) {
	y := `
migrations:
  - glob: "*.sql"
`
	p := writeTempConfig(t, y)
	_, err := config.Load(p)
	if err == nil {
		t.Fatal("expected error for missing migration dir")
	}
}

func TestValidateGlobDefault(t *testing.T) {
	y := `
migrations:
  - dir: ./mig
`
	p := writeTempConfig(t, y)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Migrations[0].Glob != "*.sql" {
		t.Errorf("migration glob = %q; want %q", cfg.Migrations[0].Glob, "*.sql")
	}
}

func TestLoadSchemaSource(t *testing.T) {
	y := `
schema:
  - file: ./schema.sql
migrations:
  - dir: ./migrations
`
	p := writeTempConfig(t, y)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Schema) != 1 {
		t.Fatalf("len(schema) = %d; want 1", len(cfg.Schema))
	}
	if cfg.Schema[0].File != "./schema.sql" {
		t.Errorf("schema[0].File = %q; want %q", cfg.Schema[0].File, "./schema.sql")
	}
}

func TestLoadSchemaDirGlobDefault(t *testing.T) {
	y := `
schema:
  - dir: ./ddl
`
	p := writeTempConfig(t, y)
	cfg, err := config.Load(p)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Schema[0].Glob != "*.sql" {
		t.Errorf("schema dir glob = %q; want %q", cfg.Schema[0].Glob, "*.sql")
	}
}

func TestValidateErrorSchemaTwoSources(t *testing.T) {
	y := `
schema:
  - file: ./schema.sql
    dir: ./ddl
`
	p := writeTempConfig(t, y)
	_, err := config.Load(p)
	if err == nil {
		t.Fatal("expected error for schema source with two fields set")
	}
}
