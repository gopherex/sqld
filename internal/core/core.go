// Package core provides the high-level collection pipeline that turns a Config
// into a fully-resolved IR Catalog, Query list, and Migration list.
package core

import (
	"fmt"
	"strconv"

	"github.com/yaroher/sqld/internal/catalog"
	"github.com/yaroher/sqld/internal/mapper"
	"github.com/yaroher/sqld/internal/nodeid"
	"github.com/yaroher/sqld/internal/parse"
	"github.com/yaroher/sqld/internal/query"
	"github.com/yaroher/sqld/internal/relate"
	"github.com/yaroher/sqld/internal/source"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
)

// Result is the full output of a collection pass.
type Result struct {
	Catalog     *irv1.Catalog
	Queries     []*pluginv1.Query
	Migrations  []*pluginv1.Migration
	Diagnostics *catalog.Diagnostics
}

// Collect parses the configured SQL + migrations into the IR Catalog.
func Collect(cfg *configv1.Config) (*irv1.Catalog, error) {
	r, err := gather(cfg)
	if err != nil {
		return nil, err
	}
	return r.Catalog, nil
}

// gather runs the full pipeline: catalog + queries + migrations.
func gather(cfg *configv1.Config) (*Result, error) {
	// Step 1: resolve all source units.
	units, err := source.Resolve(cfg)
	if err != nil {
		return nil, fmt.Errorf("source.Resolve: %w", err)
	}

	// Step 2: split units by kind, build schemaStmts.
	var schemaUnits []source.Unit
	var migrationUpUnits []source.Unit
	var queryUnits []source.Unit

	for _, u := range units {
		switch u.Kind {
		case source.KindSchema:
			schemaUnits = append(schemaUnits, u)
		case source.KindMigrationUp:
			migrationUpUnits = append(migrationUpUnits, u)
		case source.KindQuery:
			queryUnits = append(queryUnits, u)
		}
	}

	// Build schemaStmts: schema units first, then migration-up units.
	var schemaStmts []parse.Stmt
	for _, u := range schemaUnits {
		stmts, err := parse.Statements(u.SQL)
		if err != nil {
			return nil, fmt.Errorf("parse schema %q: %w", u.Path, err)
		}
		schemaStmts = append(schemaStmts, stmts...)
	}
	for _, u := range migrationUpUnits {
		stmts, err := parse.Statements(u.SQL)
		if err != nil {
			return nil, fmt.Errorf("parse migration %q: %w", u.Path, err)
		}
		schemaStmts = append(schemaStmts, stmts...)
	}

	// Step 3: build the catalog.
	cat, diags := catalog.Build(schemaStmts)

	// Step 4: build migration objects.
	var migrations []*pluginv1.Migration
	for _, u := range migrationUpUnits {
		stmts, err := parse.Statements(u.SQL)
		if err != nil {
			// Already checked above; this should not fail again, but guard anyway.
			diags.Add("error", fmt.Sprintf("re-parse migration %q: %v", u.Path, err))
			continue
		}
		mig := &pluginv1.Migration{
			Version:    u.Version,
			Name:       u.Version,
			UpSql:      u.SQL,
			SourceFile: u.Path,
		}
		for i, s := range stmts {
			irStmt := mapper.MapStatement(s.Node, nodeid.New("mig:"+u.Version+":"+strconv.Itoa(i)))
			mig.Up = append(mig.Up, irStmt)
		}
		migrations = append(migrations, mig)
	}

	// Step 5: parse and infer queries.
	var queries []*pluginv1.Query
	for _, u := range queryUnits {
		qs, err := query.ParseQueries(u.SQL, u.Path)
		if err != nil {
			diags.Add("error", fmt.Sprintf("parse queries %q: %v", u.Path, err))
			continue
		}
		for _, q := range qs {
			query.Infer(q, cat, diags)
			queries = append(queries, q)
		}
	}

	// Step 6: derive relationships.
	cat.Relationships = relate.Derive(cat)

	// Step 7: set IR version.
	cat.IrVersion = "v1"

	return &Result{
		Catalog:     cat,
		Queries:     queries,
		Migrations:  migrations,
		Diagnostics: diags,
	}, nil
}
