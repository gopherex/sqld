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
	_, _ = q.GetAuthor(ctx, 1)
	_, _ = q.ListAuthors(ctx)
}
