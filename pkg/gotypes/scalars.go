package gotypes

import (
	"path"
	"sort"
	"strings"

	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

// extensionTypeNames lists the PostgreSQL extension type names that need a
// runtime LoadType + RegisterType (their OIDs are not known at compile time and
// pgx has no built-in default codec keyed by name). Each maps to a Go type in
// scalarGoType (hstore → pgtype.Hstore; ltree/lquery → string). Element types
// have no dependencies, so they are registered FIRST in RegisterTypes.
var extensionTypeNames = map[string]bool{
	"hstore": true,
	"ltree":  true,
	"lquery": true,
}

// collectExtensionTypeName records the bare PostgreSQL name of t (following
// array element types) into used if it is a known extension type.
func collectExtensionTypeName(t *irv1.TypeRef, used map[string]bool) {
	if t == nil {
		return
	}
	if t.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
		collectExtensionTypeName(t.GetElement(), used)
		return
	}
	if extensionTypeNames[t.GetPgName()] {
		used[t.GetPgName()] = true
	}
}

// CollectUsedExtensionTypes scans the catalog (all schemas → tables → columns)
// and the query result columns + parameters for extension types (hstore, ltree,
// lquery) that are actually used, returning their names sorted for deterministic
// output. Only used extension types are registered in RegisterTypes.
func CollectUsedExtensionTypes(catalog *irv1.Catalog, queries []*pluginv1.Query) []string {
	used := make(map[string]bool)
	for _, schema := range catalog.GetSchemas() {
		for _, table := range schema.GetTables() {
			for _, col := range table.GetColumns() {
				collectExtensionTypeName(col.GetType(), used)
			}
		}
	}
	for _, q := range queries {
		for _, c := range q.GetColumns() {
			collectExtensionTypeName(c.GetType(), used)
		}
		for _, p := range q.GetParameters() {
			collectExtensionTypeName(p.GetType(), used)
		}
	}
	out := make([]string, 0, len(used))
	for name := range used {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// UDTName returns the Go identifier for a UDT given its schema and bare name.
// public (or empty) schema → no prefix; other schemas → Pascal(schema) prefix.
func UDTName(schema, name string) string {
	if schema == "public" || schema == "" {
		return Pascal(name)
	}
	return Pascal(schema) + Pascal(name)
}

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

// ParseOverrideValue is the exported form of parseOverrideValue, used by
// callers (and tests) that need to resolve an override value directly.
func ParseOverrideValue(v string) (goExpr string, imports []string) {
	return parseOverrideValue(v)
}

// pgtypeStructTypes is the set of pgtype struct Go types used by scalarGoType
// (core geometric, interval, bit, and identifier types). Each carries a Valid
// field that represents SQL NULL internally, so a nullable column of one of
// these types is a value type, never a pointer (see goTypeNoPointer).
var pgtypeStructTypes = map[string]bool{
	"pgtype.Interval": true,
	"pgtype.Point":    true,
	"pgtype.Line":     true,
	"pgtype.Lseg":     true,
	"pgtype.Box":      true,
	"pgtype.Path":     true,
	"pgtype.Polygon":  true,
	"pgtype.Circle":   true,
	"pgtype.Bits":     true,
	"pgtype.TID":      true,
	"pgtype.Uint32":   true,
}

// goTypeNoPointer reports whether a Go type expression should NOT be wrapped in
// a pointer when its column is nullable: slices, maps, json.RawMessage, the
// pgtype.Hstore map type, the pgtype core struct types (Interval/Point/Bits/…),
// and the pgtype.Range / pgtype.Multirange generic types (all of which already
// represent SQL NULL internally — slices/maps via a nil zero value, the pgtype
// struct/range types via their Valid field).
func goTypeNoPointer(goExpr string) bool {
	return strings.HasPrefix(goExpr, "[]") ||
		strings.HasPrefix(goExpr, "map[") ||
		goExpr == "json.RawMessage" ||
		goExpr == "pgtype.Hstore" ||
		pgtypeStructTypes[goExpr] ||
		strings.HasPrefix(goExpr, "pgtype.Range[") ||
		strings.HasPrefix(goExpr, "pgtype.Multirange[")
}

// IsCompositeType reports whether t resolves (through the registry) to a
// composite type. Domains are followed transparently to their base type.
func IsCompositeType(reg *Registry, t *irv1.TypeRef) bool {
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
		return IsCompositeType(reg, entry.d.GetBaseType())
	}
	return false
}

// CompositeDepName returns the bare name of the composite type that t depends
// on, or "" if t does not (transitively) resolve to a composite. It follows
// array element types (a field of `address[]` depends on `address`) and domains
// (transparently, to their base type). The returned name is the registry key
// (bare PostgreSQL type name), suitable for building the composite dependency
// graph used to topologically order RegisterTypes.
func CompositeDepName(reg *Registry, t *irv1.TypeRef) string {
	if reg == nil || t == nil {
		return ""
	}
	if t.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
		return CompositeDepName(reg, t.GetElement())
	}
	pgName := t.GetPgName()
	if _, ok := reg.composites[pgName]; ok {
		return pgName
	}
	if entry, ok := reg.domains[pgName]; ok {
		return CompositeDepName(reg, entry.d.GetBaseType())
	}
	return ""
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
	case "inet", "cidr":
		// pgx ships an InetCodec but no concrete pgtype struct; the textual
		// representation maps cleanly to string (and back via the codec's
		// TextScanner support).
		return "string", nil
	// --- interval ---
	//
	// pgtype.Interval is a struct (Microseconds/Days/Months + Valid); pgx
	// registers a default codec for it, so no RegisterTypes entry is needed.
	case "interval":
		return "pgtype.Interval", []string{"github.com/jackc/pgx/v5/pgtype"}
	// --- geometric types ---
	//
	// All are pgtype structs with a Valid field and a default pgx codec.
	case "point":
		return "pgtype.Point", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "line":
		return "pgtype.Line", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "lseg":
		return "pgtype.Lseg", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "box":
		return "pgtype.Box", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "path":
		return "pgtype.Path", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "polygon":
		return "pgtype.Polygon", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "circle":
		return "pgtype.Circle", []string{"github.com/jackc/pgx/v5/pgtype"}
	// --- bit string types ---
	//
	// pgtype.Bits (Bytes/Len + Valid) represents both bit and varbit.
	case "bit", "varbit":
		return "pgtype.Bits", []string{"github.com/jackc/pgx/v5/pgtype"}
	// --- MAC address types ---
	//
	// pgx's MacaddrCodec scans into / encodes from net.HardwareAddr.
	case "macaddr", "macaddr8":
		return "net.HardwareAddr", []string{"net"}
	// --- system identifier types ---
	//
	// tid → pgtype.TID (BlockNumber/OffsetNumber + Valid).
	// xid/cid use pgx's Uint32Codec, whose Go struct is pgtype.Uint32.
	case "tid":
		return "pgtype.TID", []string{"github.com/jackc/pgx/v5/pgtype"}
	case "xid", "cid":
		return "pgtype.Uint32", []string{"github.com/jackc/pgx/v5/pgtype"}
	// --- extension types: hstore / ltree (registered at runtime by RegisterTypes) ---
	//
	// hstore → pgtype.Hstore (map[string]*string); the nil map is SQL NULL, so a
	// nullable hstore is a value type, never a pointer (see goTypeNoPointer).
	case "hstore":
		return "pgtype.Hstore", []string{"github.com/jackc/pgx/v5/pgtype"}
	// ltree/lquery have only a text-based codec in pgtype, so they map to string.
	case "ltree", "lquery":
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
