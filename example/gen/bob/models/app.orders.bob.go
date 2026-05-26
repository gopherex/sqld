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

// AppOrder is an object representing the database table.
type AppOrder struct {
	ID       int64     `db:"id,pk" `
	UserID   int64     `db:"user_id" `
	Total    string    `db:"total" `
	PlacedAt time.Time `db:"placed_at" `

	R appOrderR `db:"-" `
}

// AppOrderSlice is an alias for a slice of pointers to AppOrder.
// This should almost always be used instead of []*AppOrder.
type AppOrderSlice []*AppOrder

// AppOrders contains methods to work with the orders table
var AppOrders = psql.NewTablex[*AppOrder, AppOrderSlice, *AppOrderSetter]("app", "orders", buildAppOrderColumns("app.orders"))

// AppOrdersQuery is a query on the orders table
type AppOrdersQuery = *psql.ViewQuery[*AppOrder, AppOrderSlice]

// appOrderR is where relationships are stored.
type appOrderR struct {
	User *AppUser // orders_fkey_0
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded appOrderRLoaded `db:"-" `
}

// appOrderRLoaded tracks which relationships on AppOrder have been loaded.
type appOrderRLoaded struct {
	User bool // orders_fkey_0
}

func buildAppOrderColumns(tableName string) appOrderColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "user_id", "total", "placed_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return appOrderColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildAppOrderColumn(tableName, "id"),
		UserID:      buildAppOrderColumn(tableName, "user_id"),
		Total:       buildAppOrderColumn(tableName, "total"),
		PlacedAt:    buildAppOrderColumn(tableName, "placed_at"),
	}
}

type appOrderColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         appOrderColumn
	UserID     appOrderColumn
	Total      appOrderColumn
	PlacedAt   appOrderColumn
}

// Alias returns the current table alias for the columns set.
func (c appOrderColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (appOrderColumns) AliasedAs(tableName string) appOrderColumns {
	return buildAppOrderColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c appOrderColumns) Unqualified() appOrderColumns {
	return buildAppOrderColumns("")
}

func buildAppOrderColumn(alias, name string) appOrderColumn {
	return appOrderColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type appOrderColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c appOrderColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c appOrderColumn) ShouldOmitParens() bool {
	return true
}

// AppOrderSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type AppOrderSetter struct {
	ID       *int64     `db:"id,pk" `
	UserID   *int64     `db:"user_id" `
	Total    *string    `db:"total" `
	PlacedAt *time.Time `db:"placed_at" `
}

