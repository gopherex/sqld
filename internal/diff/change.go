// Package diff compares two IR catalogs and produces an ordered migration
// Plan whose forward (UpSQL) and inverse (DownSQL) DDL can be rendered.
//
// The diff is DB-free: it operates purely on *irv1.Catalog values (typically
// produced by internal/parse + internal/catalog.Build) and emits PostgreSQL
// DDL strings. This file defines the Change interface, the Plan container and
// the concrete change types for the object kinds handled in this task
// (schemas, types, sequences, tables, columns, constraints, indexes, foreign
// keys, views, materialized views, functions, procedures and triggers).
package diff

import (
	"sort"
	"strings"

	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

// Dependency ordering keys. Creates are applied in ascending order so that an
// object always exists before things that depend on it. DownSQL reverses the
// whole plan, which yields a correct teardown order (e.g. a CreateTable at 30
// before an AddForeignKey at 70 reverses to fk-drop before table-drop).
const (
	sortKeySchema     = 0
	sortKeyType       = 10
	sortKeySequence   = 20
	sortKeyTable      = 30
	sortKeyColumn     = 40
	sortKeyConstraint = 50  // PK / UNIQUE / CHECK / EXCLUSION
	sortKeyIndex      = 60  // CREATE INDEX
	sortKeyForeignKey = 70  // FOREIGN KEY (after all tables exist)
	sortKeyView       = 80  // views and materialized views
	sortKeyFunc       = 90  // functions and procedures
	sortKeyTrigger    = 100 // triggers (depend on tables + functions)
)

// Change is a single reversible schema change.
type Change interface {
	// UpSQL renders the forward DDL for this change.
	UpSQL() string
	// DownSQL renders the inverse DDL for this change.
	DownSQL() string
	// sortKey returns the dependency-order bucket for this change. Drops share
	// the key of their matching create kind; the Plan reverses the whole change
	// list for DownSQL, which produces a correct teardown order.
	sortKey() int
}

// Plan is an ordered set of schema changes.
type Plan struct {
	Changes []Change
}

// Empty reports whether the plan has no changes.
func (p *Plan) Empty() bool { return len(p.Changes) == 0 }

// upOrdered returns the plan's changes in forward (apply) order: a stable sort
// by ascending sortKey that preserves the relative order within a bucket. The
// down migration is the exact reverse of this slice, so both directions agree
// on a single canonical ordering.
func (p *Plan) upOrdered() []Change {
	cs := make([]Change, len(p.Changes))
	copy(cs, p.Changes)
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].sortKey() < cs[j].sortKey() })
	return cs
}

// UpSQL renders the forward migration: changes in ascending sortKey order, with
// each non-empty UpSQL joined by newlines.
func (p *Plan) UpSQL() string {
	cs := p.upOrdered()
	var b []string
	for _, c := range cs {
		if s := strings.TrimSpace(c.UpSQL()); s != "" {
			b = append(b, s)
		}
	}
	return strings.Join(b, "\n")
}

// DownSQL renders the inverse migration: the exact reverse of the up order,
// emitting each change's DownSQL. Reversing the up order (rather than
// re-sorting descending) is required for correctness when a single logical
// change expands to an ordered pair in the same bucket — e.g. a changed
// constraint emits [Drop(old), Add(new)] on up, whose correct inverse is
// [Drop(new).Down=Add(new)... ] reversed to [Add(new).Down=Drop(new),
// Drop(old).Down=Add(old)], i.e. drop the new then re-add the old.
func (p *Plan) DownSQL() string {
	cs := p.upOrdered()
	var b []string
	for i := len(cs) - 1; i >= 0; i-- {
		if s := strings.TrimSpace(cs[i].DownSQL()); s != "" {
			b = append(b, s)
		}
	}
	return strings.Join(b, "\n")
}

// lossWarning is prepended to best-effort re-creates rendered on rollback of a
// destructive change.
const lossWarning = "-- WARNING: data loss on rollback\n"

// --- schemas -----------------------------------------------------------------

// CreateSchema creates a schema.
type CreateSchema struct{ Schema *irv1.Schema }

func (c *CreateSchema) UpSQL() string   { return renderCreateSchema(c.Schema) }
func (c *CreateSchema) DownSQL() string { return renderDropSchema(c.Schema) }
func (c *CreateSchema) sortKey() int    { return sortKeySchema }

// DropSchema drops a schema.
type DropSchema struct{ Schema *irv1.Schema }

func (c *DropSchema) UpSQL() string   { return renderDropSchema(c.Schema) }
func (c *DropSchema) DownSQL() string { return renderCreateSchema(c.Schema) }
func (c *DropSchema) sortKey() int    { return sortKeySchema }

