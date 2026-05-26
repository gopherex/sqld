package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/stephenafamo/bob/gen"
	"github.com/stephenafamo/bob/gen/plugins"

	"github.com/yaroher/sqld/pkg/gotypes"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// Generate runs bob's code generator over the request's catalog, writing the
// ORM (models, relationships, factories, where/loaders/joins) directly into
// req.OutDir. bob owns multi-package output whose import paths are rooted at the
// final location, so unlike sqld-gen-go this plugin writes to disk itself and
// returns an EMPTY file list — the host's file-writing loop is then a no-op.
func Generate(req *pluginv1.GenerateRequest) (*pluginv1.GenerateResponse, error) {
	opts, err := parseBobOptions(req.GetOptions())
	if err != nil {
		return nil, fmt.Errorf("options: %w", err)
	}

	// nullMode selects bob's TypeSystem: model fields are null.Val[T] either way;
	// the mode controls the SETTER/optional wrapper — "pointer" → *T, "opt" →
	// omit.Val[T]. It must match sqld-gen-go's nullMode.
	typeSystem := "github.com/aarondl/opt/null" // pointer-based setters
	if opts.NullMode == "opt" {
		typeSystem = "" // aarondl/opt (omit.Val/null.Val value types)
	}

	driver := newDriver(req.GetCatalog(), opts.TypesPackage, gotypes.Overrides(opts.Overrides))

	out := req.GetOutDir()
	// Factories require a RandomExpr for every column type; bob cannot synthesize
	// one for the externally-owned shared types (composites, uuid, pgtype.*), so
	// factories are opt-in (default off) rather than on like the other outputs.
	factories := opts.Factories != nil && *opts.Factories
	pcfg := plugins.Config{
		Models:   output(filepath.Join(out, opts.Package), opts.Package, opts.on(opts.Models)),
		Factory:  output(filepath.Join(out, "factory"), "factory", factories),
		DBErrors: disabled(),
		DBInfo:   disabled(),
		// The Enums output must be enabled (the Models/Factory outputs depend on
		// it), but DBInfo.Enums is left empty in the driver, so this produces an
		// empty enums package and bob emits NO competing enum types — model
		// fields still resolve to sqld-gen-go's shared types.
		Enums:   output(filepath.Join(out, "enums"), "enums", true),
		Where:   onOff(opts.on(opts.WhereLoadersJoins)),
		Loaders: onOff(opts.on(opts.WhereLoadersJoins)),
		Joins:   onOff(opts.on(opts.WhereLoadersJoins)),
		Counts:  onOff(opts.on(opts.WhereLoadersJoins)),
	}
	outPlugins := plugins.Setup[any, any, any](pcfg, gen.PSQLTemplates)

	state := &gen.State[any]{Config: gen.Config[any]{TypeSystem: typeSystem, NoTests: true}}

	if err := gen.Run[any, any, any](context.Background(), state, driver, outPlugins...); err != nil {
		return nil, fmt.Errorf("bob gen: %w", err)
	}

	// When the shared leaf-type package is configured, emit a ToSqld bridge on
	// every bob model: a field-copy into sqld-gen-go's flat model that unwraps
	// bob's null.Val[T] back to what sqld-gen-go emits. This auto-enables the
	// interop without requiring nullMode to match sqld-gen-go.
	if opts.TypesPackage != "" {
		if err := generateBridge(req, opts, driver.mapper); err != nil {
			return nil, fmt.Errorf("bob gen bridge: %w", err)
		}
	}

	// bob already wrote the files into out; nothing for the host to write.
	return &pluginv1.GenerateResponse{}, nil
}

// output builds an OutputConfig pointing at dir with package name pkg, disabled
// when enabled is false.
func output(dir, pkg string, enabled bool) plugins.OutputConfig {
	if !enabled {
		return disabled()
	}
	return plugins.OutputConfig{Destination: dir, Pkgname: pkg}
}

func disabled() plugins.OutputConfig {
	t := true
	return plugins.OutputConfig{Disabled: &t}
}

func onOff(enabled bool) plugins.OnOffConfig {
	if enabled {
		return plugins.OnOffConfig{}
	}
	t := true
	return plugins.OnOffConfig{Disabled: &t}
}
