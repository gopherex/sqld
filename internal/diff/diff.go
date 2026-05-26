package diff

import (
	"sort"
	"strings"

	"google.golang.org/protobuf/proto"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// Diff compares two catalogs and produces an ordered Plan of changes that turns
// `from` into `to`. It handles schemas, types (enum/domain/composite/range),
// sequences, tables, columns, constraints, indexes, foreign keys, views,
// materialized views, functions, procedures and triggers.
//
// Iteration is deterministic: object ids are sorted before processing, so the
// resulting plan is stable for a given input pair.
func Diff(from, to *irv1.Catalog) (*Plan, error) {
	p := &Plan{}

	f := indexCatalog(from)
	t := indexCatalog(to)

	diffSchemas(p, f, t)
	diffEnums(p, f, t)
	diffDomains(p, f, t)
	diffComposites(p, f, t)
	diffRanges(p, f, t)
	diffSequences(p, f, t)
	diffTables(p, f, t)
	diffViews(p, f, t)
	diffMatViews(p, f, t)
	diffFunctions(p, f, t)
	diffProcedures(p, f, t)
	diffTriggers(p, f, t)

	return p, nil
}

// catalogIndex holds the objects of a catalog keyed by their canonical id for
// O(1) lookup during diffing.
type catalogIndex struct {
	schemas    map[string]*irv1.Schema
	enums      map[string]*irv1.EnumType
	domains    map[string]*irv1.DomainType
	composites map[string]*irv1.CompositeType
	ranges     map[string]*irv1.RangeType
	sequences  map[string]*irv1.Sequence
	tables     map[string]*irv1.Table
	views      map[string]*irv1.View
	matviews   map[string]*irv1.MaterializedView
	functions  map[string]*irv1.Function
	procedures map[string]*irv1.Procedure
	triggers   map[string]*irv1.Trigger
}

func indexCatalog(c *irv1.Catalog) *catalogIndex {
	idx := &catalogIndex{
		schemas:    map[string]*irv1.Schema{},
		enums:      map[string]*irv1.EnumType{},
		domains:    map[string]*irv1.DomainType{},
		composites: map[string]*irv1.CompositeType{},
		ranges:     map[string]*irv1.RangeType{},
		sequences:  map[string]*irv1.Sequence{},
		tables:     map[string]*irv1.Table{},
		views:      map[string]*irv1.View{},
		matviews:   map[string]*irv1.MaterializedView{},
		functions:  map[string]*irv1.Function{},
		procedures: map[string]*irv1.Procedure{},
		triggers:   map[string]*irv1.Trigger{},
	}
	for _, s := range c.GetSchemas() {
		idx.schemas[s.GetName()] = s
		for _, e := range s.GetEnums() {
			idx.enums[typeKey(e.GetName())] = e
		}
		for _, d := range s.GetDomains() {
			idx.domains[typeKey(d.GetName())] = d
		}
		for _, cp := range s.GetComposites() {
			idx.composites[typeKey(cp.GetName())] = cp
		}
		for _, r := range s.GetRanges() {
			idx.ranges[typeKey(r.GetName())] = r
		}
		for _, q := range s.GetSequences() {
			idx.sequences[typeKey(q.GetName())] = q
		}
		for _, tb := range s.GetTables() {
			idx.tables[tb.GetId()] = tb
		}
		for _, v := range s.GetViews() {
			idx.views[viewKey(s.GetName(), v)] = v
		}
		for _, mv := range s.GetMaterializedViews() {
			idx.matviews[typeKey(mv.GetName())] = mv
		}
		for _, fn := range s.GetFunctions() {
			idx.functions[funcKey(s.GetName(), fn.GetName(), fn.GetArguments())] = fn
		}
		for _, proc := range s.GetProcedures() {
			idx.procedures[funcKey(s.GetName(), proc.GetName(), proc.GetArguments())] = proc
		}
		for _, tr := range s.GetTriggers() {
			idx.triggers[triggerKey(s.GetName(), tr)] = tr
		}
	}
	return idx
}

// viewKey keys a view by schema.name.
func viewKey(schema string, v *irv1.View) string {
	return schema + "." + v.GetName().GetName()
}

// funcKey keys a function/procedure by schema, name, and its argument type
// signature so overloads are treated as distinct objects. The parse path does
// not fold args into the IR id, so the signature is derived here.
func funcKey(schema string, n *irv1.QualifiedName, args []*irv1.Argument) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		// OUT/TABLE params do not participate in the call signature.
		if a.GetMode() == irv1.ArgMode_ARG_MODE_OUT {
			continue
		}
		parts = append(parts, renderType(a.GetType()))
	}
	return schema + "." + n.GetName() + "(" + strings.Join(parts, ",") + ")"
}

