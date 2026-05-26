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

// Role is an object representing the database table.
type Role struct {
	ID   int64  `db:"id,pk" `
	Name string `db:"name" `

	R roleR `db:"-" `

	C roleC `db:"-" `
}

// RoleSlice is an alias for a slice of pointers to Role.
// This should almost always be used instead of []*Role.
type RoleSlice []*Role

// Roles contains methods to work with the roles table
var Roles = psql.NewTablex[*Role, RoleSlice, *RoleSetter]("app", "roles", buildRoleColumns("roles"))

// RolesQuery is a query on the roles table
type RolesQuery = *psql.ViewQuery[*Role, RoleSlice]

// roleR is where relationships are stored.
type roleR struct {
	Users UserSlice // user_roles_fkey_0user_roles_fkey_1
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded roleRLoaded `db:"-" `
}

// roleRLoaded tracks which relationships on Role have been loaded.
type roleRLoaded struct {
	Users bool // user_roles_fkey_0user_roles_fkey_1
}

func buildRoleColumns(tableName string) roleColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "name",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return roleColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildRoleColumn(tableName, "id"),
		Name:        buildRoleColumn(tableName, "name"),
	}
}

type roleColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         roleColumn
	Name       roleColumn
}

// Alias returns the current table alias for the columns set.
func (c roleColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (roleColumns) AliasedAs(tableName string) roleColumns {
	return buildRoleColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c roleColumns) Unqualified() roleColumns {
	return buildRoleColumns("")
}

func buildRoleColumn(alias, name string) roleColumn {
	return roleColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type roleColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c roleColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c roleColumn) ShouldOmitParens() bool {
	return true
}

// RoleSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type RoleSetter struct {
	ID   *int64  `db:"id,pk" `
	Name *string `db:"name" `
}

func (s RoleSetter) SetColumns() []string {
	vals := make([]string, 0, 2)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.Name != nil {
		vals = append(vals, "name")
	}
	return vals
}

