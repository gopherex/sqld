package query

import (
	"strings"
	"testing"

	"github.com/yaroher/sqld/internal/catalog"
	"github.com/yaroher/sqld/internal/parse"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

func TestInferParamsAndColumns(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, email text not null);")
	cat, _ := catalog.Build(stmts)
	qs, _ := ParseQueries("-- name: GetUser :one\nSELECT id, email FROM users WHERE id = $1;\n", "q.sql")
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	if len(qs[0].GetParameters()) != 1 || qs[0].GetParameters()[0].GetNumber() != 1 {
		t.Fatalf("params=%+v", qs[0].GetParameters())
	}
	if qs[0].GetParameters()[0].GetType().GetPgName() != "int8" {
		t.Fatalf("param type=%v", qs[0].GetParameters()[0].GetType())
	}
	if len(qs[0].GetColumns()) != 2 {
		t.Fatalf("columns=%d", len(qs[0].GetColumns()))
	}
	if qs[0].GetColumns()[0].GetName() != "id" || qs[0].GetColumns()[0].GetType().GetPgName() != "int8" {
		t.Fatalf("col0=%+v", qs[0].GetColumns()[0])
	}
}

func TestInferStarExpansion(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, email text not null);")
	cat, _ := catalog.Build(stmts)
	qs, _ := ParseQueries("-- name: ListUsers :many\nSELECT * FROM users;\n", "q.sql")
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	if len(qs[0].GetColumns()) != 2 {
		t.Fatalf("columns=%d (want 2 from star expansion)", len(qs[0].GetColumns()))
	}
}

func TestInferNoParamsNoCols(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, email text not null);")
	cat, _ := catalog.Build(stmts)
	qs, _ := ParseQueries("-- name: DeleteUser :exec\nDELETE FROM users WHERE id = 1;\n", "q.sql")
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	if len(qs[0].GetParameters()) != 0 {
		t.Fatalf("params=%d", len(qs[0].GetParameters()))
	}
	// non-SELECT → no columns
	if len(qs[0].GetColumns()) != 0 {
		t.Fatalf("columns=%d", len(qs[0].GetColumns()))
	}
}

func TestInferUnresolvedParam(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, email text not null);")
	cat, _ := catalog.Build(stmts)
	// $1 compared to a literal not a column, type unresolvable
	qs, _ := ParseQueries("-- name: Q :exec\nDELETE FROM users WHERE $1 = $1;\n", "q.sql")
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	// param should still appear, just without a type
	if len(qs[0].GetParameters()) != 1 {
		t.Fatalf("params=%d", len(qs[0].GetParameters()))
	}
	if qs[0].GetParameters()[0].GetType() != nil {
		t.Fatalf("expected nil type for unresolved param")
	}
	// should have at least one diagnostic about unresolved param
	if len(d.Items) == 0 {
		t.Fatal("expected diagnostics for unresolved param type")
	}
}

// Bug 1: SELECT * over a JOIN should yield columns from both tables.
func TestInferStarOverJoin(t *testing.T) {
	stmts, _ := parse.Statements(`
		CREATE TABLE a(id bigint primary key, x text);
		CREATE TABLE b(id bigint primary key, a_id bigint);
	`)
	cat, _ := catalog.Build(stmts)
	qs, _ := ParseQueries("-- name: J :many\nSELECT * FROM a JOIN b ON b.a_id = a.id;\n", "q.sql")
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	if len(qs[0].GetColumns()) != 4 {
		t.Fatalf("star-join columns = %d, want 4", len(qs[0].GetColumns()))
	}
}

// Bug 2: params in JOIN ON should be collected and typed.
func TestInferParamInJoinOn(t *testing.T) {
	stmts, _ := parse.Statements(`
		CREATE TABLE a(id bigint primary key);
		CREATE TABLE b(id bigint primary key, a_id bigint);
	`)
	cat, _ := catalog.Build(stmts)
	qs, _ := ParseQueries("-- name: J :many\nSELECT a.id FROM a JOIN b ON b.a_id = $1;\n", "q.sql")
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	if len(qs[0].GetParameters()) != 1 {
		t.Fatalf("join-on params = %d, want 1", len(qs[0].GetParameters()))
	}
}

