package main

import (
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// goType maps a TypeRef to a Go type expression and the imports it needs.
// Nullable scalars are wrapped in a pointer, except bytea ([]byte) and arrays
// which always remain as-is.
func goType(t *irv1.TypeRef, nullable bool) (goExpr string, imports []string) {
	if t == nil {
		return "any", nil
	}

	if t.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
		elem := t.GetElement()
		elemExpr, elemImports := goType(elem, false)
		return "[]" + elemExpr, elemImports
	}

	expr, imps := scalarGoType(t.GetPgName())

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
