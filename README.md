# sqld

**A fully-open PostgreSQL toolkit**: protobuf semantic IR, typed Go code generation, and schema-diff migrations — everything works, nothing is paywalled.

sqld is an open alternative to **sqlc + Atlas**. sqlc lacks dynamic queries; Atlas paywalls `migrate diff` and lint. sqld ships all of it under MIT.

---

## Features

- **Typed Go codegen** — named queries (`-- name: X :one`) produce typed row structs and `*Queries` methods backed by pgx v5.
- **Full PostgreSQL type coverage** — scalars, `json`/`jsonb` → `json.RawMessage`, enums → typed string + consts, domains → base type, composites → structs with pgx scan/encode, arrays (including array-of-enum and array-of-composite), builtin and custom range/multirange, `hstore`, `ltree`, `interval`, geometry types, `bit`/`varbit`, and more. Override any type via `overrides` in `sqld.yaml`.
- **Dynamic queries** — `@name?` optional parameters, `ANY(@ids)` slice parameters, `-- @orderby` typed enum for runtime `ORDER BY`. The builder generates parameterized SQL with no `WHERE true`, no injection surface, and a fixed typed result row.
- **Open migrator with schema-diff generation** — `sqld-migrate generate <name>` diffs your declarative `schema.sql` against the current migration history via ephemeral Postgres (testcontainers or `--dev-url`) and writes the new migration with up + down DDL. Fully open; nothing is behind a paywall.
- **`embed.FS` auto-apply** — embed migrations in your binary and call `migrate.Migrate(ctx, pool, migrationsFS)` to apply pending migrations on startup.
- **Plugin architecture** — code generators implement a `Generator` gRPC service contract; the host invokes them over stdio (binary/command) or as WASM modules (wazero). Plugins depend only on the public proto contract and can be written in any language.
- **ORM ⊕ sqlc via bob** — `sqld-gen-bob` feeds the same IR into [stephenafamo/bob](https://github.com/stephenafamo/bob) to generate a full Go ORM (models, relationships, eager loading, typed where/loaders/joins) that **shares one canonical Go type per column** with the `sqld-gen-go` query code and runs on one `*pgxpool.Pool`. See [docs/bob.md](docs/bob.md).

---

## Install

Install the three binaries:

```sh
go install github.com/yaroher/sqld/cmd/sqld@latest
go install github.com/yaroher/sqld/cmd/sqld-gen-go@latest
go install github.com/yaroher/sqld/cmd/sqld-gen-bob@latest  # nested module — keeps bob out of the core go.mod
go install github.com/yaroher/sqld/cmd/sqld-migrate@latest
```

Or build from source into `bin/`:

```sh
make build
```

---

## Quickstart

**1. Write a schema and a query**

```sql
-- schema.sql
CREATE TYPE app.user_status AS ENUM ('active', 'inactive', 'banned');

CREATE TABLE app.users (
  id         bigserial PRIMARY KEY,
  email      text NOT NULL UNIQUE,
  status     app.user_status NOT NULL DEFAULT 'active',
  created_at timestamptz NOT NULL DEFAULT now()
);
```

```sql
-- queries/users.sql

-- name: GetUser :one
SELECT id, email, status FROM app.users WHERE id = @id;

-- name: ListActiveUsers :many
SELECT id, email FROM app.users WHERE status = 'active' ORDER BY created_at DESC;
```

**2. Configure `sqld.yaml`**

```yaml
version: "1"
engine: postgresql
schema:
  - file: schema.sql
queries:
  - dir: queries
migrations:
  - dir: migrations
plugins:
  - name: go
    binary: ./bin/sqld-gen-go
    out: gen/db
    options:
      package: db
      overrides:
        uuid: github.com/google/uuid.UUID
```

**3. Generate**

```sh
sqld generate -c sqld.yaml
```

**4. Use the generated code**

```go
q := db.New(pool)

user, err := q.GetUser(ctx, 42)
fmt.Println(user.Email, user.Status) // string, db.AppUserStatus

users, err := q.ListActiveUsers(ctx)
```

The generated `AppUserStatus` is a typed `string` with constants:

```go
const (
    AppUserStatusActive   AppUserStatus = "active"
    AppUserStatusInactive AppUserStatus = "inactive"
    AppUserStatusBanned   AppUserStatus = "banned"
)
```

---

## Dynamic queries

Annotate a query with optional parameters and a typed `ORDER BY` allowlist:

```sql
-- name: SearchUsers :many
SELECT id, email, status FROM app.users
WHERE
      email = @email?
  AND id = ANY(@ids)
-- @orderby created_at, email
;
```

`sqld generate` produces:

```go
type SearchUsersOrderBy string

const (
    SearchUsersOrderByCreatedAt SearchUsersOrderBy = "created_at"
    SearchUsersOrderByEmail     SearchUsersOrderBy = "email"
)

type SearchUsersParams struct {
    Email    *string             // nil → condition omitted
    Ids      []int64             // empty → condition omitted
    OrderBy  SearchUsersOrderBy
    OrderDir OrderDir            // OrderAsc | OrderDesc
}

func (q *Queries) SearchUsers(ctx context.Context, arg SearchUsersParams) ([]SearchUsersRow, error)
```

The builder assembles a clean `WHERE c1 AND c2` only when conditions are present, renumbers `$N` in append order, and appends `ORDER BY <enum> <dir>` from the allowlist — no string interpolation, no injection surface.

---

## Migrations

**Migration file format** (`migrations/<version>_<name>.sql`):

```sql
-- sqld:up
CREATE TABLE orders (id bigint PRIMARY KEY, user_id bigint NOT NULL);

-- sqld:down
DROP TABLE orders;
```

**CLI** (DSN from `--db` or `$DATABASE_URL`):

```sh
sqld-migrate up       [-c sqld.yaml] [--db DSN] [--to VERSION]
sqld-migrate down     [-c sqld.yaml] [--db DSN] [--steps N | --to VERSION]
sqld-migrate status   [-c sqld.yaml] [--db DSN]
sqld-migrate generate <name> [-c sqld.yaml] [--dev-url DSN]
sqld-migrate hash     [-c sqld.yaml]
sqld-migrate validate [-c sqld.yaml]
```

**Generate a migration from a schema diff:**

```sh
sqld-migrate generate add_orders --dev-url postgres://localhost:5432/scratch?sslmode=disable
```

Without `--dev-url`, sqld-migrate starts an ephemeral Postgres via testcontainers (requires Docker). The diff covers schemas, types, sequences, tables, columns, constraints, indexes, views, materialized views, functions, procedures, and triggers — in dependency order with a reverse `down`.

**Embed and auto-apply in your service:**

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
    pool, _ := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
    if err := migrate.Migrate(ctx, pool, migrationsFS); err != nil {
        log.Fatal(err)
    }
}
```

`migrate.Migrate` is idempotent — already-applied migrations are skipped.

**Library use:**

```go
import "github.com/yaroher/sqld/pkg/migrate"

