# sqld core (Collect & Generate) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `internal/` engine that parses PostgreSQL sources into the IR `Catalog` (`Collect`) and runs configured plugins against it (`Generate`).

**Architecture:** Pipeline `config → source → parse → mapper → catalog → query → relate → Catalog`. Parsing via `wasilibs/go-pg_query` (libpg_query as WASM, no cgo). Unmapped constructs fall back to `raw_sql`/`RawStatement` — never fail on a construct. `Generate` adds annotation parsing + plugin transports (gRPC-over-stdio + wazero wasm).

**Tech Stack:** Go 1.25, `github.com/wasilibs/go-pg_query`, `github.com/tetratelabs/wazero`, `google.golang.org/grpc`, `google.golang.org/protobuf`, `sigs.k8s.io/yaml` (via `internal/utils/protoyaml`).

Generated proto Go: `github.com/yaroher/sqld/pkg/proto/sqld/v1/{ir,config,plugin}` (aliases `irv1`, `configv1`, `pluginv1`).

---

## File structure

```
internal/
  config/load.go            Load(path) (*configv1.Config, error)
  source/source.go          Resolve(cfg) ([]Unit, error) ; type Unit
  parse/parse.go            Parse(sql) (*pg.ParseResult,error) ; Statements(sql) ([]Stmt,error)
  nodeid/nodeid.go          Builder for deterministic node ids
  mapper/types.go           MapType(*pg.TypeName) *irv1.TypeRef
  mapper/expr.go            MapExpr(*pg.Node, *Cursor) *irv1.Expr
  mapper/stmt.go            MapStatement(*pg.Node, *Cursor) *irv1.Statement
  mapper/ddl.go             MapCreateTable / MapIndex / MapView / ... -> ir objects
  catalog/build.go          Build(stmts []Stmt) (*irv1.Catalog, *Diagnostics)
  query/query.go            Parse named queries ; type NamedQuery
  query/infer.go            Infer params/columns against Catalog
  relate/relate.go          Derive(cat) []*irv1.Relationship
  plugin/annotate.go        Annotate(units, schema) []*irv1.AnnotationValue
  plugin/runner.go          Runner interface ; Open(PluginConfig) (Runner,error)
  plugin/binary.go          gRPC-over-stdio runner
  plugin/wasm.go            wazero runner
  core/core.go              Collect(cfg) (*irv1.Catalog,error) ; Generate(cfg) error
```

Test files sit beside each (`*_test.go`). Shared test fixtures in `internal/testdata/`.

---

## Task 1: Module deps + parser smoke test

**Files:**
- Modify: `go.mod`
- Create: `internal/parse/parse.go`
- Test: `internal/parse/parse_test.go`

- [ ] **Step 1: Write the failing test**

```go
package parse

import "testing"

func TestParseSelectSmoke(t *testing.T) {
	res, err := Parse("SELECT 1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(res.GetStmts()) != 1 {
		t.Fatalf("want 1 stmt, got %d", len(res.GetStmts()))
	}
	if res.GetStmts()[0].GetStmt().GetSelectStmt() == nil {
		t.Fatalf("want SelectStmt node")
	}
}
```

- [ ] **Step 2: Run, expect fail (package/func missing)**

Run: `go test ./internal/parse/ -run TestParseSelectSmoke -v`
Expected: FAIL (undefined `Parse`).

- [ ] **Step 3: Implement wrapper + add deps**

```go
// internal/parse/parse.go
package parse

import pg "github.com/wasilibs/go-pg_query"

// Parse parses one or more SQL statements into the libpg_query parse tree.
func Parse(sql string) (*pg.ParseResult, error) {
	return pg.Parse(sql)
}
```

Then: `go get github.com/wasilibs/go-pg_query && go mod tidy`

- [ ] **Step 4: Run, expect pass**

