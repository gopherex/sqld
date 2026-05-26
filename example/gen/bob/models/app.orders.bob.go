// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/bob/expr"
	"github.com/stephenafamo/bob/mods"
	"github.com/stephenafamo/bob/orm"
	"github.com/stephenafamo/bob/types/pgtypes"
)

// Order is an object representing the database table.
type Order struct {
	ID       int64     `db:"id,pk" `
	UserID   int64     `db:"user_id" `
	Total    string    `db:"total" `
	PlacedAt time.Time `db:"placed_at" `

	R orderR `db:"-" `
}

// OrderSlice is an alias for a slice of pointers to Order.
// This should almost always be used instead of []*Order.
type OrderSlice []*Order

// Orders contains methods to work with the orders table
var Orders = psql.NewTablex[*Order, OrderSlice, *OrderSetter]("app", "orders", buildOrderColumns("orders"))

// OrdersQuery is a query on the orders table
type OrdersQuery = *psql.ViewQuery[*Order, OrderSlice]

// orderR is where relationships are stored.
type orderR struct {
	User *User // orders_fkey_0
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded orderRLoaded `db:"-" `
}

// orderRLoaded tracks which relationships on Order have been loaded.
type orderRLoaded struct {
	User bool // orders_fkey_0
}

func buildOrderColumns(tableName string) orderColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "user_id", "total", "placed_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return orderColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildOrderColumn(tableName, "id"),
		UserID:      buildOrderColumn(tableName, "user_id"),
		Total:       buildOrderColumn(tableName, "total"),
		PlacedAt:    buildOrderColumn(tableName, "placed_at"),
	}
}

type orderColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         orderColumn
	UserID     orderColumn
	Total      orderColumn
	PlacedAt   orderColumn
}

// Alias returns the current table alias for the columns set.
func (c orderColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (orderColumns) AliasedAs(tableName string) orderColumns {
	return buildOrderColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c orderColumns) Unqualified() orderColumns {
	return buildOrderColumns("")
}

func buildOrderColumn(alias, name string) orderColumn {
	return orderColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type orderColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c orderColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c orderColumn) ShouldOmitParens() bool {
	return true
}

// OrderSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type OrderSetter struct {
	ID       *int64     `db:"id,pk" `
	UserID   *int64     `db:"user_id" `
	Total    *string    `db:"total" `
	PlacedAt *time.Time `db:"placed_at" `
}

func (s OrderSetter) SetColumns() []string {
	vals := make([]string, 0, 4)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.UserID != nil {
		vals = append(vals, "user_id")
	}
	if s.Total != nil {
		vals = append(vals, "total")
	}
	if s.PlacedAt != nil {
		vals = append(vals, "placed_at")
	}
	return vals
}

func (s OrderSetter) Overwrite(t *Order) {
	if s.ID != nil {
		t.ID = func() int64 {
			if s.ID == nil {
				return *new(int64)
			}
			return *s.ID
		}()
	}
	if s.UserID != nil {
		t.UserID = func() int64 {
			if s.UserID == nil {
				return *new(int64)
			}
			return *s.UserID
		}()
	}
	if s.Total != nil {
		t.Total = func() string {
			if s.Total == nil {
				return *new(string)
			}
			return *s.Total
		}()
	}
	if s.PlacedAt != nil {
		t.PlacedAt = func() time.Time {
			if s.PlacedAt == nil {
				return *new(time.Time)
			}
			return *s.PlacedAt
		}()
	}
}

func (s *OrderSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return Orders.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 4)
		if s.ID != nil {
			vals[0] = psql.Arg(func() int64 {
				if s.ID == nil {
					return *new(int64)
				}
				return *s.ID
			}())
		} else {
			vals[0] = psql.Raw("DEFAULT")
		}

		if s.UserID != nil {
			vals[1] = psql.Arg(func() int64 {
				if s.UserID == nil {
					return *new(int64)
				}
				return *s.UserID
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.Total != nil {
			vals[2] = psql.Arg(func() string {
				if s.Total == nil {
					return *new(string)
				}
				return *s.Total
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.PlacedAt != nil {
			vals[3] = psql.Arg(func() time.Time {
				if s.PlacedAt == nil {
					return *new(time.Time)
				}
				return *s.PlacedAt
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s OrderSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s OrderSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 4)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.UserID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "user_id")...),
			psql.Arg(s.UserID),
		}})
	}

	if s.Total != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "total")...),
			psql.Arg(s.Total),
		}})
	}

	if s.PlacedAt != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "placed_at")...),
			psql.Arg(s.PlacedAt),
		}})
	}

	return exprs
}

