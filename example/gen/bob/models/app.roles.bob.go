// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"fmt"
	"io"
	"strconv"

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
)

// AppRole is an object representing the database table.
type AppRole struct {
	ID   int64  `db:"id,pk" `
	Name string `db:"name" `

	R appRoleR `db:"-" `

	C appRoleC `db:"-" `
}

// AppRoleSlice is an alias for a slice of pointers to AppRole.
// This should almost always be used instead of []*AppRole.
type AppRoleSlice []*AppRole

// AppRoles contains methods to work with the roles table
var AppRoles = psql.NewTablex[*AppRole, AppRoleSlice, *AppRoleSetter]("app", "roles", buildAppRoleColumns("app.roles"))

// AppRolesQuery is a query on the roles table
type AppRolesQuery = *psql.ViewQuery[*AppRole, AppRoleSlice]

// appRoleR is where relationships are stored.
type appRoleR struct {
	Users AppUserSlice // user_roles_fkey_0user_roles_fkey_1
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded appRoleRLoaded `db:"-" `
}

// appRoleRLoaded tracks which relationships on AppRole have been loaded.
type appRoleRLoaded struct {
	Users bool // user_roles_fkey_0user_roles_fkey_1
}

func buildAppRoleColumns(tableName string) appRoleColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "name",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return appRoleColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildAppRoleColumn(tableName, "id"),
		Name:        buildAppRoleColumn(tableName, "name"),
	}
}

type appRoleColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         appRoleColumn
	Name       appRoleColumn
}

// Alias returns the current table alias for the columns set.
func (c appRoleColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (appRoleColumns) AliasedAs(tableName string) appRoleColumns {
	return buildAppRoleColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c appRoleColumns) Unqualified() appRoleColumns {
	return buildAppRoleColumns("")
}

func buildAppRoleColumn(alias, name string) appRoleColumn {
	return appRoleColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type appRoleColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c appRoleColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c appRoleColumn) ShouldOmitParens() bool {
	return true
}

// AppRoleSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type AppRoleSetter struct {
	ID   *int64  `db:"id,pk" `
	Name *string `db:"name" `
}

func (s AppRoleSetter) SetColumns() []string {
	vals := make([]string, 0, 2)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.Name != nil {
		vals = append(vals, "name")
	}
	return vals
}

func (s AppRoleSetter) Overwrite(t *AppRole) {
	if s.ID != nil {
		t.ID = func() int64 {
			if s.ID == nil {
				return *new(int64)
			}
			return *s.ID
		}()
	}
	if s.Name != nil {
		t.Name = func() string {
			if s.Name == nil {
				return *new(string)
			}
			return *s.Name
		}()
	}
}

func (s *AppRoleSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return AppRoles.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 2)
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

		if s.Name != nil {
			vals[1] = psql.Arg(func() string {
				if s.Name == nil {
					return *new(string)
				}
				return *s.Name
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s AppRoleSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s AppRoleSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 2)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.Name != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "name")...),
			psql.Arg(s.Name),
		}})
	}

	return exprs
}

