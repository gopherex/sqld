package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yaroher/sqld/pkg/devdb"
)

// TestMigrateGenerateDevURLIsolation exercises `migrate generate --dev-url
// <DSN>`, where both the desired (schema.sql) and current (migrations) catalogs
// are built against the SAME external database. With the bug, the two
// buildCatalog calls shared one polluted DB, so the "current" state was
// contaminated by the "desired" apply and the diff was wrong. The fix resets
// the external DB before each apply.
//
// Scenario: schema.sql declares table `users`; the existing migration created a
// different table `orders`. The correct diff (current=orders -> desired=users)
// must DROP orders and CREATE users.
func TestMigrateGenerateDevURLIsolation(t *testing.T) {
	ctx := context.Background()
	dev, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer dev.Close()
	devURL := dev.URL()

	got := runGenerateScenario(t, devURL)

	// Sanity: the same scenario via the fresh-container path (no --dev-url)
	// yields the same correct diff.
	want := runGenerateScenario(t, "")

	if normalizeMigration(got) != normalizeMigration(want) {
		t.Fatalf("dev-url diff differs from fresh-container diff\n--- dev-url ---\n%s\n--- fresh ---\n%s", got, want)
	}

	if !strings.Contains(got, "CREATE TABLE") || !strings.Contains(got, "users") {
		t.Fatalf("expected CREATE TABLE users in generated migration:\n%s", got)
	}
	if !strings.Contains(got, "DROP TABLE") || !strings.Contains(got, "orders") {
		t.Fatalf("expected DROP TABLE orders in generated migration:\n%s", got)
	}
}

// runGenerateScenario writes a config with schema.sql (users) + one migration
// (orders), runs `migrate generate` (optionally with --dev-url), and returns
// the body of the single generated migration file.
func runGenerateScenario(t *testing.T, devURL string) string {
	t.Helper()
	root := t.TempDir()
	migDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}

	schemaFile := filepath.Join(root, "schema.sql")
	if err := os.WriteFile(schemaFile,
		[]byte("CREATE TABLE users (id bigint PRIMARY KEY, email text NOT NULL);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Existing migration creates a DIFFERENT table.
	if err := os.WriteFile(filepath.Join(migDir, "20240101000000_init.sql"),
		[]byte("-- sqld:up\nCREATE TABLE orders (id bigint PRIMARY KEY);\n-- sqld:down\nDROP TABLE orders;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, root, migDir, schemaFile)

	args := []string{"migrate", "generate", "drift", "-c", cfgPath}
	if devURL != "" {
		args = append(args, "--dev-url", devURL)
	}
	var out, errb bytes.Buffer
	if code := run(args, &out, &errb); code != 0 {
		t.Fatalf("generate (dev-url=%q): exit %d\nstdout=%q\nstderr=%q", devURL, code, out.String(), errb.String())
	}

	entries, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatal(err)
	}
	var generated string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), "_drift.sql") {
			generated = filepath.Join(migDir, e.Name())
		}
	}
	if generated == "" {
		t.Fatalf("no generated _drift.sql; entries=%v", entries)
	}
	body, err := os.ReadFile(generated)
	if err != nil {
		t.Fatal(err)
	}
	return string(body)
}

// normalizeMigration strips surrounding whitespace so two runs can be compared
// by content.
func normalizeMigration(s string) string {
	return strings.TrimSpace(s)
}
