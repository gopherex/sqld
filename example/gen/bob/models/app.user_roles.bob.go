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

// UserRole is an object representing the database table.
type UserRole struct {
	UserID int64 `db:"user_id,pk" `
	RoleID int64 `db:"role_id,pk" `

	R userRoleR `db:"-" `
}

// UserRoleSlice is an alias for a slice of pointers to UserRole.
// This should almost always be used instead of []*UserRole.
type UserRoleSlice []*UserRole

// UserRoles contains methods to work with the user_roles table
var UserRoles = psql.NewTablex[*UserRole, UserRoleSlice, *UserRoleSetter]("app", "user_roles", buildUserRoleColumns("user_roles"))

// UserRolesQuery is a query on the user_roles table
type UserRolesQuery = *psql.ViewQuery[*UserRole, UserRoleSlice]

// userRoleR is where relationships are stored.
type userRoleR struct {
	User *User // user_roles_fkey_0
	Role *Role // user_roles_fkey_1
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded userRoleRLoaded `db:"-" `
}

// userRoleRLoaded tracks which relationships on UserRole have been loaded.
type userRoleRLoaded struct {
	User bool // user_roles_fkey_0
	Role bool // user_roles_fkey_1
}

func buildUserRoleColumns(tableName string) userRoleColumns {
	columnsExpr := expr.NewColumnsExpr(
		"user_id", "role_id",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return userRoleColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		UserID:      buildUserRoleColumn(tableName, "user_id"),
		RoleID:      buildUserRoleColumn(tableName, "role_id"),
	}
}

type userRoleColumns struct {
	expr.ColumnsExpr
	tableAlias string
	UserID     userRoleColumn
	RoleID     userRoleColumn
}

// Alias returns the current table alias for the columns set.
func (c userRoleColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (userRoleColumns) AliasedAs(tableName string) userRoleColumns {
	return buildUserRoleColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c userRoleColumns) Unqualified() userRoleColumns {
	return buildUserRoleColumns("")
}

func buildUserRoleColumn(alias, name string) userRoleColumn {
	return userRoleColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type userRoleColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c userRoleColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c userRoleColumn) ShouldOmitParens() bool {
	return true
}

// UserRoleSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type UserRoleSetter struct {
	UserID *int64 `db:"user_id,pk" `
	RoleID *int64 `db:"role_id,pk" `
}

func (s UserRoleSetter) SetColumns() []string {
	vals := make([]string, 0, 2)
	if s.UserID != nil {
		vals = append(vals, "user_id")
	}
	if s.RoleID != nil {
		vals = append(vals, "role_id")
	}
	return vals
}