// --- enums -------------------------------------------------------------------

// CreateEnum creates an enum type.
type CreateEnum struct{ Enum *irv1.EnumType }

func (c *CreateEnum) UpSQL() string   { return renderCreateEnum(c.Enum) }
func (c *CreateEnum) DownSQL() string { return renderDropType(c.Enum.GetName()) }
func (c *CreateEnum) sortKey() int    { return sortKeyType }

// DropEnum drops an enum type.
type DropEnum struct{ Enum *irv1.EnumType }

func (c *DropEnum) UpSQL() string   { return renderDropType(c.Enum.GetName()) }
func (c *DropEnum) DownSQL() string { return renderCreateEnum(c.Enum) }
func (c *DropEnum) sortKey() int    { return sortKeyType }

// AddEnumValue adds a single label to an existing enum (irreversible in PG).
type AddEnumValue struct {
	Enum  *irv1.EnumType
	Value string
	// After is the label this value should be inserted after; empty appends.
	After string
}

func (c *AddEnumValue) UpSQL() string {
	s := "ALTER TYPE " + qualified(c.Enum.GetName().GetSchema(), c.Enum.GetName().GetName()) +
		" ADD VALUE " + quoteString(c.Value)
	if c.After != "" {
		s += " AFTER " + quoteString(c.After)
	}
	return s + ";"
}

// DownSQL is a no-op with a warning: PostgreSQL cannot drop an enum label.
func (c *AddEnumValue) DownSQL() string {
	return "-- WARNING: cannot drop enum value " + quoteString(c.Value) +
		" from " + qualified(c.Enum.GetName().GetSchema(), c.Enum.GetName().GetName())
}
func (c *AddEnumValue) sortKey() int { return sortKeyType }

// --- domains -----------------------------------------------------------------

// CreateDomain creates a domain type.
type CreateDomain struct{ Domain *irv1.DomainType }

func (c *CreateDomain) UpSQL() string   { return renderCreateDomain(c.Domain) }
func (c *CreateDomain) DownSQL() string { return renderDropType(c.Domain.GetName()) }
func (c *CreateDomain) sortKey() int    { return sortKeyType }

// DropDomain drops a domain type.
type DropDomain struct{ Domain *irv1.DomainType }

func (c *DropDomain) UpSQL() string   { return renderDropType(c.Domain.GetName()) }
func (c *DropDomain) DownSQL() string { return renderCreateDomain(c.Domain) }
func (c *DropDomain) sortKey() int    { return sortKeyType }

// --- composites --------------------------------------------------------------

// CreateComposite creates a composite type.
type CreateComposite struct{ Composite *irv1.CompositeType }

func (c *CreateComposite) UpSQL() string   { return renderCreateComposite(c.Composite) }
func (c *CreateComposite) DownSQL() string { return renderDropType(c.Composite.GetName()) }
func (c *CreateComposite) sortKey() int    { return sortKeyType }

// DropComposite drops a composite type.
type DropComposite struct{ Composite *irv1.CompositeType }

func (c *DropComposite) UpSQL() string   { return renderDropType(c.Composite.GetName()) }
func (c *DropComposite) DownSQL() string { return renderCreateComposite(c.Composite) }
func (c *DropComposite) sortKey() int    { return sortKeyType }

// --- ranges ------------------------------------------------------------------

// CreateRange creates a range type.
type CreateRange struct{ Range *irv1.RangeType }

func (c *CreateRange) UpSQL() string   { return renderCreateRange(c.Range) }
func (c *CreateRange) DownSQL() string { return renderDropType(c.Range.GetName()) }
func (c *CreateRange) sortKey() int    { return sortKeyType }

// DropRange drops a range type.
type DropRange struct{ Range *irv1.RangeType }

func (c *DropRange) UpSQL() string   { return renderDropType(c.Range.GetName()) }
func (c *DropRange) DownSQL() string { return renderCreateRange(c.Range) }
func (c *DropRange) sortKey() int    { return sortKeyType }

// --- sequences ---------------------------------------------------------------

// CreateSequence creates a sequence.
type CreateSequence struct{ Sequence *irv1.Sequence }

func (c *CreateSequence) UpSQL() string   { return renderCreateSequence(c.Sequence) }
func (c *CreateSequence) DownSQL() string { return renderDropSequence(c.Sequence) }
func (c *CreateSequence) sortKey() int    { return sortKeySequence }

// DropSequence drops a sequence.
type DropSequence struct{ Sequence *irv1.Sequence }

