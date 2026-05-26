// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"fmt"
	"io"

	"github.com/aarondl/opt/null"
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
	"github.com/yaroher/sqld/example/gen/db"
)

// Profile is an object representing the database table.
type Profile struct {
	UserID        int64                                                         `db:"user_id,pk" `
	Bio           null.Val[string]                                              `db:"bio" `
	Address       null.Val[db.AppAddress]                                       `db:"address" `
	PrevAddresses null.Val[[]db.AppAddress]                                     `db:"prev_addresses" `
	StatusHistory null.Val[[]db.AppUserStatus]                                  `db:"status_history" `
	Owner         db.AppPerson                                                  `db:"owner" `
	ActiveDuring  null.Val[pgtype.Range[pgtype.Timestamptz]]                    `db:"active_during" `
	ValidWindow   null.Val[pgtype.Range[pgtype.Timestamptz]]                    `db:"valid_window" `
	Windows       null.Val[pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]] `db:"windows" `

	R profileR `db:"-" `
}

// ProfileSlice is an alias for a slice of pointers to Profile.
// This should almost always be used instead of []*Profile.
type ProfileSlice []*Profile

// Profiles contains methods to work with the profiles table
var Profiles = psql.NewTablex[*Profile, ProfileSlice, *ProfileSetter]("app", "profiles", buildProfileColumns("profiles"))

// ProfilesQuery is a query on the profiles table
type ProfilesQuery = *psql.ViewQuery[*Profile, ProfileSlice]

// profileR is where relationships are stored.
type profileR struct {
	User *User // profiles_fkey_0
	// Loaded reports whether each relationship has been loaded.
	// A relationship's bool is set by Load*, Preload, ThenLoad, factory builds,
	// and to-one Attach/Insert operations. To-many Attach/Insert operations leave it unchanged.
	Loaded profileRLoaded `db:"-" `
}

// profileRLoaded tracks which relationships on Profile have been loaded.
type profileRLoaded struct {
	User bool // profiles_fkey_0
}