// triggerKey keys a trigger by schema, target table, and trigger name (trigger
// names are unique per table, not per schema).
func triggerKey(schema string, tr *irv1.Trigger) string {
	tbl := ""
	if t := tr.GetTable(); t != nil {
		if id := t.GetId(); id != "" {
			tbl = id
		} else if n := t.GetName(); n != nil {
			tbl = n.GetName()
		}
	}
	return schema + "." + tbl + "." + tr.GetName()
}

// typeKey is the "schema.name" key for type/sequence-like objects.
func typeKey(n *irv1.QualifiedName) string {
	if n.GetSchema() == "" {
		return n.GetName()
	}
	return n.GetSchema() + "." + n.GetName()
}

// sortedKeys returns the keys of m in ascending order.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// isImplicitSchema reports whether a schema is created implicitly by the
// server (public) and therefore must not be emitted as a CREATE/DROP SCHEMA.
func isImplicitSchema(name string) bool {
	return name == "public"
}

func diffSchemas(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.schemas) {
		if isImplicitSchema(k) {
			continue
		}
		if _, ok := f.schemas[k]; !ok {
			p.Changes = append(p.Changes, &CreateSchema{Schema: t.schemas[k]})
		}
	}
	for _, k := range sortedKeys(f.schemas) {
		if isImplicitSchema(k) {
			continue
		}
		if _, ok := t.schemas[k]; !ok {
			p.Changes = append(p.Changes, &DropSchema{Schema: f.schemas[k]})
		}
	}
}

func diffEnums(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.enums) {
		te := t.enums[k]
		fe, ok := f.enums[k]
		if !ok {
			p.Changes = append(p.Changes, &CreateEnum{Enum: te})
			continue
		}
		diffEnumLabels(p, fe, te)
	}
	for _, k := range sortedKeys(f.enums) {
		if _, ok := t.enums[k]; !ok {
			p.Changes = append(p.Changes, &DropEnum{Enum: f.enums[k]})
		}
	}
}

// diffEnumLabels emits AddEnumValue for labels present in `to` but not `from`.
// Removed or reordered labels cannot be expressed cleanly in PostgreSQL, so a
// warning comment is emitted instead of a destructive change.
func diffEnumLabels(p *Plan, from, to *irv1.EnumType) {
	have := map[string]bool{}
	for _, l := range from.GetLabels() {
		have[l] = true
	}
	want := map[string]bool{}
	for _, l := range to.GetLabels() {
		want[l] = true
	}
	prev := ""
	for _, l := range to.GetLabels() {
		if !have[l] {
			p.Changes = append(p.Changes, &AddEnumValue{Enum: to, Value: l, After: prev})
		}
		prev = l
	}
	for _, l := range from.GetLabels() {
		if !want[l] {
			p.Changes = append(p.Changes, warnChange("cannot remove enum value "+quoteString(l)+
				" from "+qname(to.GetName())+" (PostgreSQL does not support dropping enum labels)", sortKeyType))
		}
	}
}

func diffDomains(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.domains) {
		if _, ok := f.domains[k]; !ok {
			p.Changes = append(p.Changes, &CreateDomain{Domain: t.domains[k]})
		}
	}
	for _, k := range sortedKeys(f.domains) {
		if _, ok := t.domains[k]; !ok {
			p.Changes = append(p.Changes, &DropDomain{Domain: f.domains[k]})
		}
	}
}

func diffComposites(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.composites) {
		if _, ok := f.composites[k]; !ok {
			p.Changes = append(p.Changes, &CreateComposite{Composite: t.composites[k]})
		}
	}
	for _, k := range sortedKeys(f.composites) {
		if _, ok := t.composites[k]; !ok {
			p.Changes = append(p.Changes, &DropComposite{Composite: f.composites[k]})
		}
	}
}

func diffRanges(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.ranges) {
		if _, ok := f.ranges[k]; !ok {
			p.Changes = append(p.Changes, &CreateRange{Range: t.ranges[k]})
		}
	}
	for _, k := range sortedKeys(f.ranges) {
		if _, ok := t.ranges[k]; !ok {
			p.Changes = append(p.Changes, &DropRange{Range: f.ranges[k]})
		}
	}
}