// TestInferNamedParam verifies end-to-end: a query that uses @email named
// param is rewritten to $1 by ParseQueries, and after Infer the parameter
// carries Name="email", Number=1, and its type resolved from the catalog.
func TestInferNamedParam(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, email text not null);")
	cat, _ := catalog.Build(stmts)
	qs, err := ParseQueries("-- name: GetByEmail :one\nSELECT id FROM users WHERE email = @email;\n", "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 1 {
		t.Fatalf("queries=%d", len(qs))
	}
	q := qs[0]
	// Sql must contain $1, not @email.
	if !strings.Contains(q.GetSql(), "$1") {
		t.Errorf("Sql=%q does not contain $1", q.GetSql())
	}
	if strings.Contains(q.GetSql(), "@email") {
		t.Errorf("Sql=%q still contains @email", q.GetSql())
	}
	var d catalog.Diagnostics
	Infer(q, cat, &d)
	params := q.GetParameters()
	if len(params) != 1 {
		t.Fatalf("params=%d want 1", len(params))
	}
	p := params[0]
	if p.GetNumber() != 1 {
		t.Errorf("Number=%d want 1", p.GetNumber())
	}
	if p.GetName() != "email" {
		t.Errorf("Name=%q want email", p.GetName())
	}
	if p.GetType() == nil || p.GetType().GetPgName() != "text" {
		t.Errorf("Type=%v want text", p.GetType())
	}
}

// TestInferOptionalNamedParam verifies that @name? sets Optional=true on the
// resulting QueryParameter while plain @name leaves Optional==false, and that
// both params are fully typed after Infer.
func TestInferOptionalNamedParam(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, email text not null);")
	cat, _ := catalog.Build(stmts)
	qs, err := ParseQueries(
		"-- name: S :many\nSELECT id FROM users WHERE email = @email? AND id = @id;\n",
		"q.sql",
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 1 {
		t.Fatalf("queries=%d", len(qs))
	}
	q := qs[0]

	// Rewritten SQL must contain $1 and $2, no @ and no ?.
	if !strings.Contains(q.GetSql(), "$1") || !strings.Contains(q.GetSql(), "$2") {
		t.Errorf("Sql=%q does not contain $1/$2", q.GetSql())
	}
	if strings.Contains(q.GetSql(), "@") {
		t.Errorf("Sql=%q still contains @", q.GetSql())
	}
	if strings.Contains(q.GetSql(), "?") {
		t.Errorf("Sql=%q still contains ?", q.GetSql())
	}

	var d catalog.Diagnostics
	Infer(q, cat, &d)

	params := q.GetParameters()
	if len(params) != 2 {
		t.Fatalf("params=%d want 2", len(params))
	}

	// Locate email and id params (order may be by position: $1=email, $2=id).
	var emailParam, idParam *pluginv1.QueryParameter
	for _, p := range params {
		switch p.GetName() {
		case "email":
			emailParam = p
		case "id":
			idParam = p
		}
	}
	if emailParam == nil {
		t.Fatal("email param not found")
	}
	if idParam == nil {
		t.Fatal("id param not found")
	}

	// email must be Optional, id must not.
	if !emailParam.GetOptional() {
		t.Errorf("email param Optional=false, want true")
	}
	if idParam.GetOptional() {
		t.Errorf("id param Optional=true, want false")
	}

	// Both must have types resolved.
	if emailParam.GetType() == nil || emailParam.GetType().GetPgName() != "text" {
		t.Errorf("email type=%v want text", emailParam.GetType())
	}
	if idParam.GetType() == nil || idParam.GetType().GetPgName() != "int8" {
		t.Errorf("id type=%v want int8", idParam.GetType())
	}
}