func buildProfileColumns(tableName string) profileColumns {
	columnsExpr := expr.NewColumnsExpr(
		"user_id", "bio", "address", "prev_addresses", "status_history", "owner", "active_during", "valid_window", "windows",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return profileColumns{
		ColumnsExpr:   columnsExpr,
		tableAlias:    tableName,
		UserID:        buildProfileColumn(tableName, "user_id"),
		Bio:           buildProfileColumn(tableName, "bio"),
		Address:       buildProfileColumn(tableName, "address"),
		PrevAddresses: buildProfileColumn(tableName, "prev_addresses"),
		StatusHistory: buildProfileColumn(tableName, "status_history"),
		Owner:         buildProfileColumn(tableName, "owner"),
		ActiveDuring:  buildProfileColumn(tableName, "active_during"),
		ValidWindow:   buildProfileColumn(tableName, "valid_window"),
		Windows:       buildProfileColumn(tableName, "windows"),
	}
}

type profileColumns struct {
	expr.ColumnsExpr
	tableAlias    string
	UserID        profileColumn
	Bio           profileColumn
	Address       profileColumn
	PrevAddresses profileColumn
	StatusHistory profileColumn
	Owner         profileColumn
	ActiveDuring  profileColumn
	ValidWindow   profileColumn
	Windows       profileColumn
}

// Alias returns the current table alias for the columns set.
func (c profileColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (profileColumns) AliasedAs(tableName string) profileColumns {
	return buildProfileColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c profileColumns) Unqualified() profileColumns {
	return buildProfileColumns("")
}

func buildProfileColumn(alias, name string) profileColumn {
	return profileColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type profileColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c profileColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c profileColumn) ShouldOmitParens() bool {
	return true
}

// ProfileSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type ProfileSetter struct {
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

func (s ProfileSetter) SetColumns() []string {
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

func (s ProfileSetter) Overwrite(t *Profile) {
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

func (s *ProfileSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return Profiles.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s ProfileSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s ProfileSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindProfile retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindProfile(ctx context.Context, exec bob.Executor, UserIDPK int64, cols ...string) (*Profile, error) {
	if len(cols) == 0 {
		return Profiles.Query(
			sm.Where(Profiles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
		).One(ctx, exec)
	}

	return Profiles.Query(
		sm.Where(Profiles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
		sm.Columns(Profiles.Columns.Only(cols...)),
	).One(ctx, exec)
}

// ProfileExists checks the presence of a single record by primary key
func ProfileExists(ctx context.Context, exec bob.Executor, UserIDPK int64) (bool, error) {
	return Profiles.Query(
		sm.Where(Profiles.Columns.UserID.EQ(psql.Arg(UserIDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after Profile is retrieved from the database
func (o *Profile) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Profiles.AfterSelectHooks.RunHooks(ctx, exec, ProfileSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = Profiles.AfterInsertHooks.RunHooks(ctx, exec, ProfileSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = Profiles.AfterUpdateHooks.RunHooks(ctx, exec, ProfileSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = Profiles.AfterDeleteHooks.RunHooks(ctx, exec, ProfileSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = Profiles.AfterMergeHooks.RunHooks(ctx, exec, ProfileSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the Profile
func (o *Profile) primaryKeyVals() bob.Expression {
	return psql.Arg(o.UserID)
}

func (o *Profile) pkEQ() dialect.Expression {
	return psql.Quote("profiles", "user_id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the Profile
func (o *Profile) Update(ctx context.Context, exec bob.Executor, s *ProfileSetter) error {
	v, err := Profiles.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	o.R = v.R
	*o = *v

	return nil
}

// Delete deletes a single Profile record with an executor
func (o *Profile) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := Profiles.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the Profile using the executor
func (o *Profile) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := Profiles.Query(
		sm.Where(Profiles.Columns.UserID.EQ(psql.Arg(o.UserID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}
	o2.R = o.R
	*o = *o2

	return nil
}

// AfterQueryHook is called after ProfileSlice is retrieved from the database
func (o ProfileSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Profiles.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = Profiles.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = Profiles.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = Profiles.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = Profiles.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o ProfileSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("profiles", "user_id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o ProfileSlice) copyMatchingRows(from ...*Profile) {
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
func (o ProfileSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Profiles.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Profile:
				o.copyMatchingRows(retrieved)
			case []*Profile:
				o.copyMatchingRows(retrieved...)
			case ProfileSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Profile or a slice of Profile
				// then run the AfterUpdateHooks on the slice
				_, err = Profiles.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o ProfileSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Profiles.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Profile:
				o.copyMatchingRows(retrieved)
			case []*Profile:
				o.copyMatchingRows(retrieved...)
			case ProfileSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Profile or a slice of Profile
				// then run the AfterDeleteHooks on the slice
				_, err = Profiles.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o ProfileSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Profiles.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Profile:
				o.copyMatchingRows(retrieved)
			case []*Profile:
				o.copyMatchingRows(retrieved...)
			case ProfileSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Profile or a slice of Profile
				// then run the AfterMergeHooks on the slice
				_, err = Profiles.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o ProfileSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals ProfileSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Profiles.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o ProfileSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Profiles.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o ProfileSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := Profiles.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

// User starts a query for related objects on users
func (o *Profile) User(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
	return Users.Query(append(mods,
		sm.Where(Users.Columns.ID.EQ(psql.Arg(o.UserID))),
	)...)
}

func (os ProfileSlice) User(mods ...bob.Mod[*dialect.SelectQuery]) UsersQuery {
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

func attachProfileUser0(ctx context.Context, exec bob.Executor, count int, profile0 *Profile, user1 *User) (*Profile, error) {
	setter := &ProfileSetter{
		UserID: func() *int64 { return &user1.ID }(),
	}

	err := profile0.Update(ctx, exec, setter)
	if err != nil {
		return nil, fmt.Errorf("attachProfileUser0: %w", err)
	}

	return profile0, nil
}

func (profile0 *Profile) InsertUser(ctx context.Context, exec bob.Executor, related *UserSetter) error {
	var err error

	user1, err := Users.Insert(related).One(ctx, exec)
	if err != nil {
		return fmt.Errorf("inserting related objects: %w", err)
	}

	_, err = attachProfileUser0(ctx, exec, 1, profile0, user1)
	if err != nil {
		return err
	}

	profile0.R.User = user1
	profile0.R.Loaded.User = true

	user1.R.Profile = profile0
	user1.R.Loaded.Profile = true

	return nil
}

func (profile0 *Profile) AttachUser(ctx context.Context, exec bob.Executor, user1 *User) error {
	var err error

	_, err = attachProfileUser0(ctx, exec, 1, profile0, user1)
	if err != nil {
		return err
	}

	profile0.R.User = user1
	profile0.R.Loaded.User = true

	user1.R.Profile = profile0
	user1.R.Loaded.Profile = true

	return nil
}

type profileWhere[Q psql.Filterable] struct {
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

func (profileWhere[Q]) AliasedAs(alias string) profileWhere[Q] {
	return buildProfileWhere[Q](buildProfileColumns(alias))
}

func buildProfileWhere[Q psql.Filterable](cols profileColumns) profileWhere[Q] {
	return profileWhere[Q]{
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

func (o *Profile) Preload(name string, retrieved any) error {
	if o == nil {
		return nil
	}

	switch name {
	case "User":
		rel, ok := retrieved.(*User)
		if !ok {
			return fmt.Errorf("profile cannot load %T as %q", retrieved, name)
		}

		o.R.User = rel
		o.R.Loaded.User = true

		if rel != nil {
			rel.R.Profile = o
			rel.R.Loaded.Profile = true
		}
		return nil
	default:
		return fmt.Errorf("profile has no relationship %q", name)
	}
}

type profilePreloader struct {
	User func(...psql.PreloadOption) psql.Preloader
}

func buildProfilePreloader() profilePreloader {
	return profilePreloader{
		User: func(opts ...psql.PreloadOption) psql.Preloader {
			return psql.Preload[*User, UserSlice](psql.PreloadRel{
				Name: "User",
				Sides: []psql.PreloadSide{
					{
						From:        Profiles,
						To:          Users,
						FromColumns: []string{"user_id"},
						ToColumns:   []string{"id"},
					},
				},
			}, Users.Columns.Names(), opts...)
		},
	}
}

type profileThenLoader[Q orm.Loadable] struct {
	User func(...bob.Mod[*dialect.SelectQuery]) orm.Loader[Q]
}

func buildProfileThenLoader[Q orm.Loadable]() profileThenLoader[Q] {
	type UserLoadInterface interface {
		LoadUser(context.Context, bob.Executor, ...bob.Mod[*dialect.SelectQuery]) error
	}

	return profileThenLoader[Q]{
		User: thenLoadBuilder[Q](
			"User",
			func(ctx context.Context, exec bob.Executor, retrieved UserLoadInterface, mods ...bob.Mod[*dialect.SelectQuery]) error {
				return retrieved.LoadUser(ctx, exec, mods...)
			},
		),
	}
}

// LoadUser loads the profile's User into the .R struct
func (o *Profile) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

// LoadUser loads the profile's User into the .R struct
func (os ProfileSlice) LoadUser(ctx context.Context, exec bob.Executor, mods ...bob.Mod[*dialect.SelectQuery]) error {
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

			rel.R.Profile = o
			rel.R.Loaded.Profile = true

			o.R.User = rel
			break
		}
	}

	return nil
}

type profileJoins[Q dialect.Joinable] struct {
	typ  string
	User modAs[Q, userColumns]
}

func (j profileJoins[Q]) aliasedAs(alias string) profileJoins[Q] {
	return buildProfileJoins[Q](buildProfileColumns(alias), j.typ)
}

func buildProfileJoins[Q dialect.Joinable](cols profileColumns, typ string) profileJoins[Q] {
	return profileJoins[Q]{
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