func diffSequences(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.sequences) {
		if _, ok := f.sequences[k]; !ok {
			p.Changes = append(p.Changes, &CreateSequence{Sequence: t.sequences[k]})
		}
	}
	for _, k := range sortedKeys(f.sequences) {
		if _, ok := t.sequences[k]; !ok {
			p.Changes = append(p.Changes, &DropSequence{Sequence: f.sequences[k]})
		}
	}
}

func diffTables(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.tables) {
		tt := t.tables[k]
		ft, ok := f.tables[k]
		if !ok {
			// New table: CreateTable carries the inline PRIMARY KEY; its other
			// constraints (FK/UNIQUE/CHECK/EXCLUSION) and indexes are emitted as
			// separate changes so they sort after every table is created.
			p.Changes = append(p.Changes, &CreateTable{Table: tt})
			for _, c := range tt.GetConstraints() {
				if isInlineConstraint(c) {
					continue
				}
				p.Changes = append(p.Changes, &AddConstraint{Table: tt, Constraint: c})
			}
			for _, idx := range tt.GetIndexes() {
				if idx.GetPrimary() {
					continue // backs the inline PK
				}
				p.Changes = append(p.Changes, &CreateIndex{Table: tt, Index: idx, Schema: tableSchema(tt)})
			}
			continue
		}
		diffColumns(p, ft, tt)
		diffConstraints(p, ft, tt)
		diffIndexes(p, ft, tt)
	}
	for _, k := range sortedKeys(f.tables) {
		if _, ok := t.tables[k]; !ok {
			p.Changes = append(p.Changes, &DropTable{Table: f.tables[k]})
		}
	}
}

// tableSchema returns the schema a table lives in (defaulting to public).
func tableSchema(t *irv1.Table) string {
	if s := t.GetName().GetSchema(); s != "" {
		return s
	}
	return "public"
}

// isInlineConstraint reports whether a constraint is already rendered inline by
// CreateTable (the primary key) or never rendered as a standalone constraint
// (NOT NULL, which is a column attribute).
func isInlineConstraint(c *irv1.Constraint) bool {
	switch c.GetType() {
	case irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY,
		irv1.ConstraintType_CONSTRAINT_TYPE_NOT_NULL:
		return true
	default:
		return false
	}
}

// constraintKey keys a constraint within a table. Named constraints key by
// name; unnamed ones (the parser leaves PK/auto constraints unnamed) key by a
// stable signature of their type and column set so they can still be matched.
func constraintKey(c *irv1.Constraint) string {
	if n := c.GetName(); n != "" {
		return n
	}
	var cols []string
	switch c.GetType() {
	case irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY:
		cols = c.GetPrimaryKey().GetColumns()
	case irv1.ConstraintType_CONSTRAINT_TYPE_UNIQUE:
		cols = c.GetUnique().GetColumns()
	case irv1.ConstraintType_CONSTRAINT_TYPE_FOREIGN_KEY:
		cols = c.GetForeignKey().GetColumns()
	}
	return "#" + c.GetType().String() + "(" + strings.Join(cols, ",") + ")"
}

// diffConstraints diffs the constraints of two versions of the same table.
// Primary keys and NOT NULL are handled by table/column diffing and skipped.
func diffConstraints(p *Plan, from, to *irv1.Table) {
	fc := indexConstraints(from)
	tc := indexConstraints(to)

	for _, k := range sortedKeys(tc) {
		t := tc[k]
		fv, ok := fc[k]
		if !ok {
			p.Changes = append(p.Changes, &AddConstraint{Table: to, Constraint: t})
			continue
		}
		if !constraintsEqual(fv, t) {
			// Changed: drop the old, add the new.
			p.Changes = append(p.Changes, &DropConstraint{Table: to, Constraint: fv})
			p.Changes = append(p.Changes, &AddConstraint{Table: to, Constraint: t})
		}
	}
	for _, k := range sortedKeys(fc) {
		if _, ok := tc[k]; !ok {
			p.Changes = append(p.Changes, &DropConstraint{Table: to, Constraint: fc[k]})
		}
	}
}

func indexConstraints(t *irv1.Table) map[string]*irv1.Constraint {
	m := map[string]*irv1.Constraint{}
	for _, c := range t.GetConstraints() {
		if isInlineConstraint(c) {
			continue
		}
		m[constraintKey(c)] = c
	}
	return m
}

