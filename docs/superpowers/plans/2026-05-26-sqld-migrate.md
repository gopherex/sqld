# sqld-migrate + pkg/migrate Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** A fully-open PostgreSQL migration tool — apply/rollback/status from a Go library, and generate versioned migrations from the `schema.sql` ⇄ migrations diff via an ephemeral dev Postgres.

**Architecture:** `pkg/migrate` runs migrations against a DB + tracks them. `generate` realizes desired (`schema.sql`) and current (applied migrations) schemas in an ephemeral Postgres (testcontainers or `--dev-url`), introspects both into the IR `Catalog`, and a pure-IR diff engine emits up/down DDL. Diff is DB-free and golden-tested; introspection/migrator/devdb integration tests are Docker-gated (skip without Docker).

**Tech Stack:** Go 1.25, pgx v5, `github.com/testcontainers/testcontainers-go` + `modules/postgres`, the `irv1` IR, `internal/parse` (libpg_query).

Spec: `docs/superpowers/specs/2026-05-26-sqld-migrate-design.md`.

## File structure

```
internal/source/source.go      MOD: parse `-- sqld:up` / `-- sqld:down` markers → up/down
pkg/migrate/migrate.go          Migrator, Migration, sqld_migrations bookkeeping
pkg/migrate/source.go           load migrations dir → []Migration (version, up, down, checksum)
internal/devdb/devdb.go         ephemeral Postgres (testcontainers) + Open(devURL)
internal/introspect/introspect.go   *irv1.Catalog from a live Postgres
internal/diff/change.go         Change types + Plan (UpSQL/DownSQL)
internal/diff/diff.go           Diff(from,to *irv1.Catalog) (*Plan,error) + ordering
internal/diff/sql.go            render each Change → DDL
cmd/sqld-migrate/main.go        CLI: up|down|status|apply|generate|hash|validate
```

---

## Phase 1 — migration file format

### Task 1: parse up/down markers in source
**Files:** Modify `internal/source/source.go`; Test `internal/source/source_test.go`.

A migration file has `-- sqld:up` and optional `-- sqld:down` sections. `source.Unit` already has `Kind`/`Version`. Add fields `UpSQL`/`DownSQL` is overkill — instead, for migration units, populate `SQL` with the up section and add a `DownSQL string` field to `Unit`. Add a helper `splitMigration(sql string) (up, down string)` that splits on the markers (case-insensitive, whole-line). If no markers, the whole file is `up`.

- [ ] **Step 1: failing test**
```go
func TestSplitMigration(t *testing.T) {
	up, down := splitMigration("-- sqld:up\nCREATE TABLE a();\n-- sqld:down\nDROP TABLE a;\n")
	if !strings.Contains(up, "CREATE TABLE a") || strings.Contains(up, "DROP TABLE") {
		t.Fatalf("up=%q", up)
	}
	if !strings.Contains(down, "DROP TABLE a") {
		t.Fatalf("down=%q", down)
	}
	up2, down2 := splitMigration("CREATE TABLE b();")
	if !strings.Contains(up2, "CREATE TABLE b") || down2 != "" {
		t.Fatalf("no-marker: up=%q down=%q", up2, down2)
	}
}
```
- [ ] **Step 2:** `go test ./internal/source/ -run TestSplitMigration -v` → FAIL.
- [ ] **Step 3:** implement `splitMigration`; in `resolveMigration`, set `Unit.SQL = up`, `Unit.DownSQL = down` (add `DownSQL string` to `Unit`).
- [ ] **Step 4:** test PASS; `go build ./...`.
- [ ] **Step 5:** commit `feat(source): parse -- sqld:up/down migration markers`.

---

## Phase 2 — pkg/migrate (the migrator)

### Task 2: Migration model + dir loader
**Files:** Create `pkg/migrate/source.go`; Test `pkg/migrate/source_test.go`.

