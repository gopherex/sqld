# sqld architecture

How the whole thing fits together: the pieces of sqld, what they produce and
consume, and why the layout is what it is.

## 1. Overview

**sqld is a fully-open PostgreSQL toolkit** — an open alternative to
**sqlc + Atlas**. Where sqlc lacks dynamic queries and Atlas paywalls
`migrate diff` and lint, sqld ships typed Go codegen, dynamic queries, and
schema-diff migrations together under one permissive licence. Nothing is behind
a paywall.

The organizing idea is a single **PostgreSQL-faithful semantic IR**, expressed
in protobuf, called the `Catalog`. It is the hub of the system: every component
either **produces** the IR or **consumes** it.

- SQL DDL, named queries, and migrations are *parsed* into the IR.
- Code generators (plugins) *consume* the IR to emit code.
- The migrator *diffs* two IRs (desired vs. actual) to produce migrations.

Because the IR is the contract, the parsing front-end, the generators, and the
migrator are all decoupled from one another. A generator never sees raw SQL or
an AST library; it sees a `Catalog` (plus parsed queries and migrations). A
plugin can be written in any language that can speak protobuf over stdio.

A standout capability that falls out of the IR-as-hub design is the
**ORM ⊕ sqlc symbiosis**: the built-in Go generator `sqld-gen-go` (sqlc-style
typed queries) and the bob ORM generator `sqld-gen-bob` share **one canonical
Go type per column** and run on **one `*pgxpool.Pool`** — because both resolve
column types through the same `pkg/gotypes` mapper over the same `Catalog`.

## 2. The IR hub

The IR `Catalog` (`pkg/proto/sqld/v1/ir`) is the single semantic model that
producers fill and consumers read.

```
            PRODUCERS                       HUB                    CONSUMERS
   ┌───────────────────────────┐                      ┌──────────────────────────────┐
   │ SQL parse (schema/queries/ │                      │ sqld-gen-go  (typed queries)  │
   │ migrations)                │──┐                ┌─▶│   + leaf Go types             │
   │   internal/{parse,mapper,  │  │   ┌─────────┐  │  ├──────────────────────────────┤
   │   catalog,query,relate}    │  ├──▶│ Catalog │──┤  │ sqld-gen-bob (bob ORM)        │
   ├───────────────────────────┤  │   │  (IR)   │  │  │   refs sqld-gen-go's db.* types│
   │ live-DB introspect         │  │   └─────────┘  │  ├──────────────────────────────┤
   │   internal/introspect      │──┘        │       └─▶│ any 3rd-party plugin          │
   │   (pg_catalog over pgx)    │           │          └──────────────────────────────┘
   └───────────────────────────┘           │
                                            ▼
                                   ┌───────────────────┐
                                   │ sqld migrate diff  │  Diff(from, to) → migration SQL
                                   │   internal/diff    │  (desired IR  vs  actual IR)
                                   └───────────────────┘
```

Two producers yield the *same* `Catalog` shape, by design: `internal/catalog`
builds it from parsed DDL (the *desired* state), and `internal/introspect`
builds it from a live database via `pg_catalog` (the *actual* state). Because
they are interchangeable, `internal/diff` can compare them directly to generate
a migration.

### What the IR models (PG-faithful)

The IR's job is to be a faithful, lossless model of a PostgreSQL database, not
a lowest-common-denominator abstraction. Type names are stored as their
canonical PostgreSQL names (`int4`, `varchar`, `_int4`, ...). The proto package
`sqld.v1.ir` is split into focused files:

- `catalog.proto` — `Catalog` (database snapshot) → `Schema` (PG namespace) →
  tables, views, materialized views, sequences, functions, procedures,
  triggers, and user-defined types; plus a derived `relationships` graph.
- `schema.proto` — `Table` / `Column` / `Constraint` (primary key, foreign key,
  unique, check, exclusion, not-null) / `Index` / `Sequence`, with identity and
  generated columns, referential actions, match types, opclasses, partial and
  covering indexes.
