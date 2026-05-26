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
		{
			name:        "int4range not nullable",
			pgName:      "int4range",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    false,
			wantExpr:    "pgtype.Range[pgtype.Int4]",
			wantImports: []string{"github.com/jackc/pgx/v5/pgtype"},
		},
		{
			name:        "int4range nullable stays value (no pointer)",
			pgName:      "int4range",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    true,
			wantExpr:    "pgtype.Range[pgtype.Int4]",
			wantImports: []string{"github.com/jackc/pgx/v5/pgtype"},
		},
		{
			name:        "tstzrange not nullable",
			pgName:      "tstzrange",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    false,
			wantExpr:    "pgtype.Range[pgtype.Timestamptz]",
			wantImports: []string{"github.com/jackc/pgx/v5/pgtype"},
		},
		{
			name:        "tstzrange nullable stays value (no pointer)",
			pgName:      "tstzrange",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    true,
			wantExpr:    "pgtype.Range[pgtype.Timestamptz]",
			wantImports: []string{"github.com/jackc/pgx/v5/pgtype"},
		},
		{
			name:        "int4multirange not nullable",
			pgName:      "int4multirange",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    false,
			wantExpr:    "pgtype.Multirange[pgtype.Range[pgtype.Int4]]",
			wantImports: []string{"github.com/jackc/pgx/v5/pgtype"},
		},
		{
			name:        "nummultirange nullable stays value (no pointer)",
			pgName:      "nummultirange",
			kind:        irv1.TypeKind_TYPE_KIND_SCALAR,
			nullable:    true,
			wantExpr:    "pgtype.Multirange[pgtype.Range[pgtype.Numeric]]",
			wantImports: []string{"github.com/jackc/pgx/v5/pgtype"},
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

// TestGoTypeJSON verifies json/jsonb map to json.RawMessage (with the
// encoding/json import) and that nullable does NOT wrap it in a pointer
// (json.RawMessage is a []byte; nil means SQL NULL).
func TestGoTypeJSON(t *testing.T) {
	for _, pg := range []string{"json", "jsonb"} {
		for _, nullable := range []bool{false, true} {
			ref := &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: pg}
			expr, imps := goType(nil, ref, nullable)
			if expr != "json.RawMessage" {
				t.Errorf("goType(%q, nullable=%v) = %q; want json.RawMessage", pg, nullable, expr)
			}
			if len(imps) != 1 || imps[0] != "encoding/json" {
				t.Errorf("goType(%q) imports = %v; want [encoding/json]", pg, imps)
			}
		}
	}
}

// TestResolveGoType verifies the override resolver: column-id override,
// type-name override, bare override (no import), import-path override, nullable
// pointer rules, and column-id-beats-type-name precedence.
func TestResolveGoType(t *testing.T) {
	ov := overrides{
		"app.kitchen_sink.c_jsonb": "map[string]any",
		"uuid":                     "github.com/google/uuid.UUID",
		"app.t.c_uuid":             "string", // column-id override beats the uuid type-name override
	}

	cases := []struct {
		name        string
		columnID    string
		pgName      string
		nullable    bool
		wantExpr    string
		wantImports []string
	}{
		{
			name:        "column-id bare map override, no import",
			columnID:    "app.kitchen_sink.c_jsonb",
			pgName:      "jsonb",
			nullable:    false,
			wantExpr:    "map[string]any",
			wantImports: nil,
		},
		{
			name:        "column-id map override nullable stays map (no pointer)",
			columnID:    "app.kitchen_sink.c_jsonb",
			pgName:      "jsonb",
			nullable:    true,
			wantExpr:    "map[string]any",
			wantImports: nil,
		},
		{
			name:        "type-name override with import path",
			columnID:    "",
			pgName:      "uuid",
			nullable:    false,
			wantExpr:    "uuid.UUID",
			wantImports: []string{"github.com/google/uuid"},
		},
		{
			name:        "type-name override nullable -> pointer",
			columnID:    "app.kitchen_sink.c_uuid",
			pgName:      "uuid",
			nullable:    true,
			wantExpr:    "*uuid.UUID",
			wantImports: []string{"github.com/google/uuid"},
		},
		{
			name:        "column-id beats type-name override",
			columnID:    "app.t.c_uuid",
			pgName:      "uuid",
			nullable:    false,
			wantExpr:    "string",
			wantImports: nil,
		},
		{
			name:        "no override falls back to default mapping",
			columnID:    "app.other.c_text",
			pgName:      "text",
			nullable:    false,
			wantExpr:    "string",
			wantImports: nil,
		},
		{
			name:        "no override jsonb default -> json.RawMessage",
			columnID:    "app.other.c_jsonb",
			pgName:      "jsonb",
			nullable:    false,
			wantExpr:    "json.RawMessage",
			wantImports: []string{"encoding/json"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref := &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: tc.pgName}
			gotExpr, gotImports := resolveGoType(nil, ov, tc.columnID, ref, tc.nullable)
			if gotExpr != tc.wantExpr {
				t.Errorf("resolveGoType expr = %q; want %q", gotExpr, tc.wantExpr)
			}
			if len(gotImports) != len(tc.wantImports) {
				t.Fatalf("resolveGoType imports = %v; want %v", gotImports, tc.wantImports)
			}
			for i := range tc.wantImports {
				if gotImports[i] != tc.wantImports[i] {
					t.Errorf("resolveGoType import[%d] = %q; want %q", i, gotImports[i], tc.wantImports[i])
				}
			}
		})
	}
}