Run: `go test ./internal/parse/ -run TestParseSelectSmoke -v`
Expected: PASS. (Confirms the AST type is `*pg.ParseResult` with `GetStmts()[i].GetStmt().GetSelectStmt()`. Record the actual import alias/types observed for use in later tasks.)

- [ ] **Step 5: Commit**

```bash
git add go.mod go.sum internal/parse/
git commit -m "feat(core): add go-pg_query parser wrapper"
```

---

## Task 2: Statement splitting

**Files:**
- Modify: `internal/parse/parse.go`
- Test: `internal/parse/parse_test.go`

- [ ] **Step 1: Failing test**

```go
func TestStatements(t *testing.T) {
	stmts, err := Statements("CREATE TABLE a(id int); CREATE TABLE b(id int);")
	if err != nil {
		t.Fatal(err)
	}
	if len(stmts) != 2 {
		t.Fatalf("want 2, got %d", len(stmts))
	}
	if stmts[0].Node.GetCreateStmt() == nil {
		t.Fatal("want CreateStmt")
	}
}
```

- [ ] **Step 2: Run, expect fail.** `go test ./internal/parse/ -run TestStatements -v`

- [ ] **Step 3: Implement**

```go
// Stmt is one parsed statement with its 1-based ordinal.
type Stmt struct {
	Index int
	Node  *pg.Node // the inner statement node (RawStmt.Stmt)
}

func Statements(sql string) ([]Stmt, error) {
	res, err := pg.Parse(sql)
	if err != nil {
		return nil, err
	}
	out := make([]Stmt, 0, len(res.GetStmts()))
	for i, raw := range res.GetStmts() {
		out = append(out, Stmt{Index: i, Node: raw.GetStmt()})
	}
	return out, nil
}
```

- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(core): split SQL into statements`

---

## Task 3: Config loader

**Files:**
- Create: `internal/config/load.go`
- Test: `internal/config/load_test.go`, `internal/config/testdata/basic.yaml`

- [ ] **Step 1: Fixture + failing test**

`internal/config/testdata/basic.yaml`:
```yaml
version: "1"
engine: ENGINE_POSTGRESQL
sql:
  - dir: ./schema
    kind: SQL_KIND_SCHEMA
plugins:
  - name: go
    source: { binary: ./bin/sqld-gen-go }
    out: ./gen
```

```go
package config

import "testing"

func TestLoad(t *testing.T) {
	cfg, err := Load("testdata/basic.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GetEngine().String() != "ENGINE_POSTGRESQL" {
		t.Fatalf("engine: %v", cfg.GetEngine())
	}
	if len(cfg.GetPlugins()) != 1 || cfg.GetPlugins()[0].GetName() != "go" {
		t.Fatalf("plugins: %+v", cfg.GetPlugins())
	}
}
```

- [ ] **Step 2: Run, expect fail.** `go test ./internal/config/ -v`

- [ ] **Step 3: Implement**

```go
// internal/config/load.go
package config

import (
	"fmt"
	"os"

	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
	"github.com/yaroher/sqld/internal/utils/protoyaml"
)

// Load reads a YAML config file into a Config message.
func Load(path string) (*configv1.Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := &configv1.Config{}
	if err := protoyaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}
```

- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(core): YAML config loader`

---

## Task 4: Source resolution

**Files:**
- Create: `internal/source/source.go`
- Test: `internal/source/source_test.go`

`Unit` carries one SQL chunk with provenance.

- [ ] **Step 1: Failing test** (temp dir with two schema files + inline)

```go
func TestResolveSchemaDir(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "a.sql"), []byte("CREATE TABLE a(id int);"), 0o644)
	os.WriteFile(filepath.Join(dir, "b.sql"), []byte("CREATE TABLE b(id int);"), 0o644)

	cfg := &configv1.Config{Sql: []*configv1.SqlSource{{
		Source: &configv1.SqlSource_Dir{Dir: dir},
		Kind:   configv1.SqlKind_SQL_KIND_SCHEMA,
	}}}
	units, err := Resolve(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(units) != 2 {
		t.Fatalf("want 2 units, got %d", len(units))
	}
	// deterministic order by path
	if filepath.Base(units[0].Path) != "a.sql" {
		t.Fatalf("order: %s", units[0].Path)
	}
}
```