- `type.proto` — `TypeRef`: a fully-resolved type reference at a use site,
  PG-faithful, with `TypeKind` (scalar/array/enum/domain/composite/range/
  pseudo/user-defined), array element + dimensions, parametrized modifiers
  (numeric precision/scale, varchar length, datetime tz/precision, interval),
  and an `ObjectRef` to the defining UDT.
- `routine.proto` — user-defined types (`EnumType`, `DomainType`,
  `CompositeType`, `RangeType`), `View` / `MaterializedView`, `Function` /
  `Procedure` (args, return shapes, volatility, parallel safety, null behavior),
  and `Trigger`.
- `dml.proto` — write statements (`InsertStmt`, `UpdateStmt`, `DeleteStmt`,
  `MergeStmt`) plus a `RawStatement` lossless fallback for anything not
  structured.
- `expr.proto` — the shared recursive `Expr` tree (and `SelectStmt`, colocated
  with it), reused everywhere an expression appears: defaults, checks, generated
  columns, index expressions, view bodies, and all DML clauses. A `raw_sql`
  escape hatch keeps it lossless.
- `relation.proto` — `Relationship` edges (one-to-one, one-to-many,
  many-to-one, many-to-many via join table) *derived* from FK metadata for
  codegen convenience; the FKs themselves remain the source of truth.
- `common.proto` — cross-cutting primitives: `QualifiedName`, `ObjectRef`
  (stable by-id references), `Engine`, `SourceSpan`, `Metadata`, and the parsed
  annotation value model (`AnnotationValue` etc.).

Objects reference each other by **stable id** (`ObjectRef.id`, e.g.
`public.users`, `public.users.id`) rather than by pointer, which keeps the
graph serializable and lets it survive the protobuf wire boundary to a plugin.

The rationale behind these modeling choices is recorded in
[docs/adr.md](adr.md) — e.g. PG-faithful fidelity (ADR-0003), references by
stable id (ADR-0005), FK-truth + derived relationships (ADR-0006), and
statement-level losslessness via `RawStatement` (ADR-0026).

## 3. Pipeline: SQL → IR

The host turns configured SQL sources into a fully-resolved `Catalog`, a list
of named `Query` objects, and a list of `Migration` objects. This is
orchestrated by `core.Gather` in `internal/core/core.go`, which runs these
stages:

1. **Resolve sources** — `internal/source` reads the `sqld.yaml` config and
   expands files / dirs / globs / inline SQL into `Unit`s, tagged as schema,
   migration-up, or query.
2. **Parse** — `internal/parse` wraps `libpg_query` (via
   `github.com/wasilibs/go-pgquery` / `pganalyze/pg_query_go`) to turn SQL text
   into PostgreSQL's own AST (`parse.Statements`).
3. **Map** — `internal/mapper` translates the libpg_query AST nodes into IR
   proto messages: DDL (`ddl.go`), DML statements (`stmt.go`), expressions
   (`expr.go`), and type references (`types.go`). Every AST node gets a stable,
   deterministic `node_id` from `internal/nodeid`.
4. **Build catalog** — `internal/catalog` (`Build`) assembles the mapped DDL
   into a `Catalog`: grouping objects into schemas, assigning canonical ids,
   wiring `ObjectRef`s, and attaching indexes/constraints to their tables. It
   accumulates non-fatal `Diagnostics` rather than panicking.
5. **Parse + infer queries** — `internal/query` parses sqlc-style named query
   blocks (`-- name: GetUser :one`), rewrites `@name` named parameters to
   positional `$N` placeholders (`named.go`), and then `Infer` walks the
   statement AST against the catalog to deduce each query's bind parameters
   (`QueryParameter`) and output columns (`QueryColumn`), including nullability
   and originating column.
