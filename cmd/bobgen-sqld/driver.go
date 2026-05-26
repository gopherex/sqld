package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	helpers "github.com/stephenafamo/bob/gen/bobgen-helpers"
	"github.com/stephenafamo/bob/gen/drivers"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// sharedPkgPath is the import path of the hand-written package that stands in
// for the canonical types sqld's own Go generator would emit.
const sharedPkgPath = "github.com/yaroher/sqld/cmd/bobgen-sqld/pocshared"

// sharedImport is the quoted, aliased import spec bob expects in drivers.Type.Imports.
const sharedImport = `pocshared "` + sharedPkgPath + `"`

// enumSharedTypes maps a Postgres enum type name to the canonical Go type
// expression (and registry key) that sqld controls. This is the linchpin: by
// pointing a column at one of these keys, bob is forced to use OUR type instead
// of generating its own enum type.
//
// In the real feature this map would be derived from the sqld<->sqld-gen-go
// naming contract; here we hard-code the single PoC enum.
var enumSharedTypes = map[string]string{
	"account_status": "pocshared.AccountStatus",
}

// sqldDriver implements drivers.Interface backed by a loaded sqld IR Catalog.
// The three type parameters are: DBExtra, ConstraintExtra, IndexExtra — all `any`
// for the PoC since we do not need driver-specific extra metadata.
type sqldDriver struct {
	catalog       *irv1.Catalog
	defaultSchema string
	types         drivers.Types
	// emitBobEnums controls whether enum types are also reported in DBInfo.Enums.
	// false  -> bob generates NO enum type; the column resolves to our shared type.
	// true   -> bob ALSO generates its own enums package (the type fight, documented).
	emitBobEnums bool
}

func newSQLDDriver(cat *irv1.Catalog, emitBobEnums bool) *sqldDriver {
	defaultSchema := cat.GetDefaultSchema()
	if defaultSchema == "" {
		defaultSchema = "public"
	}

	// Start from bob's curated base type registry (int64, string, bool,
	// time.Time, []byte, ...). We then layer the sqld-controlled types on top.
	types := helpers.Types()

	// Register every sqld-controlled enum type. The registry KEY is the literal
	// Go type expression bob will write into the model field; Imports[0] is
	// matched against the current package and the alias is stripped when local.
	for _, goType := range enumSharedTypes {
		types.Register(goType, drivers.Type{
			Imports: []string{sharedImport},
		})
	}

	return &sqldDriver{
		catalog:       cat,
		defaultSchema: defaultSchema,
		types:         types,
		emitBobEnums:  emitBobEnums,
	}
}

func (d *sqldDriver) Dialect() string { return "psql" }

func (d *sqldDriver) Types() drivers.Types { return d.types }

// enumNames returns the set of Postgres enum type names declared in the catalog,
// so that scalar columns whose pgName matches can be recognised as enum columns.
// (sqld's catalog currently emits enum columns with kind=SCALAR and pgName set
// to the enum's name rather than kind=ENUM; see report.)
func (d *sqldDriver) enumNames() map[string]*irv1.EnumType {
	out := map[string]*irv1.EnumType{}
	for _, sc := range d.catalog.GetSchemas() {
		for _, e := range sc.GetEnums() {
			out[e.GetName().GetName()] = e
		}
	}
	return out
}

// goTypeForColumn returns the bob registry key (== Go type expression) for a
// column, registering any needed type on the fly. This is where sqld dictates
// the emitted Go type.
func (d *sqldDriver) goTypeForColumn(col *irv1.Column, enums map[string]*irv1.EnumType) string {
	tr := col.GetType()
	pg := tr.GetPgName()

	// Enum columns: force our shared canonical type.
	if _, isEnum := enums[pg]; isEnum {
		if shared, ok := enumSharedTypes[pg]; ok {
			return shared
		}
	}
	// Also honour an explicitly-resolved UDT enum name (future-proofing for when
	// sqld sets kind=ENUM + udt).
	if tr.GetKind() == irv1.TypeKind_TYPE_KIND_ENUM {
		name := tr.GetUdt().GetName().GetName()
		if shared, ok := enumSharedTypes[name]; ok {
			return shared
		}
	}

	switch pg {
	case "int8", "bigint", "bigserial", "serial8":
		return "int64"
	case "int4", "int", "integer", "serial", "serial4":
		return "int32"
	case "int2", "smallint", "smallserial", "serial2":
		return "int16"
	case "text", "varchar", "char", "bpchar", "name", "citext":
		return "string"
	case "bool", "boolean":
		return "bool"
	case "float8", "double precision":
		return "float64"
	case "float4", "real":
		return "float32"
	case "timestamptz", "timestamp", "timestamp with time zone", "timestamp without time zone", "date", "timetz", "time":
		// time.Time is already registered (with the "time" import) by helpers.Types().
		return "time.Time"
	case "bytea":
		return "[]byte"
	default:
		// Unknown type: fall back to string so the PoC still compiles.
		return "string"
	}
}