migs, _ := migrate.Load("migrations")
m := migrate.New(pool, migs)
if err := m.Up(ctx); err != nil { /* ... */ }
st, _ := m.Status(ctx) // st.Applied, st.Pending, st.Drift
```

---

## Plugins

sqld's code generation is plugin-driven. A plugin implements the `Generator` gRPC service (defined in `proto/sqld/v1/plugin/plugin.proto`):

```
service Generator {
    rpc GetInfo(GetInfoRequest)     returns (GetInfoResponse);
    rpc Generate(GenerateRequest)   returns (GenerateResponse);
}
```

The host feeds the plugin a `Catalog` (full schema IR) plus typed `Query` objects (parameters + result columns already inferred). Two transports:

- **Binary / command** — the plugin is a native executable; the host communicates via stdio-framed protobuf.
- **WASM** — the plugin is a WASI command module (`.wasm`); the host runs it via [wazero](https://wazero.io) with the same stdio-framed protobuf protocol. No external runtime is required — wazero is embedded.

Both transports use an identical wire format: `stdin = [1-byte method tag] [proto-encoded request]`, `stdout = [proto-encoded response]`. A plugin compiled for one transport works on the other without modification.

Configure a plugin in `sqld.yaml` with **exactly one** of `binary`, `command`,
or `wasm` (plus `out`):

```yaml
plugins:
  - name: my-gen
    command: sqld-gen-my    # resolved via $PATH (e.g. after `go install …@latest`)
    # binary: ./bin/my-gen  # explicit path instead (see resolution below)
    # wasm:   ./bin/my-gen.wasm  # WASM/wazero transport
    out: gen/
    options:                # opaque bytes, decoded by the plugin
      package: mypackage
```

### How the plugin is located

| field | resolution |
|-------|------------|
| `command` | a bare program name looked up on **`$PATH`** (Go's `os/exec` `LookPath`). Use this for plugins installed via `go install` (e.g. `sqld-gen-go`, `sqld-gen-bob`). |
| `binary` | a **filesystem path**, used as-is — absolute, or **relative to the current working directory** of the `sqld` process (NOT the `sqld.yaml` location). `./bin/my-gen` only resolves when you run `sqld` from that directory. |
| `wasm` | a filesystem path to a `.wasm` module (same cwd-relative rule), run via wazero. |

`command` is the portable choice (install once, no path juggling); `binary` is
handy for a locally-built plugin during development. The example configs use
`binary: ./bin/…` because they run from the repo root after `make build`.

The built-in `sqld-gen-go` is itself a plugin and dogfoods this contract. It can be built as a native binary (`make build`) **or** as a WASM module (`make build-wasm`):

```sh
make build-wasm          # → bin/sqld-gen-go.wasm  (GOOS=wasip1 GOARCH=wasm)
make example-wasm        # generate example/gen/dbwasm/ via the WASM transport
```

`example/sqld.wasm.yaml` is an example config that drives `sqld-gen-go.wasm` through wazero and writes the output to `example/gen/dbwasm/` (package `dbwasm`) — demonstrating the language-agnostic plugin contract: any language that can read/write stdio-framed protobuf and compile to WASI can be used as a sqld plugin.

---

## Documentation

- [`docs/adr.md`](docs/adr.md) — architecture decision records
- [`docs/types.md`](docs/types.md) — full PostgreSQL → Go type mapping table
- [`docs/migrations.md`](docs/migrations.md) — migration system reference

---

## Development

Build all binaries:

```sh
make build         # → bin/sqld, bin/sqld-gen-go, bin/sqld-migrate
```

Run the full example (codegen + IR dump + build check):

```sh
make example
```

Regenerate Go from proto sources (requires `easyp`):

```sh
make protocols
```

Run tests:

```sh
go test ./...
```

Integration tests (migration generation, `sqld-migrate generate`) spin up Postgres via testcontainers and require Docker.

---

## License

MIT — see [LICENSE](LICENSE).
