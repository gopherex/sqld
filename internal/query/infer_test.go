package query

import (
	"testing"

	"github.com/yaroher/sqld/internal/catalog"
	"github.com/yaroher/sqld/internal/parse"
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
