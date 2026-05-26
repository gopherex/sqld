# sqld-gen-bob Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A `sqld-gen-bob` generator plugin that feeds sqld's IR `Catalog` into stephenafamo/bob's code generator, producing a full Go ORM (models, relationships, factories, where/loaders/joins) whose types are the SAME canonical Go types `sqld-gen-go` emits, runnable on one `*pgxpool.Pool`.

**Architecture:** Extract sqld-gen-go's pg→Go type mapper into a new public `pkg/gotypes` (null-mode aware) imported by BOTH generators. The new `cmd/sqld-gen-bob` plugin implements bob's `drivers.Interface` over a `*irv1.Catalog` using `pkg/gotypes` to force bob's `Column.Type` to the shared types, then runs `bob/gen` writing directly into the plugin's output dir.

**Tech Stack:** Go 1.25, `github.com/stephenafamo/bob v0.44.0` (`gen`, `gen/drivers`, `gen/plugins`, `drivers/pgx`), sqld IR proto (`pkg/proto/sqld/v1/{ir,plugin}`), pgx v5, testcontainers-go.

**Branch:** `feat/sqld-gen-bob` (already checked out). The compile + runtime PoC is merged on `master` at `cmd/bobgen-sqld/` (commit range `a933ac3..15fa7c8`) — reuse its `driver.go`, `main.go`, `runtime_test.go`, `pocshared/` as reference; the final code replaces it.

**Commit rule:** NO `Co-Authored-By` trailer on any commit.

---

## File Structure

```
pkg/gotypes/
  gotypes.go        NEW: Mapper{reg, overrides, null}; NewMapper, GoType, ParamType, UDTName; NullMode
  names.go          NEW: Pascal, LowerCamel, splitWords, splitCamel, capitalize, initialisms (moved from gogen.go)
  registry.go       NEW: Registry (was udtRegistry) + BuildRegistry (was buildUDTRegistry) + entry structs
  scalars.go        NEW: scalarGoType, pgtypeElement, rangeGoType, multirangeGoType, goTypeNoPointer, pgtypeStructTypes, extension helpers, parseOverrideValue
  gotypes_test.go   NEW: table-driven tests for both null modes
cmd/sqld-gen-go/
  types.go          MODIFY: delete moved code; thin shims delegating to pkg/gotypes (keep package-local names used by gogen.go)
  gogen.go          MODIFY: pascal/lowerCamel/splitWords/splitCamel call pkg/gotypes; read nullMode option (Phase 4)
cmd/sqld-gen-bob/    NEW plugin (promote/replace cmd/bobgen-sqld)
  main.go           stdio transport (tag 0=GetInfo, 1=Generate) — mirror cmd/sqld-gen-go/main.go
  info.go           Info() *pluginv1.GetInfoResponse
  driver.go         sqldDriver: drivers.Interface[any,any,any]; Catalog→DBInfo via pkg/gotypes
  options.go        bobOptions{Package, TypesPackage, NullMode, Models, Factories, Relationships, WhereLoadersJoins}
  generate.go       Generate(req): build driver, run bob/gen into req.OutDir, return empty files
  driver_test.go    unit: Catalog→DBInfo (no DB)
  generate_test.go  golden: example schema → bob output compiles + references shared types
  runtime_test.go   integration (testcontainers): bob write + sqld read, one pool (promote PoC)
example/
  sqld.yaml         MODIFY: add the bob plugin entry
  gen/bob/...        generated bob output (committed)
  bob_symbiosis_test.go  NEW: mixes a bob model + a sqld query on one pool
docs/
  bob.md            NEW: usage
  adr.md            MODIFY: ADR for the symbiosis + pkg/gotypes + direct-write
  README.md         MODIFY: mention sqld-gen-bob
Makefile            MODIFY: build sqld-gen-bob; example target runs it
```

---

## Phase 1 — Extract `pkg/gotypes` (behavior-preserving, pointer mode)

Goal of phase: a public type mapper used by sqld-gen-go with byte-identical output. No bob yet.

### Task 1: Scaffold `pkg/gotypes` with naming + registry + scalars (pure moves)

**Files:**
- Create: `pkg/gotypes/names.go`, `pkg/gotypes/registry.go`, `pkg/gotypes/scalars.go`
- Reference: `cmd/sqld-gen-go/gogen.go:150-238` (pascal/lowerCamel/splitWords/splitCamel/initialisms/capitalize), `cmd/sqld-gen-go/types.go` (all of it)

- [ ] **Step 1: Create `pkg/gotypes/names.go`** — move, verbatim, from `cmd/sqld-gen-go/gogen.go`: `initialisms` map, `capitalize`, `pascal`, `lowerCamel`, `splitWords`, `splitCamel`. Export the two callers need: rename `pascal`→`Pascal`, `lowerCamel`→`LowerCamel`. Keep `capitalize`, `splitWords`, `splitCamel`, `initialisms` unexported. Package clause `package gotypes`. Add `import ("strings"; "unicode")`.

