package diff

import (
	"strings"
	"testing"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// firstTable returns the first table in the first schema of the catalog built
// from ddl.
func firstTable(t *testing.T, ddl string) *irv1.Table {
	t.Helper()
	c := cat(t, ddl)
	for _, s := range c.GetSchemas() {
		if len(s.GetTables()) > 0 {
			return s.GetTables()[0]
		}
	}
	t.Fatalf("no table built from: %s", ddl)
	return nil
}

func TestQuoteIdent(t *testing.T) {
	if got := quoteIdent("users"); got != `"users"` {
		t.Fatalf("quoteIdent=%q", got)
	}
	if got := quoteIdent(`we"ird`); got != `"we""ird"` {
		t.Fatalf("quoteIdent escape=%q", got)
	}
}

func TestQualified(t *testing.T) {
	if got := qualified("app", "users"); got != `"app"."users"` {
		t.Fatalf("qualified=%q", got)
	}
	if got := qualified("", "users"); got != `"users"` {
		t.Fatalf("qualified no schema=%q", got)
	}
}

func TestRenderColumn(t *testing.T) {
	tbl := firstTable(t, "CREATE TABLE t(email text not null);")
	col := tbl.GetColumns()[0]
	got := renderColumn(col)
	if !strings.Contains(got, `"email"`) {
		t.Fatalf("missing name: %q", got)
	}
	if !strings.Contains(got, "text") {
		t.Fatalf("missing type: %q", got)
	}
	if !strings.Contains(got, "NOT NULL") {
		t.Fatalf("missing NOT NULL: %q", got)
	}
}

func TestRenderColumnNullable(t *testing.T) {
	tbl := firstTable(t, "CREATE TABLE t(id bigint);")
	got := renderColumn(tbl.GetColumns()[0])
	if strings.Contains(got, "NOT NULL") {
		t.Fatalf("unexpected NOT NULL: %q", got)
	}
}

func TestRenderType(t *testing.T) {
	tbl := firstTable(t, "CREATE TABLE t(a numeric(10,2), b varchar(20), c int[]);")
	cols := tbl.GetColumns()
	if got := renderType(cols[0].GetType()); got != "numeric(10,2)" {
		t.Fatalf("numeric=%q", got)
	}
	if got := renderType(cols[1].GetType()); got != "varchar(20)" {
		t.Fatalf("varchar=%q", got)
	}
	if got := renderType(cols[2].GetType()); !strings.HasSuffix(got, "[]") {
		t.Fatalf("array=%q", got)
	}
}

func TestRenderCreateTable(t *testing.T) {
	tbl := firstTable(t, "CREATE TABLE app.users(id bigint primary key, email text not null);")
	got := renderCreateTable(tbl)
	if !strings.HasPrefix(got, "CREATE TABLE ") {
		t.Fatalf("prefix: %q", got)
	}
	if !strings.Contains(got, `"users"`) {
		t.Fatalf("table name: %q", got)
	}
	if !strings.Contains(got, `"id"`) || !strings.Contains(got, `"email"`) {
		t.Fatalf("columns: %q", got)
	}
	if !strings.Contains(got, "PRIMARY KEY") {
		t.Fatalf("missing PRIMARY KEY: %q", got)
	}
	if !strings.HasSuffix(strings.TrimSpace(got), ";") {
		t.Fatalf("missing terminator: %q", got)
	}
}

func TestRenderCreateEnum(t *testing.T) {
	c := cat(t, "CREATE TYPE s AS ENUM ('a','b');")
	e := c.GetSchemas()[0].GetEnums()[0]
	got := renderCreateEnum(e)
	if !strings.Contains(got, "CREATE TYPE") || !strings.Contains(got, "AS ENUM") {
		t.Fatalf("enum: %q", got)
	}
	if !strings.Contains(got, "'a'") || !strings.Contains(got, "'b'") {
		t.Fatalf("labels: %q", got)
	}
}

func TestRenderCreateComposite(t *testing.T) {
	c := cat(t, "CREATE TYPE addr AS (street text, zip varchar(10));")
	cp := c.GetSchemas()[0].GetComposites()[0]
	got := renderCreateComposite(cp)
	if !strings.Contains(got, "CREATE TYPE") || !strings.Contains(got, "AS (") {
		t.Fatalf("composite: %q", got)
	}
	if !strings.Contains(got, `"street"`) || !strings.Contains(got, `"zip"`) {
		t.Fatalf("fields: %q", got)
	}
}

func TestRenderCreateRange(t *testing.T) {
	c := cat(t, "CREATE TYPE fl AS RANGE (subtype = float8);")
	r := c.GetSchemas()[0].GetRanges()[0]
	got := renderCreateRange(r)
	if !strings.Contains(got, "AS RANGE") || !strings.Contains(got, "subtype =") {
		t.Fatalf("range: %q", got)
	}
}

func TestRenderCreateSequence(t *testing.T) {
	c := cat(t, "CREATE SEQUENCE seq1 START 5 INCREMENT 2;")
	q := c.GetSchemas()[0].GetSequences()[0]
	got := renderCreateSequence(q)
	if !strings.HasPrefix(got, "CREATE SEQUENCE") {
		t.Fatalf("sequence: %q", got)
	}
	if !strings.Contains(got, "INCREMENT BY 2") || !strings.Contains(got, "START WITH 5") {
		t.Fatalf("sequence options: %q", got)
	}
}
