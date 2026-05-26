// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"fmt"
	"io"

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

// AppUserRole is an object representing the database table.
type AppUserRole struct {
	UserID int64 `db:"user_id,pk" `
	RoleID int64 `db:"role_id,pk" `

	R appUserRoleR `db:"-" `
}

// AppUserRoleSlice is an alias for a slice of pointers to AppUserRole.
// This should almost always be used instead of []*AppUserRole.
type AppUserRoleSlice []*AppUserRole

// AppUserRoles contains methods to work with the user_roles table
var AppUserRoles = psql.NewTablex[*AppUserRole, AppUserRoleSlice, *AppUserRoleSetter]("app", "user_roles", buildAppUserRoleColumns("app.user_roles"))

// AppUserRolesQuery is a query on the user_roles table
type AppUserRolesQuery = *psql.ViewQuery[*AppUserRole, AppUserRoleSlice]

// appUserRoleR is where relationships are stored.
type appUserRoleR struct {
	User *AppUser // user_roles_fkey_0
	Role *AppRole // user_roles_fkey_1
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded appUserRoleRLoaded `db:"-" `
}

// appUserRoleRLoaded tracks which relationships on AppUserRole have been loaded.
type appUserRoleRLoaded struct {
	User bool // user_roles_fkey_0
	Role bool // user_roles_fkey_1
}

func buildAppUserRoleColumns(tableName string) appUserRoleColumns {
	columnsExpr := expr.NewColumnsExpr(
		"user_id", "role_id",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return appUserRoleColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		UserID:      buildAppUserRoleColumn(tableName, "user_id"),
		RoleID:      buildAppUserRoleColumn(tableName, "role_id"),
	}
}

type appUserRoleColumns struct {
	expr.ColumnsExpr
	tableAlias string
	UserID     appUserRoleColumn
	RoleID     appUserRoleColumn
}

// Alias returns the current table alias for the columns set.
func (c appUserRoleColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (appUserRoleColumns) AliasedAs(tableName string) appUserRoleColumns {
	return buildAppUserRoleColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c appUserRoleColumns) Unqualified() appUserRoleColumns {
	return buildAppUserRoleColumns("")
}

func buildAppUserRoleColumn(alias, name string) appUserRoleColumn {
	return appUserRoleColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type appUserRoleColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c appUserRoleColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c appUserRoleColumn) ShouldOmitParens() bool {
	return true
}

// AppUserRoleSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type AppUserRoleSetter struct {
	UserID *int64 `db:"user_id,pk" `
	RoleID *int64 `db:"role_id,pk" `
}

func (s AppUserRoleSetter) SetColumns() []string {
	vals := make([]string, 0, 2)
	if s.UserID != nil {
		vals = append(vals, "user_id")
	}
	if s.RoleID != nil {
		vals = append(vals, "role_id")
	}
	return vals
}

func (s AppUserRoleSetter) Overwrite(t *AppUserRole) {
	if s.UserID != nil {
		t.UserID = func() int64 {
			if s.UserID == nil {
				return *new(int64)
			}
			return *s.UserID
		}()
	}
	if s.RoleID != nil {
		t.RoleID = func() int64 {
			if s.RoleID == nil {
				return *new(int64)
			}
			return *s.RoleID
		}()
	}
}

func (s *AppUserRoleSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return AppUserRoles.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 2)
		if s.UserID != nil {
			vals[0] = psql.Arg(func() int64 {
				if s.UserID == nil {
					return *new(int64)
				}
				return *s.UserID
			}())
		} else {
			vals[0] = psql.Raw("DEFAULT")
		}

		if s.RoleID != nil {
			vals[1] = psql.Arg(func() int64 {
				if s.RoleID == nil {
					return *new(int64)
				}
				return *s.RoleID
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s AppUserRoleSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s AppUserRoleSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 2)

	if s.UserID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "user_id")...),
			psql.Arg(s.UserID),
		}})
	}

	if s.RoleID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "role_id")...),
			psql.Arg(s.RoleID),
		}})
	}

	return exprs
}

