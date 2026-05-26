# Architecture Decision Records — sqld

PostgreSQL semantic IR (protobuf) + plugin-driven code generation.

Each record: **Status · Context · Decision · Consequences**. Statuses: `accepted`
(decided this session), `deferred` (intentionally out of scope, revisit later).

The companion design doc with full rationale and examples lives at
`docs/superpowers/specs/2026-05-24-postgresql-semantic-ir-design.md`.

## Index

- [ADR-0001 — PostgreSQL semantic IR in protobuf](#adr-0001)
- [ADR-0002 — Scope of modeled objects](#adr-0002)
- [ADR-0003 — PG-faithful fidelity](#adr-0003)
- [ADR-0004 — Target-agnostic, plugin-consumed IR](#adr-0004)
- [ADR-0005 — References by stable id](#adr-0005)
- [ADR-0006 — Relationships: FK truth + derived graph](#adr-0006)
- [ADR-0007 — DML: structured + raw_sql fallback](#adr-0007)
- [ADR-0008 — SELECT colocated with Expr](#adr-0008)
- [ADR-0009 — File layout `v1/<folder>` and packages](#adr-0009)
- [ADR-0010 — Plugin contract is a gRPC service, two transports](#adr-0010)
- [ADR-0011 — Plugin options are opaque bytes](#adr-0011)
- [ADR-0012 — Host feeds schema + queries](#adr-0012)
- [ADR-0013 — Engine enum in `common.proto`](#adr-0013)
- [ADR-0014 — Config is a proto document](#adr-0014)
- [ADR-0015 — MigrationSource: directory only, native format](#adr-0015)
- [ADR-0016 — Annotations: plugin declares grammar, host parses](#adr-0016)
- [ADR-0017 — Annotation value model: generic forms](#adr-0017)
- [ADR-0018 — Annotation delivery: both flat list and Metadata](#adr-0018)
- [ADR-0019 — Value model in ir, grammar in plugin](#adr-0019)
- [ADR-0020 — Dynamic queries: foundation only, plugin owns semantics](#adr-0020)
- [ADR-0021 — Stable AST `node_id` for annotation binding](#adr-0021)
- [ADR-0022 — Proto→Go codegen via easyp into `pkg/proto`](#adr-0022)
- [ADR-0023 — Three binaries: sqld, sqld-migrate, sqld-gen-go](#adr-0023)
- [ADR-0024 — `pkg/migrate` importable migrator library](#adr-0024)
- [ADR-0025 — Go module path & go_package alignment](#adr-0025)
- [ADR-0026 — Statement-level losslessness: MERGE + RawStatement](#adr-0026)
- [ADR-0027 — Core engine implemented; plugin transport is stdio-framed protobuf](#adr-0027)
- [ADR-0028 — Dynamic queries implemented (named params, smart WHERE, typed @orderby)](#adr-0028)
- [ADR-0029 — sqld-gen-go Go type mapping (scalars, UDTs, json, range, overrides)](#adr-0029)
- [ADR-0030 — sqld-migrate: open migrator + schema-diff via dev Postgres](#adr-0030)
- [ADR-0031 — ORM ⊕ sqlc symbiosis via bob (sqld-gen-bob + pkg/gotypes)](#adr-0031--orm--sqlc-symbiosis-via-bob-sqld-gen-bob--pkggotypes)
- [Semantics reference](#semantics-reference)

---

<a id="adr-0001"></a>
## ADR-0001 — PostgreSQL semantic IR in protobuf

**Status:** accepted

**Context:** Need a machine-readable intermediate representation of a PostgreSQL
database that downstream tools can generate from (SQL, code, docs).

**Decision:** Model the IR as protobuf (proto3). The serialized `Catalog` message
is the unit of exchange.

**Consequences:** Language-neutral, versioned wire format, free codegen for any
language. Schema evolution governed by proto field-number rules.

---

<a id="adr-0002"></a>
## ADR-0002 — Scope of modeled objects

**Status:** accepted

**Context:** PostgreSQL surface is huge; modeling everything up front is wasteful.

**Decision:** In scope: schemas (namespaces), tables, columns, types,
constraints (PK/FK/unique/check/exclusion/not-null/default), indexes, sequences;
programmable objects (views, materialized views, functions, procedures,
triggers, enum/domain/composite/range types); DML (SELECT/INSERT/UPDATE/DELETE);
comments and object metadata.

**Deferred:** RLS policies, partitioning, extensions, tablespaces, roles/grants,
collation objects. Field numbers are reserved generously so these can be added
without breaking the wire format.

**Consequences:** Covers the relational + programmable surface most generators
need. Advanced features deferred but non-blocking.

---

<a id="adr-0003"></a>
## ADR-0003 — PG-faithful fidelity

**Status:** accepted

**Context:** Choice between faithfully modeling PostgreSQL semantics vs an
abstract, portable type model.

**Decision:** PG-faithful. `TypeRef` carries the canonical `pg_name` (`int4`,
`varchar`, `_int4`). PG-specific options modeled explicitly (referential
actions, volatility, identity, storage, etc.).

**Consequences:** Lossless for PostgreSQL; not portable to other dialects without
a mapping layer (acceptable — this is a "postgresql dialect" IR).

---

<a id="adr-0004"></a>
## ADR-0004 — Target-agnostic, plugin-consumed IR

**Status:** accepted

**Context:** Many possible outputs (SQL DDL, ORM models, docs, diagrams).

**Decision:** Keep the IR neutral. Generation is done by plugins that consume a
`Catalog`. The IR encodes no target-specific concepts.

**Consequences:** One IR, many generators. Per-generator hints ride on
`Metadata.options` and opaque plugin `options` rather than IR schema changes.

---

<a id="adr-0005"></a>
## ADR-0005 — References by stable id

**Status:** accepted

**Context:** Objects reference each other (FK → table, trigger → function).

**Decision:** Every object has a stable `id` (e.g. `"public.users"`,
`"public.users.id"`) plus a structured `QualifiedName`. Cross-object references
use `ObjectRef{id, kind, name}`.

**Consequences:** Robust references that survive reordering; generators resolve
by id. Host must assign deterministic ids.

---

<a id="adr-0006"></a>
## ADR-0006 — Relationships: FK truth + derived graph

**Status:** accepted (fork 1c)

**Context:** Generators want relationship/association info; foreign keys are the
source but require derivation (cardinality, join-table detection).

**Decision:** Keep foreign keys on tables as constraints (lossless source of
truth). Additionally emit a derived `Relationship` graph in `Catalog`:
`ONE_TO_ONE` / `ONE_TO_MANY` / `MANY_TO_ONE` / `MANY_TO_MANY`, join-table
detection, optionality, suggested name.

**Consequences:** Generators get associations for free without re-deriving;
fidelity preserved in the FK constraints. Host computes the graph.

---

<a id="adr-0007"></a>
## ADR-0007 — DML: structured + raw_sql fallback

**Status:** accepted (fork 2b)

**Context:** Full structured modeling of every PG expression/statement is large;
pure raw SQL loses semantics.

**Decision:** Model common shapes with a shared recursive `Expr` tree and
statement messages. Every `Expr` and `SelectStmt` carries a `raw_sql` escape
hatch for constructs not yet structured.

**Consequences:** Lossless by construction, incremental coverage. Generators must
handle `raw_sql` fallback where present.

---

<a id="adr-0008"></a>
## ADR-0008 — SELECT colocated with Expr

**Status:** accepted

**Context:** `Expr` (scalar/EXISTS subqueries) and `SelectStmt` are mutually
recursive; protobuf forbids circular *file* imports.

**Decision:** Define `Expr` and `SelectStmt` (+ query clauses) in one file,
`ir/expr.proto`. Write statements (`ir/dml.proto`) depend on it one-directionally.

**Consequences:** Acyclic file graph. `expr.proto` is large but cohesive.

---

<a id="adr-0009"></a>
## ADR-0009 — File layout `v1/<folder>` and packages

**Status:** accepted

**Context:** Started with mixed layout (`config/v1`, `plugin/v1`, flat core).

**Decision:** Version-first layout: every component is a folder under `v1` with a
matching package.

| folder | package |
|--------|---------|
| `proto/sqld/v1/ir/` | `sqld.v1.ir` |
| `proto/sqld/v1/config/` | `sqld.v1.config` |
| `proto/sqld/v1/plugin/` | `sqld.v1.plugin` |

Dependency graph (acyclic): `common ← type ← expr ← schema ← routine`,
`expr ← dml`, `common ← relation`, `all ← catalog`. `plugin` and `config`
import `ir`; neither imports the other.

**Consequences:** buf-lint clean (dir == package). `routine` imports `schema`
only for `Index` (materialized-view indexes); `schema` does not import `routine`.

---

<a id="adr-0010"></a>
## ADR-0010 — Plugin contract is a gRPC service, two transports

**Status:** accepted

**Context:** Plugins may be native binaries or WASM modules; need one contract.

**Decision:** Define `service Generator { GetInfo; Generate }` in
`plugin/plugin.proto`. Two transports share the same messages:
- binary / command plugins serve the gRPC service over stdio;
- WASM plugins export `get_info` / `generate` functions taking the serialized
  request and returning the serialized response.

**Consequences:** Single message contract regardless of transport. Host abstracts
the transport.

---

<a id="adr-0011"></a>
## ADR-0011 — Plugin options are opaque bytes

**Status:** accepted

**Context:** Each plugin needs its own settings; choices were `Struct`, opaque
bytes, or both.

**Decision:** Opaque `bytes options` in both `PluginConfig` (config) and
`GenerateRequest` (passed through unchanged). The plugin decodes them into its
own typed message. `GetInfoResponse.options_schema` is an informational
description only.

**Consequences:** Strong typing on the plugin side; host stays agnostic. Config
files are not generically editable without knowing the plugin's schema.

---

<a id="adr-0012"></a>
## ADR-0012 — Host feeds schema + queries

**Status:** accepted

**Context:** Generators may want only the schema, or also named DML queries
(sqlc-style typed query codegen).

**Decision:** The host parses DDL/migrations into a `Catalog` AND parses named
DML queries into `Query{ ast, parameters, columns }`. Both go in
`GenerateRequest`.

**Consequences:** Enables typed query codegen (params + result columns inferred).
Larger host responsibility (query inference).

---

<a id="adr-0013"></a>
## ADR-0013 — Engine enum in `common.proto`

**Status:** accepted

**Context:** Both `config` and `plugin` reference the SQL dialect; neither should
depend on the other.

**Decision:** Define `Engine` (`ENGINE_POSTGRESQL`) in `ir/common.proto`. Both
packages reuse it.

**Consequences:** `config` and `plugin` stay decoupled.

---

<a id="adr-0014"></a>
## ADR-0014 — Config is a proto document

**Status:** accepted

**Context:** Users must declare inputs and plugins for a generation run.

**Decision:** `Config` (serializable as textproto/JSON/YAML) holds: `engine`,
`sql[]` (`SqlSource`: file/dir/inline, schema vs query), `migrations[]`,
`plugins[]` (`PluginConfig` with `PluginSource` = wasm | binary | command), and
`GlobalOptions`.

**Consequences:** One declarative document drives a run. PluginSource carries an
optional `sha256` for integrity and `args`/`env` for process plugins.

---

<a id="adr-0015"></a>
## ADR-0015 — MigrationSource: directory only, native format

**Status:** accepted

**Context:** Migrations are always a folder; multiple third-party formats exist.

**Decision:** `MigrationSource` = `dir` + `glob` only. Dropped the
file/dir `oneof` and the `MigrationFormat` enum. Only the native (our own)
migration format is supported now.

**Deferred:** goose / golang-migrate / dbmate / atlas format support.

**Consequences:** Simpler config now; format support added later without removing
fields.

---

<a id="adr-0016"></a>
## ADR-0016 — Annotations: plugin declares grammar, host parses

**Status:** accepted

**Context:** Plugins need SQL-comment annotations (`-- @name`, `-- @cache ...`).
Each plugin has its own conventions; the plugin should not re-implement comment
parsing.

**Decision:** The plugin declares its annotation grammar (`AnnotationSchema`) in
`GetInfoResponse`. The host parses all SQL comments against that grammar and
returns structured `AnnotationValue`s. The plugin never parses comment text.

**Consequences:** Bidirectional contract (plugin → schema, host → parsed values).
One generic parser in the host serves all plugins.

---

<a id="adr-0017"></a>
## ADR-0017 — Annotation value model: generic forms

**Status:** accepted

**Context:** How expressive should the annotation grammar be.

**Decision:** Generic forms — `FLAG` / `SCALAR` / `POSITIONAL` / `KEYED` /
`LIST` / `FREEFORM` — with typed `FieldSpec`s
(`string/int/float/bool/ident/duration/enum`). Covers sqlc/gorm/ent-style
conventions. The host tokenizes the value text after sigil+name and coerces per
field type.

**Consequences:** Declarative grammar handles the common cases; nested/structured
values not supported (deferred).

---

<a id="adr-0018"></a>
## ADR-0018 — Annotation delivery: both flat list and Metadata

**Status:** accepted

**Context:** Parsed annotations could be delivered as a global list, embedded per
object, or both.

**Decision:** Both. `GenerateRequest.annotations` is the flat global list (keyed
by target, also covers query/migration targets); each catalog object's
`Metadata.annotations` mirrors its own.

**Consequences:** Convenient inline lookup plus a global view. Forced the value
model into `ir` (see ADR-0019).

---

<a id="adr-0019"></a>
## ADR-0019 — Value model in ir, grammar in plugin

**Status:** accepted

**Context:** "Both" delivery means `Metadata` (in `ir/common.proto`) must embed
parsed annotations — but the grammar model is plugin-facing. `ir` must not depend
on `plugin` (would cycle).

**Decision:** Split. The parsed VALUE model (`AnnotationValue`, `AnnotationArg`,
`AnnotationTargetRef`, enums `AnnotationTargetKind` / `AnnotationArgType`) lives
in `ir/common.proto`. The GRAMMAR model (`AnnotationSchema`, `AnnotationDef`,
`AnnotationValueSpec`, `FieldSpec`) lives in `plugin/annotation.proto`, which
imports `ir`. Shared enums live in `ir`.

**Consequences:** `ir` never depends on `plugin`. `Metadata` embeds values
freely.

---

<a id="adr-0020"></a>
## ADR-0020 — Dynamic queries: foundation only, plugin owns semantics

**Status:** accepted

**Context:** sqlc's blocking gap is the lack of dynamic queries (optional
filters, variable `IN`, runtime `ORDER BY`, optional joins, partial `UPDATE`).

**Decision:** The IR does **not** encode dynamic constructs. Dynamic semantics
are the plugin's responsibility (declared via its annotation grammar:
`@if`/`@slice`/`@orderby`...). The host supplies raw material only: full
`Query.ast`, `Query.sql`, typed `QueryParameter`/`QueryColumn`, annotations,
`Catalog`. Result shape stays **fixed** (stable typed row); an ORM may be layered
on top later.

**Consequences:** IR stays simple and generator-neutral. A future generator
builds parameterized dynamic SQL at runtime (omit clauses, `= ANY($n)`,
allowlisted `ORDER BY`, placeholder renumbering). Dynamic projection (varying
SELECT list) intentionally not supported, to keep result typing.

---

<a id="adr-0021"></a>
## ADR-0021 — Stable AST `node_id` for annotation binding

**Status:** accepted

**Context:** An annotation that guards part of a query (an optional predicate,
join) must bind to a specific AST node. Text-span matching is fragile.

**Decision:** Every structural AST node carries a host-assigned `node_id`
(field 21): `Expr` (covers all expressions), `SelectStmt`, `SimpleSelect`,
`SetOperation`, `CommonTableExpr`, `OrderByItem`, `SelectTarget`, `FromItem`,
`JoinClause`; and `Statement`, `Insert`/`Update`/`DeleteStmt`, `Assignment`.
`AnnotationTargetRef.node_id` binds an annotation to the node it decorates; the
plugin resolves it by walking `Query.ast`.

**Consequences:** Robust annotation→node binding, no span matching. `node_id`s
are deterministic (host assigns during parse, e.g. a structural path). Chosen
over source-span correlation for robustness.

---

<a id="adr-0022"></a>
## ADR-0022 — Proto→Go codegen via easyp into `pkg/proto`

**Status:** accepted

**Context:** Need generated Go from the proto IR + plugin service.

**Decision:** `make protocols` runs `easyp mod update && easyp mod vendor`, wipes
`pkg/proto`, then `easyp generate` (config: `proto/easyp.yaml`). Plugins `go` and
`go-grpc` emit to `pkg/proto` with `paths: source_relative`, mirroring
`sqld/v1/{ir,config,plugin}`. Go packages: `irv1`, `configv1`, `pluginv1`. The
gRPC service generates `GeneratorServer` / `GeneratorClient`.

**Consequences:** Generated Go lives under `pkg/proto/...` and is regenerated, not
hand-edited. Import-path alignment is governed by ADR-0025.

---

<a id="adr-0023"></a>
## ADR-0023 — Three binaries: sqld, sqld-migrate, sqld-gen-go

**Status:** accepted

**Context:** The toolchain has distinct roles: orchestration, migrations, and the
built-in Go generator.

**Decision:** Three `main` packages under `cmd/`:

| binary | path | role |
|--------|------|------|
| `sqld` | `cmd/sqld/` | host / orchestrator CLI: load `Config`, parse SQL + migrations into a `Catalog` and `Query`s, run the configured plugins (via the `Generator` contract / transports), write output files |
| `sqld-migrate` | `cmd/sqld-migrate/` | migration CLI (apply / rollback / status); thin wrapper over `pkg/migrate` |
| `sqld-gen-go` | `cmd/sqld-gen-go/` | built-in Go generator plugin; implements the `Generator` service (gRPC-over-stdio) |

**Consequences:** Standard `cmd/<bin>` layout. `sqld-gen-go` is itself a plugin —
dogfoods the plugin contract. None scaffolded yet (proto + generated code exist;
`cmd/` and `go.mod` are next).

---

<a id="adr-0024"></a>
## ADR-0024 — `pkg/migrate` importable migrator library

**Status:** accepted

**Context:** Users must be able to run migrations from their own Go code, not only
via the CLI.

**Decision:** Migration logic lives in an importable `pkg/migrate` library (Go
API: up / down / status / migrate-to-version against a `*sql.DB` or DSN).
`cmd/sqld-migrate` is a thin CLI wrapper over it.

**Consequences:** One implementation serves both programmatic and CLI use. Public
API surface in `pkg/migrate` must stay stable.

---

<a id="adr-0025"></a>
## ADR-0025 — Go module path & go_package alignment

**Status:** accepted (applied)

**Context:** Generated code is emitted to `pkg/proto/sqld/v1/...`, but the proto
`go_package` option currently points at `github.com/yaroher/sqld/proto/sqld/v1/...`
(missing the `pkg/` segment). No `go.mod` exists yet, so nothing breaks today.

**Decision:** Module path is `github.com/yaroher/sqld`. Generated code therefore
imports as `github.com/yaroher/sqld/pkg/proto/sqld/v1/{ir,config,plugin}`. The
proto `go_package` options must be updated to include `pkg/`:

```
github.com/yaroher/sqld/pkg/proto/sqld/v1/ir;irv1
github.com/yaroher/sqld/pkg/proto/sqld/v1/config;configv1
github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin;pluginv1
```

then re-run `make protocols`, and add `go.mod` with the module path.

**Consequences:** Until fixed, `go build` fails — generated files reference each
other through the `.../proto/...` path that does not match their physical
`pkg/proto/...` location. Fix touches the `go_package` line in all 10 proto files
across the 3 packages.

**Applied:** `go_package` updated in all 10 proto files, regenerated via
`make protocols`, `go.mod` added (module `github.com/yaroher/sqld`, go 1.25,
deps `google.golang.org/protobuf` + `google.golang.org/grpc`). `go build ./...`
and `go vet ./pkg/...` pass.

---

<a id="adr-0026"></a>
## ADR-0026 — Statement-level losslessness: MERGE + RawStatement

**Status:** accepted (applied)

**Context:** `Statement` modeled only `select`/`insert`/`update`/`delete` and had
no `raw_sql`. So `MERGE`, `TRUNCATE`, `COPY`, DDL, and transaction-control could
not be represented as a parsed `Statement` — and `Migration.up[]`/`down[]`
(`repeated Statement`) could not carry them at all (only the raw `up_sql`/
`down_sql` text). The statement level was not lossless.

**Decision:** Extend the `Statement` oneof with:
- `MergeStmt` — structured `MERGE` (PG 15+): CTEs, target + alias, `USING` source
  (`FromItem`), `ON` condition, `WHEN` clauses (`MATCHED` /
  `NOT_MATCHED_BY_TARGET` / `NOT_MATCHED_BY_SOURCE` PG 17), per-clause `AND`
  condition, action (INSERT/UPDATE/DELETE/DO NOTHING), and `RETURNING` (PG 17).
- `RawStatement{ StatementKind kind; string sql }` — escape hatch for any
  unmodeled statement. `StatementKind`: MERGE / TRUNCATE / COPY / DDL / TCL /
  UTILITY / OTHER.

**Consequences:** The statement level is now lossless — every statement is either
structured or carried as `RawStatement`. `Migration.up[]`/`down[]` can now hold
DDL (as `RawStatement{kind=DDL, sql=...}`) alongside the raw text. A fully
structured DDL AST (CREATE/ALTER/DROP) remains intentionally absent — the
`Catalog` is the post-DDL snapshot; DDL statements ride as `RawStatement` until a
generator needs more. Applied + verified (protoc + `make protocols` regen +
`go build ./...` + `go vet ./pkg/...` pass).

---

<a id="adr-0027"></a>
## ADR-0027 — Core engine implemented; plugin transport is stdio-framed protobuf

**Status:** accepted (implemented)

**Context:** The `internal/` engine was built per the spec
`docs/superpowers/specs/2026-05-25-core-collect-generate-design.md` (17-task
plan, subagent-driven). `Collect(cfg) (*irv1.Catalog, error)` and
`Generate(cfg) error` are functional end-to-end.

**Decision / outcome:**
- Packages: `internal/{config,source,parse,nodeid,mapper,catalog,query,relate,plugin,core}`. All build/test/vet green.
- Parser dependency is `github.com/wasilibs/go-pgquery` (package name `pg_query`); AST types come from `github.com/pganalyze/pg_query_go/v6`. libpg_query as WASM, no cgo.
- **Plugin transport revises ADR-0010's "gRPC over stdio" detail.** Both the binary/command and the wasm runners use **stdio-framed protobuf**: the host writes one method-tag byte (`0`=GetInfo, `1`=Generate) followed by the serialized request to the plugin's stdin, and reads the serialized response from stdout. The wasm runner compiles the plugin as a `wasip1` command and wires stdin/stdout via wazero (WASI) — so both transports speak identical messages. Full gRPC-over-stdio remains a possible future transport; the `Generator` service definition is unchanged.

**Deferred (known gaps, non-blocking):**
- Query inference is shallow: FROM-subquery star expansion is best-effort; complex expression/column types may be left unresolved (recorded as diagnostics).
- `PluginConfig.Enabled` is currently ignored — all configured plugins run.
- Annotation→`Metadata` mirroring is deferred; parsed annotations are delivered via `GenerateRequest.annotations` only.
- Structured `MERGE` maps to `RawStatement{kind=MERGE}`; advanced DDL maps to `RawStatement{kind=DDL}` (the `Catalog` is the post-DDL snapshot).
- `cmd/` binaries and `pkg/migrate` not yet built (ADR-0023/0024).

---

<a id="adr-0028"></a>
## ADR-0028 — Dynamic queries implemented (named params, smart WHERE, typed @orderby)

**Status:** accepted (implemented)

**Context:** The headline feature vs sqlc (ADR-0020) — dynamic queries (optional
filters, variable IN-lists, runtime ORDER BY) — was only foundation (node_id,
annotation model). It is now implemented end-to-end through the real pipeline.

**Decision / outcome:**
- **Named parameters.** Queries use `@name` (not `$N`). The host rewrites
  `@ident` → `$N` before parsing via a SQL-aware lexer that skips comments,
  string literals, dollar-quotes, and `@`-operators (`@>`, `@@`); the name↔
  position map is kept and each `QueryParameter` carries its `Name`.
- **Optionality on the parameter** (no `@if` directive). A param written
  `@name?` is OPTIONAL: the host's `?`-aware lexer strips the suffix and sets
  `QueryParameter.optional = true`. A condition using `= ANY(@p)` is a SLICE
  (included when non-empty). `-- @orderby col1, col2` (the one remaining
  directive) declares the allowlist for a runtime `ORDER BY`. The base SQL
  (comments ignored by libpg_query) stays valid, so inference works and the
  result shape is fixed.
- **Host parses, plugin interprets** (per ADR-0016/0020): `sqld-gen-go` declares
  only `orderby` (LIST of idents) in its `AnnotationSchema`. The plugin marks a
  WHERE condition optional from its `$N` param's `optional` flag (not a comment),
  detects slices from `ANY($N)`, and emits a builder. Validity is guaranteed at
  generation: the base (all conditions) is parsed by libpg_query in `Collect`
  (invalid → generation fails), and removing whole top-level AND conditions keeps
  the SQL valid by construction — no runtime parsing needed.
- **Generated Go**: a `<Name>Params` struct — `@name?` → `*T` pointer,
  `ANY(@p)` → `[]T` slice, `@orderby` → a typed enum `<Name>OrderBy` (+ a shared
  `OrderDir` ASC/DESC). The method assembles parameterized SQL with a
  `strings.Builder`, collecting included conditions into `conds` and emitting
  **a clean `WHERE c1 AND c2`** only when non-empty (no `WHERE true`, no
  `OR NULL`), **renumbering `$N`** in append order, and appending
  `ORDER BY <enum> <dir>` (sort column from the allowlist enum → injection-safe).
  Fixed typed result row.

**Consequences:** Typed dynamic queries — sqlc's blocking gap — work end-to-end
(`example/queries/search.sql` → `SearchUsers`): named params, clean dynamic
`WHERE`, typed sort enum + direction. UDTs map to Go types in `sqld-gen-go`
(enum → a typed `string` + consts, domain → its base type, composite → a struct;
resolved against the catalog by name) — so an enum param is `AppUserStatus`, a
`text` domain is `string`, not `any`. v1 condition splitting is per-line over a
top-level `WHERE` (one `$N` per condition); nested OR/paren-heavy WHEREs are
best-effort. Composites scan/encode via generated pgx methods
(`ScanIndex`/`ScanNull`/`Index`/`IsNull` + `pgtype.CompositeIndexScanner/Getter`
compile-assertions): composite columns → struct, composite params → encoded
(getter), composite/enum arrays → `[]T` (`address[]` → `[]AppAddress`,
`user_status[]` → `[]AppUserStatus`), and nested composites (a composite field
of composite/enum type → the nested Go struct/type). A generated
`RegisterTypes(ctx, *pgx.Conn)` helper (wired into `pgxpool` `AfterConnect`)
`LoadType`s every enum + composite plus each one's array (`app.address` /
`app._address`) at runtime, ordered dependency-first: enums, then composites
topologically sorted (a composite's field-composites first), element before
array — so nested composites and arrays resolve.

---

<a id="adr-0029"></a>
## ADR-0029 — sqld-gen-go Go type mapping (scalars, UDTs, json, range, overrides)

**Status:** accepted (implemented)

**Context:** How `sqld-gen-go` maps PostgreSQL types (from the IR `TypeRef` +
catalog) to Go, with pgx v5 as the runtime.

**Decision / outcome:**
- **Scalars:** int2/4/8 → int16/32/64; serial variants → same; text/varchar/
  bpchar/char/name/citext → string; numeric/money → string; bool; float4/8;
  timestamp(tz)/date/time(tz) → `time.Time`; uuid → string; bytea → `[]byte`;
  inet/cidr/macaddr → string; unknown → `any`.
- **json/jsonb → `json.RawMessage`** (was `[]byte`).
- **pgx core types** (default-registered, no `RegisterTypes`): `interval` →
  `pgtype.Interval`, geometry (`point`/`line`/`lseg`/`box`/`path`/`polygon`/
  `circle`) → `pgtype.X`, `bit`/`varbit` → `pgtype.Bits`, `macaddr` →
  `net.HardwareAddr`, `tid`/`xid`/`cid` → `pgtype.TID`/`Uint32`. `inet`/`cidr`/
  `uuid` stay `string` (overridable).
- **extension types**: `hstore` → `pgtype.Hstore` (`map[string]*string`),
  `ltree`/`lquery` → `string`. Their runtime OIDs need registration — the
  generator detects usage across columns/queries and `RegisterTypes` `LoadType`s
  them first (no element/array). Unsupported extension types (PostGIS, pgvector)
  → use `overrides`. Full table in `docs/types.md`.
- **Arrays** → `[]elem` (recursing through the resolver, incl. UDT/array-of-enum
  `[]AppUserStatus`, array-of-composite `[]AppAddress`).
- **UDTs** (resolved against the catalog by `PgName`, schema-prefixed Go names):
  enum → a typed `string` + consts; domain → its base type; composite → a struct
  with generated pgx `ScanIndex`/`ScanNull`/`Index`/`IsNull` (compile-asserted
  against `pgtype.CompositeIndexScanner/Getter`). Composite scan/encode/arrays/
  nesting work via a generated `RegisterTypes(ctx, *pgx.Conn)` that `LoadType`s
  enums + composites + their arrays in dependency-first (topological) order;
  wire it into `pgxpool.AfterConnect`.
- **range / multirange** (builtin) → `pgtype.Range[T]` / `pgtype.Multirange[...]`
  (e.g. `int4range` → `pgtype.Range[pgtype.Int4]`). **Custom `CREATE TYPE AS RANGE`**
  is collected into `Schema.Ranges` and maps to `pgtype.Range[<subtype element>]`
  (subtype→pgtype via `pgtypeElement`). Its associated **multirange** (explicit
  `multirange_type_name` or PG's auto-derived name — last `range`→`multirange`,
  else `+_multirange`) maps to `pgtype.Multirange[pgtype.Range[<elem>]]`.
  `RegisterTypes` registers range, its array, multirange, multirange array
  (element-first) via `LoadType`, like composites.
- **Nullability:** nullable → pointer `*T`, EXCEPT `[]byte`, slices, maps,
  `json.RawMessage`, and `pgtype.Range`/`Multirange` (which carry NULL via a
  `Valid` field) — those stay value types.
- **Overrides** (plugin option `overrides`, NOT global config — Go type paths are
  Go-specific): key = a column id (`schema.table.col`) or a pg type name; value =
  `importpath.Type` (imported, referenced `pkg.Type`) or a bare Go type. Precedence:
  column id > type name > default mapping. Lets a column/type map to any Go type
  (`uuid` → `github.com/google/uuid.UUID`, a jsonb column → a domain struct, etc.).

**Consequences:** Strongly-typed generated models/queries for the full type
surface; `example/` exercises enum/domain/composite (+ arrays, nested, params),
json, overrides (`uuid.UUID`, `map[string]any`), and range/multirange. Custom
range collection and `hstore`/other extension types remain future work.

---

<a id="adr-0030"></a>
## ADR-0030 — sqld-migrate: open migrator + schema-diff via dev Postgres

**Status:** accepted (implemented)

**Context:** A fully-open migration tool (Atlas paywalls `migrate diff`, lint,
etc.). Spec: `docs/superpowers/specs/2026-05-26-sqld-migrate-design.md`.

**Decision / outcome:**
- **`pkg/migrate`** (importable): `Migration` model + `Load(dir)`; `Migrator`
  over a pgx `DBTX` with `Up`/`Down`/`To`/`Status`/`Applied`/`Pending`. Each
  migration runs in its own transaction under a session advisory lock; a
  `sqld_migrations(version,name,checksum,applied_at)` table tracks state;
  checksum mismatch on an applied migration → **drift**.
- **Migration file format**: `<version>_<name>.sql` with `-- sqld:up` /
  `-- sqld:down` section markers (no marker → all up). `internal/source` parses
  them; `Unit.DownSQL` added.
- **`internal/introspect`**: builds an `*irv1.Catalog` from a live Postgres via
  `pg_catalog` — the SAME IR as the parser produces, so the diff engine compares
  parsed vs introspected uniformly.
- **`internal/diff`** (DB-free, golden-tested): `Diff(from,to *irv1.Catalog)
  (*Plan,error)` → ordered `Change`s with `UpSQL`/`DownSQL`; covers schemas,
  types (enum/domain/composite/range), sequences, tables, columns, constraints
  (PK/FK/unique/check/exclusion), indexes, views, matviews, functions,
  procedures, triggers; dependency-ordered (schema→type→table→column→
  constraint→index→fk→view→func→trigger), drops reversed for `down`.
- **`internal/devdb`**: ephemeral Postgres via **testcontainers-go** (or an
  existing one via `--dev-url`).
- **`cmd/sqld-migrate`**: `up`/`down`/`status`/`apply`/`hash`/`validate`, and
  **`generate <name>`** — realize `schema.sql` and the applied-migration state in
  two fresh dev Postgres instances, introspect both, `Diff`, and write
  `migrations/<ts>_<name>.sql` (up + down). Letting Postgres realize/normalize
  the schema means the diff supports everything PG supports.

**Consequences:** Declarative-first migrations, fully open, verified end-to-end
against real Postgres (integration tests gated on Docker; the diff engine is
Docker-free and golden-tested). Out of scope for now: grants/RLS/partitioning
diff, multi-dialect, online/zero-downtime orchestration. Docs: `docs/migrations.md`.

---

## ADR-0031 — ORM ⊕ sqlc symbiosis via bob (`sqld-gen-bob` + `pkg/gotypes`)

**Status:** accepted (implemented, v1)

**Context:** sqld-gen-go gives sqlc-style typed queries but no ORM
(relationships, eager loading, factories). stephenafamo/bob (open) has a mature
PG ORM + a code-generation framework whose driver returns plain schema metadata.
Spec: `docs/superpowers/specs/2026-05-26-sqld-gen-bob-design.md`. Two PoCs
(compile + runtime, merged on master) proved bob can be forced to emit
sqld-controlled Go types and runs natively on `*pgxpool.Pool`.

**Decision / outcome:**
- **`pkg/gotypes`** (new public package): the canonical pg→Go type mapper —
  `goType`/`resolveGoType`/`UDTName`/registry/`Pascal` — moved out of
  `cmd/sqld-gen-go` so BOTH generators share one mapping. `Mapper` is null-mode
  aware (`Pointer` = historical `*T`, `Opt` = `null.Val[T]`) and has an optional
  UDT package qualifier (`SetUDTPackage`) so enum/composite names can be emitted
  as `db.AppUserStatus` for a different package. sqld-gen-go consumes it via thin
  shims; output is byte-identical (golden-locked).
- **Nested `cmd/sqld-gen-bob` module**: the plugin, its bob-using example
  (`cmd/sqld-gen-bob/example`, incl. the `aarondl/opt`-using opt-mode tests) live
  in a nested module `github.com/yaroher/sqld/cmd/sqld-gen-bob` (with
  `replace => ../../` + a root `go.work`), so the heavy bob dependency stays OUT
  of the core `go.mod` — `go get github.com/yaroher/sqld` pulls no bob/opt.
  `internal/devdb` was promoted to `pkg/devdb` so the nested module can reuse the
  testcontainers helper.
- **`cmd/sqld-gen-bob`**: a binary plugin (no wasm — bob's generator is too
  heavy) that implements bob's `drivers.Interface` over the IR `Catalog`. Each
  `Column.Type` is resolved through `pkg/gotypes` (so bob emits sqld-gen-go's
  types); enum/composite types are qualified into the `typesPackage` option and
  `DBInfo.Enums` is left empty so bob emits **no** competing enum types. bob
  writes its multi-package output directly into the plugin `out` dir (its import
  paths are rooted there) and the plugin returns an empty file list.
- **Overrides are a shared option** — the `overrides` Go-type table must match
  between the two plugins so both emit identical types for overridden columns.
  `nullMode` (`pointer`|`opt`) **no longer has to match**: it now only tells the
  `ToSqld` bridge (below) which nullable form sqld-gen-go emits.
- **`ToSqld()` bob→sqld bridge** (auto-emitted whenever `typesPackage` is set):
  bob models are not Go-convertible to sqld-gen-go's flat models (bob carries
  `R`/`C` ORM fields and wraps every nullable column in `null.Val[T]`), so
  `sqld-gen-bob` writes `gen/bob/models/sqld_bridge.go` with a
  `func (m AppUser) ToSqld() db.AppUsers` field-copy on every model. The bob model
  type per table is discovered by parsing the generated `*.bob.go`
  `psql.NewTablex`/`NewViewx` lines (avoiding re-deriving bob's singular
  inflection). Null unwrapping: a wrapped scalar/enum/composite → `.Ptr()` in
  `pointer` mode (direct copy under `opt`); a nil-capable column (slice/map/
  `json.RawMessage`/`pgtype.*` struct/range/`[]Composite`), which bob still wraps
  in `null.Val[T]` but sqld-gen-go emits as a bare value, → `.GetOrZero()`. The
  direction is bob → sqld only; there is no reverse method. This **removes the
  nullMode-matching requirement for interop**: leave sqld-gen-go in its default
  `pointer` mode and let the bridge convert.
- **Runtime:** both generators' code runs on ONE `*pgxpool.Pool`; bob is wrapped
  via `bobpgx.NewPool`. sqld-gen-go's `RegisterTypes` (custom-type codecs) is
  registered once in `pgxpool.Config.AfterConnect` and inherited by both halves.

**Consequences:** A `users.status` column is `db.AppUserStatus` in the bob model,
the sqld-gen-go model, and the sqld query row — values flow between bob's ORM and
sqld's queries without conversion (compile-level test in `example/`). Scope v1:
models, relationships, where/loaders/joins/counts. Out of scope / limitations:
factories are opt-in (bob needs a random expression per type, unavailable for
externally-owned composites/`pgtype.*`); bob's query-folder codegen is unused
(sqld owns queries). **Interop no longer needs matching nullMode:** the
auto-emitted `ToSqld()` bridge converts bob's `null.Val[T]` model fields into
whatever sqld-gen-go produces, so sqld-gen-go can stay in its default `pointer`
mode (byte-identical historical output) while bob keeps `null.Val[T]`. Matching
`nullMode: opt` on both plugins is still *available* if you want the struct field
types themselves to align (then a nullable composite result column scans via
`null.FromPtr` glue, since pgx cannot carry a non-null composite through
`null.Val`'s `sql.Scanner`), but it is not required. Query params are unaffected
(they stay `*T`/value-typed — sqld-internal, not consumed by bob). Docs:
`docs/bob.md`.

---

<a id="semantics-reference"></a>
## Semantics reference

### Repository layout

```
proto/                  proto sources; proto/easyp.yaml is the codegen config
  sqld/v1/{ir,config,plugin}/*.proto
pkg/proto/              generated Go (irv1 / configv1 / pluginv1) — do not edit
pkg/migrate/            importable migrator API (ADR-0024)
cmd/sqld/               host / orchestrator CLI            (ADR-0023)
cmd/sqld-migrate/       migration CLI, wraps pkg/migrate   (ADR-0023)
cmd/sqld-gen-go/        built-in Go generator plugin       (ADR-0023)
docs/adr.md             this file
docs/superpowers/specs/ design doc
Makefile                `make protocols` regenerates pkg/proto via easyp
```

Module path: `github.com/yaroher/sqld` (`go.mod` present; go_package aligned per
ADR-0025). `cmd/` and `pkg/migrate/` are not scaffolded yet.

### Packages

| package | path | role |
|---------|------|------|
| `sqld.v1.ir` | `proto/sqld/v1/ir/` | the IR: catalog, schema, types, expressions, DML AST, relationships, annotation values |
| `sqld.v1.config` | `proto/sqld/v1/config/` | user-facing generation-run config |
| `sqld.v1.plugin` | `proto/sqld/v1/plugin/` | host↔plugin gRPC contract + annotation grammar |

### `ir` files

- **common.proto** — `QualifiedName`, `ObjectKind`, `ObjectRef` (id-based ref),
  `Engine`, `SourceSpan`, `Metadata` (comment + source + options map + parsed
  `annotations`), and the parsed-annotation value model
  (`AnnotationTargetKind`, `AnnotationArgType`, `AnnotationTargetRef`,
  `AnnotationArg`, `AnnotationValue`).
- **type.proto** — `TypeRef` (`kind` + canonical `pg_name` + `TypeModifier` +
  array element/dims + `udt` ref). `TypeKind`: scalar/array/enum/domain/
  composite/range/pseudo/user-defined.
- **expr.proto** — recursive `Expr` (`oneof`: literal, column_ref, parameter,
  function_call, operator, case, cast, subquery, list, star, `raw_sql`) plus the
  SELECT tree: `SelectStmt` (CTEs + `oneof body` simple/set-op/values + order/
  limit/offset/locking + `raw_sql`), `SimpleSelect`, `SetOperation`, `FromItem`
  (`oneof`: table/subquery/join/function/values), `JoinClause`, `OrderByItem`,
  `SelectTarget`, `CommonTableExpr`, `Values`, `WindowSpec`/`WindowDef`.
- **schema.proto** — `Table` (columns, constraints, indexes, trigger refs,
  persistence), `Column` (type, nullable, default expr, identity, generated,
  collation), `Constraint` (`oneof`: primary_key / foreign_key / unique / check /
  exclusion / not_null), `Index` (method, unique, elements, include, partial
  predicate), `Sequence`. Enums: `ReferentialAction`, `MatchType`,
  `ConstraintType`, `TablePersistence`, `IdentityKind`.
- **routine.proto** — `View`, `MaterializedView`, `Function` (args, returns,
  language, volatility, null-input, security, parallel, body / sql_body),
  `Procedure`, `Trigger` (timing/events/level/when/function), and user types
  `EnumType`, `DomainType`, `CompositeType`, `RangeType`. Imports `schema` for
  `Index`.
- **dml.proto** — `Statement` wrapper, `InsertStmt` (with CTEs, `oneof source`
  values/query/default, `OnConflict`, returning), `UpdateStmt`, `DeleteStmt`,
  `Assignment`, `OnConflict`.
- **relation.proto** — derived `Relationship` (`RelationshipKind`, from/to
  table+columns, `via_constraint`, `JoinTable` for many-to-many, optional,
  suggested name).
- **catalog.proto** — root `Catalog` (name, db/IR version, default schema,
  search path, `Schema[]`, `Relationship[]`) and `Schema` namespace container
  (tables, views, matviews, sequences, functions, procedures, triggers, enums,
  domains, composites, ranges).

### `config` file

- **config.proto** — `Config` (version, `Engine`, `SqlSource[]`,
  `MigrationSource[]`, `PluginConfig[]`, `GlobalOptions`). `SqlSource` (`oneof`
  file/dir/inline + glob + recursive + `SqlKind` schema/query). `MigrationSource`
  (dir + glob). `PluginSource` (`oneof` wasm/binary/command + sha256 + args).
  `PluginConfig` (name, source, out, opaque `bytes options`, enabled, env).
  `GlobalOptions` (default_schema, search_path, type_overrides, strict).

### `plugin` files

- **plugin.proto** — `service Generator { GetInfo; Generate }`.
  `GetInfoResponse` (name, version, supported_engines, options_schema,
  capabilities, `AnnotationSchema annotation_schema`). `GenerateRequest`
  (`Catalog catalog`, `Query[] queries`, `Migration[] migrations`, opaque
  `bytes options`, out_dir, `PluginContext`, `AnnotationValue[] annotations`).
  `GenerateResponse` (`GeneratedFile[]`, `Diagnostic[]` — a single `ERROR`
  diagnostic fails the run). `Query` (name, sql, `QueryCommand`, `Statement ast`,
  `QueryParameter[]`, `QueryColumn[]`, source_file, comment). `Migration`
  (version, name, up/down sql + parsed `Statement[]`).
- **annotation.proto** — grammar: `AnnotationForm` (FLAG/SCALAR/POSITIONAL/
  KEYED/LIST/FREEFORM), `AnnotationCardinality`, `CommentPlacement`, `FieldSpec`,
  `AnnotationValueSpec`, `AnnotationDef`, `AnnotationSchema` (sigil,
  case_insensitive, comment_styles, defs).

### Conventions

- **Stable ids** — `ObjectRef.id` is canonical (`schema.table[.column]`); all
  cross-object refs use it.
- **`raw_sql` escape hatch** — present on `Expr` and `SelectStmt`; lossless
  fallback for unmodeled constructs.
- **`Metadata`** — on every modeled object; carries comment, source span,
  `options` map (generic hints), and parsed `annotations`.
- **`node_id`** (field 21) — host-assigned, deterministic, on every structural
  AST node; bound from `AnnotationTargetRef.node_id`.
- **Annotation flow** — `GetInfo` → plugin's `AnnotationSchema`; host parses
  comments → `AnnotationValue`s delivered both as `GenerateRequest.annotations`
  and on each object's `Metadata.annotations`.
- **Plugin transports** — binary/command via gRPC-over-stdio; wasm via exported
  `get_info`/`generate` on identical serialized messages.

### Verification

All proto files compile clean:

```
protoc -I proto --descriptor_set_out=/dev/null $(find proto -name '*.proto')
```
