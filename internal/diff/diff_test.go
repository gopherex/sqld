package diff

import (
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

// cat builds an IR catalog from DDL using parse + catalog.Build.
func cat(t *testing.T, ddl string) *irv1.Catalog {
	t.Helper()
	stmts, err := parse.Statements(ddl)
	if err != nil {
		t.Fatal(err)
	}
	c, _ := catalog.Build(stmts)
	return c
}

func TestDiffEmpty(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint);")
	to := cat(t, "CREATE TABLE t(id bigint);")
	p, err := Diff(from, to)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Empty() {
		t.Fatalf("expected empty plan, got:\n%s", p.UpSQL())
	}
}

func TestDiffCreateTable(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE SCHEMA app; CREATE TABLE app.users(id bigint primary key, email text not null);")
	p, err := Diff(from, to)
	if err != nil {
		t.Fatal(err)
	}
	up := p.UpSQL()
	if !strings.Contains(up, "CREATE SCHEMA") || !strings.Contains(up, "CREATE TABLE") {
		t.Fatalf("up:\n%s", up)
	}
	if strings.Index(up, "CREATE SCHEMA") > strings.Index(up, "CREATE TABLE") {
		t.Fatal("schema must come before table")
	}
	if !strings.Contains(p.DownSQL(), "DROP TABLE") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffDownTeardownOrder(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE SCHEMA app; CREATE TABLE app.users(id bigint primary key);")
	p, _ := Diff(from, to)
	down := p.DownSQL()
	// On rollback the table must be dropped before the schema it lives in.
	if strings.Index(down, "DROP TABLE") > strings.Index(down, "DROP SCHEMA") {
		t.Fatalf("table must be dropped before schema:\n%s", down)
	}
}

func TestDiffDropTable(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint, email text not null);")
	to := cat(t, "")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "DROP TABLE") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	down := p.DownSQL()
	if !strings.Contains(down, "CREATE TABLE") {
		t.Fatalf("down should re-create:\n%s", down)
	}
	if !strings.Contains(down, "WARNING: data loss") {
		t.Fatalf("down should warn about data loss:\n%s", down)
	}
}

func TestDiffDropSchemaNotPublic(t *testing.T) {
	from := cat(t, "CREATE SCHEMA app;")
	to := cat(t, "")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "DROP SCHEMA") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	// public must never be emitted.
	if strings.Contains(p.UpSQL(), `"public"`) {
		t.Fatalf("public should not appear:\n%s", p.UpSQL())
	}
}

func TestDiffAddColumn(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint);")
	to := cat(t, "CREATE TABLE t(id bigint, email text not null);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "ADD COLUMN") || !strings.Contains(p.UpSQL(), "email") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "DROP COLUMN") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffDropColumn(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint, email text);")
	to := cat(t, "CREATE TABLE t(id bigint);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "DROP COLUMN") || !strings.Contains(p.UpSQL(), "email") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "ADD COLUMN") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
	if !strings.Contains(p.DownSQL(), "WARNING: data loss") {
		t.Fatalf("down should warn:\n%s", p.DownSQL())
	}
}

func TestDiffAlterColumnType(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id int);")
	to := cat(t, "CREATE TABLE t(id bigint);")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "ALTER COLUMN") || !strings.Contains(up, "TYPE") {
		t.Fatalf("up:\n%s", up)
	}
	if !strings.Contains(up, "USING") {
		t.Fatalf("up should contain USING cast:\n%s", up)
	}
	// Down reverses the type (int8 -> int4).
	if !strings.Contains(p.DownSQL(), "TYPE") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffSetNotNull(t *testing.T) {
	from := cat(t, "CREATE TABLE t(email text);")
	to := cat(t, "CREATE TABLE t(email text not null);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "SET NOT NULL") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "DROP NOT NULL") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffDropNotNull(t *testing.T) {
	from := cat(t, "CREATE TABLE t(email text not null);")
	to := cat(t, "CREATE TABLE t(email text);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "DROP NOT NULL") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "SET NOT NULL") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffCreateEnum(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE TYPE s AS ENUM ('a','b');")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "CREATE TYPE") || !strings.Contains(p.UpSQL(), "AS ENUM") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "DROP TYPE") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffAddEnumValue(t *testing.T) {
	from := cat(t, "CREATE TYPE s AS ENUM ('a');")
	to := cat(t, "CREATE TYPE s AS ENUM ('a','b');")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "ADD VALUE") || !strings.Contains(p.UpSQL(), "'b'") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
}

