package main

import (
	"fmt"
	"go/format"
	"strings"
	"testing"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

func TestGenerateDynamicQuery(t *testing.T) {
	sql := "SELECT id, email, status FROM app.users WHERE true\n" +
		"/*@if name*/ AND email = $1 /*@endif*/\n" +
		"/*@slice ids*/ AND id = ANY($2) /*@endif*/\n" +
		"/*@orderby allow=created_at,email*/"

	// offNth finds the start/end of the n-th occurrence (0-based) of sub in sql.
	offNth := func(sub string, n int) (uint32, uint32) {
		idx := 0
		for i := 0; i <= n; i++ {
			j := strings.Index(sql[idx:], sub)
			if j < 0 {
				t.Fatalf("substring %q not found (occurrence %d)", sub, n)
			}
			if i == n {
				start := idx + j
				return uint32(start), uint32(start + len(sub))
			}
			idx += j + len(sub)
		}
		panic("unreachable")
	}
	off := func(sub string) (uint32, uint32) { return offNth(sub, 0) }

	mk := func(name, argName, argVal string, s, e uint32) *irv1.AnnotationValue {
		av := &irv1.AnnotationValue{
			Name: name,
			Target: &irv1.AnnotationTargetRef{
				Kind:      irv1.AnnotationTargetKind_ANNOTATION_TARGET_KIND_QUERY,
				QueryName: "SearchUsers",
				Source:    &irv1.SourceSpan{StartOffset: uint64(s), EndOffset: uint64(e)},
			},
		}
		if argName != "" {
			av.Args = []*irv1.AnnotationArg{{
				Name:  argName,
				Value: &irv1.AnnotationArg_StringValue{StringValue: argVal},
			}}
		}
		return av
	}

	ifS, ifE := off("/*@if name*/")
	endif1S, endif1E := offNth("/*@endif*/", 0)
	sliceS, sliceE := off("/*@slice ids*/")
	endif2S, endif2E := offNth("/*@endif*/", 1)
	orderbyS, orderbyE := off("/*@orderby allow=created_at,email*/")

	req := &pluginv1.GenerateRequest{
		Queries: []*pluginv1.Query{{
			Name:    "SearchUsers",
			Sql:     sql,
			Command: pluginv1.QueryCommand_QUERY_COMMAND_MANY,
			Parameters: []*pluginv1.QueryParameter{
				{Number: 1, Type: &irv1.TypeRef{PgName: "text"}},
				{Number: 2, Type: &irv1.TypeRef{PgName: "int8"}},
			},
			Columns: []*pluginv1.QueryColumn{
				{Name: "id", Type: &irv1.TypeRef{PgName: "int8"}},
				{Name: "email", Type: &irv1.TypeRef{PgName: "text"}},
				{Name: "status", Type: &irv1.TypeRef{PgName: "text"}},
			},
		}},
		Annotations: []*irv1.AnnotationValue{
			mk("if", "condition", "name", ifS, ifE),
			mk("endif", "", "", endif1S, endif1E),
			mk("slice", "param", "ids", sliceS, sliceE),
			mk("endif", "", "", endif2S, endif2E),
			mk("orderby", "allow", "created_at,email", orderbyS, orderbyE),
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

	for _, want := range []string{
		"SearchUsersParams",
		"Name",
		"*string",
		"Ids",
		"[]int64",
		"OrderBy",
		"string",
		"arg.Name != nil",
		"len(arg.Ids) > 0",
		"ANY($%d)",
		"invalid order by",
		// Conditional fragments MUST carry a single leading space so the
		// assembled SQL is valid (e.g. "WHERE true AND email = $1", not "trueAND").
		` AND email = $%d`,
		` AND id = ANY($%d)`,
	} {
		if !strings.Contains(q, want) {
			t.Fatalf("missing %q in:\n%s", want, q)
		}
	}

	// Guard against the original bug: no concatenation without a separating space.
	for _, bad := range []string{
		`"AND email = $%d"`,   // fragment with no leading space
		`"AND id = ANY($%d)"`, // fragment with no leading space
		"trueAND",             // the malformed-SQL signature
	} {
		if strings.Contains(q, bad) {
			t.Fatalf("found malformed (no-space) fragment %q in:\n%s", bad, q)
		}
	}

	// Each conditional Fprintf format string must begin with a space.
	for _, line := range strings.Split(q, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "fmt.Fprintf(&b, ") {
			continue
		}
		// Extract the quoted format string (first arg after &b, ).
		rest := strings.TrimPrefix(trimmed, "fmt.Fprintf(&b, ")
		if !strings.HasPrefix(rest, `" `) {
			t.Fatalf("Fprintf format string does not start with a space: %q", line)
		}
	}

	if _, err := format.Source([]byte(q)); err != nil {
		t.Fatalf("invalid Go: %v\n%s", err, q)
	}

	// STRONGER: replicate the runtime assembly for this example and assert the
	// produced SQL is well-formed (no "trueAND", single spaces between fragments).
	assembled := assembleSearchUsersSQL(strPtr("alice"), []int64{1, 2})
	wantSQL := "SELECT id, email, status FROM app.users WHERE true AND email = $1 AND id = ANY($2)"
	if normalizeSpaces(assembled) != wantSQL {
		t.Fatalf("assembled SQL = %q; want %q", normalizeSpaces(assembled), wantSQL)
	}
	if strings.Contains(assembled, "trueAND") {
		t.Fatalf("assembled SQL contains 'trueAND': %q", assembled)
	}
	if strings.Contains(assembled, "  ") {
		t.Fatalf("assembled SQL contains a double space: %q", assembled)
	}
}

// assembleSearchUsersSQL replicates the runtime SQL assembly that the generated
// SearchUsers method performs, mirroring the exact WriteString/Fprintf calls so
// we can assert the assembled SQL is well-formed without running generated code.
func assembleSearchUsersSQL(name *string, ids []int64) string {
	var b strings.Builder
	var args []any
	b.WriteString("SELECT id, email, status FROM app.users WHERE true")
	if name != nil {
		args = append(args, *name)
		fmt.Fprintf(&b, " AND email = $%d", len(args))
	}
	if len(ids) > 0 {
		args = append(args, ids)
		fmt.Fprintf(&b, " AND id = ANY($%d)", len(args))
	}
	return b.String()
}

func strPtr(s string) *string { return &s }

// normalizeSpaces collapses runs of whitespace to single spaces and trims ends.
func normalizeSpaces(s string) string {
	return strings.Join(strings.Fields(s), " ")
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
