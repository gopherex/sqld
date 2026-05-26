package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yaroher/sqld/example/gen/db"
)

func main() {
	ctx := context.Background()
	cfg, err := pgxpool.ParseConfig("postgres://localhost/example")
	if err != nil {
		panic(err)
	}
	// Register composite types per connection so composite columns scan into
	// their generated Go structs.
	cfg.AfterConnect = db.RegisterTypes
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	q := db.New(pool)
	_, _ = q.GetUser(ctx, 1)
	_, _ = q.ListActiveUsers(ctx)
	_, _ = q.GetProfile(ctx, 1)

	// Dynamic query: only the supplied filters are applied at runtime.
	email := "alice@example.com"
	_, _ = q.SearchUsers(ctx, db.SearchUsersParams{
		Email:    &email,
		Ids:      []int64{1, 2},
		OrderBy:  db.SearchUsersOrderByEmail,
		OrderDir: db.OrderDesc,
	})
}
