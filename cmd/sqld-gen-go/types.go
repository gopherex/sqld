package main

import (
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

	if nullable && expr != "[]byte" {
		expr = "*" + expr
	}
	return expr, imps
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
		return "[]byte", nil
	case "inet", "cidr", "macaddr":
		return "string", nil
	default:
		return "any", nil
	}
}
