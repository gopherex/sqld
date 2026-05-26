package devdb

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// DevDB holds an ephemeral Postgres instance (via testcontainers) or a reference
// to an existing Postgres when a --dev-url is provided.
type DevDB struct {
	container *postgres.PostgresContainer
	url       string
}

// Start launches an ephemeral Postgres container using testcontainers.
func Start(ctx context.Context) (*DevDB, error) {
	ctr, err := postgres.Run(
		ctx,
		"postgres:16-alpine",
		postgres.WithDatabase("sqld_dev"),
		postgres.WithUsername("sqld"),
		postgres.WithPassword("sqld"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		return nil, err
	}

	connStr, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		_ = ctr.Terminate(ctx)
		return nil, err
	}

	return &DevDB{container: ctr, url: connStr}, nil
}

// Open returns a DevDB backed by an existing Postgres when devURL is non-empty,
// otherwise it starts an ephemeral container via Start.
func Open(ctx context.Context, devURL string) (*DevDB, error) {
	if devURL != "" {
		return &DevDB{url: devURL}, nil
	}
	return Start(ctx)
}

// URL returns a pgx-compatible DSN for the database.
func (d *DevDB) URL() string {
	return d.url
}

// External reports whether this DevDB is backed by a caller-supplied --dev-url
// (true) rather than an ephemeral container (false). An external database is
// reused across calls and therefore must be reset between independent states.
func (d *DevDB) External() bool {
	return d.container == nil
}

// Reset drops every non-system schema and recreates a clean public schema,
// returning the database to a near-pristine state. It is used to isolate
// independent introspection states that share one external --dev-url database,
// where (unlike the ephemeral-container path) Close is a no-op and a fresh
// instance is not created per call.
func (d *DevDB) Reset(ctx context.Context) error {
	conn, err := pgx.Connect(ctx, d.url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	// Collect every user (non-system) schema, then drop them all CASCADE and
	// recreate public. Doing this dynamically (rather than only public) ensures
	// objects the previous apply created in other schemas are also cleared.
	rows, err := conn.Query(ctx, `
SELECT nspname
FROM pg_catalog.pg_namespace
WHERE nspname NOT LIKE 'pg\_%'
  AND nspname <> 'information_schema'`)
	if err != nil {
		return err
	}
	var schemas []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			rows.Close()
			return err
		}
		schemas = append(schemas, n)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	var stmt strings.Builder
	for _, s := range schemas {
		stmt.WriteString(`DROP SCHEMA IF EXISTS "`)
		stmt.WriteString(strings.ReplaceAll(s, `"`, `""`))
		stmt.WriteString(`" CASCADE;`)
		stmt.WriteByte('\n')
	}
	stmt.WriteString(`CREATE SCHEMA public;`)
	if _, err := conn.Exec(ctx, stmt.String()); err != nil {
		return err
	}
	return nil
}

// Apply opens a fresh connection and executes the provided SQL (which may
// contain multiple statements).
func (d *DevDB) Apply(ctx context.Context, sql string) error {
	conn, err := pgx.Connect(ctx, d.url)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	_, err = conn.Exec(ctx, sql)
	return err
}

// Close terminates the underlying container. It is a no-op when DevDB was
// created with an external --dev-url.
func (d *DevDB) Close() error {
	if d.container != nil {
		return d.container.Terminate(context.Background())
	}
	return nil
}
