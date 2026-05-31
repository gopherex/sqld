package mapper

import (
	"testing"

	nodeid "github.com/gopherex/sqld/internal/nodeid"
	"github.com/gopherex/sqld/internal/parse"
	pg "github.com/pganalyze/pg_query_go/v6"
)

func exprFor(t *testing.T, sql string) *pg.Node {
	t.Helper()
	stmts, err := parse.Statements("SELECT " + sql)
	if err != nil {
		t.Fatal(err)
	}
	return stmts[0].Node.GetSelectStmt().GetTargetList()[0].GetResTarget().GetVal()
}

func TestMapExprColumnEqParam(t *testing.T) {
	e := MapExpr(exprFor(t, "a = $1"), nodeid.New("n"))
	op := e.GetOperator()
	if op == nil || op.GetSymbol() != "=" {
		t.Fatalf("op=%+v", op)
	}
	if len(op.GetOperands()) != 2 {
		t.Fatalf("operands=%d", len(op.GetOperands()))
	}
	if op.GetOperands()[0].GetColumnRef().GetColumn() != "a" {
		t.Fatalf("lhs column = %q, want a", op.GetOperands()[0].GetColumnRef().GetColumn())
	}
	if op.GetOperands()[1].GetParameter().GetPosition() != 1 {
		t.Fatalf("rhs param position = %d, want 1", op.GetOperands()[1].GetParameter().GetPosition())
	}
	if e.GetNodeId() == "" {
		t.Fatalf("node_id empty")
	}
}

func TestMapExprCountStar(t *testing.T) {
	e := MapExpr(exprFor(t, "count(*)"), nodeid.New("n"))
	fc := e.GetFunctionCall()
	if fc == nil || fc.GetName().GetName() != "count" {
		t.Fatalf("func=%+v", fc)
	}
}

func TestMapExprIsNull(t *testing.T) {
	e := MapExpr(exprFor(t, "x IS NULL"), nodeid.New("n"))
	if e.GetOperator().GetSymbol() != "IS NULL" {
		t.Fatalf("op=%+v", e.GetOperator())
	}
}

func TestMapExprStringLiteral(t *testing.T) {
	e := MapExpr(exprFor(t, "'hi'"), nodeid.New("n"))
	if e.GetLiteral().GetStringValue() != "hi" {
		t.Fatalf("lit=%+v", e.GetLiteral())
	}
}

func TestMapExprIntLiteral(t *testing.T) {
	e := MapExpr(exprFor(t, "42"), nodeid.New("n"))
	if e.GetLiteral().GetIntValue() != 42 {
		t.Fatalf("lit=%+v", e.GetLiteral())
	}
}

func TestMapExprNullLiteral(t *testing.T) {
	e := MapExpr(exprFor(t, "NULL"), nodeid.New("n"))
	if !e.GetLiteral().GetNullValue() {
		t.Fatalf("expected null literal, got=%+v", e.GetLiteral())
	}
}

func TestMapExprQualifiedColumn(t *testing.T) {
	e := MapExpr(exprFor(t, "t.col"), nodeid.New("n"))
	cr := e.GetColumnRef()
	if cr == nil {
		t.Fatalf("expected ColumnRef, got %+v", e)
	}
	if cr.GetQualifier() != "t" {
		t.Fatalf("qualifier = %q, want t", cr.GetQualifier())
	}
	if cr.GetColumn() != "col" {
		t.Fatalf("column = %q, want col", cr.GetColumn())
	}
}

func TestMapExprStar(t *testing.T) {
	e := MapExpr(exprFor(t, "*"), nodeid.New("n"))
	if e.GetStar() == nil {
		t.Fatalf("expected StarExpr, got %+v", e)
	}
}

func TestMapExprQualifiedStar(t *testing.T) {
	e := MapExpr(exprFor(t, "t.*"), nodeid.New("n"))
	star := e.GetStar()
	if star == nil {
		t.Fatalf("expected StarExpr, got %+v", e)
	}
	if star.GetQualifier() != "t" {
		t.Fatalf("qualifier = %q, want t", star.GetQualifier())
	}
}