// TestInferParamNamesComparison: positional param beside a column ref gets the
// column name.
func TestInferParamNamesComparison(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, email text not null);")
	cat, _ := catalog.Build(stmts)
	qs, err := ParseQueries("-- name: G :one\nSELECT id FROM users WHERE id = $1;\n", "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	params := qs[0].GetParameters()
	if len(params) != 1 {
		t.Fatalf("params=%d want 1", len(params))
	}
	p := params[0]
	if p.GetName() != "id" {
		t.Errorf("param $1 Name=%q want \"id\"", p.GetName())
	}
	if p.GetType() == nil || p.GetType().GetPgName() != "int8" {
		t.Errorf("param $1 Type=%v want int8", p.GetType())
	}
}

// TestInferParamNamesUpdate: positional params in UPDATE SET get names from the
// assigned column, and params in WHERE get names from the comparison column.
func TestInferParamNamesUpdate(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, status text not null);")
	cat, _ := catalog.Build(stmts)
	qs, err := ParseQueries("-- name: S :execrows\nUPDATE users SET status = $2 WHERE id = $1;\n", "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	params := qs[0].GetParameters()
	if len(params) != 2 {
		t.Fatalf("params=%d want 2", len(params))
	}
	// params are ordered by position: $1 first, $2 second.
	p1 := params[0]
	p2 := params[1]
	if p1.GetNumber() != 1 {
		t.Fatalf("params[0].Number=%d want 1", p1.GetNumber())
	}
	if p2.GetNumber() != 2 {
		t.Fatalf("params[1].Number=%d want 2", p2.GetNumber())
	}
	if p1.GetName() != "id" {
		t.Errorf("$1 Name=%q want \"id\"", p1.GetName())
	}
	if p2.GetName() != "status" {
		t.Errorf("$2 Name=%q want \"status\"", p2.GetName())
	}
}

// TestInferParamNamesInsert: positional params in INSERT VALUES get names from
// the corresponding column in the column list.
func TestInferParamNamesInsert(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, email text not null);")
	cat, _ := catalog.Build(stmts)
	qs, err := ParseQueries(
		"-- name: C :one\nINSERT INTO users (id, email) VALUES ($1, $2) RETURNING id;\n",
		"q.sql",
	)
	if err != nil {
		t.Fatal(err)
	}
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	params := qs[0].GetParameters()
	if len(params) != 2 {
		t.Fatalf("params=%d want 2", len(params))
	}
	p1 := params[0]
	p2 := params[1]
	if p1.GetNumber() != 1 {
		t.Fatalf("params[0].Number=%d want 1", p1.GetNumber())
	}
	if p2.GetNumber() != 2 {
		t.Fatalf("params[1].Number=%d want 2", p2.GetNumber())
	}
	if p1.GetName() != "id" {
		t.Errorf("$1 Name=%q want \"id\"", p1.GetName())
	}
	if p2.GetName() != "email" {
		t.Errorf("$2 Name=%q want \"email\"", p2.GetName())
	}
}

// TestInferParamNamesNamedNotOverwritten: named params keep their name after Infer.
func TestInferParamNamesNamedNotOverwritten(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE users(id bigint primary key, email text not null);")
	cat, _ := catalog.Build(stmts)
	qs, err := ParseQueries(
		"-- name: GetByEmail :one\nSELECT id FROM users WHERE email = @email;\n",
		"q.sql",
	)
	if err != nil {
		t.Fatal(err)
	}
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	params := qs[0].GetParameters()
	if len(params) != 1 {
		t.Fatalf("params=%d want 1", len(params))
	}
	p := params[0]
	if p.GetName() != "email" {
		t.Errorf("Name=%q want \"email\" (named param must not be overwritten)", p.GetName())
	}
	if p.GetType() == nil || p.GetType().GetPgName() != "text" {
		t.Errorf("Type=%v want text", p.GetType())
	}
}

