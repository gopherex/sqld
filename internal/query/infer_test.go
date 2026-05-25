package query

import (
	"testing"

	"github.com/yaroher/sqld/internal/catalog"
	"github.com/yaroher/sqld/internal/parse"
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
