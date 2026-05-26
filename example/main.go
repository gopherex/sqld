package main

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yaroher/sqld/example/gen/db"
)

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://localhost/example")
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	q := db.New(pool)
	_, _ = q.GetUser(ctx, 1)
	_, _ = q.ListActiveUsers(ctx)

	// Dynamic query: only the supplied filters are applied at runtime.
	var email any = "alice@example.com"
	_, _ = q.SearchUsers(ctx, db.SearchUsersParams{
		Email:    &email,
		Ids:      []int64{1, 2},
		OrderBy:  db.SearchUsersOrderByEmail,
		OrderDir: db.OrderDesc,
	})
}
