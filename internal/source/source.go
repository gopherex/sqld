// internal/source/source.go
package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/yaroher/sqld/pkg/config"
)

type Kind int

const (
	KindQuery Kind = iota
	KindMigrationUp
	KindSchema
)

// Unit is one SQL chunk with provenance.
type Unit struct {
	Path    string
	SQL     string
	DownSQL string // migrations only; empty when no -- sqld:down section
	Kind    Kind
	Version string // migrations only
}

// Resolve reads all schema, query, and migration sources into ordered units.
// Declarative schema sources (KindSchema) come from cfg.Schema.
// Named queries come from cfg.Queries (KindQuery).
// Migration files come from cfg.Migrations (KindMigrationUp).
func Resolve(cfg *config.Config) ([]Unit, error) {
	var units []Unit
	for _, s := range cfg.Schema {
		us, err := resolveSchema(s)
		if err != nil {
			return nil, err
		}
		units = append(units, us...)
	}
	for _, s := range cfg.Queries {
		us, err := resolveQuery(s)
		if err != nil {
			return nil, err
		}
		units = append(units, us...)
	}
	for _, m := range cfg.Migrations {
		us, err := resolveMigration(m)
		if err != nil {
			return nil, err
		}
		units = append(units, us...)
	}
	return units, nil
}

func resolveSchema(s config.Source) ([]Unit, error) {
	switch {
	case s.Inline != "":
		return []Unit{{Path: "<inline>", SQL: s.Inline, Kind: KindSchema}}, nil
	case s.File != "":
		b, err := os.ReadFile(s.File)
		if err != nil {
			return nil, err
		}
		return []Unit{{Path: s.File, SQL: string(b), Kind: KindSchema}}, nil
	case s.Dir != "":
		glob := s.Glob
		if glob == "" {
			glob = "*.sql"
		}
		return readDir(s.Dir, glob, s.Recursive, KindSchema)
	default:
		return nil, fmt.Errorf("schema source: empty")
	}
}

func resolveQuery(s config.Source) ([]Unit, error) {
	switch {
	case s.Inline != "":
		return []Unit{{Path: "<inline>", SQL: s.Inline, Kind: KindQuery}}, nil
	case s.File != "":
		b, err := os.ReadFile(s.File)
		if err != nil {
			return nil, err
		}
		return []Unit{{Path: s.File, SQL: string(b), Kind: KindQuery}}, nil
	case s.Dir != "":
		glob := s.Glob
		if glob == "" {
			glob = "*.sql"
		}
		return readDir(s.Dir, glob, s.Recursive, KindQuery)
	default:
		return nil, fmt.Errorf("query source: empty")
	}
}

func readDir(dir, glob string, recursive bool, k Kind) ([]Unit, error) {
	var paths []string
	walk := func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !recursive && p != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if ok, _ := filepath.Match(glob, d.Name()); ok {
			paths = append(paths, p)
		}
		return nil
	}
	if err := filepath.WalkDir(dir, walk); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	units := make([]Unit, 0, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		units = append(units, Unit{Path: p, SQL: string(b), Kind: k})
	}
	return units, nil
}

func resolveMigration(m config.MigrationSource) ([]Unit, error) {
	glob := m.Glob
	if glob == "" {
		glob = "*.sql"
	}
	us, err := readDir(m.Dir, glob, false, KindMigrationUp)
	if err != nil {
		return nil, err
	}
	for i := range us {
		us[i].Version = trimExt(filepath.Base(us[i].Path))
		up, down := splitMigration(us[i].SQL)
		us[i].SQL = up
		us[i].DownSQL = down
	}
	return us, nil
}

// splitMigration splits a migration file's SQL into up and down sections.
// Lines matching "-- sqld:up" (case-insensitive, trimmed) begin the up section;
// lines matching "-- sqld:down" begin the down section. Text before any marker
// is treated as up. The marker lines themselves are excluded from the output.
// Both returned strings are trimmed of leading/trailing whitespace.
func splitMigration(sql string) (up, down string) {
	const markerUp = "-- sqld:up"
	const markerDown = "-- sqld:down"

	type section int
	const (
		sectionUp   section = iota
		sectionDown section = iota
	)

	var upLines, downLines []string
	current := sectionUp

	for _, line := range strings.Split(sql, "\n") {
		trimmed := strings.TrimSpace(line)
		lower := strings.ToLower(trimmed)
		switch lower {
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

func trimExt(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}