// constraintsEqual compares two constraints by their rendered body so that
// definition changes (columns, referential actions, check expr) are detected.
func constraintsEqual(a, b *irv1.Constraint) bool {
	return renderConstraintBody(a) == renderConstraintBody(b)
}

// diffIndexes diffs the indexes of two versions of the same table by name.
// Indexes backing a primary key are owned by the PK constraint and skipped.
func diffIndexes(p *Plan, from, to *irv1.Table) {
	schema := tableSchema(to)
	fi := indexIndexes(from)
	ti := indexIndexes(to)

	for _, k := range sortedKeys(ti) {
		t := ti[k]
		fv, ok := fi[k]
		if !ok {
			p.Changes = append(p.Changes, &CreateIndex{Table: to, Index: t, Schema: schema})
			continue
		}
		if !indexesEqual(fv, t, from, to) {
			p.Changes = append(p.Changes, &DropIndex{Table: from, Index: fv, Schema: schema})
			p.Changes = append(p.Changes, &CreateIndex{Table: to, Index: t, Schema: schema})
		}
	}
	for _, k := range sortedKeys(fi) {
		if _, ok := ti[k]; !ok {
			p.Changes = append(p.Changes, &DropIndex{Table: from, Index: fi[k], Schema: schema})
		}
	}
}

func indexIndexes(t *irv1.Table) map[string]*irv1.Index {
	m := map[string]*irv1.Index{}
	for _, idx := range t.GetIndexes() {
		if idx.GetPrimary() {
			continue
		}
		m[idx.GetName()] = idx
	}
	return m
}

// indexesEqual compares two indexes by their rendered CREATE INDEX form.
func indexesEqual(a, b *irv1.Index, ta, tb *irv1.Table) bool {
	return renderCreateIndex(a, ta) == renderCreateIndex(b, tb)
}

// --- views -------------------------------------------------------------------

func diffViews(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.views) {
		tv := t.views[k]
		fv, ok := f.views[k]
		if !ok {
			p.Changes = append(p.Changes, &CreateView{View: tv})
			continue
		}
		if !viewsEqual(fv, tv) {
			p.Changes = append(p.Changes, &ReplaceView{From: fv, To: tv})
		}
	}
	for _, k := range sortedKeys(f.views) {
		if _, ok := t.views[k]; !ok {
			p.Changes = append(p.Changes, &DropView{View: f.views[k]})
		}
	}
}

func viewsEqual(a, b *irv1.View) bool {
	return renderCreateView(a, false) == renderCreateView(b, false)
}

func diffMatViews(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.matviews) {
		tv := t.matviews[k]
		fv, ok := f.matviews[k]
		if !ok {
			p.Changes = append(p.Changes, &CreateMatView{View: tv})
			continue
		}
		// Materialized views cannot be replaced in place; recreate on change.
		if renderCreateMatView(fv) != renderCreateMatView(tv) {
			p.Changes = append(p.Changes, &DropMatView{View: fv})
			p.Changes = append(p.Changes, &CreateMatView{View: tv})
		}
	}
	for _, k := range sortedKeys(f.matviews) {
		if _, ok := t.matviews[k]; !ok {
			p.Changes = append(p.Changes, &DropMatView{View: f.matviews[k]})
		}
	}
}

// --- functions and procedures ------------------------------------------------

func diffFunctions(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.functions) {
		tf := t.functions[k]
		ff, ok := f.functions[k]
		if !ok {
			p.Changes = append(p.Changes, &CreateFunction{Function: tf})
			continue
		}
		if renderCreateFunction(ff, false) != renderCreateFunction(tf, false) {
			p.Changes = append(p.Changes, &ReplaceFunction{From: ff, To: tf})
		}
	}
	for _, k := range sortedKeys(f.functions) {
		if _, ok := t.functions[k]; !ok {
			p.Changes = append(p.Changes, &DropFunction{Function: f.functions[k]})
		}
	}
}

func diffProcedures(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.procedures) {
		tp := t.procedures[k]
		fp, ok := f.procedures[k]
		if !ok {
			p.Changes = append(p.Changes, &CreateProcedure{Procedure: tp})
			continue
		}
		if renderCreateProcedure(fp, false) != renderCreateProcedure(tp, false) {
			p.Changes = append(p.Changes, &ReplaceProcedure{From: fp, To: tp})
		}
	}
	for _, k := range sortedKeys(f.procedures) {
		if _, ok := t.procedures[k]; !ok {
			p.Changes = append(p.Changes, &DropProcedure{Procedure: f.procedures[k]})
		}
	}
}

