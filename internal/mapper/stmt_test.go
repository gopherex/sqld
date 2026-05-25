package mapper

import (
	"testing"

	"github.com/yaroher/sqld/internal/nodeid"
	"github.com/yaroher/sqld/internal/parse"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// stmtFor parses the given SQL, maps the first statement, and returns it.
func stmtFor(t *testing.T, sql string) *irv1.Statement {
	t.Helper()
	stmts, err := parse.Statements(sql)
	if err != nil {
		t.Fatal(err)
	}
	return MapStatement(stmts[0].Node, nodeid.New("stmt0"))
}

func TestMapSelectBasic(t *testing.T) {
	s := stmtFor(t, "SELECT id, email FROM users WHERE id = $1")
	sel := s.GetSelect().GetSelect()
	if sel == nil {
		t.Fatal("want SimpleSelect")
	}
	if len(sel.GetTargets()) != 2 {
		t.Fatalf("targets=%d", len(sel.GetTargets()))
	}
	if sel.GetFrom()[0].GetTable().GetName().GetName() != "users" {
		t.Fatal("from users")
	}
	if sel.GetWhere().GetOperator().GetSymbol() != "=" {
		t.Fatalf("where op = %q, want =", sel.GetWhere().GetOperator().GetSymbol())
	}
}

func TestMapInsert(t *testing.T) {
	s := stmtFor(t, "INSERT INTO t(a,b) VALUES ($1,$2)")
	ins := s.GetInsert()
	if ins == nil || ins.GetTableName().GetName() != "t" {
		t.Fatalf("ins=%+v", ins)
	}
	if len(ins.GetColumns()) != 2 {
		t.Fatalf("cols=%d", len(ins.GetColumns()))
	}
	if ins.GetValues() == nil || len(ins.GetValues().GetRows()) != 1 {
		t.Fatal("values")
	}
}

func TestMapTruncateRaw(t *testing.T) {
	s := stmtFor(t, "TRUNCATE t")
	if s.GetRaw() == nil || s.GetRaw().GetKind() != irv1.StatementKind_STATEMENT_KIND_TRUNCATE {
		t.Fatalf("raw=%+v", s.GetRaw())
	}
}

func TestMapJoin(t *testing.T) {
	s := stmtFor(t, "SELECT a.id FROM a JOIN b ON a.id = b.a_id")
	from := s.GetSelect().GetSelect().GetFrom()
	if len(from) != 1 || from[0].GetJoin() == nil {
		t.Fatalf("want join, got %+v", from)
	}
}

func TestMapUpdate(t *testing.T) {
	s := stmtFor(t, "UPDATE users SET email = $1 WHERE id = $2")
	upd := s.GetUpdate()
	if upd == nil {
		t.Fatal("want UpdateStmt")
	}
	if upd.GetTableName().GetName() != "users" {
		t.Fatalf("table=%q, want users", upd.GetTableName().GetName())
	}
	if len(upd.GetSet()) != 1 {
		t.Fatalf("set len=%d", len(upd.GetSet()))
	}
	if upd.GetSet()[0].GetColumns()[0] != "email" {
		t.Fatalf("set col=%q", upd.GetSet()[0].GetColumns()[0])
	}
	if upd.GetWhere() == nil {
		t.Fatal("want WHERE clause")
	}
}

func TestMapDelete(t *testing.T) {
	s := stmtFor(t, "DELETE FROM users WHERE id = $1")
	del := s.GetDelete()
	if del == nil {
		t.Fatal("want DeleteStmt")
	}
	if del.GetTableName().GetName() != "users" {
		t.Fatalf("table=%q", del.GetTableName().GetName())
	}
	if del.GetWhere() == nil {
		t.Fatal("want WHERE")
	}
}

func TestMapDDLRaw(t *testing.T) {
	s := stmtFor(t, "CREATE TABLE t (id int)")
	if s.GetRaw() == nil || s.GetRaw().GetKind() != irv1.StatementKind_STATEMENT_KIND_DDL {
		t.Fatalf("raw=%+v", s.GetRaw())
	}
}

func TestMapSelectWithSubquery(t *testing.T) {
	s := stmtFor(t, "SELECT * FROM users WHERE id IN (SELECT user_id FROM orders)")
	sel := s.GetSelect().GetSelect()
	if sel == nil {
		t.Fatal("want SimpleSelect")
	}
	where := sel.GetWhere()
	// SubLink IN should map to Subquery in expression
	if where == nil {
		t.Fatal("want WHERE clause")
	}
	// The IN subquery is an Expr_Subquery with kind IN
	sub := where.GetSubquery()
	if sub == nil {
		t.Fatalf("want subquery in where, got %T / %+v", where.GetNode(), where)
	}
	if sub.GetKind() != irv1.SubqueryKind_SUBQUERY_KIND_IN {
		t.Fatalf("kind=%v", sub.GetKind())
	}
}

func TestMapSelectUnion(t *testing.T) {
	s := stmtFor(t, "SELECT 1 UNION ALL SELECT 2")
	setOp := s.GetSelect().GetSetOperation()
	if setOp == nil {
		t.Fatal("want SetOperation")
	}
	if setOp.GetKind() != irv1.SetOpKind_SET_OP_KIND_UNION {
		t.Fatalf("kind=%v", setOp.GetKind())
	}
	if !setOp.GetAll() {
		t.Fatal("want ALL")
	}
}

func TestMapInsertReturning(t *testing.T) {
	s := stmtFor(t, "INSERT INTO t (a) VALUES ($1) RETURNING id")
	ins := s.GetInsert()
	if ins == nil {
		t.Fatal("want InsertStmt")
	}
	if len(ins.GetReturning()) != 1 {
		t.Fatalf("returning len=%d", len(ins.GetReturning()))
	}
}

func TestMapSubselectFrom(t *testing.T) {
	s := stmtFor(t, "SELECT x FROM (SELECT 1 AS x) sub")
	sel := s.GetSelect().GetSelect()
	if sel == nil {
		t.Fatal("want SimpleSelect")
	}
	from := sel.GetFrom()
	if len(from) != 1 {
		t.Fatalf("from len=%d", len(from))
	}
	sub := from[0].GetSubquery()
	if sub == nil {
		t.Fatalf("want SubqueryRef in from, got %+v", from[0])
	}
	if sub.GetAlias() != "sub" {
		t.Fatalf("alias=%q", sub.GetAlias())
	}
}

func TestMapNodeIdsPresent(t *testing.T) {
	s := stmtFor(t, "SELECT id FROM users")
	if s.GetNodeId() == "" {
		t.Fatal("stmt node_id empty")
	}
	sel := s.GetSelect()
	if sel.GetNodeId() == "" {
		t.Fatal("select node_id empty")
	}
	ss := sel.GetSelect()
	if ss.GetNodeId() == "" {
		t.Fatal("simpleselect node_id empty")
	}
}