func (s UserRoleSetter) Overwrite(t *UserRole) {
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

func (s *UserRoleSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return UserRoles.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s UserRoleSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s UserRoleSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindUserRole retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindUserRole(ctx context.Context, exec bob.Executor, UserIDPK int64, RoleIDPK int64, cols ...string) (*UserRole, error) {
	if len(cols) == 0 {
		return UserRoles.Query(
			sm.Where(UserRoles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
			sm.Where(UserRoles.Columns.RoleID.EQ(psql.Arg(RoleIDPK))),
		).One(ctx, exec)
	}

	return UserRoles.Query(
		sm.Where(UserRoles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
		sm.Where(UserRoles.Columns.RoleID.EQ(psql.Arg(RoleIDPK))),
		sm.Columns(UserRoles.Columns.Only(cols...)),
	).One(ctx, exec)
}

// UserRoleExists checks the presence of a single record by primary key
func UserRoleExists(ctx context.Context, exec bob.Executor, UserIDPK int64, RoleIDPK int64) (bool, error) {
	return UserRoles.Query(
		sm.Where(UserRoles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
		sm.Where(UserRoles.Columns.RoleID.EQ(psql.Arg(RoleIDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after UserRole is retrieved from the database
func (o *UserRole) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = UserRoles.AfterSelectHooks.RunHooks(ctx, exec, UserRoleSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = UserRoles.AfterInsertHooks.RunHooks(ctx, exec, UserRoleSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = UserRoles.AfterUpdateHooks.RunHooks(ctx, exec, UserRoleSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = UserRoles.AfterDeleteHooks.RunHooks(ctx, exec, UserRoleSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = UserRoles.AfterMergeHooks.RunHooks(ctx, exec, UserRoleSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the UserRole
func (o *UserRole) primaryKeyVals() bob.Expression {
	return psql.ArgGroup(
		o.UserID,
		o.RoleID,
	)
}

func (o *UserRole) pkEQ() dialect.Expression {
	return psql.Group(psql.Quote("user_roles", "user_id"), psql.Quote("user_roles", "role_id")).EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the UserRole
func (o *UserRole) Update(ctx context.Context, exec bob.Executor, s *UserRoleSetter) error {
	v, err := UserRoles.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single UserRole record with an executor
func (o *UserRole) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := UserRoles.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the UserRole using the executor
func (o *UserRole) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := UserRoles.Query(
		sm.Where(UserRoles.Columns.UserID.EQ(psql.Arg(o.UserID))),
		sm.Where(UserRoles.Columns.RoleID.EQ(psql.Arg(o.RoleID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after UserRoleSlice is retrieved from the database
func (o UserRoleSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = UserRoles.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = UserRoles.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = UserRoles.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = UserRoles.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = UserRoles.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o UserRoleSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Group(psql.Quote("user_roles", "user_id"), psql.Quote("user_roles", "role_id")).In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o UserRoleSlice) copyMatchingRows(from ...*UserRole) {
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
func (o UserRoleSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return UserRoles.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *UserRole:
				o.copyMatchingRows(retrieved)
			case []*UserRole:
				o.copyMatchingRows(retrieved...)
			case UserRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a UserRole or a slice of UserRole
				// then run the AfterUpdateHooks on the slice
				_, err = UserRoles.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o UserRoleSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return UserRoles.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *UserRole:
				o.copyMatchingRows(retrieved)
			case []*UserRole:
				o.copyMatchingRows(retrieved...)
			case UserRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a UserRole or a slice of UserRole
				// then run the AfterDeleteHooks on the slice
				_, err = UserRoles.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o UserRoleSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return UserRoles.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *UserRole:
				o.copyMatchingRows(retrieved)
			case []*UserRole:
				o.copyMatchingRows(retrieved...)
			case UserRoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a UserRole or a slice of UserRole
				// then run the AfterMergeHooks on the slice
				_, err = UserRoles.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o UserRoleSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals UserRoleSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := UserRoles.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o UserRoleSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := UserRoles.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o UserRoleSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := UserRoles.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// User starts a query for related objects on users
func (o *UserRole) User(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
	return Users.Query(append(mods,
		sm.Where(Users.Columns.ID.EQ(psql.Arg(o.UserID))),
	)...)
}

func (os UserRoleSlice) User(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
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

// Role starts a query for related objects on roles
func (o *UserRole) Role(mods ...bob.Mod[*dialect.SelectQuery]) RolesQuery {
	return Roles.Query(append(mods,
		sm.Where(Roles.Columns.ID.EQ(psql.Arg(o.RoleID))),
	)...)
}

func (os UserRoleSlice) Role(mods ...bob.Mod[*dialect.SelectQuery]) RolesQuery {
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

	return Roles.Query(append(mods,
		sm.Where(psql.Group(Roles.Columns.ID).OP("IN", PKArgExpr)),
	)...)
}

func attachUserRoleUser0(ctx context.Context, exec bob.Executor, count int, userRole0 *UserRole, user1 *User) (*UserRole, error) {
	setter := &UserRoleSetter{
		UserID: func() *int64 { return &user1.ID }(),
	}

	err := userRole0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachUserRoleUser0: %w", err)
	}

	return userRole0, nil
}

func (userRole0 *UserRole) InsertUser(ctx context.Context, exec bob.Executor, related *UserSetter) error {
	var err error

	user1, err := Users.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachUserRoleUser0(ctx, exec, 1, userRole0, user1)
	if err != nil {
		return err
	}

	userRole0.R.User = user1
	userRole0.R.Loaded.User = true

	return nil
}

func (userRole0 *UserRole) AttachUser(ctx context.Context, exec bob.Executor, user1 *User) error {
	var err error

	_, err = attachUserRoleUser0(ctx, exec, 1, userRole0, user1)
	if err != nil {
		return err
	}

	userRole0.R.User = user1
	userRole0.R.Loaded.User = true

	return nil
}

func attachUserRoleRole0(ctx context.Context, exec bob.Executor, count int, userRole0 *UserRole, role1 *Role) (*UserRole, error) {
	setter := &UserRoleSetter{
		RoleID: func() *int64 { return &role1.ID }(),
	}

	err := userRole0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachUserRoleRole0: %w", err)
	}

	return userRole0, nil
}

func (userRole0 *UserRole) InsertRole(ctx context.Context, exec bob.Executor, related *RoleSetter) error {
	var err error

	role1, err := Roles.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachUserRoleRole0(ctx, exec, 1, userRole0, role1)
	if err != nil {
		return err
	}

	userRole0.R.Role = role1
	userRole0.R.Loaded.Role = true

	return nil
}

func (userRole0 *UserRole) AttachRole(ctx context.Context, exec bob.Executor, role1 *Role) error {
	var err error

	_, err = attachUserRoleRole0(ctx, exec, 1, userRole0, role1)
	if err != nil {
		return err
	}

	userRole0.R.Role = role1
	userRole0.R.Loaded.Role = true

	return nil
}

type userRoleWhere[Q psql.Filterable] struct {
	UserID psql.WhereMod[Q, int64]
	RoleID psql.WhereMod[Q, int64]
}

func (userRoleWhere[Q]) AliasedAs(alias string) userRoleWhere[Q] {
	return buildUserRoleWhere[Q](buildUserRoleColumns(alias))
}

func buildUserRoleWhere[Q psql.Filterable](cols userRoleColumns) userRoleWhere[Q] {
	return userRoleWhere[Q]{
		UserID: psql.Where[Q, int64](cols.UserID.Expression),
		RoleID: psql.Where[Q, int64](cols.RoleID.Expression),
	}
}

func (o *UserRole) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "User":
		rel, ok := retrieved.(*User)
		if !ok {
			return fmt.Errorf("userRole cannot load %T as %q", retrieved, name)
		}

		o.R.User = rel
		o.R.Loaded.User = true

		return nil
	case "Role":
		rel, ok := retrieved.(*Role)
		if !ok {
			return fmt.Errorf("userRole cannot load %T as %q", retrieved, name)
		}

		o.R.Role = rel
		o.R.Loaded.Role = true

		return nil
	default:
		return fmt.Errorf("userRole has no relationship %q", name)
	}
}

type userRolePreloader struct {
	User func(...psql.PreloadOption) psql.Preloader
	Role func(...psql.PreloadOption) psql.Preloader
}

func buildUserRolePreloader() userRolePreloader {
	return userRolePreloader{
		User: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*User, UserSlice](psql.PreloadRel{
				Name: "User",
				Sides: []psql.PreloadSide{
					{
						From:        UserRoles,
						To:          Users,
						FromColumns: []string{"user_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, Users.Columns.Names(), opts...)
		},
		Role: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*Role, RoleSlice](psql.PreloadRel{
				Name: "Role",
				Sides: []psql.PreloadSide{
					{
						From:        UserRoles,
						To:          Roles,
						FromColumns: []string{"role_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, Roles.Columns.Names(), opts...)
		},
	}
}

type userRoleThenLoader[Q orm.Loadable] struct {
	User func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Role func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildUserRoleThenLoader[Q orm.Loadable]() userRoleThenLoader[Q] {
	type UserLoadInterface interface {
		LoadUser(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type RoleLoadInterface interface {
		LoadRole(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return userRoleThenLoader[Q]{
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

// LoadUser loads the userRole's User into the .R struct
func (o *UserRole) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

// LoadUser loads the userRole's User into the .R struct
func (os UserRoleSlice) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

			o.R.User = rel
			break
		}
	}

	return nil
}

// LoadRole loads the userRole's Role into the .R struct
func (o *UserRole) LoadRole(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

// LoadRole loads the userRole's Role into the .R struct
func (os UserRoleSlice) LoadRole(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	roles, err := os.Role(mods...).All(ctx, exec)
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

		for _, rel := range roles {

			if !(o.RoleID == rel.ID) {
				continue
			}

			o.R.Role = rel
			break
		}
	}

	return nil
}

type userRoleJoins[Q dialect.Joinable] struct {
	typ  string
	User modAs[Q, userColumns]
	Role modAs[Q, roleColumns]
}

func (j userRoleJoins[Q]) aliasedAs(alias string) userRoleJoins[Q] {
	return buildUserRoleJoins[Q](buildUserRoleColumns(alias), j.typ)
}

func buildUserRoleJoins[Q dialect.Joinable](cols userRoleColumns, typ string) userRoleJoins[Q] {
	return userRoleJoins[Q]{
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
		Role: modAs[Q, roleColumns]{
			c: Roles.Columns,
			f: func(to roleColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, Roles.Name().As(to.Alias())).On(
						to.ID.EQ(cols.RoleID),
					))
				}

				return mods
			},
		},
	}
}
