package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestLoad(t *testing.T) {
	dir := t.TempDir()

	const f1 = `-- sqld:up
CREATE TABLE init(id int);
-- sqld:down
DROP TABLE init;
`
	const f2 = `CREATE TABLE more(id int);`

	// Write out of order to confirm Load sorts by version.
	mustWrite(t, filepath.Join(dir, "0002_more.sql"), f2)
	mustWrite(t, filepath.Join(dir, "0001_init.sql"), f1)
	// A non-.sql file must be ignored.
	mustWrite(t, filepath.Join(dir, "README.md"), "ignore me")

	migs, err := Load(dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(migs) != 2 {
		t.Fatalf("want 2 migrations, got %d: %+v", len(migs), migs)
	}

	// Sorted by version.
	if migs[0].Version != "0001" || migs[1].Version != "0002" {
		t.Fatalf("versions out of order: %q, %q", migs[0].Version, migs[1].Version)
	}
	if migs[0].Name != "init" || migs[1].Name != "more" {
		t.Fatalf("names: %q, %q", migs[0].Name, migs[1].Name)
	}

	// 0001 up/down split correctly.
	if !strings.Contains(migs[0].UpSQL, "CREATE TABLE init") || strings.Contains(migs[0].UpSQL, "DROP TABLE") {
		t.Fatalf("0001 up=%q", migs[0].UpSQL)
	}
	if !strings.Contains(migs[0].DownSQL, "DROP TABLE init") {
		t.Fatalf("0001 down=%q", migs[0].DownSQL)
	}

	// 0002 has no markers → all up, empty down.
	if !strings.Contains(migs[1].UpSQL, "CREATE TABLE more") {
		t.Fatalf("0002 up=%q", migs[1].UpSQL)
	}
	if migs[1].DownSQL != "" {
		t.Fatalf("0002 down should be empty, got %q", migs[1].DownSQL)
	}

	// Checksums non-empty and distinct.
	if migs[0].Checksum == "" || migs[1].Checksum == "" {
		t.Fatalf("empty checksum: %q %q", migs[0].Checksum, migs[1].Checksum)
	}
	if migs[0].Checksum == migs[1].Checksum {
		t.Fatalf("checksums not distinct: %q", migs[0].Checksum)
	}
}

func TestLoadFS(t *testing.T) {
	// Files live under "migrations/" to mirror the //go:embed migrations layout
	// and prove LoadFS walks recursively without the caller doing fs.Sub.
	const f1 = `-- sqld:up
CREATE TABLE a(id int);
-- sqld:down
DROP TABLE a;
`
	const f2 = `CREATE TABLE b(id int);`

	fsys := fstest.MapFS{
		"migrations/0002_b.sql": {Data: []byte(f2)},
		"migrations/0001_a.sql": {Data: []byte(f1)},
		// A non-.sql file must be ignored even nested deeper.
		"migrations/README.md": {Data: []byte("ignore me")},
	}

	migs, err := LoadFS(fsys)
	if err != nil {
		t.Fatalf("LoadFS: %v", err)
	}
	if len(migs) != 2 {
		t.Fatalf("want 2 migrations, got %d: %+v", len(migs), migs)
	}

	// Sorted by version.
	if migs[0].Version != "0001" || migs[1].Version != "0002" {
		t.Fatalf("versions out of order: %q, %q", migs[0].Version, migs[1].Version)
	}
	if migs[0].Name != "a" || migs[1].Name != "b" {
		t.Fatalf("names: %q, %q", migs[0].Name, migs[1].Name)
	}

	// 0001 up/down split correctly.
	if !strings.Contains(migs[0].UpSQL, "CREATE TABLE a") || strings.Contains(migs[0].UpSQL, "DROP TABLE") {
		t.Fatalf("0001 up=%q", migs[0].UpSQL)
	}
	if !strings.Contains(migs[0].DownSQL, "DROP TABLE a") {
		t.Fatalf("0001 down=%q", migs[0].DownSQL)
	}

	// 0002 has no markers → all up, empty down.
	if !strings.Contains(migs[1].UpSQL, "CREATE TABLE b") {
		t.Fatalf("0002 up=%q", migs[1].UpSQL)
	}
	if migs[1].DownSQL != "" {
		t.Fatalf("0002 down should be empty, got %q", migs[1].DownSQL)
	}

	// Checksum is sha256 hex of the raw file bytes.
	sum := sha256.Sum256([]byte(f1))
	if want := hex.EncodeToString(sum[:]); migs[0].Checksum != want {
		t.Fatalf("0001 checksum=%q want %q", migs[0].Checksum, want)
	}
	if migs[0].Checksum == migs[1].Checksum {
		t.Fatalf("checksums not distinct: %q", migs[0].Checksum)
	}
}

func TestLoadBadFilename(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "noversion.sql"), "CREATE TABLE x(id int);")
	if _, err := Load(dir); err == nil {
		t.Fatal("want error for filename without <version>_ prefix")
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