// Bug 3: resolveColumnRef cross-table fallback must be deterministic.
// Run it many times; if non-deterministic, the type will flip between runs.
func TestInferColumnRefFallbackDeterministic(t *testing.T) {
	stmts, _ := parse.Statements(`
		CREATE TABLE t1(val bigint);
		CREATE TABLE t2(val bigint);
		CREATE TABLE t3(val bigint);
		CREATE TABLE t4(val bigint);
		CREATE TABLE t5(val bigint);
	`)
	cat, _ := catalog.Build(stmts)
	// Query with unqualified col ref that forces cross-table fallback.
	qs, _ := ParseQueries("-- name: Q :many\nSELECT val FROM t1;\n", "q.sql")

	var firstID string
	for i := 0; i < 50; i++ {
		q := proto.Clone(qs[0]).(*pluginv1.Query)
		q.Parameters = nil
		q.Columns = nil
		var d catalog.Diagnostics
		Infer(q, cat, &d)
		if len(q.GetColumns()) == 0 {
			t.Fatal("no columns resolved")
		}
		col := q.GetColumns()[0]
		if col.GetSourceColumn() == nil {
			continue
		}
		id := col.GetSourceColumn().GetId()
		if i == 0 {
			firstID = id
		} else if id != firstID {
			t.Fatalf("non-deterministic: got source_column.id=%q on run %d, want %q", id, i, firstID)
		}
	}
}

// ---------------------------------------------------------------------------
// Deeper expression-type inference
// ---------------------------------------------------------------------------

func col0Type(t *testing.T, ddl, query string) *irv1.TypeRef {
	t.Helper()
	stmts, _ := parse.Statements(ddl)
	cat, _ := catalog.Build(stmts)
	qs, _ := ParseQueries(query, "q.sql")
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	if len(qs[0].GetColumns()) == 0 {
		t.Fatal("no columns")
	}
	return qs[0].GetColumns()[0].GetType()
}

func col0(t *testing.T, ddl, query string) *pluginv1.QueryColumn {
	t.Helper()
	stmts, _ := parse.Statements(ddl)
	cat, _ := catalog.Build(stmts)
	qs, _ := ParseQueries(query, "q.sql")
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	if len(qs[0].GetColumns()) == 0 {
		t.Fatal("no columns")
	}
	return qs[0].GetColumns()[0]
}

func TestInferCount(t *testing.T) {
	ty := col0Type(t, "CREATE TABLE u(id bigint);", "-- name: C :one\nSELECT count(*) FROM u;")
	if ty.GetPgName() != "int8" {
		t.Fatalf("count → %v", ty)
	}
}

func TestInferCast(t *testing.T) {
	ty := col0Type(t, "CREATE TABLE u(id bigint);", "-- name: C :one\nSELECT id::text FROM u;")
	if ty.GetPgName() != "text" {
		t.Fatalf("cast → %v", ty)
	}
}

func TestInferMax(t *testing.T) {
	ty := col0Type(t, "CREATE TABLE u(id bigint, created timestamptz);", "-- name: C :one\nSELECT max(created) FROM u;")
	if ty.GetPgName() != "timestamptz" {
		t.Fatalf("max → %v", ty)
	}
}

func TestInferLiteralAndLower(t *testing.T) {
	ty := col0Type(t, "CREATE TABLE u(id bigint, name text);", "-- name: C :one\nSELECT lower(name) FROM u;")
	if ty.GetPgName() != "text" {
		t.Fatalf("lower → %v", ty)
	}
}

