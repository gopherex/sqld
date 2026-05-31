package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/pkg/config"
)

func TestCollectCommand(t *testing.T) {
	// Write a temp migrations dir with the schema.
	migDir := t.TempDir()
	migFile := filepath.Join(migDir, "0001.sql")
	if err := os.WriteFile(migFile, []byte("CREATE TABLE users(id bigint primary key, email text not null);"), 0o644); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	testConfigYAML := fmt.Sprintf(`migrations:
  - dir: %q
`, migDir)

	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sqld.yaml")
	if err := os.WriteFile(cfgPath, []byte(testConfigYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := run([]string{"collect", "-c", cfgPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("run returned %d; stderr: %s", code, errBuf.String())
	}
	outStr := out.String()
	if !strings.Contains(outStr, "users") {
		t.Errorf("stdout does not contain 'users':\n%s", outStr)
	}
}

func TestUsageOnNoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(nil, &out, &errBuf)
	if code != 2 {
		t.Errorf("run(nil) returned %d; want 2", code)
	}
}

func TestInit(t *testing.T) {
	tmpDir := t.TempDir()
	var out, errBuf bytes.Buffer
	code := run([]string{"init", tmpDir}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("run returned %d; stderr: %s", code, errBuf.String())
	}

	// Assert all expected files exist.
	expectedFiles := []string{
		filepath.Join(tmpDir, "sqld.yaml"),
		filepath.Join(tmpDir, "schema.sql"),
		filepath.Join(tmpDir, "queries", "authors.sql"),
		filepath.Join(tmpDir, "migrations", ".gitkeep"),
	}
	for _, f := range expectedFiles {
		if _, err := os.Stat(f); err != nil {
			t.Errorf("expected file %s to exist: %v", f, err)
		}
	}

	// Assert the generated sqld.yaml loads via config.Load and has correct counts.
	cfgPath := filepath.Join(tmpDir, "sqld.yaml")
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("config.Load(%s): %v", cfgPath, err)
	}
	if len(cfg.Schema) != 1 {
		t.Errorf("schema sources: got %d, want 1", len(cfg.Schema))
	}
	if len(cfg.Queries) != 1 {
		t.Errorf("queries sources: got %d, want 1", len(cfg.Queries))
	}
	if len(cfg.Plugins) != 1 {
		t.Errorf("plugins: got %d, want 1", len(cfg.Plugins))
	}

	// Assert schema.sql parses without error via parse.Statements.
	schemaPath := filepath.Join(tmpDir, "schema.sql")
	schemaSQL, err := os.ReadFile(schemaPath)
	if err != nil {
		t.Fatalf("read %s: %v", schemaPath, err)
	}
	stmts, err := parse.Statements(string(schemaSQL))
	if err != nil {
		t.Fatalf("parse.Statements(schema.sql): %v", err)
	}
	if len(stmts) == 0 {
		t.Error("parse.Statements(schema.sql): got 0 statements, want at least 1")
	}

	// Assert stdout contains the expected "next:" hint.
	outStr := out.String()
	if !strings.Contains(outStr, "sqld generate") {
		t.Errorf("stdout missing 'sqld generate'; got:\n%s", outStr)
	}
}

func TestInitNoClobber(t *testing.T) {
	tmpDir := t.TempDir()

	// Pre-create sqld.yaml with sentinel content.
	cfgPath := filepath.Join(tmpDir, "sqld.yaml")
	original := []byte("# existing config\n")
	if err := os.WriteFile(cfgPath, original, 0o644); err != nil {
		t.Fatalf("pre-create sqld.yaml: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := run([]string{"init", tmpDir}, &out, &errBuf)
	if code == 0 {
		t.Fatal("run returned 0; want non-zero (should refuse to clobber)")
	}

	// Confirm sqld.yaml was not overwritten.
	got, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read sqld.yaml after refused init: %v", err)
	}
	if string(got) != string(original) {
		t.Errorf("sqld.yaml was overwritten; got:\n%s", got)
	}
}
