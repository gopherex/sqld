# sqld-migrate + pkg/migrate — Design

Date: 2026-05-26
Status: approved

## Goal & positioning

A fully open, full-featured PostgreSQL migration tool. Everything works, nothing
is paywalled (contrast: Atlas gates `migrate diff`, lint rules, etc. behind a
paid tier). Two faces:

- **`pkg/migrate`** — importable Go library to apply/roll back/inspect migrations
  from code.
- **`cmd/sqld-migrate`** — CLI wrapping it, plus the headline feature:
  **generate a migration from the schema diff** (declarative `schema.sql` →
  versioned migration), realized through an ephemeral dev Postgres.

`schema.sql` (ADR-0015, restored) is the declarative source of truth; the
`migrations/` directory is the versioned history `sqld-migrate` manages.

## Decisions

- **Generate strategy: dev database.** To diff, both the *desired* state
  (`schema.sql`) and the *current* state (existing migrations applied) are
  realized in an ephemeral Postgres, then introspected into the IR `Catalog`;
  our diff engine compares the two catalogs and emits DDL. Postgres normalizes
  everything (serial→sequence+default, type aliases, defaults, etc.), so the
  diff covers everything PG supports — no gaps from our parser. The dev DB is
  spun via **testcontainers-go**; `--dev-url <dsn>` uses an existing one (no
  Docker needed).
- **up + down.** `generate` emits both: `up` = diff(current→desired),
  `down` = the inverse plan. Enables rollback.
- **Build it all in one plan** (migrator + introspection + diff engine + devdb +
  CLI), phased internally.

## Components

```
pkg/migrate/            Migrator (apply/rollback/status/integrity) + Migration model + source loader
internal/introspect/    *irv1.Catalog from a live Postgres (pg_catalog/information_schema)
internal/diff/          Diff(from,to *irv1.Catalog) (*Plan,error); Plan.UpSQL()/DownSQL()
internal/devdb/         ephemeral Postgres via testcontainers; --dev-url override
cmd/sqld-migrate/       CLI: up | down | status | apply | generate | hash | validate
```

### Migration file format

One file per version: `migrations/<version>_<name>.sql`, version = zero-padded
sequence or UTC timestamp (`20260526120000`). Sections delimited by markers:

```sql
-- sqld:up
CREATE TABLE ... ;

-- sqld:down
DROP TABLE ... ;
```

`source.Resolve` is extended to parse these markers into `up_sql`/`down_sql`
(today it treats a migration file as up-only). The loader records each
migration's SHA-256 checksum.

### pkg/migrate — Migrator

- Backed by a `*pgxpool.Pool` / `pgx.Conn` (a `DBTX`-style interface).
- Bookkeeping table `sqld_migrations(version text primary key, name text,
  checksum text, applied_at timestamptz default now())` (schema configurable).
- Operations:
  - `Up(ctx)` — apply all pending, in version order, each in a transaction,
    record on success.
  - `To(ctx, version)` — migrate up or down to a target.
  - `Down(ctx, n)` / `Rollback` — revert the last n applied (run their `down`).
  - `Status(ctx)` — applied vs pending, with checksum-drift detection
    (an applied migration whose file checksum changed → error/warn).
  - `Pending`/`Applied` accessors.
- Concurrency-safe via a session advisory lock during apply.
- Errors are explicit; a failed migration aborts (its tx rolls back) and is not
  recorded.

### internal/introspect

`Introspect(ctx, conn, schemas []string) (*irv1.Catalog, error)` builds the IR
from `pg_catalog`:
- namespaces → `Schema`
- `pg_class`/`pg_attribute` → tables + columns (type via `pg_type`, nullability,
  defaults, identity, generated)
- `pg_constraint` → PK/FK (with actions)/unique/check/exclusion
- `pg_index` → indexes (method, partial, expr, include, opclass, order)
- `pg_type`/`pg_enum` → enums; domains; composites; ranges (+ multirange)
- `pg_proc` → functions/procedures; `pg_trigger` → triggers; views/matviews;
  sequences; comments (`pg_description`).
Reuses the `irv1` messages so the diff engine has one representation regardless
of source (introspection or our parser).

### internal/diff — the engine

