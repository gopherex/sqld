package plugin_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yaroher/sqld/pkg/config"
	"github.com/yaroher/sqld/pkg/sqld"
)

// repoRoot returns the module root (the directory containing go.mod), resolved
// from the location of this test file (internal/plugin/).
func repoRoot(t *testing.T) string {
	t.Helper()
	// internal/plugin → internal → repo root
	dir, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "go.mod")); err != nil {
		t.Fatalf("go.mod not found at %s: %v", dir, err)
	}
	return dir
}

// buildSqldGenGoWasm compiles ./cmd/sqld-gen-go for GOOS=wasip1 GOARCH=wasm
// into a temp file and returns the path. The test is skipped (not failed) if
// the wasip1 toolchain is unavailable.
func buildSqldGenGoWasm(t *testing.T) string {
	t.Helper()
	out := filepath.Join(t.TempDir(), "gen.wasm")
	cmd := exec.Command("go", "build", "-o", out, "./cmd/sqld-gen-go")
	cmd.Env = append(os.Environ(), "GOOS=wasip1", "GOARCH=wasm")
	cmd.Dir = repoRoot(t)
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		t.Skipf("cannot build wasip1 sqld-gen-go (skipping wasm e2e test): %v", err)
	}
	return out
}

// TestGenerateViaWasmPlugin is an end-to-end test that:
//  1. Compiles sqld-gen-go to wasip1/wasm.
//  2. Runs sqld.Generate with a PluginConfig whose Wasm field points at the
//     compiled .wasm, using a simple schema and a named query.
//  3. Asserts that the generated models.go and queries.go exist and contain
//     expected identifiers — proving that the wazero transport produced real
//     Go code identical in shape to the binary-transport output.
func TestGenerateViaWasmPlugin(t *testing.T) {
	wasmPath := buildSqldGenGoWasm(t)

	root := repoRoot(t)
	outDir := filepath.Join(t.TempDir(), "dbwasm")

	// Use the example schema and queries so the generated output is directly
	// comparable to the binary-transport output in example/gen/db/.
	cfg := &config.Config{
		Engine: config.EnginePostgreSQL,
		Schema: []config.Source{
			{File: filepath.Join(root, "example", "schema.sql")},
		},
		Queries: []config.Source{
			{Dir: filepath.Join(root, "example", "queries")},
		},
		Migrations: []config.MigrationSource{
			{Dir: filepath.Join(root, "example", "migrations")},
		},
		Plugins: []config.PluginConfig{
			{
				Name: "go",
				Wasm: wasmPath,
				Out:  outDir,
				Options: map[string]any{
					"package": "dbwasm",
					"overrides": map[string]any{
						"app.kitchen_sink.c_jsonb": "map[string]any",
						"uuid":                     "github.com/google/uuid.UUID",
					},
				},
			},
		},
	}

	if err := sqld.Generate(cfg); err != nil {
		t.Fatalf("sqld.Generate via wasm: %v", err)
	}

	// --- models.go assertions ---
	modelsPath := filepath.Join(outDir, "models.go")
	modelsBytes, err := os.ReadFile(modelsPath)
	if err != nil {
		t.Fatalf("models.go not written: %v", err)
	}
	modelsContent := string(modelsBytes)

	modelChecks := []string{
		"package dbwasm",
		"type AppUsers struct",
		"type AppProfiles struct",
		"type AppKitchenSink struct",
		"type AppUserStatus string",
		"AppUserStatusActive",
		"AppUserStatusInactive",
		"AppUserStatusBanned",
	}
	for _, want := range modelChecks {
		if !strings.Contains(modelsContent, want) {
			t.Errorf("models.go missing %q", want)
		}
	}

	// --- queries.go assertions ---
	queriesPath := filepath.Join(outDir, "queries.go")
	queriesBytes, err := os.ReadFile(queriesPath)
	if err != nil {
		t.Fatalf("queries.go not written: %v", err)
	}
	queriesContent := string(queriesBytes)

	queryChecks := []string{
		"package dbwasm",
		"func (q *Queries) GetUser(",
		"func (q *Queries) ListActiveUsers(",
		"func (q *Queries) SearchUsers(",
	}
	for _, want := range queryChecks {
		if !strings.Contains(queriesContent, want) {
			t.Errorf("queries.go missing %q", want)
		}
	}

	// --- parity check: the wasm output must contain the same table structs
	//     as the binary-generated example/gen/db/models.go ---
	binaryModels := filepath.Join(root, "example", "gen", "db", "models.go")
	binaryBytes, err := os.ReadFile(binaryModels)
	if err != nil {
		t.Logf("note: could not read binary models.go for parity check: %v", err)
	} else {
		parityCases := []struct{ name, needle string }{
			{"AppUsers struct", "type AppUsers struct"},
			{"AppProfiles struct", "type AppProfiles struct"},
			{"AppKitchenSink struct", "type AppKitchenSink struct"},
		}
		for _, tc := range parityCases {
			inBinary := strings.Contains(string(binaryBytes), tc.needle)
			inWasm := strings.Contains(modelsContent, tc.needle)
			if inBinary != inWasm {
				t.Errorf("parity mismatch for %s: binary=%v wasm=%v", tc.name, inBinary, inWasm)
			}
		}
	}
}
