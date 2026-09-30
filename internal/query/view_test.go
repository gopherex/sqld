package query

import (
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	"google.golang.org/protobuf/proto"
)

func TestInferViewColumns(t *testing.T) {
	stmts, err := parse.Statements(`
CREATE SCHEMA backplane;
CREATE TABLE backplane.a (id uuid PRIMARY KEY, t timestamptz NOT NULL, attrs jsonb NOT NULL DEFAULT '{}');
CREATE TABLE backplane.b (id uuid PRIMARY KEY, t timestamptz NOT NULL, detail jsonb NOT NULL DEFAULT '{}');
CREATE VIEW backplane.feed AS SELECT 'a' AS source, id, t, attrs AS attributes FROM backplane.a UNION ALL SELECT 'b', id, t, detail FROM backplane.b;
CREATE VIEW backplane.cast_feed AS SELECT 'a'::text AS source, id FROM backplane.a UNION ALL SELECT 'b'::text, id FROM backplane.b;
CREATE VIEW backplane.nullable_feed AS SELECT NULL AS id UNION ALL SELECT id FROM backplane.a;
CREATE VIEW backplane.named(origin, item_id, created_at, payload) AS SELECT * FROM backplane.feed;
CREATE VIEW backplane.chained AS SELECT item_id, payload FROM backplane.named;
CREATE MATERIALIZED VIEW backplane.cached AS SELECT * FROM backplane.feed;
CREATE VIEW backplane.widen AS SELECT 1::smallint AS n UNION ALL SELECT 2::bigint;
CREATE VIEW backplane.reverse_widen AS SELECT 1::bigint AS n UNION ALL SELECT 2::smallint;
CREATE SCHEMA other;
CREATE TABLE other.feed(id bigint NOT NULL);
CREATE VIEW backplane.other_feed AS SELECT f.id FROM other.feed f;
CREATE VIEW backplane.cte_feed AS WITH feed AS (SELECT 42::int AS id) SELECT id FROM feed;
CREATE VIEW backplane.scoped_feed AS WITH feed AS (SELECT 42::int AS id) SELECT f.id FROM backplane.feed f;
CREATE VIEW backplane.null_right AS SELECT id FROM backplane.a UNION ALL SELECT NULL;
CREATE VIEW backplane.literal_uuid AS SELECT '00000000-0000-0000-0000-000000000001' AS id UNION ALL SELECT id FROM backplane.a;
CREATE VIEW backplane.literal_json AS SELECT '{}' AS attributes UNION ALL SELECT attrs FROM backplane.a;
CREATE VIEW backplane.intersection AS SELECT id FROM backplane.nullable_feed INTERSECT SELECT id FROM backplane.a;
CREATE VIEW backplane.difference AS SELECT id FROM backplane.nullable_feed EXCEPT SELECT id FROM backplane.a;
CREATE VIEW backplane.star_union AS SELECT a.*, '{}' AS extra FROM backplane.a a UNION ALL SELECT b.*, detail FROM backplane.b b;
CREATE VIEW backplane.null_literal AS SELECT NULL AS value;
CREATE VIEW backplane.null_cte AS WITH v AS (SELECT NULL AS value) SELECT value FROM v;
`)
	if err != nil {
		t.Fatal(err)
	}
	cat, d := catalog.Build(stmts)
	if len(d.Items) != 0 {
		t.Fatalf("catalog: %v", d.Items)
	}
	before := proto.Clone(cat)
	for _, tc := range []struct {
		name, sql string
		types     []string
		nullable  []bool
	}{
		{"plain", `SELECT source,id,t,attributes FROM backplane.feed`, []string{"text", "uuid", "timestamptz", "jsonb"}, []bool{false, false, false, false}},
		{"alias", `SELECT f.source,f.id,f.t,f.attributes FROM backplane.feed f`, []string{"text", "uuid", "timestamptz", "jsonb"}, []bool{false, false, false, false}},
		{"star", `SELECT * FROM backplane.feed`, []string{"text", "uuid", "timestamptz", "jsonb"}, []bool{false, false, false, false}},
		{"alias_star", `SELECT f.* FROM backplane.feed f`, []string{"text", "uuid", "timestamptz", "jsonb"}, []bool{false, false, false, false}},
		{"cast", `SELECT f.source,f.id FROM backplane.cast_feed f`, []string{"text", "uuid"}, []bool{false, false}},
		{"null_left", `SELECT f.id FROM backplane.nullable_feed f`, []string{"uuid"}, []bool{true}},
		{"chain", `SELECT f.item_id,f.payload FROM backplane.chained f`, []string{"uuid", "jsonb"}, []bool{false, false}},
		{"materialized", `SELECT f.id,f.attributes FROM backplane.cached f`, []string{"uuid", "jsonb"}, []bool{false, false}},
		{"widen", `SELECT n FROM backplane.widen`, []string{"int8"}, []bool{false}},
		{"reverse_widen", `SELECT n FROM backplane.reverse_widen`, []string{"int8"}, []bool{false}},
		{"schema", `SELECT f.id FROM backplane.other_feed f`, []string{"int8"}, []bool{false}},
		{"cte", `SELECT id FROM backplane.cte_feed`, []string{"int4"}, []bool{false}},
		{"qualified_over_cte", `SELECT id FROM backplane.scoped_feed`, []string{"uuid"}, []bool{false}},
		{"lateral", `SELECT f.id,x.payload FROM backplane.feed f CROSS JOIN LATERAL (SELECT f.attributes AS payload) x`, []string{"uuid", "jsonb"}, []bool{false, false}},
		{"left_join", `SELECT f.id, f.source FROM backplane.a a LEFT JOIN backplane.feed f ON f.id=a.id`, []string{"uuid", "text"}, []bool{true, true}},
		{"coalesce", `SELECT COALESCE(f.source,'missing') AS source FROM backplane.a a LEFT JOIN backplane.feed f ON f.id=a.id`, []string{"text"}, []bool{false}},
		{"union_query", `SELECT id FROM backplane.a UNION ALL SELECT id FROM backplane.b`, []string{"uuid"}, []bool{false}},
		{"whole_row", `SELECT to_jsonb(f) AS payload FROM backplane.feed f`, []string{"jsonb"}, []bool{false}},
		{"outer_whole_row", `SELECT to_jsonb(f) AS payload FROM backplane.a a LEFT JOIN backplane.feed f ON false`, []string{"jsonb"}, []bool{true}},
		{"null_right", `SELECT id FROM backplane.null_right`, []string{"uuid"}, []bool{true}},
		{"literal_uuid", `SELECT id FROM backplane.literal_uuid`, []string{"uuid"}, []bool{false}},
		{"literal_json", `SELECT attributes FROM backplane.literal_json`, []string{"jsonb"}, []bool{false}},
		{"intersection", `SELECT id FROM backplane.intersection`, []string{"uuid"}, []bool{false}},
		{"difference", `SELECT id FROM backplane.difference`, []string{"uuid"}, []bool{true}},
		{"star_union", `SELECT id,t,attrs,extra FROM backplane.star_union`, []string{"uuid", "timestamptz", "jsonb", "jsonb"}, []bool{false, false, false, false}},
		{"null_literal", `SELECT value FROM backplane.null_literal`, []string{"text"}, []bool{true}},
		{"null_cte", `SELECT value FROM backplane.null_cte`, []string{"text"}, []bool{true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			qs, err := ParseQueries("-- name: Q :many\n"+tc.sql, "q.sql")
			if err != nil {
				t.Fatal(err)
			}
			var diags catalog.Diagnostics
			Infer(qs[0], cat, &diags)
			cols := qs[0].GetColumns()
			if len(cols) != len(tc.types) {
				t.Fatalf("columns=%v; diagnostics=%v", cols, diags.Items)
			}
			for n, c := range cols {
				if c.GetType().GetPgName() != tc.types[n] || c.GetNullable() != tc.nullable[n] {
					t.Errorf("column %d: %v; want type=%s nullable=%v", n, c, tc.types[n], tc.nullable[n])
				}
			}
			if len(diags.Items) != 0 {
				t.Errorf("diagnostics: %v", diags.Items)
			}
		})
	}
	if !proto.Equal(before, cat) {
		t.Fatal("inference mutated the shared catalog")
	}
}

