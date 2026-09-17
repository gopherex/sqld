package query

import irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"

// A scalar subquery is NULL when it returns no rows, even if its target is
// NOT NULL. Only remove that possibility for a proven singleton projection.
func scalarSubqueryNullable(sub *irv1.SubqueryExpr) bool {
	if sub.GetKind() != irv1.SubqueryKind_SUBQUERY_KIND_SCALAR {
		return true
	}
	query := sub.GetQuery()
	selectStmt := query.GetSelect()
	if selectStmt == nil || len(selectStmt.GetTargets()) != 1 ||
		len(selectStmt.GetGroupBy()) != 0 || selectStmt.GetGroupByAll() ||
		selectStmt.GetHaving() != nil || query.GetLimit() != nil || query.GetOffset() != nil {
		return true
	}
	target := selectStmt.GetTargets()[0].GetExpr()
	scalar, aggregate := singletonProjection(target)
	if !scalar || (!aggregate && (len(selectStmt.GetFrom()) != 0 || selectStmt.GetWhere() != nil)) {
		return true
	}
	// Do not resolve inner column names against the outer query's scope.
	return exprNullable(target, nil)
}

// Unknown functions may return a set, including zero rows. COUNT(*) is a
// local aggregate even in a correlated query; aggregates over outer columns
// alone can belong to an outer query, so they are deliberately not proven here.
func singletonProjection(expr *irv1.Expr) (scalar, aggregate bool) {
	if expr == nil {
		return true, false
	}
	if expr.GetLiteral() != nil || expr.GetColumnRef() != nil || expr.GetParameter() != nil {
		return true, false
	}
	if cast := expr.GetCast(); cast != nil {
		return singletonProjection(cast.GetExpr())
	}
	var args []*irv1.Expr
	if fn := expr.GetFunctionCall(); fn != nil {
		if fn.GetOver() != nil || (fn.GetName().GetSchema() != "" && fn.GetName().GetSchema() != "pg_catalog") {
			return false, false
		}
		if funcName(fn) == "count" {
			args := fn.GetArguments()
			return true, len(args) == 0 || (len(args) == 1 && args[0].GetStar() != nil)
		}
		if !strictNullFunction(fn) && funcName(fn) != "coalesce" && funcName(fn) != "nullif" {
			return false, false
		}
		args = fn.GetArguments()
	} else if op := expr.GetOperator(); op != nil {
		args = op.GetOperands()
	} else if c := expr.GetCaseExpr(); c != nil {
		args = append(args, c.GetOperand(), c.GetElseResult())
		for _, when := range c.GetWhens() {
			args = append(args, when.GetCondition(), when.GetResult())
		}
	} else {
		return false, false
	}
	for _, arg := range args {
		safe, agg := singletonProjection(arg)
		if !safe {
			return false, false
		}
		aggregate = aggregate || agg
	}
	return true, aggregate
}
