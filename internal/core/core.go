// Package core provides the high-level collection pipeline that turns a Config
// into a fully-resolved IR Catalog, Query list, and Migration list.
package core

import (
	"context"
	"encoding/json"
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
	"github.com/yaroher/sqld/pkg/config"
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

// engineToIR maps the config Engine string to the protobuf irv1.Engine value.
func engineToIR(e config.Engine) irv1.Engine {
	switch e {
	case config.EnginePostgreSQL:
		return irv1.Engine_ENGINE_POSTGRESQL
	default:
		return irv1.Engine_ENGINE_UNSPECIFIED
	}
}

// Collect parses the configured SQL + migrations into the IR Catalog.
func Collect(cfg *config.Config) (*irv1.Catalog, error) {
	r, err := Gather(cfg)
	if err != nil {
		return nil, err
	}
	return r.Catalog, nil
}

// Gather runs the full pipeline and returns the catalog, queries, migrations,
// source units, and accumulated diagnostics.
func Gather(cfg *config.Config) (*Result, error) {
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
// the generated files to disk. Disabled plugins (IsEnabled() == false) are
// skipped.
func Generate(cfg *config.Config) error {
	r, err := Gather(cfg)
	if err != nil {
		return err
	}

	ctx := context.Background()

	for _, pc := range cfg.Plugins {
		if !pc.IsEnabled() {
			continue
		}

		runner, err := plugin.Open(pc)
		if err != nil {
			return fmt.Errorf("plugin %q: %w", pc.Name, err)
		}

		info, err := runner.GetInfo(ctx)
		if err != nil {
			_ = runner.Close()
			return fmt.Errorf("plugin %q: %w", pc.Name, err)
		}

		// Build annotations best-effort: skip per-unit errors.
		var anns []*irv1.AnnotationValue
		if info.GetAnnotationSchema() != nil {
			for _, u := range r.Units {
				vs, _ := plugin.Annotate(u.SQL, u.Path, info.GetAnnotationSchema())
				anns = append(anns, vs...)
			}
		}

		// Marshal plugin options to JSON bytes (nil when no options).
		var optBytes []byte
		if pc.Options != nil {
			optBytes, err = json.Marshal(pc.Options)
			if err != nil {
				_ = runner.Close()
				return fmt.Errorf("plugin %q: marshal options: %w", pc.Name, err)
			}
		}

		req := &pluginv1.GenerateRequest{
			Catalog:     r.Catalog,
			Queries:     r.Queries,
			Migrations:  r.Migrations,
			Options:     optBytes,
			OutDir:      pc.Out,
			Annotations: anns,
			Context: &pluginv1.PluginContext{
				HostVersion: "dev",
				Engine:      engineToIR(cfg.Engine),
				Env:         pc.Env,
			},
		}

		resp, err := runner.Generate(ctx, req)
		if err != nil {
			_ = runner.Close()
			return fmt.Errorf("plugin %q: %w", pc.Name, err)
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
			return fmt.Errorf("plugin %q failed: %w", pc.Name, errors.Join(errs...))
		}

		// Write generated files.
		for _, f := range resp.GetFiles() {
			out := filepath.Join(pc.Out, f.GetPath())
			if err := os.MkdirAll(filepath.Dir(out), 0o755); err != nil {
				_ = runner.Close()
				return fmt.Errorf("plugin %q: mkdir %q: %w", pc.Name, filepath.Dir(out), err)
			}
			mode := os.FileMode(0o644)
			if f.GetExecutable() {
				mode = 0o755
			}
			if err := os.WriteFile(out, f.GetContents(), mode); err != nil {
				_ = runner.Close()
				return fmt.Errorf("plugin %q: write %q: %w", pc.Name, out, err)
			}
		}

		if err := runner.Close(); err != nil {
			return fmt.Errorf("plugin %q: close: %w", pc.Name, err)
		}
	}

	return nil
}
