// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"fmt"
	"io"

	"github.com/aarondl/opt/null"
	"github.com/gopherex/sqld/example/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
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

// AppProfile is an object representing the database table.
type AppProfile struct {
	UserID        int64                                                         `db:"user_id,pk" `
	Bio           null.Val[string]                                              `db:"bio" `
	Address       null.Val[db.AppAddress]                                       `db:"address" `
	PrevAddresses null.Val[[]db.AppAddress]                                     `db:"prev_addresses" `
	StatusHistory null.Val[[]db.AppUserStatus]                                  `db:"status_history" `
	Owner         db.AppPerson                                                  `db:"owner" `
	ActiveDuring  null.Val[pgtype.Range[pgtype.Timestamptz]]                    `db:"active_during" `
	ValidWindow   null.Val[pgtype.Range[pgtype.Timestamptz]]                    `db:"valid_window" `
	Windows       null.Val[pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]] `db:"windows" `

	R appProfileR `db:"-" `
}

// AppProfileSlice is an alias for a slice of pointers to AppProfile.
// This should almost always be used instead of []*AppProfile.
type AppProfileSlice []*AppProfile

// AppProfiles contains methods to work with the profiles table
var AppProfiles = psql.NewTablex[*AppProfile, AppProfileSlice, *AppProfileSetter]("app", "profiles", buildAppProfileColumns("app.profiles"))

// AppProfilesQuery is a query on the profiles table
type AppProfilesQuery = *psql.ViewQuery[*AppProfile, AppProfileSlice]

// appProfileR is where relationships are stored.
type appProfileR struct {
	User *AppUser // profiles_fkey_0
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded appProfileRLoaded `db:"-" `
}

// appProfileRLoaded tracks which relationships on AppProfile have been loaded.
type appProfileRLoaded struct {
	User bool // profiles_fkey_0
}

func buildAppProfileColumns(tableName string) appProfileColumns {
	columnsExpr := expr.NewColumnsExpr(
		"user_id", "bio", "address", "prev_addresses", "status_history", "owner", "active_during", "valid_window", "windows",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return appProfileColumns{
		ColumnsExpr:   columnsExpr,
		tableAlias:    tableName,
		UserID:        buildAppProfileColumn(tableName, "user_id"),
		Bio:           buildAppProfileColumn(tableName, "bio"),
		Address:       buildAppProfileColumn(tableName, "address"),
		PrevAddresses: buildAppProfileColumn(tableName, "prev_addresses"),
		StatusHistory: buildAppProfileColumn(tableName, "status_history"),
		Owner:         buildAppProfileColumn(tableName, "owner"),
		ActiveDuring:  buildAppProfileColumn(tableName, "active_during"),
		ValidWindow:   buildAppProfileColumn(tableName, "valid_window"),
		Windows:       buildAppProfileColumn(tableName, "windows"),
	}
}

type appProfileColumns struct {
	expr.ColumnsExpr
	tableAlias    string
	UserID        appProfileColumn
	Bio           appProfileColumn
	Address       appProfileColumn
	PrevAddresses appProfileColumn
	StatusHistory appProfileColumn
	Owner         appProfileColumn
	ActiveDuring  appProfileColumn
	ValidWindow   appProfileColumn
	Windows       appProfileColumn
}

// Alias returns the current table alias for the columns set.
func (c appProfileColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (appProfileColumns) AliasedAs(tableName string) appProfileColumns {
	return buildAppProfileColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c appProfileColumns) Unqualified() appProfileColumns {
	return buildAppProfileColumns("")
}

func buildAppProfileColumn(alias, name string) appProfileColumn {
	return appProfileColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type appProfileColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c appProfileColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c appProfileColumn) ShouldOmitParens() bool {
	return true
}

// AppProfileSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type AppProfileSetter struct {
	UserID        *int64                                                         `db:"user_id,pk" `
	Bio           *null.Val[string]                                              `db:"bio" `
	Address       *null.Val[db.AppAddress]                                       `db:"address" `
	PrevAddresses *null.Val[[]db.AppAddress]                                     `db:"prev_addresses" `
	StatusHistory *null.Val[[]db.AppUserStatus]                                  `db:"status_history" `
	Owner         *db.AppPerson                                                  `db:"owner" `
	ActiveDuring  *null.Val[pgtype.Range[pgtype.Timestamptz]]                    `db:"active_during" `
	ValidWindow   *null.Val[pgtype.Range[pgtype.Timestamptz]]                    `db:"valid_window" `
	Windows       *null.Val[pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]] `db:"windows" `
}