6. **Derive relationships** — `internal/relate` (`Derive`) inspects FK
   constraints to produce the deterministic `Relationship` graph (detecting
   join tables for many-to-many), which is stamped onto `Catalog.relationships`.

The result is a `core.Result` (catalog + queries + migrations + diagnostics +
source units). `core.Collect` is the catalog-only convenience wrapper;
`core.Generate` continues on to run plugins.

The live-database path is separate: `internal/introspect` queries `pg_catalog`
through a pgx handle to build a `Catalog` of the *actual* database. It uses the
same id conventions as `internal/catalog` so the two are diffable.

## 4. Plugin architecture

Code generators are **plugins**. The contract is the protobuf `Generator`
service in `pkg/proto/sqld/v1/plugin` (`plugin.proto`):

```proto
service Generator {
  rpc GetInfo(GetInfoRequest) returns (GetInfoResponse);   // identity + capabilities
  rpc Generate(GenerateRequest) returns (GenerateResponse); // IR → files
}
```

`GetInfo` is a cheap handshake: the plugin reports its name, version, supported
engines, capabilities, and — importantly — its **annotation grammar**.
`Generate` receives the IR (`catalog`, parsed `queries`, ordered `migrations`),
opaque plugin `options` bytes, an `out_dir`, host `context`, and the parsed
`annotations`; it returns `GeneratedFile`s and `Diagnostic`s (a single
ERROR-severity diagnostic fails the run).

### Wire format

The transport is **stdio-framed protobuf** — *not* full gRPC-over-stdio.
For each RPC the host runs the plugin and frames one message each way:

```
stdin   = [1-byte method tag] [serialized proto request]   tag 0 = GetInfo, 1 = Generate
stdout  =                     [serialized proto response]
stderr  = captured for error diagnostics only
```

The plugin reads the first byte to dispatch, unmarshals the request, and writes
the marshaled response to stdout (see `cmd/sqld-gen-go/main.go` and
`cmd/sqld-gen-bob/main.go` for the canonical ~50-line plugin loops). The host
side lives in `internal/plugin`.

### Three transports

`internal/plugin/runner.go` defines the `Runner` interface and `Open` selects
the transport from the plugin's config:

- **binary** (`binary.go`) — an exec'd executable resolved by absolute/relative
  path; one process per RPC, stateless.
- **command** (`binary.go`) — identical mechanics, but the executable is
  resolved from `$PATH` (e.g. after `go install`). Same `binaryRunner`.
- **wasm** (`wasm.go`) — a WASI command module run in-process via
  `tetratelabs/wazero`. The compiled module is cached; each RPC instantiates a
  fresh instance. It speaks the **identical** tag-byte + protobuf framing over
  the module's stdin/stdout, so wasm and native plugins are wire-compatible.

Because the only coupling is the public proto contract, plugins can be written
in any language with protobuf support; no dependency on sqld's Go internals is
required.

### Annotation system

Plugins do not parse SQL comments themselves. Instead a plugin **declares its
grammar** in `GetInfoResponse.annotation_schema` (`annotation.proto`): a sigil
(e.g. `@`), comment styles to scan, and a set of `AnnotationDef`s each with a
form (flag / scalar / positional / keyed / list / freeform), targets, and field
specs. The host (`internal/plugin/annotate.go`) is a **generic parser** driven
by that schema; it scans comments and returns structured `AnnotationValue`s
(the value model lives in `ir/common.proto` so `Metadata` can embed it without
depending on the plugin package). Parsed annotations are delivered both as a
flat list on `GenerateRequest.annotations` and mirrored onto each object's
`Metadata.annotations`. AST `node_id`s let an annotation bind to a specific part
of a query.

For host CLI usage and configuration see [docs/cmd/sqld.md](cmd/sqld.md).

## 5. The ORM ⊕ sqlc symbiosis

The headline integration is running **two** Go generators against one IR such
that their outputs interlock instead of conflict.

