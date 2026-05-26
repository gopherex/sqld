package devdb

import (
	"context"

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