func TestInferRawAndCyclicViews(t *testing.T) {
	stmts, err := parse.Statements(`CREATE TABLE a(id uuid PRIMARY KEY); CREATE VIEW feed AS SELECT id FROM a;`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	v := cat.Schemas[0].Views[0]
	v.Query = &irv1.SelectStmt{RawSql: `SELECT id FROM public.a UNION ALL SELECT NULL`}
	qs, err := ParseQueries("-- name: Q :many\nSELECT f.id FROM public.feed f", "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	var d catalog.Diagnostics
	Infer(qs[0], cat, &d)
	col := qs[0].Columns[0]
	if col.GetType().GetPgName() != "uuid" || !col.GetNullable() || len(d.Items) != 0 {
		t.Fatalf("raw view: %v diagnostics=%v", col, d.Items)
	}

	// A malformed/cyclic catalog must report unresolved types, not recurse
	// forever or borrow a column from an unrelated table.
	v.Query = &irv1.SelectStmt{RawSql: `SELECT f.id FROM public.feed f`}
	qs, err = ParseQueries("-- name: Q :many\nSELECT f.id FROM public.feed f", "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	d = catalog.Diagnostics{}
	Infer(qs[0], cat, &d)
	if qs[0].Columns[0].GetType() != nil || len(d.Items) == 0 {
		t.Fatalf("cycle: %v diagnostics=%v", qs[0].Columns, d.Items)
	}
}
