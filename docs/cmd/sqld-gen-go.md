# sqld-gen-go — Go code generator for sqld

`sqld-gen-go` is sqld's built-in Go code-generation plugin. It reads sqld's IR
`Catalog` (schema types, tables) together with a set of named SQL queries and
emits typed Go: model structs for every table, a `*Queries` type with pgx v5
backed methods, query-row structs, typed enum/composite/domain/range Go types,
and a `RegisterTypes` helper. It is the sqlc-equivalent inside the sqld toolchain
and adds one capability that sqlc does not have: **typed dynamic queries** —
where optional WHERE conditions, slice-IN conditions, and an allowlisted ORDER BY
column are all resolved at runtime with a strongly-typed Go params struct rather
than a hand-written query builder.

The plugin is distributed both as a native binary and as a WASI 1 (wasip1) WASM
module so it can run on any platform that the sqld host supports.

---

## Install / build

Install the latest release with `go install`:

```bash
go install github.com/yaroher/sqld/cmd/sqld-gen-go@latest
```

Or build from the repository (place the binary in `./bin/` as expected by the
example configs):

```bash
# Native binary
make build                      # builds sqld (includes migrate), sqld-gen-go → bin/

# WASM module (wasip1)
make build-wasm                 # GOOS=wasip1 GOARCH=wasm → bin/sqld-gen-go.wasm
```

