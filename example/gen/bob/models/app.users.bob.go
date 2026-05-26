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

// User is an object representing the database table.
type User struct {
	ID        int64            `db:"id,pk" `
	Email     string           `db:"email" `
	Status    db.AppUserStatus `db:"status" `
	ManagerID null.Val[int64]  `db:"manager_id" `
	CreatedAt time.Time        `db:"created_at" `

	R userR `db:"-" `

	C userC `db:"-" `
}

// UserSlice is an alias for a slice of pointers to User.
// This should almost always be used instead of []*User.
type UserSlice []*User

// Users contains methods to work with the users table
var Users = psql.NewTablex[*User, UserSlice, *UserSetter]("app", "users", buildUserColumns("users"))

// UsersQuery is a query on the users table
type UsersQuery = *psql.ViewQuery[*User, UserSlice]

// userR is where relationships are stored.
type userR struct {
	Orders          OrderSlice // orders_fkey_0
	Profile         *Profile   // profiles_fkey_0
	Roles           RoleSlice  // user_roles_fkey_0user_roles_fkey_1
	Manager         *User      // users_fkey_0
	ReverseManagers UserSlice  // users_fkey_0__self_join_reverse
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded userRLoaded `db:"-" `
}

// userRLoaded tracks which relationships on User have been loaded.
type userRLoaded struct {
	Orders          bool // orders_fkey_0
	Profile         bool // profiles_fkey_0
	Roles           bool // user_roles_fkey_0user_roles_fkey_1
	Manager         bool // users_fkey_0
	ReverseManagers bool // users_fkey_0__self_join_reverse
}

func buildUserColumns(tableName string) userColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "email", "status", "manager_id", "created_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return userColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildUserColumn(tableName, "id"),
		Email:       buildUserColumn(tableName, "email"),
		Status:      buildUserColumn(tableName, "status"),
		ManagerID:   buildUserColumn(tableName, "manager_id"),
		CreatedAt:   buildUserColumn(tableName, "created_at"),
	}
}

type userColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         userColumn
	Email      userColumn
	Status     userColumn
	ManagerID  userColumn
	CreatedAt  userColumn
}

// Alias returns the current table alias for the columns set.
func (c userColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (userColumns) AliasedAs(tableName string) userColumns {
	return buildUserColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c userColumns) Unqualified() userColumns {
	return buildUserColumns("")
}

func buildUserColumn(alias, name string) userColumn {
	return userColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type userColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c userColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c userColumn) ShouldOmitParens() bool {
	return true
}

// UserSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type UserSetter struct {
	ID        *int64            `db:"id,pk" `
	Email     *string           `db:"email" `
	Status    *db.AppUserStatus `db:"status" `
	ManagerID *null.Val[int64]  `db:"manager_id" `
	CreatedAt *time.Time        `db:"created_at" `
}