// FindAppUserRole retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindAppUserRole(ctx context.Context, exec bob.Executor, UserIDPK int64, RoleIDPK int64, cols ...string) (*AppUserRole, error) {
	if len(cols) == 0 {
		return AppUserRoles.Query(
			sm.Where(AppUserRoles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
			sm.Where(AppUserRoles.Columns.RoleID.EQ(psql.Arg(RoleIDPK))),
		).One(ctx, exec)
	}

	return AppUserRoles.Query(
		sm.Where(AppUserRoles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
		sm.Where(AppUserRoles.Columns.RoleID.EQ(psql.Arg(RoleIDPK))),
		sm.Columns(AppUserRoles.Columns.Only(cols...)),
	).One(ctx, exec)
}

// AppUserRoleExists checks the presence of a single record by primary key
func AppUserRoleExists(ctx context.Context, exec bob.Executor, UserIDPK int64, RoleIDPK int64) (bool, error) {
	return AppUserRoles.Query(
		sm.Where(AppUserRoles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
		sm.Where(AppUserRoles.Columns.RoleID.EQ(psql.Arg(RoleIDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after AppUserRole is retrieved from the database
func (o *AppUserRole) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppUserRoles.AfterSelectHooks.RunHooks(ctx, exec, AppUserRoleSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = AppUserRoles.AfterInsertHooks.RunHooks(ctx, exec, AppUserRoleSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = AppUserRoles.AfterUpdateHooks.RunHooks(ctx, exec, AppUserRoleSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = AppUserRoles.AfterDeleteHooks.RunHooks(ctx, exec, AppUserRoleSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = AppUserRoles.AfterMergeHooks.RunHooks(ctx, exec, AppUserRoleSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the AppUserRole
func (o *AppUserRole) primaryKeyVals() bob.Expression {
	return psql.ArgGroup(
		o.UserID,
		o.RoleID,
	)
}

func (o *AppUserRole) pkEQ() dialect.Expression {
	return psql.Group(psql.Quote("app.user_roles", "user_id"), psql.Quote("app.user_roles", "role_id")).EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the AppUserRole
func (o *AppUserRole) Update(ctx context.Context, exec bob.Executor, s *AppUserRoleSetter) error {
	v, err := AppUserRoles.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single AppUserRole record with an executor
func (o *AppUserRole) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := AppUserRoles.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the AppUserRole using the executor
func (o *AppUserRole) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := AppUserRoles.Query(
		sm.Where(AppUserRoles.Columns.UserID.EQ(psql.Arg(o.UserID))),
		sm.Where(AppUserRoles.Columns.RoleID.EQ(psql.Arg(o.RoleID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after AppUserRoleSlice is retrieved from the database
func (o AppUserRoleSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppUserRoles.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = AppUserRoles.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = AppUserRoles.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = AppUserRoles.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = AppUserRoles.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o AppUserRoleSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Group(psql.Quote("app.user_roles", "user_id"), psql.Quote("app.user_roles", "role_id")).In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o AppUserRoleSlice) copyMatchingRows(from ...*AppUserRole) {
	for i, old := range o {
		for _, new := range from {
			if new.UserID != old.UserID {
				continue
			}
			if new.RoleID != old.RoleID {
				continue
			}
			new.R = old.R
			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o AppUserRoleSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppUserRoles.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppUserRole:
				o.copyMatchingRows(retrieved)
			case []*AppUserRole:
				o.copyMatchingRows(retrieved...)
			case AppUserRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppUserRole or a slice of AppUserRole
				// then run the AfterUpdateHooks on the slice
				_, err = AppUserRoles.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o AppUserRoleSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppUserRoles.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppUserRole:
				o.copyMatchingRows(retrieved)
			case []*AppUserRole:
				o.copyMatchingRows(retrieved...)
			case AppUserRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppUserRole or a slice of AppUserRole
				// then run the AfterDeleteHooks on the slice
				_, err = AppUserRoles.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o AppUserRoleSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppUserRoles.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppUserRole:
				o.copyMatchingRows(retrieved)
			case []*AppUserRole:
				o.copyMatchingRows(retrieved...)
			case AppUserRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppUserRole or a slice of AppUserRole
				// then run the AfterMergeHooks on the slice
				_, err = AppUserRoles.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o AppUserRoleSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals AppUserRoleSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppUserRoles.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o AppUserRoleSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppUserRoles.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o AppUserRoleSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := AppUserRoles.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// User starts a query for related objects on app.users
func (o *AppUserRole) User(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
	return AppUsers.Query(append(mods,
		sm.Where(AppUsers.Columns.ID.EQ(psql.Arg(o.UserID))),
	)...)
}

func (os AppUserRoleSlice) User(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
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

// Role starts a query for related objects on app.roles
func (o *AppUserRole) Role(mods ...bob.Mod[*dialect.SelectQuery]) AppRolesQuery {
	return AppRoles.Query(append(mods,
		sm.Where(AppRoles.Columns.ID.EQ(psql.Arg(o.RoleID))),
	)...)
}

func (os AppUserRoleSlice) Role(mods ...bob.Mod[*dialect.SelectQuery]) AppRolesQuery {
	pkRoleID := make(pgtypes.Array[int64], 0, len(os))
	for _, o := range os {
		if o == nil {
			continue
		}
		pkRoleID = append(pkRoleID, o.RoleID)
	}
	PKArgExpr := psql.Select(sm.Columns(
		psql.F("unnest", psql.Cast(psql.Arg(pkRoleID), "int8[]")),
	))

	return AppRoles.Query(append(mods,
		sm.Where(psql.Group(AppRoles.Columns.ID).OP("IN", PKArgExpr)),
	)...)
}

func attachAppUserRoleUser0(ctx context.Context, exec bob.Executor, count int, appUserRole0 *AppUserRole, appUser1 *AppUser) (*AppUserRole, error) {
	setter := &AppUserRoleSetter{
		UserID: func() *int64 { return &appUser1.ID }(),
	}

	err := appUserRole0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachAppUserRoleUser0: %w", err)
	}

	return appUserRole0, nil
}

func (appUserRole0 *AppUserRole) InsertUser(ctx context.Context, exec bob.Executor, related *AppUserSetter) error {
	var err error

	appUser1, err := AppUsers.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachAppUserRoleUser0(ctx, exec, 1, appUserRole0, appUser1)
	if err != nil {
		return err
	}

	appUserRole0.R.User = appUser1
	appUserRole0.R.Loaded.User = true

	return nil
}

func (appUserRole0 *AppUserRole) AttachUser(ctx context.Context, exec bob.Executor, appUser1 *AppUser) error {
	var err error

	_, err = attachAppUserRoleUser0(ctx, exec, 1, appUserRole0, appUser1)
	if err != nil {
		return err
	}

	appUserRole0.R.User = appUser1
	appUserRole0.R.Loaded.User = true

	return nil
}

func attachAppUserRoleRole0(ctx context.Context, exec bob.Executor, count int, appUserRole0 *AppUserRole, appRole1 *AppRole) (*AppUserRole, error) {
	setter := &AppUserRoleSetter{
		RoleID: func() *int64 { return &appRole1.ID }(),
	}

	err := appUserRole0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachAppUserRoleRole0: %w", err)
	}

	return appUserRole0, nil
}

func (appUserRole0 *AppUserRole) InsertRole(ctx context.Context, exec bob.Executor, related *AppRoleSetter) error {
	var err error

	appRole1, err := AppRoles.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachAppUserRoleRole0(ctx, exec, 1, appUserRole0, appRole1)
	if err != nil {
		return err
	}

	appUserRole0.R.Role = appRole1
	appUserRole0.R.Loaded.Role = true

	return nil
}

func (appUserRole0 *AppUserRole) AttachRole(ctx context.Context, exec bob.Executor, appRole1 *AppRole) error {
	var err error

	_, err = attachAppUserRoleRole0(ctx, exec, 1, appUserRole0, appRole1)
	if err != nil {
		return err
	}

	appUserRole0.R.Role = appRole1
	appUserRole0.R.Loaded.Role = true

	return nil
}

type appUserRoleWhere[Q psql.Filterable] struct {
	UserID psql.WhereMod[Q, int64]
	RoleID psql.WhereMod[Q, int64]
}

func (appUserRoleWhere[Q]) AliasedAs(alias string) appUserRoleWhere[Q] {
	return buildAppUserRoleWhere[Q](buildAppUserRoleColumns(alias))
}

func buildAppUserRoleWhere[Q psql.Filterable](cols appUserRoleColumns) appUserRoleWhere[Q] {
	return appUserRoleWhere[Q]{
		UserID: psql.Where[Q, int64](cols.UserID.Expression),
		RoleID: psql.Where[Q, int64](cols.RoleID.Expression),
	}
}

func (o *AppUserRole) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "User":
		rel, ok := retrieved.(*AppUser)
		if !ok {
			return fmt.Errorf("appUserRole cannot load %T as %q", retrieved, name)
		}

		o.R.User = rel
		o.R.Loaded.User = true

		return nil
	case "Role":
		rel, ok := retrieved.(*AppRole)
		if !ok {
			return fmt.Errorf("appUserRole cannot load %T as %q", retrieved, name)
		}

		o.R.Role = rel
		o.R.Loaded.Role = true

		return nil
	default:
		return fmt.Errorf("appUserRole has no relationship %q", name)
	}
}

type appUserRolePreloader struct {
	User func(...psql.PreloadOption) psql.Preloader
	Role func(...psql.PreloadOption) psql.Preloader
}

func buildAppUserRolePreloader() appUserRolePreloader {
	return appUserRolePreloader{
		User: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*AppUser, AppUserSlice](psql.PreloadRel{
				Name: "User",
				Sides: []psql.PreloadSide{
					{
						From:        AppUserRoles,
						To:          AppUsers,
						FromColumns: []string{"user_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, AppUsers.Columns.Names(), opts...)
		},
		Role: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*AppRole, AppRoleSlice](psql.PreloadRel{
				Name: "Role",
				Sides: []psql.PreloadSide{
					{
						From:        AppUserRoles,
						To:          AppRoles,
						FromColumns: []string{"role_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, AppRoles.Columns.Names(), opts...)
		},
	}
}

type appUserRoleThenLoader[Q orm.Loadable] struct {
	User func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Role func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildAppUserRoleThenLoader[Q orm.Loadable]() appUserRoleThenLoader[Q] {
	type UserLoadInterface interface {
		LoadUser(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type RoleLoadInterface interface {
		LoadRole(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return appUserRoleThenLoader[Q]{
		User: thenLoadBuilder[Q](
			"User",
			func(ctx context.Context, exec bob.Executor, retrieved UserLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadUser(ctx, exec, mods...)
			},
		),
		Role: thenLoadBuilder[Q](
			"Role",
			func(ctx context.Context, exec bob.Executor, retrieved RoleLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadRole(ctx, exec, mods...)
			},
		),
	}
}

// LoadUser loads the appUserRole's User into the .R struct
func (o *AppUserRole) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

	o.R.User = related
	o.R.Loaded.User = true
	return nil
}

// LoadUser loads the appUserRole's User into the .R struct
func (os AppUserRoleSlice) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

			o.R.User = rel
			break
		}
	}

	return nil
}

// LoadRole loads the appUserRole's Role into the .R struct
func (o *AppUserRole) LoadRole(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	// Reset the relationship
	o.R.Role = nil
	o.R.Loaded.Role = false

	related, err := o.Role(mods...).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R.Role = related
	o.R.Loaded.Role = true
	return nil
}

// LoadRole loads the appUserRole's Role into the .R struct
func (os AppUserRoleSlice) LoadRole(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	appRoles, err := os.Role(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		o.R.Role = nil
		o.R.Loaded.Role = true
	}

	for _, o := range os {
		if o == nil {
			continue
		}

		for _, rel := range appRoles {

			if !(o.RoleID == rel.ID) {
				continue
			}

			o.R.Role = rel
			break
		}
	}

	return nil
}

type appUserRoleJoins[Q dialect.Joinable] struct {
	typ  string
	User modAs[Q, appUserColumns]
	Role modAs[Q, appRoleColumns]
}

func (j appUserRoleJoins[Q]) aliasedAs(alias string) appUserRoleJoins[Q] {
	return buildAppUserRoleJoins[Q](buildAppUserRoleColumns(alias), j.typ)
}

func buildAppUserRoleJoins[Q dialect.Joinable](cols appUserRoleColumns, typ string) appUserRoleJoins[Q] {
	return appUserRoleJoins[Q]{
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
		Role: modAs[Q, appRoleColumns]{
			c: AppRoles.Columns,
			f: func(to appRoleColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, AppRoles.Name().As(to.Alias())).On(
						to.ID.EQ(cols.RoleID),
					))
				}

				return mods
			},
		},
	}
}