- [ ] **Step 2: Run, expect fail.**

- [ ] **Step 3: Implement.** Resolve `SqlSource` (file/dir+glob default `*.sql`/recursive/inline) and `MigrationSource` (dir+glob, ordered by filename). Sort entries by path for determinism.

```go
// internal/source/source.go
package source

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
)

type Kind int

const (
	KindSchema Kind = iota
	KindQuery
	KindMigrationUp
	KindMigrationDown
)

// Unit is one SQL chunk with provenance.
type Unit struct {
	Path    string
	SQL     string
	Kind    Kind
	Version string // migrations only
}

func Resolve(cfg *configv1.Config) ([]Unit, error) {
	var units []Unit
	for _, s := range cfg.GetSql() {
		us, err := resolveSQL(s)
		if err != nil {
			return nil, err
		}
		units = append(units, us...)
	}
	for _, m := range cfg.GetMigrations() {
		us, err := resolveMigration(m)
		if err != nil {
			return nil, err
		}
		units = append(units, us...)
	}
	return units, nil
}

func kindOf(k configv1.SqlKind) Kind {
	if k == configv1.SqlKind_SQL_KIND_QUERY {
		return KindQuery
	}
	return KindSchema
}

func resolveSQL(s *configv1.SqlSource) ([]Unit, error) {
	k := kindOf(s.GetKind())
	switch src := s.GetSource().(type) {
	case *configv1.SqlSource_Inline:
		return []Unit{{Path: "<inline>", SQL: src.Inline, Kind: k}}, nil
	case *configv1.SqlSource_File:
		b, err := os.ReadFile(src.File)
		if err != nil {
			return nil, err
		}
		return []Unit{{Path: src.File, SQL: string(b), Kind: k}}, nil
	case *configv1.SqlSource_Dir:
		glob := s.GetGlob()
		if glob == "" {
			glob = "*.sql"
		}
		return readDir(src.Dir, glob, s.GetRecursive(), k)
	default:
		return nil, fmt.Errorf("sql source: empty")
	}
}

func readDir(dir, glob string, recursive bool, k Kind) ([]Unit, error) {
	var paths []string
	walk := func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if !recursive && p != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if ok, _ := filepath.Match(glob, d.Name()); ok {
			paths = append(paths, p)
		}
		return nil
	}
	if err := filepath.WalkDir(dir, walk); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	units := make([]Unit, 0, len(paths))
	for _, p := range paths {
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		units = append(units, Unit{Path: p, SQL: string(b), Kind: k})
	}
	return units, nil
}

// resolveMigration: read dir+glob (default *.sql), sort by name, Kind=KindMigrationUp,
// Version = filename without extension. (Down handling deferred; native format = up only.)
func resolveMigration(m *configv1.MigrationSource) ([]Unit, error) {
	glob := m.GetGlob()
	if glob == "" {
		glob = "*.sql"
	}
	us, err := readDir(m.GetDir(), glob, false, KindMigrationUp)
	if err != nil {
		return nil, err
	}
	for i := range us {
		us[i].Version = trimExt(filepath.Base(us[i].Path))
	}
	return us, nil
}

func trimExt(name string) string {
	return name[:len(name)-len(filepath.Ext(name))]
}
```

- [ ] **Step 4: Run, expect pass.** Add a second test for `MigrationSource` ordering.
- [ ] **Step 5: Commit** `feat(core): resolve SQL and migration sources`

---

## Task 5: Node id builder

**Files:**
- Create: `internal/nodeid/nodeid.go`
- Test: `internal/nodeid/nodeid_test.go`

- [ ] **Step 1: Failing test**

