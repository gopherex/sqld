// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/aarondl/opt/null"
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
	"github.com/stephenafamo/scan"
	"github.com/yaroher/sqld/example/gen/db"
)

// AppUser is an object representing the database table.
type AppUser struct {
	ID        int64            `db:"id,pk" `
	Email     string           `db:"email" `
	Status    db.AppUserStatus `db:"status" `
	ManagerID null.Val[int64]  `db:"manager_id" `
	CreatedAt time.Time        `db:"created_at" `

	R appUserR `db:"-" `

	C appUserC `db:"-" `
}

// AppUserSlice is an alias for a slice of pointers to AppUser.
// This should almost always be used instead of []*AppUser.
type AppUserSlice []*AppUser

// AppUsers contains methods to work with the users table
var AppUsers = psql.NewTablex[*AppUser, AppUserSlice, *AppUserSetter]("app", "users", buildAppUserColumns("app.users"))

// AppUsersQuery is a query on the users table
type AppUsersQuery = *psql.ViewQuery[*AppUser, AppUserSlice]

// appUserR is where relationships are stored.
type appUserR struct {
	Orders          AppOrderSlice // orders_fkey_0
	Profile         *AppProfile   // profiles_fkey_0
	Roles           AppRoleSlice  // user_roles_fkey_0user_roles_fkey_1
	Manager         *AppUser      // users_fkey_0
	ReverseManagers AppUserSlice  // users_fkey_0__self_join_reverse
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded appUserRLoaded `db:"-" `
}

// appUserRLoaded tracks which relationships on AppUser have been loaded.
type appUserRLoaded struct {
	Orders          bool // orders_fkey_0
	Profile         bool // profiles_fkey_0
	Roles           bool // user_roles_fkey_0user_roles_fkey_1
	Manager         bool // users_fkey_0
	ReverseManagers bool // users_fkey_0__self_join_reverse
}

func buildAppUserColumns(tableName string) appUserColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "email", "status", "manager_id", "created_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return appUserColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildAppUserColumn(tableName, "id"),
		Email:       buildAppUserColumn(tableName, "email"),
		Status:      buildAppUserColumn(tableName, "status"),
		ManagerID:   buildAppUserColumn(tableName, "manager_id"),
		CreatedAt:   buildAppUserColumn(tableName, "created_at"),
	}
}

type appUserColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         appUserColumn
	Email      appUserColumn
	Status     appUserColumn
	ManagerID  appUserColumn
	CreatedAt  appUserColumn
}

// Alias returns the current table alias for the columns set.
func (c appUserColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (appUserColumns) AliasedAs(tableName string) appUserColumns {
	return buildAppUserColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c appUserColumns) Unqualified() appUserColumns {
	return buildAppUserColumns("")
}

func buildAppUserColumn(alias, name string) appUserColumn {
	return appUserColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type appUserColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c appUserColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c appUserColumn) ShouldOmitParens() bool {
	return true
}

// AppUserSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type AppUserSetter struct {
	ID        *int64            `db:"id,pk" `
	Email     *string           `db:"email" `
	Status    *db.AppUserStatus `db:"status" `
	ManagerID *null.Val[int64]  `db:"manager_id" `
	CreatedAt *time.Time        `db:"created_at" `
}

func (s AppUserSetter) SetColumns() []string {
	vals := make([]string, 0, 5)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.Email != nil {
		vals = append(vals, "email")
	}
	if s.Status != nil {
		vals = append(vals, "status")
	}
	if s.ManagerID != nil {
		vals = append(vals, "manager_id")
	}
	if s.CreatedAt != nil {
		vals = append(vals, "created_at")
	}
	return vals
}

func (s AppUserSetter) Overwrite(t *AppUser) {
	if s.ID != nil {
		t.ID = func() int64 {
			if s.ID == nil {
				return *new(int64)
			}
			return *s.ID
		}()
	}
	if s.Email != nil {
		t.Email = func() string {
			if s.Email == nil {
				return *new(string)
			}
			return *s.Email
		}()
	}
	if s.Status != nil {
		t.Status = func() db.AppUserStatus {
			if s.Status == nil {
				return *new(db.AppUserStatus)
			}
			return *s.Status
		}()
	}
	if s.ManagerID != nil {
		t.ManagerID = func() null.Val[int64] {
			if s.ManagerID == nil {
				return *new(null.Val[int64])
			}
			v := s.ManagerID
			return *v
		}()
	}
	if s.CreatedAt != nil {
		t.CreatedAt = func() time.Time {
			if s.CreatedAt == nil {
				return *new(time.Time)
			}
			return *s.CreatedAt
		}()
	}
}

func (s *AppUserSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return AppUsers.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 5)
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

		if s.Email != nil {
			vals[1] = psql.Arg(func() string {
				if s.Email == nil {
					return *new(string)
				}
				return *s.Email
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.Status != nil {
			vals[2] = psql.Arg(func() db.AppUserStatus {
				if s.Status == nil {
					return *new(db.AppUserStatus)
				}
				return *s.Status
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.ManagerID != nil {
			vals[3] = psql.Arg(func() null.Val[int64] {
				if s.ManagerID == nil {
					return *new(null.Val[int64])
				}
				v := s.ManagerID
				return *v
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.CreatedAt != nil {
			vals[4] = psql.Arg(func() time.Time {
				if s.CreatedAt == nil {
					return *new(time.Time)
				}
				return *s.CreatedAt
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s AppUserSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s AppUserSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 5)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.Email != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "email")...),
			psql.Arg(s.Email),
		}})
	}

	if s.Status != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "status")...),
			psql.Arg(s.Status),
		}})
	}

	if s.ManagerID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "manager_id")...),
			psql.Arg(s.ManagerID),
		}})
	}

	if s.CreatedAt != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "created_at")...),
			psql.Arg(s.CreatedAt),
		}})
	}

	return exprs
}