func (s UserSetter) SetColumns() []string {
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

func (s UserSetter) Overwrite(t *User) {
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

func (s *UserSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return Users.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s UserSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s UserSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindUser retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindUser(ctx context.Context, exec bob.Executor, IDPK int64, cols ...string) (*User, error) {
	if len(cols) == 0 {
		return Users.Query(
			sm.Where(Users.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return Users.Query(
		sm.Where(Users.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(Users.Columns.Only(cols...)),
	).One(ctx, exec)
}

// UserExists checks the presence of a single record by primary key
func UserExists(ctx context.Context, exec bob.Executor, IDPK int64) (bool, error) {
	return Users.Query(
		sm.Where(Users.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after User is retrieved from the database
func (o *User) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Users.AfterSelectHooks.RunHooks(ctx, exec, UserSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = Users.AfterInsertHooks.RunHooks(ctx, exec, UserSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = Users.AfterUpdateHooks.RunHooks(ctx, exec, UserSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = Users.AfterDeleteHooks.RunHooks(ctx, exec, UserSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = Users.AfterMergeHooks.RunHooks(ctx, exec, UserSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the User
func (o *User) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *User) pkEQ() dialect.Expression {
	return psql.Quote("users", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the User
func (o *User) Update(ctx context.Context, exec bob.Executor, s *UserSetter) error {
	v, err := Users.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single User record with an executor
func (o *User) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := Users.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the User using the executor
func (o *User) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := Users.Query(
		sm.Where(Users.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after UserSlice is retrieved from the database
func (o UserSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Users.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = Users.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = Users.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = Users.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = Users.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o UserSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("users", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o UserSlice) copyMatchingRows(from ...*User) {
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
func (o UserSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Users.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *User:
				o.copyMatchingRows(retrieved)
			case []*User:
				o.copyMatchingRows(retrieved...)
			case UserSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a User or a slice of User
				// then run the AfterUpdateHooks on the slice
				_, err = Users.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o UserSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Users.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *User:
				o.copyMatchingRows(retrieved)
			case []*User:
				o.copyMatchingRows(retrieved...)
			case UserSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a User or a slice of User
				// then run the AfterDeleteHooks on the slice
				_, err = Users.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o UserSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Users.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *User:
				o.copyMatchingRows(retrieved)
			case []*User:
				o.copyMatchingRows(retrieved...)
			case UserSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a User or a slice of User
				// then run the AfterMergeHooks on the slice
				_, err = Users.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o UserSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals UserSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Users.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o UserSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Users.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o UserSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := Users.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// Orders starts a query for related objects on orders
func (o *User) Orders(mods ...bob.Mod[*dialect.SelectQuery]) OrdersQuery {
	return Orders.Query(append(mods,
		sm.Where(Orders.Columns.UserID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os UserSlice) Orders(mods ...bob.Mod[*dialect.SelectQuery]) OrdersQuery {
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

	return Orders.Query(append(mods,
		sm.Where(psql.Group(Orders.Columns.UserID).OP("IN", PKArgExpr)),
	)...)
}

// Profile starts a query for related objects on profiles
func (o *User) Profile(mods ...bob.Mod[*dialect.SelectQuery]) ProfilesQuery {
	return Profiles.Query(append(mods,
		sm.Where(Profiles.Columns.UserID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os UserSlice) Profile(mods ...bob.Mod[*dialect.SelectQuery]) ProfilesQuery {
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

	return Profiles.Query(append(mods,
		sm.Where(psql.Group(Profiles.Columns.UserID).OP("IN", PKArgExpr)),
	)...)
}

// Roles starts a query for related objects on roles
func (o *User) Roles(mods ...bob.Mod[*dialect.SelectQuery]) RolesQuery {
	return Roles.Query(append(mods,
		sm.InnerJoin(UserRoles.NameAs()).On(
			Roles.Columns.ID.EQ(UserRoles.Columns.RoleID)),
		sm.Where(UserRoles.Columns.UserID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os UserSlice) Roles(mods ...bob.Mod[*dialect.SelectQuery]) RolesQuery {
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

	return Roles.Query(append(mods,
		sm.InnerJoin(UserRoles.NameAs()).On(
			Roles.Columns.ID.EQ(UserRoles.Columns.RoleID),
		),
		sm.Where(psql.Group(UserRoles.Columns.UserID).OP("IN", PKArgExpr)),
	)...)
}

// Manager starts a query for related objects on users
func (o *User) Manager(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
	return Users.Query(append(mods,
		sm.Where(Users.Columns.ID.EQ(psql.Arg(o.ManagerID))),
	)...)
}

func (os UserSlice) Manager(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
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

	return Users.Query(append(mods,
		sm.Where(psql.Group(Users.Columns.ID).OP("IN", PKArgExpr)),
	)...)
}

// ReverseManagers starts a query for related objects on users
func (o *User) ReverseManagers(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
	return Users.Query(append(mods,
		sm.Where(Users.Columns.ManagerID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os UserSlice) ReverseManagers(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
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

	return Users.Query(append(mods,
		sm.Where(psql.Group(Users.Columns.ManagerID).OP("IN", PKArgExpr)),
	)...)
}

func insertUserOrders0(ctx context.Context, exec bob.Executor, orders1 []*OrderSetter, user0 *User) (OrderSlice, error) {
	for i := range orders1 {
		orders1[i].UserID = func() *int64 { return &user0.ID }()
	}

	ret, err := Orders.Insert(bob.ToMods(orders1...)).All(ctx, exec)
	if err != nil {
		return ret, fmt.Errorf("insertUserOrders0: %w", err)
	}

	return ret, nil
}

func attachUserOrders0(ctx context.Context, exec bob.Executor, count int, orders1 OrderSlice, user0 *User) (OrderSlice, error) {
	setter := &OrderSetter{
		UserID: func() *int64 { return &user0.ID }(),
	}

	err := orders1.UpdateAll(ctx, exec, *setter)
	if err != nil {
		return nil, fmt.Errorf("attachUserOrders0: %w", err)
	}

	return orders1, nil
}

func (user0 *User) InsertOrders(ctx context.Context, exec bob.Executor, related ...*OrderSetter) error {
	if len(related) == 0 {
		return nil
	}

	var err error

	orders1, err := insertUserOrders0(ctx, exec, related, user0)
	if err != nil {
		return err
	}

	user0.R.Orders = append(user0.R.Orders, orders1...)

	for _, rel := range orders1 {
		rel.R.User = user0
		rel.R.Loaded.User = true
	}
	return nil
}

func (user0 *User) AttachOrders(ctx context.Context, exec bob.Executor, related ...*Order) error {
	if len(related) == 0 {
		return nil
	}

	var err error
	orders1 := OrderSlice(related)

	_, err = attachUserOrders0(ctx, exec, len(related), orders1, user0)
	if err != nil {
		return err
	}

	user0.R.Orders = append(user0.R.Orders, orders1...)

	for _, rel := range related {
		rel.R.User = user0
		rel.R.Loaded.User = true
	}

	return nil
}

func insertUserProfile0(ctx context.Context, exec bob.Executor, profile1 *ProfileSetter, user0 *User) (*Profile, error) {
	profile1.UserID = func() *int64 { return &user0.ID }()

	ret, err := Profiles.Insert(profile1).One(ctx, exec)
	if err != nil {
		return ret, fmt.Errorf("insertUserProfile0: %w", err)
	}

	return ret, nil
}

func attachUserProfile0(ctx context.Context, exec bob.Executor, count int, profile1 *Profile, user0 *User) (*Profile, error) {
	setter := &ProfileSetter{
		UserID: func() *int64 { return &user0.ID }(),
	}

	err := profile1.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachUserProfile0: %w", err)
	}

	return profile1, nil
}

func (user0 *User) InsertProfile(ctx context.Context, exec bob.Executor, related *ProfileSetter) error {
	var err error

	profile1, err := insertUserProfile0(ctx, exec, related, user0)
	if err != nil {
		return err
	}

	user0.R.Profile = profile1
	user0.R.Loaded.Profile = true

	profile1.R.User = user0
	profile1.R.Loaded.User = true

	return nil
}

func (user0 *User) AttachProfile(ctx context.Context, exec bob.Executor, profile1 *Profile) error {
	var err error

	_, err = attachUserProfile0(ctx, exec, 1, profile1, user0)
	if err != nil {
		return err
	}

	user0.R.Profile = profile1
	user0.R.Loaded.Profile = true

	profile1.R.User = user0
	profile1.R.Loaded.User = true

	return nil
}

func attachUserRoles0(ctx context.Context, exec bob.Executor, count int, user0 *User, roles2 RoleSlice) (UserRoleSlice, error) {
	setters := make([]*UserRoleSetter, count)
	for i := range count {
		setters[i] = &UserRoleSetter{
			UserID: func() *int64 { return &user0.ID }(),
			RoleID: func() *int64 { return &roles2[i].ID }(),
		}
	}

	userRoles1, err := UserRoles.Insert(bob.ToMods(setters...)).All(ctx, exec)
	if err != nil {
		return nil, fmt.Errorf("attachUserRoles0: %w", err)
	}

	return userRoles1, nil
}

func (user0 *User) InsertRoles(ctx context.Context, exec bob.Executor, related ...*RoleSetter) error {
	if len(related) == 0 {
		return nil
	}

	var err error

	inserted, err := Roles.Insert(bob.ToMods(related...)).All(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}
	roles2 := RoleSlice(inserted)

	_, err = attachUserRoles0(ctx, exec, len(related), user0, roles2)
	if err != nil {
		return err
	}

	user0.R.Roles = append(user0.R.Roles, roles2...)

	for _, rel := range roles2 {
		rel.R.Users = append(rel.R.Users, user0)
	}
	return nil
}

func (user0 *User) AttachRoles(ctx context.Context, exec bob.Executor, related ...*Role) error {
	if len(related) == 0 {
		return nil
	}

	var err error
	roles2 := RoleSlice(related)

	_, err = attachUserRoles0(ctx, exec, len(related), user0, roles2)
	if err != nil {
		return err
	}

	user0.R.Roles = append(user0.R.Roles, roles2...)

	for _, rel := range related {
		rel.R.Users = append(rel.R.Users, user0)
	}

	return nil
}

func attachUserManager0(ctx context.Context, exec bob.Executor, count int, user0 *User, user1 *User) (*User, error) {
	setter := &UserSetter{
		ManagerID: func() *null.Val[int64] { v := null.From(user1.ID); return &v }(),
	}

	err := user0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachUserManager0: %w", err)
	}

	return user0, nil
}

func (user0 *User) InsertManager(ctx context.Context, exec bob.Executor, related *UserSetter) error {
	var err error

	user1, err := Users.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachUserManager0(ctx, exec, 1, user0, user1)
	if err != nil {
		return err
	}

	user0.R.Manager = user1
	user0.R.Loaded.Manager = true

	user1.R.ReverseManagers = append(user1.R.ReverseManagers, user0)

	return nil
}

func (user0 *User) AttachManager(ctx context.Context, exec bob.Executor, user1 *User) error {
	var err error

	_, err = attachUserManager0(ctx, exec, 1, user0, user1)
	if err != nil {
		return err
	}

	user0.R.Manager = user1
	user0.R.Loaded.Manager = true

	user1.R.ReverseManagers = append(user1.R.ReverseManagers, user0)

	return nil
}

func insertUserReverseManagers0(ctx context.Context, exec bob.Executor, users1 []*UserSetter, user0 *User) (UserSlice, error) {
	for i := range users1 {
		users1[i].ManagerID = func() *null.Val[int64] { v := null.From(user0.ID); return &v }()
	}

	ret, err := Users.Insert(bob.ToMods(users1...)).All(ctx, exec)
	if err != nil {
		return ret, fmt.Errorf("insertUserReverseManagers0: %w", err)
	}

	return ret, nil
}

func attachUserReverseManagers0(ctx context.Context, exec bob.Executor, count int, users1 UserSlice, user0 *User) (UserSlice, error) {
	setter := &UserSetter{
		ManagerID: func() *null.Val[int64] { v := null.From(user0.ID); return &v }(),
	}

	err := users1.UpdateAll(ctx, exec, *setter)
	if err != nil {
		return nil, fmt.Errorf("attachUserReverseManagers0: %w", err)
	}

	return users1, nil
}

func (user0 *User) InsertReverseManagers(ctx context.Context, exec bob.Executor, related ...*UserSetter) error {
	if len(related) == 0 {
		return nil
	}

	var err error

	users1, err := insertUserReverseManagers0(ctx, exec, related, user0)
	if err != nil {
		return err
	}

	user0.R.ReverseManagers = append(user0.R.ReverseManagers, users1...)

	for _, rel := range users1 {
		rel.R.Manager = user0
		rel.R.Loaded.Manager = true
	}
	return nil
}

func (user0 *User) AttachReverseManagers(ctx context.Context, exec bob.Executor, related ...*User) error {
	if len(related) == 0 {
		return nil
	}

	var err error
	users1 := UserSlice(related)

	_, err = attachUserReverseManagers0(ctx, exec, len(related), users1, user0)
	if err != nil {
		return err
	}

	user0.R.ReverseManagers = append(user0.R.ReverseManagers, users1...)

	for _, rel := range related {
		rel.R.Manager = user0
		rel.R.Loaded.Manager = true
	}

	return nil
}

type userWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, int64]
	Email     psql.WhereMod[Q, string]
	Status    psql.WhereMod[Q, db.AppUserStatus]
	ManagerID psql.WhereNullMod[Q, int64]
	CreatedAt psql.WhereMod[Q, time.Time]
}

func (userWhere[Q]) AliasedAs(alias string) userWhere[Q] {
	return buildUserWhere[Q](buildUserColumns(alias))
}

func buildUserWhere[Q psql.Filterable](cols userColumns) userWhere[Q] {
	return userWhere[Q]{
		ID:        psql.Where[Q, int64](cols.ID.Expression),
		Email:     psql.Where[Q, string](cols.Email.Expression),
		Status:    psql.Where[Q, db.AppUserStatus](cols.Status.Expression),
		ManagerID: psql.WhereNull[Q, int64](cols.ManagerID.Expression),
		CreatedAt: psql.Where[Q, time.Time](cols.CreatedAt.Expression),
	}
}

func (o *User) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "Orders":
		rels, ok := retrieved.(OrderSlice)
		if !ok {
			return fmt.Errorf("user cannot load %T as %q", retrieved, name)
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
		rel, ok := retrieved.(*Profile)
		if !ok {
			return fmt.Errorf("user cannot load %T as %q", retrieved, name)
		}

		o.R.Profile = rel
		o.R.Loaded.Profile = true

		if rel != nil {
			rel.R.User = o
			rel.R.Loaded.User = true
		}
		return nil
	case "Roles":
		rels, ok := retrieved.(RoleSlice)
		if !ok {
			return fmt.Errorf("user cannot load %T as %q", retrieved, name)
		}

		o.R.Roles = rels
		o.R.Loaded.Roles = true

		for _, rel := range rels {
			if rel != nil {
				rel.R.Users = UserSlice{o}
			}
		}
		return nil
	case "Manager":
		rel, ok := retrieved.(*User)
		if !ok {
			return fmt.Errorf("user cannot load %T as %q", retrieved, name)
		}

		o.R.Manager = rel
		o.R.Loaded.Manager = true

		if rel != nil {
			rel.R.ReverseManagers = UserSlice{o}
		}
		return nil
	case "ReverseManagers":
		rels, ok := retrieved.(UserSlice)
		if !ok {
			return fmt.Errorf("user cannot load %T as %q", retrieved, name)
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
		return fmt.Errorf("user has no relationship %q", name)
	}
}

type userPreloader struct {
	Profile func(...psql.PreloadOption) psql.Preloader
	Manager func(...psql.PreloadOption) psql.Preloader
}

func buildUserPreloader() userPreloader {
	return userPreloader{
		Profile: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*Profile, ProfileSlice](psql.PreloadRel{
				Name: "Profile",
				Sides: []psql.PreloadSide{
					{
						From:        Users,
						To:          Profiles,
						FromColumns: []string{"id"},
						ToColumns:   []string{"user_id"},
					},
				},
			}, Profiles.Columns.Names(), opts...)
		},
		Manager: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*User, UserSlice](psql.PreloadRel{
				Name: "Manager",
				Sides: []psql.PreloadSide{
					{
						From:        Users,
						To:          Users,
						FromColumns: []string{"manager_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, Users.Columns.Names(), opts...)
		},
	}
}

type userThenLoader[Q orm.Loadable] struct {
	Orders          func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Profile         func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Roles           func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Manager         func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	ReverseManagers func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildUserThenLoader[Q orm.Loadable]() userThenLoader[Q] {
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

	return userThenLoader[Q]{
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

// LoadOrders loads the user's Orders into the .R struct
func (o *User) LoadOrders(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

// LoadOrders loads the user's Orders into the .R struct
func (os UserSlice) LoadOrders(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	orders, err := os.Orders(mods...).All(ctx, exec)
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

		for _, rel := range orders {

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

// LoadProfile loads the user's Profile into the .R struct
func (o *User) LoadProfile(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

// LoadProfile loads the user's Profile into the .R struct
func (os UserSlice) LoadProfile(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	profiles, err := os.Profile(mods...).All(ctx, exec)
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

		for _, rel := range profiles {

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

// LoadRoles loads the user's Roles into the .R struct
func (o *User) LoadRoles(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
		rel.R.Users = UserSlice{o}
	}

	o.R.Roles = related
	o.R.Loaded.Roles = true
	return nil
}

// LoadRoles loads the user's Roles into the .R struct
func (os UserSlice) LoadRoles(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	// since we are changing the columns, we need to check if the original columns were set or add the defaults
	sq := dialect.SelectQuery{}
	for _, mod := range mods {
		mod.Apply(&sq)
	}

	if len(sq.SelectList.Columns) == 0 {
		mods = append(mods, sm.Columns(Roles.Columns))
	}

	q := os.Roles(append(
		mods,
		sm.Columns(UserRoles.Columns.UserID.As("related_users.ID")),
	)...)

	IDSlice := []int64{}

	mapper := scan.Mod(scan.StructMapper[*Role](), func(ctx context.Context, cols []string) (scan.BeforeFunc, func(any, any) error) {
		return func(row *scan.Row) (any, error) {
				IDSlice = append(IDSlice, *new(int64))
				row.ScheduleScanByName("related_users.ID", &IDSlice[len(IDSlice)-1])

				return nil, nil
			},
			func(any, any) error {
				return nil
			}
	})

	roles, err := bob.Allx[bob.SliceTransformer[*Role, RoleSlice]](ctx, exec, q, mapper)
	if err != nil {
		return err
	}

	for _, o := range os {
		o.R.Roles = nil
		o.R.Loaded.Roles = true
	}

	for _, o := range os {
		for i, rel := range roles {
			if !(o.ID == IDSlice[i]) {
				continue
			}

			rel.R.Users = append(rel.R.Users, o)

			o.R.Roles = append(o.R.Roles, rel)
		}
	}

	return nil
}

// LoadManager loads the user's Manager into the .R struct
func (o *User) LoadManager(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

	related.R.ReverseManagers = UserSlice{o}

	o.R.Manager = related
	o.R.Loaded.Manager = true
	return nil
}

// LoadManager loads the user's Manager into the .R struct
func (os UserSlice) LoadManager(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	users, err := os.Manager(mods...).All(ctx, exec)
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

		for _, rel := range users {
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

// LoadReverseManagers loads the user's ReverseManagers into the .R struct
func (o *User) LoadReverseManagers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

// LoadReverseManagers loads the user's ReverseManagers into the .R struct
func (os UserSlice) LoadReverseManagers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	users, err := os.ReverseManagers(mods...).All(ctx, exec)
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

		for _, rel := range users {

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

// userC is where relationship counts are stored.
type userC struct {
	Orders          *int64
	Roles           *int64
	ReverseManagers *int64
}

// PreloadCount sets a count in the C struct by name
func (o *User) PreloadCount(name string, count int64) error {
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

type userCountPreloader struct {
	Orders          func(...bob.Mod[*dialect.SelectQuery]) psql.Preloader
	Roles           func(...bob.Mod[*dialect.SelectQuery]) psql.Preloader
	ReverseManagers func(...bob.Mod[*dialect.SelectQuery]) psql.Preloader
}

func buildUserCountPreloader() userCountPreloader {
	return userCountPreloader{
		Orders: func(mods ...bob.Mod[*dialect.SelectQuery]) psql.Preloader {
			return countPreloader[*User]("Orders", func(parent string) bob.Expression {
				// Build a correlated subquery: (SELECT COUNT(*) FROM related WHERE fk = parent.pk)
				if parent == "" {
					parent = Users.Alias()
				}

				subqueryMods := []bob.Mod[*dialect.SelectQuery]{
					sm.Columns(psql.Raw("count(*)")),

					sm.From(Orders.Name()),
					sm.Where(psql.Quote(Orders.Alias(), "user_id").EQ(psql.Quote(parent, "id"))),
				}
				subqueryMods = append(subqueryMods, mods...)
				return psql.Group(psql.Select(subqueryMods...).Expression)
			})
		},
		Roles: func(mods ...bob.Mod[*dialect.SelectQuery]) psql.Preloader {
			return countPreloader[*User]("Roles", func(parent string) bob.Expression {
				// Build a correlated subquery: (SELECT COUNT(*) FROM related WHERE fk = parent.pk)
				if parent == "" {
					parent = Users.Alias()
				}

				subqueryMods := []bob.Mod[*dialect.SelectQuery]{
					sm.Columns(psql.Raw("count(*)")),

					sm.From(UserRoles.Name()),
					sm.Where(psql.Quote(UserRoles.Alias(), "user_id").EQ(psql.Quote(parent, "id"))),
					sm.InnerJoin(Roles.Name()).On(
						psql.Quote(Roles.Alias(), "id").EQ(psql.Quote(UserRoles.Alias(), "role_id")),
					),
				}
				subqueryMods = append(subqueryMods, mods...)
				return psql.Group(psql.Select(subqueryMods...).Expression)
			})
		},
		ReverseManagers: func(mods ...bob.Mod[*dialect.SelectQuery]) psql.Preloader {
			return countPreloader[*User]("ReverseManagers", func(parent string) bob.Expression {
				// Build a correlated subquery: (SELECT COUNT(*) FROM related WHERE fk = parent.pk)
				if parent == "" {
					parent = Users.Alias()
				}

				subqueryMods := []bob.Mod[*dialect.SelectQuery]{
					sm.Columns(psql.Raw("count(*)")),

					sm.From(Users.Name()),
					sm.Where(psql.Quote(Users.Alias(), "manager_id").EQ(psql.Quote(parent, "id"))),
				}
				subqueryMods = append(subqueryMods, mods...)
				return psql.Group(psql.Select(subqueryMods...).Expression)
			})
		},
	}
}

type userCountThenLoader[Q orm.Loadable] struct {
	Orders          func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	Roles           func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
	ReverseManagers func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildUserCountThenLoader[Q orm.Loadable]() userCountThenLoader[Q] {
	type OrdersCountInterface interface {
		LoadCountOrders(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type RolesCountInterface interface {
		LoadCountRoles(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}
	type ReverseManagersCountInterface interface {
		LoadCountReverseManagers(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return userCountThenLoader[Q]{
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
func (o *User) LoadCountOrders(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
func (os UserSlice) LoadCountOrders(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
			Orders.Columns.UserID.As("id"),
			psql.Raw("count(*) as count"),
		),
		// Single-hop: FROM related table directly
		sm.From(Orders.NameAs()),

		// WHERE fk IN (parent PKs)
		sm.Where(Orders.Columns.UserID.OP("IN", PKArgExpr)),
		// GROUP BY fk columns
		sm.GroupBy(Orders.Columns.UserID),
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
func (o *User) LoadCountRoles(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
func (os UserSlice) LoadCountRoles(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
			UserRoles.Columns.UserID.As("id"),
			psql.Raw("count(*) as count"),
		),
		// Multi-hop: FROM first join table, JOIN through to final related table
		sm.From(UserRoles.NameAs()),
		sm.InnerJoin(Roles.NameAs()).On(
			Roles.Columns.ID.EQ(UserRoles.Columns.RoleID),
		),

		// WHERE fk IN (parent PKs)
		sm.Where(UserRoles.Columns.UserID.OP("IN", PKArgExpr)),
		// GROUP BY fk columns
		sm.GroupBy(UserRoles.Columns.UserID),
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
func (o *User) LoadCountReverseManagers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
func (os UserSlice) LoadCountReverseManagers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
			Users.Columns.ManagerID.As("id"),
			psql.Raw("count(*) as count"),
		),
		// Single-hop: FROM related table directly
		sm.From(Users.NameAs()),

		// WHERE fk IN (parent PKs)
		sm.Where(Users.Columns.ManagerID.OP("IN", PKArgExpr)),
		// GROUP BY fk columns
		sm.GroupBy(Users.Columns.ManagerID),
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

type userJoins[Q dialect.Joinable] struct {
	typ             string
	Orders          modAs[Q, orderColumns]
	Profile         modAs[Q, profileColumns]
	Roles           modAs[Q, roleColumns]
	Manager         modAs[Q, userColumns]
	ReverseManagers modAs[Q, userColumns]
}

func (j userJoins[Q]) aliasedAs(alias string) userJoins[Q] {
	return buildUserJoins[Q](buildUserColumns(alias), j.typ)
}

func buildUserJoins[Q dialect.Joinable](cols userColumns, typ string) userJoins[Q] {
	return userJoins[Q]{
		typ: typ,
		Orders: modAs[Q, orderColumns]{
			c: Orders.Columns,
			f: func(to orderColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, Orders.Name().As(to.Alias())).On(
						to.UserID.EQ(cols.ID),
					))
				}

				return mods
			},
		},
		Profile: modAs[Q, profileColumns]{
			c: Profiles.Columns,
			f: func(to profileColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, Profiles.Name().As(to.Alias())).On(
						to.UserID.EQ(cols.ID),
					))
				}

				return mods
			},
		},
		Roles: modAs[Q, roleColumns]{
			c: Roles.Columns,
			f: func(to roleColumns) bob.Mod[Q] {
				random := strconv.FormatInt(randInt(), 10)
				mods := make(mods.QueryMods[Q], 0, 2)

				{
					to := UserRoles.Columns.AliasedAs(UserRoles.Columns.Alias() + random)
					mods = append(mods, dialect.Join[Q](typ, UserRoles.Name().As(to.Alias())).On(
						to.UserID.EQ(cols.ID),
					))
				}
				{
					cols := UserRoles.Columns.AliasedAs(UserRoles.Columns.Alias() + random)
					mods = append(mods, dialect.Join[Q](typ, Roles.Name().As(to.Alias())).On(
						to.ID.EQ(cols.RoleID),
					))
				}

				return mods
			},
		},
		Manager: modAs[Q, userColumns]{
			c: Users.Columns,
			f: func(to userColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, Users.Name().As(to.Alias())).On(
						to.ID.EQ(cols.ManagerID),
					))
				}

				return mods
			},
		},
		ReverseManagers: modAs[Q, userColumns]{
			c: Users.Columns,
			f: func(to userColumns) bob.Mod[Q] {
				mods := make(mods.QueryMods[Q], 0, 1)

				{
					mods = append(mods, dialect.Join[Q](typ, Users.Name().As(to.Alias())).On(
						to.ManagerID.EQ(cols.ID),
					))
				}

				return mods
			},
		},
	}
}