func (s AppProfileSetter) SetColumns() []string {
	vals := make([]string, 0, 9)
	if s.UserID != nil {
		vals = append(vals, "user_id")
	}
	if s.Bio != nil {
		vals = append(vals, "bio")
	}
	if s.Address != nil {
		vals = append(vals, "address")
	}
	if s.PrevAddresses != nil {
		vals = append(vals, "prev_addresses")
	}
	if s.StatusHistory != nil {
		vals = append(vals, "status_history")
	}
	if s.Owner != nil {
		vals = append(vals, "owner")
	}
	if s.ActiveDuring != nil {
		vals = append(vals, "active_during")
	}
	if s.ValidWindow != nil {
		vals = append(vals, "valid_window")
	}
	if s.Windows != nil {
		vals = append(vals, "windows")
	}
	return vals
}

func (s AppProfileSetter) Overwrite(t *AppProfile) {
	if s.UserID != nil {
		t.UserID = func() int64 {
			if s.UserID == nil {
				return *new(int64)
			}
			return *s.UserID
		}()
	}
	if s.Bio != nil {
		t.Bio = func() null.Val[string] {
			if s.Bio == nil {
				return *new(null.Val[string])
			}
			v := s.Bio
			return *v
		}()
	}
	if s.Address != nil {
		t.Address = func() null.Val[db.AppAddress] {
			if s.Address == nil {
				return *new(null.Val[db.AppAddress])
			}
			v := s.Address
			return *v
		}()
	}
	if s.PrevAddresses != nil {
		t.PrevAddresses = func() null.Val[[]db.AppAddress] {
			if s.PrevAddresses == nil {
				return *new(null.Val[[]db.AppAddress])
			}
			v := s.PrevAddresses
			return *v
		}()
	}
	if s.StatusHistory != nil {
		t.StatusHistory = func() null.Val[[]db.AppUserStatus] {
			if s.StatusHistory == nil {
				return *new(null.Val[[]db.AppUserStatus])
			}
			v := s.StatusHistory
			return *v
		}()
	}
	if s.Owner != nil {
		t.Owner = func() db.AppPerson {
			if s.Owner == nil {
				return *new(db.AppPerson)
			}
			return *s.Owner
		}()
	}
	if s.ActiveDuring != nil {
		t.ActiveDuring = func() null.Val[pgtype.Range[pgtype.Timestamptz]] {
			if s.ActiveDuring == nil {
				return *new(null.Val[pgtype.Range[pgtype.Timestamptz]])
			}
			v := s.ActiveDuring
			return *v
		}()
	}
	if s.ValidWindow != nil {
		t.ValidWindow = func() null.Val[pgtype.Range[pgtype.Timestamptz]] {
			if s.ValidWindow == nil {
				return *new(null.Val[pgtype.Range[pgtype.Timestamptz]])
			}
			v := s.ValidWindow
			return *v
		}()
	}
	if s.Windows != nil {
		t.Windows = func() null.Val[pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]] {
			if s.Windows == nil {
				return *new(null.Val[pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]])
			}
			v := s.Windows
			return *v
		}()
	}
}

