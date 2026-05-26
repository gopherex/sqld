// Command bobgen-sqld is a proof-of-concept that drives stephenafamo/bob's
// code generator from sqld's protobuf IR (*irv1.Catalog), forcing bob to emit
// Go model types that sqld controls (the type-unification linchpin).
//
// Usage:
//
//	go run ./cmd/bobgen-sqld            # generate models into out/models
//
// It is intentionally tiny: it loads testdata/schema.sql into an sqld Catalog,
// wraps it in a drivers.Interface, and runs bob's Models output plugin.
package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/stephenafamo/bob/gen"
	"github.com/stephenafamo/bob/gen/plugins"

	"github.com/yaroher/sqld/pkg/config"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	"github.com/yaroher/sqld/pkg/sqld"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "bobgen-sqld:", err)
		os.Exit(1)
	}
}

// loadCatalog parses testdata/schema.sql into an sqld IR Catalog.
func loadCatalog(schemaPath string) (*irv1.Catalog, error) {
	cfg := &config.Config{
		Engine: config.EnginePostgreSQL,
		Schema: []config.Source{{File: schemaPath}},
	}
	return sqld.Collect(cfg)
}

// generate runs bob's generator with the sqld-backed driver, writing the models
// package into outDir. emitBobEnums toggles whether bob also generates its own
// enum package (see report on the type fight).
func generate(ctx context.Context, cat *irv1.Catalog, outDir string, emitBobEnums bool) error {
	modelsDir := filepath.Join(outDir, "models")

	pcfg := plugins.Config{
		// Point models at our out dir.
		Models: plugins.OutputConfig{Destination: modelsDir, Pkgname: "models"},
		// Keep the PoC small: disable everything except models (and the enums
		// output, which auto-disables when DBInfo.Enums is empty).
		Factory:  disabled(),
		DBInfo:   disabled(),
		DBErrors: disabled(),
		Where:    off(),
		Loaders:  off(),
		Joins:    off(),
		Counts:   off(),
	}
	if !emitBobEnums {
		pcfg.Enums = disabled()
	} else {
		pcfg.Enums = plugins.OutputConfig{Destination: filepath.Join(outDir, "enums"), Pkgname: "enums"}
	}

	outPlugins := plugins.Setup[any, any, any](pcfg, gen.PSQLTemplates)

	state := &gen.State[any]{
		Config: gen.Config[any]{
			// "" -> github.com/aarondl/opt (null.Val/omit.Val). Documented in report.
			TypeSystem: "",
			NoTests:    true,
		},
	}

	return gen.Run[any, any, any](ctx, state, newSQLDDriver(cat, emitBobEnums), outPlugins...)
}

func run() error {
	cat, err := loadCatalog(filepath.Join("cmd", "bobgen-sqld", "testdata", "schema.sql"))
	if err != nil {
		return fmt.Errorf("collect catalog: %w", err)
	}
	return generate(context.Background(), cat, filepath.Join("cmd", "bobgen-sqld", "out"), false)
}

func disabled() plugins.OutputConfig {
	t := true
	return plugins.OutputConfig{Disabled: &t}
}

func off() plugins.OnOffConfig {
	t := true
	return plugins.OnOffConfig{Disabled: &t}
}
