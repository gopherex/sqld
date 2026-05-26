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

// AuditLog is an object representing the database table.
type AuditLog struct {
	ID        int64     `db:"id,pk" `
	TableName string    `db:"table_name" `
	ChangedAt time.Time `db:"changed_at" `
}

// AuditLogSlice is an alias for a slice of pointers to AuditLog.
// This should almost always be used instead of []*AuditLog.
type AuditLogSlice []*AuditLog

// AuditLogs contains methods to work with the log table
var AuditLogs = psql.NewTablex[*AuditLog, AuditLogSlice, *AuditLogSetter]("audit", "log", buildAuditLogColumns("audit.log"))

// AuditLogsQuery is a query on the log table
type AuditLogsQuery = *psql.ViewQuery[*AuditLog, AuditLogSlice]

func buildAuditLogColumns(tableName string) auditLogColumns {
	columnsExpr := expr.NewColumnsExpr(
		"id", "table_name", "changed_at",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return auditLogColumns{
		ColumnsExpr: columnsExpr,
		tableAlias:  tableName,
		ID:          buildAuditLogColumn(tableName, "id"),
		TableName:   buildAuditLogColumn(tableName, "table_name"),
		ChangedAt:   buildAuditLogColumn(tableName, "changed_at"),
	}
}

type auditLogColumns struct {
	expr.ColumnsExpr
	tableAlias string
	ID         auditLogColumn
	TableName  auditLogColumn
	ChangedAt  auditLogColumn
}

// Alias returns the current table alias for the columns set.
func (c auditLogColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (auditLogColumns) AliasedAs(tableName string) auditLogColumns {
	return buildAuditLogColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c auditLogColumns) Unqualified() auditLogColumns {
	return buildAuditLogColumns("")
}

func buildAuditLogColumn(alias, name string) auditLogColumn {
	return auditLogColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type auditLogColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c auditLogColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c auditLogColumn) ShouldOmitParens() bool {
	return true
}

// AuditLogSetter is used for insert/upsert/update operations
// All values are optional, and do not have to be set
// Generated columns are not included
type AuditLogSetter struct {
	ID        *int64     `db:"id,pk" `
	TableName *string    `db:"table_name" `
	ChangedAt *time.Time `db:"changed_at" `
}

func (s AuditLogSetter) SetColumns() []string {
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

func (s AuditLogSetter) Overwrite(t *AuditLog) {
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

func (s *AuditLogSetter) Apply(q *dialect.InsertQuery) {
	q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
		return AuditLogs.BeforeInsertHooks.RunHooks(ctx, exec, s)
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

func (s AuditLogSetter) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return um.Set(s.Expressions()...)
}

func (s AuditLogSetter) Expressions(prefix ...string) []bob.Expression {
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

// FindAuditLog retrieves a single record by primary key
// If cols is empty Find will return all columns.
func FindAuditLog(ctx context.Context, exec bob.Executor, IDPK int64, cols ...string) (*AuditLog, error) {
	if len(cols) == 0 {
		return AuditLogs.Query(
			sm.Where(AuditLogs.Columns.ID.EQ(psql.Arg(IDPK))),
		).One(ctx, exec)
	}

	return AuditLogs.Query(
		sm.Where(AuditLogs.Columns.ID.EQ(psql.Arg(IDPK))),
		sm.Columns(AuditLogs.Columns.Only(cols...)),
	).One(ctx, exec)
}

// AuditLogExists checks the presence of a single record by primary key
func AuditLogExists(ctx context.Context, exec bob.Executor, IDPK int64) (bool, error) {
	return AuditLogs.Query(
		sm.Where(AuditLogs.Columns.ID.EQ(psql.Arg(IDPK))),
	).Exists(ctx, exec)
}

// AfterQueryHook is called after AuditLog is retrieved from the database
func (o *AuditLog) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AuditLogs.AfterSelectHooks.RunHooks(ctx, exec, AuditLogSlice{o})
	case bob.QueryTypeInsert:
		ctx, err = AuditLogs.AfterInsertHooks.RunHooks(ctx, exec, AuditLogSlice{o})
	case bob.QueryTypeUpdate:
		ctx, err = AuditLogs.AfterUpdateHooks.RunHooks(ctx, exec, AuditLogSlice{o})
	case bob.QueryTypeDelete:
		ctx, err = AuditLogs.AfterDeleteHooks.RunHooks(ctx, exec, AuditLogSlice{o})
	case bob.QueryTypeMerge:
		ctx, err = AuditLogs.AfterMergeHooks.RunHooks(ctx, exec, AuditLogSlice{o})
	}

	return err
}

// primaryKeyVals returns the primary key values of the AuditLog
func (o *AuditLog) primaryKeyVals() bob.Expression {
	return psql.Arg(o.ID)
}

func (o *AuditLog) pkEQ() dialect.Expression {
	return psql.Quote("audit.log", "id").EQ(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
		return o.primaryKeyVals().WriteSQL(ctx, w, d, start)
	}))
}

// Update uses an executor to update the AuditLog
func (o *AuditLog) Update(ctx context.Context, exec bob.Executor, s *AuditLogSetter) error {
	v, err := AuditLogs.Update(s.UpdateMod(), um.Where(o.pkEQ())).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *v

	return nil
}

// Delete deletes a single AuditLog record with an executor
func (o *AuditLog) Delete(ctx context.Context, exec bob.Executor) error {
	_, err := AuditLogs.Delete(dm.Where(o.pkEQ())).Exec(ctx, exec)
	return err
}

// Reload refreshes the AuditLog using the executor
func (o *AuditLog) Reload(ctx context.Context, exec bob.Executor) error {
	o2, err := AuditLogs.Query(
		sm.Where(AuditLogs.Columns.ID.EQ(psql.Arg(o.ID))),
	).One(ctx, exec)
	if err != nil {
		return err
	}

	*o = *o2

	return nil
}

// AfterQueryHook is called after AuditLogSlice is retrieved from the database
func (o AuditLogSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AuditLogs.AfterSelectHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeInsert:
		ctx, err = AuditLogs.AfterInsertHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeUpdate:
		ctx, err = AuditLogs.AfterUpdateHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeDelete:
		ctx, err = AuditLogs.AfterDeleteHooks.RunHooks(ctx, exec, o)
	case bob.QueryTypeMerge:
		ctx, err = AuditLogs.AfterMergeHooks.RunHooks(ctx, exec, o)
	}

	return err
}

