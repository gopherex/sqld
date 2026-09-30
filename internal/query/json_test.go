package query

import (
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
)

func TestInferJSONResults(t *testing.T) {
	stmts, err := parse.Statements(`CREATE SCHEMA backplane;
 CREATE TABLE backplane.audit (attributes jsonb, doc json);
 CREATE VIEW backplane.audit_feed AS SELECT attributes, doc FROM backplane.audit;`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	for _, tc := range []struct {
		name, sql string
		types     []string
		nullable  []bool
	}{
		{"audit_fields", `SELECT k.key::text AS attribute, count(*) AS count FROM backplane.audit_feed f, LATERAL jsonb_object_keys(f.attributes) AS k(key) GROUP BY k.key`, []string{"text", "int8"}, []bool{false, false}},
		{"implicit_lateral", `SELECT k.key FROM backplane.audit_feed f, jsonb_object_keys(f.attributes) k(key)`, []string{"text"}, []bool{false}},
		{"scalar_alias", `SELECT k FROM backplane.audit_feed f, jsonb_object_keys(f.attributes) k`, []string{"text"}, []bool{false}},
		{"default_name", `SELECT jsonb_object_keys FROM jsonb_object_keys(NULL::jsonb)`, []string{"text"}, []bool{false}},
		{"pg_catalog", `SELECT k.* FROM pg_catalog.jsonb_object_keys('{}'::jsonb) k(key)`, []string{"text"}, []bool{false}},
		{"json_keys", `SELECT k.* FROM backplane.audit_feed f CROSS JOIN json_object_keys(f.doc) k(key)`, []string{"text"}, []bool{false}},
		{"ordinality", `SELECT k.* FROM jsonb_object_keys('{}'::jsonb) WITH ORDINALITY k(key, pos)`, []string{"text", "int8"}, []bool{false, false}},
		{"left_lateral", `SELECT k.* FROM backplane.audit_feed f LEFT JOIN LATERAL jsonb_object_keys(f.attributes) WITH ORDINALITY k(key, pos) ON true`, []string{"text", "int8"}, []bool{true, true}},
		{"select_srf_padding", `SELECT jsonb_object_keys('{}'::jsonb) AS a, jsonb_object_keys('{"x":1}'::jsonb) AS b`, []string{"text", "text"}, []bool{true, true}},
		{"json_elements", `SELECT e.* FROM json_array_elements('[null]'::json) e(value)`, []string{"json"}, []bool{false}},
		{"jsonb_elements", `SELECT e.* FROM jsonb_array_elements('[null]'::jsonb) e(value)`, []string{"jsonb"}, []bool{false}},
		{"text_elements", `SELECT e.* FROM jsonb_array_elements_text('[null]'::jsonb) e(value)`, []string{"text"}, []bool{true}},
		{"json_text_elements", `SELECT e.* FROM json_array_elements_text('[null]'::json) e(value)`, []string{"text"}, []bool{true}},
		{"extract", `SELECT attributes -> 'key', attributes -> 0, attributes #> '{a,b}', attributes ->> 'key', attributes #>> '{a,b}' FROM backplane.audit_feed`, []string{"jsonb", "jsonb", "jsonb", "text", "text"}, []bool{true, true, true, true, true}},
		{"json_extract", `SELECT doc -> 'key', doc #> '{a,b}', doc ->> 'key', doc #>> '{a,b}' FROM backplane.audit_feed`, []string{"json", "json", "text", "text"}, []bool{true, true, true, true}},
		{"coalesce_nullable", `SELECT COALESCE(attributes -> 'a', attributes -> 'b') AS value FROM backplane.audit_feed`, []string{"jsonb"}, []bool{true}},
		{"coalesce_default", `SELECT COALESCE(attributes -> 'a' -> 'b', '{}') AS value FROM backplane.audit_feed`, []string{"jsonb"}, []bool{false}},
		{"coalesce_json", `SELECT COALESCE(doc #> '{a,b}', '{}') AS value FROM backplane.audit_feed`, []string{"json"}, []bool{false}},
		{"coalesce_text", `SELECT COALESCE(attributes ->> 'a', '') AS value FROM backplane.audit_feed`, []string{"text"}, []bool{false}},
		{"missing_nonnull", `SELECT '{}'::jsonb -> 'missing' AS value`, []string{"jsonb"}, []bool{true}},
		{"cte", `WITH keys AS (SELECT k.* FROM jsonb_object_keys('{}'::jsonb) k(key)) SELECT key FROM keys`, []string{"text"}, []bool{false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			qs, err := ParseQueries("-- name: Q :many\n"+tc.sql, "q.sql")
			if err != nil {
				t.Fatal(err)
			}
			var d catalog.Diagnostics
			Infer(qs[0], cat, &d)
			cols := qs[0].GetColumns()
			if len(cols) != len(tc.types) {
				t.Fatalf("columns=%v diagnostics=%v", cols, d.Items)
			}
			for n, c := range cols {
				if c.GetType().GetPgName() != tc.types[n] || c.GetNullable() != tc.nullable[n] {
					t.Errorf("column %d: %v; want %s nullable=%v", n, c, tc.types[n], tc.nullable[n])
				}
			}
			if len(d.Items) > 0 {
				t.Errorf("diagnostics: %v", d.Items)
			}
		})
	}
}

func TestInferJSONParameters(t *testing.T) {
	stmts, err := parse.Statements(`CREATE TABLE events (attributes jsonb NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	for _, tc := range []struct {
		sql, typ string
		array    bool
	}{
		{`SELECT attributes -> @key AS value FROM events`, "text", false},
		{`SELECT attributes -> @index::int AS value FROM events`, "int4", false},
		{`SELECT attributes #> @path AS value FROM events`, "text", true},
		{`SELECT k.key FROM jsonb_object_keys(@doc) k(key)`, "jsonb", false},
		{`SELECT k.key FROM json_object_keys(@doc) k(key)`, "json", false},
	} {
		qs, err := ParseQueries("-- name: Q :many\n"+tc.sql, "q.sql")
		if err != nil {
			t.Fatal(err)
		}
		var d catalog.Diagnostics
		Infer(qs[0], cat, &d)
		if len(d.Items) > 0 {
			t.Errorf("%s: diagnostics=%v", tc.sql, d.Items)
		}
		params := qs[0].GetParameters()
		if len(params) != 1 {
			t.Fatalf("%s: params=%v", tc.sql, params)
		}
		typ := params[0].GetType()
		if tc.array {
			typ = typ.GetElement()
		}
		if typ.GetPgName() != tc.typ {
			t.Errorf("%s: param=%v want=%s", tc.sql, params[0], tc.typ)
		}
	}
}

func TestInferJSONUserFunction(t *testing.T) {
	qs, err := ParseQueries("-- name: Q :many\nSELECT custom.jsonb_object_keys('{}'::jsonb) AS key", "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	var d catalog.Diagnostics
	Infer(qs[0], nil, &d)
	col := qs[0].GetColumns()[0]
	if col.GetType() != nil || !col.GetNullable() {
		t.Fatalf("user function acquired builtin signature: %v", col)
	}
}
