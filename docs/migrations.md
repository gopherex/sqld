# Migrations (`sqld migrate`)

A fully-open PostgreSQL migration tool: apply/roll back/inspect migrations, and
**generate** a new migration from the diff between your declarative `schema.sql`
and the current migration history. Everything works — nothing is paywalled.

Two faces:

- **`pkg/migrate`** — import it to run migrations from Go.
- **`sqld migrate`** — the CLI (a subcommand of the `sqld` binary).

## Migration files

One file per version in the migrations directory, named `<version>_<name>.sql`
(`version` = a zero-padded sequence or a UTC timestamp). Up and down sections are
delimited by markers:

```sql
-- sqld:up
CREATE TABLE users (id bigint PRIMARY KEY, email text NOT NULL);

-- sqld:down
DROP TABLE users;
```

No markers → the whole file is the `up`. The loader records a SHA-256 checksum of
each file; an applied migration whose file later changes is reported as **drift**.

## Config

`sqld migrate` reads `sqld.yaml` (the same config as the rest of sqld):

```yaml
schema:                       # declarative source of truth (for `generate`)
  - file: schema.sql
migrations:                   # the versioned history this tool manages
  - dir: migrations
```

## CLI

```
sqld migrate up      [-c sqld.yaml] [--db DSN] [--to VERSION]   # apply pending
sqld migrate down    [-c sqld.yaml] [--db DSN] [--steps N | --to VERSION]
sqld migrate status  [-c sqld.yaml] [--db DSN]                  # applied / pending / drift
sqld migrate apply   [-c sqld.yaml] [--db DSN]                  # alias of up
sqld migrate generate <name> [-c sqld.yaml] [--dev-url DSN]     # diff schema.sql -> new migration
sqld migrate hash    [-c sqld.yaml]                             # print versions + checksums
sqld migrate validate[-c sqld.yaml]                             # parse-check every migration
```

DSN comes from `--db` or `$DATABASE_URL`. Each migration applies in its own
transaction under a session advisory lock; bookkeeping lives in a
`sqld_migrations` table.

## Generating a migration

`generate` produces a versioned migration from the difference between your
desired schema (`schema.sql`) and the state the existing migrations produce:

```
desired := introspect( apply schema.sql to a fresh Postgres )
current := introspect( apply all existing migrations to a fresh Postgres )
plan    := diff(current, desired)            # ordered DDL, dependency-aware
write migrations/<timestamp>_<name>.sql      # -- sqld:up <plan> / -- sqld:down <inverse>
```

To realize and normalize the schemas, `generate` needs a throwaway Postgres. By
default it starts one automatically via **testcontainers** (requires Docker).
If you already have a scratch database, pass `--dev-url`:

```
sqld migrate generate add_orders --dev-url postgres://localhost:5432/scratch?sslmode=disable
```

The diff covers schemas, types (enum/domain/composite/range), sequences, tables,
columns, constraints (PK/FK/unique/check/exclusion), indexes, views, materialized
views, functions, procedures, and triggers — in dependency order, with a reverse
`down`. Letting Postgres realize the schema normalizes definitions before they
are compared.

Index introspection parses PostgreSQL's `CREATE INDEX` definition into
individual keys. Mixed indexes such as `(tenant_id, lower(email))` retain the
boundary between the column and expression keys, including nested expressions.
Per-key `DESC` and `NULLS FIRST`/`NULLS LAST` survive migration generation, along
with `INCLUDE` columns and partial-index predicates. Changing index ordering is
therefore detected by the schema diff.

If there are no changes, `generate` prints `no changes` and writes nothing.

## Library use

```go
migs, _ := migrate.Load("migrations")   // from an OS directory
pool, _ := pgxpool.New(ctx, dsn)
m := migrate.New(pool, migs)
if err := m.Up(ctx); err != nil { /* ... */ }
st, _ := m.Status(ctx)   // st.Applied, st.Pending, st.Drift
```

### Embedded migrations + auto-apply on startup

Embed the migration files in the binary and apply pending ones when the service
boots — no separate migrate step, no files shipped alongside:

```go
//go:embed migrations/*.sql
var migrationsFS embed.FS

func main() {
	pool, _ := pgxpool.New(ctx, dsn)
	// loads embedded migrations (merged across the given filesystems,
	// sorted by version) and applies all pending ones.
	if err := migrate.Migrate(ctx, pool, migrationsFS); err != nil {
		log.Fatal(err)
	}
}
```

`migrate.Migrate(ctx, db, sources ...fs.FS)` accepts any `fs.FS` (so `embed.FS`,
`os.DirFS`, or a test `fstest.MapFS`), walks each recursively for `*.sql`, merges
them (duplicate versions across sources are an error), and runs `Up`. It is
idempotent — already-applied migrations are skipped. Use `migrate.LoadFS(fsys)`
if you want the `[]Migration` without applying.