// TestParseOverrideValue verifies the value→(type,import) parsing rules.
func TestParseOverrideValue(t *testing.T) {
	cases := []struct {
		in          string
		wantExpr    string
		wantImports []string
	}{
		{"map[string]any", "map[string]any", nil},
		{"string", "string", nil},
		{"json.RawMessage", "json.RawMessage", []string{"encoding/json"}},
		{"github.com/google/uuid.UUID", "uuid.UUID", []string{"github.com/google/uuid"}},
		{"github.com/shopspring/decimal.Decimal", "decimal.Decimal", []string{"github.com/shopspring/decimal"}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			expr, imps := parseOverrideValue(tc.in)
			if expr != tc.wantExpr {
				t.Errorf("parseOverrideValue(%q) expr = %q; want %q", tc.in, expr, tc.wantExpr)
			}
			if len(imps) != len(tc.wantImports) {
				t.Fatalf("parseOverrideValue(%q) imports = %v; want %v", tc.in, imps, tc.wantImports)
			}
			for i := range tc.wantImports {
				if imps[i] != tc.wantImports[i] {
					t.Errorf("parseOverrideValue(%q) import[%d] = %q; want %q", tc.in, i, imps[i], tc.wantImports[i])
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

// TestCustomRangeType verifies that a custom CREATE TYPE ... AS RANGE type maps
// a column to pgtype.Range[<subtypeElem>] and that RegisterTypes registers the
// range and its array type after the enum/composite types.
func TestCustomRangeType(t *testing.T) {
	// Catalog: app.timerange AS RANGE (subtype = timestamptz), plus an enum and
	// a composite so we can assert ordering (enums/composites then ranges).
	req := &pluginv1.GenerateRequest{
		OutDir: "gen/db",
		Catalog: &irv1.Catalog{
			Schemas: []*irv1.Schema{{
				Name: "app",
				Enums: []*irv1.EnumType{{
					Name:   &irv1.QualifiedName{Schema: "app", Name: "user_status"},
					Labels: []string{"active", "inactive"},
				}},
				Composites: []*irv1.CompositeType{{
					Name: &irv1.QualifiedName{Schema: "app", Name: "address"},
					Fields: []*irv1.CompositeField{
						{Name: "street", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "text"}},
					},
				}},
				Ranges: []*irv1.RangeType{{
					Name:    &irv1.QualifiedName{Schema: "app", Name: "timerange"},
					Subtype: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "timestamptz"},
				}},
				Tables: []*irv1.Table{{
					Name: &irv1.QualifiedName{Schema: "app", Name: "profiles"},
					Columns: []*irv1.Column{
						{Name: "user_id", Type: &irv1.TypeRef{PgName: "int8"}, Nullable: false},
						// nullable custom range → value type (Range carries Valid).
						{Name: "valid_window", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_RANGE, PgName: "timerange"}, Nullable: true},
					},
				}},
			}},
		},
	}

	// 1. Direct goType resolution: custom range → pgtype.Range[pgtype.Timestamptz],
	//    value type even when nullable.
	reg := buildUDTRegistry(req.GetCatalog())
	ref := &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_RANGE, PgName: "timerange"}
	got, imps := goType(reg, ref, true)
	if got != "pgtype.Range[pgtype.Timestamptz]" {
		t.Errorf("custom range goType = %q; want pgtype.Range[pgtype.Timestamptz]", got)
	}
	wantImp := "github.com/jackc/pgx/v5/pgtype"
	if len(imps) != 1 || imps[0] != wantImp {
		t.Errorf("custom range imports = %v; want [%s]", imps, wantImp)
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

	// 2. The table field uses the resolved Go type.
	if !strings.Contains(normalizeSpaces(models), normalizeSpaces("ValidWindow pgtype.Range[pgtype.Timestamptz]")) {
		t.Errorf("missing ValidWindow field in models.go:\n%s", models)
	}

	// 3. RegisterTypes lists the range and its array type, after enums/composites.
	for _, name := range []string{`"app.timerange"`, `"app._timerange"`} {
		if !strings.Contains(models, name) {
			t.Errorf("RegisterTypes missing %s in models.go:\n%s", name, models)
		}
	}
	// Ordering: enum (app.user_status) and composite (app.address) come before
	// the range (app.timerange).
	idxEnum := strings.Index(models, `"app.user_status"`)
	idxComposite := strings.Index(models, `"app.address"`)
	idxRange := strings.Index(models, `"app.timerange"`)
	idxRangeArr := strings.Index(models, `"app._timerange"`)
	if idxEnum < 0 || idxComposite < 0 || idxRange < 0 || idxRangeArr < 0 {
		t.Fatalf("expected enum, composite, range, and range-array entries in RegisterTypes:\n%s", models)
	}
	if !(idxEnum < idxRange && idxComposite < idxRange) {
		t.Errorf("range %d must come after enum %d and composite %d in RegisterTypes", idxRange, idxEnum, idxComposite)
	}
	if !(idxRange < idxRangeArr) {
		t.Errorf("range element %d must come before its array %d in RegisterTypes", idxRange, idxRangeArr)
	}
}

// TestCustomMultirangeType verifies that the MULTIRANGE auto-created for a
// custom CREATE TYPE ... AS RANGE maps a column to
// pgtype.Multirange[pgtype.Range[<subtypeElem>]], and that RegisterTypes lists
// the range, its array, the multirange, and the multirange array in that order
// (element RANGE before its MULTIRANGE; each element before its array).
func TestCustomMultirangeType(t *testing.T) {
	req := &pluginv1.GenerateRequest{
		OutDir: "gen/db",
		Catalog: &irv1.Catalog{
			Schemas: []*irv1.Schema{{
				Name: "app",
				Ranges: []*irv1.RangeType{{
					Name:       &irv1.QualifiedName{Schema: "app", Name: "timerange"},
					Subtype:    &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: "timestamptz"},
					Multirange: "timemultirange",
				}},
				Tables: []*irv1.Table{{
					Name: &irv1.QualifiedName{Schema: "app", Name: "profiles"},
					Columns: []*irv1.Column{
						{Name: "user_id", Type: &irv1.TypeRef{PgName: "int8"}, Nullable: false},
						// nullable custom multirange → value type (Multirange carries Valid).
						{Name: "windows", Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_RANGE, PgName: "timemultirange"}, Nullable: true},
					},
				}},
			}},
		},
	}

	// 1. Direct goType resolution: custom multirange →
	//    pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]], value type even when nullable.
	reg := buildUDTRegistry(req.GetCatalog())
	ref := &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_RANGE, PgName: "timemultirange"}
	got, imps := goType(reg, ref, true)
	want := "pgtype.Multirange[pgtype.Range[pgtype.Timestamptz]]"
	if got != want {
		t.Errorf("custom multirange goType = %q; want %q", got, want)
	}
	wantImp := "github.com/jackc/pgx/v5/pgtype"
	if len(imps) != 1 || imps[0] != wantImp {
		t.Errorf("custom multirange imports = %v; want [%s]", imps, wantImp)
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

	// 2. The table field uses the resolved Go type.
	if !strings.Contains(normalizeSpaces(models), normalizeSpaces("Windows "+want)) {
		t.Errorf("missing Windows field in models.go:\n%s", models)
	}

	// 3. RegisterTypes lists range, _range, multirange, _multirange in that order.
	idxRange := strings.Index(models, `"app.timerange"`)
	idxRangeArr := strings.Index(models, `"app._timerange"`)
	idxMR := strings.Index(models, `"app.timemultirange"`)
	idxMRArr := strings.Index(models, `"app._timemultirange"`)
	if idxRange < 0 || idxRangeArr < 0 || idxMR < 0 || idxMRArr < 0 {
		t.Fatalf("expected range, _range, multirange, _multirange entries in RegisterTypes:\n%s", models)
	}
	if !(idxRange < idxRangeArr && idxRangeArr < idxMR && idxMR < idxMRArr) {
		t.Errorf("RegisterTypes order wrong: range=%d _range=%d multirange=%d _multirange=%d (want strictly increasing)",
			idxRange, idxRangeArr, idxMR, idxMRArr)
	}
}