func (c *DropSequence) UpSQL() string   { return renderDropSequence(c.Sequence) }
func (c *DropSequence) DownSQL() string { return renderCreateSequence(c.Sequence) }
func (c *DropSequence) sortKey() int    { return sortKeySequence }

// --- tables ------------------------------------------------------------------

// CreateTable creates a table with its columns and inline primary key.
type CreateTable struct{ Table *irv1.Table }

func (c *CreateTable) UpSQL() string   { return renderCreateTable(c.Table) }
func (c *CreateTable) DownSQL() string { return renderDropTable(c.Table) }
func (c *CreateTable) sortKey() int    { return sortKeyTable }

// DropTable drops a table. The inverse re-creates it best-effort with a
// data-loss warning.
type DropTable struct{ Table *irv1.Table }

func (c *DropTable) UpSQL() string   { return renderDropTable(c.Table) }
func (c *DropTable) DownSQL() string { return lossWarning + renderCreateTable(c.Table) }
func (c *DropTable) sortKey() int    { return sortKeyTable }

// --- columns -----------------------------------------------------------------

// AddColumn adds a column to a table.
type AddColumn struct {
	Table  *irv1.Table
	Column *irv1.Column
}

func (c *AddColumn) UpSQL() string {
	return "ALTER TABLE " + tableQualified(c.Table) + " ADD COLUMN " + renderColumn(c.Column) + ";"
}
func (c *AddColumn) DownSQL() string {
	return "ALTER TABLE " + tableQualified(c.Table) + " DROP COLUMN " + quoteIdent(c.Column.GetName()) + ";"
}
func (c *AddColumn) sortKey() int { return sortKeyColumn }

// DropColumn drops a column. The inverse re-adds it best-effort with a
// data-loss warning.
type DropColumn struct {
	Table  *irv1.Table
	Column *irv1.Column
}

func (c *DropColumn) UpSQL() string {
	return "ALTER TABLE " + tableQualified(c.Table) + " DROP COLUMN " + quoteIdent(c.Column.GetName()) + ";"
}
func (c *DropColumn) DownSQL() string {
	return lossWarning + "ALTER TABLE " + tableQualified(c.Table) + " ADD COLUMN " + renderColumn(c.Column) + ";"
}
func (c *DropColumn) sortKey() int { return sortKeyColumn }

// AlterColumnType changes a column's type using a USING cast.
type AlterColumnType struct {
	Table  *irv1.Table
	Column string
	From   *irv1.TypeRef
	To     *irv1.TypeRef
}

func (c *AlterColumnType) UpSQL() string   { return alterColumnType(c.Table, c.Column, c.To) }
func (c *AlterColumnType) DownSQL() string { return alterColumnType(c.Table, c.Column, c.From) }
func (c *AlterColumnType) sortKey() int    { return sortKeyColumn }

func alterColumnType(t *irv1.Table, col string, typ *irv1.TypeRef) string {
	rt := renderType(typ)
	return "ALTER TABLE " + tableQualified(t) + " ALTER COLUMN " + quoteIdent(col) +
		" TYPE " + rt + " USING " + quoteIdent(col) + "::" + rt + ";"
}

// SetNotNull adds a NOT NULL constraint to a column (nullable true -> false).
type SetNotNull struct {
	Table  *irv1.Table
	Column string
}

func (c *SetNotNull) UpSQL() string {
	return "ALTER TABLE " + tableQualified(c.Table) + " ALTER COLUMN " + quoteIdent(c.Column) + " SET NOT NULL;"
}
func (c *SetNotNull) DownSQL() string {
	return "ALTER TABLE " + tableQualified(c.Table) + " ALTER COLUMN " + quoteIdent(c.Column) + " DROP NOT NULL;"
}
func (c *SetNotNull) sortKey() int { return sortKeyColumn }

// DropNotNull removes a NOT NULL constraint (nullable false -> true).
type DropNotNull struct {
	Table  *irv1.Table
	Column string
}

func (c *DropNotNull) UpSQL() string {
	return "ALTER TABLE " + tableQualified(c.Table) + " ALTER COLUMN " + quoteIdent(c.Column) + " DROP NOT NULL;"
}
func (c *DropNotNull) DownSQL() string {
	return "ALTER TABLE " + tableQualified(c.Table) + " ALTER COLUMN " + quoteIdent(c.Column) + " SET NOT NULL;"
}
func (c *DropNotNull) sortKey() int { return sortKeyColumn }

// SetDefault sets (or changes) a column default.
type SetDefault struct {
	Table   *irv1.Table
	Column  string
	Default *irv1.Expr // new default
	Old     *irv1.Expr // previous default, for DownSQL (nil => DROP DEFAULT)
}

