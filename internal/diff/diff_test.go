package diff

import (
	"strings"
	"testing"

	"github.com/yaroher/sqld/internal/catalog"
	"github.com/yaroher/sqld/internal/parse"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
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
