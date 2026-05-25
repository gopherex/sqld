package config

// Engine identifies the SQL dialect.
type Engine string

const EnginePostgreSQL Engine = "postgresql"

// Config is the root of a generation run, read from a YAML file.
type Config struct {
	Version    string            `yaml:"version"`
	Engine     Engine            `yaml:"engine"`
	Queries    []Source          `yaml:"queries"`
	Migrations []MigrationSource `yaml:"migrations"`
	Plugins    []PluginConfig    `yaml:"plugins"`
	Options    GlobalOptions     `yaml:"options"`
}

// Source describes a single SQL input for named queries: either a file,
// a directory, inline SQL text, or a glob pattern applied to the current
// directory.
type Source struct {
	File      string `yaml:"file"`
	Dir       string `yaml:"dir"`
	Inline    string `yaml:"inline"`
	Glob      string `yaml:"glob"`
	Recursive bool   `yaml:"recursive"`
}

// MigrationSource points at a directory (or glob) of up-migration SQL files.
// Migrations are the sole source of schema DDL.
type MigrationSource struct {
	Dir  string `yaml:"dir"`
	Glob string `yaml:"glob"`
}

// PluginConfig describes one code-generation plugin.
type PluginConfig struct {
	Name    string            `yaml:"name"`
	Wasm    string            `yaml:"wasm"`
	Binary  string            `yaml:"binary"`
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	SHA256  string            `yaml:"sha256"`
	Out     string            `yaml:"out"`
	Options map[string]any    `yaml:"options"`
	Env     map[string]string `yaml:"env"`
	Enabled *bool             `yaml:"enabled"` // nil means true
}

// GlobalOptions are host-wide generation settings.
type GlobalOptions struct {
	DefaultSchema string            `yaml:"defaultSchema"`
	SearchPath    []string          `yaml:"searchPath"`
	TypeOverrides map[string]string `yaml:"typeOverrides"`
	Strict        bool              `yaml:"strict"`
}

// IsEnabled reports whether the plugin should run (default true when Enabled is nil).
func (p PluginConfig) IsEnabled() bool { return p.Enabled == nil || *p.Enabled }
