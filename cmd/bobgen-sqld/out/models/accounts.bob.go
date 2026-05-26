// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"io"
	"time"

	"github.com/aarondl/opt/null"
	"github.com/aarondl/opt/omit"
	"github.com/aarondl/opt/omitnull"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/bob/expr"
	pocshared "github.com/yaroher/sqld/cmd/bobgen-sqld/pocshared"
)

// Account is an object representing the database table.
type Account struct {
	ID         int64                   `db:"id,pk" `
	Email      string                  `db:"email" `
	Status     pocshared.AccountStatus `db:"status" `
	IsVerified bool                    `db:"is_verified" `
	CreatedAt  time.Time               `db:"created_at" `
	Nickname   null.Val[string]        `db:"nickname" `
}

// AccountSlice is an alias for a slice of pointers to Account.
// This should almost always be used instead of []*Account.
type AccountSlice []*Account

// Accounts contains methods to work with the accounts table
var Accounts = psql.NewTablex[*Account, AccountSlice, *AccountSetter]("", "accounts", buildAccountColumns("accounts"))

// AccountsQuery is a query on the accounts table
type AccountsQuery = *psql.ViewQuery[*Account, AccountSlice]

func buildAccountColumns(tableName string) accountColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "email", "status", "is_verified", "created_at", "nickname",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return accountColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildAccountColumn(tableName, "id"),
		Email:       buildAccountColumn(tableName, "email"),
		Status:      buildAccountColumn(tableName, "status"),
		IsVerified:  buildAccountColumn(tableName, "is_verified"),
		CreatedAt:   buildAccountColumn(tableName, "created_at"),
		Nickname:    buildAccountColumn(tableName, "nickname"),
	}
}

type accountColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         accountColumn
	Email      accountColumn
	Status     accountColumn
	IsVerified accountColumn
	CreatedAt  accountColumn
	Nickname   accountColumn
}

// Alias returns the current table alias for the columns set.
func (c accountColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (accountColumns) AliasedAs(tableName string) accountColumns {
	return buildAccountColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c accountColumns) Unqualified() accountColumns {
	return buildAccountColumns("")
}

func buildAccountColumn(alias, name string) accountColumn {
	return accountColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type accountColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c accountColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c accountColumn) ShouldOmitParens() bool {
	return true
}

// AccountSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type AccountSetter struct {
	ID         omit.Val[int64]                   `db:"id,pk" `
	Email      omit.Val[string]                  `db:"email" `
	Status     omit.Val[pocshared.AccountStatus] `db:"status" `
	IsVerified omit.Val[bool]                    `db:"is_verified" `
	CreatedAt  omit.Val[time.Time]               `db:"created_at" `
	Nickname   omitnull.Val[string]              `db:"nickname" `
}

func (s AccountSetter) SetColumns() []string {
	vals := make([]string, 0, 6)
	if s.ID.IsValue() {
		vals = append(vals, "id")
	}
	if s.Email.IsValue() {
		vals = append(vals, "email")
	}
	if s.Status.IsValue() {
		vals = append(vals, "status")
	}
	if s.IsVerified.IsValue() {
		vals = append(vals, "is_verified")
	}
	if s.CreatedAt.IsValue() {
		vals = append(vals, "created_at")
	}
	if !s.Nickname.IsUnset() {
		vals = append(vals, "nickname")
	}
	return vals
}

func (s AccountSetter) Overwrite(t *Account) {
	if s.ID.IsValue() {
		t.ID = s.ID.MustGet()
	}
	if s.Email.IsValue() {
		t.Email = s.Email.MustGet()
	}
	if s.Status.IsValue() {
		t.Status = s.Status.MustGet()
	}
	if s.IsVerified.IsValue() {
		t.IsVerified = s.IsVerified.MustGet()
	}
	if s.CreatedAt.IsValue() {
		t.CreatedAt = s.CreatedAt.MustGet()
	}
	if !s.Nickname.IsUnset() {
		t.Nickname = s.Nickname.MustGetNull()
	}
}

