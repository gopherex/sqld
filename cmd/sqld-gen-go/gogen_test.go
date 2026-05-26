package main

import (
	"fmt"
	"go/format"
	"strings"
	"testing"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// TestGenerateDynamicQuery tests the WHERE-aware dynamic builder with named
// params whose optionality comes from QueryParameter.Optional (the `@name?`
// suffix), an ANY($N) slice condition, and an @orderby LIST annotation.
func TestGenerateDynamicQuery(t *testing.T) {
	// Query.Sql as produced by the host: $N placeholders (the `?` suffix is
	// already stripped), and a standalone "-- @orderby ..." comment line. The
	// optional condition carries NO trailing annotation — its optionality is
	// conveyed via Parameters[0].Optional.
	sql := "SELECT id, email, status FROM app.users\n" +
		"WHERE\n" +
		"      email = $1\n" +
		"  AND id = ANY($2)\n" +
		"-- @orderby created_at, email\n" +
		";"

	// off finds the start/end byte offsets of the first occurrence of sub.
	off := func(sub string) (uint64, uint64) {
		j := strings.Index(sql, sub)
		if j < 0 {
			t.Fatalf("substring %q not found", sub)
		}
		return uint64(j), uint64(j + len(sub))
	}

	// mkList creates a LIST annotation with multiple string-value args (one per column).
	mkList := func(name string, s, e uint64, cols ...string) *irv1.AnnotationValue {
		av := &irv1.AnnotationValue{
			Name: name,
			Target: &irv1.AnnotationTargetRef{
				Kind:      irv1.AnnotationTargetKind_ANNOTATION_TARGET_KIND_QUERY,
				QueryName: "SearchUsers",
				Source:    &irv1.SourceSpan{StartOffset: s, EndOffset: e},
			},
		}
		for i, col := range cols {
			av.Args = append(av.Args, &irv1.AnnotationArg{
				Index: uint32(i),
				Value: &irv1.AnnotationArg_StringValue{StringValue: col},
				Raw:   col,
			})
		}
		return av
	}

	orderbyS, orderbyE := off("-- @orderby created_at, email")

	req := &pluginv1.GenerateRequest{
		Queries: []*pluginv1.Query{{
			Name:    "SearchUsers",
			Sql:     sql,
			Command: pluginv1.QueryCommand_QUERY_COMMAND_MANY,
			Parameters: []*pluginv1.QueryParameter{
				// email = $1 → optional (`@email?`) → pointer field.
				{Number: 1, Name: "email", Optional: true, Type: &irv1.TypeRef{PgName: "text"}},
				// id = ANY($2) → slice (inherently conditional, no `?`).
				{Number: 2, Name: "ids", Type: &irv1.TypeRef{PgName: "int8"}},
			},
			Columns: []*pluginv1.QueryColumn{
				{Name: "id", Type: &irv1.TypeRef{PgName: "int8"}},
				{Name: "email", Type: &irv1.TypeRef{PgName: "text"}},
				{Name: "status", Type: &irv1.TypeRef{PgName: "text"}},
			},
		}},
		Annotations: []*irv1.AnnotationValue{
			mkList("orderby", orderbyS, orderbyE, "created_at", "email"),
		},
	}

	resp, err := Generate(req)
	if err != nil {
		t.Fatal(err)
	}

	var q string
	for _, f := range resp.GetFiles() {
		if f.GetPath() == "queries.go" {
			q = string(f.GetContents())
		}
	}
	if q == "" {
		t.Fatal("queries.go not found in response")
	}

	// ---- Positive assertions ----
	// Note: gofmt aligns struct fields so we check field names and types
	// separately rather than requiring exact spacing.
	for _, want := range []string{
		// Typed OrderBy enum for this query.
		"type SearchUsersOrderBy string",
		"SearchUsersOrderByCreatedAt",
		"SearchUsersOrderByEmail",
		// Shared direction type.
		"type OrderDir string",
		"OrderAsc",
		"OrderDesc",
		// Params struct fields (names).
		"type SearchUsersParams struct",
		"Email",   // optional (@email?) → pointer field
		"*string", // ... of element type *string
		"Ids",     // ANY($2) slice
		"[]int64", // ... of element type []int64
		"OrderBy", // @orderby enum field
		"SearchUsersOrderBy",
		"OrderDir", // shared direction field
		// WHERE-aware builder.
		"var conds []string",
		`strings.Join(conds, " AND ")`,
		`b.WriteString(" WHERE " + `,
		// ORDER BY block.
		`ORDER BY %s %s`,
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in:\n%s", want, q)
		}
	}

	// ---- Negative assertions ----
	for _, bad := range []string{
		"WHERE true",
	} {
		if strings.Contains(q, bad) {
			t.Fatalf("found disallowed %q in:\n%s", bad, q)
		}
	}

	// ---- Valid Go ----
	if _, err := format.Source([]byte(q)); err != nil {
		t.Fatalf("invalid Go: %v\n%s", err, q)
	}
}

