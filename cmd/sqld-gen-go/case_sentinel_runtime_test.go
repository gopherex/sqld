package main

import (
	_ "embed"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/internal/query"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

//go:embed testdata/case_sentinel_runtime_test.go
var caseSentinelRuntimeTest []byte

func TestGeneratedCaseAndSentinelRuntime(t *testing.T) {
	stmts, err := parse.Statements(`CREATE TABLE profiles(id bigint PRIMARY KEY, nickname text);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	queries, err := query.ParseQueries(`
-- name: AssignNickname :execrows
UPDATE profiles SET nickname = CASE WHEN @set_name::bool THEN @name::text ELSE nickname END WHERE id = @id;
-- name: DynamicAssignNickname :execrows
UPDATE profiles SET nickname = CASE WHEN @set_name::bool THEN @name::text ELSE nickname END WHERE id = ANY(@ids::bigint[]);
-- name: ListByNickname :many
SELECT id FROM profiles WHERE (@filter::text = '' OR nickname::text = @filter) ORDER BY id;
-- name: DynamicListByNickname :many
SELECT id FROM profiles WHERE (@filter::text = '' OR nickname::text = @filter) AND id = ANY(@ids::bigint[]) ORDER BY id;
`, "cases.sql")
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics catalog.Diagnostics
	for _, q := range queries {
		query.Infer(q, cat, &diagnostics)
	}
	if len(diagnostics.Items) != 0 {
		t.Fatalf("inference diagnostics: %v", diagnostics.Items)
	}
	response, err := Generate(&pluginv1.GenerateRequest{Catalog: cat, Queries: queries, Options: []byte(`{"nullMode":"pointer"}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(response.Diagnostics) != 0 {
		t.Fatalf("generation diagnostics: %v", response.Diagnostics)
	}
	testGeneratedPostgres(t, response, map[string][]byte{"case_sentinel_runtime_test.go": caseSentinelRuntimeTest})
}
