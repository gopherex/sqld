// internal/source/source.go
package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
)

type Kind int

const (
	KindSchema Kind = iota
	KindQuery
	KindMigrationUp
	KindMigrationDown
)

// Unit is one SQL chunk with provenance.
type Unit struct {
	Path    string
	SQL     string
	Kind    Kind
	Version string // migrations only
}

// Resolve reads all SQL and migration sources into ordered units.
func Resolve(cfg *configv1.Config) ([]Unit, error) {
	var units []Unit
	for _, s := range cfg.GetSql() {
		us, err := resolveSQL(s)
		if err != nil {
			return nil, err
		}
		units = append(units, us...)
	}
	for _, m := range cfg.GetMigrations() {
		us, err := resolveMigration(m)
		if err != nil {
			return nil, err
		}
		units = append(units, us...)
	}
	return units, nil
}

func kindOf(k configv1.SqlKind) Kind {
	if k == configv1.SqlKind_SQL_KIND_QUERY {
		return KindQuery
	}
	return KindSchema
}

func resolveSQL(s *configv1.SqlSource) ([]Unit, error) {
	k := kindOf(s.GetKind())
	switch src := s.GetSource().(type) {
	case *configv1.SqlSource_Inline:
		return []Unit{{Path: "<inline>", SQL: src.Inline, Kind: k}}, nil
	case *configv1.SqlSource_File:
		b, err := os.ReadFile(src.File)
		if err != nil {
			return nil, err
		}
		return []Unit{{Path: src.File, SQL: string(b), Kind: k}}, nil
	case *configv1.SqlSource_Dir:
		glob := s.GetGlob()
		if glob == "" {
			glob = "*.sql"
		}
		return readDir(src.Dir, glob, s.GetRecursive(), k)
	default:
		return nil, fmt.Errorf("sql source: empty")
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

func resolveMigration(m *configv1.MigrationSource) ([]Unit, error) {
	glob := m.GetGlob()
	if glob == "" {
		glob = "*.sql"
	}
	us, err := readDir(m.GetDir(), glob, false, KindMigrationUp)
	if err != nil {
		return nil, err
	}
	for i := range us {
		us[i].Version = trimExt(filepath.Base(us[i].Path))
	}
	return us, nil
}

func trimExt(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}