func TestDiffRemoveEnumValueWarns(t *testing.T) {
	from := cat(t, "CREATE TYPE s AS ENUM ('a','b');")
	to := cat(t, "CREATE TYPE s AS ENUM ('a');")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "WARNING") {
		t.Fatalf("expected warning for removed enum value:\n%s", p.UpSQL())
	}
}

func TestDiffCreateDomain(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE DOMAIN pos AS integer NOT NULL;")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "CREATE DOMAIN") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "DROP TYPE") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffCreateComposite(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE TYPE addr AS (street text, zip varchar(10));")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "CREATE TYPE") || !strings.Contains(p.UpSQL(), "AS (") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
}

func TestDiffCreateRange(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE TYPE fl AS RANGE (subtype = float8);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "AS RANGE") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
}

func TestDiffCreateSequence(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE SEQUENCE seq1 START 5;")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "CREATE SEQUENCE") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "DROP SEQUENCE") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffTypeBeforeTableOrdering(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE TYPE st AS ENUM ('x'); CREATE TABLE t(id bigint, s st);")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if strings.Index(up, "CREATE TYPE") > strings.Index(up, "CREATE TABLE") {
		t.Fatalf("type must come before table:\n%s", up)
	}
}

// --- constraints -------------------------------------------------------------

func TestDiffAddFK(t *testing.T) {
	from := cat(t, "CREATE TABLE a(id bigint primary key); CREATE TABLE b(id bigint primary key);")
	to := cat(t, "CREATE TABLE a(id bigint primary key); CREATE TABLE b(id bigint primary key, a_id bigint references a(id) on delete cascade);")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "FOREIGN KEY") || !strings.Contains(up, "REFERENCES") || !strings.Contains(up, "CASCADE") {
		t.Fatalf("up:\n%s", up)
	}
}

func TestDiffAddFKNewTablesOrdering(t *testing.T) {
	// Two new tables with an FK: the FK must be emitted after both CREATE TABLEs.
	from := cat(t, "")
	to := cat(t, "CREATE TABLE a(id bigint primary key); CREATE TABLE b(id bigint primary key, a_id bigint references a(id));")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	fkIdx := strings.Index(up, "FOREIGN KEY")
	if fkIdx < 0 {
		t.Fatalf("expected FOREIGN KEY in:\n%s", up)
	}
	if fkIdx < strings.LastIndex(up, "CREATE TABLE") {
		t.Fatalf("FK must come after all CREATE TABLEs:\n%s", up)
	}
}

func TestDiffAddUniqueConstraint(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint, email text);")
	to := cat(t, "CREATE TABLE t(id bigint, email text, CONSTRAINT uq_email UNIQUE(email));")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "ADD CONSTRAINT") || !strings.Contains(up, "UNIQUE") || !strings.Contains(up, "uq_email") {
		t.Fatalf("up:\n%s", up)
	}
	if !strings.Contains(p.DownSQL(), "DROP CONSTRAINT") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffAddCheckConstraint(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint);")
	to := cat(t, "CREATE TABLE t(id bigint, CONSTRAINT pos CHECK (id > 0));")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "CHECK") || !strings.Contains(up, "> 0") {
		t.Fatalf("up should render the check expression:\n%s", up)
	}
}

