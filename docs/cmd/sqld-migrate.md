# sqld-migrate — CLI Reference

`sqld-migrate` is sqld's fully-open PostgreSQL migration tool. It applies,
reverts, and inspects versioned SQL migrations, lints them for destructive or
risky changes, and — uniquely — **generates** a new migration automatically by
diffing your declarative `schema.sql` against the state produced by the existing
migration history. All features are free and open; nothing is paywalled.

Under the hood every operation delegates to the importable
[`pkg/migrate`](../../pkg/migrate) library, which handles bookkeeping,
advisory locking, and drift detection. The `generate` subcommand additionally
uses `pkg/devdb`, `internal/diff`, and `internal/introspect` to realise both
the desired and current database states in ephemeral Postgres instances and
compute a dependency-ordered DDL plan.

For the migration file format, the diff/generate strategy in depth, and the
full `pkg/migrate` API, see [docs/migrations.md](../migrations.md).

---

## Install

```bash
# from any module (no local clone required)
go install github.com/yaroher/sqld/cmd/sqld-migrate@latest

# or build from the repository
make build
```

---

## Connection and configuration

**Database DSN** — every subcommand that connects to a live database reads the
DSN from the `--db` flag. When `--db` is absent it falls back to the
`$DATABASE_URL` environment variable. If neither is set the command exits with
code 2 and prints a usage hint.

```bash
# flag
sqld-migrate status --db "postgres://user:pass@localhost:5432/mydb?sslmode=disable"

# environment
export DATABASE_URL="postgres://user:pass@localhost:5432/mydb?sslmode=disable"
sqld-migrate status
```

**Config file** — every subcommand accepts `-c <path>` (default `sqld.yaml`) to
locate the project configuration. The relevant sections are:

```yaml
schema:                   # declarative source of truth (used by `generate`)
  - file: schema.sql

migrations:               # the versioned history this tool manages
  - dir: migrations       # default: "migrations"
```

`options.defaultSchema` and `options.searchPath` are also read by `generate` to
decide which schemas to introspect; when only the implicit `public` is present
all non-system schemas are introspected.

---

## Commands

### `up` / `apply`

```
sqld-migrate up    [-c FILE] [--db DSN] [--to VERSION]
sqld-migrate apply [-c FILE] [--db DSN] [--to VERSION]
```

`apply` is an alias for `up`; both are identical in behaviour.

Loads migrations from the configured directory, connects to the database, and
applies every pending migration in ascending version order. Each migration runs
in its own transaction. A session-level advisory lock serialises concurrent
runs against the same database.

| Flag | Default | Description |
|------|---------|-------------|
| `-c` | `sqld.yaml` | Config file path |
| `--db` | `$DATABASE_URL` | Target database DSN |
| `--to` | _(all pending)_ | Apply up to and including this version, then stop |

**Exit codes:** `0` success · `1` migration or connection error · `2` bad usage
/ missing DSN.

```bash
# apply all pending
sqld-migrate up

# migrate to a specific version
sqld-migrate up --to 20240601120000000
```

---

### `down`

```
sqld-migrate down [-c FILE] [--db DSN] [--steps N] [--to VERSION]
```

Reverts applied migrations in descending version order. `--steps` and `--to`
are mutually exclusive: supply one or the other.

| Flag | Default | Description |
|------|---------|-------------|
| `-c` | `sqld.yaml` | Config file path |
| `--db` | `$DATABASE_URL` | Target database DSN |
| `--steps` | `1` | Number of migrations to revert |
| `--to` | _(not set)_ | Revert down to (and including) this version |

A migration with no `-- sqld:down` section cannot be reverted; the command
exits with code 1.

```bash
# revert the most recent migration
sqld-migrate down

# revert the last three migrations
sqld-migrate down --steps 3

# revert everything above a known-good version
sqld-migrate down --to 20240101000000000
```

---

### `status`

```
sqld-migrate status [-c FILE] [--db DSN]
```

Prints three groups: applied migrations (with UTC timestamp), pending
migrations, and **drift** — versions whose file checksum has changed since they
were applied.

