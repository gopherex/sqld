package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yaroher/sqld/cmd/bobgen-sqld/out/models"
	"github.com/yaroher/sqld/cmd/bobgen-sqld/pocshared"
)

// TestTypeUnificationLinchpin is the keystone proof: the bob-generated Account
// model's Status field IS the sqld-controlled shared type pocshared.AccountStatus.
// This is a compile-level assertion — if bob had emitted its own enum type
// (e.g. enums.AccountStatus) instead, none of the assignments below would compile.
func TestTypeUnificationLinchpin(t *testing.T) {
	var s pocshared.AccountStatus = pocshared.AccountStatusActive

	var m models.Account
	m.Status = s // must compile: the bob field IS the shared type

	// And the reverse direction.
	var back pocshared.AccountStatus = m.Status
	if back != pocshared.AccountStatusActive {
		t.Fatalf("round-trip mismatch: %q", back)
	}

	// Sanity-check the other columns mapped to the expected canonical Go types.
	m.ID = int64(1)
	m.Email = "a@b.c"
	m.IsVerified = true
	// Nickname is nullable -> null.Val[string]; ensure it is NOT a bare string,
	// proving the null-convention path also composes over our shared types.
	_ = m.Nickname.IsValue()
}

// TestRegenerationIsStable re-runs generation and confirms the field type is
// still produced as pocshared.AccountStatus (guards against silent drift).
func TestRegenerationIsStable(t *testing.T) {
	// bob's PackageForFolder requires the output dir to live inside a Go module,
	// so generate into a sub-dir of the package (cleaned up afterwards) rather
	// than t.TempDir() which is outside the module tree.
	outDir, err := os.MkdirTemp(".", "regen-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(outDir) })

	cat, err := loadCatalog(filepath.Join("testdata", "schema.sql"))
	if err != nil {
		t.Fatal(err)
	}
	if err := generate(context.Background(), cat, outDir, false); err != nil {
		t.Fatal(err)
	}
	src, err := os.ReadFile(filepath.Join(outDir, "models", "accounts.bob.go"))
	if err != nil {
		t.Fatal(err)
	}
	want := "Status     pocshared.AccountStatus `db:\"status\" `"
	if !strings.Contains(string(src), want) {
		t.Fatalf("generated model does not contain the sqld-controlled field type.\nwant substring: %s", want)
	}
	// It must NOT have generated a bob-owned enum type for the column.
	if strings.Contains(string(src), "enums.AccountStatus") {
		t.Fatalf("unexpected bob-owned enum type reference in generated model")
	}
}
