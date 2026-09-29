package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/gopherex/sqld/pkg/devdb"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

//go:embed testdata/dynamic_runtime_test.go
var dynamicRuntimeTest []byte

type dynamicRuntimeCase struct {
	Name         string
	SQL          string
	OptionalIDs  bool
	OptionalFlag bool
	OrderBy      bool
	Mutation     bool
	Want         [][]int64 // nil array, empty array, populated array
}

// Compile and execute the actual generated methods against PostgreSQL. Text
// assertions alone cannot catch dropped predicates, wrong precedence, broken
// argument numbering, or generated code that formats but does not compile.
func TestDynamicGeneratedRuntime(t *testing.T) {
	filtered := [][]int64{{}, {}, {1}}
	cases := []dynamicRuntimeCase{
		{Name: "ScalarSubquery", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id=$2 AND (SELECT p.id > 0 AND true) AND p.id=ANY($1) AND ($3::boolean OR true)", OptionalFlag: true, Want: filtered},
		{Name: "RepeatedUntyped", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id=$2 AND p.id=ANY($1) AND (p.enabled=$3::bool OR $3 IS NULL)", OptionalFlag: true, Want: filtered},
		{Name: "DynamicOrder", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id=$2 AND p.id=ANY($1) ORDER BY p.id DESC LIMIT $4", OptionalIDs: true, OrderBy: true, Want: [][]int64{{1, 2}, {}, {1}}},
		{Name: "DynamicOrderNoWhere", SQL: "SELECT p.id FROM profiles p ORDER BY p.id DESC LIMIT $4; -- WHERE $99", OrderBy: true, Want: [][]int64{{1, 2, 3, 4}, {1, 2, 3, 4}, {1, 2, 3, 4}}},
		{Name: "DynamicOrderGroup", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id=$2 AND p.id=ANY($1) GROUP BY p.id HAVING count(*) > 0 ORDER BY p.id LIMIT $4", OptionalIDs: true, OrderBy: true, Want: [][]int64{{1, 2}, {}, {1}}},
		{Name: "Plain", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY($1)", Want: filtered},
		{Name: "Cast", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY($1::bigint[])", Want: filtered},
		{Name: "Wrapped", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY(($1::bigint[]))", Want: filtered},
		{Name: "InlineOptional", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY($1)", OptionalIDs: true, Want: [][]int64{{1, 2}, {}, {1}}},
		{Name: "OptionalWrapped", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY(($1::bigint[]))", OptionalIDs: true, Want: [][]int64{{1, 2}, {}, {1}}},
		{Name: "AllOptional", SQL: "SELECT p.id FROM profiles p WHERE p.id = ANY($1)", OptionalIDs: true, Want: [][]int64{{1, 2, 3, 4}, {}, {1, 3}}},
		{Name: "Or", SQL: "SELECT p.id FROM profiles p WHERE (p.tenant_id = $2\nOR p.id = ANY($1)) AND ($3::boolean OR true)", OptionalFlag: true, Want: [][]int64{{1, 2}, {1, 2}, {1, 2, 3}}},
		{Name: "Not", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND NOT (p.id = ANY($1))", OptionalIDs: true, Want: [][]int64{{1, 2}, {1, 2}, {2}}},
		{Name: "MixedGroup", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND (p.id = ANY($1) OR p.id = 2)", OptionalIDs: true, Want: [][]int64{{2}, {2}, {1, 2}}},
		{Name: "Between", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id BETWEEN 1\nAND 2 AND p.id = ANY($1)", OptionalIDs: true, Want: [][]int64{{1, 2}, {}, {1}}},
		{Name: "Case", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND CASE WHEN p.id > 0 AND p.id < 5 THEN p.id = ANY($1) ELSE false END", OptionalIDs: true, Want: [][]int64{{1, 2}, {}, {1}}},
		{Name: "Repeated", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY($1) AND ($3::boolean IS NULL OR p.enabled = $3)", OptionalFlag: true, Want: filtered},
		{Name: "NullableRequired", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY($1) AND $3::boolean IS NULL", Want: [][]int64{{}, {}, {}}},
		{Name: "Suffix", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY($1) ORDER BY p.id LIMIT $4 OFFSET 0 FOR UPDATE;", OptionalIDs: true, Want: [][]int64{{1, 2}, {}, {1}}},
		{Name: "BaseParameter", SQL: "SELECT p.id FROM profiles p JOIN (SELECT $2::bigint AS tenant) t ON p.tenant_id = t.tenant WHERE p.id = ANY($1) ORDER BY p.id LIMIT $4", OptionalIDs: true, Want: [][]int64{{1, 2}, {}, {1}}},
		{Name: "OptionalOutsideWhere", SQL: "SELECT p.id FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY($1) LIMIT $4", OptionalIDs: true, Want: [][]int64{{1, 2}, {}, {1}}},
	}
	// Exercise every lexical trap with an ordinary array AND a genuinely
	// optional scalar, so the structural rewriting path is also covered.
	for name, prefix := range map[string]string{
		"LineComment":      "-- WHERE $1 AND ANY($99)\nSELECT p.id FROM profiles p -- trailing WHERE\n",
		"BlockComment":     "/* WHERE /* nested */ $99 ANY($99) */ SELECT p.id FROM profiles p ",
		"Subquery":         "SELECT p.id FROM profiles p JOIN (SELECT id FROM profiles WHERE id > 0) s ON s.id = p.id ",
		"Cte":              "WITH s AS (SELECT id FROM profiles WHERE id > 0) SELECT p.id FROM profiles p JOIN s ON s.id = p.id ",
		"QuotedIdentifier": "SELECT p.id FROM profiles p JOIN (SELECT 1 AS \"WHERE $99\") x ON true ",
		"Literal":          "SELECT p.id FROM profiles p JOIN (SELECT 'WHERE $99 ANY($99)' AS x) s ON s.x = 'WHERE $99 ANY($99)' ",
		"DollarLiteral":    "SELECT p.id FROM profiles p JOIN (SELECT $tag$WHERE $99$tag$ AS x) s ON s.x = $tag$WHERE $99$tag$ ",
		"EscapeLiteral":    `SELECT p.id FROM profiles p JOIN (SELECT E'\' WHERE $99' AS x) s ON length(s.x) > 0 `,
	} {
		for _, optional := range []bool{false, true} {
			suffix := ""
			caseName := name
			if optional {
				suffix = " AND ($3::boolean OR true)"
				caseName += "Dynamic"
			}
			cases = append(cases, dynamicRuntimeCase{Name: caseName, SQL: prefix + "WHERE p.tenant_id = $2 AND p.id = ANY($1)" + suffix, OptionalFlag: optional, Want: filtered})
		}
	}
	for _, mutation := range []struct{ name, sql string }{
		{"Delete", "DELETE FROM profiles p WHERE p.tenant_id = $2 AND p.id = ANY($1)"},
		{"Update", "UPDATE profiles p SET enabled = true WHERE p.tenant_id = $2 AND p.id = ANY($1)"},
	} {
		cases = append(cases, dynamicRuntimeCase{Name: mutation.name, SQL: mutation.sql, Mutation: true, Want: filtered})
		cases = append(cases, dynamicRuntimeCase{Name: mutation.name + "Dynamic", SQL: mutation.sql + " AND ($3::boolean OR true) RETURNING p.id", Mutation: true, OptionalFlag: true, Want: filtered})
	}

	req := &pluginv1.GenerateRequest{Options: []byte(`{"nullMode":"pointer"}`)}
	for _, tc := range cases {
		q := &pluginv1.Query{
			Name: tc.Name, Sql: tc.SQL, Command: pluginv1.QueryCommand_QUERY_COMMAND_MANY,
			Parameters: []*pluginv1.QueryParameter{
				{Number: 1, Name: "ids", Optional: tc.OptionalIDs, Type: &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_ARRAY, Element: &irv1.TypeRef{PgName: "int8"}}},
				{Number: 2, Name: "tenant_id", Type: &irv1.TypeRef{PgName: "int8"}},
				{Number: 3, Name: "flag", Optional: tc.OptionalFlag, Nullable: true, Type: &irv1.TypeRef{PgName: "bool"}},
				{Number: 4, Name: "limit", Optional: tc.Name == "OptionalOutsideWhere", Type: &irv1.TypeRef{PgName: "int8"}},
			},
			Columns: []*pluginv1.QueryColumn{{Name: "id", Type: &irv1.TypeRef{PgName: "int8"}}},
		}
		if tc.Mutation {
			q.Command = pluginv1.QueryCommand_QUERY_COMMAND_EXEC_ROWS
			q.Columns = nil
		}
		req.Queries = append(req.Queries, q)
		if tc.OrderBy {
			req.Annotations = append(req.Annotations, &irv1.AnnotationValue{
				Name: "orderby", Target: &irv1.AnnotationTargetRef{Kind: irv1.AnnotationTargetKind_ANNOTATION_TARGET_KIND_QUERY, QueryName: tc.Name},
				Args: []*irv1.AnnotationArg{{Value: &irv1.AnnotationArg_StringValue{StringValue: "id"}}},
			})
		}
	}
	response, err := Generate(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Diagnostics) > 0 {
		t.Fatalf("generation diagnostics: %v", response.Diagnostics)
	}
	dir := t.TempDir()
	write := func(name string, data []byte) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, file := range response.Files {
		write(file.Path, file.Contents)
	}
	for _, name := range []string{"go.mod", "go.sum"} {
		data, err := os.ReadFile(filepath.Join("..", "..", name))
		if err != nil {
			t.Fatal(err)
		}
		write(name, data)
	}
	data, err := json.Marshal(cases)
	if err != nil {
		t.Fatal(err)
	}
	write("cases.json", data)
	write("dynamic_runtime_test.go", dynamicRuntimeTest)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db, err := devdb.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	cmd := exec.CommandContext(ctx, "go", "test", "-count=1", "-v", ".")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off", "SQLD_DYNAMIC_TEST_URL="+db.URL())
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generated runtime tests: %v\n%s", err, output)
	}
}
