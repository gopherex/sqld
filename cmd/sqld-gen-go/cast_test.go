package main

import (
	"go/format"
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/internal/query"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

func TestGenerateCastParametersFromSQL(t *testing.T) {
	stmts, err := parse.Statements("CREATE TABLE users(id bigint PRIMARY KEY, col text NOT NULL);")
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	qs, err := query.ParseQueries(`
-- name: CoalesceCast :exec
UPDATE users SET col = COALESCE(@p::text, col) WHERE id = @id;
-- name: DirectCast :exec
UPDATE users SET col = @p::text;
-- name: DirectParam :exec
UPDATE users SET col = @p;
-- name: SelectCast :one
SELECT @p::text AS value;
-- name: NestedCast :exec
UPDATE users SET col = (@p::integer)::text;
-- name: ArrayCast :one
SELECT @p::text[] AS value;
-- name: ConflictCast :exec
INSERT INTO users (id, col) VALUES (1, 'x')
ON CONFLICT (id) DO UPDATE SET col = COALESCE(@p::text, users.col);
-- name: CoalesceContext :exec
UPDATE users SET col = COALESCE(@p, col);
-- name: CaseContext :exec
UPDATE users SET col = CASE WHEN id > 0 THEN @p ELSE col END;
-- name: FunctionContext :exec
UPDATE users SET col = lower(@p);
-- name: ArithmeticContext :exec
UPDATE users SET id = id + @p;
-- name: AllContext :exec
DELETE FROM users WHERE id = ALL(@p);
-- name: DerivedContext :one
SELECT v.col FROM (SELECT col FROM users) v WHERE v.col = @p;
`, "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics catalog.Diagnostics
	for _, q := range qs {
		query.Infer(q, cat, &diagnostics)
	}
	if len(diagnostics.Items) != 0 {
		t.Fatalf("inference diagnostics: %v", diagnostics.Items)
	}
	resp, err := Generate(&pluginv1.GenerateRequest{Catalog: cat, Queries: qs})
	if err != nil {
		t.Fatal(err)
	}
	var source string
	for _, file := range resp.GetFiles() {
		if file.GetPath() == "queries.go" {
			source = string(file.GetContents())
		}
	}
	if _, err := format.Source([]byte(source)); err != nil {
		t.Fatalf("invalid generated Go: %v\n%s", err, source)
	}
	for _, want := range []string{
		"type CoalesceCastParams struct {\n\tP  *string\n\tID int64\n}",
		"DirectCast(ctx context.Context, p *string) error",
		"DirectParam(ctx context.Context, p string) error",
		"SelectCast(ctx context.Context, p *string)",
		"NestedCast(ctx context.Context, p *int32) error",
		"ArrayCast(ctx context.Context, p []string)",
		"ConflictCast(ctx context.Context, p *string) error",
		"CoalesceContext(ctx context.Context, p *string) error",
		"CaseContext(ctx context.Context, p *string) error",
		"FunctionContext(ctx context.Context, p *string) error",
		"ArithmeticContext(ctx context.Context, p *int64) error",
		"AllContext(ctx context.Context, p []int64) error",
		"DerivedContext(ctx context.Context, p string)",
		"COALESCE($1::text, col)",
		"arg.P, arg.ID",
	} {
		if !strings.Contains(source, want) {
			t.Errorf("missing %q in generated Go:\n%s", want, source)
		}
	}
	if strings.Contains(source, " any") {
		t.Errorf("unresolved type in generated queries:\n%s", source)
	}
}
