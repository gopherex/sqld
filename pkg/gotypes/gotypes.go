package gotypes

import (
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// NullMode selects how a nullable column's Go type is wrapped.
type NullMode int

const (
	// Pointer wraps nullable scalars/enums/composites/overrides in a pointer
	// (*T), leaving nil-capable types (slices, maps, json.RawMessage, pgtype
	// struct/range types) unwrapped. This is the historical sqld-gen-go mode.
	Pointer NullMode = iota
	// Opt wraps nullable values in null.Val[T] (github.com/aarondl/opt/null),
	// leaving the same nil-capable types unwrapped. Used by sqld-gen-bob.
	Opt
)

// Overrides is a Go-type override table parsed from the plugin's options. Each
// key is EITHER a fully-qualified column id ("schema.table.column") OR a bare
// PostgreSQL type name (matched against TypeRef.PgName); each value is a Go type
// specification (see parseOverrideValue). Column-id overrides take precedence
// over type-name overrides, which take precedence over the default mapping.
type Overrides map[string]string

// Mapper maps PostgreSQL TypeRefs to Go type expressions using a UDT registry,
// an override table, and a null-wrapping mode.
type Mapper struct {
	reg  *Registry
	ov   Overrides
	null NullMode
	// udtPkg, when non-empty, qualifies enum/composite Go type names with this
	// package alias (e.g. "db" → "db.AppUserStatus") and adds udtImport to the
	// returned imports. Used by sqld-gen-bob, where the canonical UDT Go types
	// live in a separate package (sqld-gen-go's output) rather than locally.
	// Empty (the default) preserves sqld-gen-go's same-package behaviour.
	udtPkg    string
	udtImport string
}

// NewMapper builds a registry from cat and returns a Mapper.
func NewMapper(cat *irv1.Catalog, ov Overrides, null NullMode) *Mapper {
	return &Mapper{reg: BuildRegistry(cat), ov: ov, null: null}
}

// NewMapper2 returns a Mapper that uses an existing registry.
func NewMapper2(reg *Registry, ov Overrides, null NullMode) *Mapper {
	return &Mapper{reg: reg, ov: ov, null: null}
}

// Registry returns the Mapper's UDT registry.
func (m *Mapper) Registry() *Registry {
	return m.reg
}

// SetUDTPackage makes the Mapper qualify enum/composite Go type names with the
// given package alias and add importPath (a raw, unquoted import path — the same
// form scalarGoType returns) to their imports. With an empty alias the Mapper
// emits unqualified UDT names (the default). Returns the Mapper for chaining.
func (m *Mapper) SetUDTPackage(alias, importPath string) *Mapper {
	m.udtPkg = alias
	m.udtImport = importPath
	return m
}

// qualifyUDT applies the configured UDT package alias to a bare UDT type name.
func (m *Mapper) qualifyUDT(name string) (string, []string) {
	if m.udtPkg == "" {
		return name, nil
	}
	return m.udtPkg + "." + name, []string{m.udtImport}
}

// UDTName returns the Go identifier for a UDT given its schema and bare name.
func (m *Mapper) UDTName(schema, pgName string) string {
	return UDTName(schema, pgName)
}

// wrapNull applies the Mapper's null mode to expr when nullable. nil-capable
// types (see goTypeNoPointer) are never wrapped. In Pointer mode the result is
// "*"+expr; in Opt mode it is null.Val[expr] with the opt/null import added.
func (m *Mapper) wrapNull(expr string, imps []string, nullable bool) (string, []string) {
	if !nullable || goTypeNoPointer(expr) {
		return expr, imps
	}
	switch m.null {
	case Opt:
		return "null.Val[" + expr + "]", append(imps, "github.com/aarondl/opt/null")
	default:
		return "*" + expr, imps
	}
}

// GoType chooses the Go type for a value, applying overrides before falling back
// to the default mapping. Precedence:
//  1. an override keyed by the (non-empty) columnID;
//  2. an override keyed by the TypeRef's PostgreSQL name;
//  3. the default mapping.
//
// For override hits, a nullable value is wrapped via wrapNull unless the
// override Go type already has a nil-capable zero value.
func (m *Mapper) GoType(columnID string, t *irv1.TypeRef, nullable bool) (goExpr string, imports []string) {
	if m.ov != nil {
		if columnID != "" {
			if v, ok := m.ov[columnID]; ok {
				return m.overrideGoType(v, nullable)
			}
		}
		if t != nil {
			if v, ok := m.ov[t.GetPgName()]; ok {
				return m.overrideGoType(v, nullable)
			}
		}
	}
	return m.goType(t, nullable)
}

// ParamType maps a query parameter's TypeRef to a Go type. It matches GoType
// except that a composite parameter is always emitted as a value type (never a
// pointer): the generated composite codec reports IsNull()==false, so a NULL
// composite cannot be encoded and a pointer field would be meaningless. The
// caller passes the composite by value (e.g. SetAddressParams{Address: AppAddress{...}}).
//
// An override (by column id or type name) wins, otherwise it falls back to the
// composite-aware default mapping.
func (m *Mapper) ParamType(columnID string, t *irv1.TypeRef, nullable bool) (goExpr string, imports []string) {
	if m.ov != nil {
		if columnID != "" {
			if v, ok := m.ov[columnID]; ok {
				return m.overrideGoType(v, nullable)
			}
		}
		if t != nil {
			if v, ok := m.ov[t.GetPgName()]; ok {
				return m.overrideGoType(v, nullable)
			}
		}
	}
	return m.goParamType(t, nullable)
}

// overrideGoType resolves an override value to a (possibly wrapped) Go type
// expression and its imports.
func (m *Mapper) overrideGoType(v string, nullable bool) (string, []string) {
	expr, imps := parseOverrideValue(v)
	return m.wrapNull(expr, imps, nullable)
}

// goType maps a TypeRef to a Go type expression and the imports it needs.
// Nullable scalars are wrapped via wrapNull, except nil-capable types (bytea
// []byte, json.RawMessage, pgtype struct/range types) and arrays which always
// remain as-is. The registry may be nil (treated as empty), for callers that
// have no catalog.
func (m *Mapper) goType(t *irv1.TypeRef, nullable bool) (goExpr string, imports []string) {
	if t == nil {
		return "any", nil
	}

	if t.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
		elem := t.GetElement()
		elemExpr, elemImports := m.goType(elem, false)
		return "[]" + elemExpr, elemImports
	}

	pgName := t.GetPgName()

	if m.reg != nil {
		// --- enum ---
		if entry, ok := m.reg.enums[pgName]; ok {
			name, imps := m.qualifyUDT(UDTName(entry.schema, pgName))
			return m.wrapNull(name, imps, nullable)
		}
		// --- domain: transparent — recurse on base type ---
		if entry, ok := m.reg.domains[pgName]; ok {
			return m.goType(entry.d.GetBaseType(), nullable)
		}
		// --- composite ---
		if entry, ok := m.reg.composites[pgName]; ok {
			name, imps := m.qualifyUDT(UDTName(entry.schema, pgName))
			return m.wrapNull(name, imps, nullable)
		}
		// --- custom range (CREATE TYPE ... AS RANGE) ---
		//
		// Maps to pgtype.Range[<subtypeElem>] where <subtypeElem> is the pgtype
		// element type for the range's subtype (e.g. timestamptz → pgtype.Timestamptz).
		// pgtype.Range carries a Valid field, so a nullable custom range is a value
		// type, never a pointer (see goTypeNoPointer).
		if entry, ok := m.reg.ranges[pgName]; ok {
			subPg := entry.r.GetSubtype().GetPgName()
			elem, ok := pgtypeElement(subPg)
			if !ok {
				// Unknown subtype: fall back to pgtype.Range[pgtype.Text] as a
				// best-effort (the scan may need a manual override).
				elem = "pgtype.Text"
			}
			return "pgtype.Range[" + elem + "]", []string{"github.com/jackc/pgx/v5/pgtype"}
		}
		// --- custom multirange (auto-created for CREATE TYPE ... AS RANGE) ---
		//
		// Maps to pgtype.Multirange[pgtype.Range[<subtypeElem>]] where
		// <subtypeElem> is the pgtype element for the range's subtype. Like
		// pgtype.Range, pgtype.Multirange carries Valid/IsNull, so a nullable
		// custom multirange is a value type, never a pointer (see goTypeNoPointer).
		if entry, ok := m.reg.multiranges[pgName]; ok {
			subPg := entry.subtype.GetPgName()
			elem, ok := pgtypeElement(subPg)
			if !ok {
				// Unknown subtype: fall back like the range case.
				elem = "pgtype.Text"
			}
			return "pgtype.Multirange[pgtype.Range[" + elem + "]]", []string{"github.com/jackc/pgx/v5/pgtype"}
		}
	}

	expr, imps := scalarGoType(pgName)
	return m.wrapNull(expr, imps, nullable)
}

// goParamType is goType for query parameters: a composite parameter is always
// emitted as a value type (never wrapped).
func (m *Mapper) goParamType(t *irv1.TypeRef, nullable bool) (goExpr string, imports []string) {
	if IsCompositeType(m.reg, t) {
		return m.goType(t, false)
	}
	return m.goType(t, nullable)
}
