package main

import (
	"testing"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

func TestGoType(t *testing.T) {
	cases := []struct {
		name        string
		pgName      string
		kind        irv1.TypeKind
		element     *irv1.TypeRef
		nullable    bool
		wantExpr    string
		wantImports []string
	}{
		{
			name:        "int8 not nullable",
			pgName:      "int8",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    false,
			wantExpr:    "int64",
			wantImports: nil,
		},
		{
			name:        "int8 nullable",
			pgName:      "int8",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    true,
			wantExpr:    "*int64",
			wantImports: nil,
		},
		{
			name:        "text not nullable",
			pgName:      "text",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    false,
			wantExpr:    "string",
			wantImports: nil,
		},
		{
			name:        "text nullable",
			pgName:      "text",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    true,
			wantExpr:    "*string",
			wantImports: nil,
		},
		{
			name:        "timestamptz not nullable",
			pgName:      "timestamptz",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    false,
			wantExpr:    "time.Time",
			wantImports: []string{"time"},
		},
		{
			name:        "timestamptz nullable",
			pgName:      "timestamptz",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    true,
			wantExpr:    "*time.Time",
			wantImports: []string{"time"},
		},
		{
			name:        "bytea not nullable",
			pgName:      "bytea",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    false,
			wantExpr:    "[]byte",
			wantImports: nil,
		},
		{
			name:        "bytea nullable stays []byte",
			pgName:      "bytea",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    true,
			wantExpr:    "[]byte",
			wantImports: nil,
		},
		{
			name:   "int4 array",
			pgName: "_int4",
			kind:   irv1.TypeKind_TYPE_KIND_ARRAY,
			element: &irv1.TypeRef{
				Kind:   irv1.TypeKind_TYPE_KIND_SCALAR,
				PgName: "int4",
			},
			nullable:    false,
			wantExpr:    "[]int32",
			wantImports: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref := &irv1.TypeRef{
				Kind:    tc.kind,
				PgName:  tc.pgName,
				Element: tc.element,
			}
			gotExpr, gotImports := goType(ref, tc.nullable)
			if gotExpr != tc.wantExpr {
				t.Errorf("goType(%q, %v) expr = %q; want %q", tc.pgName, tc.nullable, gotExpr, tc.wantExpr)
			}
			if len(gotImports) != len(tc.wantImports) {
				t.Errorf("goType(%q, %v) imports = %v; want %v", tc.pgName, tc.nullable, gotImports, tc.wantImports)
				return
			}
			for i, imp := range tc.wantImports {
				if gotImports[i] != imp {
					t.Errorf("goType(%q, %v) import[%d] = %q; want %q", tc.pgName, tc.nullable, i, gotImports[i], imp)
				}
			}
		})
	}
}