func (s *AppProfileSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return AppProfiles.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 9)
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

		if s.Bio != nil {
			vals[1] = psql.Arg(func() null.Val[string] {
				if s.Bio == nil {
					return *new(null.Val[string])
				}
				v := s.Bio
				return *v
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.Address != nil {
			vals[2] = psql.Arg(func() null.Val[db.AppAddress] {
				if s.Address == nil {
					return *new(null.Val[db.AppAddress])
				}
				v := s.Address
				return *v
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.PrevAddresses != nil {
			vals[3] = psql.Arg(func() null.Val[[]db.AppAddress] {
				if s.PrevAddresses == nil {
					return *new(null.Val[[]db.AppAddress])
				}
				v := s.PrevAddresses
				return *v
			}())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.StatusHistory != nil {
			vals[4] = psql.Arg(func() null.Val[[]db.AppUserStatus] {
				if s.StatusHistory == nil {
					return *new(null.Val[[]db.AppUserStatus])
				}
				v := s.StatusHistory
				return *v
			}())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		if s.Owner != nil {
			vals[5] = psql.Arg(func() db.AppPerson {
				if s.Owner == nil {
					return *new(db.AppPerson)
				}
				return *s.Owner
			}())
		} else {
			vals[5] = psql.Raw("DEFAULT")
		}

		if s.ActiveDuring != nil {
			vals[6] = psql.Arg(func() null.Val[pgtype.Range[pgtype.Timestamptz]] {
				if s.ActiveDuring == nil {
					return *new(null.Val[pgtype.Range[pgtype.Timestamptz]])
				}
				v := s.ActiveDuring
				return *v
			}())
		} else {
			vals[6] = psql.Raw("DEFAULT")
		}

		if s.ValidWindow != nil {
			vals[7] = psql.Arg(func() null.Val[pgtype.Range[pgtype.Timestamptz]] {
				if s.ValidWindow == nil {
					return *new(null.Val[pgtype.Range[pgtype.Timestamptz]])
				}
				v := s.ValidWindow
				return *v
			}())
		} else {
			vals[7] = psql.Raw("DEFAULT")
		}

		if s.Windows != nil {
			vals[8] = psql.Arg(func() null.Val[pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]] {
				if s.Windows == nil {
					return *new(null.Val[pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]])
				}
				v := s.Windows
				return *v
			}())
		} else {
			vals[8] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s AppProfileSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s AppProfileSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 9)

	if s.UserID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "user_id")...),
			psql.Arg(s.UserID),
		}})
	}

	if s.Bio != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "bio")...),
			psql.Arg(s.Bio),
		}})
	}

	if s.Address != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "address")...),
			psql.Arg(s.Address),
		}})
	}

	if s.PrevAddresses != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "prev_addresses")...),
			psql.Arg(s.PrevAddresses),
		}})
	}

	if s.StatusHistory != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "status_history")...),
			psql.Arg(s.StatusHistory),
		}})
	}

	if s.Owner != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "owner")...),
			psql.Arg(s.Owner),
		}})
	}

	if s.ActiveDuring != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "active_during")...),
			psql.Arg(s.ActiveDuring),
		}})
	}

	if s.ValidWindow != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "valid_window")...),
			psql.Arg(s.ValidWindow),
		}})
	}

	if s.Windows != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "windows")...),
			psql.Arg(s.Windows),
		}})
	}

	return exprs
}

