package main

import (
	"path"
	"strings"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// udtEntry holds a resolved UDT (enum, domain, or composite) together with
// the schema name it was declared in (needed for Go type naming).
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

// udtRegistry is an index of all UDTs in the catalog, keyed by bare type name
// (the Name.Name field — not schema-qualified).  When two schemas define a type
// with the same bare name the first one wins; that matches the PostgreSQL
// search_path behaviour that the host already applied.
type udtRegistry struct {
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

// buildUDTRegistry walks catalog schemas and builds a flat lookup table.
func buildUDTRegistry(catalog *irv1.Catalog) *udtRegistry {
	reg := &udtRegistry{
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

// udtGoTypeName returns the Go identifier for a UDT given its schema and bare name.
// public (or empty) schema → no prefix; other schemas → pascal(schema) prefix.
func udtGoTypeName(schema, name string) string {
	if schema == "public" || schema == "" {
		return pascal(name)
	}
	return pascal(schema) + pascal(name)
}

// overrides is a Go-type override table parsed from the plugin's options. Each
// key is EITHER a fully-qualified column id ("schema.table.column") OR a bare
// PostgreSQL type name (matched against TypeRef.PgName); each value is a Go type
// specification (see parseOverrideValue). Column-id overrides take precedence
// over type-name overrides, which take precedence over the default mapping.
type overrides map[string]string

// parseOverrideValue turns an override value into a Go type expression and the
// import (if any) it requires.
//
//   - A value containing "/" is an import path with the type name appended after
//     the LAST ".": "github.com/google/uuid.UUID" → import "github.com/google/uuid",
//     reference as "uuid.UUID" (the package name is assumed to equal the last
//     path segment).
//   - A value without "/" is a bare Go type used verbatim with no import, EXCEPT
//     that "json.RawMessage" (or any "json.*") adds the "encoding/json" import.
func parseOverrideValue(v string) (goExpr string, imports []string) {
	if strings.Contains(v, "/") {
		// import path + ".TypeName" split on the last dot.
		dot := strings.LastIndex(v, ".")
		if dot < 0 {
			// No type name; treat the whole thing as a bare type (degenerate).
			return v, nil
		}
		importPath := v[:dot]
		typeName := v[dot+1:]
		pkgName := path.Base(importPath)
		return pkgName + "." + typeName, []string{importPath}
	}
	// Bare Go type. Special-case the std-lib json package import.
	if v == "json.RawMessage" || strings.HasPrefix(v, "json.") {
		return v, []string{"encoding/json"}
	}
	return v, nil
}

// goTypeNoPointer reports whether a Go type expression should NOT be wrapped in
// a pointer when its column is nullable: slices, maps, json.RawMessage, and the
// pgtype.Range / pgtype.Multirange types (all of which already represent SQL
// NULL internally — slices/maps via a nil zero value, the pgtype range types via
// their Valid field).
func goTypeNoPointer(goExpr string) bool {
	return strings.HasPrefix(goExpr, "[]") ||
		strings.HasPrefix(goExpr, "map[") ||
		goExpr == "json.RawMessage" ||
		strings.HasPrefix(goExpr, "pgtype.Range[") ||
		strings.HasPrefix(goExpr, "pgtype.Multirange[")
}

// resolveGoType chooses the Go type for a value, applying overrides before
// falling back to the default mapping. Precedence:
//  1. an override keyed by the (non-empty) columnID;
//  2. an override keyed by the TypeRef's PostgreSQL name;
//  3. the default goType mapping.
//
// For override hits, a nullable value becomes a pointer unless the override Go
// type already has a nil-capable zero value (slice, map, or json.RawMessage).
func resolveGoType(reg *udtRegistry, ov overrides, columnID string, t *irv1.TypeRef, nullable bool) (goExpr string, imports []string) {
	if ov != nil {
		if columnID != "" {
			if v, ok := ov[columnID]; ok {
				return overrideGoType(v, nullable)
			}
		}
		if t != nil {
			if v, ok := ov[t.GetPgName()]; ok {
				return overrideGoType(v, nullable)
			}
		}
	}
	return goType(reg, t, nullable)
}

// overrideGoType resolves an override value to a (possibly pointer-wrapped) Go
// type expression and its imports.
func overrideGoType(v string, nullable bool) (string, []string) {
	expr, imps := parseOverrideValue(v)
	if nullable && !goTypeNoPointer(expr) {
		expr = "*" + expr
	}
	return expr, imps
}

// goType maps a TypeRef to a Go type expression and the imports it needs.
// Nullable scalars are wrapped in a pointer, except bytea ([]byte) and arrays
// which always remain as-is.
// reg may be nil (treated as empty registry), for callers that have no catalog.
func goType(reg *udtRegistry, t *irv1.TypeRef, nullable bool) (goExpr string, imports []string) {
	if t == nil {
		return "any", nil
	}

	if t.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
		elem := t.GetElement()
		elemExpr, elemImports := goType(reg, elem, false)
		return "[]" + elemExpr, elemImports
	}

	pgName := t.GetPgName()

	if reg != nil {
		// --- enum ---
		if entry, ok := reg.enums[pgName]; ok {
			typeName := udtGoTypeName(entry.schema, pgName)
			if nullable {
				return "*" + typeName, nil
			}
			return typeName, nil
		}
		// --- domain: transparent — recurse on base type ---
		if entry, ok := reg.domains[pgName]; ok {
			return goType(reg, entry.d.GetBaseType(), nullable)
		}
		// --- composite ---
		if entry, ok := reg.composites[pgName]; ok {
			typeName := udtGoTypeName(entry.schema, pgName)
			if nullable {
				return "*" + typeName, nil
			}
			return typeName, nil
		}
		// --- custom range (CREATE TYPE ... AS RANGE) ---
		//
		// Maps to pgtype.Range[<subtypeElem>] where <subtypeElem> is the pgtype
		// element type for the range's subtype (e.g. timestamptz → pgtype.Timestamptz).
		// pgtype.Range carries a Valid field, so a nullable custom range is a value
		// type, never a pointer (see goTypeNoPointer).
		if entry, ok := reg.ranges[pgName]; ok {
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
		if entry, ok := reg.multiranges[pgName]; ok {
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

	if nullable && !goTypeNoPointer(expr) {
		expr = "*" + expr
	}
	return expr, imps
}

// isCompositeType reports whether t resolves (through the registry) to a
// composite type. Domains are followed transparently to their base type.
func isCompositeType(reg *udtRegistry, t *irv1.TypeRef) bool {
	if reg == nil || t == nil {
		return false
	}
	if t.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
		return false
	}
	pgName := t.GetPgName()
	if _, ok := reg.composites[pgName]; ok {
		return true
	}
	if entry, ok := reg.domains[pgName]; ok {
		return isCompositeType(reg, entry.d.GetBaseType())
	}
	return false
}

// compositeDepName returns the bare name of the composite type that t depends
// on, or "" if t does not (transitively) resolve to a composite. It follows
// array element types (a field of `address[]` depends on `address`) and domains
// (transparently, to their base type). The returned name is the registry key
// (bare PostgreSQL type name), suitable for building the composite dependency
// graph used to topologically order RegisterTypes.
func compositeDepName(reg *udtRegistry, t *irv1.TypeRef) string {
	if reg == nil || t == nil {
		return ""
	}
	if t.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
		return compositeDepName(reg, t.GetElement())
	}
	pgName := t.GetPgName()
	if _, ok := reg.composites[pgName]; ok {
		return pgName
	}
	if entry, ok := reg.domains[pgName]; ok {
		return compositeDepName(reg, entry.d.GetBaseType())
	}
	return ""
}

// goParamType maps a query parameter's TypeRef to a Go type. It matches goType
// except that a composite parameter is always emitted as a value type (never a
// pointer): the generated composite codec reports IsNull()==false, so a NULL
// composite cannot be encoded and a pointer field would be meaningless. The
// caller passes the composite by value (e.g. SetAddressParams{Address: AppAddress{...}}).
func goParamType(reg *udtRegistry, t *irv1.TypeRef, nullable bool) (goExpr string, imports []string) {
	if isCompositeType(reg, t) {
		return goType(reg, t, false)
	}
	return goType(reg, t, nullable)
}

// resolveGoParamType is resolveGoType for query parameters: an override (by
// column id or type name) wins, otherwise it falls back to goParamType (which
// forces composite params to a value type).
func resolveGoParamType(reg *udtRegistry, ov overrides, columnID string, t *irv1.TypeRef, nullable bool) (goExpr string, imports []string) {
	if ov != nil {
		if columnID != "" {
			if v, ok := ov[columnID]; ok {
				return overrideGoType(v, nullable)
			}
		}
		if t != nil {
			if v, ok := ov[t.GetPgName()]; ok {
				return overrideGoType(v, nullable)
			}
		}
	}
	return goParamType(reg, t, nullable)
}

// pgtypeElement maps a PostgreSQL scalar type name to the pgtype element type
// used as the type parameter of pgtype.Range[T] / pgtype.Multirange[pgtype.Range[T]].
// It is the single source of truth for range subtype → pgtype element used by
// BOTH the builtin range table (scalarGoType) and custom AS RANGE types
// (goType). Returns ok=false for subtypes with no pgtype element mapping.
func pgtypeElement(pgName string) (string, bool) {
	switch pgName {
	case "int2":
		return "pgtype.Int2", true
	case "int4":
		return "pgtype.Int4", true
	case "int8":
		return "pgtype.Int8", true
	case "numeric":
		return "pgtype.Numeric", true
	case "float4":
		return "pgtype.Float4", true
	case "float8":
		return "pgtype.Float8", true
	case "bool":
		return "pgtype.Bool", true
	case "text", "varchar", "bpchar":
		return "pgtype.Text", true
	case "timestamptz":
		return "pgtype.Timestamptz", true
	case "timestamp":
		return "pgtype.Timestamp", true
	case "date":
		return "pgtype.Date", true
	default:
		return "", false
	}
}

// rangeGoType returns the pgtype.Range[T] Go expression for a builtin range
// whose subtype maps via pgtypeElement.
func rangeGoType(subPg string) (string, []string) {
	elem, _ := pgtypeElement(subPg)
	return "pgtype.Range[" + elem + "]", []string{"github.com/jackc/pgx/v5/pgtype"}
}

// multirangeGoType returns the pgtype.Multirange[pgtype.Range[T]] Go expression
// for a builtin multirange whose subtype maps via pgtypeElement.
func multirangeGoType(subPg string) (string, []string) {
	elem, _ := pgtypeElement(subPg)
	return "pgtype.Multirange[pgtype.Range[" + elem + "]]", []string{"github.com/jackc/pgx/v5/pgtype"}
}

// scalarGoType maps a PostgreSQL scalar type name to its Go expression.
func scalarGoType(pgName string) (string, []string) {
	switch pgName {
	case "int2", "smallserial":
		return "int16", nil
	case "int4", "serial":
		return "int32", nil
	case "int8", "bigserial":
		return "int64", nil
	case "bool":
		return "bool", nil
	case "float4":
		return "float32", nil
	case "float8":
		return "float64", nil
	case "text", "varchar", "bpchar", "name", "char", "citext":
		return "string", nil
	case "numeric", "money":
		return "string", nil
	case "timestamptz", "timestamp", "date", "time", "timetz":
		return "time.Time", []string{"time"}
	case "uuid":
		return "string", nil
	case "bytea":
		return "[]byte", nil
	case "json", "jsonb":
		return "json.RawMessage", []string{"encoding/json"}
	case "inet", "cidr", "macaddr":
		return "string", nil
	// --- builtin range types ---
	//
	// pgx ships built-in codecs for these, so no RegisterTypes entry is needed.
	// pgtype.Range[T] carries a Valid field, so a nullable range column is
	// represented by the value type (not a pointer): see goTypeNoPointer. The
	// subtype → pgtype element mapping is shared with custom AS RANGE types via
	// pgtypeElement.
	case "int4range":
		return rangeGoType("int4")
	case "int8range":
		return rangeGoType("int8")
	case "numrange":
		return rangeGoType("numeric")
	case "tsrange":
		return rangeGoType("timestamp")
	case "tstzrange":
		return rangeGoType("timestamptz")
	case "daterange":
		return rangeGoType("date")
	// --- builtin multirange types ---
	//
	// pgtype.Multirange[T] is []T where T is a pgtype.Range[...]; it also carries
	// NULL via IsNull(), so a nullable multirange column is a value (slice) type.
	case "int4multirange":
		return multirangeGoType("int4")
	case "int8multirange":
		return multirangeGoType("int8")
	case "nummultirange":
		return multirangeGoType("numeric")
	case "tsmultirange":
		return multirangeGoType("timestamp")
	case "tstzmultirange":
		return multirangeGoType("timestamptz")
	case "datemultirange":
		return multirangeGoType("date")
	default:
		return "any", nil
	}
}
