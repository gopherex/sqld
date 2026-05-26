package migrate

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// advisoryLockKey is a fixed key used for pg_advisory_lock so that concurrent
// migration runs against the same database serialize rather than race.
const advisoryLockKey int64 = 0x5371_6c64_4d69_6772 // "SqldMigr" bytes-ish

const defaultTable = "sqld_migrations"

// DBTX is the database handle the Migrator runs against. *pgxpool.Pool
// satisfies it, as does a *pgx.Conn wrapper that exposes Begin.
type DBTX interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Begin(ctx context.Context) (pgx.Tx, error)
}

// AppliedMigration is a row recorded in the bookkeeping table.
type AppliedMigration struct {
	Version   string
	Name      string
	Checksum  string
	AppliedAt time.Time
}

// Status summarizes the migration state of a database.
//
// Applied lists the rows recorded in the bookkeeping table (version order).
// Pending lists loaded migrations not yet applied (version order). Drift lists
// versions whose recorded checksum differs from the loaded migration's
// checksum — i.e. a migration file changed after being applied.
type Status struct {
	Applied []AppliedMigration
	Pending []Migration
	Drift   []string
}

// Migrator applies, reverts, and reports the status of a set of migrations.
type Migrator struct {
	db         DBTX
	migrations []Migration
	table      string
}

// Option configures a Migrator.
type Option func(*Migrator)

// WithTable overrides the bookkeeping table name (default "sqld_migrations").
func WithTable(name string) Option {
	return func(m *Migrator) {
		if name != "" {
			m.table = name
		}
	}
}

// New creates a Migrator over db using the given migrations. The migrations are
// kept in version order regardless of input order.
func New(db DBTX, migrations []Migration, opts ...Option) *Migrator {
	m := &Migrator{
		db:         db,
		migrations: sortedByVersion(migrations),
		table:      defaultTable,
	}
	for _, opt := range opts {
		opt(m)
	}
	return m
}

// Migrate loads migrations from one or more filesystems (merged, then sorted by
// version) and applies all pending ones. It is intended for service startup,
// e.g.:
//
//	//go:embed migrations/*.sql
//	var migrationsFS embed.FS
//
//	if err := migrate.Migrate(ctx, pool, migrationsFS); err != nil { ... }
//
// *pgxpool.Pool satisfies DBTX. Duplicate versions across the merged sources are
// an error. With no sources Migrate is a no-op and returns nil.
func Migrate(ctx context.Context, db DBTX, sources ...fs.FS) error {
	if len(sources) == 0 {
		return nil
	}

	var migs []Migration
	for _, fsys := range sources {
		loaded, err := LoadFS(fsys)
		if err != nil {
			return err
		}
		migs = append(migs, loaded...)
	}

	seen := make(map[string]struct{}, len(migs))
	for _, mig := range migs {
		if _, dup := seen[mig.Version]; dup {
			return fmt.Errorf("migrate: duplicate migration version %q across sources", mig.Version)
		}
		seen[mig.Version] = struct{}{}
	}

	return New(db, migs).Up(ctx)
}

func sortedByVersion(in []Migration) []Migration {
	out := make([]Migration, len(in))
	copy(out, in)
	// Insertion sort keeps it dependency-free and stable for small slices.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j-1].Version > out[j].Version; j-- {
			out[j-1], out[j] = out[j], out[j-1]
		}
	}
	return out
}

