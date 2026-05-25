// Package sqld is the public API for the sqld engine: parse PostgreSQL sources
// into the semantic IR (a Catalog) and run code-generation plugins against it.
//
// Typical use:
//
//	cfg, err := sqld.LoadConfig("sqld.yaml")
//	if err != nil { ... }
//	cat, err := sqld.Collect(cfg)   // schema + migrations -> IR
//	// or
//	err = sqld.Generate(cfg)        // Collect, then run the configured plugins
//
// The IR and plugin contract messages are protobuf types under
// github.com/yaroher/sqld/pkg/proto/sqld/v1/{ir,plugin}.
// The config is a plain Go struct under github.com/yaroher/sqld/pkg/config.
package sqld

import (
	"github.com/yaroher/sqld/internal/core"
	"github.com/yaroher/sqld/pkg/config"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// Config re-exports the config type for callers who import only pkg/sqld.
type Config = config.Config

// Diagnostic is a non-fatal issue surfaced while building the IR (e.g. an
// unresolved column type or an unhandled statement).
type Diagnostic struct {
	Severity string // "info" | "warning" | "error"
	Message  string
}

// Result is the full output of a collection pass.
type Result struct {
	Catalog     *irv1.Catalog
	Queries     []*pluginv1.Query
	Migrations  []*pluginv1.Migration
	Diagnostics []Diagnostic
}

// LoadConfig reads and parses a YAML config file into a Config struct.
func LoadConfig(path string) (*Config, error) {
	return config.Load(path)
}

// Collect parses the configured SQL and migrations into the IR Catalog.
func Collect(cfg *Config) (*irv1.Catalog, error) {
	return core.Collect(cfg)
}

// CollectAll runs the full pipeline and returns the Catalog together with the
// inferred queries, parsed migrations, and any diagnostics.
func CollectAll(cfg *Config) (*Result, error) {
	r, err := core.Gather(cfg)
	if err != nil {
		return nil, err
	}
	out := &Result{
		Catalog:    r.Catalog,
		Queries:    r.Queries,
		Migrations: r.Migrations,
	}
	if r.Diagnostics != nil {
		for _, d := range r.Diagnostics.Items {
			out.Diagnostics = append(out.Diagnostics, Diagnostic{Severity: d.Severity, Message: d.Message})
		}
	}
	return out, nil
}

// Generate runs Collect and then drives the configured plugins, writing their
// output files to disk.
func Generate(cfg *Config) error {
	return core.Generate(cfg)
}

// CollectFile loads a config file and collects the IR Catalog from it.
func CollectFile(path string) (*irv1.Catalog, error) {
	cfg, err := LoadConfig(path)
	if err != nil {
		return nil, err
	}
	return Collect(cfg)
}

// GenerateFile loads a config file and runs generation from it.
func GenerateFile(path string) error {
	cfg, err := LoadConfig(path)
	if err != nil {
		return err
	}
	return Generate(cfg)
}