func (s RoleSetter) Overwrite(t *Role) {
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

func (s *RoleSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return Roles.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s RoleSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s RoleSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindRole retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindRole(ctx context.Context, exec bob.Executor, IDPK int64, cols ...string) (*Role, error) {
	if len(cols) == 0 {
		return Roles.Query(
			sm.Where(Roles.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return Roles.Query(
		sm.Where(Roles.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(Roles.Columns.Only(cols...)),
	).One(ctx, exec)
}

// RoleExists checks the presence of a single record by primary key
func RoleExists(ctx context.Context, exec bob.Executor, IDPK int64) (bool, error) {
	return Roles.Query(
		sm.Where(Roles.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after Role is retrieved from the database
func (o *Role) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Roles.AfterSelectHooks.RunHooks(ctx, exec, RoleSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = Roles.AfterInsertHooks.RunHooks(ctx, exec, RoleSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = Roles.AfterUpdateHooks.RunHooks(ctx, exec, RoleSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = Roles.AfterDeleteHooks.RunHooks(ctx, exec, RoleSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = Roles.AfterMergeHooks.RunHooks(ctx, exec, RoleSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the Role
func (o *Role) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *Role) pkEQ() dialect.Expression {
	return psql.Quote("roles", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the Role
func (o *Role) Update(ctx context.Context, exec bob.Executor, s *RoleSetter) error {
	v, err := Roles.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single Role record with an executor
func (o *Role) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := Roles.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the Role using the executor
func (o *Role) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := Roles.Query(
		sm.Where(Roles.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after RoleSlice is retrieved from the database
func (o RoleSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Roles.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = Roles.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = Roles.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = Roles.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = Roles.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o RoleSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("roles", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o RoleSlice) copyMatchingRows(from ...*Role) {
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
func (o RoleSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Roles.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Role:
				o.copyMatchingRows(retrieved)
			case []*Role:
				o.copyMatchingRows(retrieved...)
			case RoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Role or a slice of Role
				// then run the AfterUpdateHooks on the slice
				_, err = Roles.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o RoleSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Roles.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Role:
				o.copyMatchingRows(retrieved)
			case []*Role:
				o.copyMatchingRows(retrieved...)
			case RoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Role or a slice of Role
				// then run the AfterDeleteHooks on the slice
				_, err = Roles.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o RoleSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Roles.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Role:
				o.copyMatchingRows(retrieved)
			case []*Role:
				o.copyMatchingRows(retrieved...)
			case RoleSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Role or a slice of Role
				// then run the AfterMergeHooks on the slice
				_, err = Roles.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o RoleSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals RoleSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Roles.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o RoleSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Roles.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o RoleSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := Roles.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// Users starts a query for related objects on users
func (o *Role) Users(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
	return Users.Query(append(mods,
		sm.InnerJoin(UserRoles.NameAs()).On(
			Users.Columns.ID.EQ(UserRoles.Columns.UserID)),
		sm.Where(UserRoles.Columns.RoleID.EQ(psql.Arg(o.ID))),
	)...)
}

func (os RoleSlice) Users(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
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
		sm.InnerJoin(UserRoles.NameAs()).On(
			Users.Columns.ID.EQ(UserRoles.Columns.UserID),
		),
		sm.Where(psql.Group(UserRoles.Columns.RoleID).OP("IN", PKArgExpr)),
	)...)
}

func attachRoleUsers0(ctx context.Context, exec bob.Executor, count int, role0 *Role, users2 UserSlice) (UserRoleSlice, error) {
	setters := make([]*UserRoleSetter, count)
	for i := range count {
		setters[i] = &UserRoleSetter{
			RoleID: func() *int64 { return &role0.ID }(),
			UserID: func() *int64 { return &users2[i].ID }(),
		}
	}

	userRoles1, err := UserRoles.Insert(bob.ToMods(setters...)).All(ctx, exec)
	if err != nil {
		return nil, fmt.Errorf("attachRoleUsers0: %w", err)
	}

	return userRoles1, nil
}

func (role0 *Role) InsertUsers(ctx context.Context, exec bob.Executor, related ...*UserSetter) error {
	if len(related) == 0 {
		return nil
	}

	var err error

	inserted, err := Users.Insert(bob.ToMods(related...)).All(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}
	users2 := UserSlice(inserted)

	_, err = attachRoleUsers0(ctx, exec, len(related), role0, users2)
	if err != nil {
		return err
	}

	role0.R.Users = append(role0.R.Users, users2...)

	for _, rel := range users2 {
		rel.R.Roles = append(rel.R.Roles, role0)
	}
	return nil
}

func (role0 *Role) AttachUsers(ctx context.Context, exec bob.Executor, related ...*User) error {
	if len(related) == 0 {
		return nil
	}

	var err error
	users2 := UserSlice(related)

	_, err = attachRoleUsers0(ctx, exec, len(related), role0, users2)
	if err != nil {
		return err
	}

	role0.R.Users = append(role0.R.Users, users2...)

	for _, rel := range related {
		rel.R.Roles = append(rel.R.Roles, role0)
	}

	return nil
}

type roleWhere[Q psql.Filterable] struct {
	ID   psql.WhereMod[Q, int64]
	Name psql.WhereMod[Q, string]
}

func (roleWhere[Q]) AliasedAs(alias string) roleWhere[Q] {
	return buildRoleWhere[Q](buildRoleColumns(alias))
}

func buildRoleWhere[Q psql.Filterable](cols roleColumns) roleWhere[Q] {
	return roleWhere[Q]{
		ID:   psql.Where[Q, int64](cols.ID.Expression),
		Name: psql.Where[Q, string](cols.Name.Expression),
	}
}

func (o *Role) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "Users":
		rels, ok := retrieved.(UserSlice)
		if !ok {
			return fmt.Errorf("role cannot load %T as %q", retrieved, name)
		}

		o.R.Users = rels
		o.R.Loaded.Users = true

		for _, rel := range rels {
			if rel != nil {
				rel.R.Roles = RoleSlice{o}
			}
		}
		return nil
	default:
		return fmt.Errorf("role has no relationship %q", name)
	}
}

type rolePreloader struct{}

func buildRolePreloader() rolePreloader {
	return rolePreloader{}
}

type roleThenLoader[Q orm.Loadable] struct {
	Users func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildRoleThenLoader[Q orm.Loadable]() roleThenLoader[Q] {
	type UsersLoadInterface interface {
		LoadUsers(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return roleThenLoader[Q]{
		Users: thenLoadBuilder[Q](
			"Users",
			func(ctx context.Context, exec bob.Executor, retrieved UsersLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadUsers(ctx, exec, mods...)
			},
		),
	}
}

// LoadUsers loads the role's Users into the .R struct
func (o *Role) LoadUsers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
		rel.R.Roles = RoleSlice{o}
	}

	o.R.Users = related
	o.R.Loaded.Users = true
	return nil
}

// LoadUsers loads the role's Users into the .R struct
func (os RoleSlice) LoadUsers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
	if len(os) == 0 {
		return nil
	}

	// since we are changing the columns, we need to check if the original columns were set or add the defaults
	sq := dialect.SelectQuery{}
	for _, mod := range mods {
		mod.Apply(&sq)
	}

	if len(sq.SelectList.Columns) == 0 {
		mods = append(mods, sm.Columns(Users.Columns))
	}

	q := os.Users(append(
		mods,
		sm.Columns(UserRoles.Columns.RoleID.As("related_roles.ID")),
	)...)

	IDSlice := []int64{}

	mapper := scan.Mod(scan.StructMapper[*User](), func(ctx context.Context, cols []string) (scan.BeforeFunc, func(any, any) error) {
		return func(row *scan.Row) (any, error) {
				IDSlice = append(IDSlice, *new(int64))
				row.ScheduleScanByName("related_roles.ID", &IDSlice[len(IDSlice)-1])

				return nil, nil
			},
			func(any, any) error {
				return nil
			}
	})

	users, err := bob.Allx[bob.SliceTransformer[*User, UserSlice]](ctx, exec, q, mapper)
	if err != nil {
		return err
	}

	for _, o := range os {
		o.R.Users = nil
		o.R.Loaded.Users = true
	}

	for _, o := range os {
		for i, rel := range users {
			if !(o.ID == IDSlice[i]) {
				continue
			}

			rel.R.Roles = append(rel.R.Roles, o)

			o.R.Users = append(o.R.Users, rel)
		}
	}

	return nil
}

// roleC is where relationship counts are stored.
type roleC struct {
	Users *int64
}

// PreloadCount sets a count in the C struct by name
func (o *Role) PreloadCount(name string, count int64) error {
	if o == nil {
		return nil
	}

	switch name {
	case "Users":
		o.C.Users = &count
	}
	return nil
}

type roleCountPreloader struct {
	Users func(...bob.Mod[*dialect.SelectQuery]) psql.Preloader
}

func buildRoleCountPreloader() roleCountPreloader {
	return roleCountPreloader{
		Users: func(mods ...bob.Mod[*dialect.SelectQuery]) psql.Preloader {
			return countPreloader[*Role]("Users", func(parent string) bob.Expression {
				// Build a correlated subquery: (SELECT COUNT(*) FROM related WHERE fk = parent.pk)
				if parent == "" {
					parent = Roles.Alias()
				}

				subqueryMods := []bob.Mod[*dialect.SelectQuery]{
					sm.Columns(psql.Raw("count(*)")),

					sm.From(UserRoles.Name()),
					sm.Where(psql.Quote(UserRoles.Alias(), "role_id").EQ(psql.Quote(parent, "id"))),
					sm.InnerJoin(Users.Name()).On(
						psql.Quote(Users.Alias(), "id").EQ(psql.Quote(UserRoles.Alias(), "user_id")),
					),
				}
				subqueryMods = append(subqueryMods, mods...)
				return psql.Group(psql.Select(subqueryMods...).Expression)
			})
		},
	}
}

type roleCountThenLoader[Q orm.Loadable] struct {
	Users func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildRoleCountThenLoader[Q orm.Loadable]() roleCountThenLoader[Q] {
	type UsersCountInterface interface {
		LoadCountUsers(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return roleCountThenLoader[Q]{
		Users: countThenLoadBuilder[Q](
			"Users",
			func(ctx context.Context, exec bob.Executor, retrieved UsersCountInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadCountUsers(ctx, exec, mods...)
			},
		),
	}
}

// LoadCountUsers loads the count of Users into the C struct
func (o *Role) LoadCountUsers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
func (os RoleSlice) LoadCountUsers(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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
			UserRoles.Columns.RoleID.As("id"),
			psql.Raw("count(*) as count"),
		),
		// Multi-hop: FROM first join table, JOIN through to final related table
		sm.From(UserRoles.NameAs()),
		sm.InnerJoin(Users.NameAs()).On(
			Users.Columns.ID.EQ(UserRoles.Columns.UserID),
		),

		// WHERE fk IN (parent PKs)
		sm.Where(UserRoles.Columns.RoleID.OP("IN", PKArgExpr)),
		// GROUP BY fk columns
		sm.GroupBy(UserRoles.Columns.RoleID),
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

type roleJoins[Q dialect.Joinable] struct {
	typ   string
	Users modAs[Q, userColumns]
}

func (j roleJoins[Q]) aliasedAs(alias string) roleJoins[Q] {
	return buildRoleJoins[Q](buildRoleColumns(alias), j.typ)
}

func buildRoleJoins[Q dialect.Joinable](cols roleColumns, typ string) roleJoins[Q] {
	return roleJoins[Q]{
		typ: typ,
		Users: modAs[Q, userColumns]{
			c: Users.Columns,
			f: func(to userColumns) bob.Mod[Q] {
				random := strconv.FormatInt(randInt(), 10)
				mods := make(mods.QueryMods[Q], 0, 2)

				{
					to := UserRoles.Columns.AliasedAs(UserRoles.Columns.Alias() + random)
					mods = append(mods, dialect.Join[Q](typ, UserRoles.Name().As(to.Alias())).On(
						to.RoleID.EQ(cols.ID),
					))
				}
				{
					cols := UserRoles.Columns.AliasedAs(UserRoles.Columns.Alias() + random)
					mods = append(mods, dialect.Join[Q](typ, Users.Name().As(to.Alias())).On(
						to.ID.EQ(cols.UserID),
					))
				}

				return mods
			},
		},
	}
}
