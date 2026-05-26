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

// udtRegistry is an index of all UDTs in the catalog, keyed by bare type name
// (the Name.Name field — not schema-qualified).  When two schemas define a type
// with the same bare name the first one wins; that matches the PostgreSQL
// search_path behaviour that the host already applied.
type udtRegistry struct {
	enums      map[string]enumEntry
	domains    map[string]domainEntry
	composites map[string]compositeEntry
}

// buildUDTRegistry walks catalog schemas and builds a flat lookup table.
func buildUDTRegistry(catalog *irv1.Catalog) *udtRegistry {
	reg := &udtRegistry{
		enums:      make(map[string]enumEntry),
		domains:    make(map[string]domainEntry),
		composites: make(map[string]compositeEntry),
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
	// represented by the value type (not a pointer): see goTypeNoPointer.
	case "int4range":
		return "pgtype.Range[pgtype.Int4]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "int8range":
		return "pgtype.Range[pgtype.Int8]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "numrange":
		return "pgtype.Range[pgtype.Numeric]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "tsrange":
		return "pgtype.Range[pgtype.Timestamp]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "tstzrange":
		return "pgtype.Range[pgtype.Timestamptz]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "daterange":
		return "pgtype.Range[pgtype.Date]", []string{"github.com/jackc/pgx/v5/pgtype"}
	// --- builtin multirange types ---
	//
	// pgtype.Multirange[T] is []T where T is a pgtype.Range[...]; it also carries
	// NULL via IsNull(), so a nullable multirange column is a value (slice) type.
	case "int4multirange":
		return "pgtype.Multirange[pgtype.Range[pgtype.Int4]]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "int8multirange":
		return "pgtype.Multirange[pgtype.Range[pgtype.Int8]]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "nummultirange":
		return "pgtype.Multirange[pgtype.Range[pgtype.Numeric]]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "tsmultirange":
		return "pgtype.Multirange[pgtype.Range[pgtype.Timestamp]]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "tstzmultirange":
		return "pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "datemultirange":
		return "pgtype.Multirange[pgtype.Range[pgtype.Date]]", []string{"github.com/jackc/pgx/v5/pgtype"}
	default:
		return "any", nil
	}
}