// FindAppUser retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindAppUser(ctx context.Context, exec bob.Executor, IDPK int64, cols ...string) (*AppUser, error) {
	if len(cols) == 0 {
		return AppUsers.Query(
			sm.Where(AppUsers.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return AppUsers.Query(
		sm.Where(AppUsers.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(AppUsers.Columns.Only(cols...)),
	).One(ctx, exec)
}

// AppUserExists checks the presence of a single record by primary key
func AppUserExists(ctx context.Context, exec bob.Executor, IDPK int64) (bool, error) {
	return AppUsers.Query(
		sm.Where(AppUsers.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after AppUser is retrieved from the database
func (o *AppUser) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppUsers.AfterSelectHooks.RunHooks(ctx, exec, AppUserSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = AppUsers.AfterInsertHooks.RunHooks(ctx, exec, AppUserSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = AppUsers.AfterUpdateHooks.RunHooks(ctx, exec, AppUserSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = AppUsers.AfterDeleteHooks.RunHooks(ctx, exec, AppUserSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = AppUsers.AfterMergeHooks.RunHooks(ctx, exec, AppUserSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the AppUser
func (o *AppUser) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *AppUser) pkEQ() dialect.Expression {
	return psql.Quote("app.users", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the AppUser
func (o *AppUser) Update(ctx context.Context, exec bob.Executor, s *AppUserSetter) error {
	v, err := AppUsers.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single AppUser record with an executor
func (o *AppUser) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := AppUsers.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the AppUser using the executor
func (o *AppUser) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := AppUsers.Query(
		sm.Where(AppUsers.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after AppUserSlice is retrieved from the database
func (o AppUserSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppUsers.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = AppUsers.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = AppUsers.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = AppUsers.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = AppUsers.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o AppUserSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("app.users", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o AppUserSlice) copyMatchingRows(from ...*AppUser) {
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
func (o AppUserSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppUsers.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppUser:
				o.copyMatchingRows(retrieved)
			case []*AppUser:
				o.copyMatchingRows(retrieved...)
			case AppUserSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppUser or a slice of AppUser
				// then run the AfterUpdateHooks on the slice
				_, err = AppUsers.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o AppUserSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppUsers.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppUser:
				o.copyMatchingRows(retrieved)
			case []*AppUser:
				o.copyMatchingRows(retrieved...)
			case AppUserSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppUser or a slice of AppUser
				// then run the AfterDeleteHooks on the slice
				_, err = AppUsers.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o AppUserSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppUsers.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppUser:
				o.copyMatchingRows(retrieved)
			case []*AppUser:
				o.copyMatchingRows(retrieved...)
			case AppUserSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppUser or a slice of AppUser
				// then run the AfterMergeHooks on the slice
				_, err = AppUsers.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o AppUserSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals AppUserSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppUsers.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o AppUserSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppUsers.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o AppUserSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := AppUsers.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// Orders starts a query for related objects on app.orders
func (o *AppUser) Orders(mods ...bob.Mod[*dialect.SelectQuery]) AppOrdersQuery {
	return AppOrders.Query(append(mods,
		sm.Where(AppOrders.Columns.UserID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os AppUserSlice) Orders(mods ...bob.Mod[*dialect.SelectQuery]) AppOrdersQuery {
	pkID := make(pgtypes.Array[int64], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkID = append(pkID, o.ID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkID), "bigserial[]")),
	))

	return AppOrders.Query(append(mods,
		sm.Where(psql.Group(AppOrders.Columns.UserID).OP("IN", PKArgExpr)),
	)...)
}

// Profile starts a query for related objects on app.profiles
func (o *AppUser) Profile(mods ...bob.Mod[*dialect.SelectQuery]) AppProfilesQuery {
	return AppProfiles.Query(append(mods,
		sm.Where(AppProfiles.Columns.UserID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os AppUserSlice) Profile(mods ...bob.Mod[*dialect.SelectQuery]) AppProfilesQuery {
	pkID := make(pgtypes.Array[int64], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkID = append(pkID, o.ID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkID), "bigserial[]")),
	))

	return AppProfiles.Query(append(mods,
		sm.Where(psql.Group(AppProfiles.Columns.UserID).OP("IN", PKArgExpr)),
	)...)
}

// Roles starts a query for related objects on app.roles
func (o *AppUser) Roles(mods ...bob.Mod[*dialect.SelectQuery]) AppRolesQuery {
	return AppRoles.Query(append(mods,
		sm.InnerJoin(AppUserRoles.NameAs()).On(
			AppRoles.Columns.ID.EQ(AppUserRoles.Columns.RoleID)),
		sm.Where(AppUserRoles.Columns.UserID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os AppUserSlice) Roles(mods ...bob.Mod[*dialect.SelectQuery]) AppRolesQuery {
	pkID := make(pgtypes.Array[int64], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkID = append(pkID, o.ID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkID), "bigserial[]")),
	))

	return AppRoles.Query(append(mods,
		sm.InnerJoin(AppUserRoles.NameAs()).On(
			AppRoles.Columns.ID.EQ(AppUserRoles.Columns.RoleID),
		),
		sm.Where(psql.Group(AppUserRoles.Columns.UserID).OP("IN", PKArgExpr)),
	)...)
}

// Manager starts a query for related objects on app.users
func (o *AppUser) Manager(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
	return AppUsers.Query(append(mods,
		sm.Where(AppUsers.Columns.ID.EQ(psql.Arg(o.ManagerID))),
	)...)
}

func (os AppUserSlice) Manager(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
	pkManagerID := make(pgtypes.Array[null.Val[int64]], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkManagerID = append(pkManagerID, o.ManagerID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkManagerID), "int8[]")),
	))

	return AppUsers.Query(append(mods,
		sm.Where(psql.Group(AppUsers.Columns.ID).OP("IN", PKArgExpr)),
	)...)
}

// ReverseManagers starts a query for related objects on app.users
func (o *AppUser) ReverseManagers(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
	return AppUsers.Query(append(mods,
		sm.Where(AppUsers.Columns.ManagerID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os AppUserSlice) ReverseManagers(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
	pkID := make(pgtypes.Array[int64], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkID = append(pkID, o.ID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkID), "bigserial[]")),
	))

	return AppUsers.Query(append(mods,
		sm.Where(psql.Group(AppUsers.Columns.ManagerID).OP("IN", PKArgExpr)),
	)...)
}

func insertAppUserOrders0(ctx context.Context, exec bob.Executor, appOrders1 []*AppOrderSetter, appUser0 *AppUser) (AppOrderSlice, error) {
	for i := range appOrders1 {
		appOrders1[i].UserID = func() *int64 { return &appUser0.ID }()
	}

	ret, err := AppOrders.Insert(bob.ToMods(appOrders1...)).All(ctx, exec)
	if err != nil {
		return ret, fmt.Errorf("insertAppUserOrders0: %w", err)
	}

	return ret, nil
}

func attachAppUserOrders0(ctx context.Context, exec bob.Executor, count int, appOrders1 AppOrderSlice, appUser0 *AppUser) (AppOrderSlice, error) {
	setter := &AppOrderSetter{
		UserID: func() *int64 { return &appUser0.ID }(),
	}

	err := appOrders1.UpdateAll(ctx, exec, *setter)
	if err != nil {
		return nil, fmt.Errorf("attachAppUserOrders0: %w", err)
	}

	return appOrders1, nil
}

func (appUser0 *AppUser) InsertOrders(ctx context.Context, exec bob.Executor, related ...*AppOrderSetter) error {
	if len(related) == 0 {
		return nil
	}

	var err error

	appOrders1, err := insertAppUserOrders0(ctx, exec, related, appUser0)
	if err != nil {
		return err
	}

	appUser0.R.Orders = append(appUser0.R.Orders, appOrders1...)

	for _, rel := range appOrders1 {
		rel.R.User = appUser0
		rel.R.Loaded.User = true
	}
	return nil
}

func (appUser0 *AppUser) AttachOrders(ctx context.Context, exec bob.Executor, related ...*AppOrder) error {
	if len(related) == 0 {
		return nil
	}

	var err error
	appOrders1 := AppOrderSlice(related)

	_, err = attachAppUserOrders0(ctx, exec, len(related), appOrders1, appUser0)
	if err != nil {
		return err
	}

	appUser0.R.Orders = append(appUser0.R.Orders, appOrders1...)

	for _, rel := range related {
		rel.R.User = appUser0
		rel.R.Loaded.User = true
	}

	return nil
}

func insertAppUserProfile0(ctx context.Context, exec bob.Executor, appProfile1 *AppProfileSetter, appUser0 *AppUser) (*AppProfile, error) {
	appProfile1.UserID = func() *int64 { return &appUser0.ID }()

	ret, err := AppProfiles.Insert(appProfile1).One(ctx, exec)
	if err != nil {
		return ret, fmt.Errorf("insertAppUserProfile0: %w", err)
	}

	return ret, nil
}

func attachAppUserProfile0(ctx context.Context, exec bob.Executor, count int, appProfile1 *AppProfile, appUser0 *AppUser) (*AppProfile, error) {
	setter := &AppProfileSetter{
		UserID: func() *int64 { return &appUser0.ID }(),
	}

	err := appProfile1.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachAppUserProfile0: %w", err)
	}

	return appProfile1, nil
}

func (appUser0 *AppUser) InsertProfile(ctx context.Context, exec bob.Executor, related *AppProfileSetter) error {
	var err error

	appProfile1, err := insertAppUserProfile0(ctx, exec, related, appUser0)
	if err != nil {
		return err
	}

	appUser0.R.Profile = appProfile1
	appUser0.R.Loaded.Profile = true

	appProfile1.R.User = appUser0
	appProfile1.R.Loaded.User = true

	return nil
}

func (appUser0 *AppUser) AttachProfile(ctx context.Context, exec bob.Executor, appProfile1 *AppProfile) error {
	var err error

	_, err = attachAppUserProfile0(ctx, exec, 1, appProfile1, appUser0)
	if err != nil {
		return err
	}

	appUser0.R.Profile = appProfile1
	appUser0.R.Loaded.Profile = true

	appProfile1.R.User = appUser0
	appProfile1.R.Loaded.User = true

	return nil
}

func attachAppUserRoles0(ctx context.Context, exec bob.Executor, count int, appUser0 *AppUser, appRoles2 AppRoleSlice) (AppUserRoleSlice, error) {
	setters := make([]*AppUserRoleSetter, count)
	for i := range count {
		setters[i] = &AppUserRoleSetter{
			UserID: func() *int64 { return &appUser0.ID }(),
			RoleID: func() *int64 { return &appRoles2[i].ID }(),
		}
	}

	appUserRoles1, err := AppUserRoles.Insert(bob.ToMods(setters...)).All(ctx, exec)
	if err != nil {
		return nil, fmt.Errorf("attachAppUserRoles0: %w", err)
	}

	return appUserRoles1, nil
}

func (appUser0 *AppUser) InsertRoles(ctx context.Context, exec bob.Executor, related ...*AppRoleSetter) error {
	if len(related) == 0 {
		return nil
	}

	var err error

	inserted, err := AppRoles.Insert(bob.ToMods(related...)).All(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}
	appRoles2 := AppRoleSlice(inserted)

	_, err = attachAppUserRoles0(ctx, exec, len(related), appUser0, appRoles2)
	if err != nil {
		return err
	}

	appUser0.R.Roles = append(appUser0.R.Roles, appRoles2...)

	for _, rel := range appRoles2 {
		rel.R.Users = append(rel.R.Users, appUser0)
	}
	return nil
}

func (appUser0 *AppUser) AttachRoles(ctx context.Context, exec bob.Executor, related ...*AppRole) error {
	if len(related) == 0 {
		return nil
	}

	var err error
	appRoles2 := AppRoleSlice(related)

	_, err = attachAppUserRoles0(ctx, exec, len(related), appUser0, appRoles2)
	if err != nil {
		return err
	}

	appUser0.R.Roles = append(appUser0.R.Roles, appRoles2...)

	for _, rel := range related {
		rel.R.Users = append(rel.R.Users, appUser0)
	}

	return nil
}

func attachAppUserManager0(ctx context.Context, exec bob.Executor, count int, appUser0 *AppUser, appUser1 *AppUser) (*AppUser, error) {
	setter := &AppUserSetter{
		ManagerID: func() *null.Val[int64] { v := null.From(appUser1.ID); return &v }(),
	}

	err := appUser0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachAppUserManager0: %w", err)
	}

	return appUser0, nil
}

func (appUser0 *AppUser) InsertManager(ctx context.Context, exec bob.Executor, related *AppUserSetter) error {
	var err error

	appUser1, err := AppUsers.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachAppUserManager0(ctx, exec, 1, appUser0, appUser1)
	if err != nil {
		return err
	}

	appUser0.R.Manager = appUser1
	appUser0.R.Loaded.Manager = true

	appUser1.R.ReverseManagers = append(appUser1.R.ReverseManagers, appUser0)

	return nil
}

func (appUser0 *AppUser) AttachManager(ctx context.Context, exec bob.Executor, appUser1 *AppUser) error {
	var err error

	_, err = attachAppUserManager0(ctx, exec, 1, appUser0, appUser1)
	if err != nil {
		return err
	}

	appUser0.R.Manager = appUser1
	appUser0.R.Loaded.Manager = true

	appUser1.R.ReverseManagers = append(appUser1.R.ReverseManagers, appUser0)

	return nil
}

func insertAppUserReverseManagers0(ctx context.Context, exec bob.Executor, appUsers1 []*AppUserSetter, appUser0 *AppUser) (AppUserSlice, error) {
	for i := range appUsers1 {
		appUsers1[i].ManagerID = func() *null.Val[int64] { v := null.From(appUser0.ID); return &v }()
	}

	ret, err := AppUsers.Insert(bob.ToMods(appUsers1...)).All(ctx, exec)
	if err != nil {
		return ret, fmt.Errorf("insertAppUserReverseManagers0: %w", err)
	}

	return ret, nil
}

func attachAppUserReverseManagers0(ctx context.Context, exec bob.Executor, count int, appUsers1 AppUserSlice, appUser0 *AppUser) (AppUserSlice, error) {
	setter := &AppUserSetter{
		ManagerID: func() *null.Val[int64] { v := null.From(appUser0.ID); return &v }(),
	}

	err := appUsers1.UpdateAll(ctx, exec, *setter)
	if err != nil {
		return nil, fmt.Errorf("attachAppUserReverseManagers0: %w", err)
	}

	return appUsers1, nil
}

func (appUser0 *AppUser) InsertReverseManagers(ctx context.Context, exec bob.Executor, related ...*AppUserSetter) error {
	if len(related) == 0 {
		return nil
	}

	var err error

	appUsers1, err := insertAppUserReverseManagers0(ctx, exec, related, appUser0)
	if err != nil {
		return err
	}

	appUser0.R.ReverseManagers = append(appUser0.R.ReverseManagers, appUsers1...)

	for _, rel := range appUsers1 {
		rel.R.Manager = appUser0
		rel.R.Loaded.Manager = true
	}
	return nil
}

func (appUser0 *AppUser) AttachReverseManagers(ctx context.Context, exec bob.Executor, related ...*AppUser) error {
	if len(related) == 0 {
		return nil
	}

	var err error
	appUsers1 := AppUserSlice(related)

	_, err = attachAppUserReverseManagers0(ctx, exec, len(related), appUsers1, appUser0)
	if err != nil {
		return err
	}

	appUser0.R.ReverseManagers = append(appUser0.R.ReverseManagers, appUsers1...)

	for _, rel := range related {
		rel.R.Manager = appUser0
		rel.R.Loaded.Manager = true
	}

	return nil
}

type appUserWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, int64]
	Email     psql.WhereMod[Q, string]
	Status    psql.WhereMod[Q, db.AppUserStatus]
	ManagerID psql.WhereNullMod[Q, int64]
	CreatedAt psql.WhereMod[Q, time.Time]
}

func (appUserWhere[Q]) AliasedAs(alias string) appUserWhere[Q] {
	return buildAppUserWhere[Q](buildAppUserColumns(alias))
}

func buildAppUserWhere[Q psql.Filterable](cols appUserColumns) appUserWhere[Q] {
	return appUserWhere[Q]{
		ID:        psql.Where[Q, int64](cols.ID.Expression),
		Email:     psql.Where[Q, string](cols.Email.Expression),
		Status:    psql.Where[Q, db.AppUserStatus](cols.Status.Expression),
		ManagerID: psql.WhereNull[Q, int64](cols.ManagerID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
	}
}

func (o *AppUser) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "Orders":
		rels, ok := retrieved.(AppOrderSlice)
		if !ok {
			return fmt.Errorf("appUser cannot load %T as %q", retrieved, name)
		}

		o.R.Orders = rels
		o.R.Loaded.Orders = true

		for _, rel := range rels {
			if rel != nil {
				rel.R.User = o
				rel.R.Loaded.User = true
			}
		}
		return nil
	case "Profile":
		rel, ok := retrieved.(*AppProfile)
		if !ok {
			return fmt.Errorf("appUser cannot load %T as %q", retrieved, name)
		}

		o.R.Profile = rel
		o.R.Loaded.Profile = true

		if rel != nil {
			rel.R.User = o
			rel.R.Loaded.User = true
		}
		return nil
	case "Roles":
		rels, ok := retrieved.(AppRoleSlice)
		if !ok {
			return fmt.Errorf("appUser cannot load %T as %q", retrieved, name)
		}

		o.R.Roles = rels
		o.R.Loaded.Roles = true

		for _, rel := range rels {
			if rel != nil {
				rel.R.Users = AppUserSlice{o}
			}
		}
		return nil
	case "Manager":
		rel, ok := retrieved.(*AppUser)
		if !ok {
			return fmt.Errorf("appUser cannot load %T as %q", retrieved, name)
		}

		o.R.Manager = rel
		o.R.Loaded.Manager = true

		if rel != nil {
			rel.R.ReverseManagers = AppUserSlice{o}
		}
		return nil
	case "ReverseManagers":
		rels, ok := retrieved.(AppUserSlice)
		if !ok {
			return fmt.Errorf("appUser cannot load %T as %q", retrieved, name)
		}

		o.R.ReverseManagers = rels
		o.R.Loaded.ReverseManagers = true

		for _, rel := range rels {
			if rel != nil {
				rel.R.Manager = o
				rel.R.Loaded.Manager = true
			}
		}
		return nil
	default:
		return fmt.Errorf("appUser has no relationship %q", name)
	}
}

type appUserPreloader struct {
	Profile func(...psql.PreloadOption) psql.Preloader
	Manager func(...psql.PreloadOption) psql.Preloader
}

func buildAppUserPreloader() appUserPreloader {
	return appUserPreloader{
		Profile: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*AppProfile, AppProfileSlice](psql.PreloadRel{
				Name: "Profile",
				Sides: []psql.PreloadSide{
					{
						From:        AppUsers,
						To:          AppProfiles,
						FromColumns: []string{"id"},
						ToColumns:   []string{"user_id"},
					},
				},
			}, AppProfiles.Columns.Names(), opts...)
		},
		Manager: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*AppUser, AppUserSlice](psql.PreloadRel{
				Name: "Manager",
				Sides: []psql.PreloadSide{
					{
						From:        AppUsers,
						To:          AppUsers,
						FromColumns: []string{"manager_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, AppUsers.Columns.Names(), opts...)
		},
	}
}

type appUserThenLoader[Q orm.Loadable] struct {
	Orders          func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Profile         func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Roles           func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Manager         func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	ReverseManagers func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildAppUserThenLoader[Q orm.Loadable]() appUserThenLoader[Q] {
	type OrdersLoadInterface interface {
		LoadOrders(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type ProfileLoadInterface interface {
		LoadProfile(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type RolesLoadInterface interface {
		LoadRoles(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type ManagerLoadInterface interface {
		LoadManager(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type ReverseManagersLoadInterface interface {
		LoadReverseManagers(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return appUserThenLoader[Q]{
		Orders: thenLoadBuilder[Q](
			"Orders",
			func(ctx context.Context, exec bob.Executor, retrieved OrdersLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadOrders(ctx, exec, mods...)
			},
		),
		Profile: thenLoadBuilder[Q](
			"Profile",
			func(ctx context.Context, exec bob.Executor, retrieved ProfileLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadProfile(ctx, exec, mods...)
			},
		),
		Roles: thenLoadBuilder[Q](
			"Roles",
			func(ctx context.Context, exec bob.Executor, retrieved RolesLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadRoles(ctx, exec, mods...)
			},
		),
		Manager: thenLoadBuilder[Q](
			"Manager",
			func(ctx context.Context, exec bob.Executor, retrieved ManagerLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadManager(ctx, exec, mods...)
			},
		),
		ReverseManagers: thenLoadBuilder[Q](
			"ReverseManagers",
			func(ctx context.Context, exec bob.Executor, retrieved ReverseManagersLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadReverseManagers(ctx, exec, mods...)
			},
		),
	}
}

// LoadOrders loads the appUser's Orders into the .R struct
func (o *AppUser) LoadOrders(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	// Reset the relationship
	o.R.Orders = nil
	o.R.Loaded.Orders = false

	related, err := o.Orders(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, rel := range related {
		rel.R.User = o
		rel.R.Loaded.User = true
	}

	o.R.Orders = related
	o.R.Loaded.Orders = true
	return nil
}

// LoadOrders loads the appUser's Orders into the .R struct
func (os AppUserSlice) LoadOrders(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	appOrders, err := os.Orders(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		o.R.Orders = nil
		o.R.Loaded.Orders = true
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		for _, rel := range appOrders {

			if !(o.ID == rel.UserID) {
				continue
			}

			rel.R.User = o
			rel.R.Loaded.User = true

			o.R.Orders = append(o.R.Orders, rel)
		}
	}

	return nil
}

// LoadProfile loads the appUser's Profile into the .R struct
func (o *AppUser) LoadProfile(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	// Reset the relationship
	o.R.Profile = nil
	o.R.Loaded.Profile = false

	related, err := o.Profile(mods...).One(ctx, exec)
	if err != nil {
		return err
	}

	related.R.User = o
	related.R.Loaded.User = true

	o.R.Profile = related
	o.R.Loaded.Profile = true
	return nil
}

// LoadProfile loads the appUser's Profile into the .R struct
func (os AppUserSlice) LoadProfile(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	appProfiles, err := os.Profile(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		o.R.Profile = nil
		o.R.Loaded.Profile = true
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		for _, rel := range appProfiles {

			if !(o.ID == rel.UserID) {
				continue
			}

			rel.R.User = o
			rel.R.Loaded.User = true

			o.R.Profile = rel
			break
		}
	}

	return nil
}

// LoadRoles loads the appUser's Roles into the .R struct
func (o *AppUser) LoadRoles(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	// Reset the relationship
	o.R.Roles = nil
	o.R.Loaded.Roles = false

	related, err := o.Roles(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, rel := range related {
		rel.R.Users = AppUserSlice{o}
	}

	o.R.Roles = related
	o.R.Loaded.Roles = true
	return nil
}

// LoadRoles loads the appUser's Roles into the .R struct
func (os AppUserSlice) LoadRoles(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	// since we are changing the columns, we need to check if the original columns were set or add the defaults
	sq := dialect.SelectQuery{}
	for _, mod := range mods {
		mod.Apply(&sq)
	}

	if len(sq.SelectList.Columns) == 0 {
		mods = append(mods, sm.Columns(AppRoles.Columns))
	}

	q := os.Roles(append(
		mods,
		sm.Columns(AppUserRoles.Columns.UserID.As("related_app.users.ID")),
	)...)

	IDSlice := []int64{}

	mapper := scan.Mod(scan.StructMapper[*AppRole](), func(ctx context.Context, cols []string) (scan.BeforeFunc, func(any, any) error) {
		return func(row *scan.Row) (any, error) {
				IDSlice = append(IDSlice, *new(int64))
				row.ScheduleScanByName("related_app.users.ID", &IDSlice[len(IDSlice)-1])

				return nil, nil
			},
			func(any, any) error {
				return nil
			}
	})

	appRoles, err := bob.Allx[bob.SliceTransformer[*AppRole, AppRoleSlice]](ctx, exec, q, mapper)
	if err != nil {
		return err
	}

	for _, o := range os {
		o.R.Roles = nil
		o.R.Loaded.Roles = true
	}

	for _, o := range os {
		for i, rel := range appRoles {
			if !(o.ID == IDSlice[i]) {
				continue
			}

			rel.R.Users = append(rel.R.Users, o)

			o.R.Roles = append(o.R.Roles, rel)
		}
	}

	return nil
}

// LoadManager loads the appUser's Manager into the .R struct
func (o *AppUser) LoadManager(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	// Reset the relationship
	o.R.Manager = nil
	o.R.Loaded.Manager = false

	related, err := o.Manager(mods...).One(ctx, exec)
	if err != nil {
		return err
	}

	related.R.ReverseManagers = AppUserSlice{o}

	o.R.Manager = related
	o.R.Loaded.Manager = true
	return nil
}

// LoadManager loads the appUser's Manager into the .R struct
func (os AppUserSlice) LoadManager(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	appUsers, err := os.Manager(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		o.R.Manager = nil
		o.R.Loaded.Manager = true
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		for _, rel := range appUsers {
			if !o.ManagerID.IsValue() {
				continue
			}

			if !(o.ManagerID.IsValue() && o.ManagerID.MustGet() == rel.ID) {
				continue
			}

			rel.R.ReverseManagers = append(rel.R.ReverseManagers, o)

			o.R.Manager = rel
			break
		}
	}

	return nil
}

// LoadReverseManagers loads the appUser's ReverseManagers into the .R struct
func (o *AppUser) LoadReverseManagers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	// Reset the relationship
	o.R.ReverseManagers = nil
	o.R.Loaded.ReverseManagers = false

	related, err := o.ReverseManagers(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, rel := range related {
		rel.R.Manager = o
		rel.R.Loaded.Manager = true
	}

	o.R.ReverseManagers = related
	o.R.Loaded.ReverseManagers = true
	return nil
}

// LoadReverseManagers loads the appUser's ReverseManagers into the .R struct
func (os AppUserSlice) LoadReverseManagers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	appUsers, err := os.ReverseManagers(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		o.R.ReverseManagers = nil
		o.R.Loaded.ReverseManagers = true
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		for _, rel := range appUsers {

			if !rel.ManagerID.IsValue() {
				continue
			}
			if !(rel.ManagerID.IsValue() && o.ID == rel.ManagerID.MustGet()) {
				continue
			}

			rel.R.Manager = o
			rel.R.Loaded.Manager = true

			o.R.ReverseManagers = append(o.R.ReverseManagers, rel)
		}
	}

	return nil
}

// appUserC is where relationship counts are stored.
type appUserC struct {
	Orders          *int64
	Roles           *int64
	ReverseManagers *int64
}

// PreloadCount sets a count in the C struct by name
func (o *AppUser) PreloadCount(name string, count int64) error {
	if o == nil {
		return nil
	}

	switch name {
	case "Orders":
		o.C.Orders = &count
	case "Roles":
		o.C.Roles = &count
	case "ReverseManagers":
		o.C.ReverseManagers = &count
	}
	return nil
}

type appUserCountPreloader struct {
	Orders          func(...bob.Mod[*dialect.SelectQuery]) psql.Preloader
	Roles           func(...bob.Mod[*dialect.SelectQuery]) psql.Preloader
	ReverseManagers func(...bob.Mod[*dialect.SelectQuery]) psql.Preloader
}

func buildAppUserCountPreloader() appUserCountPreloader {
	return appUserCountPreloader{
		Orders: func(mods ...bob.Mod[*dialect.SelectQuery]) psql.Preloader {
			return countPreloader[*AppUser]("Orders", func(parent string) bob.Expression {
				// Build a correlated subquery: (SELECT COUNT(*) FROM related WHERE fk = parent.pk)
				if parent == "" {
					parent = AppUsers.Alias()
				}

				subqueryMods := []bob.Mod[*dialect.SelectQuery]{
					sm.Columns(psql.Raw("count(*)")),

					sm.From(AppOrders.Name()),
					sm.Where(psql.Quote(AppOrders.Alias(), "user_id").EQ(psql.Quote(parent, "id"))),
				}
				subqueryMods = append(subqueryMods, mods...)
				return psql.Group(psql.Select(subqueryMods...).Expression)
			})
		},
		Roles: func(mods ...bob.Mod[*dialect.SelectQuery]) psql.Preloader {
			return countPreloader[*AppUser]("Roles", func(parent string) bob.Expression {
				// Build a correlated subquery: (SELECT COUNT(*) FROM related WHERE fk = parent.pk)
				if parent == "" {
					parent = AppUsers.Alias()
				}

				subqueryMods := []bob.Mod[*dialect.SelectQuery]{
					sm.Columns(psql.Raw("count(*)")),

					sm.From(AppUserRoles.Name()),
					sm.Where(psql.Quote(AppUserRoles.Alias(), "user_id").EQ(psql.Quote(parent, "id"))),
					sm.InnerJoin(AppRoles.Name()).On(
						psql.Quote(AppRoles.Alias(), "id").EQ(psql.Quote(AppUserRoles.Alias(), "role_id")),
					),
				}
				subqueryMods = append(subqueryMods, mods...)
				return psql.Group(psql.Select(subqueryMods...).Expression)
			})
		},
		ReverseManagers: func(mods ...bob.Mod[*dialect.SelectQuery]) psql.Preloader {
			return countPreloader[*AppUser]("ReverseManagers", func(parent string) bob.Expression {
				// Build a correlated subquery: (SELECT COUNT(*) FROM related WHERE fk = parent.pk)
				if parent == "" {
					parent = AppUsers.Alias()
				}

				subqueryMods := []bob.Mod[*dialect.SelectQuery]{
					sm.Columns(psql.Raw("count(*)")),

					sm.From(AppUsers.Name()),
					sm.Where(psql.Quote(AppUsers.Alias(), "manager_id").EQ(psql.Quote(parent, "id"))),
				}
				subqueryMods = append(subqueryMods, mods...)
				return psql.Group(psql.Select(subqueryMods...).Expression)
			})
		},
	}
}

type appUserCountThenLoader[Q orm.Loadable] struct {
	Orders          func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Roles           func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	ReverseManagers func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildAppUserCountThenLoader[Q orm.Loadable]() appUserCountThenLoader[Q] {
	type OrdersCountInterface interface {
		LoadCountOrders(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type RolesCountInterface interface {
		LoadCountRoles(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type ReverseManagersCountInterface interface {
		LoadCountReverseManagers(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return appUserCountThenLoader[Q]{
		Orders: countThenLoadBuilder[Q](
			"Orders",
			func(ctx context.Context, exec bob.Executor, retrieved OrdersCountInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadCountOrders(ctx, exec, mods...)
			},
		),
		Roles: countThenLoadBuilder[Q](
			"Roles",
			func(ctx context.Context, exec bob.Executor, retrieved RolesCountInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadCountRoles(ctx, exec, mods...)
			},
		),
		ReverseManagers: countThenLoadBuilder[Q](
			"ReverseManagers",
			func(ctx context.Context, exec bob.Executor, retrieved ReverseManagersCountInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadCountReverseManagers(ctx, exec, mods...)
			},
		),
	}
}

// LoadCountOrders loads the count of Orders into the C struct
func (o *AppUser) LoadCountOrders(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	count, err := o.Orders(mods...).Count(ctx, exec)
	if err != nil {
		return err
	}

	o.C.Orders = &count
	return nil
}

// LoadCountOrders loads the count of Orders for a slice in a single batch query
func (os AppUserSlice) LoadCountOrders(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	// Build the IN arg expression from parent PKs

	pkID := make(pgtypes.Array[int64], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkID = append(pkID, o.ID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkID), "bigserial[]")),
	))

	// countResult holds one scanned row from the batch count query.
	// FK columns are aliased to the parent PK column names for direct map lookup.
	type countResult struct {
		ID    int64
		Count int64
	}

	batchMods := []bob.Mod[*dialect.SelectQuery]{
		// SELECT fk AS parent_pk, count(*)
		sm.Columns(
			AppOrders.Columns.UserID.As("id"),
			psql.Raw("count(*) as count"),
		),
		// Single-hop: FROM related table directly
		sm.From(AppOrders.NameAs()),

		// WHERE fk IN (parent PKs)
		sm.Where(AppOrders.Columns.UserID.OP("IN", PKArgExpr)),
		// GROUP BY fk columns
		sm.GroupBy(AppOrders.Columns.UserID),
	}
	batchMods = append(batchMods, mods...)

	results, err := bob.All(ctx, exec,
		psql.Select(batchMods...),
		scan.StructMapper[countResult](),
	)
	if err != nil {
		return err
	}

	// Single-column FK: direct map lookup
	countMap := make(map[int64]int64, len(results))
	for _, r := range results {
		countMap[r.ID] = r.Count
	}
	for _, o := range os {
		if o == nil {
			continue
		}
		count := countMap[o.ID]
		o.C.Orders = &count
	}

	return nil
}

// LoadCountRoles loads the count of Roles into the C struct
func (o *AppUser) LoadCountRoles(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	count, err := o.Roles(mods...).Count(ctx, exec)
	if err != nil {
		return err
	}

	o.C.Roles = &count
	return nil
}

// LoadCountRoles loads the count of Roles for a slice in a single batch query
func (os AppUserSlice) LoadCountRoles(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	// Build the IN arg expression from parent PKs

	pkID := make(pgtypes.Array[int64], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkID = append(pkID, o.ID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkID), "bigserial[]")),
	))

	// countResult holds one scanned row from the batch count query.
	// FK columns are aliased to the parent PK column names for direct map lookup.
	type countResult struct {
		ID    int64
		Count int64
	}

	batchMods := []bob.Mod[*dialect.SelectQuery]{
		// SELECT fk AS parent_pk, count(*)
		sm.Columns(
			AppUserRoles.Columns.UserID.As("id"),
			psql.Raw("count(*) as count"),
		),
		// Multi-hop: FROM first join table, JOIN through to final related table
		sm.From(AppUserRoles.NameAs()),
		sm.InnerJoin(AppRoles.NameAs()).On(
			AppRoles.Columns.ID.EQ(AppUserRoles.Columns.RoleID),
		),

		// WHERE fk IN (parent PKs)
		sm.Where(AppUserRoles.Columns.UserID.OP("IN", PKArgExpr)),
		// GROUP BY fk columns
		sm.GroupBy(AppUserRoles.Columns.UserID),
	}
	batchMods = append(batchMods, mods...)

	results, err := bob.All(ctx, exec,
		psql.Select(batchMods...),
		scan.StructMapper[countResult](),
	)
	if err != nil {
		return err
	}

	// Single-column FK: direct map lookup
	countMap := make(map[int64]int64, len(results))
	for _, r := range results {
		countMap[r.ID] = r.Count
	}
	for _, o := range os {
		if o == nil {
			continue
		}
		count := countMap[o.ID]
		o.C.Roles = &count
	}

	return nil
}

// LoadCountReverseManagers loads the count of ReverseManagers into the C struct
func (o *AppUser) LoadCountReverseManagers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	count, err := o.ReverseManagers(mods...).Count(ctx, exec)
	if err != nil {
		return err
	}

	o.C.ReverseManagers = &count
	return nil
}

// LoadCountReverseManagers loads the count of ReverseManagers for a slice in a single batch query
func (os AppUserSlice) LoadCountReverseManagers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	// Build the IN arg expression from parent PKs

	pkID := make(pgtypes.Array[int64], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkID = append(pkID, o.ID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkID), "bigserial[]")),
	))

	// countResult holds one scanned row from the batch count query.
	// FK columns are aliased to the parent PK column names for direct map lookup.
	type countResult struct {
		ID    int64
		Count int64
	}

	batchMods := []bob.Mod[*dialect.SelectQuery]{
		// SELECT fk AS parent_pk, count(*)
		sm.Columns(
			AppUsers.Columns.ManagerID.As("id"),
			psql.Raw("count(*) as count"),
		),
		// Single-hop: FROM related table directly
		sm.From(AppUsers.NameAs()),

		// WHERE fk IN (parent PKs)
		sm.Where(AppUsers.Columns.ManagerID.OP("IN", PKArgExpr)),
		// GROUP BY fk columns
		sm.GroupBy(AppUsers.Columns.ManagerID),
	}
	batchMods = append(batchMods, mods...)

	results, err := bob.All(ctx, exec,
		psql.Select(batchMods...),
		scan.StructMapper[countResult](),
	)
	if err != nil {
		return err
	}

	// Single-column FK: direct map lookup
	countMap := make(map[int64]int64, len(results))
	for _, r := range results {
		countMap[r.ID] = r.Count
	}
	for _, o := range os {
		if o == nil {
			continue
		}
		count := countMap[o.ID]
		o.C.ReverseManagers = &count
	}

	return nil
}

type appUserJoins[Q dialect.Joinable] struct {
	typ             string
	Orders          modAs[Q, appOrderColumns]
	Profile         modAs[Q, appProfileColumns]
	Roles           modAs[Q, appRoleColumns]
	Manager         modAs[Q, appUserColumns]
	ReverseManagers modAs[Q, appUserColumns]
}

func (j appUserJoins[Q]) aliasedAs(alias string) appUserJoins[Q] {
	return buildAppUserJoins[Q](buildAppUserColumns(alias), j.typ)
}

func buildAppUserJoins[Q dialect.Joinable](cols appUserColumns, typ string) appUserJoins[Q] {
	return appUserJoins[Q]{
		typ: typ,
		Orders: modAs[Q, appOrderColumns]{
			c: AppOrders.Columns,
			f: func(to appOrderColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, AppOrders.Name().As(to.Alias())).On(
						to.UserID.EQ(cols.ID),
					))
				}

				return mods
			},
		},
		Profile: modAs[Q, appProfileColumns]{
			c: AppProfiles.Columns,
			f: func(to appProfileColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, AppProfiles.Name().As(to.Alias())).On(
						to.UserID.EQ(cols.ID),
					))
				}

				return mods
			},
		},
		Roles: modAs[Q, appRoleColumns]{
			c: AppRoles.Columns,
			f: func(to appRoleColumns) bob.Mod[Q] {
				random := strconv.FormatInt(randInt(), 10)
				mods := make(mods.QueryMods[Q], 0, 2)

				{
					to := AppUserRoles.Columns.AliasedAs(AppUserRoles.Columns.Alias() + random)
					mods = append(mods, dialect.Join[Q](typ, AppUserRoles.Name().As(to.Alias())).On(
						to.UserID.EQ(cols.ID),
					))
				}
				{
					cols := AppUserRoles.Columns.AliasedAs(AppUserRoles.Columns.Alias() + random)
					mods = append(mods, dialect.Join[Q](typ, AppRoles.Name().As(to.Alias())).On(
						to.ID.EQ(cols.RoleID),
					))
				}

				return mods
			},
		},
		Manager: modAs[Q, appUserColumns]{
			c: AppUsers.Columns,
			f: func(to appUserColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, AppUsers.Name().As(to.Alias())).On(
						to.ID.EQ(cols.ManagerID),
					))
				}

				return mods
			},
		},
		ReverseManagers: modAs[Q, appUserColumns]{
			c: AppUsers.Columns,
			f: func(to appUserColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, AppUsers.Name().As(to.Alias())).On(
						to.ManagerID.EQ(cols.ID),
					))
				}

				return mods
			},
		},
	}
}