Both targets produce the same generator logic; only the transport differs (see
[WASM transport](#wasm-transport)).

---

## Configuring sqld-gen-go in sqld.yaml

Add a plugin entry under `plugins:`. The plugin name must be `go` (it is the
name the binary reports to the host via its `GetInfo` response).

```yaml
version: "1"
engine: postgresql
options:
  defaultSchema: public
schema:
  - file: schema.sql
queries:
  - dir: queries
plugins:
  - name: go
    binary: ./bin/sqld-gen-go   # path to the native binary …
    # wasm: ./bin/sqld-gen-go.wasm  # … or the WASM module (see WASM section)
    out: gen/db                 # output directory; also used to derive the package name
    options:
      package: db               # Go package name (defaults to the base of out)
      nullMode: pointer         # "pointer" (default) | "opt"
      overrides:
        # column-id key: schema.table.column → custom Go type
        "app.kitchen_sink.c_jsonb": map[string]any
        # pg-type key: bare PostgreSQL type name → custom Go type
        uuid: github.com/google/uuid.UUID
```

### Options reference

| Option | Type | Default | Description |
|---|---|---|---|
| `package` | string | base of `out` | Go package name emitted in the generated files. |
| `nullMode` | `"pointer"` \| `"opt"` | `"pointer"` | How nullable model/row fields are wrapped. `"pointer"` → `*T`; `"opt"` → `null.Val[T]` (from `github.com/aarondl/opt/null`). Query parameter types always use pointer mode regardless. |
| `overrides` | map | — | Custom Go type mappings. Keys are either a fully-qualified **column id** (`schema.table.column`) or a bare **PostgreSQL type name**. Column-id overrides take precedence over type-name overrides. |

#### Override value syntax

- `github.com/google/uuid.UUID` — contains `/`: treated as an import path. The
  import is `github.com/google/uuid`; the Go reference is `uuid.UUID`.
- `map[string]any` — no `/`: emitted verbatim with no import.
- `json.RawMessage` — special-cased: `encoding/json` is added automatically.

---

## Query annotations

Queries are written in `.sql` files. Each query begins with an annotation comment
of the form:

```sql
-- name: QueryName :command
```

### Command kinds

| Command | Go return | Description |
|---|---|---|
| `:one` | `(XRow, error)` | `QueryRow` — expects at most one row; returns the row struct. |
| `:many` | `([]XRow, error)` | `Query` + iteration — returns a slice of row structs. |
| `:exec` | `error` | `Exec` — ignores results. |
| `:execrows` | `(int64, error)` | `Exec` — returns `RowsAffected()`. |
| `:execresult` | `(pgconn.CommandTag, error)` | `Exec` — returns the raw `CommandTag`. |
| `:copyfrom` | `(int64, error)` | `CopyFrom` via pgx COPY protocol (bulk INSERT). |
| `:batchexec` | `*XBatchResults` | `SendBatch` via `pgx.Batch` (batched DML). |

> `:execlastid` is also accepted for compatibility but generates a method that
> returns `error` and adds a source note; PostgreSQL has no `LastInsertId` — use
> `RETURNING` with `:one` instead.

### Named parameters

Use `@name` instead of positional `$N` for readable, self-documenting queries.
The host resolves `@name` to a positional parameter and names the Go field after
it.

```sql
-- name: GetProfile :one
SELECT user_id, bio, address FROM app.profiles WHERE user_id = @user_id;
```

Generated:

```go
func (q *Queries) GetProfile(ctx context.Context, userID int64) (GetProfileRow, error)
```

A **single** named param is passed directly; two or more params become a
`XParams` struct.

### Optional parameters (`@name?`)

Appending `?` to a parameter name marks it optional. Optional parameters become
pointer fields in the `XParams` struct. The condition that uses an optional
parameter is included in the SQL at runtime only when the field is non-nil.

### Slice parameters (`ANY(@ids)`)

A parameter written as `ANY(@ids)` declares a slice condition. The Go field is
`[]T`; the condition is included at runtime only when the slice is non-empty.
pgx passes a slice as a single `$N` argument and PostgreSQL's `= ANY($N)`
expands it server-side.

### The `@orderby` annotation — typed dynamic ORDER BY

Add `-- @orderby col1,col2` to a query to declare a compile-time allowlist of
column names that may be used for ORDER BY. The host passes the annotation to the
plugin, which generates:

- a per-query `XOrderBy string` type with one exported constant per allowed column;
- a shared `OrderDir string` type with constants `OrderAsc` / `OrderDesc`;
- `OrderBy XOrderBy` and `OrderDir OrderDir` fields on the params struct.

The ORDER BY clause is appended at runtime only when `OrderBy` is non-empty.

A query is treated as **dynamic** (and uses the WHERE-aware runtime builder) if
it has an `@orderby` annotation, any optional parameter, or any `ANY(@name)` slice
parameter.

### Full example: static and dynamic queries

```sql
-- name: GetUser :one
SELECT id, email, status FROM app.users WHERE id = $1;

-- name: SetUserStatus :execrows
UPDATE app.users SET status = $2 WHERE id = $1;

-- name: SearchUsers :many
SELECT id, email, status FROM app.users
WHERE
      email = @email?
  AND id = ANY(@ids)
-- @orderby created_at, email
;
```

Generated (excerpt from `queries.go`):

```go
// :one — single param, no struct
func (q *Queries) GetUser(ctx context.Context, id int64) (GetUserRow, error)

// :execrows — two params → XParams struct
type SetUserStatusParams struct {
    ID     int64
    Status AppUserStatus
}
func (q *Queries) SetUserStatus(ctx context.Context, arg SetUserStatusParams) (int64, error)

// dynamic :many — optional + slice + @orderby
type SearchUsersOrderBy string
const (
    SearchUsersOrderByCreatedAt SearchUsersOrderBy = "created_at"
    SearchUsersOrderByEmail     SearchUsersOrderBy = "email"
)
type SearchUsersParams struct {
    Email    *string            // optional: non-nil → condition included
    Ids      []int64            // slice: non-empty → condition included
    OrderBy  SearchUsersOrderBy
    OrderDir OrderDir
}
type SearchUsersRow struct {
    ID     int64
    Email  string
    Status AppUserStatus
}
func (q *Queries) SearchUsers(ctx context.Context, arg SearchUsersParams) ([]SearchUsersRow, error)
```

Calling the dynamic query:

```go
email := "alice@example.com"
rows, err := q.SearchUsers(ctx, db.SearchUsersParams{
    Email:    &email,
    Ids:      []int64{1, 2},
    OrderBy:  db.SearchUsersOrderByEmail,
    OrderDir: db.OrderDesc,
})
```

---

## Generated output

Running `sqld generate` with the `go` plugin produces two files inside `out`:

### `models.go`

- **Enum types** — one `type XFoo string` per `CREATE TYPE … AS ENUM`, with one
  exported constant per label (e.g. `AppUserStatusActive AppUserStatus = "active"`).
- **Composite types** — one `struct` per `CREATE TYPE … AS (…)`, with pgx
  codec methods (`ScanIndex`, `ScanNull`, `Index`, `IsNull`) and compile-time
  interface assertions for `pgtype.CompositeIndexScanner` /
  `pgtype.CompositeIndexGetter`.
- **Domain types** — transparent; the generated Go type is the domain's base
  type (e.g. an `app.email` domain over `text` column becomes `string`).
- **Table structs** — one `type XTable struct` per table; schema prefix is
  dropped for `public` and added for all others (e.g. `app.users` →
  `AppUsers`).
- **`RegisterTypes`** — emitted whenever the schema contains enums, composites,
  custom ranges, or hstore (see [Custom types at runtime](#custom-types-at-runtime)).

### `queries.go`

- **`DBTX` interface** — `Exec`, `Query`, `QueryRow` are always present;
  `CopyFrom` is added when any `:copyfrom` query exists; `SendBatch` is added
  when any `:batchexec` query exists. `*pgxpool.Pool`, `*pgx.Conn`, and `pgx.Tx`
  all satisfy the full interface.
- **`type Queries struct`** with a private `db DBTX` field.
- **`New(db DBTX) *Queries`** constructor.
- **`WithTx(tx pgx.Tx) *Queries`** — returns a `*Queries` bound to a
  transaction.
- **Shared `OrderDir` type** (only when at least one query uses `@orderby`).
- **Per-query SQL constants, params structs, row structs, and methods.**

---

## Type mapping

The PostgreSQL → Go type mapping is driven by `pkg/gotypes`. See
[`docs/types.md`](../types.md) for the complete table. The headline rules:

| PostgreSQL category | Go type |
|---|---|
| `int2` / `smallserial` | `int16` |
| `int4` / `serial` | `int32` |
| `int8` / `bigserial` | `int64` |
| `bool` | `bool` |
| `float4` | `float32` |
| `float8` | `float64` |
| `text`, `varchar`, `bpchar`, `name`, `citext`, `char` | `string` |
| `numeric`, `money` | `string` |
| `uuid` | `string` (override with `github.com/google/uuid.UUID`) |
| `timestamptz`, `timestamp`, `date`, `time`, `timetz` | `time.Time` |
| `bytea` | `[]byte` |
| `json`, `jsonb` | `json.RawMessage` |
| `inet`, `cidr` | `string` |
| `interval` | `pgtype.Interval` |
| geometric (`point`, `line`, `lseg`, `box`, `path`, `polygon`, `circle`) | `pgtype.Point` / … |
| `bit`, `varbit` | `pgtype.Bits` |
| `macaddr`, `macaddr8` | `net.HardwareAddr` |
| `tid` | `pgtype.TID` |
| `xid`, `cid` | `pgtype.Uint32` |
| `hstore` | `pgtype.Hstore` |
| `ltree`, `lquery` | `string` |
| builtin ranges (`int4range`, `tstzrange`, …) | `pgtype.Range[pgtype.Int4]` / … |
| builtin multiranges (`int4multirange`, …) | `pgtype.Multirange[pgtype.Range[pgtype.Int4]]` / … |
| `CREATE TYPE … AS ENUM` | typed `string` alias (generated) |
| `CREATE TYPE … AS (…)` composite | generated struct |
| `CREATE DOMAIN … AS` | transparent; base type |
| `CREATE TYPE … AS RANGE` custom | `pgtype.Range[<subtypeElem>]` |
| custom multirange (auto-created) | `pgtype.Multirange[pgtype.Range[<subtypeElem>]]` |
| `T[]` array | `[]<GoElem>` |
| unknown | `any` |

### Nullability and null modes

A nullable column is wrapped unless its Go type already encodes SQL NULL
internally. Types that are **never** wrapped: slices, `map[…]`, `json.RawMessage`,
`pgtype.Hstore`, all `pgtype` struct types (`Interval`, `Point`, `Bits`, …),
`pgtype.Range[T]`, `pgtype.Multirange[T]`.

The wrapping strategy is controlled by the `nullMode` option:

| `nullMode` | Nullable scalar/enum/composite | Example |
|---|---|---|
| `pointer` (default) | `*T` | `*string`, `*AppUserStatus` |
| `opt` | `null.Val[T]` | `null.Val[string]`, `null.Val[AppUserStatus]` |

Query **parameter** types always use pointer mode regardless of `nullMode`; the
setting only affects model struct fields and query row struct fields.

`"opt"` mode is useful when you also run `sqld-gen-bob` (the ORM plugin): both
tools then emit `null.Val[T]` for nullable columns, making bob models and sqld
query rows share the same Go type for a given nullable column.

---

## Custom types at runtime

`RegisterTypes` must be wired into `pgxpool.Config.AfterConnect` (it runs once
per connection) so that enum arrays, composite types, custom ranges, and hstore
columns scan into and encode from their Go types. Without it, pgx does not know
the OIDs of user-defined types and will fail at scan time.

```go
cfg, err := pgxpool.ParseConfig("postgres://localhost/mydb")
if err != nil {
    log.Fatal(err)
}
cfg.AfterConnect = db.RegisterTypes   // wire in the generated helper
pool, err := pgxpool.NewWithConfig(ctx, cfg)
```

`RegisterTypes` performs:

1. hstore (when used) — looks up the runtime OID via `pg_type`, registers
   `pgtype.HstoreCodec` and the array codec explicitly (pgx's `Conn.LoadType`
   cannot load a non-array base type).
2. Enums — `Conn.LoadType(name)` + `RegisterType`, each followed by its array
   type (`_name`).
3. Composites — topologically sorted (a composite is registered after all
   composites it has fields of), each followed by its array type.
4. Custom ranges — registered last (subtypes are already in pgx's default map),
   each followed by its array type; if a PG 14+ multirange exists for the range,
   it is registered immediately after the range (multirange requires the range to
   be registered first).

**ltree / lquery need no registration.** Their wire format is text; pgx scans
them into `string` via the default text codec without a custom OID.

Scalar enum columns (non-array) also scan without registration — pgx returns
them as text and the generated `string`-alias type accepts that directly. The
registration requirement applies to enum **arrays** and all composite/range/hstore
types.

---

## copyfrom / batch / transactions

### `:copyfrom` — bulk INSERT via COPY protocol

```sql
-- name: BulkCreateRoles :copyfrom
INSERT INTO app.roles (name) VALUES (@name);
```

Generated:

```go
type BulkCreateRolesParams struct {
    Name string
}

func (q *Queries) BulkCreateRoles(ctx context.Context, arg []BulkCreateRolesParams) (int64, error)
```

The method calls `q.db.CopyFrom` with a `pgx.CopyFromSlice` source. It returns
the number of rows copied. The DBTX interface gains a `CopyFrom` method when any
`:copyfrom` query exists. Column types come from the catalog table (the host does
not seed copyfrom parameter types), falling back to `any` for unresolved columns.

### `:batchexec` — batched DML via pgx.Batch

```sql
-- name: BulkTouchUsers :batchexec
UPDATE app.users SET status = @status WHERE id = @id;
```

Generated:

```go
type BulkTouchUsersParams struct {
    Status AppUserStatus
    ID     int64
}

type BulkTouchUsersBatchResults struct { /* br, tot, closed */ }

func (q *Queries) BulkTouchUsers(ctx context.Context, arg []BulkTouchUsersParams) *BulkTouchUsersBatchResults
func (b *BulkTouchUsersBatchResults) Exec(f func(int, error))
func (b *BulkTouchUsersBatchResults) Close() error
```

`BulkTouchUsers` queues one statement per element into a `pgx.Batch` and calls
`SendBatch`. The returned `BulkTouchUsersBatchResults` wrapper exposes:

- `Exec(f func(int, error))` — walks each queued result, calling `f(i, err)` per
  element; closes the batch when done.
- `Close() error` — closes the batch early (marks it closed so further `Exec`
  calls report an error).

The DBTX interface gains a `SendBatch` method when any `:batchexec` query exists.

### Transactions — `WithTx`

`WithTx(tx pgx.Tx) *Queries` returns a new `*Queries` that runs all methods
against the given transaction. `pgx.Tx` satisfies the generated `DBTX` interface
(it has `Exec`, `Query`, `QueryRow`, and — when present — `CopyFrom` /
`SendBatch`).

```go
tx, err := pool.Begin(ctx)
if err != nil {
    return err
}
defer tx.Rollback(ctx)

qtx := q.WithTx(tx)
_, err = qtx.BulkCreateRoles(ctx, rows)
if err != nil {
    return err
}
return tx.Commit(ctx)
```

---

## WASM transport

`sqld-gen-go` can be compiled as a `wasip1` WASM module and referenced via the
`wasm:` key in `sqld.yaml` instead of `binary:`:

```bash
make build-wasm     # → bin/sqld-gen-go.wasm
```

```yaml
plugins:
  - name: go
    wasm: ./bin/sqld-gen-go.wasm
    out: gen/dbwasm
    options:
      package: dbwasm
      overrides:
        uuid: github.com/google/uuid.UUID
```

The plugin protocol is identical to the native binary: the sqld host writes a
single tag byte on stdin (`0` = GetInfo, `1` = Generate) followed by a
protobuf-encoded payload for Generate, and reads the protobuf-encoded response
from stdout.

See `example/sqld.wasm.yaml` for the complete working example.

---

## See also

- [`docs/types.md`](../types.md) — complete PostgreSQL → Go type mapping table.
- [`docs/cmd/sqld.md`](sqld.md) — sqld host CLI reference.
- [`docs/cmd/sqld-gen-bob.md`](sqld-gen-bob.md) — the bob ORM plugin that reuses
  the same leaf Go types emitted by sqld-gen-go.
