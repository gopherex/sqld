// Package relate derives a relationship graph from foreign-key metadata in an
// IR Catalog.
package relate

import (
	"sort"
	"strings"

	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

// Derive inspects every table's FK constraints in cat and returns a
// deterministic slice of Relationship edges.  It never panics.
func Derive(cat *irv1.Catalog) []*irv1.Relationship {
	if cat == nil {
		return []*irv1.Relationship{}
	}

	// --- 1. Build a table index (id → *Table) ---------------------------------
	tables := make(map[string]*irv1.Table)
	for _, schema := range cat.GetSchemas() {
		for _, tbl := range schema.GetTables() {
			tables[tbl.GetId()] = tbl
		}
	}

	// --- 2. Identify join tables -----------------------------------------------
	// A table is a join table when:
	//   - It has exactly 2 FK constraints.
	//   - It has a PK constraint.
	//   - Every column listed in the PK is covered by the union of the two FKs'
	//     local column sets (i.e. PK cols ⊆ FK1.cols ∪ FK2.cols).
	joinTables := make(map[string]bool) // set of table ids
	for _, schema := range cat.GetSchemas() {
		for _, tbl := range schema.GetTables() {
			if isJoinTable(tbl) {
				joinTables[tbl.GetId()] = true
			}
		}
	}

	// --- 3. Emit relationships in schema/table/constraint slice order ----------
	var out []*irv1.Relationship

	for _, schema := range cat.GetSchemas() {
		for _, tbl := range schema.GetTables() {
			if joinTables[tbl.GetId()] {
				// Emit exactly one MANY_TO_MANY for this join table.
				rel := buildManyToMany(tbl, tables)
				if rel != nil {
					out = append(out, rel)
				}
				continue
			}

			// Ordinary table: emit directed pairs for each FK.
			for _, c := range tbl.GetConstraints() {
				fk := c.GetForeignKey()
				if fk == nil {
					continue
				}
				refTable, ok := tables[fk.GetReferencedTable().GetId()]
				if !ok {
					// Referenced table is unknown — skip (already diagnosed by catalog).
					continue
				}

				optional := anyNullable(tbl, fk.GetColumns())
				unique := fkIsUnique(tbl, fk.GetColumns())

				var kindFwd, kindRev irv1.RelationshipKind
				if unique {
					kindFwd = irv1.RelationshipKind_RELATIONSHIP_KIND_ONE_TO_ONE
					kindRev = irv1.RelationshipKind_RELATIONSHIP_KIND_ONE_TO_ONE
				} else {
					kindFwd = irv1.RelationshipKind_RELATIONSHIP_KIND_MANY_TO_ONE
					kindRev = irv1.RelationshipKind_RELATIONSHIP_KIND_ONE_TO_MANY
				}

				viaConstraint := &irv1.ObjectRef{
					Id:   c.GetId(),
					Kind: irv1.ObjectKind_OBJECT_KIND_CONSTRAINT,
					Name: &irv1.QualifiedName{Name: c.GetName()},
				}

				fromRef := tableRef(tbl)
				toRef := tableRef(refTable)

				// Forward edge: referencing → referenced
				out = append(out, &irv1.Relationship{
					Id:            relID(tbl.GetId(), refTable.GetId(), fk.GetColumns()),
					Kind:          kindFwd,
					FromTable:     fromRef,
					FromColumns:   fk.GetColumns(),
					ToTable:       toRef,
					ToColumns:     fk.GetReferencedColumns(),
					ViaConstraint: viaConstraint,
					Optional:      optional,
					SuggestedName: refTable.GetName().GetName(),
				})

				// Reverse edge: referenced → referencing
				out = append(out, &irv1.Relationship{
					Id:            relID(refTable.GetId(), tbl.GetId(), fk.GetReferencedColumns()),
					Kind:          kindRev,
					FromTable:     toRef,
					FromColumns:   fk.GetReferencedColumns(),
					ToTable:       fromRef,
					ToColumns:     fk.GetColumns(),
					ViaConstraint: viaConstraint,
					Optional:      optional,
					SuggestedName: tbl.GetName().GetName(),
				})
			}
		}
	}

	return out
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// isJoinTable returns true when tbl is a pure association table:
//   - exactly 2 FK constraints
//   - has a PK constraint whose column set equals the union of the 2 FKs'
//     local column sets.
func isJoinTable(tbl *irv1.Table) bool {
	var fks []*irv1.Constraint
	var pk *irv1.PrimaryKey
	for _, c := range tbl.GetConstraints() {
		if c.GetForeignKey() != nil {
			fks = append(fks, c)
		}
		if c.GetPrimaryKey() != nil {
			pk = c.GetPrimaryKey()
		}
	}
	if len(fks) != 2 || pk == nil {
		return false
	}

	// Build union of FK local columns.
	fkCols := make(map[string]bool)
	for _, fk := range fks {
		for _, col := range fk.GetForeignKey().GetColumns() {
			fkCols[col] = true
		}
	}

	// Every PK column must be in the FK union.
	pkCols := pk.GetColumns()
	if len(pkCols) == 0 {
		return false
	}
	for _, col := range pkCols {
		if !fkCols[col] {
			return false
		}
	}
	return true
}

// buildManyToMany emits a single MANY_TO_MANY relationship for a join table.
// The first FK becomes the "from" side; the second becomes the "to" side.
// If either referenced table is missing from the index, nil is returned.
func buildManyToMany(join *irv1.Table, tables map[string]*irv1.Table) *irv1.Relationship {
	var fks []*irv1.Constraint
	for _, c := range join.GetConstraints() {
		if c.GetForeignKey() != nil {
			fks = append(fks, c)
		}
	}
	if len(fks) != 2 {
		return nil
	}

	fk0 := fks[0].GetForeignKey()
	fk1 := fks[1].GetForeignKey()

	fromTable, ok0 := tables[fk0.GetReferencedTable().GetId()]
	toTable, ok1 := tables[fk1.GetReferencedTable().GetId()]
	if !ok0 || !ok1 {
		return nil
	}

	joinRef := tableRef(join)

	return &irv1.Relationship{
		Id:          relID(fromTable.GetId(), toTable.GetId(), fk0.GetColumns()),
		Kind:        irv1.RelationshipKind_RELATIONSHIP_KIND_MANY_TO_MANY,
		FromTable:   tableRef(fromTable),
		FromColumns: fk0.GetReferencedColumns(),
		ToTable:     tableRef(toTable),
		ToColumns:   fk1.GetReferencedColumns(),
		JoinTable: &irv1.JoinTable{
			Table:       joinRef,
			FromColumns: fk0.GetColumns(),
			FromConstraint: &irv1.ObjectRef{
				Id:   fks[0].GetId(),
				Kind: irv1.ObjectKind_OBJECT_KIND_CONSTRAINT,
				Name: &irv1.QualifiedName{Name: fks[0].GetName()},
			},
			ToColumns: fk1.GetColumns(),
			ToConstraint: &irv1.ObjectRef{
				Id:   fks[1].GetId(),
				Kind: irv1.ObjectKind_OBJECT_KIND_CONSTRAINT,
				Name: &irv1.QualifiedName{Name: fks[1].GetName()},
			},
		},
		SuggestedName: toTable.GetName().GetName(),
	}
}

// fkIsUnique returns true if the given local column set is covered by a PK or
// UNIQUE constraint on tbl (i.e. the FK references a unique side).
func fkIsUnique(tbl *irv1.Table, fkCols []string) bool {
	target := colSet(fkCols)
	for _, c := range tbl.GetConstraints() {
		if pk := c.GetPrimaryKey(); pk != nil {
			if setsEqual(target, colSet(pk.GetColumns())) {
				return true
			}
		}
		if uq := c.GetUnique(); uq != nil {
			if setsEqual(target, colSet(uq.GetColumns())) {
				return true
			}
		}
	}
	return false
}

// anyNullable returns true if any of the named columns in tbl is nullable.
func anyNullable(tbl *irv1.Table, cols []string) bool {
	want := colSet(cols)
	for _, col := range tbl.GetColumns() {
		if want[col.GetName()] && col.GetNullable() {
			return true
		}
	}
	return false
}

// tableRef builds a lightweight ObjectRef for a table.
func tableRef(tbl *irv1.Table) *irv1.ObjectRef {
	return &irv1.ObjectRef{
		Id:   tbl.GetId(),
		Kind: irv1.ObjectKind_OBJECT_KIND_TABLE,
		Name: tbl.GetName(),
	}
}

// relID constructs a stable relationship id.
func relID(fromID, toID string, fromCols []string) string {
	cols := make([]string, len(fromCols))
	copy(cols, fromCols)
	sort.Strings(cols)
	return fromID + "->" + toID + ":" + strings.Join(cols, ",")
}

// colSet converts a slice of column names into a set (map).
func colSet(cols []string) map[string]bool {
	m := make(map[string]bool, len(cols))
	for _, c := range cols {
		m[c] = true
	}
	return m
}

// setsEqual returns true if two column-name sets are identical.
func setsEqual(a, b map[string]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}
