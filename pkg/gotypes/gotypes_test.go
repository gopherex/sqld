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

func TestGoTypeUDTPackage(t *testing.T) {
	// Catalog with an enum app.user_status; gen-go names it "AppUserStatus".
	cat := &irv1.Catalog{
		Schemas: []*irv1.Schema{{
			Name: "app",
			Enums: []*irv1.EnumType{{
				Name:   &irv1.QualifiedName{Schema: "app", Name: "user_status"},
				Labels: []string{"active", "closed"},
			}},
		}},
	}
	m := NewMapper(cat, nil, Pointer).SetUDTPackage("db", "example.com/app/db")

	enumRef := &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "user_status"}
	expr, imps := m.GoType("", enumRef, false)
	if expr != "db.AppUserStatus" {
		t.Errorf("enum expr = %q; want db.AppUserStatus", expr)
	}
	if !importsEqual(imps, []string{"example.com/app/db"}) {
		t.Errorf("enum imports = %v; want the db import", imps)
	}

	// Array of enum qualifies the element through recursion.
	arrRef := &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_ARRAY, Element: enumRef}
	expr, imps = m.GoType("", arrRef, false)
	if expr != "[]db.AppUserStatus" {
		t.Errorf("array-of-enum expr = %q; want []db.AppUserStatus", expr)
	}
	if !importsEqual(imps, []string{"example.com/app/db"}) {
		t.Errorf("array-of-enum imports = %v", imps)
	}

	// Without SetUDTPackage, names are unqualified (gen-go behaviour).
	plain := NewMapper(cat, nil, Pointer)
	if expr, _ := plain.GoType("", enumRef, false); expr != "AppUserStatus" {
		t.Errorf("unqualified enum expr = %q; want AppUserStatus", expr)
	}
}

func TestIsNullWrapped(t *testing.T) {
	m := NewMapper2(nil, nil, Pointer)
	cases := []struct {
		name     string
		pg       string
		nullable bool
		want     bool
	}{
		{"int8 nullable is wrapped", "int8", true, true},
		{"text nullable is wrapped", "text", true, true},
		{"jsonb nullable is nil-capable (unwrapped)", "jsonb", true, false},
		{"bytea nullable is nil-capable (unwrapped)", "bytea", true, false},
		{"interval nullable is nil-capable (unwrapped)", "interval", true, false},
		{"int8 non-null is unwrapped", "int8", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := m.IsNullWrapped("", scalarRef(tc.pg), tc.nullable); got != tc.want {
				t.Errorf("IsNullWrapped(%q, nullable=%v) = %v; want %v", tc.pg, tc.nullable, got, tc.want)
			}
		})
	}

	// An override to a nil-capable Go type is reported unwrapped.
	mo := NewMapper2(nil, Overrides{"jsonb": "map[string]any"}, Pointer)
	if mo.IsNullWrapped("", scalarRef("jsonb"), true) {
		t.Errorf("override map[string]any should be reported unwrapped")
	}
	// An override to a value Go type is reported wrapped.
	mu := NewMapper2(nil, Overrides{"uuid": "github.com/google/uuid.UUID"}, Pointer)
	if !mu.IsNullWrapped("", scalarRef("uuid"), true) {
		t.Errorf("override uuid.UUID should be reported wrapped")
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