func TestMapExprBoolAnd(t *testing.T) {
	e := MapExpr(exprFor(t, "a = 1 AND b = 2"), nodeid.New("n"))
	op := e.GetOperator()
	if op == nil || op.GetSymbol() != "AND" {
		t.Fatalf("op=%+v", op)
	}
	if len(op.GetOperands()) != 2 {
		t.Fatalf("operands count = %d, want 2", len(op.GetOperands()))
	}
}

func TestMapExprIsNotNull(t *testing.T) {
	e := MapExpr(exprFor(t, "x IS NOT NULL"), nodeid.New("n"))
	op := e.GetOperator()
	if op == nil || op.GetSymbol() != "IS NOT NULL" {
		t.Fatalf("op=%+v", op)
	}
}

func TestMapExprTypeCast(t *testing.T) {
	e := MapExpr(exprFor(t, "'2020-01-01'::date"), nodeid.New("n"))
	cast := e.GetCast()
	if cast == nil {
		t.Fatalf("expected CastExpr, got %+v", e)
	}
	if cast.GetTargetType().GetPgName() != "date" {
		t.Fatalf("target type = %q, want date", cast.GetTargetType().GetPgName())
	}
}

func TestMapExprCaseExpr(t *testing.T) {
	e := MapExpr(exprFor(t, "CASE WHEN a = 1 THEN 'yes' ELSE 'no' END"), nodeid.New("n"))
	ce := e.GetCaseExpr()
	if ce == nil {
		t.Fatalf("expected CaseExpr, got %+v", e)
	}
	if len(ce.GetWhens()) != 1 {
		t.Fatalf("whens count = %d, want 1", len(ce.GetWhens()))
	}
	if ce.GetElseResult() == nil {
		t.Fatalf("expected ElseResult")
	}
}

func TestMapExprNodeIdSet(t *testing.T) {
	e := MapExpr(exprFor(t, "a"), nodeid.New("root"))
	if e.GetNodeId() == "" {
		t.Fatalf("node_id empty")
	}
	if e.GetNodeId() != "root" {
		t.Fatalf("node_id = %q, want root", e.GetNodeId())
	}
}

func TestMapExprSubLinkRawSql(t *testing.T) {
	e := MapExpr(exprFor(t, "(SELECT 1)"), nodeid.New("n"))
	// SubLink should produce a raw_sql fallback
	if e.GetRawSql() == "" && e.GetSubquery() == nil {
		t.Fatalf("expected raw_sql or subquery for sublink, got %+v", e)
	}
	if e.GetNodeId() == "" {
		t.Fatalf("node_id empty")
	}
}

func TestMapExprNilNoPanic(t *testing.T) {
	// nil node must not panic; returns raw_sql fallback
	e := MapExpr(nil, nodeid.New("n"))
	if e == nil {
		t.Fatalf("expected non-nil Expr for nil input")
	}
}

func TestMapExprFuncWithArgs(t *testing.T) {
	e := MapExpr(exprFor(t, "coalesce(a, b, 0)"), nodeid.New("n"))
	fc := e.GetFunctionCall()
	if fc == nil {
		t.Fatalf("expected FunctionCall, got %+v", e)
	}
	if fc.GetName().GetName() != "coalesce" {
		t.Fatalf("func name = %q, want coalesce", fc.GetName().GetName())
	}
	if len(fc.GetArguments()) != 3 {
		t.Fatalf("args count = %d, want 3", len(fc.GetArguments()))
	}
}

func TestMapExprInList(t *testing.T) {
	e := MapExpr(exprFor(t, "a IN (1, 2, 3)"), nodeid.New("n"))
	// A_Expr with AEXPR_IN kind
	op := e.GetOperator()
	if op == nil {
		t.Fatalf("expected OperatorExpr for IN, got %+v", e)
	}
	if op.GetSymbol() != "IN" {
		t.Fatalf("symbol = %q, want IN", op.GetSymbol())
	}
}
