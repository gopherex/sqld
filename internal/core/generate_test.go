package core

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/yaroher/sqld/pkg/config"
)

func TestGenerateWritesFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "schema", "001.sql"),
		"CREATE TABLE users(id bigint primary key, email text not null);")
	outDir := filepath.Join(root, "gen")

	// build the fake plugin binary
	bin := filepath.Join(t.TempDir(), "fakeplugin")
	cmd := exec.Command("go", "build", "-o", bin, "../plugin/testdata/fakeplugin")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build fake plugin: %v", err)
	}

	cfg := &config.Config{
		Engine: config.EnginePostgreSQL,
		SQL: []config.SQLSource{{
			Dir:  filepath.Join(root, "schema"),
			Kind: config.SQLSchema,
		}},
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
