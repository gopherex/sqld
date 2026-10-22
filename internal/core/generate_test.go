package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherex/sqld/pkg/config"
)

func TestGenerateWritesFiles(t *testing.T) {
	migDir := writeMigrations(t, "CREATE TABLE users(id bigint primary key, email text not null);")
	outDir := filepath.Join(t.TempDir(), "gen")

	// build the fake plugin binary
	bin := filepath.Join(t.TempDir(), "fakeplugin")
	cmd := exec.Command("go", "build", "-o", bin, "../plugin/testdata/fakeplugin")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build fake plugin: %v", err)
	}

	cfg := &config.Config{
		Engine:     config.EnginePostgreSQL,
		Migrations: []config.MigrationSource{{Dir: migDir}},
		Plugins: []config.PluginConfig{{
			Name:   "fake",
			Binary: bin,
			Out:    outDir,
		}},
	}
	if err := Generate(cfg); err != nil {
		t.Fatal(err)
	}

	b, err := os.ReadFile(filepath.Join(outDir, "out.txt"))
	if err != nil {
		t.Fatalf("read generated file: %v", err)
	}
	if string(b) != "hi" {
		t.Fatalf("contents=%q", b)
	}
}

func TestGenerateRejectsQueryErrorsBeforePlugins(t *testing.T) {
	for _, body := range []string{`SELECT sqlc.embed(t) FROM t`, `SELECT sqlc.narg(42)`, `SELECT sqlc.arg('x') OVER ()`, `not valid sql !!!`} {
		t.Run(body, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "queries.sql")
			if err := os.WriteFile(path, []byte("-- name: Bad :one\n"+body), 0600); err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{Engine: config.EnginePostgreSQL,
				Schema:  []config.Source{{Inline: "CREATE TABLE t (id bigint);"}},
				Queries: []config.Source{{File: path}},
				// Opening this plugin would fail with an unrelated error. The query
				// diagnostic must take precedence, leaving existing output untouched.
				Plugins: []config.PluginConfig{{Name: "go", Binary: filepath.Join(dir, "missing-plugin"), Out: dir}},
			}
			output := filepath.Join(dir, "queries.go")
			if err := os.WriteFile(output, []byte("existing output"), 0600); err != nil {
				t.Fatal(err)
			}
			err := Generate(cfg)
			if err == nil || !strings.Contains(err.Error(), path+": query Bad:") {
				t.Fatalf("err=%v", err)
			}
			got, err := os.ReadFile(output)
			if err != nil || string(got) != "existing output" {
				t.Fatalf("output=%q err=%v", got, err)
			}
		})
	}
}
