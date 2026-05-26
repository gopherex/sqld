// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"io"
	"time"

	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/dialect/psql/dialect"
	"github.com/stephenafamo/bob/dialect/psql/dm"
	"github.com/stephenafamo/bob/dialect/psql/sm"
	"github.com/stephenafamo/bob/dialect/psql/um"
	"github.com/stephenafamo/bob/expr"
)

// Log is an object representing the database table.
type Log struct {
	ID        int64     `db:"id,pk" `
	TableName string    `db:"table_name" `
	ChangedAt time.Time `db:"changed_at" `
}

// LogSlice is an alias for a slice of pointers to Log.
// This should almost always be used instead of []*Log.
type LogSlice []*Log

// Logs contains methods to work with the log table
var Logs = psql.NewTablex[*Log, LogSlice, *LogSetter]("audit", "log", buildLogColumns("log"))

// LogsQuery is a query on the log table
type LogsQuery = *psql.ViewQuery[*Log, LogSlice]

func buildLogColumns(tableName string) logColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "table_name", "changed_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return logColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildLogColumn(tableName, "id"),
		TableName:   buildLogColumn(tableName, "table_name"),
		ChangedAt:   buildLogColumn(tableName, "changed_at"),
	}
}

type logColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         logColumn
	TableName  logColumn
	ChangedAt  logColumn
}

// Alias returns the current table alias for the columns set.
func (c logColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (logColumns) AliasedAs(tableName string) logColumns {
	return buildLogColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c logColumns) Unqualified() logColumns {
	return buildLogColumns("")
}

func buildLogColumn(alias, name string) logColumn {
	return logColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type logColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c logColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c logColumn) ShouldOmitParens() bool {
	return true
}

// LogSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type LogSetter struct {
	ID        *int64     `db:"id,pk" `
	TableName *string    `db:"table_name" `
	ChangedAt *time.Time `db:"changed_at" `
}

func (s LogSetter) SetColumns() []string {
	vals := make([]string, 0, 3)
	if s.ID != nil {
		vals = append(vals, "id")
	}
	if s.TableName != nil {
		vals = append(vals, "table_name")
	}
	if s.ChangedAt != nil {
		vals = append(vals, "changed_at")
	}
	return vals
}

func (s LogSetter) Overwrite(t *Log) {
	if s.ID != nil {
		t.ID = func() int64 {
			if s.ID == nil {
				return *new(int64)
			}
			return *s.ID
		}()
	}
	if s.TableName != nil {
		t.TableName = func() string {
			if s.TableName == nil {
				return *new(string)
			}
			return *s.TableName
		}()
	}
	if s.ChangedAt != nil {
		t.ChangedAt = func() time.Time {
			if s.ChangedAt == nil {
				return *new(time.Time)
			}
			return *s.ChangedAt
		}()
	}
}

func (s *LogSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return Logs.BeforeInsertHooks.RunHooks(ctx, exec, s)
	})

	q.AppendValues(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		vals := make([]bob.Expression, 3)
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

		if s.TableName != nil {
			vals[1] = psql.Arg(func() string {
				if s.TableName == nil {
					return *new(string)
				}
				return *s.TableName
			}())
		} else {
			vals[1] = psql.Raw("DEFAULT")
		}

		if s.ChangedAt != nil {
			vals[2] = psql.Arg(func() time.Time {
				if s.ChangedAt == nil {
					return *new(time.Time)
				}
				return *s.ChangedAt
			}())
		} else {
			vals[2] = psql.Raw("DEFAULT")
		}

		return bob.ExpressSlice(ctx, w, d, start, vals, "", ", ", "")
	}))
}

func (s LogSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s LogSetter) Expressions(prefix ...string) []bob.Expression {
	exprs := make([]bob.Expression, 0, 3)

	if s.ID != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "id")...),
			psql.Arg(s.ID),
		}})
	}

	if s.TableName != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "table_name")...),
			psql.Arg(s.TableName),
		}})
	}

	if s.ChangedAt != nil {
		exprs = append(exprs, expr.Join{Sep: " = ", Exprs: []bob.Expression{
			psql.Quote(append(prefix, "changed_at")...),
			psql.Arg(s.ChangedAt),
		}})
	}

	return exprs
}

// FindLog retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindLog(ctx context.Context, exec bob.Executor, IDPK int64, cols ...string) (*Log, error) {
	if len(cols) == 0 {
		return Logs.Query(
			sm.Where(Logs.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return Logs.Query(
		sm.Where(Logs.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(Logs.Columns.Only(cols...)),
	).One(ctx, exec)
}

// LogExists checks the presence of a single record by primary key
func LogExists(ctx context.Context, exec bob.Executor, IDPK int64) (bool, error) {
	return Logs.Query(
		sm.Where(Logs.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after Log is retrieved from the database
func (o *Log) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Logs.AfterSelectHooks.RunHooks(ctx, exec, LogSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = Logs.AfterInsertHooks.RunHooks(ctx, exec, LogSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = Logs.AfterUpdateHooks.RunHooks(ctx, exec, LogSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = Logs.AfterDeleteHooks.RunHooks(ctx, exec, LogSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = Logs.AfterMergeHooks.RunHooks(ctx, exec, LogSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the Log
func (o *Log) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *Log) pkEQ() dialect.Expression {
	return psql.Quote("log", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the Log
func (o *Log) Update(ctx context.Context, exec bob.Executor, s *LogSetter) error {
	v, err := Logs.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single Log record with an executor
func (o *Log) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := Logs.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the Log using the executor
func (o *Log) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := Logs.Query(
		sm.Where(Logs.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after LogSlice is retrieved from the database
func (o LogSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = Logs.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = Logs.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = Logs.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = Logs.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = Logs.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o LogSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("log", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o LogSlice) copyMatchingRows(from ...*Log) {
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
func (o LogSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Logs.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Log:
				o.copyMatchingRows(retrieved)
			case []*Log:
				o.copyMatchingRows(retrieved...)
			case LogSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Log or a slice of Log
				// then run the AfterUpdateHooks on the slice
				_, err = Logs.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o LogSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Logs.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Log:
				o.copyMatchingRows(retrieved)
			case []*Log:
				o.copyMatchingRows(retrieved...)
			case LogSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Log or a slice of Log
				// then run the AfterDeleteHooks on the slice
				_, err = Logs.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o LogSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return Logs.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *Log:
				o.copyMatchingRows(retrieved)
			case []*Log:
				o.copyMatchingRows(retrieved...)
			case LogSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a Log or a slice of Log
				// then run the AfterMergeHooks on the slice
				_, err = Logs.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o LogSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals LogSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Logs.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o LogSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := Logs.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o LogSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := Logs.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type logWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, int64]
	TableName psql.WhereMod[Q, string]
	ChangedAt psql.WhereMod[Q, time.Time]
}

func (logWhere[Q]) AliasedAs(alias string) logWhere[Q] {
	return buildLogWhere[Q](buildLogColumns(alias))
}

func buildLogWhere[Q psql.Filterable](cols logColumns) logWhere[Q] {
	return logWhere[Q]{
		ID:        psql.Where[Q, int64](cols.ID.Expression),
		TableName: psql.Where[Q, string](cols.TableName.Expression),
		ChangedAt: psql.Where[Q, time.Time](cols.ChangedAt.Expression),
	}
}
