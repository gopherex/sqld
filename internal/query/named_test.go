package query

import (
	"reflect"
	"testing"
)

func TestRewriteNamedParamsBasic(t *testing.T) {
	rewritten, names := rewriteNamedParams("email = @email AND org = @org")
	want := "email = $1 AND org = $2"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "email", 2: "org"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
}

func TestRewriteNamedParamsRepeated(t *testing.T) {
	rewritten, names := rewriteNamedParams("@x OR a = @x")
	want := "$1 OR a = $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "x"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
}

func TestRewriteNamedParamsSkipOperator(t *testing.T) {
	// @> is a PostgreSQL operator (contains); should NOT be rewritten.
	// @t after the operator IS a named param.
	rewritten, names := rewriteNamedParams("tags @> @t")
	want := "tags @> $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "t"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
}

func TestRewriteNamedParamsSkipStringAndComment(t *testing.T) {
	// @b inside single-quoted string, @c inside block comment, @d inside line
	// comment must NOT be rewritten.  Only @real at the end is a param.
	sql := "'a@b' /* @c */ -- @d\n= @real"
	rewritten, names := rewriteNamedParams(sql)
	want := "'a@b' /* @c */ -- @d\n= $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "real"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
}

func TestRewriteNamedParamsNoParams(t *testing.T) {
	sql := "SELECT id FROM users WHERE id = $1"
	rewritten, names := rewriteNamedParams(sql)
	if rewritten != sql {
		t.Errorf("rewritten=%q want unchanged", rewritten)
	}
	if len(names) != 0 {
		t.Errorf("names=%v want empty", names)
	}
}

func TestRewriteNamedParamsDollarQuote(t *testing.T) {
	// @inside inside a dollar-quoted string must be left alone.
	sql := "$fmt$@inside$fmt$ = @outside"
	rewritten, names := rewriteNamedParams(sql)
	want := "$fmt$@inside$fmt$ = $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	wantNames := map[uint32]string{1: "outside"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Errorf("names=%v want=%v", names, wantNames)
	}
}

func TestRewriteNamedParamsDoubleAt(t *testing.T) {
	// @@ is a text-search operator; neither @ starts an identifier.
	rewritten, names := rewriteNamedParams("to_tsvector('english', body) @@ @query")
	want := "to_tsvector('english', body) @@ $1"
	if rewritten != want {
		t.Errorf("rewritten=%q want=%q", rewritten, want)
	}
	if len(names) != 1 || names[1] != "query" {
		t.Errorf("names=%v", names)
	}
}
