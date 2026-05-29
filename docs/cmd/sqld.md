# sqld

`sqld` is the host CLI of the sqld toolkit — an open alternative to sqlc + Atlas for
PostgreSQL. It reads your SQL schema and named query files, parses them into a
protobuf IR called a `Catalog`, and then drives one or more code-generation plugins
(e.g. `sqld-gen-go`, `sqld-gen-bob`). Each plugin is a separate binary that receives
the `Catalog` over stdio-framed protobuf and writes generated files back to disk.
`sqld` itself does not generate any code; its job is: collect the IR → invoke plugins
in config order → write the response files under each plugin's `out` directory.

---

## Install

```sh
# via go install (recommended)
go install github.com/yaroher/sqld/cmd/sqld@latest

# or build from source
make build       # → bin/sqld (includes migrate), bin/sqld-gen-go
```

---

## Subcommands

### `init`

**Synopsis**

```
sqld init [dir]
```

Scaffold a new sqld project in `dir` (defaults to `.`). The target directory is
created if it does not exist. `init` refuses to overwrite an existing `sqld.yaml`.

**Files created**

| Path | Description |
|------|-------------|
| `sqld.yaml` | Starter config (version 1, postgresql engine, one schema file, queries dir, migrations dir, `sqld-gen-go` plugin) |
| `schema.sql` | Declarative schema stub with an `authors` table |
| `queries/authors.sql` | Two sample named queries (`GetAuthor :one`, `ListAuthors :many`) |
| `migrations/.gitkeep` | Empty sentinel so git tracks the empty directory |

**Example**

```sh
sqld init ./myproject
# created myproject/sqld.yaml
# created myproject/schema.sql
# created myproject/queries/authors.sql
# created myproject/migrations/.gitkeep
#
# next: sqld generate -c myproject/sqld.yaml
```

---

### `generate`

**Synopsis**

```
sqld generate -c <config.yaml>
```

Run the full generation pipeline: collect the IR Catalog from the schema and query
sources declared in `config.yaml`, then invoke each enabled plugin in config order,
writing the generated files to each plugin's `out` directory.

**Flags**

| Flag | Required | Description |
|------|----------|-------------|
| `-c <path>` | yes | Path to the `sqld.yaml` config file |

Prints `generated` to stderr on success. Any plugin that returns error-severity
diagnostics causes `generate` to abort and print the error.

**Example**

```sh
sqld generate -c sqld.yaml
```

---

### `collect`

**Synopsis**

```
sqld collect -c <config.yaml> [--format json|prototext]
```

Collect the IR Catalog from the schema sources and print it to stdout. No plugins are
invoked. Useful for inspecting the parsed schema, debugging type resolution, or
feeding the IR into external tooling.

**Flags**

| Flag | Default | Description |
|------|---------|-------------|
| `-c <path>` | — (required) | Path to the `sqld.yaml` config file |
| `--format <fmt>` | `json` | Output format: `json` (pretty-printed protojson) or `prototext` |

**Example**

```sh
# Pretty-print the IR as JSON
sqld collect -c sqld.yaml

# Prototext format
sqld collect -c sqld.yaml --format prototext

# Pipe into jq
sqld collect -c sqld.yaml | jq '.tables[].name'
```

---

## `sqld.yaml` reference

### Top-level fields