// Assemble walks the sqld catalog and produces bob's DBInfo.
func (d *sqldDriver) Assemble(ctx context.Context) (*drivers.DBInfo[any, any, any], error) {
	enums := d.enumNames()

	info := &drivers.DBInfo[any, any, any]{
		Driver: "github.com/jackc/pgx/v5/stdlib",
	}

	for _, sc := range d.catalog.GetSchemas() {
		schemaName := sc.GetName()

		for _, tbl := range sc.GetTables() {
			t := drivers.Table[any, any]{
				Key:    tbl.GetName().GetName(),
				Name:   tbl.GetName().GetName(),
				Schema: bobSchema(schemaName, d.defaultSchema),
			}

			for _, col := range tbl.GetColumns() {
				t.Columns = append(t.Columns, drivers.Column{
					Name:     col.GetName(),
					DBType:   col.GetType().GetPgName(),
					Nullable: col.GetNullable(),
					Type:     d.goTypeForColumn(col, enums),
				})
			}

			t.Constraints = buildConstraints(tbl)
			info.Tables = append(info.Tables, t)
		}
	}

	if d.emitBobEnums {
		seen := map[string]bool{}
		for _, sc := range d.catalog.GetSchemas() {
			for _, e := range sc.GetEnums() {
				name := e.GetName().GetName()
				if seen[name] {
					continue
				}
				seen[name] = true
				info.Enums = append(info.Enums, drivers.Enum{
					Type:   bobEnumTypeName(name),
					Values: e.GetLabels(),
				})
			}
		}
		sort.Slice(info.Enums, func(i, j int) bool { return info.Enums[i].Type < info.Enums[j].Type })
	}

	if len(info.Tables) == 0 {
		return nil, fmt.Errorf("no tables found in catalog")
	}

	return info, nil
}

// buildConstraints maps sqld constraints onto bob's Constraints struct. For the
// PoC we map the primary key (required so bob generates a table, not a view) and
// unique constraints. Foreign keys are mapped too when present.
func buildConstraints(tbl *irv1.Table) drivers.Constraints[any] {
	var cons drivers.Constraints[any]
	pkIdx := 0
	for _, c := range tbl.GetConstraints() {
		switch c.GetType() {
		case irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY:
			name := c.GetName()
			if name == "" {
				name = tbl.GetName().GetName() + "_pkey"
			}
			cons.Primary = &drivers.Constraint[any]{
				Name:    name,
				Columns: c.GetPrimaryKey().GetColumns(),
			}
			pkIdx++
		case irv1.ConstraintType_CONSTRAINT_TYPE_UNIQUE:
			name := c.GetName()
			if name == "" {
				name = fmt.Sprintf("%s_uniq_%d", tbl.GetName().GetName(), len(cons.Uniques))
			}
			cons.Uniques = append(cons.Uniques, drivers.Constraint[any]{
				Name:    name,
				Columns: c.GetUnique().GetColumns(),
			})
		case irv1.ConstraintType_CONSTRAINT_TYPE_FOREIGN_KEY:
			fk := c.GetForeignKey()
			name := c.GetName()
			if name == "" {
				name = fmt.Sprintf("%s_fkey_%d", tbl.GetName().GetName(), len(cons.Foreign))
			}
			cons.Foreign = append(cons.Foreign, drivers.ForeignKey[any]{
				Constraint: drivers.Constraint[any]{
					Name:    name,
					Columns: fk.GetColumns(),
				},
				ForeignTable:   fk.GetReferencedTable().GetName().GetName(),
				ForeignColumns: fk.GetReferencedColumns(),
			})
		}
	}
	return cons
}

// bobSchema mirrors bob's psql driver: the shared/default schema is emitted as
// an empty string so it is not prefixed onto generated identifiers.
func bobSchema(schema, defaultSchema string) string {
	if schema == defaultSchema {
		return ""
	}
	return schema
}

// bobEnumTypeName mirrors bob's TitleCase enum naming (used only when emitting
// bob-owned enums for the comparison case).
func bobEnumTypeName(name string) string {
	parts := strings.Split(name, "_")
	for i, p := range parts {
		if p == "" {
			continue
		}
		parts[i] = strings.ToUpper(p[:1]) + p[1:]
	}
	return strings.Join(parts, "")
}