```go
// pkg/gotypes/names.go
package gotypes

import (
	"strings"
	"unicode"
)

var initialisms = map[string]string{ /* copy exact map from gogen.go */ }

func capitalize(s string) string { /* copy exact */ }

// Pascal converts a snake_case name to PascalCase applying Go initialisms.
func Pascal(s string) string { /* body of gogen.go pascal, calling capitalize/splitWords */ }

// LowerCamel converts to lowerCamelCase.
func LowerCamel(s string) string { /* body of gogen.go lowerCamel */ }

func splitWords(s string) []string { /* copy exact */ }
func splitCamel(s string) []string { /* copy exact */ }
```

- [ ] **Step 2: Create `pkg/gotypes/registry.go`** — move from `cmd/sqld-gen-go/types.go` (lines ~12-105): the entry structs (`enumEntry`, `domainEntry`, `compositeEntry`, `rangeEntry`, `multirangeEntry`), the `udtRegistry` struct, `buildUDTRegistry`. Rename `udtRegistry`→`Registry`, `buildUDTRegistry`→`BuildRegistry`. Keep entry structs unexported. Add accessor methods used by the bob driver later (so its internals stay private):

```go
// pkg/gotypes/registry.go
package gotypes

import irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"

type enumEntry struct { schema string; e *irv1.EnumType }
type domainEntry struct { schema string; d *irv1.DomainType }
type compositeEntry struct { schema string; c *irv1.CompositeType }
type rangeEntry struct { schema string; r *irv1.RangeType }
type multirangeEntry struct { schema string; subtype *irv1.TypeRef }

type Registry struct {
	enums       map[string]enumEntry
	domains     map[string]domainEntry
	composites  map[string]compositeEntry
	ranges      map[string]rangeEntry
	multiranges map[string]multirangeEntry
}

func BuildRegistry(catalog *irv1.Catalog) *Registry { /* body of buildUDTRegistry */ }

// IsEnum reports whether bare pg type name is a registered enum.
func (r *Registry) IsEnum(pgName string) bool { _, ok := r.enums[pgName]; return ok }
// EnumSchema returns the declaring schema of an enum (for UDTName).
func (r *Registry) EnumSchema(pgName string) (string, bool) { e, ok := r.enums[pgName]; return e.schema, ok }
```

- [ ] **Step 3: Create `pkg/gotypes/scalars.go`** — move from `cmd/sqld-gen-go/types.go`: `extensionTypeNames`, `collectExtensionTypeName`, `collectUsedExtensionTypes`, `udtGoTypeName`, `parseOverrideValue`, `pgtypeStructTypes`, `goTypeNoPointer`, `pgtypeElement`, `rangeGoType`, `multirangeGoType`, `scalarGoType`, `isCompositeType`, `compositeDepName`. Keep them as package functions. Rename `udtGoTypeName`→`UDTName` (exported), `collectUsedExtensionTypes`→`CollectUsedExtensionTypes` (exported — used by gen-go). `isCompositeType`/`compositeDepName` take `*Registry` now. Imports: `path`, `sort`, `strings`, `irv1`, `pluginv1`.