```go
func TestNodeIDPath(t *testing.T) {
	b := New("stmt0")
	if got := b.Child("select").Child("where").String(); got != "stmt0/select/where" {
		t.Fatalf("got %q", got)
	}
	if got := b.Child("from").Index(1).String(); got != "stmt0/from:1" {
		t.Fatalf("got %q", got)
	}
}
```

- [ ] **Step 2: Run, expect fail.**

- [ ] **Step 3: Implement** an immutable path builder: `New(root)`, `Child(name)` returns a new builder appending `/name`, `Index(i)` appends `:i`, `String()` renders. Deterministic, no shared mutation.

- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(core): deterministic node id builder`

---

## Task 6: Type mapper

**Files:**
- Create: `internal/mapper/types.go`
- Test: `internal/mapper/types_test.go`

`MapType(*pg.TypeName) *irv1.TypeRef`. Map libpg_query `TypeName` (joined `names[]`, `typmods`, `arrayBounds`) → `TypeRef` with canonical `pg_name`, `TypeModifier`, array element/dims.

- [ ] **Step 1: Failing tests** — table-driven:

```go
func TestMapType(t *testing.T) {
	cases := []struct{ sql, pgName string; kind irv1.TypeKind }{
		{"int4", "int4", irv1.TypeKind_TYPE_KIND_SCALAR},
		{"varchar(20)", "varchar", irv1.TypeKind_TYPE_KIND_SCALAR},
		{"numeric(10,2)", "numeric", irv1.TypeKind_TYPE_KIND_SCALAR},
		{"int4[]", "int4", irv1.TypeKind_TYPE_KIND_ARRAY},
		{"timestamptz", "timestamptz", irv1.TypeKind_TYPE_KIND_SCALAR},
	}
	for _, c := range cases {
		tn := typeNameFromColumn(t, c.sql) // helper: parse "CREATE TABLE t(c <sql>)" -> ColumnDef.TypeName
		got := MapType(tn)
		// for array, pg_name is the element name and kind=ARRAY with element set
		...assert pgName/kind...
	}
}
```

- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement.** Extract type name from `TypeName.Names` (last `String` node), `typmods` → numeric precision/scale or varchar length, `ArrayBounds` non-empty → `TYPE_KIND_ARRAY` with recursive element. Unknown shapes still produce a `TypeRef` with `pg_name` set (never fail).
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(mapper): postgres TypeName -> ir.TypeRef`

---

## Task 7: Expression mapper

**Files:**
- Create: `internal/mapper/expr.go`
- Test: `internal/mapper/expr_test.go`

`MapExpr(node *pg.Node, c *nodeid.Builder) *irv1.Expr`. Handle: `ColumnRef`→`ColumnRef`, `A_Const`→`Literal`, `ParamRef`/named→`ParameterRef`, `FuncCall`→`FunctionCall`, `A_Expr`/`BoolExpr`/`NullTest`→`OperatorExpr`, `CaseExpr`→`CaseExpr`, `TypeCast`→`CastExpr`, `SubLink`→`SubqueryExpr`, `List`/`RowExpr`→`ListExpr`, `ColumnRef` with `A_Star`→`StarExpr`. Anything else → `Expr{raw_sql: deparse(node)}`. Every returned `Expr` gets `node_id`.

