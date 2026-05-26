# sqld-gen-bob — ORM⊕sqlc symbiosis via bob — Design

Date: 2026-05-26
Status: draft (awaiting user review)

## Goal & positioning

Give sqld users a full Go ORM — models, relationships/eager-loading, factories,
typed where/loaders/joins — by feeding sqld's IR `Catalog` into
[stephenafamo/bob](https://github.com/stephenafamo/bob)'s code generator, while
**bob models and sqld query structs share ONE canonical Go type per Postgres
type** and run on **ONE `*pgxpool.Pool`**.

This is the "ORM ⊕ sqlc" tandem: `sqld-gen-go` owns hand-written + dynamic
queries (`@if`/`@orderby`) and the leaf Go types; `sqld-gen-bob` (new) owns the
ORM surface. Both are 100% open (sqld + bob are open; contrast Atlas's paywall).
PostgreSQL only — bob is used purely at the PG dialect level.

Both PoCs are merged on `master` (commit range `a933ac3..15fa7c8`) and prove the
two linchpins:

- **Type authority (compile-level):** a bob `drivers.Interface` backed by
  `*irv1.Catalog` forces bob's `Column.Type` to a Go type sqld controls. Generated
  model field: `Status pocshared.AccountStatus` — sqld's type, not a bob-emitted
  enum. `DBInfo.Enums` left empty ⇒ bob emits no competing enum package.
- **Runtime (testcontainers):** bob runs natively on `*pgxpool.Pool` via
  `github.com/stephenafamo/bob/drivers/pgx` (`bobpgx.NewPool(pool)`). A value
  written by bob's ORM is read back identically by raw pgx on the SAME pool.
  String enums need no codec registration; composites need ONE
  `pgxpool.Config.AfterConnect` registration (= what `sqld-gen-go.RegisterTypes`
  already emits), inherited by both halves.

## Decisions (locked with user)

1. **Delivery: `sqld-gen-bob`** — a generator binary in the `sqld-gen-go` family,
   wired as a **binary plugin** (`service Generator` stdio transport, like
   `sqld-gen-go`'s binary mode). **No wasm** — bob's generator (templates,
   reflection, big dep tree) is not a realistic wasip1 target. Depends ONLY on
   public exports (`pkg/proto/.../ir`, `pkg/sqld`, the new `pkg/gotypes`) + bob;
   never `internal/`.
2. **Null-mode is user-selectable**, shared by both generators so structs
   interop: `opt` (bob-native `aarondl/opt`: `null.Val[T]`/`omit.Val[T]`) or
   `pointer` (`*T`; bob `TypeSystem = github.com/aarondl/opt/null`). Both
   generators MUST use the same mode in a given project.
3. **Scope = all of bob's schema-derived outputs**: models, relationships,
   factories, where, loaders, joins, counts, dberrors. **Excluded:** bob's
   query-folder codegen — sqld owns queries (bob can't parse sqld's dynamic DSL).
4. **`sqld-gen-go` owns leaf types** (enum/composite/domain Go types +
   `RegisterTypes` for pgx); `sqld-gen-bob` imports them.

## Type contract — the shared mapper (`pkg/gotypes`)

The two generators must produce **identical Go type names and imports** for every
column. Rather than duplicate logic or pass a manifest file (ordering-coupled),
extract the single canonical pg→Go mapper into a new **public** package
`pkg/gotypes` that BOTH plugins import:

- Move `sqld-gen-go`'s type logic (`goType`, `resolveGoType`, `udtGoTypeName`,
  `pascal`, the udt registry, `pgtypeElement`, overrides) from
  `cmd/sqld-gen-go/types.go` into `pkg/gotypes`.
- Parameterize by **null-mode** (`opt` | `pointer`) instead of the current
  hard-coded `*T`.
- API sketch:
  ```go
  package gotypes
  type NullMode int // Pointer, Opt
  type Mapper struct { reg *Registry; overrides Overrides; null NullMode }
  func NewMapper(cat *irv1.Catalog, ov Overrides, null NullMode) *Mapper
  // Go type expression + imports for a column/type.
  func (m *Mapper) GoType(columnID string, t *irv1.TypeRef, nullable bool) (expr string, imports []string)
  // Stable exported Go name for a UDT (enum/composite/domain), e.g. "AppUserStatus".
  func (m *Mapper) UDTName(schema, pgName string) string
  func Pascal(s string) string
  ```
- `cmd/sqld-gen-go` is refactored to call `pkg/gotypes` (behavior preserved —
  guarded by the existing `gogen_test.go` + example golden). `sqld-gen-bob`'s
  driver calls the SAME `Mapper` to fill bob's `Column.Type` and to register
  imports in `drivers.Types`. Guaranteed identical types, no duplication.

Risk: the refactor must not change `sqld-gen-go` output. Mitigation: extract
verbatim first (pointer mode == today), green the full suite + example golden,
THEN add opt mode.

## Architecture

```
schema.sql + queries → sqld host → IR Catalog (proto)
                            │  GenerateRequest (stdio proto), plugins run in config order
        ┌───────────────────┴───────────────────┐
 sqld-gen-go (plugin)                   sqld-gen-bob (plugin, NEW)
 • query rows/params, dynamic queries   • Catalog → bob DBInfo
 • emits shared leaf types +            • gen.Run → models, relationships,
   RegisterTypes(pgx)                      factories, where, loaders, joins, …
        │            both import pkg/gotypes (one canonical pg→Go map)            │
        └───────────────→ one Go pkg `db` (or split pkgs) ←──────────────────────┘
              leaf types + sqld query rows + bob models, ONE *pgxpool.Pool
```

## Components

```
pkg/gotypes/            NEW public: canonical pg→Go mapper (null-mode aware), used by BOTH plugins
cmd/sqld-gen-go/        refactor types.go → call pkg/gotypes; add nullMode option (opt|pointer)
cmd/sqld-gen-bob/       NEW plugin
  main.go               plugin transport (stdio proto: GetInfo/Generate) — mirror sqld-gen-go/main.go
  info.go               AnnotationSchema (likely none/minimal), GetInfo
  driver.go             sqldDriver: drivers.Interface[any,C,I] — Catalog → DBInfo using pkg/gotypes
  generate.go           wire plugins.Setup(gen.PSQLTemplates) + gen.State + gen.Run; write files into req.Out
docs/                   bob.md (usage), update README + adr.md (new ADRs)
example/                add bob output target + a test exercising bob model ⊕ sqld query on one pool
```

`sqld-gen-bob` reuses the PoC's `cmd/bobgen-sqld` code as the starting point
(promote `driver.go`, the converter, the runtime test) and renames/relocates it.

### driver.go — Catalog → bob DBInfo

- `Dialect() = "psql"`.
- `Assemble`: for each `Schema.Tables`, build `drivers.Table{Key, Schema, Name,
  Columns, Indexes, Constraints, Comment}`.
  - **Columns**: `Column.Type = mapper.GoType(col.Id, col.Type, col.Nullable)`'s
    expression; register that type in `drivers.Types` with its imports (use bob's
    `output(...)` import-rewrite hook where the type lives in a generated pkg).
    `DBType` = pg name (for factory/randomization hints); `Nullable`, `Generated`
    from the column.
  - **Enums**: leave `DBInfo.Enums` EMPTY (sqld-gen-go owns enum Go types). The
    enums output auto-disables; models still generate (PoC-confirmed).
  - **Constraints**: map IR PK/unique/FK → `drivers.Constraints` so bob derives
    relationships + eager-loaders. Synthesize a PK name when IR leaves it empty.
  - **Indexes**: map IR indexes → `drivers.Index`.
- IR quirks to honor (from PoC): enum columns currently arrive as
  `kind=SCALAR` with `PgName=<enum name>` and empty `Udt` — detect enums by
  matching `PgName` against `Schema.Enums`; also honor `kind=ENUM`+`Udt` if sqld
  later resolves them. `bigserial`/serial preserved as-is (map to int types).
  NOT NULL is a separate `CONSTRAINT_TYPE_NOT_NULL` entry as well as the column
  flag.

### generate.go — run bob

Mirror `/tmp` PoC + `bobgen-psql/main.go` pattern:
```go
pcfg := plugins.Config{ /* enable models, factory, where, loaders, joins, counts, dberrors; Models/etc dirs under req.Out */ }
outPlugins := plugins.Setup[any, C, I](pcfg, gen.PSQLTemplates)
state := &gen.State[C]{ Config: gen.Config[C]{ TypeSystem: <"" for opt | "github.com/aarondl/opt/null" for pointer>, /* Aliases, Tags, etc */ } }
err := gen.Run[any, C, I](ctx, state, driver, outPlugins...)
```
Output dirs come from the plugin's `out` + options.

## Config

`sqld-gen-bob` plugin entry (existing `PluginConfig` shape):
```yaml
plugins:
  - name: go            # sqld-gen-go runs first (emits leaf types)
    binary: bin/sqld-gen-go
    out: gen/db
    options: { package: db, nullMode: opt }
  - name: bob           # sqld-gen-bob runs second
    binary: bin/sqld-gen-bob
    out: gen/db          # may share gen-go's package dir or a sibling
    options:
      typesPackage: github.com/acme/app/gen/db   # import path of gen-go's leaf-types pkg
      nullMode: opt                               # MUST equal gen-go's
      models: true
      factories: true
      relationships: true
      whereLoadersJoins: true
```
`sqld-gen-bob` needs `typesPackage` (import path) to reference shared leaf types,
and `nullMode` (must match `sqld-gen-go`). The host validates the two `nullMode`s
agree when both plugins are present (or a shared `options.go.nullMode` — decide at
spec review; default proposal: per-plugin option + host cross-check).

## Runtime usage (documented, generated)

One pool, shared registrations:
```go
cfg, _ := pgxpool.ParseConfig(dsn)
cfg.AfterConnect = func(ctx context.Context, c *pgx.Conn) error { return db.RegisterTypes(ctx, c) } // sqld-gen-go emits RegisterTypes
pool, _ := pgxpool.NewWithConfig(ctx, cfg)
exec := bobpgx.NewPool(pool)            // bob ORM over the SAME pool
// bob:  models.Users.Insert(setter).One(ctx, exec)
// sqld: db.New(pool).GetUser(ctx, id)  // raw pgx, same pool, same db.AppUserStatus type
```

## Testing

- **`pkg/gotypes`**: table-driven unit tests for both null-modes; the
  `sqld-gen-go` refactor is covered by its existing `gogen_test.go`/`types_test.go`
  + example golden (output must be byte-identical in pointer mode).
- **driver.go**: unit tests Catalog→DBInfo (no DB) — columns/types/constraints.
- **generate.go (golden)**: run on the example schema → generated bob package
  compiles and references `db.*` shared types (grep assertions, like the PoC's
  `unify_test.go`).
- **Integration (testcontainers, Docker-gated)**: promote the PoC's
  `runtime_test.go` — bob write + sqld read on one pool, enum + composite
  round-trip.
- **example/**: extend the golden example to generate bob alongside gen-go and a
  test mixing a bob model and a sqld query.

## Risks

- **gen-go refactor regression** — extracting types.go into `pkg/gotypes` must
  preserve output. Mitigation: verbatim extraction first, golden-locked, opt mode
  added after.
- **Null-mode drift** — if the two plugins' modes disagree, structs won't interop.
  Mitigation: host cross-check + docs; `init` scaffolds matching values.
- **bob generics/API churn** — pin `bob v0.44.0`; wrap `gen.Run` behind
  `generate.go`.
- **bob version of relationships/inflection naming** — bob owns model/relation
  names; sqld owns query/leaf-type names; shared `db` pkg must not collide. Keep
  bob models and sqld query rows in distinct files/identifiers.
- **Generation order** — gen-go must run before gen-bob isn't strictly required
  (gen-bob only imports the package path, doesn't read gen-go's files), but the
  shared package must exist at compile time. Document the two-plugin setup.

## Out of scope (now)

- bob query-folder codegen (sqld owns queries).
- wasm transport for sqld-gen-bob.
- Multi-dialect (PG only).
- A third "merge into one package" mode beyond opt/pointer null-modes (sql.Null[T]
  is a possible future 3rd mode).
- IR change to resolve enum columns to `kind=ENUM`+`Udt` (driver tolerates the
  current SCALAR shape; the IR cleanup is a separate task).
