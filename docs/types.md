# Type mappings — `sqld-gen-go`

How the Go generator (`cmd/sqld-gen-go`) maps PostgreSQL types to Go types, and
how to override the defaults for types it does not map natively.

The mapping is driven by `scalarGoType` / `goType` in
`cmd/sqld-gen-go/types.go`. The target driver is **pgx v5**
(`github.com/jackc/pgx/v5`).

## Nullability

A `NULL`-able column normally becomes a Go pointer (`*T`). The exceptions are
types that already encode SQL `NULL` internally, which stay **value types**:

- slices (`[]T`, e.g. `bytea` → `[]byte`, arrays) — `nil` is `NULL`;
- maps (`pgtype.Hstore` = `map[string]*string`) — `nil` is `NULL`;
- `json.RawMessage` (a `[]byte`) — `nil` is `NULL`;
- the `pgtype` struct types (`Interval`, `Point`, `Bits`, `Range[T]`, …) — they
  carry a `Valid bool` (or `IsNull`) field.

Composite **parameters** are always passed by value (never a pointer): the
generated composite codec cannot encode a `NULL` composite.

## Scalar types

| PostgreSQL | Go | Import | Notes |
|---|---|---|---|
| `int2`, `smallserial` | `int16` | | |
| `int4`, `serial` | `int32` | | |
| `int8`, `bigserial` | `int64` | | |
| `bool` | `bool` | | |
| `float4` | `float32` | | |
| `float8` | `float64` | | |
| `text`, `varchar`, `bpchar`, `name`, `char`, `citext` | `string` | | |
| `numeric`, `money` | `string` | | exact text; override for a decimal type |
| `uuid` | `string` | | override to `github.com/google/uuid.UUID` (see below) |
| `bytea` | `[]byte` | | value type when nullable |
| `json`, `jsonb` | `json.RawMessage` | `encoding/json` | value type when nullable |
| `date`, `time`, `timetz`, `timestamp`, `timestamptz` | `time.Time` | `time` | |
| `inet`, `cidr` | `string` | | text form; pgx codec scans/encodes via text |

## Core pgx types (registered by default — no `RegisterTypes` needed)

These map to concrete `pgtype` structs. pgx ships default codecs for all of
them, so they need no per-connection registration. Each `pgtype` struct carries
a `Valid` field, so a nullable column is the **value type** (no pointer).

| PostgreSQL | Go | Import |
|---|---|---|
| `interval` | `pgtype.Interval` | `github.com/jackc/pgx/v5/pgtype` |
| `point` | `pgtype.Point` | `github.com/jackc/pgx/v5/pgtype` |
| `line` | `pgtype.Line` | `github.com/jackc/pgx/v5/pgtype` |
| `lseg` | `pgtype.Lseg` | `github.com/jackc/pgx/v5/pgtype` |
| `box` | `pgtype.Box` | `github.com/jackc/pgx/v5/pgtype` |
| `path` | `pgtype.Path` | `github.com/jackc/pgx/v5/pgtype` |
| `polygon` | `pgtype.Polygon` | `github.com/jackc/pgx/v5/pgtype` |
| `circle` | `pgtype.Circle` | `github.com/jackc/pgx/v5/pgtype` |
| `bit`, `varbit` | `pgtype.Bits` | `github.com/jackc/pgx/v5/pgtype` |
| `macaddr`, `macaddr8` | `net.HardwareAddr` | `net` |
| `tid` | `pgtype.TID` | `github.com/jackc/pgx/v5/pgtype` |
| `xid`, `cid` | `pgtype.Uint32` | `github.com/jackc/pgx/v5/pgtype` |

## Extension types (detected → registered at runtime)

These have no compile-time OID and no default codec keyed by name, so the
generator registers them in `RegisterTypes` (see below) **only when a column,
query result, or parameter actually uses them**.

| PostgreSQL | Go | Import | Nullable |
|---|---|---|---|
| `hstore` | `pgtype.Hstore` (`map[string]*string`) | `github.com/jackc/pgx/v5/pgtype` | value type (`nil` map) |
| `ltree`, `lquery` | `string` | | `*string` |

`hstore` requires the `hstore` extension; `ltree`/`lquery` require `ltree`. The
type names parse fine for codegen without `CREATE EXTENSION` (only the names are
needed). At runtime the extension must be installed so `LoadType` can resolve
the OID.

## Arrays

A PostgreSQL array maps to a Go slice of the element's Go type: `integer[]` →
`[]int32`, `text[]` → `[]string`, `point[]` → `[]pgtype.Point`. An array column
is a value type (`nil` is `NULL`), so it is never pointer-wrapped.

## Range and multirange types

Built-in ranges map to `pgtype.Range[T]`; built-in multiranges to
`pgtype.Multirange[pgtype.Range[T]]`. pgx has default codecs for these, so no
registration is needed. They carry `Valid`/`IsNull`, so nullable columns stay
value types.