| Flag | Default | Description |
|------|---------|-------------|
| `-c` | `sqld.yaml` | Config file path |
| `--db` | `$DATABASE_URL` | Target database DSN |

Sample output:

```
Applied (2):
  20240101000000000_init           2024-01-01T00:00:00Z
  20240601120000000_add_users      2024-06-01T12:00:00Z
Pending (1):
  20240701090000000_add_orders
Drift (0):
```

---

### `generate <name>`

```
sqld-migrate generate <name> [-c FILE] [--dev-url DSN]
```

Generates a new migration file by comparing what `schema.sql` declares to what
the existing migrations produce. Both states are realised in throwaway Postgres
instances, introspected into an internal IR, and diffed to produce a
dependency-ordered DDL plan.

```
desired := introspect( apply schema.sql  → fresh Postgres )
current := introspect( apply migrations  → fresh Postgres )
plan    := diff(current, desired)
write   migrations/<timestamp>_<name>.sql
```

The output file follows the standard format with `-- sqld:up` and
`-- sqld:down` sections. The version stamp is a millisecond-resolution UTC
timestamp (`20060102150405123`) so two rapid generates never collide.

If the diff is empty the command prints `no changes` and exits with code `0`
without writing any file.

| Flag | Default | Description |
|------|---------|-------------|
| `-c` | `sqld.yaml` | Config file path |
| `--dev-url` | _(not set)_ | DSN of an existing scratch database to use as the dev DB |

**Dev database — two modes:**