- [ ] **Step 1: Failing tests** for each kind (parse `SELECT <expr>` → take target → MapExpr). Examples: `a = $1` → OperatorExpr{symbol:"=", operands:[colref, param]}; `count(*)` → FunctionCall; `x IS NULL` → OperatorExpr{symbol:"IS NULL"}.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** with a `switch node.GetNode().(type)`-style dispatch on the pg oneof getters; `raw_sql` fallback via `pg.Deparse` of a wrapping stmt or the node's stored text. Assign `node_id` from the cursor.
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(mapper): postgres expr -> ir.Expr with raw_sql fallback`

---

## Task 8: Statement mapper

**Files:**
- Create: `internal/mapper/stmt.go`
- Test: `internal/mapper/stmt_test.go`

`MapStatement(node *pg.Node, c *nodeid.Builder) *irv1.Statement`. Dispatch: `SelectStmt`→build `SelectStmt` (targets/from/where/group/having/order/limit; set-ops via `op`), `InsertStmt`/`UpdateStmt`/`DeleteStmt`→respective, `MergeStmt`→`MergeStmt`. Unknown (DDL/TRUNCATE/COPY/transaction) → `Statement{raw: RawStatement{kind, sql}}` with `classify(node)` → `StatementKind`. SELECT/expr subtrees that don't map → `raw_sql`.

- [ ] **Step 1: Failing tests:** `SELECT id FROM t WHERE id=$1` → Statement.GetSelect() with one target + where OperatorExpr; `INSERT INTO t(a) VALUES ($1)` → InsertStmt; `TRUNCATE t` → RawStatement{kind=TRUNCATE}.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement.** Reuse `MapExpr`. From-items: `RangeVar`→TableRef, `JoinExpr`→JoinClause, `RangeSubselect`→SubqueryRef. `classify` inspects node type for the RawStatement kind.
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(mapper): postgres DML -> ir.Statement`

---

## Task 9: DDL mapper

**Files:**
- Create: `internal/mapper/ddl.go`
- Test: `internal/mapper/ddl_test.go`

Functions producing IR objects (no ids yet — catalog assigns): `MapCreateTable(*pg.CreateStmt) *irv1.Table`, `MapColumn(*pg.ColumnDef) *irv1.Column` (type via `MapType`, NOT NULL/DEFAULT/GENERATED/identity from constraints), inline + table `Constraint`s (PK/FK/unique/check), `MapIndex(*pg.IndexStmt) *irv1.Index`, `MapView(*pg.ViewStmt) *irv1.View`, `MapCreateSeq`, `MapCreateEnum`/`CompositeTypeStmt`, `MapCreateFunction`, `MapCreateTrigger`. Each unknown → skipped (returns nil) and recorded as a diagnostic by the caller.

- [ ] **Step 1: Failing tests:** `CREATE TABLE users(id bigserial primary key, email text not null unique, org_id bigint references orgs(id) on delete cascade)` → Table with 3 columns, PK on id, unique on email, FK org_id→orgs(id) CASCADE. Plus a `CREATE TYPE status AS ENUM('a','b')` test.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** column constraint extraction (`ColumnDef.Constraints`), table constraints (`CreateStmt.TableElts` of type `Constraint`), referential actions enum mapping. Persistence from `relpersistence`.
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(mapper): CREATE TABLE/INDEX/VIEW/TYPE -> ir objects`

---

## Task 10: Catalog builder

**Files:**
- Create: `internal/catalog/build.go`
- Test: `internal/catalog/build_test.go`, `internal/catalog/testdata/shop.sql`

`Build(stmts []parse.Stmt) (*irv1.Catalog, *Diagnostics)`. Walk statements in order (schema + migration up). For each known DDL → place into the right `Schema` (default `public`), via the mapper. Assign `ObjectRef.id` = `schema.table[.col]`; resolve FK `referenced_table` ids. DROP/ALTER in first pass: ALTER TABLE handled minimally (ADD COLUMN/CONSTRAINT) or recorded as diagnostic + RawStatement; DROP removes the object. Unknown → diagnostic.

- [ ] **Step 1: Failing golden test:** parse `testdata/shop.sql` (schemas: users, orders FK users, an enum, a view) → assert `len(schemas)`, table names, FK resolved id, enum present.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** the assembler + id assignment + a `Diagnostics` accumulator (`Add(sev, msg, span)`).
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(catalog): assemble Catalog from DDL statements`

---

## Task 11: Named queries + inference

**Files:**
- Create: `internal/query/query.go`, `internal/query/infer.go`
- Test: `internal/query/query_test.go`, `internal/query/infer_test.go`