func (c *SetDefault) UpSQL() string {
	return "ALTER TABLE " + tableQualified(c.Table) + " ALTER COLUMN " + quoteIdent(c.Column) +
		" SET DEFAULT " + renderExpr(c.Default) + ";"
}
func (c *SetDefault) DownSQL() string {
	if c.Old == nil {
		return "ALTER TABLE " + tableQualified(c.Table) + " ALTER COLUMN " + quoteIdent(c.Column) + " DROP DEFAULT;"
	}
	return "ALTER TABLE " + tableQualified(c.Table) + " ALTER COLUMN " + quoteIdent(c.Column) +
		" SET DEFAULT " + renderExpr(c.Old) + ";"
}
func (c *SetDefault) sortKey() int { return sortKeyColumn }

// DropDefault removes a column default.
type DropDefault struct {
	Table  *irv1.Table
	Column string
	Old    *irv1.Expr // previous default, for DownSQL
}

func (c *DropDefault) UpSQL() string {
	return "ALTER TABLE " + tableQualified(c.Table) + " ALTER COLUMN " + quoteIdent(c.Column) + " DROP DEFAULT;"
}
func (c *DropDefault) DownSQL() string {
	if c.Old == nil {
		return ""
	}
	return "ALTER TABLE " + tableQualified(c.Table) + " ALTER COLUMN " + quoteIdent(c.Column) +
		" SET DEFAULT " + renderExpr(c.Old) + ";"
}
func (c *DropDefault) sortKey() int { return sortKeyColumn }

// --- constraints -------------------------------------------------------------

// constraintSortKey returns the dependency bucket for a constraint: foreign
// keys order after all tables/columns/other-constraints exist, everything else
// (PK/UNIQUE/CHECK/EXCLUSION) at the constraint bucket.
func constraintSortKey(c *irv1.Constraint) int {
	if c.GetType() == irv1.ConstraintType_CONSTRAINT_TYPE_FOREIGN_KEY {
		return sortKeyForeignKey
	}
	return sortKeyConstraint
}

// AddConstraint adds a table constraint (PK/UNIQUE/FK/CHECK/EXCLUSION).
type AddConstraint struct {
	Table      *irv1.Table
	Constraint *irv1.Constraint
}

func (c *AddConstraint) UpSQL() string   { return renderAddConstraint(c.Table, c.Constraint) }
func (c *AddConstraint) DownSQL() string { return renderDropConstraint(c.Table, c.Constraint) }
func (c *AddConstraint) sortKey() int    { return constraintSortKey(c.Constraint) }

// DropConstraint drops a table constraint. The inverse re-adds it.
type DropConstraint struct {
	Table      *irv1.Table
	Constraint *irv1.Constraint
}

func (c *DropConstraint) UpSQL() string   { return renderDropConstraint(c.Table, c.Constraint) }
func (c *DropConstraint) DownSQL() string { return renderAddConstraint(c.Table, c.Constraint) }
func (c *DropConstraint) sortKey() int    { return constraintSortKey(c.Constraint) }

// --- indexes -----------------------------------------------------------------

// CreateIndex creates an index on a table.
type CreateIndex struct {
	Table  *irv1.Table
	Index  *irv1.Index
	Schema string
}

func (c *CreateIndex) UpSQL() string   { return renderCreateIndex(c.Index, c.Table) }
func (c *CreateIndex) DownSQL() string { return renderDropIndex(c.Index, c.Schema) }
func (c *CreateIndex) sortKey() int    { return sortKeyIndex }

// DropIndex drops an index. The inverse re-creates it (non-lossy).
type DropIndex struct {
	Table  *irv1.Table
	Index  *irv1.Index
	Schema string
}

func (c *DropIndex) UpSQL() string   { return renderDropIndex(c.Index, c.Schema) }
func (c *DropIndex) DownSQL() string { return renderCreateIndex(c.Index, c.Table) }
func (c *DropIndex) sortKey() int    { return sortKeyIndex }

// --- views -------------------------------------------------------------------

// CreateView creates a view.
type CreateView struct{ View *irv1.View }

func (c *CreateView) UpSQL() string   { return renderCreateView(c.View, false) }
func (c *CreateView) DownSQL() string { return renderDropView(c.View) }
func (c *CreateView) sortKey() int    { return sortKeyView }

// DropView drops a view. The inverse re-creates it from its definition.
type DropView struct{ View *irv1.View }

func (c *DropView) UpSQL() string   { return renderDropView(c.View) }
func (c *DropView) DownSQL() string { return renderCreateView(c.View, false) }
func (c *DropView) sortKey() int    { return sortKeyView }

