package query

import (
	"reflect"
	"testing"
)

func TestRewriteNamedParamsBasic(t *testing.T) {
	rewritten, names, optional := rewriteNamedParams("email = @email AND org = @org")
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
	rewritten, names, optional := rewriteNamedParams("@x OR a = @x")
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
	rewritten, names, optional := rewriteNamedParams("tags @> @t")
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
	rewritten, names, optional := rewriteNamedParams(sql)
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
	rewritten, names, optional := rewriteNamedParams(sql)
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
	rewritten, names, optional := rewriteNamedParams(sql)
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
	rewritten, names, optional := rewriteNamedParams("to_tsvector('english', body) @@ @query")
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
	rewritten, names, optional := rewriteNamedParams("email = @email? AND org = @org")
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
