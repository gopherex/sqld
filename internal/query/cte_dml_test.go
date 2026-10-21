package query

import (
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	"google.golang.org/protobuf/proto"
)

func TestInferDataModifyingCTE(t *testing.T) {
	stmts, err := parse.Statements(`
 CREATE TABLE feeds (kind text NOT NULL, id bytea NOT NULL, parent_kind text, parent_id bytea, PRIMARY KEY(kind,id));
 CREATE TABLE purged_feeds (kind text NOT NULL, id bytea NOT NULL);
 CREATE TABLE device_owners(device bytea PRIMARY KEY, owner bytea NOT NULL, seen_at timestamptz NOT NULL);
 CREATE TABLE device_delivery(device bytea PRIMARY KEY, position bigint NOT NULL);
 CREATE TABLE feed_snapshots(kind text NOT NULL, id bytea NOT NULL, position bigint NOT NULL, type bigint NOT NULL, payload bytea NOT NULL, PRIMARY KEY(kind,id));
 CREATE TABLE feed_items(kind text NOT NULL, id bytea NOT NULL, position bigint NOT NULL, expires timestamptz NOT NULL);
 `)
	if err != nil {
		t.Fatal(err)
	}
	cat, d := catalog.Build(stmts)
	if len(d.Items) != 0 {
		t.Fatal(d.Items)
	}
	before := proto.Clone(cat)
	for _, tc := range []struct {
		name, sql    string
		params, cols []string
		nullable     []bool
	}{
		{"create_feed", `WITH buried AS (SELECT 1 FROM purged_feeds WHERE kind=@kind AND id=@id), made AS (
 INSERT INTO feeds(kind,id,parent_kind,parent_id) SELECT @kind,@id,@parent_kind,@parent_id WHERE NOT EXISTS(SELECT 1 FROM buried) ON CONFLICT DO NOTHING)
 SELECT EXISTS(SELECT 1 FROM buried) AS purged`, []string{"text", "bytea", "text", "bytea"}, []string{"bool"}, []bool{false, false, true, true}},
		{"bind_device", `WITH bound AS (INSERT INTO device_owners(device,owner,seen_at) VALUES(@device,@owner,now())
 ON CONFLICT(device) DO UPDATE SET seen_at=now() WHERE device_owners.owner=EXCLUDED.owner RETURNING owner)
 SELECT owner FROM bound UNION ALL SELECT owner FROM device_owners WHERE device=@device LIMIT 1`, []string{"bytea", "bytea"}, []string{"bytea"}, []bool{false, false}},
		{"store_snapshot", `WITH stored AS (INSERT INTO feed_snapshots(kind,id,position,type,payload) VALUES(@kind,@id,@position,@type,@payload)
 ON CONFLICT(kind,id) DO UPDATE SET position=EXCLUDED.position,type=EXCLUDED.type,payload=EXCLUDED.payload WHERE feed_snapshots.position<=EXCLUDED.position RETURNING position)
 UPDATE feed_items SET expires=now() WHERE kind=@kind AND id=@id AND position<=(SELECT position FROM stored) AND expires='infinity'`,
			[]string{"text", "bytea", "int8", "int8", "bytea"}, nil, []bool{false, false, false, false, false}},
		{"sweep_devices", `WITH dead AS (DELETE FROM device_owners WHERE device IN (
 SELECT device FROM device_owners WHERE seen_at<@dead_before ORDER BY seen_at LIMIT @limit FOR UPDATE SKIP LOCKED) RETURNING device),
 delivery AS (DELETE FROM device_delivery WHERE device IN (SELECT device FROM dead)) SELECT count(*)::int FROM dead`,
			[]string{"timestamptz", "int8"}, []string{"int4"}, []bool{false, false}},
		{"insert_returning_aliases", `WITH made(k,ident) AS (INSERT INTO feeds(kind,id) VALUES(@kind,@id) RETURNING kind,id) SELECT m.k,m.ident FROM made m`, []string{"text", "bytea"}, []string{"text", "bytea"}, []bool{false, false}},
		{"update_returning", `WITH changed AS (UPDATE feeds f SET parent_kind=@parent_kind WHERE f.kind=@kind RETURNING f.*) SELECT parent_kind FROM changed`, []string{"text", "text"}, []string{"text"}, []bool{true, false}},
		{"delete_using", `WITH wanted AS (SELECT id FROM feeds WHERE kind=@kind), gone AS (DELETE FROM device_delivery d USING wanted w WHERE d.device=w.id AND d.position<@position RETURNING d.device) SELECT * FROM gone`, []string{"text", "int8"}, []string{"bytea"}, []bool{false, false}},
		{"conflict_only", `WITH made AS (INSERT INTO feeds(kind,id) VALUES('x','') ON CONFLICT(kind,id) DO UPDATE SET parent_kind=@parent_kind WHERE feeds.id=@id RETURNING kind) SELECT kind FROM made`, []string{"text", "bytea"}, []string{"text"}, []bool{true, false}},
		{"returning_cast", `WITH made AS (INSERT INTO feeds(kind,id) VALUES('x','') RETURNING @marker::uuid AS marker) SELECT marker FROM made`, []string{"uuid"}, []string{"uuid"}, []bool{false}},
		{"unused_update", `WITH changed AS (UPDATE feeds SET parent_kind=@parent_kind) SELECT 42 AS answer`, []string{"text"}, []string{"int4"}, []bool{true}},
		{"positional", `WITH made AS (INSERT INTO feeds(kind,id) VALUES($1,$2) RETURNING kind) SELECT kind FROM made`, []string{"text", "bytea"}, []string{"text"}, []bool{false, false}},
		{"outer_insert", `WITH gone AS (DELETE FROM device_owners WHERE owner=@owner RETURNING device) INSERT INTO device_delivery(device,position) SELECT device,@position FROM gone RETURNING position`, []string{"bytea", "int8"}, []string{"int8"}, []bool{false, false}},
		{"target_not_cte", `WITH feeds AS (SELECT 1 AS kind), made AS (INSERT INTO feeds(kind,id) VALUES(@kind,@id) RETURNING kind) SELECT kind FROM made`, []string{"text", "bytea"}, []string{"text"}, []bool{false, false}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			qs, err := ParseQueries("-- name: Q :many\n"+tc.sql, "q.sql")
			if err != nil {
				t.Fatal(err)
			}
			var d catalog.Diagnostics
			Infer(qs[0], cat, &d)
			params, cols := qs[0].GetParameters(), qs[0].GetColumns()
			if len(params) != len(tc.params) || len(cols) != len(tc.cols) {
				t.Fatalf("params=%v cols=%v diagnostics=%v", params, cols, d.Items)
			}
			for n, p := range params {
				if p.GetType().GetPgName() != tc.params[n] || p.GetNullable() != tc.nullable[n] {
					t.Errorf("param %s: %v; want %s nullable=%v", p.GetName(), p, tc.params[n], tc.nullable[n])
				}
			}
			for n, c := range cols {
				if c.GetType().GetPgName() != tc.cols[n] {
					t.Errorf("column %d: %v; want %s", n, c, tc.cols[n])
				}
			}
			if len(d.Items) != 0 {
				t.Errorf("diagnostics: %v", d.Items)
			}
		})
	}
	if !proto.Equal(before, cat) {
		t.Fatal("inference mutated catalog")
	}
}

