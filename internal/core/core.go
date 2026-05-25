// Package core provides the high-level collection pipeline that turns a Config
// into a fully-resolved IR Catalog, Query list, and Migration list.
package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/yaroher/sqld/internal/catalog"
	"github.com/yaroher/sqld/internal/mapper"
	"github.com/yaroher/sqld/internal/nodeid"
	"github.com/yaroher/sqld/internal/parse"
	"github.com/yaroher/sqld/internal/plugin"
	"github.com/yaroher/sqld/internal/query"
	"github.com/yaroher/sqld/internal/relate"
	"github.com/yaroher/sqld/internal/source"
	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// Result is the full output of a collection pass.
type Result struct {
	Catalog     *irv1.Catalog
	Queries     []*pluginv1.Query
	Migrations  []*pluginv1.Migration
	Diagnostics *catalog.Diagnostics
	Units       []source.Unit
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
		Units:       units,
	}, nil
}

// Generate runs each configured plugin against the collected IR and writes
// the generated files to disk.
//
// Note: the Enabled field on PluginConfig is currently not consulted — all
// configured plugins are run. Annotation→Metadata mirroring is deferred;
// annotations are delivered via GenerateRequest.Annotations only.
func Generate(cfg *configv1.Config) error {
	r, err := gather(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()

	for _, pc := range cfg.GetPlugins() {
		runner, err := plugin.Open(pc)
		if err != nil {
			return fmt.Errorf("plugin %q: %w", pc.GetName(), err)
		}

		info, err := runner.GetInfo(ctx)
		if err != nil {
			_ = runner.Close()
			return fmt.Errorf("plugin %q: %w", pc.GetName(), err)
		}

		// Build annotations best-effort: skip per-unit errors.
		var anns []*irv1.AnnotationValue
		if info.GetAnnotationSchema() != nil {
			for _, u := range r.Units {
				vs, _ := plugin.Annotate(u.SQL, u.Path, info.GetAnnotationSchema())
				anns = append(anns, vs...)
			}
		}

		req := &pluginv1.GenerateRequest{
			Catalog:     r.Catalog,
			Queries:     r.Queries,
			Migrations:  r.Migrations,
			Options:     pc.GetOptions(),
			OutDir:      pc.GetOut(),
			Annotations: anns,
			Context: &pluginv1.PluginContext{
				HostVersion: "dev",
				Engine:      cfg.GetEngine(),
				Env:         pc.GetEnv(),
			},
		}

		resp, err := runner.Generate(ctx, req)
		if err != nil {
			_ = runner.Close()
			return fmt.Errorf("plugin %q: %w", pc.GetName(), err)
		}

		// Check for error-severity diagnostics.
		var diagMsgs []string
		for _, d := range resp.GetDiagnostics() {
			if d.GetSeverity() == pluginv1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_ERROR {
				diagMsgs = append(diagMsgs, d.GetMessage())
			}
		}
		if len(diagMsgs) > 0 {
			_ = runner.Close()
			var errs []error
			for _, msg := range diagMsgs {
				errs = append(errs, errors.New(msg))
			}
			return fmt.Errorf("plugin %q failed: %w", pc.GetName(), errors.Join(errs...))
		}

		// Write generated files.
		for _, f := range resp.GetFiles() {
			out := filepath.Join(pc.GetOut(), f.GetPath())
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				_ = runner.Close()
				return fmt.Errorf("plugin %q: mkdir %q: %w", pc.GetName(), filepath.Dir(out), err)
			}
			mode := os.FileMode(0o644)
			if f.GetExecutable() {
				mode = 0o755
			}
			if err := os.WriteFile(out, f.GetContents(), mode); err != nil {
				_ = runner.Close()
				return fmt.Errorf("plugin %q: write %q: %w", pc.GetName(), out, err)
			}
		}

		if err := runner.Close(); err != nil {
			return fmt.Errorf("plugin %q: close: %w", pc.GetName(), err)
		}
	}

	return nil
}
