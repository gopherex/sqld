package mapper

import (
	"testing"

	pg "github.com/pganalyze/pg_query_go/v6"
	"github.com/yaroher/sqld/internal/parse"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func createTableFor(t *testing.T, sql string) *pg.CreateStmt {
	t.Helper()
	stmts, err := parse.Statements(sql)
	if err != nil {
		t.Fatal(err)
	}
	return stmts[0].Node.GetCreateStmt()
}

// ---------------------------------------------------------------------------
// TestMapCreateTable
// ---------------------------------------------------------------------------

func TestMapCreateTable(t *testing.T) {
	tbl := MapCreateTable(createTableFor(t,
		`CREATE TABLE users(
			id bigserial primary key,
			email text not null unique,
			org_id bigint references orgs(id) on delete cascade
		)`))
	if tbl == nil {
		t.Fatal("MapCreateTable returned nil")
	}
	if tbl.GetName().GetName() != "users" {
		t.Fatalf("name=%v", tbl.GetName())
	}
	if len(tbl.GetColumns()) != 3 {
		t.Fatalf("cols=%d", len(tbl.GetColumns()))
	}

	// email NOT NULL
	var email *irv1.Column
	for _, c := range tbl.GetColumns() {
		if c.GetName() == "email" {
			email = c
		}
	}
	if email == nil || email.GetNullable() {
		t.Fatalf("email nullable wrong: %+v", email)
	}

	// a FK constraint to orgs with CASCADE exists
	var fk *irv1.ForeignKey
	for _, c := range tbl.GetConstraints() {
		if c.GetForeignKey() != nil {
			fk = c.GetForeignKey()
		}
	}
	if fk == nil {
		t.Fatal("no FK")
	}
	if fk.GetReferencedTable().GetName().GetName() != "orgs" {
		t.Fatalf("fk ref=%v", fk.GetReferencedTable())
	}
	if fk.GetOnDelete() != irv1.ReferentialAction_REFERENTIAL_ACTION_CASCADE {
		t.Fatalf("on delete=%v", fk.GetOnDelete())
	}
}

// TestMapCreateTable_Persistence checks that UNLOGGED/TEMP are mapped.
func TestMapCreateTable_Persistence(t *testing.T) {
	cases := []struct {
		sql  string
		want irv1.TablePersistence
	}{
		{"CREATE TABLE t(id int)", irv1.TablePersistence_TABLE_PERSISTENCE_PERMANENT},
		{"CREATE UNLOGGED TABLE t(id int)", irv1.TablePersistence_TABLE_PERSISTENCE_UNLOGGED},
		{"CREATE TEMPORARY TABLE t(id int)", irv1.TablePersistence_TABLE_PERSISTENCE_TEMPORARY},
	}
	for _, c := range cases {
		tbl := MapCreateTable(createTableFor(t, c.sql))
		if tbl == nil {
			t.Fatalf("%s: nil", c.sql)
		}
		if tbl.GetPersistence() != c.want {
			t.Errorf("%s: persistence=%v want=%v", c.sql, tbl.GetPersistence(), c.want)
		}
	}
}

// TestMapCreateTable_PK checks that a PK constraint is created.
func TestMapCreateTable_PK(t *testing.T) {
	tbl := MapCreateTable(createTableFor(t,
		`CREATE TABLE orders(
			id serial,
			CONSTRAINT orders_pkey PRIMARY KEY(id)
		)`))
	var pk *irv1.PrimaryKey
	for _, c := range tbl.GetConstraints() {
		if c.GetPrimaryKey() != nil {
			pk = c.GetPrimaryKey()
		}
	}
	if pk == nil {
		t.Fatal("no PK constraint found")
	}
	if len(pk.GetColumns()) == 0 {
		t.Fatal("PK has no columns")
	}
}

// TestMapCreateTable_UniqueConstraint checks table-level UNIQUE.
func TestMapCreateTable_UniqueConstraint(t *testing.T) {
	tbl := MapCreateTable(createTableFor(t,
		`CREATE TABLE t(a int, b int, UNIQUE(a,b))`))
	var uc *irv1.UniqueConstraint
	for _, c := range tbl.GetConstraints() {
		if c.GetUnique() != nil {
			uc = c.GetUnique()
		}
	}
	if uc == nil {
		t.Fatal("no unique constraint")
	}
	if len(uc.GetColumns()) != 2 {
		t.Fatalf("unique cols=%d", len(uc.GetColumns()))
	}
}

// TestMapCreateTable_CheckConstraint checks CHECK constraint.
func TestMapCreateTable_CheckConstraint(t *testing.T) {
	tbl := MapCreateTable(createTableFor(t,
		`CREATE TABLE t(score int CHECK(score > 0))`))
	var cc *irv1.CheckConstraint
	for _, c := range tbl.GetConstraints() {
		if c.GetCheck() != nil {
			cc = c.GetCheck()
		}
	}
	if cc == nil {
		t.Fatal("no check constraint")
	}
	if cc.GetExpression() == nil {
		t.Fatal("check expression is nil")
	}
}

// TestMapCreateTable_ColumnPosition checks 1-based positions.
func TestMapCreateTable_ColumnPosition(t *testing.T) {
	tbl := MapCreateTable(createTableFor(t, `CREATE TABLE t(a int, b text, c bool)`))
	for i, col := range tbl.GetColumns() {
		want := uint32(i + 1)
		if col.GetPosition() != want {
			t.Errorf("col %d position=%d want %d", i, col.GetPosition(), want)
		}
	}
}

// ---------------------------------------------------------------------------
// TestMapCreateEnum
// ---------------------------------------------------------------------------

func TestMapCreateEnum(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TYPE status AS ENUM ('a','b')")
	et := MapCreateEnum(stmts[0].Node.GetCreateEnumStmt())
	if et == nil {
		t.Fatal("MapCreateEnum returned nil")
	}
	if et.GetName().GetName() != "status" || len(et.GetLabels()) != 2 {
		t.Fatalf("enum=%+v", et)
	}
}

// ---------------------------------------------------------------------------
// TestMapIndex
// ---------------------------------------------------------------------------

func TestMapIndex(t *testing.T) {
	stmts, err := parse.Statements(
		`CREATE UNIQUE INDEX idx_users_email ON users(email) WHERE active = true`)
	if err != nil {
		t.Fatal(err)
	}
	idx := MapIndex(stmts[0].Node.GetIndexStmt())
	if idx == nil {
		t.Fatal("MapIndex returned nil")
	}
	if idx.GetName() != "idx_users_email" {
		t.Fatalf("name=%q", idx.GetName())
	}
	if !idx.GetUnique() {
		t.Fatal("not unique")
	}
	if len(idx.GetElements()) != 1 || idx.GetElements()[0].GetColumn() != "email" {
		t.Fatalf("elements=%v", idx.GetElements())
	}
	if idx.GetPredicate() == nil {
		t.Fatal("predicate nil")
	}
}

// TestMapIndex_ExprElement checks expression-based index elements.
func TestMapIndex_ExprElement(t *testing.T) {
	stmts, err := parse.Statements(`CREATE INDEX idx_lower ON t(lower(email))`)
	if err != nil {
		t.Fatal(err)
	}
	idx := MapIndex(stmts[0].Node.GetIndexStmt())
	if idx == nil {
		t.Fatal("nil")
	}
	if len(idx.GetElements()) != 1 {
		t.Fatalf("elements=%d", len(idx.GetElements()))
	}
	if idx.GetElements()[0].GetExpr() == nil {
		t.Fatalf("expected expr element, got column=%q", idx.GetElements()[0].GetColumn())
	}
}

// ---------------------------------------------------------------------------
// TestMapView
// ---------------------------------------------------------------------------

func TestMapView(t *testing.T) {
	stmts, err := parse.Statements(`CREATE VIEW active_users AS SELECT id, email FROM users WHERE active = true`)
	if err != nil {
		t.Fatal(err)
	}
	v := MapView(stmts[0].Node.GetViewStmt())
	if v == nil {
		t.Fatal("MapView returned nil")
	}
	if v.GetName().GetName() != "active_users" {
		t.Fatalf("name=%q", v.GetName().GetName())
	}
	if v.GetQuery() == nil {
		t.Fatal("query nil")
	}
}

// ---------------------------------------------------------------------------
// TestMapCreateSequence
// ---------------------------------------------------------------------------

func TestMapCreateSequence(t *testing.T) {
	stmts, err := parse.Statements(`CREATE SEQUENCE my_seq START 10 INCREMENT 2`)
	if err != nil {
		t.Fatal(err)
	}
	seq := MapCreateSequence(stmts[0].Node.GetCreateSeqStmt())
	if seq == nil {
		t.Fatal("MapCreateSequence returned nil")
	}
	if seq.GetName().GetName() != "my_seq" {
		t.Fatalf("name=%q", seq.GetName().GetName())
	}
	if seq.GetStart() != 10 {
		t.Fatalf("start=%d", seq.GetStart())
	}
	if seq.GetIncrement() != 2 {
		t.Fatalf("increment=%d", seq.GetIncrement())
	}
}

// ---------------------------------------------------------------------------
// TestMapConstraint_NilInput
// ---------------------------------------------------------------------------

// Nil inputs must not panic.
func TestMapDDL_NilInputs(t *testing.T) {
	if MapCreateTable(nil) != nil {
		t.Error("expected nil for nil CreateStmt")
	}
	if MapIndex(nil) != nil {
		t.Error("expected nil for nil IndexStmt")
	}
	if MapView(nil) != nil {
		t.Error("expected nil for nil ViewStmt")
	}
	if MapCreateEnum(nil) != nil {
		t.Error("expected nil for nil CreateEnumStmt")
	}
	if MapCreateSequence(nil) != nil {
		t.Error("expected nil for nil CreateSeqStmt")
	}
}