func (s AppOrderSetter) SetColumns() []string {
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

func (s AppOrderSetter) Overwrite(t *AppOrder) {
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

func (s *AppOrderSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return AppOrders.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s AppOrderSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s AppOrderSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindAppOrder retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindAppOrder(ctx context.Context, exec bob.Executor, IDPK int64, cols ...string) (*AppOrder, error) {
	if len(cols) == 0 {
		return AppOrders.Query(
			sm.Where(AppOrders.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return AppOrders.Query(
		sm.Where(AppOrders.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(AppOrders.Columns.Only(cols...)),
	).One(ctx, exec)
}

// AppOrderExists checks the presence of a single record by primary key
func AppOrderExists(ctx context.Context, exec bob.Executor, IDPK int64) (bool, error) {
	return AppOrders.Query(
		sm.Where(AppOrders.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after AppOrder is retrieved from the database
func (o *AppOrder) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppOrders.AfterSelectHooks.RunHooks(ctx, exec, AppOrderSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = AppOrders.AfterInsertHooks.RunHooks(ctx, exec, AppOrderSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = AppOrders.AfterUpdateHooks.RunHooks(ctx, exec, AppOrderSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = AppOrders.AfterDeleteHooks.RunHooks(ctx, exec, AppOrderSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = AppOrders.AfterMergeHooks.RunHooks(ctx, exec, AppOrderSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the AppOrder
func (o *AppOrder) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *AppOrder) pkEQ() dialect.Expression {
	return psql.Quote("app.orders", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the AppOrder
func (o *AppOrder) Update(ctx context.Context, exec bob.Executor, s *AppOrderSetter) error {
	v, err := AppOrders.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single AppOrder record with an executor
func (o *AppOrder) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := AppOrders.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the AppOrder using the executor
func (o *AppOrder) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := AppOrders.Query(
		sm.Where(AppOrders.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after AppOrderSlice is retrieved from the database
func (o AppOrderSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppOrders.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = AppOrders.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = AppOrders.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = AppOrders.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = AppOrders.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o AppOrderSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("app.orders", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o AppOrderSlice) copyMatchingRows(from ...*AppOrder) {
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
func (o AppOrderSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppOrders.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppOrder:
				o.copyMatchingRows(retrieved)
			case []*AppOrder:
				o.copyMatchingRows(retrieved...)
			case AppOrderSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppOrder or a slice of AppOrder
				// then run the AfterUpdateHooks on the slice
				_, err = AppOrders.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o AppOrderSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppOrders.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppOrder:
				o.copyMatchingRows(retrieved)
			case []*AppOrder:
				o.copyMatchingRows(retrieved...)
			case AppOrderSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppOrder or a slice of AppOrder
				// then run the AfterDeleteHooks on the slice
				_, err = AppOrders.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o AppOrderSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppOrders.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppOrder:
				o.copyMatchingRows(retrieved)
			case []*AppOrder:
				o.copyMatchingRows(retrieved...)
			case AppOrderSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppOrder or a slice of AppOrder
				// then run the AfterMergeHooks on the slice
				_, err = AppOrders.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o AppOrderSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals AppOrderSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppOrders.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o AppOrderSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppOrders.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o AppOrderSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := AppOrders.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// User starts a query for related objects on app.users
func (o *AppOrder) User(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
	return AppUsers.Query(append(mods,
		sm.Where(AppUsers.Columns.ID.EQ(psql.Arg(o.UserID))),
	)...)
}

func (os AppOrderSlice) User(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
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

	return AppUsers.Query(append(mods,
		sm.Where(psql.Group(AppUsers.Columns.ID).OP("IN", PKArgExpr)),
	)...)
}

func attachAppOrderUser0(ctx context.Context, exec bob.Executor, count int, appOrder0 *AppOrder, appUser1 *AppUser) (*AppOrder, error) {
	setter := &AppOrderSetter{
		UserID: func() *int64 { return &appUser1.ID }(),
	}

	err := appOrder0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachAppOrderUser0: %w", err)
	}

	return appOrder0, nil
}

func (appOrder0 *AppOrder) InsertUser(ctx context.Context, exec bob.Executor, related *AppUserSetter) error {
	var err error

	appUser1, err := AppUsers.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachAppOrderUser0(ctx, exec, 1, appOrder0, appUser1)
	if err != nil {
		return err
	}

	appOrder0.R.User = appUser1
	appOrder0.R.Loaded.User = true

	appUser1.R.Orders = append(appUser1.R.Orders, appOrder0)

	return nil
}

func (appOrder0 *AppOrder) AttachUser(ctx context.Context, exec bob.Executor, appUser1 *AppUser) error {
	var err error

	_, err = attachAppOrderUser0(ctx, exec, 1, appOrder0, appUser1)
	if err != nil {
		return err
	}

	appOrder0.R.User = appUser1
	appOrder0.R.Loaded.User = true

	appUser1.R.Orders = append(appUser1.R.Orders, appOrder0)

	return nil
}

type appOrderWhere[Q psql.Filterable] struct {
	ID       psql.WhereMod[Q, int64]
	UserID   psql.WhereMod[Q, int64]
	Total    psql.WhereMod[Q, string]
	PlacedAt psql.WhereMod[Q, time.Time]
}

func (appOrderWhere[Q]) AliasedAs(alias string) appOrderWhere[Q] {
	return buildAppOrderWhere[Q](buildAppOrderColumns(alias))
}

func buildAppOrderWhere[Q psql.Filterable](cols appOrderColumns) appOrderWhere[Q] {
	return appOrderWhere[Q]{
		ID:       psql.Where[Q, int64](cols.ID.Expression),
		UserID:   psql.Where[Q, int64](cols.UserID.Expression),
		Total:    psql.Where[Q, string](cols.Total.Expression),
		PlacedAt: psql.Where[Q, time.Time](cols.PlacedAt.Expression),
	}
}

func (o *AppOrder) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "User":
		rel, ok := retrieved.(*AppUser)
		if !ok {
			return fmt.Errorf("appOrder cannot load %T as %q", retrieved, name)
		}

		o.R.User = rel
		o.R.Loaded.User = true

		if rel != nil {
			rel.R.Orders = AppOrderSlice{o}
		}
		return nil
	default:
		return fmt.Errorf("appOrder has no relationship %q", name)
	}
}

type appOrderPreloader struct {
	User func(...psql.PreloadOption) psql.Preloader
}

func buildAppOrderPreloader() appOrderPreloader {
	return appOrderPreloader{
		User: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*AppUser, AppUserSlice](psql.PreloadRel{
				Name: "User",
				Sides: []psql.PreloadSide{
					{
						From:        AppOrders,
						To:          AppUsers,
						FromColumns: []string{"user_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, AppUsers.Columns.Names(), opts...)
		},
	}
}

type appOrderThenLoader[Q orm.Loadable] struct {
	User func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildAppOrderThenLoader[Q orm.Loadable]() appOrderThenLoader[Q] {
	type UserLoadInterface interface {
		LoadUser(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return appOrderThenLoader[Q]{
		User: thenLoadBuilder[Q](
			"User",
			func(ctx context.Context, exec bob.Executor, retrieved UserLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadUser(ctx, exec, mods...)
			},
		),
	}
}

// LoadUser loads the appOrder's User into the .R struct
func (o *AppOrder) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

	related.R.Orders = AppOrderSlice{o}

	o.R.User = related
	o.R.Loaded.User = true
	return nil
}

// LoadUser loads the appOrder's User into the .R struct
func (os AppOrderSlice) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	appUsers, err := os.User(mods...).All(ctx, exec)
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

		for _, rel := range appUsers {

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

type appOrderJoins[Q dialect.Joinable] struct {
	typ  string
	User modAs[Q, appUserColumns]
}

func (j appOrderJoins[Q]) aliasedAs(alias string) appOrderJoins[Q] {
	return buildAppOrderJoins[Q](buildAppOrderColumns(alias), j.typ)
}

func buildAppOrderJoins[Q dialect.Joinable](cols appOrderColumns, typ string) appOrderJoins[Q] {
	return appOrderJoins[Q]{
		typ: typ,
		User: modAs[Q, appUserColumns]{
			c: AppUsers.Columns,
			f: func(to appUserColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, AppUsers.Name().As(to.Alias())).On(
						to.ID.EQ(cols.UserID),
					))
				}

				return mods
			},
		},
	}
}
