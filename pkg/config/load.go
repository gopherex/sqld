package config

import (
	"bytes"
	"fmt"

	"gopkg.in/yaml.v3"
	"os"
)

// Load reads a YAML config file at path, validates it, applies defaults,
// and returns the populated Config.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := &Config{}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config %s: %w", path, err)
	}
	return cfg, nil
}

// Validate applies defaults and checks required fields.
// It returns the first error encountered with enough context to locate it.
func (cfg *Config) Validate() error {
	// Engine: default to postgresql; reject anything else.
	if cfg.Engine == "" {
		cfg.Engine = EnginePostgreSQL
	} else if cfg.Engine != EnginePostgreSQL {
		return fmt.Errorf("engine: unsupported value %q (only %q is supported)", cfg.Engine, EnginePostgreSQL)
	}

	// Global options defaults.
	if cfg.Options.DefaultSchema == "" {
		cfg.Options.DefaultSchema = "public"
	}

	// SQL sources.
	for i := range cfg.SQL {
		s := &cfg.SQL[i]
		// Exactly one source field must be set.
		set := 0
		if s.File != "" {
			set++
		}
		if s.Dir != "" {
			set++
		}
		if s.Inline != "" {
			set++
		}
		if set != 1 {
			return fmt.Errorf("sql[%d]: set exactly one of file|dir|inline", i)
		}
		// Kind default.
		if s.Kind == "" {
			s.Kind = SQLSchema
		}
		if s.Kind != SQLSchema && s.Kind != SQLQuery {
			return fmt.Errorf("sql[%d]: kind must be %q or %q, got %q", i, SQLSchema, SQLQuery, s.Kind)
		}
		// Glob default for dir sources.
		if s.Dir != "" && s.Glob == "" {
			s.Glob = "*.sql"
		}
	}

	// Migration sources.
	for i := range cfg.Migrations {
		m := &cfg.Migrations[i]
		if m.Dir == "" {
			return fmt.Errorf("migrations[%d]: dir is required", i)
		}
		if m.Glob == "" {
			m.Glob = "*.sql"
		}
	}

	// Plugins.
	for i := range cfg.Plugins {
		p := &cfg.Plugins[i]
		if p.Name == "" {
			return fmt.Errorf("plugins[%d]: name is required", i)
		}
		set := 0
		if p.Wasm != "" {
			set++
		}
		if p.Binary != "" {
			set++
		}
		if p.Command != "" {
			set++
		}
		if set != 1 {
			return fmt.Errorf("plugins[%d] (%q): set exactly one of wasm|binary|command", i, p.Name)
		}
		if p.Out == "" {
			return fmt.Errorf("plugins[%d] (%q): out is required", i, p.Name)
		}
	}

	return nil
}
