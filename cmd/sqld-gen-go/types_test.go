package main

import (
	"strings"
	"testing"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
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
			gotExpr, gotImports := goType(nil, ref, tc.nullable)
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

// TestGoTypeUDT verifies enum, domain, and composite resolution through
// a small in-memory catalog.
func TestGoTypeUDT(t *testing.T) {
	// Build a catalog with:
	//   public.status   (enum: active, inactive)
	//   public.email    (domain over text)
	//   public.addr     (composite: a text, b int4)
	catalog := &irv1.Catalog{
		Schemas: []*irv1.Schema{{
			Name: "public",
			Enums: []*irv1.EnumType{{
				Name:   &irv1.QualifiedName{Schema: "public", Name: "status"},
				Labels: []string{"active", "inactive"},
			}},
			Domains: []*irv1.DomainType{{
				Name:     &irv1.QualifiedName{Schema: "public", Name: "email"},
				BaseType: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "text"},
			}},
			Composites: []*irv1.CompositeType{{
				Name: &irv1.QualifiedName{Schema: "public", Name: "addr"},
				Fields: []*irv1.CompositeField{
					{Name: "a", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "text"}},
					{Name: "b", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "int4"}},
				},
			}},
		}},
	}
	reg := buildUDTRegistry(catalog)

	cases := []struct {
		name     string
		pgName   string
		kind     irv1.TypeKind
		nullable bool
		want     string
	}{
		// enum: public schema → no prefix
		{"enum not nullable", "status", irv1.TypeKind_TYPE_KIND_ENUM, false, "Status"},
		{"enum nullable", "status", irv1.TypeKind_TYPE_KIND_ENUM, true, "*Status"},
		// domain: transparent, resolves to base type
		{"domain not nullable", "email", irv1.TypeKind_TYPE_KIND_DOMAIN, false, "string"},
		{"domain nullable", "email", irv1.TypeKind_TYPE_KIND_DOMAIN, true, "*string"},
		// composite: public schema → no prefix
		{"composite not nullable", "addr", irv1.TypeKind_TYPE_KIND_COMPOSITE, false, "Addr"},
		{"composite nullable", "addr", irv1.TypeKind_TYPE_KIND_COMPOSITE, true, "*Addr"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref := &irv1.TypeRef{Kind: tc.kind, PgName: tc.pgName}
			got, _ := goType(reg, ref, tc.nullable)
			if got != tc.want {
				t.Errorf("goType(%q, nullable=%v) = %q; want %q", tc.pgName, tc.nullable, got, tc.want)
			}
		})
	}
}

// TestGenerateUDTModels verifies that enum + composite type definitions are
// emitted in models.go and that table fields use the resolved Go types.
func TestGenerateUDTModels(t *testing.T) {
	req := &pluginv1.GenerateRequest{
		OutDir: "gen/db",
		Catalog: &irv1.Catalog{
			Schemas: []*irv1.Schema{{
				Name: "public",
				Enums: []*irv1.EnumType{{
					Name:   &irv1.QualifiedName{Schema: "public", Name: "status"},
					Labels: []string{"active", "inactive"},
				}},
				Domains: []*irv1.DomainType{{
					Name:     &irv1.QualifiedName{Schema: "public", Name: "email"},
					BaseType: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "text"},
				}},
				Composites: []*irv1.CompositeType{{
					Name: &irv1.QualifiedName{Schema: "public", Name: "addr"},
					Fields: []*irv1.CompositeField{
						{Name: "a", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "text"}},
						{Name: "b", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "int4"}},
					},
				}},
				Tables: []*irv1.Table{{
					Name: &irv1.QualifiedName{Name: "users"},
					Columns: []*irv1.Column{
						{Name: "id", Type: &irv1.TypeRef{PgName: "int8"}, Nullable: false},
						// domain col: NOT NULL → string (not pointer)
						{Name: "email_col", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_DOMAIN, PgName: "email"}, Nullable: false},
						// enum col: NOT NULL → Status
						{Name: "status_col", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_ENUM, PgName: "status"}, Nullable: false},
						// composite col: nullable → *Addr
						{Name: "addr_col", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_COMPOSITE, PgName: "addr"}, Nullable: true},
					},
				}},
			}},
		},
	}

	resp, err := Generate(req)
	if err != nil {
		t.Fatal(err)
	}

	var models string
	for _, f := range resp.GetFiles() {
		if f.GetPath() == "models.go" {
			models = string(f.GetContents())
		}
	}
	if models == "" {
		t.Fatal("models.go not found in response")
	}

	checks := []string{
		// enum type declaration
		"type Status string",
		// enum const block
		"StatusActive",
		`StatusActive   Status = "active"`,
		"StatusInactive",
		// composite struct
		"type Addr struct",
		// table fields resolved correctly
		"EmailCol  string", // domain → base type string, NOT NULL
		"StatusCol Status", // enum → Status, NOT NULL
		"AddrCol   *Addr",  // composite, nullable → pointer
	}

	for _, want := range checks {
		// normalise whitespace for struct field checks
		if !strings.Contains(models, want) && !strings.Contains(normalizeSpaces(models), normalizeSpaces(want)) {
			t.Errorf("missing %q in models.go:\n%s", want, models)
		}
	}

	// Domains emit NO new type.
	if strings.Contains(models, "type Email") {
		t.Errorf("domain should not produce a Go type, but found 'type Email' in:\n%s", models)
	}
}