`query.go`: scan a query unit for `-- name: <Name> :<cmd>` headers, split bodies, map command suffix → `QueryCommand`, map body via `mapper.MapStatement`, capture leading comment. `infer.go`: `Infer(q *irv1.Query, cat *irv1.Catalog, d *Diagnostics)` — params from `$n`/`@name` (number/name, type from comparison against a resolved column when available), columns from select targets (resolve `ColumnRef` against catalog tables in FROM; literals → literal type; unresolved → leave type unset + diagnostic).

- [ ] **Step 1: Failing tests:** header parse (`-- name: GetUser :one\nSELECT id, email FROM users WHERE id = $1;`) → Query{name:"GetUser", command:ONE}; infer → 1 param (type int8 from users.id), 2 columns (id int8, email text).
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** header regex `^--\s*name:\s*(\w+)\s*:(\w+)\s*$`, body accumulation until next header/`;`, command map, then inference walking FROM → alias map → target resolution.
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(query): named query parsing + param/column inference`

---

## Task 12: Relationship derivation

**Files:**
- Create: `internal/relate/relate.go`
- Test: `internal/relate/relate_test.go`

`Derive(cat *irv1.Catalog) []*irv1.Relationship`. For each FK: emit `MANY_TO_ONE` (referencing→referenced) and `ONE_TO_MANY` (reverse); if FK columns are unique/PK → `ONE_TO_ONE`. Detect join tables (a table whose non-audit columns are exactly two FKs, both part of PK) → emit `MANY_TO_MANY` with `JoinTable`. Set `optional` from column nullability. `suggested_name` from referenced table name.

- [ ] **Step 1: Failing tests:** users←orders FK → MANY_TO_ONE + ONE_TO_MANY; `user_roles(user_id,role_id)` join table → MANY_TO_MANY users↔roles with JoinTable.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** FK walk + uniqueness check (match FK cols against a unique/PK constraint) + join-table heuristic.
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(relate): derive relationship graph from FKs`

---

## Task 13: Collect facade

**Files:**
- Create: `internal/core/core.go`
- Test: `internal/core/collect_test.go`, `internal/core/testdata/*`

`Collect(cfg *configv1.Config) (*irv1.Catalog, error)`: `source.Resolve` → split schema/query/migration units → `parse.Statements` for schema+migration(up) in order → `catalog.Build` → `query.Build` for query units → `relate.Derive` → set `cat.Relationships`, `cat.Schemas`, version fields → return. Hard error only on parse failure; diagnostics attached/logged.

- [ ] **Step 1: Failing integration test:** a temp project (schema dir + query dir) → `Collect` → assert tables, a query with inferred columns, ≥1 relationship.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** wiring.
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(core): Collect pipeline`

---

## Task 14: Annotation parsing

**Files:**
- Create: `internal/plugin/annotate.go`
- Test: `internal/plugin/annotate_test.go`

`Annotate(units []source.Unit, schema *pluginv1.AnnotationSchema) ([]*irv1.AnnotationValue, error)`. Generic comment parser driven by the schema: scan comments per `comment_styles`, match `sigil`+name (+aliases, case-insensitive option), tokenize value by `AnnotationValueSpec.form` (FLAG/SCALAR/POSITIONAL/KEYED/LIST/FREEFORM), coerce each token per `FieldSpec.type` → `AnnotationArg`. Target binding: leading comment → next object/query (by source proximity); inline → trailing object.

- [ ] **Step 1: Failing tests:** schema with `@cache` (KEYED ttl:DURATION, region:STRING) + `@deprecated` (FLAG); SQL comments → expected `AnnotationValue`s with typed args (`ttl` → duration_nanos=30e9).
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** the tokenizer + per-form parsing + type coercion (duration via `time.ParseDuration`).
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(plugin): generic annotation parser`

---

## Task 15: Plugin runner + binary/stdio transport

