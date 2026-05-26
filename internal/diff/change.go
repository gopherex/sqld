// Package diff compares two IR catalogs and produces an ordered migration
// Plan whose forward (UpSQL) and inverse (DownSQL) DDL can be rendered.
//
// The diff is DB-free: it operates purely on *irv1.Catalog values (typically
// produced by internal/parse + internal/catalog.Build) and emits PostgreSQL
// DDL strings. This file defines the Change interface, the Plan container and
// the concrete change types for the object kinds handled in this task
// (schemas, types, sequences, tables, columns). Constraints, indexes, views,
// functions and triggers are handled by a follow-up task; their dependency
// sortKeys are reserved below.
package diff

import (
	"sort"
	"strings"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
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
	sortKeyConstraint = 50 // reserved: next task
	sortKeyIndex      = 60 // reserved: next task
	sortKeyForeignKey = 70 // reserved: next task
	sortKeyView       = 80 // reserved: next task
	sortKeyFunc       = 90 // reserved: next task
	sortKeyTrigger    = 100
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

// UpSQL renders the forward migration: changes stable-sorted by ascending
// sortKey, with each non-empty UpSQL joined by newlines.
func (p *Plan) UpSQL() string {
	cs := make([]Change, len(p.Changes))
	copy(cs, p.Changes)
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].sortKey() < cs[j].sortKey() })
	var b []string
	for _, c := range cs {
		if s := strings.TrimSpace(c.UpSQL()); s != "" {
			b = append(b, s)
		}
	}
	return strings.Join(b, "\n")
}

// DownSQL renders the inverse migration: changes stable-sorted by descending
// sortKey, with each non-empty DownSQL joined by newlines.
func (p *Plan) DownSQL() string {
	cs := make([]Change, len(p.Changes))
	copy(cs, p.Changes)
	sort.SliceStable(cs, func(i, j int) bool { return cs[i].sortKey() > cs[j].sortKey() })
	var b []string
	for _, c := range cs {
		if s := strings.TrimSpace(c.DownSQL()); s != "" {
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
