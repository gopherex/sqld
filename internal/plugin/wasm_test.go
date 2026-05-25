package plugin

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

func buildFakeWasm(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "fake.wasm")
	cmd := exec.Command("go", "build", "-o", out, "./testdata/fakeplugin")
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot build wasip1 fake plugin (skipping wasm test): %v", err)
	}
	return out
}

func TestWasmRunner(t *testing.T) {
	wasmPath := buildFakeWasm(t)
	r, err := Open(&configv1.PluginConfig{
		Name:   "fake",
		Source: &configv1.PluginSource{Location: &configv1.PluginSource_Wasm{Wasm: wasmPath}},
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
	if len(resp.GetFiles()) != 1 || string(resp.GetFiles()[0].GetContents()) != "hi" {
		t.Fatalf("files=%+v", resp.GetFiles())
	}
}