**Files:**
- Create: `internal/plugin/runner.go`, `internal/plugin/binary.go`
- Test: `internal/plugin/binary_test.go`, `internal/plugin/testdata/fakeplugin/main.go`

`Runner` interface: `GetInfo(ctx) (*pluginv1.GetInfoResponse, error)`, `Generate(ctx, *pluginv1.GenerateRequest) (*pluginv1.GenerateResponse, error)`, `Close() error`. `Open(*configv1.PluginConfig) (Runner, error)` dispatches on `PluginSource` (binary/command → this task; wasm → Task 16). Binary transport: launch the process, serve/dial the `Generator` gRPC service over its stdio pipes.

- [ ] **Step 1: Failing test:** build the fake plugin (a tiny `main` serving `GeneratorServer` over stdio that returns one file) into a temp bin, `Open` it, call `Generate`, assert the returned file.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** stdio gRPC (a `net.Conn` over stdin/stdout via `os/exec` pipes + `grpc.NewClient` with a custom dialer, or a length-prefixed stdio stream). Document the handshake the fake plugin mirrors.
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(plugin): binary/stdio gRPC runner`

---

## Task 16: wasm transport

**Files:**
- Create: `internal/plugin/wasm.go`
- Test: `internal/plugin/wasm_test.go`, `internal/plugin/testdata/echo.wasm` (or build script)

wazero runner: instantiate the `.wasm`, call exported `get_info`/`generate` passing a serialized request via linear memory (alloc/write/call/read protocol), unmarshal the response bytes. Define the ABI (exported `alloc(size) ptr`, `generate(ptr,len) packed_ptr_len`).

- [ ] **Step 1: Failing test** with a minimal echo wasm (or a Go-compiled `GOOS=wasip1` module) returning a fixed `GenerateResponse`.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** wazero host + ABI. If a real wasm fixture is infeasible in this pass, mark the test `t.Skip` with a TODO and keep the runner code compiling — the binary path (Task 15) remains the working transport.
- [ ] **Step 4: Run, expect pass (or skip).**
- [ ] **Step 5: Commit** `feat(plugin): wazero wasm runner`

---

## Task 17: Generate facade

**Files:**
- Modify: `internal/core/core.go`
- Test: `internal/core/generate_test.go`

`Generate(cfg) error`: `Collect(cfg)`; for each enabled plugin → `plugin.Open` → `GetInfo` → `Annotate(units, info.annotation_schema)` → attach annotations onto catalog `Metadata` + build flat list → build `GenerateRequest` (catalog, queries, migrations, options bytes, out_dir, annotations) → `Generate` → write each `GeneratedFile` under `plugin.out` (create dirs, set exec bit) → fail if any `ERROR` diagnostic.

- [ ] **Step 1: Failing test:** config with the fake binary plugin (Task 15) + a temp schema → `Generate` → assert the output file exists under `out`.
- [ ] **Step 2: Run, expect fail.**
- [ ] **Step 3: Implement** wiring + file writing + diagnostic gate.
- [ ] **Step 4: Run, expect pass.**
- [ ] **Step 5: Commit** `feat(core): Generate runs plugins and writes output`

---

## Self-review notes

- **Spec coverage:** config (T3), source incl. migrations (T4), parse (T1-2), nodeid (T5), mapper types/expr/stmt/ddl (T6-9), catalog + ids (T10), query inference (T11), relate (T12), Collect (T13), annotate (T14), binary + wasm transports (T15-16), Generate (T17). All spec sections mapped.
- **Fallback:** raw_sql (T7-8) and RawStatement (T8) honor the never-fail rule.
- **Type consistency:** `Unit`/`Kind` (T4) reused by T13/T14; `Runner`/`Open` (T15) reused by T16/T17; `MapType`/`MapExpr`/`MapStatement` signatures stable T6→T11.
- **Risk markers:** T11 inference shallow; T16 wasm may `t.Skip` if fixture infeasible — both noted in spec Risks.
