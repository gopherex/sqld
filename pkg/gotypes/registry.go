package gotypes

import (
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

// enumEntry holds a resolved enum together with the schema name it was declared
// in (needed for Go type naming).
type enumEntry struct {
	schema string
	e      *irv1.EnumType
}

type domainEntry struct {
	schema string
	d      *irv1.DomainType
}

type compositeEntry struct {
	schema string
	c      *irv1.CompositeType
}

type rangeEntry struct {
	schema string
	r      *irv1.RangeType
}

// multirangeEntry holds the schema and subtype of a custom multirange type (the
// MULTIRANGE auto-created for a custom CREATE TYPE ... AS RANGE). It is derived
// from a RangeType's Multirange name; the subtype is the range's subtype.
type multirangeEntry struct {
	schema  string
	subtype *irv1.TypeRef
}

// Registry is an index of all UDTs in the catalog, keyed by bare type name
// (the Name.Name field — not schema-qualified).  When two schemas define a type
// with the same bare name the first one wins; that matches the PostgreSQL
// search_path behaviour that the host already applied.
type Registry struct {
	enums      map[string]enumEntry
	domains    map[string]domainEntry
	composites map[string]compositeEntry
	// ranges holds custom CREATE TYPE ... AS RANGE types, keyed by bare type
	// name. Builtin ranges (int4range, …) are NOT here — they map directly in
	// scalarGoType and need no registration.
	ranges map[string]rangeEntry
	// multiranges holds the MULTIRANGE types auto-created for custom ranges,
	// keyed by bare multirange name (RangeType.Multirange). Builtin multiranges
	// (int4multirange, …) are NOT here — they map directly in scalarGoType.
	multiranges map[string]multirangeEntry
}

// BuildRegistry walks catalog schemas and builds a flat lookup table.
func BuildRegistry(catalog *irv1.Catalog) *Registry {
	reg := &Registry{
		enums:       make(map[string]enumEntry),
		domains:     make(map[string]domainEntry),
		composites:  make(map[string]compositeEntry),
		ranges:      make(map[string]rangeEntry),
		multiranges: make(map[string]multirangeEntry),
	}
	for _, schema := range catalog.GetSchemas() {
		sName := schema.GetName()
		for _, e := range schema.GetEnums() {
			name := e.GetName().GetName()
			if _, exists := reg.enums[name]; !exists {
				reg.enums[name] = enumEntry{schema: sName, e: e}
			}
		}
		for _, d := range schema.GetDomains() {
			name := d.GetName().GetName()
			if _, exists := reg.domains[name]; !exists {
				reg.domains[name] = domainEntry{schema: sName, d: d}
			}
		}
		for _, c := range schema.GetComposites() {
			name := c.GetName().GetName()
			if _, exists := reg.composites[name]; !exists {
				reg.composites[name] = compositeEntry{schema: sName, c: c}
			}
		}
		for _, r := range schema.GetRanges() {
			name := r.GetName().GetName()
			if _, exists := reg.ranges[name]; !exists {
				reg.ranges[name] = rangeEntry{schema: sName, r: r}
			}
			// Register the associated multirange (PG 14+) keyed by its bare
			// name, sharing the range's subtype so a multirange column maps to
			// pgtype.Multirange[pgtype.Range[<subtypeElem>]].
			if mr := r.GetMultirange(); mr != "" {
				if _, exists := reg.multiranges[mr]; !exists {
					reg.multiranges[mr] = multirangeEntry{schema: sName, subtype: r.GetSubtype()}
				}
			}
		}
	}
	return reg
}

// IsEnum reports whether pgName (bare PostgreSQL type name) is a registered enum.
func (r *Registry) IsEnum(pgName string) bool {
	if r == nil {
		return false
	}
	_, ok := r.enums[pgName]
	return ok
}

// EnumSchema returns the schema an enum was declared in, and ok=false when
// pgName is not a registered enum.
func (r *Registry) EnumSchema(pgName string) (string, bool) {
	if r == nil {
		return "", false
	}
	entry, ok := r.enums[pgName]
	if !ok {
		return "", false
	}
	return entry.schema, true
}

// IsComposite reports whether pgName (bare PostgreSQL type name) is a registered
// composite type.
func (r *Registry) IsComposite(pgName string) bool {
	if r == nil {
		return false
	}
	_, ok := r.composites[pgName]
	return ok
}

// CompositeSchema returns the schema a composite was declared in, and ok=false
// when pgName is not a registered composite.
func (r *Registry) CompositeSchema(pgName string) (string, bool) {
	if r == nil {
		return "", false
	}
	entry, ok := r.composites[pgName]
	if !ok {
		return "", false
	}
	return entry.schema, true
}