`Diff(from, to *irv1.Catalog) (*Plan, error)`. `Plan` is an ordered list of
typed `Change`s; `Plan.UpSQL()` renders forward DDL, `Plan.DownSQL()` the
inverse. Coverage (the "support everything"):
- **Schemas**: create/drop.
- **Types**: enum (create/drop, `ADD VALUE`), domain (create/drop, constraint
  changes), composite (create/drop, add/drop/alter attribute), range/multirange.
- **Sequences**: create/drop/alter.
- **Tables**: create/drop; **columns** add/drop, type change (`ALTER COLUMN
  TYPE ... USING`), set/drop NOT NULL, set/drop default, identity, generated;
  **constraints** add/drop (PK/FK/unique/check/exclusion); **indexes**
  create/drop.
- **Views / materialized views**: create/replace/drop.
- **Functions / procedures / triggers**: create-or-replace/drop.
- **Comments**, and (later) ownership/grants/RLS/partitioning.
- **Ordering**: dependency-aware — create schemas → types → sequences → tables →
  columns → constraints (FKs last) → indexes → views → functions → triggers;
  **drops in reverse**. Down plan = structural inverse (a create becomes a drop,
  an add-column a drop-column, etc.); where an inverse is lossy (e.g. drop
  column down = re-add without data) emit it with a warning comment.

The diff engine works on two `*irv1.Catalog`s and is **fully unit-testable
without a database** (golden tests: two catalogs → expected SQL).

### internal/devdb

`Start(ctx) (*DevDB, error)` → ephemeral Postgres (testcontainers-go +
`modules/postgres`), `DevDB.URL()`, `DevDB.Apply(ctx, sql)`, `DevDB.Close()`.
`Open(ctx, devURL string)` uses an existing DSN when `--dev-url` is set (skips
Docker). Used only by `generate`.

### cmd/sqld-migrate — CLI

- `sqld-migrate up [--db DSN] [--to V]`
- `sqld-migrate down [--db DSN] [--to V | --steps N]`
- `sqld-migrate status [--db DSN]`
- `sqld-migrate generate <name> [-c sqld.yaml] [--dev-url DSN]` — the diff:
  realize `schema.sql` and the applied-migrations state in the dev DB,
  introspect both, `Diff`, write `migrations/<version>_<name>.sql` (up + down).
  Exit non-zero with "no changes" if the diff is empty.
- `sqld-migrate hash` (recompute/verify checksums), `validate` (parse + lint
  migrations, detect drift).
DSN from `--db`/`$DATABASE_URL`; config (schema + migrations dir) from
`sqld.yaml`.

## Data flow — generate

```
cfg        := config.Load(sqld.yaml)
dev        := devdb.Open(ctx, devURL)         // testcontainers or --dev-url
dev.Apply(schema.sql);  desired := introspect(dev)        // fresh dev DB
dev2.Apply(all migrations up); current := introspect(dev2) // separate fresh dev DB
plan       := diff.Diff(current, desired)
if plan.Empty() { exit "no changes" }
write migrations/<ts>_<name>.sql  { -- sqld:up\n plan.UpSQL \n-- sqld:down\n plan.DownSQL }
```

## Testing

- **diff**: pure unit/golden — hand-built or parsed `*irv1.Catalog` pairs →
  expected up/down SQL. No DB. This is where correctness is pinned.
- **introspect / migrator / devdb / generate**: integration tests gated on
  Docker (testcontainers); `t.Skip` when Docker is unavailable so `go test`
  stays green in constrained environments.
- **CLI**: `run(args, stdout, stderr) int` testable; generate end-to-end behind
  the Docker gate.

## New dependencies

`github.com/testcontainers/testcontainers-go` + `.../modules/postgres` (dev DB).
Introspection/migrator reuse pgx v5 (already present).

## Risks

- **Diff completeness** — the surface is large; implement core (schemas, types,
  tables/columns/constraints/indexes, sequences) first, then views/functions/
  triggers, then ownership/RLS/partitioning. The engine is structured so object
  kinds are added incrementally.
- **Introspection fidelity** — must match what the diff expects; the dev-DB
  round-trip (apply → introspect) is itself a test that schema.sql ⇄ catalog is
  stable.
- **Docker in CI** — integration tests skip without Docker; diff (the IP) is
  Docker-free.
- **Lossy down** (dropped data on reverse) — emitted with explicit warnings; not
  auto-run destructively without confirmation.

## Out of scope (now)

Multi-dialect (PG only), online/zero-downtime orchestration, data migrations
(DML transforms beyond what the user writes), grants/RLS/partitioning diff
(later phase).