```go
package migrate

type Migration struct {
	Version  string
	Name     string
	UpSQL    string
	DownSQL  string
	Checksum string // sha256 hex of the file bytes
}

// Load reads *.sql migrations from dir, sorted by version (filename up to first '_').
func Load(dir string) ([]Migration, error)
```
- [ ] **Step 1:** failing test — write two temp files `0001_a.sql`/`0002_b.sql` (with up/down markers) → `Load` returns 2 sorted, version `0001`/`0002`, name `a`/`b`, non-empty Checksum, up/down split.
- [ ] **Step 2:** run → FAIL.
- [ ] **Step 3:** implement: read dir, filter `*.sql`, parse `<version>_<name>.sql`, split via the same marker logic (reuse a small local splitter or `source`'s — duplicate a tiny `splitMigration` here to keep pkg/migrate dependency-light), sha256 the file bytes, sort by version string.
- [ ] **Step 4:** PASS; build.
- [ ] **Step 5:** commit `feat(migrate): load migrations directory`.

### Task 3: Migrator — schema table + Applied/Pending/Status
**Files:** Create `pkg/migrate/migrate.go`; Test `pkg/migrate/migrate_test.go` (Docker-gated).

```go
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Migrator struct {
	db          DBTX
	migrations  []Migration
	table       string // default "sqld_migrations"
}

func New(db DBTX, migrations []Migration, opts ...Option) *Migrator
func (m *Migrator) ensureTable(ctx context.Context) error // CREATE TABLE IF NOT EXISTS sqld_migrations(...)
func (m *Migrator) Applied(ctx context.Context) ([]AppliedMigration, error) // version, checksum, applied_at
func (m *Migrator) Pending(ctx context.Context) ([]Migration, error)
type Status struct { Applied []AppliedMigration; Pending []Migration; Drift []string }
func (m *Migrator) Status(ctx context.Context) (Status, error) // Drift = applied whose checksum != file
```
- [ ] **Step 1:** failing integration test guarded by Docker:
```go
func newTestDB(t *testing.T) (DBTX, func()) {
	t.Helper()
	ctx := context.Background()
	dev, err := devdb.Start(ctx)   // depends on Task 5; if Task 5 not done yet, use a raw testcontainers pg here
	if err != nil { t.Skipf("docker unavailable: %v", err) }
	pool, err := pgxpool.New(ctx, dev.URL())
	if err != nil { t.Fatal(err) }
	return pool, func() { pool.Close(); dev.Close() }
}

func TestStatusPending(t *testing.T) {
	db, done := newTestDB(t); defer done()
	migs := []Migration{{Version: "0001", Name: "a", UpSQL: "CREATE TABLE a(id int);", DownSQL: "DROP TABLE a;", Checksum: "x"}}
	m := New(db, migs)
	st, err := m.Status(context.Background())
	if err != nil { t.Fatal(err) }
	if len(st.Pending) != 1 || len(st.Applied) != 0 { t.Fatalf("status=%+v", st) }
}
```
- [ ] **Step 2:** run (skips if no Docker) → FAIL when Docker present.
- [ ] **Step 3:** implement table creation + Applied/Pending/Status (Pending = migrations not in Applied; Drift = checksum mismatch).
- [ ] **Step 4:** test PASS (or SKIP without Docker); `go build ./...`, `go vet`.
- [ ] **Step 5:** commit `feat(migrate): migrator status/applied/pending + bookkeeping table`.

### Task 4: Migrator — Up / Down / To
**Files:** Modify `pkg/migrate/migrate.go`; Test `pkg/migrate/migrate_test.go`.

`Up(ctx)` applies pending in order, each in a tx (`db.Begin`), runs `UpSQL`, inserts the bookkeeping row, commits; on error rolls back and returns. `Down(ctx, n)` reverts the last n applied (runs `DownSQL`, deletes the row). `To(ctx, version)` applies up or rolls back down to reach `version`. Take a session advisory lock (`pg_advisory_lock`) around the run.
- [ ] **Step 1:** failing Docker-gated test: `New(db, migs).Up(ctx)` → table `a` exists + `Applied` has `0001`; then `Down(ctx,1)` → table `a` gone + `Applied` empty.
- [ ] **Step 2:** run → FAIL.
- [ ] **Step 3:** implement Up/Down/To with per-migration tx + advisory lock. (Note: pgx pool tx — use `db` as a `pgx.Tx` capable handle; for `*pgxpool.Pool` use `pool.Begin`. Constrain `DBTX` or add a `beginner` interface; simplest: require `*pgxpool.Pool` in `New` for apply, or add `Begin(ctx)` to a `txDBTX`.)
- [ ] **Step 4:** PASS/SKIP; build/vet.
- [ ] **Step 5:** commit `feat(migrate): Up/Down/To with per-migration transactions`.

---

## Phase 3 — dev database

### Task 5: internal/devdb
**Files:** Create `internal/devdb/devdb.go`; Test `internal/devdb/devdb_test.go` (Docker-gated).

```go
type DevDB struct { /* container + url */ }
func Start(ctx context.Context) (*DevDB, error)          // testcontainers postgres
func Open(ctx context.Context, devURL string) (*DevDB, error) // if devURL!="" wrap it (no container); else Start
func (d *DevDB) URL() string
func (d *DevDB) Apply(ctx context.Context, sql string) error // exec SQL on a fresh connection
func (d *DevDB) Close() error
```
- [ ] **Step 1:** failing Docker-gated test: `Start` → `Apply("CREATE TABLE t(id int)")` no error → a connection sees table `t`. `Skipf` if Docker missing.
- [ ] **Step 2:** `go get github.com/testcontainers/testcontainers-go github.com/testcontainers/testcontainers-go/modules/postgres && go mod tidy`; run → FAIL then implement.
- [ ] **Step 3:** implement with `postgres.Run(ctx, "postgres:16-alpine", postgres.WithDatabase(...), ...)`; `Open` returns a non-container DevDB when `devURL` set.
- [ ] **Step 4:** PASS/SKIP; build/vet.
- [ ] **Step 5:** commit `feat(devdb): ephemeral Postgres via testcontainers + --dev-url`.

---

## Phase 4 — introspection

### Task 6: introspect schemas + tables + columns
**Files:** Create `internal/introspect/introspect.go`; Test `internal/introspect/introspect_test.go` (Docker-gated, uses devdb).

`Introspect(ctx, conn DBTX, schemas []string) (*irv1.Catalog, error)`. This task: namespaces → `Schema`; `pg_class` (relkind 'r') + `pg_attribute` → `Table`+`Column` (name, ordinal, type via `format_type`/`pg_type`, NOT NULL, default from `pg_attrdef`, identity, generated). Assign ids `schema.table[.col]`.
- [ ] **Step 1:** Docker-gated test: apply `CREATE SCHEMA app; CREATE TABLE app.users(id bigint primary key, email text not null);` to a devdb → `Introspect` → catalog has schema `app`, table `users` with cols `id`(int8, not null) + `email`(text, not null).
- [ ] **Step 2..5:** implement queries (join pg_namespace/pg_class/pg_attribute/pg_attrdef/pg_type), test, commit `feat(introspect): schemas/tables/columns`.

### Task 7: introspect constraints + indexes
**Files:** Modify `internal/introspect/introspect.go`; Test same.
`pg_constraint` (contype p/f/u/c/x) → PK/FK(with on update/delete actions + referenced table/cols)/unique/check/exclusion; `pg_index` → indexes (method via `pg_am`, unique, predicate, columns/expressions, include, opclass, order). 
- [ ] Docker-gated test: a table with PK + FK(on delete cascade) + unique + an index → introspected constraints/indexes match. Commit `feat(introspect): constraints + indexes`.

### Task 8: introspect types + routines
**Files:** Modify `internal/introspect/introspect.go`; Test same.
`pg_type`/`pg_enum` → enums; domains (base + constraints); composites (attrs); ranges (+multirange); `pg_proc` → functions/procedures; `pg_trigger` → triggers; views/matviews (definition); sequences. 
- [ ] Docker-gated test: an enum + composite + a function + a view introspected. Commit `feat(introspect): types, functions, triggers, views, sequences`.

---

## Phase 5 — diff engine (DB-free, golden-tested)

### Task 9: Change model + Plan
**Files:** Create `internal/diff/change.go`; Test `internal/diff/change_test.go`.
```go
type Change interface { UpSQL() string; DownSQL() string; sortKey() int }
type Plan struct { Changes []Change }
func (p *Plan) Empty() bool
func (p *Plan) UpSQL() string   // join Changes' UpSQL in order, ";\n" separated
func (p *Plan) DownSQL() string // join Changes' DownSQL in REVERSE order
```
Concrete changes (each implements `Change`): `CreateSchema`, `DropSchema`, `CreateEnum`, `DropEnum`, `AddEnumValue`, `CreateTable`, `DropTable`, `AddColumn`, `DropColumn`, `AlterColumnType`, `SetNotNull`, `DropNotNull`, `SetDefault`, `DropDefault`, `AddConstraint`, `DropConstraint`, `CreateIndex`, `DropIndex`, `CreateSequence`/`DropSequence`, `CreateView`/`DropView`, `CreateFunction`/`DropFunction`, `CreateTrigger`/`DropTrigger`. `sortKey` encodes dependency order (schemas=0, types=10, sequences=20, tables=30, columns=40, constraints-nonfk=50, indexes=60, fks=70, views=80, functions=90, triggers=100); drops use negative/high keys so `DownSQL` reverses correctly.
- [ ] Tests: a `CreateTable` change → `UpSQL` contains `CREATE TABLE`, `DownSQL` contains `DROP TABLE`; `Plan.UpSQL` orders by sortKey, `DownSQL` reverses. Commit `feat(diff): change model + plan ordering`.

### Task 10: SQL rendering for changes
**Files:** Create `internal/diff/sql.go`; Test `internal/diff/sql_test.go`.
Render each `Change` from `irv1` objects: `renderColumn(*irv1.Column) string` (name type [NOT NULL] [DEFAULT ...]), `renderConstraint`, `renderCreateTable(*irv1.Table)`, `renderCreateEnum`, etc. Quote identifiers; schema-qualify.
- [ ] Tests: `renderCreateTable` of a 2-col table with PK → expected `CREATE TABLE "app"."users" (...)`. `renderColumn` nullable/default. Commit `feat(diff): DDL rendering`.

### Task 11: Diff — schemas, types, sequences
**Files:** Create `internal/diff/diff.go`; Test `internal/diff/diff_test.go`.
`Diff(from, to *irv1.Catalog) (*Plan, error)`. This task: schema add/drop; enum add/drop + `ADD VALUE` (labels added to an existing enum); domain/composite/range create/drop; sequence create/drop. Index objects by id for comparison.
- [ ] Tests (hand-built or `catalog.Build`-parsed catalogs): from empty → to {schema app, enum status} ⇒ plan has CreateSchema + CreateEnum, UpSQL order schema-before-enum; adding an enum label ⇒ `ALTER TYPE ... ADD VALUE`. Commit `feat(diff): schemas/types/sequences`.

### Task 12: Diff — tables, columns
**Files:** Modify `internal/diff/diff.go`; Test same.
Table add (CreateTable incl. inline columns + PK) / drop; for matching tables, column add/drop, type change (`ALTER COLUMN TYPE ... USING`), set/drop NOT NULL, set/drop default, identity/generated changes.
- [ ] Tests: from {users(id)} to {users(id, email text not null)} ⇒ AddColumn email; changing email nullable→not null ⇒ SetNotNull; dropping a table ⇒ DropTable (and DownSQL re-creates). Commit `feat(diff): tables + columns`.

### Task 13: Diff — constraints + indexes
**Files:** Modify `internal/diff/diff.go`; Test same.
Per matching table: add/drop PK/FK/unique/check/exclusion (compare by name + definition; a changed constraint = drop+add); add/drop indexes (compare by name + definition). FKs ordered after table/column creation (sortKey 70).
- [ ] Tests: adding an FK ⇒ AddConstraint with `REFERENCES ... ON DELETE ...`; adding an index ⇒ CreateIndex; dropping reverses. Commit `feat(diff): constraints + indexes`.

### Task 14: Diff — views, functions, triggers
**Files:** Modify `internal/diff/diff.go`; Test same.
Views/matviews: create/drop, and replace when the definition changed (`CREATE OR REPLACE VIEW`); functions/procedures: create-or-replace/drop (compare by signature + body); triggers: create/drop (recreate on change). Dependency order (views after tables, triggers last).
- [ ] Tests: adding a view ⇒ CreateView after its tables; changing a function body ⇒ CREATE OR REPLACE; dropping a trigger reverses. Commit `feat(diff): views/functions/triggers`.

---

## Phase 6 — CLI

### Task 15: cmd/sqld-migrate — up/down/status/apply
**Files:** Create `cmd/sqld-migrate/main.go`; Test `cmd/sqld-migrate/main_test.go`.
`run(args []string, stdout, stderr io.Writer) int`. Subcommands via stdlib flag. `up`/`down`/`status`/`apply` load migrations (from `sqld.yaml` migrations dir via `config` + `migrate.Load`), connect to `--db`/`$DATABASE_URL` (pgxpool), run the migrator. `status` prints applied/pending/drift.
- [ ] Tests: `run(nil,...)` → usage exit 2; a `status` against a Docker-gated devdb prints pending (skip without Docker); unit-test arg parsing for unknown command. Commit `feat(cmd): sqld-migrate up/down/status/apply`.

### Task 16: cmd/sqld-migrate — generate
**Files:** Modify `cmd/sqld-migrate/main.go`; Test `cmd/sqld-migrate/main_test.go` (Docker-gated).
`generate <name> [-c sqld.yaml] [--dev-url DSN]`:
1. `cfg := config.Load`; read `schema.sql` (the schema sources) + load existing migrations.
2. `dev := devdb.Open(ctx, devURL)`; on a fresh DB apply schema sources → `desired := introspect.Introspect`; on another fresh DB apply all migrations' up → `current := introspect`.
3. `plan := diff.Diff(current, desired)`; if `plan.Empty()` → print "no changes", exit 0.
4. Write `migrations/<timestamp>_<name>.sql` with `-- sqld:up\n<plan.UpSQL>\n\n-- sqld:down\n<plan.DownSQL>`.
- [ ] Test (Docker-gated): a project whose `schema.sql` has a table not in any migration → `generate add_x` writes a migration whose up contains `CREATE TABLE`. Skip without Docker. Commit `feat(cmd): sqld-migrate generate (schema diff via dev DB)`.

### Task 17: hash / validate + Makefile + docs
**Files:** Modify `cmd/sqld-migrate/main.go`, `Makefile`, `docs/`.
`hash` recomputes/prints checksums; `validate` parses every migration (via `parse.Statements`) + reports drift vs applied (needs `--db`). Add a `Makefile` target to build `sqld-migrate`. Document usage in `docs/` (a `migrations` section: file format, `generate`, `--dev-url`).
- [ ] Test: `validate` on a temp migrations dir with a syntactically-bad file → non-zero + message; `hash` prints stable checksums. Commit `feat(cmd): sqld-migrate hash/validate + docs`.

---

## Self-review notes
- **Spec coverage:** migrator apply/rollback/status/integrity (T2-4), devdb (T5), introspect schemas/tables/cols/constraints/indexes/types/routines (T6-8), diff engine all object kinds + ordering + down (T9-14), CLI up/down/status/apply/generate/hash/validate (T15-17), up/down file format (T1). Dev-DB generate flow (T16) matches the spec data-flow.
- **Docker gating:** every test needing a live PG calls `devdb.Start` and `t.Skipf` on error → `go test ./...` stays green without Docker; the diff engine (T9-14) is fully DB-free and golden-tested.
- **Type consistency:** `Migration`/`DBTX`/`Migrator` (T2-4), `DevDB` (T5), `Introspect` (T6-8), `Change`/`Plan`/`Diff` (T9-14), `run` (T15-17) used consistently downstream.
- **Risk:** diff/introspect breadth — phased by object kind; core (schemas/types/tables/columns/constraints/indexes) before views/functions/triggers; ownership/RLS/partitioning out of scope (spec).