| PostgreSQL | Go |
|---|---|
| `int4range` | `pgtype.Range[pgtype.Int4]` |
| `int8range` | `pgtype.Range[pgtype.Int8]` |
| `numrange` | `pgtype.Range[pgtype.Numeric]` |
| `tsrange` | `pgtype.Range[pgtype.Timestamp]` |
| `tstzrange` | `pgtype.Range[pgtype.Timestamptz]` |
| `daterange` | `pgtype.Range[pgtype.Date]` |
| `int4multirange` | `pgtype.Multirange[pgtype.Range[pgtype.Int4]]` |
| `int8multirange` | `pgtype.Multirange[pgtype.Range[pgtype.Int8]]` |
| `nummultirange` | `pgtype.Multirange[pgtype.Range[pgtype.Numeric]]` |
| `tsmultirange` | `pgtype.Multirange[pgtype.Range[pgtype.Timestamp]]` |
| `tstzmultirange` | `pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]` |
| `datemultirange` | `pgtype.Multirange[pgtype.Range[pgtype.Date]]` |

A **custom** `CREATE TYPE … AS RANGE` maps to `pgtype.Range[<subtype elem>]`
(and its auto-created multirange to `pgtype.Multirange[…]`); these are
registered by `RegisterTypes`.

## User-defined types (UDTs)

| Kind | Go | Registered? |
|---|---|---|
| `CREATE TYPE … AS ENUM` | a `string`-based named type with a const per label | yes (for enum arrays) |
| `CREATE DOMAIN` | transparent — the base type's Go type (no new Go type) | no |
| `CREATE TYPE … AS (…)` (composite) | a generated `struct` implementing pgx's `CompositeIndexScanner`/`Getter` | yes |
| custom range / multirange | `pgtype.Range[T]` / `pgtype.Multirange[…]` | yes |

Names are PascalCased; types outside `public` are prefixed with the PascalCased
schema (e.g. `app.address` → `AppAddress`).

## `RegisterTypes`

When the catalog or queries use any registrable type (extension type, enum,
composite, or custom range), `models.go` emits:

```go
func RegisterTypes(ctx context.Context, conn *pgx.Conn) error { /* … */ }
```

Wire it into `pgxpool.Config.AfterConnect` so it runs per connection. It calls
`conn.LoadType(ctx, name)` + `conn.TypeMap().RegisterType(t)` in a
**dependency-safe order**:

1. **extension types** (`hstore`, `ltree`, …) first — no element dependencies,
   registered by bare name (no array companion);
2. **enums**, each followed by its `_name` array type;
3. **composites** in topological order (a composite is registered after every
   composite it has a field of), each followed by its array type;
4. **custom ranges** last, each followed by its array type, then the associated
   multirange and its array (PG 14+).

pgx's `Conn.LoadType` resolves a derived type only once its element/field/subtype
types are registered, which this order guarantees.

## Custom Go types via `overrides`

For types the generator does not map natively (PostGIS `geometry`, pgvector
`vector`, …) or to change a default (`uuid`, `numeric`), use the plugin's
`overrides` option in `sqld.yaml`. A key is **either** a fully-qualified column
id (`schema.table.column`) **or** a bare PostgreSQL type name; a column-id
override beats a type-name override, which beats the default mapping.

```yaml
plugins:
  - name: go
    binary: ./bin/sqld-gen-go
    out: example/gen/db
    options:
      package: db
      overrides:
        # bare type name → applies to every column of that type
        uuid: github.com/google/uuid.UUID
        vector: github.com/pgvector/pgvector-go.Vector
        geometry: github.com/twpayne/go-geos.Geometry
        numeric: github.com/shopspring/decimal.Decimal
        # column id → applies to one column only (wins over a type-name override)
        app.kitchen_sink.c_jsonb: map[string]any
```

Override **value** rules (`parseOverrideValue`):

- a value containing `/` is an import path with the type name after the **last**
  `.`: `github.com/google/uuid.UUID` → import `github.com/google/uuid`, used as
  `uuid.UUID` (the package name is assumed to equal the last path segment);
- a value with no `/` is a bare Go type used verbatim with no import, except
  `json.*` (e.g. `json.RawMessage`) which adds `encoding/json`;
- a nullable column becomes `*T` **unless** the override type is a slice, map, or
  `json.RawMessage` (those already represent `NULL`).

Overrides only change the Go type the generator emits — they do not register a
codec. The override Go type must already be a valid pgx scan/encode target (a
registered codec, or a type implementing the relevant pgx interfaces). For
example, `github.com/pgvector/pgvector-go.Vector` ships a pgx registration
helper, and `github.com/google/uuid.UUID` is supported by pgx out of the box.
