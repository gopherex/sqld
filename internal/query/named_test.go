package query

import (
	"reflect"
	"testing"
)

func TestRewriteNamedParamsBasic(t *testing.T) {
	rewritten, names, optional, _, _, err := rewriteNamedParams("email = @email AND org = @org")
	if err != nil {
		t.Fatal(err)
	}
	want := "email = $1 AND org = $2"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "email", 2: "org"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
	if len(optional) != 0 {
		t.Errorf("optional=%v want empty", optional)
	}
}

func TestRewriteNamedParamsRepeated(t *testing.T) {
	rewritten, names, optional, _, _, err := rewriteNamedParams("@x OR a = @x")
	if err != nil {
		t.Fatal(err)
	}
	want := "$1 OR a = $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "x"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
	if len(optional) != 0 {
		t.Errorf("optional=%v want empty", optional)
	}
}

func TestRewriteNamedParamsSkipOperator(t *testing.T) {
	// @> is a PostgreSQL operator (contains); should NOT be rewritten.
	// @t after the operator IS a named param.
	rewritten, names, optional, _, _, err := rewriteNamedParams("tags @> @t")
	if err != nil {
		t.Fatal(err)
	}
	want := "tags @> $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "t"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
	if len(optional) != 0 {
		t.Errorf("optional=%v want empty", optional)
	}
}

func TestRewriteNamedParamsSkipStringAndComment(t *testing.T) {
	// @b inside single-quoted string, @c inside block comment, @d inside line
	// comment must NOT be rewritten.  Only @real at the end is a param.
	sql := "'a@b' /* @c */ -- @d\n= @real"
	rewritten, names, optional, _, _, err := rewriteNamedParams(sql)
	if err != nil {
		t.Fatal(err)
	}
	want := "'a@b' /* @c */ -- @d\n= $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "real"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
	if len(optional) != 0 {
		t.Errorf("optional=%v want empty", optional)
	}
}

func TestRewriteNamedParamsNoParams(t *testing.T) {
	sql := "SELECT id FROM users WHERE id = $1"
	rewritten, names, optional, _, _, err := rewriteNamedParams(sql)
	if err != nil {
		t.Fatal(err)
	}
	if rewritten != sql {
		t.Errorf("rewritten=%q want unchanged", rewritten)
	}
	if len(names) != 0 {
		t.Errorf("names=%v want empty", names)
	}
	if len(optional) != 0 {
		t.Errorf("optional=%v want empty", optional)
	}
}

func TestRewriteNamedParamsDollarQuote(t *testing.T) {
	// @inside inside a dollar-quoted string must be left alone.
	sql := "$fmt$@inside$fmt$ = @outside"
	rewritten, names, optional, _, _, err := rewriteNamedParams(sql)
	if err != nil {
		t.Fatal(err)
	}
	want := "$fmt$@inside$fmt$ = $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "outside"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
	if len(optional) != 0 {
		t.Errorf("optional=%v want empty", optional)
	}
}

func TestRewriteNamedParamsDoubleAt(t *testing.T) {
	// @@ is a text-search operator; neither @ starts an identifier.
	rewritten, names, optional, _, _, err := rewriteNamedParams("to_tsvector('english', body) @@ @query")
	if err != nil {
		t.Fatal(err)
	}
	want := "to_tsvector('english', body) @@ $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	if len(names) != 1 || names[1] != "query" {
		t.Errorf("names=%v", names)
	}
	if len(optional) != 0 {
		t.Errorf("optional=%v want empty", optional)
	}
}

func TestRewriteNamedParamsOptional(t *testing.T) {
	// @email? is optional (? suffix consumed), @org is plain (not optional).
	rewritten, names, optional, _, _, err := rewriteNamedParams("email = @email? AND org = @org")
	if err != nil {
		t.Fatal(err)
	}
	want := "email = $1 AND org = $2"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "email", 2: "org"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
	// email (pos 1) must be optional; org (pos 2) must not be.
	if !optional[1] {
		t.Errorf("optional[1] (email) = false, want true")
	}
	if optional[2] {
		t.Errorf("optional[2] (org) = true, want false")
	}
	// The '?' must not appear in the rewritten SQL.
	for _, ch := range rewritten {
		if ch == '?' {
			t.Errorf("rewritten SQL contains '?': %q", rewritten)
			break
		}
	}
}

func TestRewriteNullElements(t *testing.T) {
	for _, sql := range []string{
		`SELECT sqld.nullelems('x'), @x, sqlc.arg(x), sqlc.narg(x), $2`,
		`SELECT SQLD /* comment */ . NULLELEMS ( x ), @x, sqlc.arg('x'), sqlc.narg(x), $2`,
		`SELECT "sqld"."nullelems"("x"), @x, sqlc.arg(x), sqlc.narg(x), $2`,
		`SELECT sqld.nullelems(E'\x78'), @x, sqlc.arg(x), sqlc.narg(x), $2`,
	} {
		got, names, optional, nullable, elements, err := rewriteNamedParams(sql)
		if err != nil || got != `SELECT $3, $3, $3, $3, $2` || names[3] != "x" || len(names) != 1 || len(optional) != 0 || !nullable[3] || !elements[3] {
			t.Fatalf("sql=%q names=%v optional=%v nullable=%v elements=%v err=%v", got, names, optional, nullable, elements, err)
		}
	}
}

func TestNullElementsLexicalBoundaries(t *testing.T) {
	for _, sql := range []string{
		`SELECT 'sqld.nullelems(''x'')', $$sqld.other(x)$$`,
		`SELECT "sqld.nullelems(x)", "SQLD".nullelems(x), app.sqld.nullelems(x)`,
		"SELECT 1 /* sqld.other(x) /* nested */ */ -- sqld.nullelems(x)\n",
	} {
		got, names, _, _, elements, err := rewriteNamedParams(sql)
		if err != nil || got != sql || len(names) != 0 || len(elements) != 0 {
			t.Fatalf("sql=%q names=%v elements=%v err=%v", got, names, elements, err)
		}
	}
}
