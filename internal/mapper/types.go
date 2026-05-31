// Package mapper translates libpg_query AST nodes to sqld IR proto types.
package mapper

import (
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	pg "github.com/pganalyze/pg_query_go/v6"
)

// MapType maps a parsed PostgreSQL TypeName AST node to an IR TypeRef.
//
// AST accessor path:
//   - Names:        tn.GetNames()  → each *pg.Node → .GetString_().GetSval()
//   - Typmods:      tn.GetTypmods() → each *pg.Node → .GetAConst().GetIval().GetIval() (int32)
//   - ArrayBounds:  tn.GetArrayBounds() — non-empty means an array type
//
// The function never returns nil and never panics: unknown shapes produce a
// TypeRef with Kind=SCALAR and PgName set.
func MapType(tn *pg.TypeName) *irv1.TypeRef {
	pgName := extractPgName(tn)
	mod := buildModifier(pgName, tn.GetTypmods())

	scalar := &irv1.TypeRef{
		Kind:     irv1.TypeKind_TYPE_KIND_SCALAR,
		PgName:   pgName,
		Modifier: mod,
	}

	bounds := tn.GetArrayBounds()
	if len(bounds) == 0 {
		return scalar
	}

	// Array type: wrap scalar as the element.
	return &irv1.TypeRef{
		Kind:            irv1.TypeKind_TYPE_KIND_ARRAY,
		PgName:          pgName,
		ArrayDimensions: uint32(len(bounds)),
		Element:         scalar,
	}
}

// extractPgName returns the meaningful name from TypeName.Names.
//
// libpg_query prefixes most built-in types with "pg_catalog"; for those we
// take the last element. For user types without that catalog prefix (e.g.
// "timestamptz" which has only one name node), we take the last element too.
// Either way the last non-"pg_catalog" name is correct.
func extractPgName(tn *pg.TypeName) string {
	names := tn.GetNames()
	if len(names) == 0 {
		return ""
	}
	// Walk backwards; skip the "pg_catalog" schema qualifier.
	for i := len(names) - 1; i >= 0; i-- {
		sval := names[i].GetString_().GetSval()
		if sval != "pg_catalog" {
			return sval
		}
	}
	// Fallback: return last element even if it is "pg_catalog".
	return names[len(names)-1].GetString_().GetSval()
}

// buildModifier constructs a TypeModifier from Typmods nodes when applicable.
// typmodInts reads A_Const integer values (the common case for all modifiers).
func buildModifier(pgName string, typmods []*pg.Node) *irv1.TypeModifier {
	if len(typmods) == 0 {
		// Some tz types carry no typmod but still imply WithTimezone.
		switch pgName {
		case "timestamptz":
			return &irv1.TypeModifier{
				Modifier: &irv1.TypeModifier_DateTime{
					DateTime: &irv1.DateTimeModifier{WithTimezone: true},
				},
			}
		case "timetz":
			return &irv1.TypeModifier{
				Modifier: &irv1.TypeModifier_DateTime{
					DateTime: &irv1.DateTimeModifier{WithTimezone: true},
				},
			}
		}
		return nil
	}

	switch pgName {
	case "numeric", "float4", "float8":
		precision := typmodIval(typmods, 0)
		scale := typmodIval(typmods, 1)
		return &irv1.TypeModifier{
			Modifier: &irv1.TypeModifier_Numeric{
				Numeric: &irv1.NumericModifier{
					Precision: uint32(precision),
					Scale:     uint32(scale),
				},
			},
		}

	case "varchar", "bpchar", "char", "bit", "varbit":
		length := typmodIval(typmods, 0)
		return &irv1.TypeModifier{
			Modifier: &irv1.TypeModifier_Text{
				Text: &irv1.StringModifier{
					Length: uint32(length),
				},
			},
		}

	case "timestamp", "timestamptz":
		precision := typmodIval(typmods, 0)
		tz := pgName == "timestamptz"
		return &irv1.TypeModifier{
			Modifier: &irv1.TypeModifier_DateTime{
				DateTime: &irv1.DateTimeModifier{
					WithTimezone: tz,
					Precision:    uint32(precision),
				},
			},
		}

	case "time", "timetz":
		precision := typmodIval(typmods, 0)
		tz := pgName == "timetz"
		return &irv1.TypeModifier{
			Modifier: &irv1.TypeModifier_DateTime{
				DateTime: &irv1.DateTimeModifier{
					WithTimezone: tz,
					Precision:    uint32(precision),
				},
			},
		}

	case "interval":
		// interval typmods: [0] = fields bitmask (integer), [1] = precision (optional).
		// We don't decode the bitmask to a string here; leave Fields empty for now.
		precision := uint32(0)
		if len(typmods) >= 2 {
			precision = uint32(typmodIval(typmods, 1))
		}
		return &irv1.TypeModifier{
			Modifier: &irv1.TypeModifier_Interval{
				Interval: &irv1.IntervalModifier{
					Precision: precision,
				},
			},
		}
	}

	// Unknown parametrized type — return nil modifier (still non-failing).
	return nil
}

// typmodIval safely reads the integer value of the i-th typmod node.
// Returns 0 when the index is out of range or the node has no integer value.
func typmodIval(typmods []*pg.Node, i int) int32 {
	if i >= len(typmods) {
		return 0
	}
	return typmods[i].GetAConst().GetIval().GetIval()
}
