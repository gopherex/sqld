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

	// Composite parameter (encode): pass an app.address value.
	_ = q.SetAddress(ctx, db.SetAddressParams{Address: db.AppAddress{Street: "x"}, UserID: 1})
	// Composite ARRAY scan: the row field is []AppAddress.
	_, _ = q.GetPrevAddresses(ctx, 1)

	// Array-of-enum scan: the row field is []AppUserStatus.
	hist, _ := q.GetStatusHistory(ctx, 1)
	_ = hist.StatusHistory // []db.AppUserStatus

	// Nested composite scan: the row field is AppPerson, whose Home field is an
	// AppAddress and whose Status field is an AppUserStatus.
	owner, _ := q.GetOwner(ctx, 1)
	_ = owner.Owner.Home   // db.AppAddress
	_ = owner.Owner.Status // db.AppUserStatus

	// Dynamic query: only the supplied filters are applied at runtime.
	email := "alice@example.com"
	_, _ = q.SearchUsers(ctx, db.SearchUsersParams{
		Email:    &email,
		Ids:      []int64{1, 2},
		OrderBy:  db.SearchUsersOrderByEmail,
		OrderDir: db.OrderDesc,
	})
}