func TestInferSum(t *testing.T) {
	// sum(int4) → int8
	ty := col0Type(t, "CREATE TABLE u(ii integer);", "-- name: C :one\nSELECT sum(ii) FROM u;")
	if ty.GetPgName() != "int8" {
		t.Fatalf("sum(int4) → %v", ty)
	}
	// sum(bigint) → numeric
	ty = col0Type(t, "CREATE TABLE u(id bigint);", "-- name: C :one\nSELECT sum(id) FROM u;")
	if ty.GetPgName() != "numeric" {
		t.Fatalf("sum(int8) → %v", ty)
	}
	// sum(float8) → float8
	ty = col0Type(t, "CREATE TABLE u(f double precision);", "-- name: C :one\nSELECT sum(f) FROM u;")
	if ty.GetPgName() != "float8" {
		t.Fatalf("sum(float8) → %v", ty)
	}
}

func TestInferAvg(t *testing.T) {
	ty := col0Type(t, "CREATE TABLE u(id bigint);", "-- name: C :one\nSELECT avg(id) FROM u;")
	if ty.GetPgName() != "numeric" {
		t.Fatalf("avg → %v", ty)
	}
}

func TestInferIntLiteral(t *testing.T) {
	ty := col0Type(t, "CREATE TABLE u(id bigint);", "-- name: C :one\nSELECT 42 AS n FROM u;")
	if ty.GetPgName() != "int4" {
		t.Fatalf("int literal → %v", ty)
	}
}

func TestInferComparisonIsBool(t *testing.T) {
	c := col0(t, "CREATE TABLE u(id bigint);", "-- name: C :one\nSELECT id = 1 AS eq FROM u;")
	if c.GetType().GetPgName() != "bool" {
		t.Fatalf("comparison → %v", c.GetType())
	}
	if c.GetNullable() {
		t.Fatalf("comparison should be non-null")
	}
}

func TestInferArithmetic(t *testing.T) {
	// int8 + int literal → int8 (widest)
	ty := col0Type(t, "CREATE TABLE u(id bigint);", "-- name: C :one\nSELECT id + 1 AS s FROM u;")
	if ty.GetPgName() != "int8" {
		t.Fatalf("arithmetic → %v", ty)
	}
}

func TestInferNowAndLength(t *testing.T) {
	ty := col0Type(t, "CREATE TABLE u(name text);", "-- name: C :one\nSELECT now() AS t FROM u;")
	if ty.GetPgName() != "timestamptz" {
		t.Fatalf("now → %v", ty)
	}
	ty = col0Type(t, "CREATE TABLE u(name text);", "-- name: C :one\nSELECT length(name) AS l FROM u;")
	if ty.GetPgName() != "int4" {
		t.Fatalf("length → %v", ty)
	}
}

func TestInferArrayAgg(t *testing.T) {
	c := col0(t, "CREATE TABLE u(name text);", "-- name: C :one\nSELECT array_agg(name) AS names FROM u;")
	ty := c.GetType()
	if ty.GetKind() != irv1.TypeKind_TYPE_KIND_ARRAY || ty.GetElement().GetPgName() != "text" {
		t.Fatalf("array_agg → %v", ty)
	}
}

func TestInferCoalesce(t *testing.T) {
	ty := col0Type(t, "CREATE TABLE u(name text);", "-- name: C :one\nSELECT coalesce(name, 'x') AS n FROM u;")
	if ty.GetPgName() != "text" {
		t.Fatalf("coalesce → %v", ty)
	}
}

func TestInferUnknownFuncStaysNil(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TABLE u(id bigint);")
	cat, _ := catalog.Build(stmts)
	qs, _ := ParseQueries("-- name: C :one\nSELECT some_unknown_fn(id) FROM u;", "q.sql")
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	if len(qs[0].GetColumns()) == 0 {
		t.Fatal("no columns")
	}
	if qs[0].GetColumns()[0].GetType() != nil {
		t.Fatalf("unknown func should stay nil, got %v", qs[0].GetColumns()[0].GetType())
	}
}

func TestInferDefaultColumnName(t *testing.T) {
	// Function call with no alias → column name is the function name.
	c := col0(t, "CREATE TABLE u(id bigint);", "-- name: C :one\nSELECT count(*) FROM u;")
	if c.GetName() != "count" {
		t.Fatalf("default name → %q", c.GetName())
	}
}