func TestCTEParameterColumnProvenance(t *testing.T) {
	stmts, err := parse.Statements(`CREATE TABLE tags(code bigint NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	for _, tc := range []struct{ sql, typ, column string }{
		{`WITH made AS (INSERT INTO tags VALUES(@code::bigint) RETURNING code) SELECT code FROM made`, "int8", "public.tags.code"},
		{`WITH made AS (INSERT INTO tags SELECT @code::bigint RETURNING code) SELECT code FROM made`, "int8", "public.tags.code"},
		{`WITH made AS (INSERT INTO tags VALUES(@code::text::bigint) RETURNING code) SELECT code FROM made`, "text", ""},
		{`WITH made AS (INSERT INTO tags SELECT @code::text::bigint RETURNING code) SELECT code FROM made`, "text", ""},
	} {
		qs, err := ParseQueries("-- name: Q :one\n"+tc.sql, "q.sql")
		if err != nil {
			t.Fatal(err)
		}
		var d catalog.Diagnostics
		Infer(qs[0], cat, &d)
		if len(d.Items) != 0 {
			t.Fatal(d.Items)
		}
		p := qs[0].GetParameters()[0]
		if p.GetType().GetPgName() != tc.typ || p.GetColumn().GetId() != tc.column {
			t.Errorf("%s: %v; want %s column=%s", tc.sql, p, tc.typ, tc.column)
		}
	}
}
