package diff

import (
	"sort"

	"google.golang.org/protobuf/proto"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// Diff compares two catalogs and produces an ordered Plan of changes that turns
// `from` into `to`. It handles schemas, types (enum/domain/composite/range),
// sequences, tables and columns. Constraints, indexes, views, functions and
// triggers are handled by a follow-up task and are intentionally not diffed
// here.
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
	}
	return idx
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
			p.Changes = append(p.Changes, &CreateTable{Table: tt})
			continue
		}
		diffColumns(p, ft, tt)
	}
	for _, k := range sortedKeys(f.tables) {
		if _, ok := t.tables[k]; !ok {
			p.Changes = append(p.Changes, &DropTable{Table: f.tables[k]})
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