| Field | YAML key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Version | `version` | string | no | Config file version. Use `"1"`. |
| Engine | `engine` | string | no | SQL dialect. Only `"postgresql"` is supported. Defaults to `"postgresql"` if omitted. |
| Options | `options` | object | no | Host-wide generation settings (see [Options](#options)). |
| Schema | `schema` | list of Source | no | Declarative schema DDL files. When present, these are used to build the Catalog. |
| Queries | `queries` | list of Source | no | Named query files (annotated with `-- name: X :one/:many/:exec`). |
| Migrations | `migrations` | list of MigrationSource | no | Up-migration SQL files. Used as the schema source when no `schema` entries are present; always passed to plugins as migration objects. |
| Plugins | `plugins` | list of PluginConfig | no | Code-generation plugins to run (see [Plugins](#plugins)). |

---

### `options`

Host-wide settings applied to every generation run.

| Field | YAML key | Type | Default | Description |
|-------|----------|------|---------|-------------|
| DefaultSchema | `defaultSchema` | string | `"public"` | The PostgreSQL schema assumed when a table or type is unqualified. |
| SearchPath | `searchPath` | list of string | — | Additional schemas to search when resolving unqualified names. |
| TypeOverrides | `typeOverrides` | map[string]string | — | Override the Go type for a specific SQL type (key: SQL type name, value: Go type path). |
| Strict | `strict` | bool | `false` | When true, unresolved column types and other warnings are treated as errors. |

---

### `schema` and `queries` — Source fields

Each entry in `schema` or `queries` must set **exactly one** of `file`, `dir`, or
`inline`. `dir` sources default their `glob` to `*.sql` when `glob` is omitted.

| Field | YAML key | Type | Description |
|-------|----------|------|-------------|
| File | `file` | string | Path to a single SQL file. |
| Dir | `dir` | string | Directory of SQL files. Matched by `glob` (default `*.sql`). |
| Inline | `inline` | string | SQL text written directly in the YAML. |
| Glob | `glob` | string | Glob pattern applied when `dir` is set (e.g. `**/*.sql`). Defaults to `*.sql`. |
| Recursive | `recursive` | bool | When true, walk `dir` recursively. |

---

### `migrations` — MigrationSource fields

Each migration source requires a `dir`. The `glob` defaults to `*.sql`.

| Field | YAML key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Dir | `dir` | string | yes | Directory containing up-migration SQL files. |
| Glob | `glob` | string | no | Glob pattern within `dir`. Defaults to `*.sql`. |

---

### `plugins`

Each entry in `plugins` describes one code-generation plugin. Exactly one of
`binary`, `command`, or `wasm` must be set, and `out` is required.

| Field | YAML key | Type | Required | Description |
|-------|----------|------|----------|-------------|
| Name | `name` | string | yes | Identifier for this plugin (used in error messages). |
| Command | `command` | string | one of | Bare executable name resolved via `$PATH` (e.g. `sqld-gen-go`). Use for plugins installed with `go install`. |
| Binary | `binary` | string | one of | Filesystem path to an executable. Relative paths are resolved from the `sqld` process's working directory, not the `sqld.yaml` location. |
| Wasm | `wasm` | string | one of | Filesystem path to a `.wasm` WASI module. Same cwd-relative resolution as `binary`. Executed via embedded wazero — no external runtime needed. |
| Out | `out` | string | yes | Output directory where the plugin's generated files are written. Created automatically if it does not exist. |
| Args | `args` | list of string | no | Extra command-line arguments passed to the plugin process. |
| Options | `options` | map | no | Plugin-specific options serialized as JSON and passed in the `GenerateRequest`. Interpreted by the plugin. |
| Env | `env` | map[string]string | no | Additional environment variables set for the plugin process (merged on top of the host environment). |
| SHA256 | `sha256` | string | no | Expected SHA-256 hex digest of the plugin binary. Reserved for future integrity verification (not currently enforced). |
| Enabled | `enabled` | bool | no | Set to `false` to skip this plugin without removing its config. Defaults to `true` when omitted. |

**Plugin location resolution summary**

| Field | Resolution |
|-------|-----------|
| `command` | `$PATH` lookup (Go `exec.LookPath`) — portable; use for `go install`'d plugins |
| `binary` | Used as-is; relative to the `sqld` process cwd |
| `wasm` | Same cwd-relative rule as `binary`; run via wazero |

See the main [README](../../README.md#how-the-plugin-is-located) for full details.

---

### Complete annotated example

```yaml
version: "1"
engine: postgresql       # only supported value; may be omitted

options:
  defaultSchema: public  # unqualified names resolve here (default: public)
  searchPath:            # extra schemas searched after defaultSchema
    - extensions
  typeOverrides:         # override Go type for a SQL type
    citext: string
  strict: false          # treat warnings as errors

# Declarative schema: the source of truth for the Catalog.
# Use schema OR migrations as the primary schema source (not both required).
schema:
  - file: schema.sql                   # single file
  - dir: schema/                       # all *.sql in schema/
    glob: "*.sql"
    recursive: false
  - inline: |                          # inline DDL (useful for tests)
      CREATE TABLE events (id bigserial PRIMARY KEY, name text NOT NULL);

# Named queries (annotated with -- name: X :one/:many/:exec).
queries:
  - dir: queries/        # default glob *.sql
  - file: queries/special.sql

# Migration files: passed to plugins as ordered migration objects.
# Also used as the schema source when no `schema` entries exist.
migrations:
  - dir: migrations/
    glob: "*.sql"

plugins:
  # sqld-gen-go: generate type-safe Go query functions
  - name: go
    command: sqld-gen-go   # resolved via $PATH after: go install …/cmd/sqld-gen-go@latest
    out: gen/db
    options:
      package: db
      emitExactTableNames: false

  # sqld-gen-bob: generate bob ORM models from the same Catalog
  - name: bob
    command: sqld-gen-bob
    out: gen/bobdb
    options:
      package: bobdb

  # Example: local binary path (run sqld from the repo root after make build)
  - name: go-local
    binary: ./bin/sqld-gen-go
    out: gen/db-local
    enabled: false        # skipped; remove or set true to activate

  # Example: WASM plugin (no external runtime — embedded wazero)
  - name: go-wasm
    wasm: ./bin/sqld-gen-go.wasm
    out: gen/dbwasm
    options:
      package: dbwasm
    enabled: false

  # Example: plugin with env overrides and extra args
  - name: custom
    command: my-sqld-plugin
    args: ["--verbose"]
    env:
      MY_PLUGIN_TOKEN: "secret"
    out: gen/custom
    sha256: "abc123..."   # reserved for future integrity check
```

---

## How generation works

1. **Resolve sources** — all `schema`, `queries`, and `migrations` entries are expanded
   to a list of SQL text units (files read, globs matched, inline text extracted).

2. **Build the Catalog** — schema DDL units (or migration-up units when no `schema`
   is configured) are parsed into an IR `Catalog` of tables, columns, types, indexes,
   and relationships. Query units are parsed and their parameter/result types are
   inferred against the Catalog.

3. **Invoke plugins** — for each enabled plugin in config order, `sqld` opens a
   `Runner` (binary/command → subprocess over stdio; wasm → wazero), calls
   `GetInfo` to retrieve plugin metadata and any annotation schema, then sends a
   `GenerateRequest` (Catalog + queries + migrations + serialized options + output
   dir) over stdio-framed protobuf.

4. **Write files** — files in the plugin's `GenerateResponse` are written under
   `plugins[i].out`. Subdirectories are created automatically. Files marked
   executable receive mode `0755`; others `0644`.

5. **Error handling** — if a plugin process exits non-zero, or its response contains
   any error-severity diagnostic, generation aborts immediately.

---

## Library use

The `pkg/sqld` package exposes the same pipeline programmatically:

```go
import "github.com/yaroher/sqld/pkg/sqld"

// Load and validate sqld.yaml
cfg, err := sqld.LoadConfig("sqld.yaml")

// Collect only the IR Catalog (no plugins)
catalog, err := sqld.Collect(cfg)

// Collect Catalog + queries + migrations + diagnostics
result, err := sqld.CollectAll(cfg)  // returns *sqld.Result

// Run the full generation pipeline (Collect + all plugins)
err = sqld.Generate(cfg)

// Convenience wrappers that load the config file internally
catalog, err = sqld.CollectFile("sqld.yaml")
err           = sqld.GenerateFile("sqld.yaml")
```

Proto types are under `github.com/yaroher/sqld/pkg/proto/sqld/v1/{ir,plugin}`.
Config struct is under `github.com/yaroher/sqld/pkg/config`.

---

## See also

- [`docs/cmd/sqld-gen-go.md`](sqld-gen-go.md) — Go query-function generator plugin
- [`docs/cmd/sqld-gen-bob.md`](sqld-gen-bob.md) — bob ORM generator plugin
- [`docs/migrations.md`](../migrations.md) — `sqld migrate` and the migration system
- [`docs/adr.md`](../adr.md) — Architecture Decision Records