func (o AuditLogSlice) pkIN() dialect.Expression {
	if len(o) == 0 {
		return psql.Raw("NULL")
	}

	return psql.Quote("audit.log", "id").In(bob.ExpressionFunc(func(ctx context.Context, w io.StringWriter, d bob.Dialect, start int) ([]any, error) {
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
func (o AuditLogSlice) copyMatchingRows(from ...*AuditLog) {
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
func (o AuditLogSlice) UpdateMod() bob.Mod[*dialect.UpdateQuery] {
	return bob.ModFunc[*dialect.UpdateQuery](func(q *dialect.UpdateQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AuditLogs.BeforeUpdateHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AuditLog:
				o.copyMatchingRows(retrieved)
			case []*AuditLog:
				o.copyMatchingRows(retrieved...)
			case AuditLogSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AuditLog or a slice of AuditLog
				// then run the AfterUpdateHooks on the slice
				_, err = AuditLogs.AfterUpdateHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// DeleteMod modifies an delete query with "WHERE primary_key IN (o...)"
func (o AuditLogSlice) DeleteMod() bob.Mod[*dialect.DeleteQuery] {
	return bob.ModFunc[*dialect.DeleteQuery](func(q *dialect.DeleteQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AuditLogs.BeforeDeleteHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AuditLog:
				o.copyMatchingRows(retrieved)
			case []*AuditLog:
				o.copyMatchingRows(retrieved...)
			case AuditLogSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AuditLog or a slice of AuditLog
				// then run the AfterDeleteHooks on the slice
				_, err = AuditLogs.AfterDeleteHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))

		q.AppendWhere(o.pkIN())
	})
}

// MergeMod modifies a merge query to run BeforeMergeHooks and AfterMergeHooks
// and updates the slice with the returned rows.
func (o AuditLogSlice) MergeMod() bob.Mod[*dialect.MergeQuery] {
	return bob.ModFunc[*dialect.MergeQuery](func(q *dialect.MergeQuery) {
		q.AppendHooks(func(ctx context.Context, exec bob.Executor) (context.Context, error) {
			return AuditLogs.BeforeMergeHooks.RunHooks(ctx, exec, o)
		})

		q.AppendLoader(bob.LoaderFunc(func(ctx context.Context, exec bob.Executor, retrieved any) error {
			var err error
			switch retrieved := retrieved.(type) {
			case *AuditLog:
				o.copyMatchingRows(retrieved)
			case []*AuditLog:
				o.copyMatchingRows(retrieved...)
			case AuditLogSlice:
				o.copyMatchingRows(retrieved...)
			default:
				// If the retrieved value is not a AuditLog or a slice of AuditLog
				// then run the AfterMergeHooks on the slice
				_, err = AuditLogs.AfterMergeHooks.RunHooks(ctx, exec, o)
			}

			return err
		}))
	})
}

func (o AuditLogSlice) UpdateAll(ctx context.Context, exec bob.Executor, vals AuditLogSetter) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AuditLogs.Update(vals.UpdateMod(), o.UpdateMod()).All(ctx, exec)
	return err
}

func (o AuditLogSlice) DeleteAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	_, err := AuditLogs.Delete(o.DeleteMod()).Exec(ctx, exec)
	return err
}

func (o AuditLogSlice) ReloadAll(ctx context.Context, exec bob.Executor) error {
	if len(o) == 0 {
		return nil
	}

	o2, err := AuditLogs.Query(sm.Where(o.pkIN())).All(ctx, exec)
	if err != nil {
		return err
	}

	o.copyMatchingRows(o2...)

	return nil
}

type auditLogWhere[Q psql.Filterable] struct {
	ID        psql.WhereMod[Q, int64]
	TableName psql.WhereMod[Q, string]
	ChangedAt psql.WhereMod[Q, time.Time]
}

func (auditLogWhere[Q]) AliasedAs(alias string) auditLogWhere[Q] {
	return buildAuditLogWhere[Q](buildAuditLogColumns(alias))
}

func buildAuditLogWhere[Q psql.Filterable](cols auditLogColumns) auditLogWhere[Q] {
	return auditLogWhere[Q]{
		ID:        psql.Where[Q, int64](cols.ID.Expression),
		TableName: psql.Where[Q, string](cols.TableName.Expression),
		ChangedAt: psql.Where[Q, time.Time](cols.ChangedAt.Expression),
	}
}
