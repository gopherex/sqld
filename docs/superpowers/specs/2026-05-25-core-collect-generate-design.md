# sqld core — Collect & Generate

Date: 2026-05-25
Status: approved

## Goal

Implement the engine in `internal/`. Two entry points:

```go
func Collect(cfg *configv1.Config) (*irv1.Catalog, error)   // SQL + migrations -> IR
func Generate(cfg *configv1.Config) error                   // Collect, then run plugins
```

`Collect` parses PostgreSQL sources into the IR `Catalog`. `Generate` runs
`Collect` then drives the configured plugins (annotation parsing, GetInfo,
Generate, writing output files).

## Decisions

- **Parser:** `github.com/wasilibs/go-pg_query` — libpg_query (the real
  PostgreSQL parser) compiled to WASM and run via wazero. No cgo; pure-Go build;
  exact PG grammar for all DDL/DML. Its protobuf parse tree is mapped to our IR.
- **Config format:** YAML, loaded via `internal/utils/protoyaml`
  (`Unmarshal(bytes, *configv1.Config)` = YAML → protojson → proto). Keys accept
  camelCase or proto field names.
- **Collect returns `*irv1.Catalog`** (the root: `[]Schema` + `relationships`).
- **Lossless fallback, never fail on a construct:** unmapped expressions →
  `Expr.raw_sql`; unmapped statements → `RawStatement{kind}`. Parse failures are
  hard errors; per-object inference gaps become `Diagnostic`s, not fatal.
- **Scope: maximum first pass** — DDL + named-query inference + relationship
  graph + both plugin transports (binary/stdio gRPC and wasm/wazero). Depth of
  query column inference is shallow in v1 (see Risks).

## Package layout (`internal/`)

Each package has one purpose, a small surface, and is testable in isolation.

| package | purpose | depends on |
|---------|---------|-----------|
| `core` | facade: `Collect`, `Generate` | config, source, parse, mapper, catalog, query, relate, plugin |
| `config` | load `*configv1.Config` from a YAML file via `utils/protoyaml` | protoyaml, configv1 |
| `source` | resolve `SqlSource`/`MigrationSource` → ordered `[]Unit{path, sql, kind}`; migrations ordered by version | configv1 |
| `parse` | wrap `go-pg_query`: `sql -> *pg_query.ParseResult`; split into statements | go-pg_query |
| `nodeid` | deterministic AST node ids (structural path, e.g. `stmt0/select/where/and:1`) | — |
| `mapper` | pg parse tree → ir.*: `types.go`, `expr.go`, `stmt.go`, `ddl.go`; assigns node_id; raw_sql fallback | irv1, parse, nodeid |
| `catalog` | apply migrations (`up`, in order) + schema DDL → `Schema`/`Table`/...; assign `ObjectRef.id` = `schema.table[.col]` | irv1, mapper |
| `query` | extract `-- name: X :cmd`; map body to `Statement`; infer `QueryParameter`/`QueryColumn` against the Catalog | irv1, mapper, catalog |
| `relate` | derive `Relationship` graph from FK constraints (cardinality, join-table detection) | irv1 |
| `plugin` | host side: `annotate` (parse comments per `AnnotationSchema` → `AnnotationValue`, attach to `Metadata` + flat list); `Runner` iface; `binary.go` (exec + gRPC over stdio); `wasm.go` (wazero, exported `get_info`/`generate`) | irv1, pluginv1, configv1, grpc, wazero |

## Data flow

```
Generate(cfg):
  cat := Collect(cfg)
  for each enabled plugin p:
     info := p.GetInfo()
     anns := annotate(sources, info.annotation_schema)   // -> []AnnotationValue
     attach anns onto cat (Metadata) + build flat list
     resp := p.Generate(GenerateRequest{cat, queries, migrations, options, annotations, ...})
     write resp.files under p.out ; fail on ERROR diagnostics

Collect(cfg):
  units      := source.Resolve(cfg)              // schema sql + migrations + query sql
  schemaStmts := parse(schema units + migration up, in order)
  cat        := catalog.Build(schemaStmts)        // mapper: DDL -> objects ; resolve ids
  queries    := query.Build(query units, cat)     // mapper + inference
  cat.Relationships = relate.Derive(cat)
  return cat
```

## Error handling

- `parse` failure → hard error (wrap with file + position).
- Unmapped construct → `raw_sql` / `RawStatement`; not an error.
- Inference gap (e.g. column type through a join) → leave `type` unset, add a
  `Diagnostic` to a collector; `Collect` succeeds. `Generate` surfaces plugin
  `ERROR` diagnostics as a failed run.

## Testing (TDD)

- **mapper** unit tests: small SQL snippet → expected `ir.*` subtree
  (`types_test.go`, `expr_test.go`, `stmt_test.go`, `ddl_test.go`).
- **catalog** golden test: sample schema (several tables, FKs, view, enum) →
  expected `Catalog` (golden `.txtpb`/`.json`).
- **query** tests: `-- name:` queries → inferred params/columns; commands.
- **relate** tests: FK fixtures → expected relationship kinds + join-table.
- **plugin** tests: a fake in-process plugin (gRPC) and a tiny fake binary; wasm
  path tested with a minimal `.wasm` echo plugin if feasible, else deferred.

## New dependencies

`github.com/wasilibs/go-pg_query`, `github.com/tetratelabs/wazero`,
`sigs.k8s.io/yaml` (already pulled by `utils/protoyaml`). grpc + protobuf
already present.

## Risks

- **Query column/param inference depth.** v1 resolves direct column refs and
  literals against the Catalog; complex expressions, joins, and subqueries yield
  `type` unspecified + `nullable` unknown + a diagnostic. Deepened later.
- **wasm plugin host.** wazero instantiation + the host ABI (pass serialized
  request bytes in/out of linear memory) is the riskiest new surface; if it
  slips, the binary/stdio transport still delivers a working plugin path.
- **DDL coverage.** First pass targets `CREATE TABLE/VIEW/INDEX/SEQUENCE/TYPE/
  FUNCTION/TRIGGER`; other DDL lands as `RawStatement{kind=DDL}` and is skipped
  by the catalog builder.

## Out of scope (this spec)

`cmd/` binaries (`sqld`, `sqld-migrate`, `sqld-gen-go`), `pkg/migrate`, the
SchemaChange diff IR. Those follow once the core is green.