// --- triggers ----------------------------------------------------------------

func diffTriggers(p *Plan, f, t *catalogIndex) {
	for _, k := range sortedKeys(t.triggers) {
		tt := t.triggers[k]
		ft, ok := f.triggers[k]
		if !ok {
			p.Changes = append(p.Changes, &CreateTrigger{Trigger: tt})
			continue
		}
		// Triggers cannot be altered in place; recreate on change.
		if renderCreateTrigger(ft) != renderCreateTrigger(tt) {
			p.Changes = append(p.Changes, &DropTrigger{Trigger: ft})
			p.Changes = append(p.Changes, &CreateTrigger{Trigger: tt})
		}
	}
	for _, k := range sortedKeys(f.triggers) {
		if _, ok := t.triggers[k]; !ok {
			p.Changes = append(p.Changes, &DropTrigger{Trigger: f.triggers[k]})
		}
	}
}

// diffColumns diffs the columns of two versions of the same table by name.
// Constraints and indexes within the table are handled by the follow-up task.
func diffColumns(p *Plan, from, to *irv1.Table) {
	fcols := indexColumns(from)
	tcols := indexColumns(to)

	added := map[string]bool{}
	for _, k := range sortedKeys(tcols) {
		tc := tcols[k]
		fc, ok := fcols[k]
		if !ok {
			p.Changes = append(p.Changes, &AddColumn{Table: to, Column: tc})
			added[k] = true
			continue
		}
		diffColumn(p, to, fc, tc)
	}
	for _, k := range sortedKeys(fcols) {
		if _, ok := tcols[k]; !ok {
			p.Changes = append(p.Changes, &DropColumn{Table: to, Column: fcols[k]})
		}
	}
}

func indexColumns(t *irv1.Table) map[string]*irv1.Column {
	m := map[string]*irv1.Column{}
	for _, c := range t.GetColumns() {
		m[c.GetName()] = c
	}
	return m
}

// diffColumn emits the changes needed to turn column `from` into `to` on table
// `tbl`: type change, nullability change, default change.
func diffColumn(p *Plan, tbl *irv1.Table, from, to *irv1.Column) {
	if !typesEqual(from.GetType(), to.GetType()) {
		p.Changes = append(p.Changes, &AlterColumnType{
			Table:  tbl,
			Column: to.GetName(),
			From:   from.GetType(),
			To:     to.GetType(),
		})
	}

	switch {
	case from.GetNullable() && !to.GetNullable():
		p.Changes = append(p.Changes, &SetNotNull{Table: tbl, Column: to.GetName()})
	case !from.GetNullable() && to.GetNullable():
		p.Changes = append(p.Changes, &DropNotNull{Table: tbl, Column: to.GetName()})
	}

	fd, td := from.GetDefaultExpr(), to.GetDefaultExpr()
	if !defaultsEqual(fd, td) {
		switch {
		case td != nil:
			p.Changes = append(p.Changes, &SetDefault{Table: tbl, Column: to.GetName(), Default: td, Old: fd})
		case fd != nil:
			p.Changes = append(p.Changes, &DropDefault{Table: tbl, Column: to.GetName(), Old: fd})
		}
	}
}

// typesEqual compares two type references by their rendered DDL form so that
// equivalent types (including modifiers and array dimensions) are treated as
// unchanged.
func typesEqual(a, b *irv1.TypeRef) bool {
	return renderType(a) == renderType(b)
}

// defaultsEqual compares two default expressions by their rendered form, with
// proto equality as a fallback for nodes that render empty.
func defaultsEqual(a, b *irv1.Expr) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	if renderExpr(a) != renderExpr(b) {
		return false
	}
	return proto.Equal(a, b)
}

// warnChange is a no-op Change that renders a warning comment in both
// directions at the given sortKey.
func warnChange(msg string, key int) Change {
	return &comment{msg: "-- WARNING: " + msg, key: key}
}

type comment struct {
	msg string
	key int
}

func (c *comment) UpSQL() string   { return c.msg }
func (c *comment) DownSQL() string { return c.msg }
func (c *comment) sortKey() int    { return c.key }
