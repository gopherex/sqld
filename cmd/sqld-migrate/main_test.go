package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yaroher/sqld/internal/devdb"
)

// TestGenVersionUnique (M1) verifies the version stamp has millisecond
// resolution, so two generates within the same second produce distinct,
// sortable versions. The old second-resolution format collided.
func TestGenVersionUnique(t *testing.T) {
	base := time.Date(2026, 5, 26, 10, 30, 15, 0, time.UTC)

	v0 := genVersion(base)                                  // .000 ms
	v1 := genVersion(base.Add(123 * time.Millisecond))      // .123 ms
	v2 := genVersion(base.Add(1 * time.Millisecond))        // .001 ms
	sameSec := genVersion(base.Add(900 * time.Millisecond)) // .900 ms

	// All-digit and 17 chars (yyyymmddHHMMSS + 3 ms digits).
	for _, v := range []string{v0, v1, v2, sameSec} {
		if len(v) != 17 {
			t.Fatalf("version %q length=%d, want 17", v, len(v))
		}
		for _, r := range v {
			if r < '0' || r > '9' {
				t.Fatalf("version %q is not all-digit", v)
			}
		}
	}

	// Distinct within the same second.
	if v0 == v1 || v0 == v2 || v1 == sameSec {
		t.Fatalf("versions within one second collided: %q %q %q %q", v0, v1, v2, sameSec)
	}
	// Sortable: later millisecond sorts lexicographically after earlier.
	if !(v0 < v2 && v2 < v1 && v1 < sameSec) {
		t.Fatalf("versions not lexicographically sortable: %q %q %q %q", v0, v2, v1, sameSec)
	}
}

func TestUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run(nil, &out, &errb); code != 2 {
		t.Fatalf("no args: got exit %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "Usage") {
		t.Fatalf("no args: expected usage on stderr, got %q", errb.String())
	}

	out.Reset()
	errb.Reset()
	if code := run([]string{"frobnicate"}, &out, &errb); code != 2 {
		t.Fatalf("unknown subcommand: got exit %d, want 2", code)
	}
	if !strings.Contains(errb.String(), "unknown subcommand") {
		t.Fatalf("unknown subcommand: expected error on stderr, got %q", errb.String())
	}
}