func TestDiffDropConstraint(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint, email text, CONSTRAINT uq UNIQUE(email));")
	to := cat(t, "CREATE TABLE t(id bigint, email text);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "DROP CONSTRAINT") || !strings.Contains(p.UpSQL(), "uq") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "ADD CONSTRAINT") {
		t.Fatalf("down should re-add:\n%s", p.DownSQL())
	}
}

// --- indexes -----------------------------------------------------------------

func TestDiffAddIndex(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint, x text);")
	to := cat(t, "CREATE TABLE t(id bigint, x text); CREATE INDEX idx_t_x ON t(x);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "CREATE INDEX") || !strings.Contains(p.UpSQL(), "idx_t_x") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "DROP INDEX") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffDropIndex(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint, x text); CREATE INDEX idx_t_x ON t(x);")
	to := cat(t, "CREATE TABLE t(id bigint, x text);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "DROP INDEX") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "CREATE INDEX") {
		t.Fatalf("down should re-create:\n%s", p.DownSQL())
	}
}

func TestDiffUniqueIndex(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint, x text);")
	to := cat(t, "CREATE TABLE t(id bigint, x text); CREATE UNIQUE INDEX idx_t_x ON t(x);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "CREATE UNIQUE INDEX") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
}

// --- views -------------------------------------------------------------------

func TestDiffAddView(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint);")
	to := cat(t, "CREATE TABLE t(id bigint); CREATE VIEW v AS SELECT id FROM t;")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "CREATE") || !strings.Contains(up, "VIEW") {
		t.Fatalf("up:\n%s", up)
	}
	if !strings.Contains(p.DownSQL(), "DROP VIEW") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffReplaceView(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint, x text); CREATE VIEW v AS SELECT id FROM t;")
	to := cat(t, "CREATE TABLE t(id bigint, x text); CREATE VIEW v AS SELECT id, x FROM t;")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "CREATE OR REPLACE VIEW") {
		t.Fatalf("up should replace the view:\n%s", up)
	}
}

func TestDiffDropView(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint); CREATE VIEW v AS SELECT id FROM t;")
	to := cat(t, "CREATE TABLE t(id bigint);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "DROP VIEW") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "VIEW") {
		t.Fatalf("down should re-create the view:\n%s", p.DownSQL())
	}
}

func TestDiffViewAfterTableOrdering(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE TABLE t(id bigint); CREATE VIEW v AS SELECT id FROM t;")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if strings.Index(up, "CREATE TABLE") > strings.Index(up, "VIEW") {
		t.Fatalf("table must come before view:\n%s", up)
	}
}

func TestDiffAddMatView(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint);")
	to := cat(t, "CREATE TABLE t(id bigint); CREATE MATERIALIZED VIEW mv AS SELECT id FROM t;")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "CREATE MATERIALIZED VIEW") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "DROP MATERIALIZED VIEW") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

// --- functions and procedures ------------------------------------------------

func TestDiffAddFunction(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE FUNCTION add(a int, b int) RETURNS int LANGUAGE sql AS $$ SELECT a + b $$;")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "CREATE FUNCTION") || !strings.Contains(up, "RETURNS int4") || !strings.Contains(up, "LANGUAGE sql") {
		t.Fatalf("up:\n%s", up)
	}
	if !strings.Contains(p.DownSQL(), "DROP FUNCTION") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffReplaceFunction(t *testing.T) {
	from := cat(t, "CREATE FUNCTION f(a int) RETURNS int LANGUAGE sql AS $$ SELECT a $$;")
	to := cat(t, "CREATE FUNCTION f(a int) RETURNS int LANGUAGE sql AS $$ SELECT a + 1 $$;")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "CREATE OR REPLACE FUNCTION") {
		t.Fatalf("up should replace the function:\n%s", p.UpSQL())
	}
}

func TestDiffDropFunction(t *testing.T) {
	from := cat(t, "CREATE FUNCTION f(a int) RETURNS int LANGUAGE sql AS $$ SELECT a $$;")
	to := cat(t, "")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "DROP FUNCTION") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "CREATE FUNCTION") {
		t.Fatalf("down should re-create:\n%s", p.DownSQL())
	}
}

