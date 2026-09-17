package main

import (
	"bytes"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/internal/query"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

func TestGenerateParameterAndResultNullability(t *testing.T) {
	stmts, err := parse.Statements(`
CREATE TABLE users(id uuid PRIMARY KEY, handle text NOT NULL, nickname text);
CREATE TABLE commands(user_id uuid NOT NULL, name text NOT NULL, position int NOT NULL, cursor bytea NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	qs, err := query.ParseQueries(`
-- name: Search :many
SELECT u.handle AS direct_handle, h.handle AS joined_handle,
       COALESCE(h.handle, '') AS handle,
       EXISTS(SELECT 1 FROM commands c WHERE c.user_id = u.id) AS on_air,
       (SELECT count(*) FROM commands c WHERE c.user_id = u.id)::int AS new_count
FROM users u LEFT JOIN users h ON h.id = u.id
WHERE (@handle_exact::text = '' OR h.handle = @handle_exact)
  AND (@query_pattern::text = '' OR u.handle ILIKE @query_pattern)
  AND (@after::uuid IS NULL OR u.id > @after)
LIMIT @lim;
-- name: Owner :one
SELECT id FROM users WHERE lower(handle) = lower(@handle::text);
-- name: Scope :one
SELECT COALESCE((SELECT id FROM users WHERE id = @chat_id::uuid), @chat_id::uuid) AS scope;
-- name: Batch :exec
INSERT INTO commands(user_id, name, position, cursor)
SELECT @user_id::uuid, unnest(@names::text[]), unnest(@positions::int[]), unnest(@cursors::bytea[]);
-- name: Patch :exec
UPDATE users SET handle = COALESCE(@handle::text, handle) WHERE id = @id;
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
	resp, err := Generate(&pluginv1.GenerateRequest{
		Catalog: cat, Queries: qs,
		Options: []byte(`{"nullMode":"pointer","overrides":{"uuid":"github.com/google/uuid.UUID"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var source []byte
	for _, file := range resp.GetFiles() {
		if file.GetPath() == "queries.go" {
			source = file.GetContents()
		}
	}
	got := generatedGoTypes(t, source)
	for field, want := range map[string]string{
		"SearchParams.HandleExact": "string", "SearchParams.QueryPattern": "string",
		"SearchParams.After": "*uuid.UUID", "SearchParams.Lim": "int64",
		"SearchRow.DirectHandle": "string", "SearchRow.JoinedHandle": "*string",
		"SearchRow.Handle": "string", "SearchRow.OnAir": "bool", "SearchRow.NewCount": "int32",
		"Owner.handle": "string", "Scope.chatID": "uuid.UUID",
		"BatchParams.UserID": "uuid.UUID", "BatchParams.Names": "[]string",
		"BatchParams.Positions": "[]int32", "BatchParams.Cursors": "[][]byte",
		"PatchParams.Handle": "*string", "PatchParams.ID": "uuid.UUID",
	} {
		if got[field] != want {
			t.Errorf("%s: got %q, want %q", field, got[field], want)
		}
	}
}

func generatedGoTypes(t *testing.T, source []byte) map[string]string {
	t.Helper()
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "queries.go", source, parser.AllErrors)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	addFields := func(owner string, fields *ast.FieldList) {
		for _, field := range fields.List {
			var typ bytes.Buffer
			if err := format.Node(&typ, fset, field.Type); err != nil {
				t.Fatal(err)
			}
			for _, name := range field.Names {
				got[owner+"."+name.Name] = typ.String()
			}
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.TypeSpec:
			if s, ok := n.Type.(*ast.StructType); ok {
				addFields(n.Name.Name, s.Fields)
			}
		case *ast.FuncDecl:
			addFields(n.Name.Name, n.Type.Params)
		}
		return true
	})
	return got
}