// ReplaceView replaces a view's definition via CREATE OR REPLACE VIEW. The
// inverse restores the previous definition the same way.
type ReplaceView struct {
	From *irv1.View
	To   *irv1.View
}

func (c *ReplaceView) UpSQL() string   { return renderCreateView(c.To, true) }
func (c *ReplaceView) DownSQL() string { return renderCreateView(c.From, true) }
func (c *ReplaceView) sortKey() int    { return sortKeyView }

// CreateMatView creates a materialized view.
type CreateMatView struct{ View *irv1.MaterializedView }

func (c *CreateMatView) UpSQL() string   { return renderCreateMatView(c.View) }
func (c *CreateMatView) DownSQL() string { return renderDropMatView(c.View) }
func (c *CreateMatView) sortKey() int    { return sortKeyView }

// DropMatView drops a materialized view. The inverse re-creates it (WITH NO
// DATA on the catalog form; data is not part of the schema).
type DropMatView struct{ View *irv1.MaterializedView }

func (c *DropMatView) UpSQL() string   { return renderDropMatView(c.View) }
func (c *DropMatView) DownSQL() string { return renderCreateMatView(c.View) }
func (c *DropMatView) sortKey() int    { return sortKeyView }

// --- functions and procedures ------------------------------------------------

// CreateFunction creates a function.
type CreateFunction struct{ Function *irv1.Function }

func (c *CreateFunction) UpSQL() string   { return renderCreateFunction(c.Function, false) }
func (c *CreateFunction) DownSQL() string { return renderDropFunction(c.Function) }
func (c *CreateFunction) sortKey() int    { return sortKeyFunc }

// DropFunction drops a function. The inverse re-creates it from its body.
type DropFunction struct{ Function *irv1.Function }

func (c *DropFunction) UpSQL() string   { return renderDropFunction(c.Function) }
func (c *DropFunction) DownSQL() string { return renderCreateFunction(c.Function, false) }
func (c *DropFunction) sortKey() int    { return sortKeyFunc }

// ReplaceFunction replaces a function via CREATE OR REPLACE FUNCTION. The
// inverse restores the previous definition the same way.
type ReplaceFunction struct {
	From *irv1.Function
	To   *irv1.Function
}

func (c *ReplaceFunction) UpSQL() string   { return renderCreateFunction(c.To, true) }
func (c *ReplaceFunction) DownSQL() string { return renderCreateFunction(c.From, true) }
func (c *ReplaceFunction) sortKey() int    { return sortKeyFunc }

// CreateProcedure creates a procedure.
type CreateProcedure struct{ Procedure *irv1.Procedure }

func (c *CreateProcedure) UpSQL() string   { return renderCreateProcedure(c.Procedure, false) }
func (c *CreateProcedure) DownSQL() string { return renderDropProcedure(c.Procedure) }
func (c *CreateProcedure) sortKey() int    { return sortKeyFunc }

// DropProcedure drops a procedure. The inverse re-creates it from its body.
type DropProcedure struct{ Procedure *irv1.Procedure }

func (c *DropProcedure) UpSQL() string   { return renderDropProcedure(c.Procedure) }
func (c *DropProcedure) DownSQL() string { return renderCreateProcedure(c.Procedure, false) }
func (c *DropProcedure) sortKey() int    { return sortKeyFunc }

// ReplaceProcedure replaces a procedure via CREATE OR REPLACE PROCEDURE.
type ReplaceProcedure struct {
	From *irv1.Procedure
	To   *irv1.Procedure
}

func (c *ReplaceProcedure) UpSQL() string   { return renderCreateProcedure(c.To, true) }
func (c *ReplaceProcedure) DownSQL() string { return renderCreateProcedure(c.From, true) }
func (c *ReplaceProcedure) sortKey() int    { return sortKeyFunc }

// --- triggers ----------------------------------------------------------------

// CreateTrigger creates a trigger.
type CreateTrigger struct{ Trigger *irv1.Trigger }

func (c *CreateTrigger) UpSQL() string   { return renderCreateTrigger(c.Trigger) }
func (c *CreateTrigger) DownSQL() string { return renderDropTrigger(c.Trigger) }
func (c *CreateTrigger) sortKey() int    { return sortKeyTrigger }

// DropTrigger drops a trigger. The inverse re-creates it from its definition.
type DropTrigger struct{ Trigger *irv1.Trigger }

func (c *DropTrigger) UpSQL() string   { return renderDropTrigger(c.Trigger) }
func (c *DropTrigger) DownSQL() string { return renderCreateTrigger(c.Trigger) }
func (c *DropTrigger) sortKey() int    { return sortKeyTrigger }