func (s *AccountSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return Accounts.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 6)
		if s.ID.IsValue() {
			vals[0] = psql.Arg(s.ID.MustGet())
		} else {
			vals[0] = psql.Raw("DEFAULT")
		}

		if s.Email.IsValue() {
			vals[1] = psql.Arg(s.Email.MustGet())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.Status.IsValue() {
			vals[2] = psql.Arg(s.Status.MustGet())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		if s.IsVerified.IsValue() {
			vals[3] = psql.Arg(s.IsVerified.MustGet())
		} else {
			vals[3] = psql.Raw("DEFAULT")
		}

		if s.CreatedAt.IsValue() {
			vals[4] = psql.Arg(s.CreatedAt.MustGet())
		} else {
			vals[4] = psql.Raw("DEFAULT")
		}

		if !s.Nickname.IsUnset() {
			vals[5] = psql.Arg(s.Nickname.MustGetNull())
		} else {
			vals[5] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s AccountSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s AccountSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 6)

	if s.ID.IsValue() {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.Email.IsValue() {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "email")...),
			psql.Arg(s.Email),
		}})
	}

	if s.Status.IsValue() {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "status")...),
			psql.Arg(s.Status),
		}})
	}

	if s.IsVerified.IsValue() {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "is_verified")...),
			psql.Arg(s.IsVerified),
		}})
	}

	if s.CreatedAt.IsValue() {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "created_at")...),
			psql.Arg(s.CreatedAt),
		}})
	}

	if !s.Nickname.IsUnset() {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "nickname")...),
			psql.Arg(s.Nickname),
		}})
	}

	return exprs
}

// FindAccount retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindAccount(ctx context.Context, exec bob.Executor, IDPK int64, cols ...string) (*Account, error) {
	if len(cols) == 0 {
		return Accounts.Query(
			sm.Where(Accounts.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return Accounts.Query(
		sm.Where(Accounts.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(Accounts.Columns.Only(cols...)),
	).One(ctx, exec)
}

// AccountExists checks the presence of a single record by primary key
func AccountExists(ctx context.Context, exec bob.Executor, IDPK int64) (bool, error) {
	return Accounts.Query(
		sm.Where(Accounts.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after Account is retrieved from the database
func (o *Account) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Accounts.AfterSelectHooks.RunHooks(ctx, exec, AccountSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = Accounts.AfterInsertHooks.RunHooks(ctx, exec, AccountSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = Accounts.AfterUpdateHooks.RunHooks(ctx, exec, AccountSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = Accounts.AfterDeleteHooks.RunHooks(ctx, exec, AccountSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = Accounts.AfterMergeHooks.RunHooks(ctx, exec, AccountSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the Account
func (o *Account) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *Account) pkEQ() dialect.Expression {
	return psql.Quote("accounts", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the Account
func (o *Account) Update(ctx context.Context, exec bob.Executor, s *AccountSetter) error {
	v, err := Accounts.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single Account record with an executor
func (o *Account) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := Accounts.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the Account using the executor
func (o *Account) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := Accounts.Query(
		sm.Where(Accounts.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after AccountSlice is retrieved from the database
func (o AccountSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Accounts.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = Accounts.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = Accounts.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = Accounts.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = Accounts.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o AccountSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("accounts", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o AccountSlice) copyMatchingRows(from ...*Account) {
	for i, old := range o {
		for _, new := range from {
			if new.ID != old.ID {
				continue
			}

			o[i] = new
			break
		}
	}
}

// UpdateMod modifies an update query with "WHERE primary_key IN (o...)"
func (o AccountSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Accounts.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Account:
				o.copyMatchingRows(retrieved)
			case []*Account:
				o.copyMatchingRows(retrieved...)
			case AccountSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Account or a slice of Account
				// then run the AfterUpdateHooks on the slice
				_, err = Accounts.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o AccountSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Accounts.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Account:
				o.copyMatchingRows(retrieved)
			case []*Account:
				o.copyMatchingRows(retrieved...)
			case AccountSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Account or a slice of Account
				// then run the AfterDeleteHooks on the slice
				_, err = Accounts.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o AccountSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Accounts.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Account:
				o.copyMatchingRows(retrieved)
			case []*Account:
				o.copyMatchingRows(retrieved...)
			case AccountSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Account or a slice of Account
				// then run the AfterMergeHooks on the slice
				_, err = Accounts.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o AccountSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals AccountSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Accounts.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o AccountSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Accounts.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o AccountSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := Accounts.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}
