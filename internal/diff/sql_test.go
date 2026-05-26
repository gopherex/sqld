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

// firstTableConstraint returns the first non-inline (non-PK/non-NOTNULL)
// constraint across all tables built from ddl, with its owning table.
func firstTableConstraint(t *testing.T, ddl string) (*irv1.Table, *irv1.Constraint) {
	t.Helper()
	c := cat(t, ddl)
	for _, s := range c.GetSchemas() {
		for _, tbl := range s.GetTables() {
			for _, con := range tbl.GetConstraints() {
				switch con.GetType() {
				case irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY,
					irv1.ConstraintType_CONSTRAINT_TYPE_NOT_NULL:
					continue
				}
				return tbl, con
			}
		}
	}
	t.Fatalf("no standalone constraint built from: %s", ddl)
	return nil, nil
}

func TestRenderConstraintFK(t *testing.T) {
	tbl, c := firstTableConstraint(t,
		"CREATE TABLE a(id bigint primary key); "+
			"CREATE TABLE b(id bigint, a_id bigint, CONSTRAINT fk FOREIGN KEY (a_id) REFERENCES a(id) ON DELETE CASCADE);")
	got := renderAddConstraint(tbl, c)
	if !strings.Contains(got, "FOREIGN KEY") || !strings.Contains(got, "REFERENCES") ||
		!strings.Contains(got, "ON DELETE CASCADE") || !strings.Contains(got, "fk") {
		t.Fatalf("fk: %q", got)
	}
}

func TestRenderConstraintCheck(t *testing.T) {
	tbl, c := firstTableConstraint(t, "CREATE TABLE t(id bigint, CONSTRAINT pos CHECK (id > 0));")
	got := renderAddConstraint(tbl, c)
	if !strings.Contains(got, "CHECK") || !strings.Contains(got, "> 0") {
		t.Fatalf("check: %q", got)
	}
}

func TestRenderCreateIndex(t *testing.T) {
	tbl := firstTable(t, "CREATE TABLE t(id bigint, x text); CREATE INDEX idx_t_x ON t(x DESC);")
	idx := tbl.GetIndexes()[0]
	got := renderCreateIndex(idx, tbl)
	if !strings.HasPrefix(got, "CREATE INDEX") || !strings.Contains(got, `"idx_t_x"`) || !strings.Contains(got, `"x" DESC`) {
		t.Fatalf("index: %q", got)
	}
}

func TestRenderCreateView(t *testing.T) {
	c := cat(t, "CREATE TABLE t(id bigint); CREATE VIEW v AS SELECT id FROM t;")
	v := c.GetSchemas()[0].GetViews()[0]
	got := renderCreateView(v, false)
	if !strings.HasPrefix(got, "CREATE VIEW") || !strings.Contains(got, "AS SELECT") {
		t.Fatalf("view: %q", got)
	}
	if !strings.HasPrefix(renderCreateView(v, true), "CREATE OR REPLACE VIEW") {
		t.Fatalf("or replace view: %q", renderCreateView(v, true))
	}
}

func TestRenderCreateFunction(t *testing.T) {
	c := cat(t, "CREATE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a + b $$;")
	fn := c.GetSchemas()[0].GetFunctions()[0]
	got := renderCreateFunction(fn, false)
	if !strings.HasPrefix(got, "CREATE FUNCTION") || !strings.Contains(got, "RETURNS int4") ||
		!strings.Contains(got, "LANGUAGE sql") || !strings.Contains(got, "$$") {
		t.Fatalf("function: %q", got)
	}
}

func TestRenderCreateTrigger(t *testing.T) {
	c := cat(t, "CREATE TABLE t(id bigint); CREATE TRIGGER trg BEFORE INSERT ON t FOR EACH ROW EXECUTE FUNCTION f();")
	tr := c.GetSchemas()[0].GetTriggers()[0]
	got := renderCreateTrigger(tr)
	if !strings.HasPrefix(got, "CREATE TRIGGER") || !strings.Contains(got, "BEFORE INSERT") ||
		!strings.Contains(got, "FOR EACH ROW") || !strings.Contains(got, "EXECUTE FUNCTION") {
		t.Fatalf("trigger: %q", got)
	}
}