- **No `--dev-url` (default):** `generate` starts an ephemeral
  `postgres:16-alpine` container via [testcontainers](https://testcontainers.com/).
  Docker must be running. Each of the two catalogs (desired / current) gets its
  own fresh container, so they never share schema state.

- **With `--dev-url`:** the specified database is reused for both catalogs.
  Between the two introspections `generate` drops all non-system schemas and
  recreates `public` (a full reset) to provide the same isolation guarantee
  without spawning containers.

```bash
# auto testcontainers (Docker required)
sqld-migrate generate add_orders

# explicit dev database
sqld-migrate generate add_orders \
  --dev-url "postgres://localhost:5432/scratch?sslmode=disable"
```

For a detailed explanation of the diff coverage (schemas, enums, domains,
tables, constraints, indexes, views, functions, triggers, …) see
[docs/migrations.md — Generating a migration](../migrations.md#generating-a-migration).

---

### `hash`

```
sqld-migrate hash [-c FILE]
```

Prints each migration's version and SHA-256 checksum, one per line. No database
connection is required. Useful for auditing whether migration files have changed
since they were committed.

| Flag | Default | Description |
|------|---------|-------------|
| `-c` | `sqld.yaml` | Config file path |

```bash
sqld-migrate hash
# 20240101000000000  a3f2...
# 20240601120000000  7c91...
```

---

### `validate`

```
sqld-migrate validate [-c FILE]
```

Parses the `up` SQL of every migration using the same PostgreSQL parser that
the rest of sqld uses. Reports any migration whose SQL cannot be parsed and
exits with code `1` if any fail. Succeeding migrations print `ok: <version>_<name>`.
No database connection is required.

| Flag | Default | Description |
|------|---------|-------------|
| `-c` | `sqld.yaml` | Config file path |

```bash
sqld-migrate validate
# ok: 20240101000000000_init
# ok: 20240601120000000_add_users
```

---

### `lint`

```
sqld-migrate lint [-c FILE] [--strict]
```

Analyses each migration's `up` SQL for destructive or risky patterns using a
database-free parse-tree walker. Findings are grouped by version and printed
with their rule name and severity. No database connection is required.

| Flag | Default | Description |
|------|---------|-------------|
| `-c` | `sqld.yaml` | Config file path |
| `--strict` | `false` | Treat `warning`-severity findings as failures |

**Exit codes:**

- `0` — no findings, or only `warning`-severity findings and `--strict` is not
  set.
- `1` — at least one `error`-severity finding, **or** at least one
  `warning`-severity finding with `--strict`.
- `2` — bad usage / config error.

```bash
# advisory check only
sqld-migrate lint

# fail CI on any finding
sqld-migrate lint --strict
```

Sample output:

```
20240701090000000 [error] destructive-drop: drops table "legacy_events": destroys the object and its data
20240701090000000 [warning] index-not-concurrent: creates index "idx_orders_user_id" on orders without CONCURRENTLY: locks the table against writes
lint: 1 error(s), 1 warning(s)
```

---

## Migration file format

Files live in the migrations directory (default `migrations/`) and are named:

```
<version>_<name>.sql
```

`version` is any lexicographically sortable token — typically a UTC timestamp
at second or millisecond resolution (e.g. `20240601120000000`) or a
zero-padded integer sequence. The file body uses two section markers:

```sql
-- sqld:up
CREATE TABLE orders (
    id   bigint PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users(id)
);

-- sqld:down
DROP TABLE orders;
```

Text before any marker is treated as the `up` section. Marker matching is
case-insensitive and trims surrounding whitespace. The checksum recorded in
`sqld_migrations` is SHA-256 of the raw file bytes; editing an applied file
causes it to appear as **drift** in `status`.

Full format reference: [docs/migrations.md — Migration files](../migrations.md#migration-files).

---

## Library use (`pkg/migrate`)

Import `pkg/migrate` to run migrations from Go — for example as part of service
startup — without shipping migration files separately from the binary:

```go
import (
    "context"
    "embed"

    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/yaroher/sqld/pkg/migrate"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
    pool, _ := pgxpool.New(ctx, dsn)

    // Load, merge, and apply all pending migrations.
    if err := migrate.Migrate(ctx, pool, migrationsFS); err != nil {
        log.Fatal(err)
    }
}
```

For fine-grained control use `migrate.New` directly:

```go
migs, _ := migrate.Load("migrations")   // or migrate.LoadFS(fsys)
m := migrate.New(pool, migs)

if err := m.Up(ctx); err != nil { /* ... */ }

st, _ := m.Status(ctx)
// st.Applied []AppliedMigration — rows in sqld_migrations
// st.Pending []Migration        — not yet applied
// st.Drift   []string           — versions with changed checksums
```

`migrate.Migrate` is idempotent — already-applied migrations are skipped.
Duplicate versions across merged `fs.FS` sources are rejected with an error.

Full API reference and the `WithTable` option: [docs/migrations.md — Library use](../migrations.md#library-use).

---

## Lint rules

All rules are applied against each migration's `-- sqld:up` SQL by a
database-free parse-tree walker. Rule IDs are stable strings suitable for
future allow-listing.

| Rule ID | Severity | What it flags |
|---------|----------|---------------|
| `parse-error` | error | Up SQL that cannot be parsed by the PostgreSQL parser; remaining rules are skipped for that migration |
| `destructive-drop` | error | `DROP TABLE`, `DROP SCHEMA`, `DROP SEQUENCE`, `DROP TYPE`, `DROP VIEW`, `DROP MATERIALIZED VIEW`; `ALTER TABLE … DROP COLUMN`; `ALTER TABLE … DROP CONSTRAINT` |
| `destructive-truncate` | error | `TRUNCATE` — removes all rows irreversibly |
| `column-type-change` | warning | `ALTER TABLE … ALTER COLUMN … TYPE` — may rewrite the table and lose data |
| `not-null-no-default` | error | `ALTER TABLE … ADD COLUMN … NOT NULL` without a `DEFAULT`, `GENERATED`, or `IDENTITY` clause — fails immediately on a non-empty table |
| `index-not-concurrent` | warning | `CREATE INDEX` without `CONCURRENTLY` — takes a write lock for the full index build; use `CREATE INDEX CONCURRENTLY` instead (note: cannot run inside a transaction block) |
| `missing-down` | warning | Migration has an empty `-- sqld:down` section — no rollback path |

---

## See also

- [docs/migrations.md](../migrations.md) — migration file format, generate
  strategy in depth, full `pkg/migrate` API, embedded migrations pattern.
- [docs/cmd/sqld.md](sqld.md) — the main `sqld` code-generation CLI.