```
schema.sql + queries → sqld host → IR Catalog
        ├── sqld-gen-go  → query rows/params, dynamic queries, AND the shared
        │                  leaf types (enums, composites, domains, RegisterTypes)
        └── sqld-gen-bob → bob models / relationships / where / loaders / joins
                           (each Column.Type resolved to sqld-gen-go's db.* type)
```

The keystone is **`pkg/gotypes`** — a single pg→Go type mapper used by **both**
generators. It turns a PG-faithful `TypeRef` into a Go type expression (plus the
imports it needs), driven by a UDT `Registry` built from the catalog, an
`Overrides` table, and a `NullMode`.

- **`sqld-gen-go`** owns the query code (typed row structs, `*Queries`
  methods, dynamic-query builders) **and** the *leaf types*: enum types,
  composite structs, domain aliases, and the `RegisterTypes` function that
  teaches pgx the custom OIDs. It emits these into a package (referred to here
  as `db`).
- **`sqld-gen-bob`** drives stephenafamo/bob's generator from the same
  `Catalog`, but configures `gotypes` to **qualify** enum/composite Go names
  into `sqld-gen-go`'s package via `Mapper.SetUDTPackage` (the `typesPackage`
  option). bob's own enum generation is left empty. So `users.status` is
  `db.AppUserStatus` in *both* the bob model and the sqld query row — one
  canonical Go type per column.

Because the leaf types and `RegisterTypes` come from one place, both layers run
on **one `*pgxpool.Pool`** with one type registration. The only seam is
nullability wrapping: `sqld-gen-go` uses `NullMode = Pointer` (`*T`) while
`sqld-gen-bob` uses `NullMode = Opt` (`null.Val[T]`). To bridge them,
`sqld-gen-bob` emits a `ToSqld()` method on each bob model (see
`cmd/sqld-gen-bob/bridge.go`) that field-copies the bob model into
`sqld-gen-go`'s flat model, unwrapping `null.Val[T]` (`.Ptr()` for wrapped
scalars, `.GetOrZero()` for nil-capable value types). bob writes its multi-file
output directly to its `out` dir, so the plugin returns no files to the host.

See [docs/cmd/sqld-gen-bob.md](cmd/sqld-gen-bob.md) for the full setup, and
[docs/types.md](types.md) for the pg→Go mapping table.

## 6. Repository & module layout

```
sqld/                                   module github.com/yaroher/sqld   (core)
├── go.mod                              core deps: pgx, pg_query, wazero, grpc/proto, testcontainers
├── go.work                            ties the two modules together
├── proto/sqld/v1/{ir,plugin}/*.proto  IR + plugin contract source
├── pkg/
│   ├── proto/sqld/v1/{ir,plugin}/     generated Go for the protos (irv1, pluginv1)
│   ├── gotypes/                       SHARED pg→Go type mapper (symbiosis keystone)
│   ├── sqld/                          public API: LoadConfig / Collect / Generate
│   ├── config/                        sqld.yaml config structs + loader
│   ├── migrate/                       importable migrator library (pgx)
│   └── devdb/                         ephemeral Postgres (testcontainers / --dev-url)
├── internal/
│   ├── source/  parse/  mapper/  nodeid/  catalog/  query/  relate/   (SQL → IR pipeline)
│   ├── introspect/                    live DB → IR (pg_catalog)
│   ├── diff/                          IR diff → migration plan/SQL
│   ├── lint/                          destructive/risky-change linting
│   └── core/                          Collect / Gather / Generate orchestration
├── cmd/
│   ├── sqld/                          host CLI (init / generate / collect / migrate)
│   ├── sqld-gen-go/                   built-in Go (sqlc-style) generator plugin
│   └── sqld-gen-bob/                  bob ORM generator   ── NESTED MODULE ──
│       ├── go.mod                     module github.com/yaroher/sqld/cmd/sqld-gen-bob
│       │                              replace github.com/yaroher/sqld => ../../
│       └── ...                        bridge.go, driver.go, generate.go, options.go
└── example/                           worked example: schema, queries, migrations, generated output
```

