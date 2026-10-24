package query

import (
	"reflect"
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

func TestSQLCParameterRewrite(t *testing.T) {
	for _, tc := range []struct {
		sql, want string
		names     map[uint32]string
		nullable  map[uint32]bool
	}{
		{`SELECT sqlc.arg('x'), @x, sqlc.narg(x), @y`, `SELECT $1, $1, $1, $2`, map[uint32]string{1: "x", 2: "y"}, map[uint32]bool{1: true}},
		{`SELECT @x, sqlc.narg('x'), sqlc.arg(x)`, `SELECT $1, $1, $1`, map[uint32]string{1: "x"}, map[uint32]bool{1: true}},
		{`SELECT sqlc.narg('x'), @y, $1, $2, @x`, `SELECT $3, $4, $1, $2, $3`, map[uint32]string{3: "x", 4: "y"}, map[uint32]bool{3: true}},
		{`SELECT SQLC /* nested /* x */ */ . NARG ( 'x' )::text`, `SELECT $1::text`, map[uint32]string{1: "x"}, map[uint32]bool{1: true}},
		{`SELECT "sqlc"."arg"("MixedCase"), sqlc.arg('MixedCase'), sqlc.arg(Foo)`, `SELECT $1, $1, $2`, map[uint32]string{1: "MixedCase", 2: "foo"}, map[uint32]bool{}},
		{`SELECT sqlc.arg(E'na\x6de'), sqlc.arg($tag$name$tag$)`, `SELECT $1, $1`, map[uint32]string{1: "name"}, map[uint32]bool{}},
		{`SELECT id FROM t WHERE id=@id AND name=@name?='x'`, `SELECT id FROM t WHERE id=$1 AND name=$2='x'`, map[uint32]string{1: "id", 2: "name"}, map[uint32]bool{}},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			got, names, _, nullable, _, err := rewriteNamedParams(tc.sql)
			if err != nil || got != tc.want || !reflect.DeepEqual(names, tc.names) || !reflect.DeepEqual(nullable, tc.nullable) {
				t.Fatalf("sql=%q names=%v nullable=%v err=%v; want %q %v %v", got, names, nullable, err, tc.want, tc.names, tc.nullable)
			}
		})
	}
}

func TestSQLCParameterLexicalBoundaries(t *testing.T) {
	for _, sql := range []string{
		`SELECT 'sqlc.narg(''x'') @x', $$sqlc.embed(t) @x$$, $tag$sqlc.arg(x)$tag$`,
		`SELECT E'\' sqlc.narg(x) @x'`,
		`SELECT "sqlc.narg('x') @x", "SQLC".narg('x'), app.narg('x'), app.sqlc.arg('x')`,
		"SELECT 1 /* sqlc.narg('x') /* nested */ @x */ -- sqlc.embed(t) @x\n",
		`SELECT doc @> '{}'::jsonb, doc @? '$.a', ts @@ query FROM t`,
	} {
		got, names, _, _, _, err := rewriteNamedParams(sql)
		if err != nil || got != sql || len(names) != 0 {
			t.Errorf("sql=%q names=%v err=%v; want unchanged %q", got, names, err, sql)
		}
	}
}

func TestSQLCMacroErrors(t *testing.T) {
	for _, body := range []string{
		`SELECT sqlc.embed(t) FROM t`, `SELECT sqlc.slice('ids')`, `SELECT sqlc.unknown('x')`,
		`SELECT sqlc.narg()`, `SELECT sqlc.arg('x','y')`, `SELECT sqlc.narg(42)`,
		`SELECT sqlc.narg(NULL)`, `SELECT sqlc.narg('')`, `SELECT sqlc.arg(t.x)`,
		`SELECT sqlc.narg(lower('x'))`, `SELECT sqlc.narg('x' || 'y')`,
		`SELECT sqlc.narg(DISTINCT 'x')`, `SELECT sqlc.arg(*)`, `SELECT sqlc.arg(VARIADIC 'x')`,
		`SELECT sqlc.narg('x' ORDER BY 1)`, `SELECT sqlc.narg('x') OVER ()`,
		`SELECT sqlc.narg('x'`, `SELECT sqlc.narg('x'), @x?`, `SELECT @x?, sqlc.narg('x')`,
		`SELECT U&"s\0071lc".narg('x')`, `SELECT U&"s!0071lc" UESCAPE '!'.embed(t) FROM t`,
		`WITH c AS (INSERT INTO t VALUES (sqlc.slice('x')) RETURNING *) SELECT * FROM c`,
	} {
		t.Run(body, func(t *testing.T) {
			qs, err := ParseQueries("-- name: Bad :one\n"+body, "macros.sql")
			if err == nil || !strings.Contains(err.Error(), "macros.sql: query Bad:") || len(qs) != 0 {
				t.Fatalf("queries=%v err=%v", qs, err)
			}
		})
	}
}