- [ ] **Step 4: Build the package** — `go build ./pkg/gotypes/` — Expected: compiles (no consumers yet). Fix any cross-references (the moved `goType`/`resolveGoType` come in Task 2; for now `scalars.go` must not reference them — they live in `types.go` still, so temporarily the package won't have `goType`. Move `goType`/`resolveGoType`/`overrideGoType`/`goParamType`/`resolveGoParamType` in Task 2). If build fails on missing `goType`, that's expected until Task 2 — proceed.

- [ ] **Step 5: Commit**
```bash
git add pkg/gotypes/names.go pkg/gotypes/registry.go pkg/gotypes/scalars.go
git commit -m "feat(gotypes): scaffold public pg→Go mapper — names, registry, scalars"
```

### Task 2: Add `Mapper` (null-mode aware) and the core mapping funcs

**Files:**
- Create: `pkg/gotypes/gotypes.go`
- Reference: `cmd/sqld-gen-go/types.go:241-425` (resolveGoType/overrideGoType/goType/goParamType/resolveGoParamType)

- [ ] **Step 1: Create `pkg/gotypes/gotypes.go`** with `NullMode` and `Mapper`. Move `goType`, `resolveGoType`, `overrideGoType`, `goParamType`, `resolveGoParamType` here. Thread null mode: in `Pointer` mode behave EXACTLY as today (`*T`); add `Opt` mode (used in Phase 4 — for now implement the type expressions so the mapper is complete, but gen-go keeps Pointer).

```go
// pkg/gotypes/gotypes.go
package gotypes

import irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"

type NullMode int

const (
	Pointer NullMode = iota // nullable scalar → *T (current sqld-gen-go behavior)
	Opt                     // nullable scalar → null.Val[T]; optional param → omit.Val[T]
)

type Overrides map[string]string

type Mapper struct {
	reg  *Registry
	ov   Overrides
	null NullMode
}

func NewMapper(cat *irv1.Catalog, ov Overrides, null NullMode) *Mapper {
	return &Mapper{reg: BuildRegistry(cat), ov: ov, null: null}
}

// Registry exposes the underlying registry (the bob driver needs it).
func (m *Mapper) Registry() *Registry { return m.reg }

// GoType is resolveGoType: override (by columnID, then pg name) else default mapping.
func (m *Mapper) GoType(columnID string, t *irv1.TypeRef, nullable bool) (string, []string) {
	return m.resolve(columnID, t, nullable, false)
}

// ParamType forces composite params to a value type (resolveGoParamType).
func (m *Mapper) ParamType(columnID string, t *irv1.TypeRef, nullable bool) (string, []string) {
	return m.resolve(columnID, t, nullable, true)
}

// UDTName is the exported Go identifier for a UDT (enum/composite). schema "public"/"" → no prefix.
func (m *Mapper) UDTName(schema, pgName string) string { return UDTName(schema, pgName) }

func (m *Mapper) resolve(columnID string, t *irv1.TypeRef, nullable, param bool) (string, []string) {
	if m.ov != nil {
		if columnID != "" { if v, ok := m.ov[columnID]; ok { return m.override(v, nullable) } }
		if t != nil { if v, ok := m.ov[t.GetPgName()]; ok { return m.override(v, nullable) } }
	}
	if param && isCompositeType(m.reg, t) { return m.base(t, false) }
	return m.base(t, nullable)
}

// base = goType, parameterized by null mode.
func (m *Mapper) base(t *irv1.TypeRef, nullable bool) (string, []string) { /* body of goType, but call m.wrapNull instead of inline "*"+expr; enum/composite nullable handled via wrapNull too */ }

func (m *Mapper) override(v string, nullable bool) (string, []string) {
	expr, imps := parseOverrideValue(v)
	return m.wrapNull(expr, imps, nullable)
}

// wrapNull applies the null-mode wrapper unless the type already encodes NULL.
func (m *Mapper) wrapNull(expr string, imps []string, nullable bool) (string, []string) {
	if !nullable || goTypeNoPointer(expr) { return expr, imps }
	switch m.null {
	case Opt:
		return "null.Val[" + expr + "]", append(imps, "github.com/aarondl/opt/null")
	default: // Pointer
		return "*" + expr, imps
	}
}
```

Note: rework `base` so the enum/composite/domain/range/multirange branches (from `goType`) call `m.wrapNull(typeName, nil, nullable)` rather than inlining `"*"+typeName`. This keeps the Pointer-mode output identical (`*AppUserStatus`) and gives Opt mode for free (`null.Val[AppUserStatus]`).

- [ ] **Step 2: Write `pkg/gotypes/gotypes_test.go`** — table-driven, asserting Pointer mode matches today and Opt mode wraps correctly.

```go
package gotypes

import (
	"testing"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

func scalarRef(pg string) *irv1.TypeRef { return &irv1.TypeRef{PgName: pg} }

func TestGoTypePointerMode(t *testing.T) {
	m := NewMapper(&irv1.Catalog{}, nil, Pointer)
	cases := []struct{ pg string; nullable bool; want string }{
		{"int8", false, "int64"},
		{"int8", true, "*int64"},
		{"text", true, "*string"},
		{"jsonb", true, "json.RawMessage"},   // nil-capable, no pointer
		{"bytea", true, "[]byte"},
	}
	for _, c := range cases {
		got, _ := m.GoType("", scalarRef(c.pg), c.nullable)
		if got != c.want { t.Errorf("%s nullable=%v: got %q want %q", c.pg, c.nullable, got, c.want) }
	}
}

func TestGoTypeOptMode(t *testing.T) {
	m := NewMapper(&irv1.Catalog{}, nil, Opt)
	got, imps := m.GoType("", scalarRef("text"), true)
	if got != "null.Val[string]" { t.Errorf("got %q", got) }
	wantImp := false
	for _, i := range imps { if i == "github.com/aarondl/opt/null" { wantImp = true } }
	if !wantImp { t.Errorf("missing null import: %v", imps) }
	// nil-capable types are not wrapped even in opt mode:
	if got, _ := m.GoType("", scalarRef("jsonb"), true); got != "json.RawMessage" { t.Errorf("jsonb opt got %q", got) }
}
```

- [ ] **Step 3: Run** `go test ./pkg/gotypes/ -run TestGoType -v` — Expected: FAIL (Mapper/base incomplete) → implement `base` fully → PASS.

- [ ] **Step 4: Commit**
```bash
git add pkg/gotypes/gotypes.go pkg/gotypes/gotypes_test.go
git commit -m "feat(gotypes): Mapper with selectable null mode (pointer|opt)"
```

### Task 3: Re-wire `cmd/sqld-gen-go` onto `pkg/gotypes` (output unchanged)

**Files:**
- Modify: `cmd/sqld-gen-go/types.go` (replace moved code with shims), `cmd/sqld-gen-go/gogen.go` (naming helpers delegate)

- [ ] **Step 1: Replace `cmd/sqld-gen-go/types.go`** body — delete everything moved; keep package-local shims so `gogen.go` compiles unchanged:

```go
package main

import (
	"github.com/yaroher/sqld/pkg/gotypes"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

type udtRegistry = gotypes.Registry
type overrides = gotypes.Overrides

func buildUDTRegistry(c *irv1.Catalog) *udtRegistry { return gotypes.BuildRegistry(c) }
func udtGoTypeName(schema, name string) string      { return gotypes.UDTName(schema, name) }
func collectUsedExtensionTypes(c *irv1.Catalog, q []*pluginv1.Query) []string {
	return gotypes.CollectUsedExtensionTypes(c, q)
}

// mapper used by generate*: Pointer mode preserves current output.
var goMapper *gotypes.Mapper

func resolveGoType(reg *udtRegistry, ov overrides, columnID string, t *irv1.TypeRef, nullable bool) (string, []string) {
	return gotypes.NewMapper2(reg, ov, gotypes.Pointer).GoType(columnID, t, nullable)
}
func resolveGoParamType(reg *udtRegistry, ov overrides, columnID string, t *irv1.TypeRef, nullable bool) (string, []string) {
	return gotypes.NewMapper2(reg, ov, gotypes.Pointer).ParamType(columnID, t, nullable)
}
```

Add a `gotypes.NewMapper2(reg *Registry, ov Overrides, null NullMode) *Mapper` constructor that takes an already-built registry (the gen-go callers pass `reg`), to avoid rebuilding the registry per call:
```go
// in pkg/gotypes/gotypes.go
func NewMapper2(reg *Registry, ov Overrides, null NullMode) *Mapper { return &Mapper{reg: reg, ov: ov, null: null} }
```
Also expose, for the few `isCompositeType`/`compositeDepName` callers in gogen.go, shims in types.go:
```go
func isCompositeType(reg *udtRegistry, t *irv1.TypeRef) bool { return gotypes.IsCompositeType(reg, t) }
func compositeDepName(reg *udtRegistry, t *irv1.TypeRef) string { return gotypes.CompositeDepName(reg, t) }
```
(Export `IsCompositeType`/`CompositeDepName` from `pkg/gotypes/scalars.go`.) Likewise `goTypeNoPointer` if gogen.go calls it — grep and add a shim.

- [ ] **Step 2: Update `cmd/sqld-gen-go/gogen.go`** — delete `pascal`, `lowerCamel`, `splitWords`, `splitCamel`, `capitalize`, `initialisms` (now in gotypes). Add local shims to minimize churn:
```go
func pascal(s string) string     { return gotypes.Pascal(s) }
func lowerCamel(s string) string { return gotypes.LowerCamel(s) }
```
Add `"github.com/yaroher/sqld/pkg/gotypes"` import; drop now-unused `unicode` import if no longer referenced.

- [ ] **Step 3: Build + run gen-go tests** — `go build ./cmd/sqld-gen-go/ && go test ./cmd/sqld-gen-go/ ./pkg/gotypes/` — Expected: PASS (the existing `gogen_test.go`/`types_test.go` lock behavior). Fix shim signatures until green.

- [ ] **Step 4: Verify example golden is byte-identical** — rebuild + regenerate the example and diff against committed output:
```bash
go build -o bin/sqld ./cmd/sqld && go build -o bin/sqld-gen-go ./cmd/sqld-gen-go
./bin/sqld generate -c example/sqld.yaml
git diff --stat example/gen/db/
```
Expected: NO diff under `example/gen/db/` (pointer mode preserved). If diff appears, the extraction changed behavior — fix before committing.

- [ ] **Step 5: Commit**
```bash
git add cmd/sqld-gen-go/types.go cmd/sqld-gen-go/gogen.go pkg/gotypes/
git commit -m "refactor(sqld-gen-go): consume pkg/gotypes; output unchanged (pointer mode)"
```

---

## Phase 2 — `sqld-gen-bob` plugin

Goal: a working plugin that generates bob ORM with shared types, in pointer mode (gen-go already pointer). Promotes the PoC.

### Task 4: Move PoC to `cmd/sqld-gen-bob` + options

**Files:**
- Create: `cmd/sqld-gen-bob/options.go`
- Move: `cmd/bobgen-sqld/*` → `cmd/sqld-gen-bob/` (then heavily edit)

- [ ] **Step 1: Relocate** the PoC: `git mv cmd/bobgen-sqld cmd/sqld-gen-bob`. Delete the PoC stand-ins that the real plugin replaces: `cmd/sqld-gen-bob/pocshared/`, `cmd/sqld-gen-bob/out/`, `cmd/sqld-gen-bob/testdata/`, `cmd/sqld-gen-bob/unify_test.go`. Keep `driver.go`, `main.go`, `runtime_test.go` as starting points (they will be rewritten in later tasks).

- [ ] **Step 2: Create `cmd/sqld-gen-bob/options.go`**:
```go
package main

import "encoding/json"

type bobOptions struct {
	Package           string `json:"package"`            // default "models"
	TypesPackage      string `json:"typesPackage"`       // import path of sqld-gen-go's leaf-types package (for shared enum/composite types)
	NullMode          string `json:"nullMode"`           // "pointer" (default) | "opt"; MUST match sqld-gen-go
	Models            *bool  `json:"models"`             // default true
	Factories         *bool  `json:"factories"`          // default true
	Relationships     *bool  `json:"relationships"`      // default true
	WhereLoadersJoins *bool  `json:"whereLoadersJoins"`  // default true
}

func parseBobOptions(raw []byte) (bobOptions, error) {
	o := bobOptions{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &o); err != nil { return o, err }
	}
	if o.Package == "" { o.Package = "models" }
	return o, nil
}

func (o bobOptions) on(p *bool) bool { return p == nil || *p }
```

- [ ] **Step 3: Test** `cmd/sqld-gen-bob/options_test.go`:
```go
package main

import "testing"

func TestParseBobOptionsDefaults(t *testing.T) {
	o, err := parseBobOptions([]byte(`{"typesPackage":"x/db"}`))
	if err != nil { t.Fatal(err) }
	if o.Package != "models" { t.Errorf("package=%q", o.Package) }
	if !o.on(o.Models) || !o.on(o.Factories) { t.Errorf("defaults should be on") }
	if o.TypesPackage != "x/db" { t.Errorf("typesPackage=%q", o.TypesPackage) }
}
func TestParseBobOptionsNullMode(t *testing.T) {
	o, _ := parseBobOptions([]byte(`{"nullMode":"opt","models":false}`))
	if o.NullMode != "opt" { t.Errorf("nullMode=%q", o.NullMode) }
	if o.on(o.Models) { t.Errorf("models should be off") }
}
```
Run `go test ./cmd/sqld-gen-bob/ -run TestParseBobOptions` — Expected: PASS.

- [ ] **Step 4: Commit**
```bash
git add -A cmd/sqld-gen-bob/ && git rm -r --cached cmd/bobgen-sqld 2>/dev/null; true
git commit -m "feat(sqld-gen-bob): relocate PoC to cmd/sqld-gen-bob; add options"
```

### Task 5: `driver.go` — Catalog→DBInfo via `pkg/gotypes`

**Files:**
- Rewrite: `cmd/sqld-gen-bob/driver.go`
- Reference: the PoC `driver.go` (pre-move) for bob struct-filling mechanics; `pkg/proto/sqld/v1/ir` getters.

The driver converts a real `*irv1.Catalog` into bob's `DBInfo`, using a `*gotypes.Mapper` to set `Column.Type` and register imports. `TypesPackage` is the import path where enum/composite Go types live; for those columns the registered `drivers.Type.Imports` references it.

- [ ] **Step 1: Write `driver.go`**:
```go
package main

import (
	"context"
	"fmt"

	"github.com/stephenafamo/bob/gen/drivers"
	"github.com/yaroher/sqld/pkg/gotypes"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

type sqldDriver struct {
	cat          *irv1.Catalog
	mapper       *gotypes.Mapper
	typesPackage string
	types        drivers.Types
}

func newDriver(cat *irv1.Catalog, mapper *gotypes.Mapper, typesPackage string) *sqldDriver {
	return &sqldDriver{cat: cat, mapper: mapper, typesPackage: typesPackage}
}

func (d *sqldDriver) Dialect() string { return "psql" }
func (d *sqldDriver) Types() drivers.Types { return d.types }

func (d *sqldDriver) Assemble(ctx context.Context) (*drivers.DBInfo[any, any, any], error) {
	info := &drivers.DBInfo[any, any, any]{Driver: "github.com/jackc/pgx/v5/stdlib"}
	for _, sc := range d.cat.GetSchemas() {
		for _, t := range sc.GetTables() {
			tbl, err := d.table(sc.GetName(), t)
			if err != nil { return nil, fmt.Errorf("table %s: %w", t.GetId(), err) }
			info.Tables = append(info.Tables, tbl)
		}
	}
	// Enums intentionally left empty: sqld-gen-go owns enum Go types; leaving
	// DBInfo.Enums empty stops bob emitting a competing enum package.
	return info, nil
}

func (d *sqldDriver) table(schema string, t *irv1.Table) (drivers.Table[any, any], error) {
	tbl := drivers.Table[any, any]{
		Key:    t.GetId(),                 // canonical "schema.table"
		Schema: schema,
		Name:   t.GetName().GetName(),
	}
	for _, c := range t.GetColumns() {
		expr, imps := d.mapper.GoType(c.GetId(), c.GetType(), c.GetNullable())
		d.registerType(expr, imps)
		tbl.Columns = append(tbl.Columns, drivers.Column{
			Name:     c.GetName(),
			DBType:   c.GetType().GetPgName(),
			Type:     expr,
			Nullable: c.GetNullable(),
			Generated: c.GetGenerated() != nil,
		})
	}
	tbl.Constraints = d.constraints(t)
	tbl.Indexes = d.indexes(t)
	return tbl, nil
}

// registerType records a Go type expression in bob's Types registry with its
// imports, so bob emits the import when a column uses it. Bare/builtin types
// (int64, string, bool) need no import and are skipped.
func (d *sqldDriver) registerType(expr string, imps []string) {
	if expr == "" { return }
	if d.types.Contains(expr) { return }
	d.types.Register(expr, drivers.Type{Imports: quoteImports(imps)})
}

func quoteImports(imps []string) []string {
	out := make([]string, 0, len(imps))
	for _, i := range imps { out = append(out, fmt.Sprintf("%q", i)) }
	return out
}
```

- [ ] **Step 2: Implement `constraints` and `indexes`** — map IR `Table.Constraints`/`Table.Indexes` to bob. Reference the PoC's mapping; the IR `Constraint` has a type enum (PK/FK/UNIQUE/NOT_NULL/CHECK/EXCLUSION) — read `pkg/proto/sqld/v1/ir/schema.pb.go` for `Constraint`, `ForeignKeyAction`, `Index` getters. PK → `drivers.Constraints.Primary` (synthesize name `<table>_pkey` if empty); UNIQUE → `Uniques`; FK → `Foreign` with `ForeignTable`/`ForeignColumns`. Skip `NOT_NULL`/`CHECK`/`EXCLUSION` for relationship purposes (bob derives relationships from PK+FK+unique only). Signatures:
```go
func (d *sqldDriver) constraints(t *irv1.Table) drivers.Constraints[any] { /* ... */ }
func (d *sqldDriver) indexes(t *irv1.Table) []drivers.Index[any] { /* ... */ }
```

- [ ] **Step 3: Write `driver_test.go`** — build a small `*irv1.Catalog` in code (one schema, one enum, one table `accounts(id int8 pk, email text, status account_status, bio text null)`) and assert the produced `DBInfo`:
```go
package main

import (
	"context"
	"testing"
	"github.com/yaroher/sqld/pkg/gotypes"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

func testCatalog() *irv1.Catalog { /* construct: schema "public" with enum account_status + table accounts */ }

func TestAssembleColumnTypes(t *testing.T) {
	cat := testCatalog()
	m := gotypes.NewMapper(cat, nil, gotypes.Pointer)
	d := newDriver(cat, m, "example.com/app/db")
	info, err := d.Assemble(context.Background())
	if err != nil { t.Fatal(err) }
	if len(info.Tables) != 1 { t.Fatalf("tables=%d", len(info.Tables)) }
	cols := map[string]string{}
	for _, c := range info.Tables[0].Columns { cols[c.Name] = c.Type }
	if cols["id"] != "int64" { t.Errorf("id=%q", cols["id"]) }
	if cols["status"] != "AccountStatus" { t.Errorf("status=%q", cols["status"]) }   // shared enum type, no pointer
	if cols["bio"] != "*string" { t.Errorf("bio=%q", cols["bio"]) }                  // nullable pointer mode
	if len(info.Enums) != 0 { t.Errorf("Enums must be empty, got %d", len(info.Enums)) }
}
```
Run `go test ./cmd/sqld-gen-bob/ -run TestAssemble` — Expected: FAIL then PASS after implementing constraints/indexes.

Note on the enum import: when `status` resolves to `AccountStatus`, its import must be `d.typesPackage`. In `registerType`, if the expr is a bare UDT name (enum/composite), inject the `typesPackage` import and adjust the expr to `<pkgbase>.<Name>`. Add this resolution: ask the mapper whether a type is a UDT (`mapper.Registry().IsEnum(pgName)` / composite) at the call site in `table()` and, when so, set `expr = pkgBase + "." + name` and `imps = []string{typesPackage}`. Pin this down: the bob model field must read `db.AccountStatus` with `import db "<typesPackage>"`. Use bob's alias import form `fmt.Sprintf("%s %q", pkgBase, typesPackage)` where `pkgBase` is the last path segment of `typesPackage`.

- [ ] **Step 4: Commit**
```bash
git add cmd/sqld-gen-bob/driver.go cmd/sqld-gen-bob/driver_test.go
git commit -m "feat(sqld-gen-bob): Catalog→bob DBInfo with shared leaf types"
```

### Task 6: `generate.go` — run bob into OutDir, return empty files

**Files:**
- Create: `cmd/sqld-gen-bob/generate.go`
- Reference: `/tmp/bobref/gen/bobgen-psql/main.go` (plugins.Setup + State + Run), PoC `main.go`.

The host writes `resp.GetFiles()` into `pc.Out` (`internal/core/core.go:243`) but does NOT wipe it. bob writes multi-package output with import paths rooted at the output location, so bob must write directly to `req.GetOutDir()`. Therefore `Generate` runs bob with output folders under `req.GetOutDir()` and returns a `GenerateResponse{}` with NO files.

- [ ] **Step 1: Write `generate.go`**:
```go
package main

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/stephenafamo/bob/gen"
	"github.com/stephenafamo/bob/gen/plugins"
	"github.com/yaroher/sqld/pkg/gotypes"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

func Generate(req *pluginv1.GenerateRequest) (*pluginv1.GenerateResponse, error) {
	opts, err := parseBobOptions(req.GetOptions())
	if err != nil { return nil, fmt.Errorf("options: %w", err) }

	null := gotypes.Pointer
	if opts.NullMode == "opt" { null = gotypes.Opt }
	mapper := gotypes.NewMapper(req.GetCatalog(), nil, null)
	driver := newDriver(req.GetCatalog(), mapper, opts.TypesPackage)

	out := req.GetOutDir()
	pcfg := plugins.Config{
		Models:  plugins.OutputConfig{/* dir: filepath.Join(out, opts.Package); enabled per opts.Models */},
		Factory: plugins.OutputConfig{/* dir + enabled per opts.Factories */},
		// Where/Loaders/Joins/Counts toggled by opts.WhereLoadersJoins; DBInfo/DBErrors per taste.
	}
	outPlugins := plugins.Setup[any, any, any](pcfg, gen.PSQLTemplates)

	typeSystem := ""                                   // aarondl/opt (pointer-compatible defaults)
	if null == gotypes.Pointer { typeSystem = "github.com/aarondl/opt/null" } // pointer-based optionals
	state := &gen.State[any]{Config: gen.Config[any]{TypeSystem: typeSystem}}

	if err := gen.Run[any, any, any](context.Background(), state, driver, outPlugins...); err != nil {
		return nil, fmt.Errorf("bob gen: %w", err)
	}
	_ = filepath.Join // bob wrote directly into out
	return &pluginv1.GenerateResponse{}, nil // empty: bob already wrote the files
}
```
Read `/tmp/bobref/gen/plugins/plugins.go` for the exact `plugins.OutputConfig` field names (Path/Disabled/PkgName). Set each output's directory under `out` and disable the ones the options turn off. Confirm whether `plugins.Setup` outputs self-register on `gen.State` (they do via StatePlugin in `gen.Run`) — no manual `state.Outputs` needed.

- [ ] **Step 2: Wire `main.go`** transport (mirror `cmd/sqld-gen-go/main.go` exactly): `run(stdin, stdout)` reads 1 tag byte; `0`→`Info()`, `1`→unmarshal `GenerateRequest`→`Generate`. Marshal response to stdout. Replace the PoC `main.go`.

- [ ] **Step 3: Write `info.go`**:
```go
package main

import pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"

func Info() *pluginv1.GetInfoResponse {
	return &pluginv1.GetInfoResponse{
		Name:    "sqld-gen-bob",
		Version: "0.1.0",
		// No comment annotations: bob generates from schema structure only.
	}
}
```
(Check `GetInfoResponse` fields in `pkg/proto/sqld/v1/plugin/plugin.pb.go`; set whatever the gen-go `Info()` sets minus the annotation schema.)

- [ ] **Step 4: Build** `go build ./cmd/sqld-gen-bob/` — Expected: compiles. Fix bob API field names against `/tmp/bobref` until green.

- [ ] **Step 5: Commit**
```bash
git add cmd/sqld-gen-bob/generate.go cmd/sqld-gen-bob/main.go cmd/sqld-gen-bob/info.go
git commit -m "feat(sqld-gen-bob): plugin transport + run bob into OutDir"
```

### Task 7: Golden test — example schema → bob output compiles + shared types

**Files:**
- Create: `cmd/sqld-gen-bob/generate_test.go`

- [ ] **Step 1: Write the golden test** — drive `Generate` against the example schema's catalog (collect it via `pkg/sqld.CollectFile` on a temp config, or load `example/gen/catalog.json`), into an in-module temp dir (bob needs go.mod resolution — use a dir under the repo, NOT `t.TempDir()`, per the PoC finding). Assert: a known model file exists, contains the shared-type field, and compiles via `go/parser` or a `go build` of the output dir.
```go
func TestGenerateExampleModels(t *testing.T) {
	// 1. collect example catalog
	// 2. req := &pluginv1.GenerateRequest{Catalog: cat, OutDir: <in-module temp>, Options: []byte(`{"package":"models","typesPackage":"github.com/yaroher/sqld/example/gen/db"}`)}
	// 3. Generate(req)
	// 4. read <out>/models/*.bob.go; assert it imports the typesPackage and a known enum column field uses db.<EnumName>
	// 5. go build the output dir; assert no error
}
```
Use `cmd/sqld-gen-bob/.gen-test/` as the in-module out dir; `t.Cleanup` removes it.

- [ ] **Step 2: Run** `go test ./cmd/sqld-gen-bob/ -run TestGenerateExample -v` — Expected: PASS (iterate on bob config until the model references `db.<Enum>` and compiles).

- [ ] **Step 3: Commit**
```bash
git add cmd/sqld-gen-bob/generate_test.go
git commit -m "test(sqld-gen-bob): golden — example schema generates compiling bob models with shared types"
```

### Task 8: Promote runtime symbiosis test

**Files:**
- Rewrite: `cmd/sqld-gen-bob/runtime_test.go` (from the PoC) to use the example-generated models + the example's `db` package types, on one pgxpool, Docker-gated via `internal/devdb` pattern.

- [ ] **Step 1: Adapt** the PoC `runtime_test.go`: start testcontainers PG (reuse `devdb.Start`), apply the example schema, open one `*pgxpool.Pool` with `AfterConnect` calling the example `db.RegisterTypes`, wrap with `bobpgx.NewPool`, INSERT via a bob model, SELECT the enum/composite via raw pgx into the shared `db.*` type, assert equality. `t.Skip` when Docker unavailable.

- [ ] **Step 2: Run** `go test ./cmd/sqld-gen-bob/ -run Runtime -count=1` — Expected: PASS (~10s, Docker).

- [ ] **Step 3: Commit**
```bash
git add cmd/sqld-gen-bob/runtime_test.go
git commit -m "test(sqld-gen-bob): runtime symbiosis on one pgxpool (testcontainers)"
```

---

## Phase 3 — Example, Makefile, docs

### Task 9: Wire the example + Makefile

**Files:**
- Modify: `example/sqld.yaml`, `Makefile`
- Create: `example/bob_symbiosis_test.go`, `example/gen/bob/...` (generated, committed)

- [ ] **Step 1: Add the bob plugin** to `example/sqld.yaml` after the `go` plugin:
```yaml
  - name: bob
    binary: bin/sqld-gen-bob
    out: example/gen/bob
    options:
      package: models
      typesPackage: github.com/yaroher/sqld/example/gen/db
      nullMode: pointer
```
- [ ] **Step 2: Makefile** — add `go build -o bin/sqld-gen-bob ./cmd/sqld-gen-bob` to `build`; the `example` target already runs `./bin/sqld generate -c example/sqld.yaml` (which now runs both plugins). Add `go build ./example/...` is already present.

- [ ] **Step 3: Regenerate + write `example/bob_symbiosis_test.go`** — a compile-level test (no DB) asserting a bob model field and a sqld query-row field are the same `db.*` type (e.g. `var _ db.AppUserStatus = bobmodels.User{}.Status`). Run `make example && go build ./example/... && go test ./example/...`.

- [ ] **Step 4: Commit**
```bash
git add example/sqld.yaml Makefile example/gen/bob example/bob_symbiosis_test.go
git commit -m "feat(example): generate bob ORM alongside sqld-gen-go; symbiosis test"
```

### Task 10: Docs + ADR

**Files:**
- Create: `docs/bob.md`; Modify: `docs/adr.md`, `docs/README.md`

- [ ] **Step 1: `docs/bob.md`** — what sqld-gen-bob is, the two-plugin config (gen-go owns types, gen-bob owns ORM), null-mode must match, the one-pool runtime snippet (`bobpgx.NewPool` + `AfterConnect`/`RegisterTypes`).
- [ ] **Step 2: `docs/adr.md`** — append ADRs: "ORM⊕sqlc symbiosis via bob", "pkg/gotypes shared mapper", "sqld-gen-bob writes directly to OutDir (returns empty files)", "null-mode selectable, both plugins must match".
- [ ] **Step 3: README** — one line + link to `docs/bob.md`.
- [ ] **Step 4: Commit**
```bash
git add docs/ && git commit -m "docs: sqld-gen-bob usage + ADRs"
```

---

## Phase 4 — Opt null-mode for sqld-gen-go (optional, heaviest)

Goal: let a project pick `nullMode: opt` end-to-end. Pointer mode already works without this phase. Only do this phase if opt-mode interop is required.

### Task 11: `nullMode` option in sqld-gen-go + opt scan/encode

**Files:**
- Modify: `cmd/sqld-gen-go/gogen.go` (read option, thread `gotypes.Opt` into the mapper; adjust scan/encode for `null.Val[T]`/`omit.Val[T]`)

- [ ] **Step 1: Read `nullMode` from options** — extend the gen-go options struct (`goConfig` near `gogen.go:95`) with `NullMode string`; map `"opt"`→`gotypes.Opt` else `gotypes.Pointer`; replace the `resolveGoType`/`resolveGoParamType` shims to use a package-level `goMapper := gotypes.NewMapper2(reg, ov, mode)`.
- [ ] **Step 2: Verify pgx scan compatibility** — confirm `null.Val[T]` and `omit.Val[T]` satisfy pgx scan/encode (write a tiny testcontainers test scanning `SELECT null::text` into `null.Val[string]`). If pgx needs a wrapper, generate `.Ptr()`/`.MustGet()` glue in the scan code. Document the finding.
- [ ] **Step 3: Golden for opt mode** — add an opt-mode variant of one example query; assert the row struct uses `null.Val[...]` and compiles.
- [ ] **Step 4: Commit**
```bash
git add cmd/sqld-gen-go/ && git commit -m "feat(sqld-gen-go): selectable nullMode (opt) for bob interop"
```

---

## Final verification

- [ ] `go build ./...` ; `go vet ./...` ; `gofmt -l` clean (ignore `pkg/proto`, `example/gen`).
- [ ] `go test ./...` green (incl. testcontainers integration).
- [ ] `make build && make example` regenerates both plugins; `go build ./example/...` green.
- [ ] No `Co-Authored-By` trailer in any commit (`git log origin/master..HEAD --grep=Co-Authored-By` → empty).
- [ ] Dispatch a final code reviewer over the whole branch, then use superpowers:finishing-a-development-branch.