### Why two modules

`cmd/sqld-gen-bob` is a **separate, nested Go module**
(`github.com/yaroher/sqld/cmd/sqld-gen-bob`) rather than part of the root
module. The reason: the bob ORM (`github.com/stephenafamo/bob`) and its support
libraries (`aarondl/opt`, `stephenafamo/scan`, sprig/koanf, ...) are a large
dependency that only matters to projects opting into the ORM. Isolating it in a
nested module keeps the **core `go.mod` free of bob** — projects that only use
`sqld-gen-go` never pull bob.

The nested module declares `replace github.com/yaroher/sqld => ../../` so it
builds against the in-tree core. A root **`go.work`** ties both modules together
for local development:

```
go 1.25.0

use (
	.
	./cmd/sqld-gen-bob
)
```

### Installing the three binaries

```sh
go install github.com/yaroher/sqld/cmd/sqld@latest
go install github.com/yaroher/sqld/cmd/sqld-gen-go@latest
go install github.com/yaroher/sqld/cmd/sqld-gen-bob@latest   # nested module — keeps bob out of the core go.mod
```

The migrator is **not** a separate binary — it ships inside `sqld` as the
`sqld migrate` subcommand.

## 7. The binaries

| Binary | Role | Docs |
| --- | --- | --- |
| **`sqld`** | The **host**. `init` scaffolds a project; `collect` parses sources and prints the IR (json/prototext); `generate` runs the configured plugins against the IR and writes their output. This is the orchestrator that drives `internal/core`. | [docs/cmd/sqld.md](cmd/sqld.md) |
| **`sqld-gen-go`** | The built-in **Go code generator** plugin (sqlc-style). Produces typed query rows + `*Queries` methods backed by pgx v5, dynamic-query builders, and the canonical leaf Go types (enums/composites/domains + `RegisterTypes`) shared with bob. | [docs/cmd/sqld-gen-go.md](cmd/sqld-gen-go.md) |
| **`sqld-gen-bob`** | The **bob ORM** generator plugin (nested module). Drives stephenafamo/bob from the same IR to produce models, relationships with eager loading, and typed where/loaders/joins, referencing `sqld-gen-go`'s shared types and adding a `ToSqld()` bridge. | [docs/cmd/sqld-gen-bob.md](cmd/sqld-gen-bob.md) |
| **`sqld migrate`** | The **migrator** — a subcommand of `sqld` (not a separate binary). Apply / revert / inspect migrations, and `generate <name>` to diff the declarative `schema.sql` against the current migration history (via ephemeral Postgres) and emit a new migration with up + down DDL. Lint reports risky/destructive changes. | [docs/cmd/sqld-migrate.md](cmd/sqld-migrate.md) |

`sqld-gen-go` and `sqld-gen-bob` are themselves just plugins speaking the
contract from §4; `sqld` discovers and runs them like any third-party plugin.

## 8. See also

- [docs/adr.md](adr.md) — Architecture Decision Records (the *why* behind the IR
  and plugin design).
- [docs/types.md](types.md) — PostgreSQL → Go type mapping and overrides.
- [docs/migrations.md](migrations.md) — migration files, diff generation, and
  the `pkg/migrate` library.
- [docs/cmd/sqld.md](cmd/sqld.md) — the host CLI.
- [docs/cmd/sqld-gen-go.md](cmd/sqld-gen-go.md) — the built-in Go generator.
- [docs/cmd/sqld-gen-bob.md](cmd/sqld-gen-bob.md) — the bob ORM generator.
- [docs/cmd/sqld-migrate.md](cmd/sqld-migrate.md) — the `sqld migrate` subcommand.