func keys(m map[string]string) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestGenerateModelsAndQueries(t *testing.T) {
	req := &pluginv1.GenerateRequest{
		OutDir: "gen/db",
		Catalog: &irv1.Catalog{Schemas: []*irv1.Schema{{Name: "public", Tables: []*irv1.Table{{
			Name: &irv1.QualifiedName{Name: "users"},
			Columns: []*irv1.Column{
				{Name: "id", Type: &irv1.TypeRef{PgName: "int8"}, Nullable: false},
				{Name: "email", Type: &irv1.TypeRef{PgName: "text"}, Nullable: false},
			},
		}}}}},
		Queries: []*pluginv1.Query{{
			Name: "GetUser", Sql: "SELECT id, email FROM users WHERE id = $1",
			Command:    pluginv1.QueryCommand_QUERY_COMMAND_ONE,
			Parameters: []*pluginv1.QueryParameter{{Number: 1, Type: &irv1.TypeRef{PgName: "int8"}}},
			Columns: []*pluginv1.QueryColumn{
				{Name: "id", Type: &irv1.TypeRef{PgName: "int8"}},
				{Name: "email", Type: &irv1.TypeRef{PgName: "text"}},
			},
		}},
	}
	resp, err := Generate(req)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range resp.GetFiles() {
		files[f.GetPath()] = string(f.GetContents())
	}
	models, ok := files["models.go"]
	if !ok {
		t.Fatalf("no models.go; got %v", keys(files))
	}
	if !strings.Contains(models, "type Users struct") {
		t.Fatalf("models:\n%s", models)
	}
	q, ok := files["queries.go"]
	if !ok {
		t.Fatal("no queries.go")
	}
	if !strings.Contains(q, "func (q *Queries) GetUser(ctx context.Context, id int64) (GetUserRow, error)") {
		t.Fatalf("queries:\n%s", q)
	}
	// generated code must be valid Go (parseable)
	if _, err := format.Source(resp.GetFiles()[0].GetContents()); err != nil {
		t.Fatalf("models.go not valid Go: %v", err)
	}
}

func TestInfoResponse(t *testing.T) {
	info := Info()
	if info.GetName() != "go" {
		t.Errorf("Info().Name = %q; want %q", info.GetName(), "go")
	}
	if info.GetVersion() != "0.1.0" {
		t.Errorf("Info().Version = %q; want %q", info.GetVersion(), "0.1.0")
	}
	if len(info.GetSupportedEngines()) == 0 {
		t.Error("Info().SupportedEngines is empty")
	}
	// Verify annotation schema: only @orderby (LIST) remains. Optionality is no
	// longer an annotation — it comes from QueryParameter.Optional (@name?).
	schema := info.GetAnnotationSchema()
	if schema == nil {
		t.Fatal("AnnotationSchema is nil")
	}
	annsByName := map[string]*pluginv1.AnnotationDef{}
	for _, a := range schema.GetAnnotations() {
		annsByName[a.GetName()] = a
	}
	obAnn, ok := annsByName["orderby"]
	if !ok {
		t.Fatal("missing @orderby annotation definition")
	}
	if obAnn.GetValue().GetForm() != pluginv1.AnnotationForm_ANNOTATION_FORM_LIST {
		t.Errorf("@orderby form = %v; want LIST", obAnn.GetValue().GetForm())
	}
	if obAnn.GetValue().GetElement() == nil {
		t.Error("@orderby element spec is nil")
	} else if obAnn.GetValue().GetElement().GetType() != irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_IDENT {
		t.Errorf("@orderby element type = %v; want IDENT", obAnn.GetValue().GetElement().GetType())
	}
	// Removed annotations (if, endif, slice) must NOT be present.
	for _, badName := range []string{"if", "endif", "slice"} {
		if _, found := annsByName[badName]; found {
			t.Errorf("removed annotation @%s should not be present in new schema", badName)
		}
	}
}

func TestGenerateNoQueries(t *testing.T) {
	req := &pluginv1.GenerateRequest{
		OutDir: "gen/db",
		Catalog: &irv1.Catalog{Schemas: []*irv1.Schema{{Name: "public", Tables: []*irv1.Table{{
			Name:    &irv1.QualifiedName{Name: "products"},
			Columns: []*irv1.Column{{Name: "id", Type: &irv1.TypeRef{PgName: "int4"}, Nullable: false}},
		}}}}},
	}
	resp, err := Generate(req)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range resp.GetFiles() {
		files[f.GetPath()] = string(f.GetContents())
	}
	if _, ok := files["models.go"]; !ok {
		t.Fatal("expected models.go even with no queries")
	}
	if _, ok := files["queries.go"]; ok {
		t.Fatal("should not emit queries.go when there are no queries")
	}
}

func TestPackageFromOptions(t *testing.T) {
	req := &pluginv1.GenerateRequest{
		OutDir:  "gen/db",
		Options: []byte(`{"package":"mypkg"}`),
		Catalog: &irv1.Catalog{},
	}
	resp, err := Generate(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range resp.GetFiles() {
		if !strings.Contains(string(f.GetContents()), "package mypkg") {
			t.Errorf("expected package mypkg in %s:\n%s", f.GetPath(), f.GetContents())
		}
	}
}

func TestPackageFromOutDir(t *testing.T) {
	req := &pluginv1.GenerateRequest{
		OutDir:  "gen/mydb",
		Catalog: &irv1.Catalog{},
	}
	resp, err := Generate(req)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range resp.GetFiles() {
		if !strings.Contains(string(f.GetContents()), "package mydb") {
			t.Errorf("expected package mydb in %s:\n%s", f.GetPath(), f.GetContents())
		}
	}
}

// strPtr is a helper to make a *string from a literal.
func strPtr(s string) *string { return &s }

// normalizeSpaces collapses runs of whitespace to single spaces and trims ends.
func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// Ensure fmt is used (suppress import-not-used if a test is removed).
var _ = fmt.Sprintf
