package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gopherex/sqld/pkg/config"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

func buildFakePlugin(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "fakeplugin")
	cmd := exec.Command("go", "build", "-o", bin, "./testdata/fakeplugin")
	cmd.Dir = "."
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("build fake plugin: %v", err)
	}
	return bin
}

func TestBinaryRunner(t *testing.T) {
	bin := buildFakePlugin(t)
	r, err := Open(config.PluginConfig{
		Name:   "fake",
		Binary: bin,
		Out:    "./gen",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()

	info, err := r.GetInfo(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if info.GetName() != "fake" {
		t.Fatalf("info name=%q", info.GetName())
	}

	resp, err := r.Generate(context.Background(), &pluginv1.GenerateRequest{})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.GetFiles()) != 1 || resp.GetFiles()[0].GetPath() != "out.txt" {
		t.Fatalf("files=%+v", resp.GetFiles())
	}
	if string(resp.GetFiles()[0].GetContents()) != "hi" {
		t.Fatalf("contents=%q", resp.GetFiles()[0].GetContents())
	}
}