// FindAppRole retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindAppRole(ctx context.Context, exec bob.Executor, IDPK int64, cols ...string) (*AppRole, error) {
	if len(cols) == 0 {
		return AppRoles.Query(
			sm.Where(AppRoles.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return AppRoles.Query(
		sm.Where(AppRoles.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(AppRoles.Columns.Only(cols...)),
	).One(ctx, exec)
}

// AppRoleExists checks the presence of a single record by primary key
func AppRoleExists(ctx context.Context, exec bob.Executor, IDPK int64) (bool, error) {
	return AppRoles.Query(
		sm.Where(AppRoles.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after AppRole is retrieved from the database
func (o *AppRole) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppRoles.AfterSelectHooks.RunHooks(ctx, exec, AppRoleSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = AppRoles.AfterInsertHooks.RunHooks(ctx, exec, AppRoleSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = AppRoles.AfterUpdateHooks.RunHooks(ctx, exec, AppRoleSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = AppRoles.AfterDeleteHooks.RunHooks(ctx, exec, AppRoleSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = AppRoles.AfterMergeHooks.RunHooks(ctx, exec, AppRoleSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the AppRole
func (o *AppRole) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *AppRole) pkEQ() dialect.Expression {
	return psql.Quote("app.roles", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the AppRole
func (o *AppRole) Update(ctx context.Context, exec bob.Executor, s *AppRoleSetter) error {
	v, err := AppRoles.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single AppRole record with an executor
func (o *AppRole) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := AppRoles.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the AppRole using the executor
func (o *AppRole) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := AppRoles.Query(
		sm.Where(AppRoles.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after AppRoleSlice is retrieved from the database
func (o AppRoleSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppRoles.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = AppRoles.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = AppRoles.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = AppRoles.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = AppRoles.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o AppRoleSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("app.roles", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o AppRoleSlice) copyMatchingRows(from ...*AppRole) {
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
func (o AppRoleSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppRoles.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppRole:
				o.copyMatchingRows(retrieved)
			case []*AppRole:
				o.copyMatchingRows(retrieved...)
			case AppRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppRole or a slice of AppRole
				// then run the AfterUpdateHooks on the slice
				_, err = AppRoles.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o AppRoleSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppRoles.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppRole:
				o.copyMatchingRows(retrieved)
			case []*AppRole:
				o.copyMatchingRows(retrieved...)
			case AppRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppRole or a slice of AppRole
				// then run the AfterDeleteHooks on the slice
				_, err = AppRoles.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o AppRoleSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppRoles.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppRole:
				o.copyMatchingRows(retrieved)
			case []*AppRole:
				o.copyMatchingRows(retrieved...)
			case AppRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppRole or a slice of AppRole
				// then run the AfterMergeHooks on the slice
				_, err = AppRoles.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o AppRoleSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals AppRoleSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppRoles.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o AppRoleSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppRoles.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o AppRoleSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := AppRoles.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// Users starts a query for related objects on app.users
func (o *AppRole) Users(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
	return AppUsers.Query(append(mods,
		sm.InnerJoin(AppUserRoles.NameAs()).On(
			AppUsers.Columns.ID.EQ(AppUserRoles.Columns.UserID)),
		sm.Where(AppUserRoles.Columns.RoleID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os AppRoleSlice) Users(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
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
		sm.InnerJoin(AppUserRoles.NameAs()).On(
			AppUsers.Columns.ID.EQ(AppUserRoles.Columns.UserID),
		),
		sm.Where(psql.Group(AppUserRoles.Columns.RoleID).OP("IN", PKArgExpr)),
	)...)
}

func attachAppRoleUsers0(ctx context.Context, exec bob.Executor, count int, appRole0 *AppRole, appUsers2 AppUserSlice) (AppUserRoleSlice, error) {
	setters := make([]*AppUserRoleSetter, count)
	for i := range count {
		setters[i] = &AppUserRoleSetter{
			RoleID: func() *int64 { return &appRole0.ID }(),
			UserID: func() *int64 { return &appUsers2[i].ID }(),
		}
	}

	appUserRoles1, err := AppUserRoles.Insert(bob.ToMods(setters...)).All(ctx, exec)
	if err != nil {
		return nil, fmt.Errorf("attachAppRoleUsers0: %w", err)
	}

	return appUserRoles1, nil
}

func (appRole0 *AppRole) InsertUsers(ctx context.Context, exec bob.Executor, related ...*AppUserSetter) error {
	if len(related) == 0 {
		return nil
	}

	var err error

	inserted, err := AppUsers.Insert(bob.ToMods(related...)).All(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}
	appUsers2 := AppUserSlice(inserted)

	_, err = attachAppRoleUsers0(ctx, exec, len(related), appRole0, appUsers2)
	if err != nil {
		return err
	}

	appRole0.R.Users = append(appRole0.R.Users, appUsers2...)

	for _, rel := range appUsers2 {
		rel.R.Roles = append(rel.R.Roles, appRole0)
	}
	return nil
}

func (appRole0 *AppRole) AttachUsers(ctx context.Context, exec bob.Executor, related ...*AppUser) error {
	if len(related) == 0 {
		return nil
	}

	var err error
	appUsers2 := AppUserSlice(related)

	_, err = attachAppRoleUsers0(ctx, exec, len(related), appRole0, appUsers2)
	if err != nil {
		return err
	}

	appRole0.R.Users = append(appRole0.R.Users, appUsers2...)

	for _, rel := range related {
		rel.R.Roles = append(rel.R.Roles, appRole0)
	}

	return nil
}

type appRoleWhere[Q psql.Filterable] struct {
	ID   psql.WhereMod[Q, int64]
	Name psql.WhereMod[Q, string]
}

func (appRoleWhere[Q]) AliasedAs(alias string) appRoleWhere[Q] {
	return buildAppRoleWhere[Q](buildAppRoleColumns(alias))
}

func buildAppRoleWhere[Q psql.Filterable](cols appRoleColumns) appRoleWhere[Q] {
	return appRoleWhere[Q]{
		ID:   psql.Where[Q, int64](cols.ID.Expression),
		Name: psql.Where[Q, string](cols.Name.Expression),
	}
}

func (o *AppRole) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "Users":
		rels, ok := retrieved.(AppUserSlice)
		if !ok {
			return fmt.Errorf("appRole cannot load %T as %q", retrieved, name)
		}

		o.R.Users = rels
		o.R.Loaded.Users = true

		for _, rel := range rels {
			if rel != nil {
				rel.R.Roles = AppRoleSlice{o}
			}
		}
		return nil
	default:
		return fmt.Errorf("appRole has no relationship %q", name)
	}
}

type appRolePreloader struct{}

func buildAppRolePreloader() appRolePreloader {
	return appRolePreloader{}
}

type appRoleThenLoader[Q orm.Loadable] struct {
	Users func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildAppRoleThenLoader[Q orm.Loadable]() appRoleThenLoader[Q] {
	type UsersLoadInterface interface {
		LoadUsers(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return appRoleThenLoader[Q]{
		Users: thenLoadBuilder[Q](
			"Users",
			func(ctx context.Context, exec bob.Executor, retrieved UsersLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadUsers(ctx, exec, mods...)
			},
		),
	}
}

// LoadUsers loads the appRole's Users into the .R struct
func (o *AppRole) LoadUsers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	// Reset the relationship
	o.R.Users = nil
	o.R.Loaded.Users = false

	related, err := o.Users(mods...).All(ctx, exec)
	if err != nil {
		return err
	}

	for _, rel := range related {
		rel.R.Roles = AppRoleSlice{o}
	}

	o.R.Users = related
	o.R.Loaded.Users = true
	return nil
}

// LoadUsers loads the appRole's Users into the .R struct
func (os AppRoleSlice) LoadUsers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	// since we are changing the columns, we need to check if the original columns were set or add the defaults
	sq := dialect.SelectQuery{}
	for _, mod := range mods {
		mod.Apply(&sq)
	}

	if len(sq.SelectList.Columns) == 0 {
		mods = append(mods, sm.Columns(AppUsers.Columns))
	}

	q := os.Users(append(
		mods,
		sm.Columns(AppUserRoles.Columns.RoleID.As("related_app.roles.ID")),
	)...)

	IDSlice := []int64{}

	mapper := scan.Mod(scan.StructMapper[*AppUser](), func(ctx context.Context, cols []string) (scan.BeforeFunc, func(any, any) error) {
		return func(row *scan.Row) (any, error) {
				IDSlice = append(IDSlice, *new(int64))
				row.ScheduleScanByName("related_app.roles.ID", &IDSlice[len(IDSlice)-1])

				return nil, nil
			},
			func(any, any) error {
				return nil
			}
	})

	appUsers, err := bob.Allx[bob.SliceTransformer[*AppUser, AppUserSlice]](ctx, exec, q, mapper)
	if err != nil {
		return err
	}

	for _, o := range os {
		o.R.Users = nil
		o.R.Loaded.Users = true
	}

	for _, o := range os {
		for i, rel := range appUsers {
			if !(o.ID == IDSlice[i]) {
				continue
			}

			rel.R.Roles = append(rel.R.Roles, o)

			o.R.Users = append(o.R.Users, rel)
		}
	}

	return nil
}

// appRoleC is where relationship counts are stored.
type appRoleC struct {
	Users *int64
}

// PreloadCount sets a count in the C struct by name
func (o *AppRole) PreloadCount(name string, count int64) error {
	if o == nil {
		return nil
	}

	switch name {
	case "Users":
		o.C.Users = &count
	}
	return nil
}

type appRoleCountPreloader struct {
	Users func(...bob.Mod[*dialect.SelectQuery]) psql.Preloader
}

func buildAppRoleCountPreloader() appRoleCountPreloader {
	return appRoleCountPreloader{
		Users: func(mods ...bob.Mod[*dialect.SelectQuery]) psql.Preloader {
			return countPreloader[*AppRole]("Users", func(parent string) bob.Expression {
				// Build a correlated subquery: (SELECT COUNT(*) FROM related WHERE fk = parent.pk)
				if parent == "" {
					parent = AppRoles.Alias()
				}

				subqueryMods := []bob.Mod[*dialect.SelectQuery]{
					sm.Columns(psql.Raw("count(*)")),

					sm.From(AppUserRoles.Name()),
					sm.Where(psql.Quote(AppUserRoles.Alias(), "role_id").EQ(psql.Quote(parent, "id"))),
					sm.InnerJoin(AppUsers.Name()).On(
						psql.Quote(AppUsers.Alias(), "id").EQ(psql.Quote(AppUserRoles.Alias(), "user_id")),
					),
				}
				subqueryMods = append(subqueryMods, mods...)
				return psql.Group(psql.Select(subqueryMods...).Expression)
			})
		},
	}
}

type appRoleCountThenLoader[Q orm.Loadable] struct {
	Users func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildAppRoleCountThenLoader[Q orm.Loadable]() appRoleCountThenLoader[Q] {
	type UsersCountInterface interface {
		LoadCountUsers(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return appRoleCountThenLoader[Q]{
		Users: countThenLoadBuilder[Q](
			"Users",
			func(ctx context.Context, exec bob.Executor, retrieved UsersCountInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadCountUsers(ctx, exec, mods...)
			},
		),
	}
}

// LoadCountUsers loads the count of Users into the C struct
func (o *AppRole) LoadCountUsers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if o == nil {
		return nil
	}

	count, err := o.Users(mods...).Count(ctx, exec)
	if err != nil {
		return err
	}

	o.C.Users = &count
	return nil
}

// LoadCountUsers loads the count of Users for a slice in a single batch query
func (os AppRoleSlice) LoadCountUsers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
			AppUserRoles.Columns.RoleID.As("id"),
			psql.Raw("count(*) as count"),
		),
		// Multi-hop: FROM first join table, JOIN through to final related table
		sm.From(AppUserRoles.NameAs()),
		sm.InnerJoin(AppUsers.NameAs()).On(
			AppUsers.Columns.ID.EQ(AppUserRoles.Columns.UserID),
		),

		// WHERE fk IN (parent PKs)
		sm.Where(AppUserRoles.Columns.RoleID.OP("IN", PKArgExpr)),
		// GROUP BY fk columns
		sm.GroupBy(AppUserRoles.Columns.RoleID),
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
		o.C.Users = &count
	}

	return nil
}

type appRoleJoins[Q dialect.Joinable] struct {
	typ   string
	Users modAs[Q, appUserColumns]
}

func (j appRoleJoins[Q]) aliasedAs(alias string) appRoleJoins[Q] {
	return buildAppRoleJoins[Q](buildAppRoleColumns(alias), j.typ)
}

func buildAppRoleJoins[Q dialect.Joinable](cols appRoleColumns, typ string) appRoleJoins[Q] {
	return appRoleJoins[Q]{
		typ: typ,
		Users: modAs[Q, appUserColumns]{
			c: AppUsers.Columns,
			f: func(to appUserColumns) bob.Mod[Q] {
				random := strconv.FormatInt(randInt(), 10)
				mods := make(mods.QueryMods[Q], 0, 2)

				{
					to := AppUserRoles.Columns.AliasedAs(AppUserRoles.Columns.Alias() + random)
					mods = append(mods, dialect.Join[Q](typ, AppUserRoles.Name().As(to.Alias())).On(
						to.RoleID.EQ(cols.ID),
					))
				}
				{
					cols := AppUserRoles.Columns.AliasedAs(AppUserRoles.Columns.Alias() + random)
					mods = append(mods, dialect.Join[Q](typ, AppUsers.Name().As(to.Alias())).On(
						to.ID.EQ(cols.UserID),
					))
				}

				return mods
			},
		},
	}
}
