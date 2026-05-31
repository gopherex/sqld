package main

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/gopherex/sqld/example/gen/db"
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

	// Builtin range scan: the row field is pgtype.Range[pgtype.Timestamptz].
	_, _ = q.GetActiveDuring(ctx, 1)

	// Custom range scan (CREATE TYPE app.timerange AS RANGE (subtype = timestamptz)):
	// the row field is pgtype.Range[pgtype.Timestamptz], registered by RegisterTypes.
	_, _ = q.GetValidWindow(ctx, 1)

	// Custom multirange scan (the MULTIRANGE auto-created for app.timerange):
	// the row field is pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]],
	// registered by RegisterTypes after the range element.
	_, _ = q.GetWindows(ctx, 1)

	// Dynamic query: only the supplied filters are applied at runtime.
	email := "alice@example.com"
	_, _ = q.SearchUsers(ctx, db.SearchUsersParams{
		Email:    &email,
		Ids:      []int64{1, 2},
		OrderBy:  db.SearchUsersOrderByEmail,
		OrderDir: db.OrderDesc,
	})

	// WithTx: the same generated methods run inside a transaction. pgx.Tx
	// satisfies the generated DBTX (Exec/Query/QueryRow + CopyFrom/SendBatch).
	// Compile-only: a nil pgx.Tx is never executed against here.
	var tx pgx.Tx
	qtx := q.WithTx(tx)

	// :copyfrom bulk insert via the COPY protocol → number of rows copied.
	_, _ = qtx.BulkCreateRoles(ctx, []db.BulkCreateRolesParams{
		{Name: "admin"},
		{Name: "user"},
	})

	// :batchexec batched DML via pgx.Batch → typed BatchResults wrapper.
	res := qtx.BulkTouchUsers(ctx, []db.BulkTouchUsersParams{
		{ID: 1, Status: db.AppUserStatusActive},
		{ID: 2, Status: db.AppUserStatusInactive},
	})
	res.Exec(func(i int, err error) { _ = i; _ = err })
	_ = res.Close()
}