func TestDiffFunctionOverloadDistinct(t *testing.T) {
	// Two overloads of the same name must be treated as distinct objects.
	from := cat(t, "CREATE FUNCTION f(a int) RETURNS int LANGUAGE sql AS $$ SELECT a $$;")
	to := cat(t, "CREATE FUNCTION f(a int) RETURNS int LANGUAGE sql AS $$ SELECT a $$;"+
		"CREATE FUNCTION f(a text) RETURNS text LANGUAGE sql AS $$ SELECT a $$;")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "CREATE FUNCTION") || strings.Contains(up, "OR REPLACE") {
		t.Fatalf("adding an overload should be a plain CREATE, got:\n%s", up)
	}
}

func TestDiffAddProcedure(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE PROCEDURE pr(a int) LANGUAGE plpgsql AS $$ BEGIN END $$;")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "CREATE PROCEDURE") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "DROP PROCEDURE") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

// --- triggers ----------------------------------------------------------------

func TestDiffAddTrigger(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint);")
	to := cat(t, "CREATE TABLE t(id bigint); CREATE TRIGGER trg BEFORE INSERT ON t FOR EACH ROW EXECUTE FUNCTION f();")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "CREATE TRIGGER") || !strings.Contains(up, "BEFORE INSERT") || !strings.Contains(up, "EXECUTE FUNCTION") {
		t.Fatalf("up:\n%s", up)
	}
	if !strings.Contains(p.DownSQL(), "DROP TRIGGER") {
		t.Fatalf("down:\n%s", p.DownSQL())
	}
}

func TestDiffDropTrigger(t *testing.T) {
	from := cat(t, "CREATE TABLE t(id bigint); CREATE TRIGGER trg AFTER UPDATE ON t FOR EACH ROW EXECUTE FUNCTION f();")
	to := cat(t, "CREATE TABLE t(id bigint);")
	p, _ := Diff(from, to)
	if !strings.Contains(p.UpSQL(), "DROP TRIGGER") {
		t.Fatalf("up:\n%s", p.UpSQL())
	}
	if !strings.Contains(p.DownSQL(), "CREATE TRIGGER") {
		t.Fatalf("down should re-create:\n%s", p.DownSQL())
	}
}

func TestDiffTriggerAfterFunctionOrdering(t *testing.T) {
	from := cat(t, "")
	to := cat(t, "CREATE TABLE t(id bigint);"+
		"CREATE FUNCTION trg_fn() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;"+
		"CREATE TRIGGER trg BEFORE INSERT ON t FOR EACH ROW EXECUTE FUNCTION trg_fn();")
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if strings.Index(up, "CREATE FUNCTION") > strings.Index(up, "CREATE TRIGGER") {
		t.Fatalf("function must come before trigger:\n%s", up)
	}
	if strings.Index(up, "CREATE TABLE") > strings.Index(up, "CREATE TRIGGER") {
		t.Fatalf("table must come before trigger:\n%s", up)
	}
}

// --- raw_sql view path (introspection-style) ---------------------------------

func TestDiffViewPrefersRawSQL(t *testing.T) {
	// Simulate an introspection-sourced catalog where the view query carries
	// raw_sql verbatim (the parser leaves it empty and builds a structured tree).
	from := &irv1.Catalog{}
	to := &irv1.Catalog{Schemas: []*irv1.Schema{{
		Name: "public",
		Views: []*irv1.View{{
			Id:    "public.v",
			Name:  &irv1.QualifiedName{Schema: "public", Name: "v"},
			Query: &irv1.SelectStmt{RawSql: "SELECT 1 AS one"},
		}},
	}}}
	p, _ := Diff(from, to)
	up := p.UpSQL()
	if !strings.Contains(up, "SELECT 1 AS one") {
		t.Fatalf("up should use the raw_sql view body:\n%s", up)
	}
}