// ensureTable creates the bookkeeping table if it does not exist.
func (m *Migrator) ensureTable(ctx context.Context) error {
	q := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s(
	version text PRIMARY KEY,
	name text NOT NULL,
	checksum text NOT NULL,
	applied_at timestamptz NOT NULL DEFAULT now()
)`, quoteIdent(m.table))
	if _, err := m.db.Exec(ctx, q); err != nil {
		return fmt.Errorf("migrate: ensure table %q: %w", m.table, err)
	}
	return nil
}

// Applied returns the recorded migrations in ascending version order.
func (m *Migrator) Applied(ctx context.Context) ([]AppliedMigration, error) {
	if err := m.ensureTable(ctx); err != nil {
		return nil, err
	}
	q := fmt.Sprintf(`SELECT version, name, checksum, applied_at FROM %s ORDER BY version ASC`, quoteIdent(m.table))
	rows, err := m.db.Query(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("migrate: query applied: %w", err)
	}
	defer rows.Close()

	var out []AppliedMigration
	for rows.Next() {
		var a AppliedMigration
		if err := rows.Scan(&a.Version, &a.Name, &a.Checksum, &a.AppliedAt); err != nil {
			return nil, fmt.Errorf("migrate: scan applied: %w", err)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("migrate: iterate applied: %w", err)
	}
	return out, nil
}

// Pending returns loaded migrations not yet applied, in version order.
func (m *Migrator) Pending(ctx context.Context) ([]Migration, error) {
	applied, err := m.Applied(ctx)
	if err != nil {
		return nil, err
	}
	appliedSet := make(map[string]struct{}, len(applied))
	for _, a := range applied {
		appliedSet[a.Version] = struct{}{}
	}

	var pending []Migration
	for _, mig := range m.migrations {
		if _, ok := appliedSet[mig.Version]; !ok {
			pending = append(pending, mig)
		}
	}
	return pending, nil
}

// Status reports applied, pending, and drifted migrations.
func (m *Migrator) Status(ctx context.Context) (Status, error) {
	applied, err := m.Applied(ctx)
	if err != nil {
		return Status{}, err
	}
	pending, err := m.Pending(ctx)
	if err != nil {
		return Status{}, err
	}

	loaded := make(map[string]Migration, len(m.migrations))
	for _, mig := range m.migrations {
		loaded[mig.Version] = mig
	}

	var drift []string
	for _, a := range applied {
		if mig, ok := loaded[a.Version]; ok && mig.Checksum != a.Checksum {
			drift = append(drift, a.Version)
		}
	}

	return Status{Applied: applied, Pending: pending, Drift: drift}, nil
}

// Up applies all pending migrations in version order, each in its own
// transaction, under a session advisory lock.
func (m *Migrator) Up(ctx context.Context) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}
	return m.withLock(ctx, func(ctx context.Context) error {
		pending, err := m.Pending(ctx)
		if err != nil {
			return err
		}
		for _, mig := range pending {
			if err := m.applyOne(ctx, mig); err != nil {
				return err
			}
		}
		return nil
	})
}

// Down reverts the last n applied migrations (descending version order),
// running each migration's DownSQL in its own transaction. It is an error to
// revert a migration with no DownSQL.
func (m *Migrator) Down(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}
	if err := m.ensureTable(ctx); err != nil {
		return err
	}
	return m.withLock(ctx, func(ctx context.Context) error {
		applied, err := m.Applied(ctx)
		if err != nil {
			return err
		}
		if n > len(applied) {
			n = len(applied)
		}
		loaded := make(map[string]Migration, len(m.migrations))
		for _, mig := range m.migrations {
			loaded[mig.Version] = mig
		}
		// Revert from the highest applied version downward.
		for i := 0; i < n; i++ {
			a := applied[len(applied)-1-i]
			mig, ok := loaded[a.Version]
			if !ok {
				return fmt.Errorf("migrate: cannot revert version %q: not loaded", a.Version)
			}
			if err := m.revertOne(ctx, mig); err != nil {
				return err
			}
		}
		return nil
	})
}

// To migrates up or down so the applied set covers exactly the loaded
// migrations with version <= the target version. Migrations with version <=
// target that are not applied are applied (ascending); applied migrations with
// version > target are reverted (descending).
func (m *Migrator) To(ctx context.Context, version string) error {
	if err := m.ensureTable(ctx); err != nil {
		return err
	}
	return m.withLock(ctx, func(ctx context.Context) error {
		applied, err := m.Applied(ctx)
		if err != nil {
			return err
		}
		appliedSet := make(map[string]struct{}, len(applied))
		for _, a := range applied {
			appliedSet[a.Version] = struct{}{}
		}
		loaded := make(map[string]Migration, len(m.migrations))
		for _, mig := range m.migrations {
			loaded[mig.Version] = mig
		}

		// Revert applied migrations above the target, highest first.
		for i := len(applied) - 1; i >= 0; i-- {
			a := applied[i]
			if a.Version <= version {
				break
			}
			mig, ok := loaded[a.Version]
			if !ok {
				return fmt.Errorf("migrate: cannot revert version %q: not loaded", a.Version)
			}
			if err := m.revertOne(ctx, mig); err != nil {
				return err
			}
		}

		// Apply not-yet-applied migrations up to and including the target.
		for _, mig := range m.migrations {
			if mig.Version > version {
				break
			}
			if _, ok := appliedSet[mig.Version]; ok {
				continue
			}
			if err := m.applyOne(ctx, mig); err != nil {
				return err
			}
		}
		return nil
	})
}

// applyOne runs a single migration's UpSQL and records it, in one transaction.
func (m *Migrator) applyOne(ctx context.Context, mig Migration) error {
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrate: begin (apply %s): %w", mig.Version, err)
	}
	if mig.UpSQL != "" {
		if _, err := tx.Exec(ctx, mig.UpSQL); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("migrate: apply %s_%s: %w", mig.Version, mig.Name, err)
		}
	}
	ins := fmt.Sprintf(`INSERT INTO %s(version, name, checksum) VALUES ($1, $2, $3)`, quoteIdent(m.table))
	if _, err := tx.Exec(ctx, ins, mig.Version, mig.Name, mig.Checksum); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("migrate: record %s_%s: %w", mig.Version, mig.Name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migrate: commit (apply %s): %w", mig.Version, err)
	}
	return nil
}

// revertOne runs a single migration's DownSQL and deletes its row, in one
// transaction.
func (m *Migrator) revertOne(ctx context.Context, mig Migration) error {
	if mig.DownSQL == "" {
		return fmt.Errorf("migrate: cannot revert %s_%s: no down SQL", mig.Version, mig.Name)
	}
	tx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrate: begin (revert %s): %w", mig.Version, err)
	}
	if _, err := tx.Exec(ctx, mig.DownSQL); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("migrate: revert %s_%s: %w", mig.Version, mig.Name, err)
	}
	del := fmt.Sprintf(`DELETE FROM %s WHERE version = $1`, quoteIdent(m.table))
	if _, err := tx.Exec(ctx, del, mig.Version); err != nil {
		_ = tx.Rollback(ctx)
		return fmt.Errorf("migrate: unrecord %s_%s: %w", mig.Version, mig.Name, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("migrate: commit (revert %s): %w", mig.Version, err)
	}
	return nil
}

// withLock acquires a session-level advisory lock on a dedicated connection
// (via a transaction), runs fn, then releases the lock. The lock serializes
// concurrent migration runs against the same database.
func (m *Migrator) withLock(ctx context.Context, fn func(context.Context) error) error {
	lockTx, err := m.db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("migrate: begin lock tx: %w", err)
	}
	if _, err := lockTx.Exec(ctx, "SELECT pg_advisory_lock($1)", advisoryLockKey); err != nil {
		_ = lockTx.Rollback(ctx)
		return fmt.Errorf("migrate: acquire advisory lock: %w", err)
	}

	runErr := fn(ctx)

	// Release the lock and tear down the lock-holding transaction regardless of
	// the run outcome.
	_, unlockErr := lockTx.Exec(ctx, "SELECT pg_advisory_unlock($1)", advisoryLockKey)
	rbErr := lockTx.Rollback(ctx)

	if runErr != nil {
		return runErr
	}
	if unlockErr != nil {
		return fmt.Errorf("migrate: release advisory lock: %w", unlockErr)
	}
	if rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
		return fmt.Errorf("migrate: close lock tx: %w", rbErr)
	}
	return nil
}

// quoteIdent wraps a SQL identifier in double quotes, escaping embedded quotes.
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
