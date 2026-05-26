package gotypes

import (
	"testing"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

func scalarRef(pg string) *irv1.TypeRef { return &irv1.TypeRef{PgName: pg} }

func importsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestGoTypePointerMode(t *testing.T) {
	m := NewMapper2(nil, nil, Pointer)
	cases := []struct {
		name        string
		pg          string
		nullable    bool
		wantExpr    string
		wantImports []string
	}{
		{"int8 non-null", "int8", false, "int64", nil},
		{"int8 nullable", "int8", true, "*int64", nil},
		{"text nullable", "text", true, "*string", nil},
		{"jsonb nullable unwrapped", "jsonb", true, "json.RawMessage", []string{"encoding/json"}},
		{"bytea nullable unwrapped", "bytea", true, "[]byte", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expr, imps := m.GoType("", scalarRef(tc.pg), tc.nullable)
			if expr != tc.wantExpr {
				t.Errorf("GoType(%q, nullable=%v) expr = %q; want %q", tc.pg, tc.nullable, expr, tc.wantExpr)
			}
			if !importsEqual(imps, tc.wantImports) {
				t.Errorf("GoType(%q, nullable=%v) imports = %v; want %v", tc.pg, tc.nullable, imps, tc.wantImports)
			}
		})
	}
}

func TestGoTypeOptMode(t *testing.T) {
	m := NewMapper2(nil, nil, Opt)
	cases := []struct {
		name        string
		pg          string
		nullable    bool
		wantExpr    string
		wantImports []string
	}{
		{"text nullable", "text", true, "null.Val[string]", []string{"github.com/aarondl/opt/null"}},
		{"jsonb nullable stays unwrapped", "jsonb", true, "json.RawMessage", []string{"encoding/json"}},
		{"int8 non-null", "int8", false, "int64", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expr, imps := m.GoType("", scalarRef(tc.pg), tc.nullable)
			if expr != tc.wantExpr {
				t.Errorf("GoType(%q, nullable=%v) expr = %q; want %q", tc.pg, tc.nullable, expr, tc.wantExpr)
			}
			if !importsEqual(imps, tc.wantImports) {
				t.Errorf("GoType(%q, nullable=%v) imports = %v; want %v", tc.pg, tc.nullable, imps, tc.wantImports)
			}
		})
	}
}