// writeConfig writes a minimal sqld.yaml pointing migrations at dir and
// (optionally) schema at schemaFile, and returns the config path.
func writeConfig(t *testing.T, root, migDir, schemaFile string) string {
	t.Helper()
	var b strings.Builder
	b.WriteString("version: \"1\"\nengine: postgresql\n")
	if schemaFile != "" {
		b.WriteString("schema:\n  - file: " + schemaFile + "\n")
	}
	b.WriteString("migrations:\n  - dir: " + migDir + "\n")
	cfgPath := filepath.Join(root, "sqld.yaml")
	if err := os.WriteFile(cfgPath, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return cfgPath
}

func TestValidate(t *testing.T) {
	root := t.TempDir()
	migDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}

	good := filepath.Join(migDir, "001_good.sql")
	if err := os.WriteFile(good, []byte("-- sqld:up\nCREATE TABLE good(id int);\n-- sqld:down\nDROP TABLE good;\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	bad := filepath.Join(migDir, "002_bad.sql")
	if err := os.WriteFile(bad, []byte("-- sqld:up\nCREATE TABBLE bad (this is not sql;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, root, migDir, "")

	var out, errb bytes.Buffer
	code := run([]string{"validate", "-c", cfgPath}, &out, &errb)
	if code == 0 {
		t.Fatalf("validate: expected non-zero exit, got 0\nstdout=%q\nstderr=%q", out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "002_bad") {
		t.Fatalf("validate: expected error to name 002_bad, got %q", errb.String())
	}
}

func TestLint(t *testing.T) {
	root := t.TempDir()
	migDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// A destructive migration: DROP TABLE -> error finding, non-zero exit.
	destructive := filepath.Join(migDir, "001_drop_users.sql")
	if err := os.WriteFile(destructive, []byte("-- sqld:up\nDROP TABLE users;\n-- sqld:down\nCREATE TABLE users(id int);\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, root, migDir, "")

	var out, errb bytes.Buffer
	code := run([]string{"lint", "-c", cfgPath}, &out, &errb)
	if code == 0 {
		t.Fatalf("lint: expected non-zero exit on destructive migration, got 0\nstdout=%q\nstderr=%q", out.String(), errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "destructive-drop") {
		t.Fatalf("lint: expected destructive-drop finding, got %q", s)
	}
	if !strings.Contains(s, "users") {
		t.Fatalf("lint: expected the dropped table name in output, got %q", s)
	}
	if !strings.Contains(s, "001") {
		t.Fatalf("lint: expected the migration version in output, got %q", s)
	}
}

func TestLintClean(t *testing.T) {
	root := t.TempDir()
	migDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}
	clean := filepath.Join(migDir, "001_create_users.sql")
	if err := os.WriteFile(clean, []byte("-- sqld:up\nCREATE TABLE users(id int);\n-- sqld:down\nDROP TABLE users;\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, root, migDir, "")

	var out, errb bytes.Buffer
	code := run([]string{"lint", "-c", cfgPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("lint: expected zero exit on clean migration, got %d\nstdout=%q\nstderr=%q", code, out.String(), errb.String())
	}
}

func TestHash(t *testing.T) {
	root := t.TempDir()
	migDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migDir, "001_a.sql"), []byte("CREATE TABLE a(id int);\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(migDir, "002_b.sql"), []byte("CREATE TABLE b(id int);\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, root, migDir, "")

	var out, errb bytes.Buffer
	code := run([]string{"hash", "-c", cfgPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("hash: got exit %d, want 0\nstderr=%q", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "001") || !strings.Contains(s, "002") {
		t.Fatalf("hash: expected both versions, got %q", s)
	}
	// Each line should carry a 64-hex-char sha256 checksum.
	for _, line := range strings.Split(strings.TrimSpace(s), "\n") {
		fields := strings.Fields(line)
		if len(fields) != 2 || len(fields[1]) != 64 {
			t.Fatalf("hash: malformed line %q", line)
		}
	}
}

func TestGenerate(t *testing.T) {
	// Docker gate: if a dev container cannot start, skip.
	ctx := context.Background()
	probe, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	_ = probe.Close()

	root := t.TempDir()
	migDir := filepath.Join(root, "migrations")
	if err := os.MkdirAll(migDir, 0o755); err != nil {
		t.Fatal(err)
	}

	schemaFile := filepath.Join(root, "schema.sql")
	if err := os.WriteFile(schemaFile, []byte("CREATE TABLE users (id bigint PRIMARY KEY, email text NOT NULL);\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfgPath := writeConfig(t, root, migDir, schemaFile)

	var out, errb bytes.Buffer
	code := run([]string{"generate", "add_users", "-c", cfgPath}, &out, &errb)
	if code != 0 {
		t.Fatalf("generate: got exit %d, want 0\nstdout=%q\nstderr=%q", code, out.String(), errb.String())
	}

	entries, err := os.ReadDir(migDir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("generate: expected 1 migration file, got %d", len(entries))
	}
	gen := filepath.Join(migDir, entries[0].Name())
	if !strings.HasSuffix(entries[0].Name(), "_add_users.sql") {
		t.Fatalf("generate: unexpected file name %q", entries[0].Name())
	}
	body, err := os.ReadFile(gen)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "CREATE TABLE") {
		t.Fatalf("generate: expected CREATE TABLE in migration, got:\n%s", body)
	}
	if !strings.Contains(string(body), "-- sqld:up") || !strings.Contains(string(body), "-- sqld:down") {
		t.Fatalf("generate: expected up/down markers, got:\n%s", body)
	}
}
