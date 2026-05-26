package main

import (
	"context"
	"fmt"
	"path"

	helpers "github.com/stephenafamo/bob/gen/bobgen-helpers"
	"github.com/stephenafamo/bob/gen/drivers"

	"github.com/yaroher/sqld/pkg/gotypes"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// sqldDriver implements bob's drivers.Interface backed by an sqld IR Catalog.
// It is the bridge of the ORM⊕sqlc symbiosis: every column's bob Column.Type is
// resolved through the shared pkg/gotypes mapper, so bob emits the SAME Go types
// sqld-gen-go does. Enum/composite types are qualified into the caller's
// TypesPackage (sqld-gen-go's output); DBInfo.Enums is left empty so bob does
// not generate a competing enum package.
//
// The three type parameters (DBExtra, ConstraintExtra, IndexExtra) are `any`:
// the driver needs no dialect-specific extra metadata.
type sqldDriver struct {
	catalog       *irv1.Catalog
	defaultSchema string
	mapper        *gotypes.Mapper
	types         drivers.Types
}

// newDriver builds a driver. typesPackage is the import path of the package that
// holds the shared enum/composite Go types (sqld-gen-go's output); it may be
// empty for schemas with no UDTs. ov is the (must-match-sqld-gen-go) override
// table; nullMode selects bob's null wrapping.
func newDriver(cat *irv1.Catalog, typesPackage string, ov gotypes.Overrides, null gotypes.NullMode) *sqldDriver {
	defaultSchema := cat.GetDefaultSchema()
	if defaultSchema == "" {
		defaultSchema = "public"
	}

	mapper := gotypes.NewMapper(cat, ov, null)
	if typesPackage != "" {
		mapper.SetUDTPackage(path.Base(typesPackage), typesPackage)
	}

	return &sqldDriver{
		catalog:       cat,
		defaultSchema: defaultSchema,
		mapper:        mapper,
		// Start from bob's curated base registry (int64, string, time.Time, …);
		// per-column types are layered on in table().
		types: helpers.Types(),
	}
}

func (d *sqldDriver) Dialect() string      { return "psql" }
func (d *sqldDriver) Types() drivers.Types { return d.types }

// Assemble walks the catalog and produces bob's DBInfo.
func (d *sqldDriver) Assemble(ctx context.Context) (*drivers.DBInfo[any, any, any], error) {
	info := &drivers.DBInfo[any, any, any]{Driver: "github.com/jackc/pgx/v5/stdlib"}

	for _, sc := range d.catalog.GetSchemas() {
		for _, tbl := range sc.GetTables() {
			info.Tables = append(info.Tables, d.table(sc.GetName(), tbl))
		}
	}
	// Enums intentionally left empty: sqld-gen-go owns the enum Go types, so
	// leaving DBInfo.Enums empty stops bob emitting a competing enum package
	// while the model fields still resolve to the shared types.

	if len(info.Tables) == 0 {
		return nil, fmt.Errorf("no tables found in catalog")
	}
	return info, nil
}

func (d *sqldDriver) table(schema string, tbl *irv1.Table) drivers.Table[any, any] {
	t := drivers.Table[any, any]{
		Key:    tbl.GetName().GetName(),
		Name:   tbl.GetName().GetName(),
		Schema: d.bobSchema(schema),
	}
	for _, col := range tbl.GetColumns() {
		// bob applies nullability itself (Column.Nullable + TypeSystem), so we
		// feed it the non-null base Go type and register it (with imports) so
		// bob emits the right import for that type.
		expr, imps := d.mapper.GoType(col.GetId(), col.GetType(), false)
		d.registerType(expr, imps)
		t.Columns = append(t.Columns, drivers.Column{
			Name:      col.GetName(),
			DBType:    col.GetType().GetPgName(),
			Type:      expr,
			Nullable:  col.GetNullable(),
			Generated: col.GetGenerated() != nil,
		})
	}
	t.Constraints = buildConstraints(tbl)
	return t
}

// registerType records a Go type expression in bob's Types registry with its
// imports so bob emits the import when a column uses it. Types already in the
// base registry (int64, string, time.Time, …) and bare builtins are skipped.
func (d *sqldDriver) registerType(expr string, imps []string) {
	if expr == "" || d.types.Contains(expr) {
		return
	}
	d.types.Register(expr, drivers.Type{Imports: quoteImports(imps)})
}

// quoteImports turns raw import paths into the quoted Go import specs bob's
// Type.Imports expects (e.g. "time" → `"time"`).
func quoteImports(imps []string) []string {
	if len(imps) == 0 {
		return nil
	}
	out := make([]string, 0, len(imps))
	for _, i := range imps {
		out = append(out, fmt.Sprintf("%q", i))
	}
	return out
}

// buildConstraints maps sqld's PK/unique/FK constraints onto bob's Constraints,
// so bob derives relationships and eager loaders. NOT NULL / CHECK / EXCLUSION
// constraints are skipped (they do not drive relationships). Constraint names
// are synthesized when the IR leaves them empty.
func buildConstraints(tbl *irv1.Table) drivers.Constraints[any] {
	var cons drivers.Constraints[any]
	table := tbl.GetName().GetName()
	for _, c := range tbl.GetConstraints() {
		switch c.GetType() {
		case irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY:
			name := c.GetName()
			if name == "" {
				name = table + "_pkey"
			}
			cons.Primary = &drivers.Constraint[any]{Name: name, Columns: c.GetPrimaryKey().GetColumns()}
		case irv1.ConstraintType_CONSTRAINT_TYPE_UNIQUE:
			name := c.GetName()
			if name == "" {
				name = fmt.Sprintf("%s_uniq_%d", table, len(cons.Uniques))
			}
			cons.Uniques = append(cons.Uniques, drivers.Constraint[any]{Name: name, Columns: c.GetUnique().GetColumns()})
		case irv1.ConstraintType_CONSTRAINT_TYPE_FOREIGN_KEY:
			fk := c.GetForeignKey()
			name := c.GetName()
			if name == "" {
				name = fmt.Sprintf("%s_fkey_%d", table, len(cons.Foreign))
			}
			cons.Foreign = append(cons.Foreign, drivers.ForeignKey[any]{
				Constraint:     drivers.Constraint[any]{Name: name, Columns: fk.GetColumns()},
				ForeignTable:   fk.GetReferencedTable().GetName().GetName(),
				ForeignColumns: fk.GetReferencedColumns(),
			})
		}
	}
	return cons
}

// bobSchema mirrors bob's psql driver: the default schema is emitted as an empty
// string so it is not prefixed onto generated identifiers.
func (d *sqldDriver) bobSchema(schema string) string {
	if schema == d.defaultSchema {
		return ""
	}
	return schema
}