func TestSQLCMacroInference(t *testing.T) {
	stmts, err := parse.Statements(`CREATE TABLE profiles (id bigint PRIMARY KEY, name text NOT NULL, bio text);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, d := catalog.Build(stmts)
	for _, tc := range []struct {
		sql, typ string
		nullable bool
	}{
		{`SELECT id FROM profiles WHERE name=sqlc.narg('x')`, "text", true},
		{`SELECT id FROM profiles WHERE name=sqlc.arg('x')`, "text", false},
		{`SELECT id FROM profiles WHERE bio=sqlc.arg('x')`, "text", true},
		{`SELECT id FROM profiles WHERE sqlc.narg('x')::text='' OR name=@x`, "text", true},
		{`SELECT id FROM profiles WHERE name=@x OR sqlc.narg('x')::text=''`, "text", true},
		{`SELECT id FROM profiles LIMIT sqlc.narg('x')`, "int8", true},
		{`SELECT id FROM profiles LIMIT sqlc.arg('x')`, "int8", false},
		{`INSERT INTO profiles(id,name) VALUES (1,sqlc.narg('x'))`, "text", true},
		{`INSERT INTO profiles(id,name) SELECT 1,sqlc.narg('x')`, "text", true},
		{`UPDATE profiles SET name=CASE WHEN true THEN sqlc.narg('x') ELSE name END`, "text", true},
		{`UPDATE profiles SET name=COALESCE(sqlc.narg('x'),name)`, "text", true},
		{`WITH c AS (INSERT INTO profiles(id,name) VALUES (1,sqlc.narg('x')) RETURNING id) SELECT * FROM c`, "text", true},
		{`INSERT INTO profiles(id,name) VALUES (1,'a') ON CONFLICT(id) DO UPDATE SET name=sqlc.narg('x')`, "text", true},
		{`SELECT sqlc.narg('x')::bool`, "bool", true},
		{`SELECT lower(sqlc.narg('x'))`, "text", true},
	} {
		t.Run(tc.sql, func(t *testing.T) {
			qs, err := ParseQueries("-- name: Q :one\n"+tc.sql, "q.sql")
			if err != nil || len(qs) != 1 {
				t.Fatalf("queries=%v err=%v", qs, err)
			}
			// The explicit contract survives transport and repeated inference.
			wire, err := proto.Marshal(qs[0])
			if err != nil {
				t.Fatal(err)
			}
			q := &pluginv1.Query{}
			if err := proto.Unmarshal(wire, q); err != nil {
				t.Fatal(err)
			}
			for n := 0; n < 2; n++ {
				Infer(q, cat, d)
				if len(q.Parameters) != 1 {
					t.Fatalf("parameters=%v", q.Parameters)
				}
				p := q.Parameters[0]
				if p.Name != "x" || p.GetType().GetPgName() != tc.typ || p.Nullable != tc.nullable || p.Optional {
					t.Fatalf("parameter=%v want type=%s nullable=%v", p, tc.typ, tc.nullable)
				}
			}
		})
	}
	if len(d.Items) != 0 {
		t.Fatal(d.Items)
	}
}

func TestNullElementsMacroInference(t *testing.T) {
	for _, sql := range []string{
		`SELECT sqld.nullelems('x')::bigint[]`,
		`SELECT @x::bigint[], sqld.nullelems(x)::bigint[]`,
		`SELECT sqlc.narg(x)::bigint[], sqld.nullelems('x')::bigint[]`,
	} {
		qs, err := ParseQueries("-- name: Q :one\n"+sql, "q.sql")
		if err != nil || len(qs) != 1 {
			t.Fatalf("queries=%v err=%v", qs, err)
		}
		wire, err := proto.Marshal(qs[0])
		if err != nil {
			t.Fatal(err)
		}
		q := &pluginv1.Query{}
		if err := proto.Unmarshal(wire, q); err != nil {
			t.Fatal(err)
		}
		d := &catalog.Diagnostics{}
		Infer(q, nil, d)
		Infer(q, nil, d)
		if len(d.Items) != 0 {
			t.Fatal(d.Items)
		}
		if len(q.Parameters) != 1 || !q.Parameters[0].GetNullableElements() || q.Parameters[0].GetForceNullable() != strings.Contains(sql, "sqlc.narg") {
			t.Fatalf("parameter flags=%v", q.Parameters)
		}
		if len(qs[0].Parameters) != 1 || strings.Contains(qs[0].Sql, "sqld.") {
			t.Fatalf("query=%v", qs[0])
		}
	}
}

func TestNullElementsMacroErrors(t *testing.T) {
	for _, body := range []string{
		`SELECT sqld.other('x')`, `SELECT sqld.arg(x)`,
		`SELECT sqld.nullelems()`, `SELECT sqld.nullelems('')`,
		`SELECT sqld.nullelems(42)`, `SELECT sqld.nullelems(x,y)`,
		`SELECT sqld.nullelems(t.x)`, `SELECT sqld.nullelems(lower('x'))`,
		`SELECT sqld.nullelems(DISTINCT x)`, `SELECT sqld.nullelems(*)`,
		`SELECT sqld.nullelems('x') OVER ()`, `SELECT sqld.nullelems('x'`,
		`SELECT sqld.nullelems(x), @x?`, `SELECT @x?, sqld.nullelems(x)`,
		`SELECT U&"sq\006cd".nullelems(x)`,
		`SELECT U&"sq!006cd" UESCAPE '!'.other(x)`,
	} {
		_, err := ParseQueries("-- name: Bad :one\n"+body, "macros.sql")
		if err == nil || !strings.Contains(err.Error(), "macros.sql: query Bad:") {
			t.Fatalf("%s: err=%v", body, err)
		}
		if strings.Contains(body, "sqld.other") && !strings.Contains(err.Error(), "supported macros: sqld.nullelems") {
			t.Fatalf("missing supported macros: %v", err)
		}
	}
}

func TestNullElementsRequiresArray(t *testing.T) {
	for _, expr := range []string{`sqld.nullelems(x)::text`, `sqld.nullelems(x)::bigint`, `sqld.nullelems(x)`} {
		qs, err := ParseQueries("-- name: Q :one\nSELECT "+expr, "q.sql")
		if err != nil {
			t.Fatal(err)
		}
		d := &catalog.Diagnostics{}
		Infer(qs[0], nil, d)
		found := false
		for _, diag := range d.Items {
			if diag.Severity == "error" && strings.Contains(diag.Message, `sqld.nullelems parameter "x" ($1) must have an array type`) {
				found = true
			}
		}
		if !found {
			t.Fatalf("%s: diagnostics=%v", expr, d.Items)
		}
	}
}
