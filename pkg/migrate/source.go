// Package migrate provides a PostgreSQL migration loader and a Migrator that
// applies, reverts, and reports the status of versioned SQL migrations.
package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// Migration is a single versioned SQL migration loaded from a file.
//
// A migration file is named "<version>_<name>.sql". Its body may contain
// "-- sqld:up" and "-- sqld:down" markers that split the up and down sections;
// when no markers are present the whole file is the up section.
type Migration struct {
	Version  string
	Name     string
	UpSQL    string
	DownSQL  string
	Checksum string // sha256 hex of the raw file bytes
}

// Load reads all *.sql migrations from an OS directory and returns them sorted
// by version. It delegates to LoadFS over os.DirFS(dir).
//
// Each filename is parsed as "<version>_<name>.sql": everything up to the first
// '_' is the version, the remainder (minus the .sql extension) is the name. The
// file body is split into up/down sections on the "-- sqld:up" / "-- sqld:down"
// markers (case-insensitive, whole-line); with no markers the whole file is up.
// Checksum is the sha256 hex of the raw file bytes.
func Load(dir string) ([]Migration, error) {
	migs, err := LoadFS(os.DirFS(dir))
	if err != nil {
		return nil, err
	}
	return migs, nil
}

// LoadFS reads all *.sql migrations from an fs.FS (e.g. an embed.FS) and returns
// them sorted by version.
//
// It walks the filesystem recursively, so an `//go:embed migrations` directive
// (whose files live under "migrations/") works without the caller doing fs.Sub.
// Each base filename is parsed as "<version>_<name>.sql"; the body is split on
// the "-- sqld:up" / "-- sqld:down" markers (with no markers the whole file is
// up); Checksum is the sha256 hex of the raw file bytes. Non-.sql files are
// skipped.
func LoadFS(fsys fs.FS) ([]Migration, error) {
	var migs []Migration

	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		base := path.Base(p)
		if !strings.HasSuffix(strings.ToLower(base), ".sql") {
			return nil
		}

		raw, err := fs.ReadFile(fsys, p)
		if err != nil {
			return fmt.Errorf("migrate: read %q: %w", p, err)
		}

		version, mname, err := parseFilename(base)
		if err != nil {
			return err
		}

		up, down := splitMigration(string(raw))
		sum := sha256.Sum256(raw)

		migs = append(migs, Migration{
			Version:  version,
			Name:     mname,
			UpSQL:    up,
			DownSQL:  down,
			Checksum: hex.EncodeToString(sum[:]),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("migrate: load fs: %w", err)
	}

	sort.Slice(migs, func(i, j int) bool {
		return migs[i].Version < migs[j].Version
	})
	return migs, nil
}

// parseFilename splits "<version>_<name>.sql" into its version and name parts.
func parseFilename(filename string) (version, name string, err error) {
	base := filename[:len(filename)-len(filepath.Ext(filename))]
	idx := strings.IndexByte(base, '_')
	if idx < 0 {
		return "", "", fmt.Errorf("migrate: filename %q is not <version>_<name>.sql", filename)
	}
	version = base[:idx]
	name = base[idx+1:]
	if version == "" {
		return "", "", fmt.Errorf("migrate: filename %q has empty version", filename)
	}
	return version, name, nil
}

// splitMigration splits a migration file's SQL into up and down sections.
//
// Lines equal to "-- sqld:up" (case-insensitive, trimmed) begin the up section;
// lines equal to "-- sqld:down" begin the down section. Text before any marker
// is treated as up. Marker lines are excluded from the output. Both returned
// strings are trimmed of leading/trailing whitespace.
func splitMigration(sql string) (up, down string) {
	const markerUp = "-- sqld:up"
	const markerDown = "-- sqld:down"

	type section int
	const (
		sectionUp section = iota
		sectionDown
	)

	var upLines, downLines []string
	current := sectionUp

	for _, line := range strings.Split(sql, "\n") {
		switch strings.ToLower(strings.TrimSpace(line)) {
		case markerUp:
			current = sectionUp
		case markerDown:
			current = sectionDown
		default:
			if current == sectionUp {
				upLines = append(upLines, line)
			} else {
				downLines = append(downLines, line)
			}
		}
	}

	up = strings.TrimSpace(strings.Join(upLines, "\n"))
	down = strings.TrimSpace(strings.Join(downLines, "\n"))
	return up, down
}
