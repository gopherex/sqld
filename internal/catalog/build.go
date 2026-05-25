// Package catalog assembles an IR Catalog from parsed DDL statements.
package catalog

import (
	"fmt"

	pg "github.com/pganalyze/pg_query_go/v6"
	"github.com/yaroher/sqld/internal/mapper"
	"github.com/yaroher/sqld/internal/parse"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// ---------------------------------------------------------------------------
// Diagnostics
// ---------------------------------------------------------------------------

// Diagnostics accumulates non-fatal issues found while building the catalog.
type Diagnostics struct {
	Items []Diagnostic
}

// Diagnostic is a single non-fatal issue with severity and message.
type Diagnostic struct {
	Severity string // "info" | "warning" | "error"
	Message  string
}

// Add appends a diagnostic entry.
func (d *Diagnostics) Add(sev, msg string) {
	d.Items = append(d.Items, Diagnostic{sev, msg})
}

// ---------------------------------------------------------------------------
// Internal builder state
// ---------------------------------------------------------------------------

// schemaState holds the in-progress schema and an ordered list of table names
// (schema-qualified) to support index attachment.
type schemaState struct {
	schema *irv1.Schema
	// tableIndex maps table name (lower-case) to its *irv1.Table for quick lookup.
	tableIndex map[string]*irv1.Table
}

// builder holds all mutable state during a Build call.
type builder struct {
	diag        *Diagnostics
	schemas     map[string]*schemaState // keyed by schema name
	schemaOrder []string                // first-seen order
}

func newBuilder() *builder {
	return &builder{
		diag:    &Diagnostics{},
		schemas: make(map[string]*schemaState),
	}
}

// getOrCreateSchema returns the schemaState for a given name, creating it on demand.
func (b *builder) getOrCreateSchema(name string) *schemaState {
	if ss, ok := b.schemas[name]; ok {
		return ss
	}
	ss := &schemaState{
		schema: &irv1.Schema{
			Id:   name,
			Name: name,
		},
		tableIndex: make(map[string]*irv1.Table),
	}
	b.schemas[name] = ss
	b.schemaOrder = append(b.schemaOrder, name)
	return ss
}

// resolveSchema returns the schema name from a QualifiedName (or "public").
func resolveSchema(qn *irv1.QualifiedName) string {
	if qn != nil && qn.GetSchema() != "" {
		return qn.GetSchema()
	}
	return "public"
}

// ---------------------------------------------------------------------------
// Build
// ---------------------------------------------------------------------------

// Build assembles a Catalog from parsed statements (schema DDL + applied
// migration up statements, in order). It never panics; unknown/odd shapes
// result in a diagnostic and are skipped.
func Build(stmts []parse.Stmt) (*irv1.Catalog, *Diagnostics) {
	b := newBuilder()

	for _, stmt := range stmts {
		if stmt.Node == nil {
			b.diag.Add("warning", fmt.Sprintf("stmt %d: nil node, skipping", stmt.Index))
			continue
		}
		b.dispatch(stmt)
	}

	// Assign IDs and resolve FK references after all objects are placed.
	b.assignIDs()
	b.resolveFKs()

	// Build ordered schemas slice.
	schemas := make([]*irv1.Schema, 0, len(b.schemaOrder))
	for _, name := range b.schemaOrder {
		schemas = append(schemas, b.schemas[name].schema)
	}

	cat := &irv1.Catalog{
		DefaultSchema: "public",
		Schemas:       schemas,
	}
	return cat, b.diag
}

// ---------------------------------------------------------------------------
// Statement dispatch
// ---------------------------------------------------------------------------

func (b *builder) dispatch(stmt parse.Stmt) {
	node := stmt.Node

	switch {
	// CREATE SCHEMA
	case node.GetCreateSchemaStmt() != nil:
		b.handleCreateSchema(node.GetCreateSchemaStmt())

	// CREATE TABLE
	case node.GetCreateStmt() != nil:
		b.handleCreateTable(node.GetCreateStmt())

	// CREATE VIEW
	case node.GetViewStmt() != nil:
		b.handleView(node.GetViewStmt())

	// CREATE TYPE ... AS ENUM
	case node.GetCreateEnumStmt() != nil:
		b.handleCreateEnum(node.GetCreateEnumStmt())

	// CREATE DOMAIN
	case node.GetCreateDomainStmt() != nil:
		b.handleCreateDomain(node.GetCreateDomainStmt())

	// CREATE TYPE ... AS (composite)
	case node.GetCompositeTypeStmt() != nil:
		b.handleCompositeType(node.GetCompositeTypeStmt())

	// CREATE FUNCTION / CREATE PROCEDURE
	case node.GetCreateFunctionStmt() != nil:
		b.handleCreateFunction(node.GetCreateFunctionStmt())

	// CREATE TRIGGER
	case node.GetCreateTrigStmt() != nil:
		b.handleCreateTrigger(node.GetCreateTrigStmt())

	// CREATE MATERIALIZED VIEW (and plain CREATE TABLE AS — check objtype)
	case node.GetCreateTableAsStmt() != nil:
		b.handleCreateTableAs(node.GetCreateTableAsStmt())

	// CREATE INDEX
	case node.GetIndexStmt() != nil:
		b.handleIndex(node.GetIndexStmt())

	// CREATE SEQUENCE
	case node.GetCreateSeqStmt() != nil:
		b.handleCreateSequence(node.GetCreateSeqStmt())

	// DROP
	case node.GetDropStmt() != nil:
		b.handleDrop(node.GetDropStmt())

	// ALTER TABLE
	case node.GetAlterTableStmt() != nil:
		b.handleAlterTable(node.GetAlterTableStmt())

	default:
		b.diag.Add("info", fmt.Sprintf("unhandled statement type at index %d", stmt.Index))
	}
}

// ---------------------------------------------------------------------------
// CREATE TABLE
// ---------------------------------------------------------------------------

func (b *builder) handleCreateTable(cs *pg.CreateStmt) {
	tbl := mapper.MapCreateTable(cs)
	if tbl == nil {
		b.diag.Add("warning", "MapCreateTable returned nil")
		return
	}
	schemaName := resolveSchema(tbl.GetName())
	// Ensure schema name is set on the table's QualifiedName.
	if tbl.Name != nil {
		tbl.Name.Schema = schemaName
	}
	ss := b.getOrCreateSchema(schemaName)
	ss.schema.Tables = append(ss.schema.Tables, tbl)
	ss.tableIndex[tbl.GetName().GetName()] = tbl
}

// ---------------------------------------------------------------------------
// CREATE VIEW
// ---------------------------------------------------------------------------

func (b *builder) handleView(vs *pg.ViewStmt) {
	v := mapper.MapView(vs)
	if v == nil {
		b.diag.Add("warning", "MapView returned nil")
		return
	}
	schemaName := resolveSchema(v.GetName())
	if v.Name != nil {
		v.Name.Schema = schemaName
	}
	ss := b.getOrCreateSchema(schemaName)
	ss.schema.Views = append(ss.schema.Views, v)
}

// ---------------------------------------------------------------------------
// CREATE TYPE AS ENUM
// ---------------------------------------------------------------------------

func (b *builder) handleCreateEnum(es *pg.CreateEnumStmt) {
	et := mapper.MapCreateEnum(es)
	if et == nil {
		b.diag.Add("warning", "MapCreateEnum returned nil")
		return
	}
	schemaName := resolveSchema(et.GetName())
	if et.Name != nil {
		et.Name.Schema = schemaName
	}
	ss := b.getOrCreateSchema(schemaName)
	ss.schema.Enums = append(ss.schema.Enums, et)
}

// ---------------------------------------------------------------------------
// CREATE INDEX
// ---------------------------------------------------------------------------

func (b *builder) handleIndex(is *pg.IndexStmt) {
	idx := mapper.MapIndex(is)
	if idx == nil {
		b.diag.Add("warning", "MapIndex returned nil")
		return
	}
	// Determine the target table.
	rv := is.GetRelation()
	if rv == nil {
		b.diag.Add("warning", fmt.Sprintf("CREATE INDEX %q: no relation, skipping", idx.GetName()))
		return
	}
	schemaName := rv.GetSchemaname()
	if schemaName == "" {
		schemaName = "public"
	}
	tableName := rv.GetRelname()

	ss := b.getOrCreateSchema(schemaName)
	tbl, ok := ss.tableIndex[tableName]
	if !ok {
		b.diag.Add("warning", fmt.Sprintf("CREATE INDEX %q: table %q.%q not found in catalog",
			idx.GetName(), schemaName, tableName))
		return
	}
	tbl.Indexes = append(tbl.Indexes, idx)
}

// ---------------------------------------------------------------------------
// CREATE SEQUENCE
// ---------------------------------------------------------------------------

func (b *builder) handleCreateSequence(cs *pg.CreateSeqStmt) {
	seq := mapper.MapCreateSequence(cs)
	if seq == nil {
		b.diag.Add("warning", "MapCreateSequence returned nil")
		return
	}
	schemaName := resolveSchema(seq.GetName())
	if seq.Name != nil {
		seq.Name.Schema = schemaName
	}
	ss := b.getOrCreateSchema(schemaName)
	ss.schema.Sequences = append(ss.schema.Sequences, seq)
}

// ---------------------------------------------------------------------------
// CREATE SCHEMA
// ---------------------------------------------------------------------------

func (b *builder) handleCreateSchema(cs *pg.CreateSchemaStmt) {
	schemaName := cs.GetSchemaname()
	if schemaName == "" {
		b.diag.Add("warning", "CREATE SCHEMA: empty schema name, skipping")
		return
	}
	ss := b.getOrCreateSchema(schemaName)
	// Set owner from AUTHORIZATION role if provided (best-effort).
	if ar := cs.GetAuthrole(); ar != nil && ar.GetRolename() != "" {
		ss.schema.Owner = ar.GetRolename()
	}
}

// ---------------------------------------------------------------------------
// CREATE DOMAIN
// ---------------------------------------------------------------------------

func (b *builder) handleCreateDomain(ds *pg.CreateDomainStmt) {
	dt := mapper.MapCreateDomain(ds)
	if dt == nil {
		b.diag.Add("warning", "MapCreateDomain returned nil")
		return
	}
	schemaName := resolveSchema(dt.GetName())
	if dt.Name != nil {
		dt.Name.Schema = schemaName
	}
	ss := b.getOrCreateSchema(schemaName)
	ss.schema.Domains = append(ss.schema.Domains, dt)
}

// ---------------------------------------------------------------------------
// CREATE TYPE ... AS (composite)
// ---------------------------------------------------------------------------

func (b *builder) handleCompositeType(cs *pg.CompositeTypeStmt) {
	ct := mapper.MapCompositeType(cs)
	if ct == nil {
		b.diag.Add("warning", "MapCompositeType returned nil")
		return
	}
	schemaName := resolveSchema(ct.GetName())
	if ct.Name != nil {
		ct.Name.Schema = schemaName
	}
	ss := b.getOrCreateSchema(schemaName)
	ss.schema.Composites = append(ss.schema.Composites, ct)
}

// ---------------------------------------------------------------------------
// CREATE FUNCTION / CREATE PROCEDURE
// ---------------------------------------------------------------------------

func (b *builder) handleCreateFunction(cf *pg.CreateFunctionStmt) {
	fn, proc := mapper.MapCreateFunction(cf)
	if fn != nil {
		schemaName := resolveSchema(fn.GetName())
		if fn.Name != nil {
			fn.Name.Schema = schemaName
		}
		ss := b.getOrCreateSchema(schemaName)
		ss.schema.Functions = append(ss.schema.Functions, fn)
	} else if proc != nil {
		schemaName := resolveSchema(proc.GetName())
		if proc.Name != nil {
			proc.Name.Schema = schemaName
		}
		ss := b.getOrCreateSchema(schemaName)
		ss.schema.Procedures = append(ss.schema.Procedures, proc)
	} else {
		b.diag.Add("warning", "MapCreateFunction returned (nil, nil)")
	}
}

// ---------------------------------------------------------------------------
// CREATE TRIGGER
// ---------------------------------------------------------------------------

func (b *builder) handleCreateTrigger(ct *pg.CreateTrigStmt) {
	trig := mapper.MapCreateTrigger(ct)
	if trig == nil {
		b.diag.Add("warning", "MapCreateTrigger returned nil")
		return
	}
	// Schema for the trigger is determined by the table it's ON.
	var schemaName string
	if trig.GetTable() != nil && trig.GetTable().GetName() != nil {
		schemaName = trig.GetTable().GetName().GetSchema()
	}
	if schemaName == "" {
		schemaName = "public"
	}
	ss := b.getOrCreateSchema(schemaName)
	ss.schema.Triggers = append(ss.schema.Triggers, trig)
}

// ---------------------------------------------------------------------------
// CREATE MATERIALIZED VIEW (CREATE TABLE AS with objtype=OBJECT_MATVIEW)
// ---------------------------------------------------------------------------

func (b *builder) handleCreateTableAs(cta *pg.CreateTableAsStmt) {
	if cta.GetObjtype() != pg.ObjectType_OBJECT_MATVIEW {
		b.diag.Add("info", fmt.Sprintf("CREATE TABLE AS with objtype %v: not a materialized view, skipping", cta.GetObjtype()))
		return
	}
	mv := mapper.MapMaterializedView(cta)
	if mv == nil {
		b.diag.Add("warning", "MapMaterializedView returned nil")
		return
	}
	schemaName := resolveSchema(mv.GetName())
	if mv.Name != nil {
		mv.Name.Schema = schemaName
	}
	ss := b.getOrCreateSchema(schemaName)
	ss.schema.MaterializedViews = append(ss.schema.MaterializedViews, mv)
}

// ---------------------------------------------------------------------------
// DROP
// ---------------------------------------------------------------------------

func (b *builder) handleDrop(ds *pg.DropStmt) {
	switch ds.GetRemoveType() {
	case pg.ObjectType_OBJECT_TABLE:
		for _, obj := range ds.GetObjects() {
			name, schemaName := dropObjName(obj)
			if name == "" {
				continue
			}
			ss := b.getOrCreateSchema(schemaName)
			ss.schema.Tables = removeTable(ss.schema.Tables, name)
			delete(ss.tableIndex, name)
			b.diag.Add("info", fmt.Sprintf("DROP TABLE %q.%q", schemaName, name))
		}

	case pg.ObjectType_OBJECT_VIEW:
		for _, obj := range ds.GetObjects() {
			name, schemaName := dropObjName(obj)
			if name == "" {
				continue
			}
			ss := b.getOrCreateSchema(schemaName)
			ss.schema.Views = removeView(ss.schema.Views, name)
			b.diag.Add("info", fmt.Sprintf("DROP VIEW %q.%q", schemaName, name))
		}

	case pg.ObjectType_OBJECT_INDEX:
		for _, obj := range ds.GetObjects() {
			name, schemaName := dropObjName(obj)
			if name == "" {
				continue
			}
			// Remove index from all tables in schema.
			if ss, ok := b.schemas[schemaName]; ok {
				for _, tbl := range ss.schema.Tables {
					tbl.Indexes = removeIndex(tbl.Indexes, name)
				}
			}
			b.diag.Add("info", fmt.Sprintf("DROP INDEX %q.%q", schemaName, name))
		}

	case pg.ObjectType_OBJECT_SEQUENCE:
		for _, obj := range ds.GetObjects() {
			name, schemaName := dropObjName(obj)
			if name == "" {
				continue
			}
			ss := b.getOrCreateSchema(schemaName)
			ss.schema.Sequences = removeSequence(ss.schema.Sequences, name)
			b.diag.Add("info", fmt.Sprintf("DROP SEQUENCE %q.%q", schemaName, name))
		}

	case pg.ObjectType_OBJECT_TYPE:
		for _, obj := range ds.GetObjects() {
			name, schemaName := dropTypeObjName(obj)
			if name == "" {
				continue
			}
			ss := b.getOrCreateSchema(schemaName)
			ss.schema.Enums = removeEnum(ss.schema.Enums, name)
			b.diag.Add("info", fmt.Sprintf("DROP TYPE %q.%q", schemaName, name))
		}

	default:
		b.diag.Add("info", fmt.Sprintf("DROP of type %v: best-effort skipped", ds.GetRemoveType()))
	}
}

// dropObjName extracts the (name, schema) from a DROP object node.
// For tables/views/sequences the objects list contains RangeVar nodes (or
// plain String_ nodes).  Returns ("", "public") when not parseable.
func dropObjName(obj *pg.Node) (name, schema string) {
	schema = "public"
	if rv := obj.GetRangeVar(); rv != nil {
		name = rv.GetRelname()
		if rv.GetSchemaname() != "" {
			schema = rv.GetSchemaname()
		}
		return
	}
	// Some DROP forms use a List of String_ nodes: [schema, name] or [name].
	if lst := obj.GetList(); lst != nil {
		parts := make([]string, 0, 2)
		for _, item := range lst.GetItems() {
			if s := item.GetString_().GetSval(); s != "" {
				parts = append(parts, s)
			}
		}
		switch len(parts) {
		case 1:
			name = parts[0]
		case 2:
			schema = parts[0]
			name = parts[1]
		}
		return
	}
	if s := obj.GetString_().GetSval(); s != "" {
		name = s
	}
	return
}

// dropTypeObjName handles DROP TYPE whose objects are TypeName nodes.
func dropTypeObjName(obj *pg.Node) (name, schema string) {
	schema = "public"
	if tn := obj.GetTypeName(); tn != nil {
		parts := make([]string, 0, 2)
		for _, n := range tn.GetNames() {
			if s := n.GetString_().GetSval(); s != "" && s != "pg_catalog" {
				parts = append(parts, s)
			}
		}
		switch len(parts) {
		case 1:
			name = parts[0]
		case 2:
			schema = parts[0]
			name = parts[1]
		}
		return
	}
	return dropObjName(obj)
}

// ---------------------------------------------------------------------------
// ALTER TABLE
// ---------------------------------------------------------------------------

func (b *builder) handleAlterTable(at *pg.AlterTableStmt) {
	rv := at.GetRelation()
	if rv == nil {
		b.diag.Add("warning", "ALTER TABLE: no relation")
		return
	}
	schemaName := rv.GetSchemaname()
	if schemaName == "" {
		schemaName = "public"
	}
	tableName := rv.GetRelname()

	ss, ok := b.schemas[schemaName]
	if !ok {
		b.diag.Add("warning", fmt.Sprintf("ALTER TABLE %q.%q: schema not found", schemaName, tableName))
		return
	}
	tbl, ok := ss.tableIndex[tableName]
	if !ok {
		b.diag.Add("warning", fmt.Sprintf("ALTER TABLE %q.%q: table not found", schemaName, tableName))
		return
	}

	for _, cmdNode := range at.GetCmds() {
		cmd := cmdNode.GetAlterTableCmd()
		if cmd == nil {
			b.diag.Add("warning", fmt.Sprintf("ALTER TABLE %q.%q: non-AlterTableCmd node", schemaName, tableName))
			continue
		}
		b.applyAlterCmd(tbl, cmd, schemaName, tableName)
	}
}

func (b *builder) applyAlterCmd(tbl *irv1.Table, cmd *pg.AlterTableCmd, schemaName, tableName string) {
	switch cmd.GetSubtype() {
	case pg.AlterTableType_AT_AddColumn, pg.AlterTableType_AT_AddColumnToView:
		cd := cmd.GetDef().GetColumnDef()
		if cd == nil {
			b.diag.Add("warning", fmt.Sprintf("ALTER TABLE %q.%q ADD COLUMN: missing ColumnDef", schemaName, tableName))
			return
		}
		pos := uint32(len(tbl.Columns) + 1)
		col, colConstraints := mapper.MapColumn(cd, pos), []*irv1.Constraint(nil)
		// Also extract column-level constraints.
		for _, cNode := range cd.GetConstraints() {
			c := cNode.GetConstraint()
			if c == nil {
				continue
			}
			switch c.GetContype() {
			case pg.ConstrType_CONSTR_PRIMARY,
				pg.ConstrType_CONSTR_UNIQUE,
				pg.ConstrType_CONSTR_FOREIGN,
				pg.ConstrType_CONSTR_CHECK,
				pg.ConstrType_CONSTR_NOTNULL:
				if ic := mapper.MapConstraint(c, cd.GetColname()); ic != nil {
					colConstraints = append(colConstraints, ic)
				}
			}
		}
		if col != nil {
			tbl.Columns = append(tbl.Columns, col)
		}
		tbl.Constraints = append(tbl.Constraints, colConstraints...)

	case pg.AlterTableType_AT_AddConstraint:
		c := cmd.GetDef().GetConstraint()
		if c == nil {
			b.diag.Add("warning", fmt.Sprintf("ALTER TABLE %q.%q ADD CONSTRAINT: missing Constraint node", schemaName, tableName))
			return
		}
		if ic := mapper.MapConstraint(c, ""); ic != nil {
			tbl.Constraints = append(tbl.Constraints, ic)
		}

	default:
		b.diag.Add("info", fmt.Sprintf("ALTER TABLE %q.%q: subcommand %v not handled, skipping",
			schemaName, tableName, cmd.GetSubtype()))
	}
}

// ---------------------------------------------------------------------------
// ID assignment pass
// ---------------------------------------------------------------------------

func (b *builder) assignIDs() {
	for _, schemaName := range b.schemaOrder {
		ss := b.schemas[schemaName]

		for _, tbl := range ss.schema.Tables {
			tableID := schemaName + "." + tbl.GetName().GetName()
			tbl.Id = tableID

			for _, col := range tbl.GetColumns() {
				col.Id = tableID + "." + col.GetName()
			}

			for i, c := range tbl.GetConstraints() {
				if c.GetName() != "" {
					c.Id = tableID + "." + c.GetName()
				} else {
					c.Id = fmt.Sprintf("%s.constraint_%d", tableID, i)
				}
			}
		}

		for _, v := range ss.schema.Views {
			if v.Id == "" && v.GetName() != nil {
				v.Id = schemaName + "." + v.GetName().GetName()
			}
		}

		for _, seq := range ss.schema.Sequences {
			if seq.Id == "" && seq.GetName() != nil {
				seq.Id = schemaName + "." + seq.GetName().GetName()
			}
		}

		for _, et := range ss.schema.Enums {
			if et.Id == "" && et.GetName() != nil {
				et.Id = schemaName + "." + et.GetName().GetName()
			}
		}

		for _, dt := range ss.schema.Domains {
			if dt.Id == "" && dt.GetName() != nil {
				dt.Id = schemaName + "." + dt.GetName().GetName()
			}
		}

		for _, ct := range ss.schema.Composites {
			if ct.Id == "" && ct.GetName() != nil {
				ct.Id = schemaName + "." + ct.GetName().GetName()
			}
		}

		for _, fn := range ss.schema.Functions {
			if fn.Id == "" && fn.GetName() != nil {
				fn.Id = schemaName + "." + fn.GetName().GetName()
			}
		}

		for _, proc := range ss.schema.Procedures {
			if proc.Id == "" && proc.GetName() != nil {
				proc.Id = schemaName + "." + proc.GetName().GetName()
			}
		}

		for _, mv := range ss.schema.MaterializedViews {
			if mv.Id == "" && mv.GetName() != nil {
				mv.Id = schemaName + "." + mv.GetName().GetName()
			}
		}

		for _, trig := range ss.schema.Triggers {
			if trig.Id == "" {
				trig.Id = schemaName + "." + trig.GetName()
			}
		}
	}

	// Second pass: resolve trigger table/function ObjectRef IDs after all
	// objects have been assigned IDs in the first pass above.
	b.resolveTriggers()
}

// ---------------------------------------------------------------------------
// FK resolution pass
// ---------------------------------------------------------------------------

func (b *builder) resolveFKs() {
	for _, schemaName := range b.schemaOrder {
		ss := b.schemas[schemaName]
		for _, tbl := range ss.schema.Tables {
			for _, c := range tbl.GetConstraints() {
				fk := c.GetForeignKey()
				if fk == nil || fk.GetReferencedTable() == nil {
					continue
				}
				ref := fk.GetReferencedTable()
				refName := ref.GetName()
				if refName == nil {
					continue
				}
				refSchema := refName.GetSchema()
				if refSchema == "" {
					refSchema = "public"
				}
				refTableName := refName.GetName()

				refID := refSchema + "." + refTableName

				// Check that the referenced table exists.
				if refSS, ok := b.schemas[refSchema]; ok {
					if _, exists := refSS.tableIndex[refTableName]; exists {
						ref.Id = refID
						ref.Kind = irv1.ObjectKind_OBJECT_KIND_TABLE
						// Also set schema on referenced name for clarity.
						ref.Name.Schema = refSchema
						continue
					}
				}
				// Referenced table not in catalog — keep name, add warning.
				ref.Id = refID // still set the id for best-effort
				ref.Kind = irv1.ObjectKind_OBJECT_KIND_TABLE
				b.diag.Add("warning", fmt.Sprintf("FK in table %q references unknown table %q", tbl.GetId(), refID))
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Trigger resolution pass
// ---------------------------------------------------------------------------

// resolveTriggers resolves Trigger.Table.Id and Trigger.Function.Id for every
// trigger in the catalog, and appends a back-ref ObjectRef to the referenced
// Table.Triggers slice when the table exists.
func (b *builder) resolveTriggers() {
	for _, schemaName := range b.schemaOrder {
		ss := b.schemas[schemaName]
		for _, trig := range ss.schema.Triggers {
			// --- resolve Table ref ---
			if tableRef := trig.GetTable(); tableRef != nil {
				tableQN := tableRef.GetName()
				refSchema := schemaName // default to trigger's own schema
				if tableQN != nil && tableQN.GetSchema() != "" {
					refSchema = tableQN.GetSchema()
				}
				refTableName := ""
				if tableQN != nil {
					refTableName = tableQN.GetName()
				}
				tableID := refSchema + "." + refTableName
				tableRef.Id = tableID
				tableRef.Kind = irv1.ObjectKind_OBJECT_KIND_TABLE
				if tableQN != nil {
					tableQN.Schema = refSchema
				}

				// Append back-ref to Table.Triggers if the table exists.
				if refSS, ok := b.schemas[refSchema]; ok {
					if refTbl, ok := refSS.tableIndex[refTableName]; ok {
						trigRef := &irv1.ObjectRef{
							Id:   trig.GetId(),
							Kind: irv1.ObjectKind_OBJECT_KIND_TRIGGER,
							Name: &irv1.QualifiedName{
								Schema: schemaName,
								Name:   trig.GetName(),
							},
						}
						refTbl.Triggers = append(refTbl.Triggers, trigRef)
					} else {
						b.diag.Add("warning", fmt.Sprintf("trigger %q: referenced table %q not found in catalog",
							trig.GetId(), tableID))
					}
				} else {
					b.diag.Add("warning", fmt.Sprintf("trigger %q: referenced schema %q not found",
						trig.GetId(), refSchema))
				}
			}

			// --- resolve Function ref ---
			if fnRef := trig.GetFunction(); fnRef != nil {
				fnQN := fnRef.GetName()
				refFnSchema := schemaName // default to trigger's own schema
				if fnQN != nil && fnQN.GetSchema() != "" {
					refFnSchema = fnQN.GetSchema()
				}
				refFnName := ""
				if fnQN != nil {
					refFnName = fnQN.GetName()
				}
				fnRef.Id = refFnSchema + "." + refFnName
				fnRef.Kind = irv1.ObjectKind_OBJECT_KIND_FUNCTION
				if fnQN != nil {
					fnQN.Schema = refFnSchema
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Removal helpers
// ---------------------------------------------------------------------------

func removeTable(tables []*irv1.Table, name string) []*irv1.Table {
	out := tables[:0]
	for _, t := range tables {
		if t.GetName().GetName() != name {
			out = append(out, t)
		}
	}
	return out
}

func removeView(views []*irv1.View, name string) []*irv1.View {
	out := views[:0]
	for _, v := range views {
		if v.GetName().GetName() != name {
			out = append(out, v)
		}
	}
	return out
}

func removeIndex(indexes []*irv1.Index, name string) []*irv1.Index {
	out := indexes[:0]
	for _, idx := range indexes {
		if idx.GetName() != name {
			out = append(out, idx)
		}
	}
	return out
}

func removeSequence(seqs []*irv1.Sequence, name string) []*irv1.Sequence {
	out := seqs[:0]
	for _, s := range seqs {
		if s.GetName().GetName() != name {
			out = append(out, s)
		}
	}
	return out
}

func removeEnum(enums []*irv1.EnumType, name string) []*irv1.EnumType {
	out := enums[:0]
	for _, e := range enums {
		if e.GetName().GetName() != name {
			out = append(out, e)
		}
	}
	return out
}