// FindAppProfile retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindAppProfile(ctx context.Context, exec bob.Executor, UserIDPK int64, cols ...string) (*AppProfile, error) {
	if len(cols) == 0 {
		return AppProfiles.Query(
			sm.Where(AppProfiles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
		).One(ctx, exec)
	}

	return AppProfiles.Query(
		sm.Where(AppProfiles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
		sm.Columns(AppProfiles.Columns.Only(cols...)),
	).One(ctx, exec)
}

// AppProfileExists checks the presence of a single record by primary key
func AppProfileExists(ctx context.Context, exec bob.Executor, UserIDPK int64) (bool, error) {
	return AppProfiles.Query(
		sm.Where(AppProfiles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after AppProfile is retrieved from the database
func (o *AppProfile) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppProfiles.AfterSelectHooks.RunHooks(ctx, exec, AppProfileSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = AppProfiles.AfterInsertHooks.RunHooks(ctx, exec, AppProfileSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = AppProfiles.AfterUpdateHooks.RunHooks(ctx, exec, AppProfileSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = AppProfiles.AfterDeleteHooks.RunHooks(ctx, exec, AppProfileSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = AppProfiles.AfterMergeHooks.RunHooks(ctx, exec, AppProfileSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the AppProfile
func (o *AppProfile) primaryKeyVals() bob.Expression {
	return psql.Arg(o.UserID)
}

func (o *AppProfile) pkEQ() dialect.Expression {
	return psql.Quote("app.profiles", "user_id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the AppProfile
func (o *AppProfile) Update(ctx context.Context, exec bob.Executor, s *AppProfileSetter) error {
	v, err := AppProfiles.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single AppProfile record with an executor
func (o *AppProfile) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := AppProfiles.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the AppProfile using the executor
func (o *AppProfile) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := AppProfiles.Query(
		sm.Where(AppProfiles.Columns.UserID.EQ(psql.Arg(o.UserID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after AppProfileSlice is retrieved from the database
func (o AppProfileSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppProfiles.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = AppProfiles.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = AppProfiles.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = AppProfiles.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = AppProfiles.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o AppProfileSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("app.profiles", "user_id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o AppProfileSlice) copyMatchingRows(from ...*AppProfile) {
	for i, old := range o {
		for _, new := range from {
			if new.UserID != old.UserID {
				continue
			}
			new.R = old.R
			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o AppProfileSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppProfiles.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppProfile:
				o.copyMatchingRows(retrieved)
			case []*AppProfile:
				o.copyMatchingRows(retrieved...)
			case AppProfileSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppProfile or a slice of AppProfile
				// then run the AfterUpdateHooks on the slice
				_, err = AppProfiles.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o AppProfileSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppProfiles.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppProfile:
				o.copyMatchingRows(retrieved)
			case []*AppProfile:
				o.copyMatchingRows(retrieved...)
			case AppProfileSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppProfile or a slice of AppProfile
				// then run the AfterDeleteHooks on the slice
				_, err = AppProfiles.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o AppProfileSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AppProfiles.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AppProfile:
				o.copyMatchingRows(retrieved)
			case []*AppProfile:
				o.copyMatchingRows(retrieved...)
			case AppProfileSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AppProfile or a slice of AppProfile
				// then run the AfterMergeHooks on the slice
				_, err = AppProfiles.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o AppProfileSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals AppProfileSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppProfiles.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o AppProfileSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AppProfiles.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o AppProfileSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := AppProfiles.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// User starts a query for related objects on app.users
func (o *AppProfile) User(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
	return AppUsers.Query(append(mods,
		sm.Where(AppUsers.Columns.ID.EQ(psql.Arg(o.UserID))),
	)...)
}

func (os AppProfileSlice) User(mods ...bob.Mod[*dialect.SelectQuery]) AppUsersQuery {
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

func attachAppProfileUser0(ctx context.Context, exec bob.Executor, count int, appProfile0 *AppProfile, appUser1 *AppUser) (*AppProfile, error) {
	setter := &AppProfileSetter{
		UserID: func() *int64 { return &appUser1.ID }(),
	}

	err := appProfile0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachAppProfileUser0: %w", err)
	}

	return appProfile0, nil
}

func (appProfile0 *AppProfile) InsertUser(ctx context.Context, exec bob.Executor, related *AppUserSetter) error {
	var err error

	appUser1, err := AppUsers.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachAppProfileUser0(ctx, exec, 1, appProfile0, appUser1)
	if err != nil {
		return err
	}

	appProfile0.R.User = appUser1
	appProfile0.R.Loaded.User = true

	appUser1.R.Profile = appProfile0
	appUser1.R.Loaded.Profile = true

	return nil
}

func (appProfile0 *AppProfile) AttachUser(ctx context.Context, exec bob.Executor, appUser1 *AppUser) error {
	var err error

	_, err = attachAppProfileUser0(ctx, exec, 1, appProfile0, appUser1)
	if err != nil {
		return err
	}

	appProfile0.R.User = appUser1
	appProfile0.R.Loaded.User = true

	appUser1.R.Profile = appProfile0
	appUser1.R.Loaded.Profile = true

	return nil
}

type appProfileWhere[Q psql.Filterable] struct {
	UserID        psql.WhereMod[Q, int64]
	Bio           psql.WhereNullMod[Q, string]
	Address       psql.WhereNullMod[Q, db.AppAddress]
	PrevAddresses psql.WhereNullMod[Q, []db.AppAddress]
	StatusHistory psql.WhereNullMod[Q, []db.AppUserStatus]
	Owner         psql.WhereMod[Q, db.AppPerson]
	ActiveDuring  psql.WhereNullMod[Q, pgtype.Range[pgtype.Timestamptz]]
	ValidWindow   psql.WhereNullMod[Q, pgtype.Range[pgtype.Timestamptz]]
	Windows       psql.WhereNullMod[Q, pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]]
}

func (appProfileWhere[Q]) AliasedAs(alias string) appProfileWhere[Q] {
	return buildAppProfileWhere[Q](buildAppProfileColumns(alias))
}

func buildAppProfileWhere[Q psql.Filterable](cols appProfileColumns) appProfileWhere[Q] {
	return appProfileWhere[Q]{
		UserID:        psql.Where[Q, int64](cols.UserID.Expression),
		Bio:           psql.WhereNull[Q, string](cols.Bio.Expression),
		Address:       psql.WhereNull[Q, db.AppAddress](cols.Address.Expression),
		PrevAddresses: psql.WhereNull[Q, []db.AppAddress](cols.PrevAddresses.Expression),
		StatusHistory: psql.WhereNull[Q, []db.AppUserStatus](cols.StatusHistory.Expression),
		Owner:         psql.Where[Q, db.AppPerson](cols.Owner.Expression),
		ActiveDuring:  psql.WhereNull[Q, pgtype.Range[pgtype.Timestamptz]](cols.ActiveDuring.Expression),
		ValidWindow:   psql.WhereNull[Q, pgtype.Range[pgtype.Timestamptz]](cols.ValidWindow.Expression),
		Windows:       psql.WhereNull[Q, pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]](cols.Windows.Expression),
	}
}

func (o *AppProfile) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "User":
		rel, ok := retrieved.(*AppUser)
		if !ok {
			return fmt.Errorf("appProfile cannot load %T as %q", retrieved, name)
		}

		o.R.User = rel
		o.R.Loaded.User = true

		if rel != nil {
			rel.R.Profile = o
			rel.R.Loaded.Profile = true
		}
		return nil
	default:
		return fmt.Errorf("appProfile has no relationship %q", name)
	}
}

type appProfilePreloader struct {
	User func(...psql.PreloadOption) psql.Preloader
}

func buildAppProfilePreloader() appProfilePreloader {
	return appProfilePreloader{
		User: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*AppUser, AppUserSlice](psql.PreloadRel{
				Name: "User",
				Sides: []psql.PreloadSide{
					{
						From:        AppProfiles,
						To:          AppUsers,
						FromColumns: []string{"user_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, AppUsers.Columns.Names(), opts...)
		},
	}
}

type appProfileThenLoader[Q orm.Loadable] struct {
	User func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildAppProfileThenLoader[Q orm.Loadable]() appProfileThenLoader[Q] {
	type UserLoadInterface interface {
		LoadUser(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return appProfileThenLoader[Q]{
		User: thenLoadBuilder[Q](
			"User",
			func(ctx context.Context, exec bob.Executor, retrieved UserLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadUser(ctx, exec, mods...)
			},
		),
	}
}

// LoadUser loads the appProfile's User into the .R struct
func (o *AppProfile) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

	related.R.Profile = o
	related.R.Loaded.Profile = true

	o.R.User = related
	o.R.Loaded.User = true
	return nil
}

// LoadUser loads the appProfile's User into the .R struct
func (os AppProfileSlice) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

			rel.R.Profile = o
			rel.R.Loaded.Profile = true

			o.R.User = rel
			break
		}
	}

	return nil
}

type appProfileJoins[Q dialect.Joinable] struct {
	typ  string
	User modAs[Q, appUserColumns]
}

func (j appProfileJoins[Q]) aliasedAs(alias string) appProfileJoins[Q] {
	return buildAppProfileJoins[Q](buildAppProfileColumns(alias), j.typ)
}

func buildAppProfileJoins[Q dialect.Joinable](cols appProfileColumns, typ string) appProfileJoins[Q] {
	return appProfileJoins[Q]{
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