// FindOrder retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindOrder(ctx context.Context, exec bob.Executor, IDPK int64, cols ...string) (*Order, error) {
	if len(cols) == 0 {
		return Orders.Query(
			sm.Where(Orders.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return Orders.Query(
		sm.Where(Orders.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(Orders.Columns.Only(cols...)),
	).One(ctx, exec)
}

// OrderExists checks the presence of a single record by primary key
func OrderExists(ctx context.Context, exec bob.Executor, IDPK int64) (bool, error) {
	return Orders.Query(
		sm.Where(Orders.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after Order is retrieved from the database
func (o *Order) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Orders.AfterSelectHooks.RunHooks(ctx, exec, OrderSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = Orders.AfterInsertHooks.RunHooks(ctx, exec, OrderSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = Orders.AfterUpdateHooks.RunHooks(ctx, exec, OrderSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = Orders.AfterDeleteHooks.RunHooks(ctx, exec, OrderSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = Orders.AfterMergeHooks.RunHooks(ctx, exec, OrderSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the Order
func (o *Order) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *Order) pkEQ() dialect.Expression {
	return psql.Quote("orders", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the Order
func (o *Order) Update(ctx context.Context, exec bob.Executor, s *OrderSetter) error {
	v, err := Orders.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single Order record with an executor
func (o *Order) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := Orders.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the Order using the executor
func (o *Order) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := Orders.Query(
		sm.Where(Orders.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after OrderSlice is retrieved from the database
func (o OrderSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Orders.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = Orders.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = Orders.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = Orders.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = Orders.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o OrderSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("orders", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		pkPairs := make([]bob.Expression, len(o))
		for i, row := range o {
			pkPairs[i] = row.primaryKeyVals()
		}
		return bob.ExpressSlice(ctx, w, d, start, pkPairs, "", ", ", "")
	}))
}

// copyMatchingRows finds models in the given slice that have the same primary key
// then it first copies the existing relationships from the old model to the new model
// and then replaces the old model in the slice with the new model
func (o OrderSlice) copyMatchingRows(from ...*Order) {
	for i, old := range o {
		for _, new := range from {
			if new.ID != old.ID {
				continue
			}
			new.R = old.R
			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o OrderSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Orders.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Order:
				o.copyMatchingRows(retrieved)
			case []*Order:
				o.copyMatchingRows(retrieved...)
			case OrderSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Order or a slice of Order
				// then run the AfterUpdateHooks on the slice
				_, err = Orders.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o OrderSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Orders.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Order:
				o.copyMatchingRows(retrieved)
			case []*Order:
				o.copyMatchingRows(retrieved...)
			case OrderSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Order or a slice of Order
				// then run the AfterDeleteHooks on the slice
				_, err = Orders.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o OrderSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Orders.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Order:
				o.copyMatchingRows(retrieved)
			case []*Order:
				o.copyMatchingRows(retrieved...)
			case OrderSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Order or a slice of Order
				// then run the AfterMergeHooks on the slice
				_, err = Orders.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o OrderSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals OrderSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Orders.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o OrderSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Orders.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o OrderSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := Orders.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// User starts a query for related objects on users
func (o *Order) User(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
	return Users.Query(append(mods,
		sm.Where(Users.Columns.ID.EQ(psql.Arg(o.UserID))),
	)...)
}

func (os OrderSlice) User(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
	pkUserID := make(pgtypes.Array[int64], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkUserID = append(pkUserID, o.UserID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkUserID), "int8[]")),
	))

	return Users.Query(append(mods,
		sm.Where(psql.Group(Users.Columns.ID).OP("IN", PKArgExpr)),
	)...)
}

func attachOrderUser0(ctx context.Context, exec bob.Executor, count int, order0 *Order, user1 *User) (*Order, error) {
	setter := &OrderSetter{
		UserID: func() *int64 { return &user1.ID }(),
	}

	err := order0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachOrderUser0: %w", err)
	}

	return order0, nil
}

func (order0 *Order) InsertUser(ctx context.Context, exec bob.Executor, related *UserSetter) error {
	var err error

	user1, err := Users.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachOrderUser0(ctx, exec, 1, order0, user1)
	if err != nil {
		return err
	}

	order0.R.User = user1
	order0.R.Loaded.User = true

	user1.R.Orders = append(user1.R.Orders, order0)

	return nil
}

func (order0 *Order) AttachUser(ctx context.Context, exec bob.Executor, user1 *User) error {
	var err error

	_, err = attachOrderUser0(ctx, exec, 1, order0, user1)
	if err != nil {
		return err
	}

	order0.R.User = user1
	order0.R.Loaded.User = true

	user1.R.Orders = append(user1.R.Orders, order0)

	return nil
}

type orderWhere[Q psql.Filterable] struct {
	ID       psql.WhereMod[Q, int64]
	UserID   psql.WhereMod[Q, int64]
	Total    psql.WhereMod[Q, string]
	PlacedAt psql.WhereMod[Q, time.Time]
}

func (orderWhere[Q]) AliasedAs(alias string) orderWhere[Q] {
	return buildOrderWhere[Q](buildOrderColumns(alias))
}

func buildOrderWhere[Q psql.Filterable](cols orderColumns) orderWhere[Q] {
	return orderWhere[Q]{
		ID:       psql.Where[Q, int64](cols.ID.Expression),
		UserID:   psql.Where[Q, int64](cols.UserID.Expression),
		Total:    psql.Where[Q, string](cols.Total.Expression),
		PlacedAt: psql.Where[Q, time.Time](cols.PlacedAt.Expression),
	}
}

func (o *Order) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "User":
		rel, ok := retrieved.(*User)
		if !ok {
			return fmt.Errorf("order cannot load %T as %q", retrieved, name)
		}

		o.R.User = rel
		o.R.Loaded.User = true

		if rel != nil {
			rel.R.Orders = OrderSlice{o}
		}
		return nil
	default:
		return fmt.Errorf("order has no relationship %q", name)
	}
}

type orderPreloader struct {
	User func(...psql.PreloadOption) psql.Preloader
}

func buildOrderPreloader() orderPreloader {
	return orderPreloader{
		User: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*User, UserSlice](psql.PreloadRel{
				Name: "User",
				Sides: []psql.PreloadSide{
					{
						From:        Orders,
						To:          Users,
						FromColumns: []string{"user_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, Users.Columns.Names(), opts...)
		},
	}
}

type orderThenLoader[Q orm.Loadable] struct {
	User func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildOrderThenLoader[Q orm.Loadable]() orderThenLoader[Q] {
	type UserLoadInterface interface {
		LoadUser(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return orderThenLoader[Q]{
		User: thenLoadBuilder[Q](
			"User",
			func(ctx context.Context, exec bob.Executor, retrieved UserLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadUser(ctx, exec, mods...)
			},
		),
	}
}

// LoadUser loads the order's User into the .R struct
func (o *Order) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	// Reset the relationship
	o.R.User = nil
	o.R.Loaded.User = false

	related, err := o.User(mods...).One(ctx, exec)
	if err != nil {
		return err
	}

	related.R.Orders = OrderSlice{o}

	o.R.User = related
	o.R.Loaded.User = true
	return nil
}

// LoadUser loads the order's User into the .R struct
func (os OrderSlice) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	users, err := os.User(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		o.R.User = nil
		o.R.Loaded.User = true
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		for _, rel := range users {

			if !(o.UserID == rel.ID) {
				continue
			}

			rel.R.Orders = append(rel.R.Orders, o)

			o.R.User = rel
			break
		}
	}

	return nil
}

type orderJoins[Q dialect.Joinable] struct {
	typ  string
	User modAs[Q, userColumns]
}

func (j orderJoins[Q]) aliasedAs(alias string) orderJoins[Q] {
	return buildOrderJoins[Q](buildOrderColumns(alias), j.typ)
}

func buildOrderJoins[Q dialect.Joinable](cols orderColumns, typ string) orderJoins[Q] {
	return orderJoins[Q]{
		typ: typ,
		User: modAs[Q, userColumns]{
			c: Users.Columns,
			f: func(to userColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, Users.Name().As(to.Alias())).On(
						to.ID.EQ(cols.UserID),
					))
				}

				return mods
			},
		},
	}
}
