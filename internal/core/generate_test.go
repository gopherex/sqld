package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yaroher/sqld/pkg/config"
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
