package main

import (
	_ "embed"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/internal/query"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

//go:embed testdata/view_runtime_test.go
var viewRuntimeTest []byte

const viewSchema = `CREATE SCHEMA backplane;
CREATE TABLE backplane.a (id uuid PRIMARY KEY, t timestamptz NOT NULL, attrs jsonb NOT NULL DEFAULT '{}');
CREATE TABLE backplane.b (id uuid PRIMARY KEY, t timestamptz NOT NULL, detail jsonb NOT NULL DEFAULT '{}');
CREATE VIEW backplane.feed AS
 SELECT 'a' AS source, id, t, attrs AS attributes FROM backplane.a
 UNION ALL SELECT 'b', id, t, detail FROM backplane.b;
CREATE VIEW backplane.nullable_feed AS SELECT NULL::uuid AS id UNION ALL SELECT id FROM backplane.a;
`

func TestGeneratedViewTypesAndRuntime(t *testing.T) {
	stmts, err := parse.Statements(viewSchema)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	qs, err := query.ParseQueries(`
-- name: FeedPlain :many
SELECT source,id,t,attributes FROM backplane.feed ORDER BY source;
-- name: FeedAliased :many
SELECT f.source,f.id,f.t,f.attributes FROM backplane.feed f ORDER BY f.source;
-- name: FeedStar :many
SELECT f.* FROM backplane.feed f ORDER BY f.source;
-- name: FeedLateral :many
SELECT f.id,x.attributes FROM backplane.feed f CROSS JOIN LATERAL (SELECT f.attributes) x ORDER BY f.source;
-- name: FeedPayload :many
SELECT to_jsonb(f) AS payload FROM backplane.feed f ORDER BY f.source;
-- name: FeedNullable :many
SELECT f.id FROM backplane.nullable_feed f ORDER BY f.id NULLS FIRST;
-- name: FeedOptional :many
SELECT f.id FROM backplane.feed f WHERE f.source = @source?::text AND f.id = ANY(@ids?::uuid[]) ORDER BY f.source;
`, "feed.sql")
	if err != nil {
		t.Fatal(err)
	}
	var d catalog.Diagnostics
	for _, q := range qs {
		query.Infer(q, cat, &d)
	}
	if len(d.Items) != 0 {
		t.Fatalf("inference diagnostics: %v", d.Items)
	}
	resp, err := Generate(&pluginv1.GenerateRequest{Catalog: cat, Queries: qs, Options: []byte(`{"nullMode":"pointer","overrides":{"uuid":"github.com/google/uuid.UUID"}}`)})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Diagnostics) != 0 {
		t.Fatalf("generation diagnostics: %v", resp.Diagnostics)
	}
	var source []byte
	for _, file := range resp.Files {
		if file.Path == "queries.go" {
			source = file.Contents
		}
	}
	got := generatedGoTypes(t, source)
	for _, row := range []string{"FeedPlainRow", "FeedAliasedRow", "FeedStarRow"} {
		for field, want := range map[string]string{"Source": "string", "ID": "uuid.UUID", "T": "time.Time", "Attributes": "json.RawMessage"} {
			if got[row+"."+field] != want {
				t.Errorf("%s.%s: got %q, want %s", row, field, got[row+"."+field], want)
			}
		}
	}
	for field, want := range map[string]string{"FeedLateralRow.ID": "uuid.UUID", "FeedLateralRow.Attributes": "json.RawMessage", "FeedPayloadRow.Payload": "json.RawMessage", "FeedNullableRow.ID": "*uuid.UUID", "FeedOptionalParams.Source": "*string", "FeedOptionalParams.Ids": "[]uuid.UUID"} {
		if got[field] != want {
			t.Errorf("%s: got %q, want %s", field, got[field], want)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	testGeneratedPostgres(t, resp, map[string][]byte{"view_runtime_test.go": viewRuntimeTest, "view_schema.sql": []byte(viewSchema)})
}
