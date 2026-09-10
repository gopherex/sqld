package mapper

import (
	"strconv"

	"github.com/gopherex/sqld/internal/nodeid"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	pg "github.com/pganalyze/pg_query_go/v6"
	pgquery "github.com/wasilibs/go-pgquery"
)

// MapExpr maps a single libpg_query AST Node to an IR Expr, assigning the
// stable node_id from id.  Every recursive child uses a child Builder so ids
// are unique and deterministic.
//
// Accessor paths used (for Task 8 reference):
//
//	ColumnRef  : node.GetColumnRef().GetFields() → each *pg.Node:
//	               .GetString_().GetSval()  — column/qualifier name
//	               .GetAStar()              — wildcard (*)
//	A_Expr     : node.GetAExpr()
//	               .GetName()  → []*pg.Node → .GetString_().GetSval()  (operator symbol)
//	               .GetLexpr() *pg.Node — left operand (may be nil for unary)
//	               .GetRexpr() *pg.Node — right operand; for IN/BETWEEN it is a List
//	               .GetKind()  A_Expr_Kind — AEXPR_OP, AEXPR_IN, AEXPR_LIKE, etc.
//	BoolExpr   : node.GetBoolExpr()
//	               .GetBoolop()  BoolExprType — AND_EXPR / OR_EXPR / NOT_EXPR
//	               .GetArgs()    []*pg.Node
//	NullTest   : node.GetNullTest()
//	               .GetArg()          *pg.Node
//	               .GetNulltesttype() NullTestType — IS_NULL / IS_NOT_NULL
//	FuncCall   : node.GetFuncCall()
//	               .GetFuncname()    []*pg.Node → .GetString_().GetSval()
//	               .GetArgs()        []*pg.Node
//	               .GetAggStar()     bool
//	               .GetAggDistinct() bool
//	               .GetFuncVariadic() bool
//	CaseExpr   : node.GetCaseExpr()
//	               .GetArg()       *pg.Node — CASE operand (optional)
//	               .GetArgs()      []*pg.Node — each via .GetCaseWhen()
//	               .GetDefresult() *pg.Node — ELSE result
//	TypeCast   : node.GetTypeCast()
//	               .GetArg()      *pg.Node
//	               .GetTypeName() *pg.TypeName
//	SubLink    : node.GetSubLink() → raw_sql fallback (TODO task8)
//
// Deparse fallback: wraps the node in a minimal SELECT and calls pg.Deparse.
// If that fails the raw_sql value is set to "<raw_sql_unavailable>".
//
// The function never panics and never returns nil for a non-nil input.
func MapExpr(node *pg.Node, id nodeid.Builder) *irv1.Expr {
	if node == nil {
		return &irv1.Expr{
			Node:   &irv1.Expr_RawSql{RawSql: "<nil>"},
			NodeId: id.String(),
		}
	}

	switch {
	// ------------------------------------------------------------------ //
	//  ColumnRef / A_Star (wildcard)
	// ------------------------------------------------------------------ //
	case node.GetColumnRef() != nil:
		return mapColumnRef(node.GetColumnRef(), id)

	// ------------------------------------------------------------------ //
	//  Literal (A_Const)
	// ------------------------------------------------------------------ //
	case node.GetAConst() != nil:
		return mapAConst(node.GetAConst(), id)

	// ------------------------------------------------------------------ //
	//  Parameter ($n)
	// ------------------------------------------------------------------ //
	case node.GetParamRef() != nil:
		p := node.GetParamRef()
		return &irv1.Expr{
			Node:   &irv1.Expr_Parameter{Parameter: &irv1.ParameterRef{Position: uint32(p.GetNumber())}},
			NodeId: id.String(),
		}

	// ------------------------------------------------------------------ //
	//  FuncCall
	// ------------------------------------------------------------------ //
	case node.GetFuncCall() != nil:
		return mapFuncCall(node.GetFuncCall(), id)

	// ------------------------------------------------------------------ //
	//  A_Expr  (binary / unary operators, IN, LIKE, BETWEEN, …)
	// ------------------------------------------------------------------ //
	case node.GetAExpr() != nil:
		return mapAExpr(node.GetAExpr(), id)

	// ------------------------------------------------------------------ //
	//  BoolExpr  (AND / OR / NOT)
	// ------------------------------------------------------------------ //
	case node.GetBoolExpr() != nil:
		return mapBoolExpr(node.GetBoolExpr(), id)

	// ------------------------------------------------------------------ //
	//  NullTest  (IS NULL / IS NOT NULL)
	// ------------------------------------------------------------------ //
	case node.GetNullTest() != nil:
		return mapNullTest(node.GetNullTest(), id)

	// ------------------------------------------------------------------ //
	//  CaseExpr
	// ------------------------------------------------------------------ //
	case node.GetCaseExpr() != nil:
		return mapCaseExpr(node.GetCaseExpr(), id)

	// ------------------------------------------------------------------ //
	//  TypeCast
	// ------------------------------------------------------------------ //
	case node.GetTypeCast() != nil:
		tc := node.GetTypeCast()
		return &irv1.Expr{
			Node: &irv1.Expr_Cast{Cast: &irv1.CastExpr{
				Expr:       MapExpr(tc.GetArg(), id.Child("cast_arg")),
				TargetType: MapType(tc.GetTypeName()),
			}},
			NodeId: id.String(),
		}

	// ------------------------------------------------------------------ //
	//  List  (e.g. IN-list, BETWEEN bounds, VALUES row)
	// ------------------------------------------------------------------ //
	case node.GetList() != nil:
		return mapList(node.GetList(), id, false)

	// ------------------------------------------------------------------ //
	//  RowExpr  (ROW(a, b, c))
	// ------------------------------------------------------------------ //
	case node.GetRowExpr() != nil:
		return mapRowExpr(node.GetRowExpr(), id)

	// ------------------------------------------------------------------ //
	//  SubLink  → structured SubqueryExpr
	// ------------------------------------------------------------------ //
	case node.GetSubLink() != nil:
		return mapSubLink(node.GetSubLink(), id)

	// ------------------------------------------------------------------ //
	//  CoalesceExpr  (COALESCE(a, b, …) — libpg_query promotes this)
	// ------------------------------------------------------------------ //
	case node.GetCoalesceExpr() != nil:
		ce := node.GetCoalesceExpr()
		args := make([]*irv1.Expr, 0, len(ce.GetArgs()))
		for i, a := range ce.GetArgs() {
			args = append(args, MapExpr(a, id.Child("arg").Index(i)))
		}
		return &irv1.Expr{
			Node: &irv1.Expr_FunctionCall{FunctionCall: &irv1.FunctionCall{
				Name:      &irv1.QualifiedName{Name: "coalesce"},
				Arguments: args,
			}},
			NodeId: id.String(),
		}

	// ------------------------------------------------------------------ //
	//  MinMaxExpr  (GREATEST / LEAST — libpg_query promotes this)
	// ------------------------------------------------------------------ //
	case node.GetMinMaxExpr() != nil:
		mm := node.GetMinMaxExpr()
		fnName := "greatest"
		if mm.GetOp() == pg.MinMaxOp_IS_LEAST {
			fnName = "least"
		}
		args := make([]*irv1.Expr, 0, len(mm.GetArgs()))
		for i, a := range mm.GetArgs() {
			args = append(args, MapExpr(a, id.Child("arg").Index(i)))
		}
		return &irv1.Expr{
			Node: &irv1.Expr_FunctionCall{FunctionCall: &irv1.FunctionCall{
				Name:      &irv1.QualifiedName{Name: fnName},
				Arguments: args,
			}},
			NodeId: id.String(),
		}

	// ------------------------------------------------------------------ //
	//  A_Star as a bare node (e.g. as a standalone expression)
	// ------------------------------------------------------------------ //
	case node.GetAStar() != nil:
		return &irv1.Expr{
			Node:   &irv1.Expr_Star{Star: &irv1.StarExpr{}},
			NodeId: id.String(),
		}

	// ------------------------------------------------------------------ //
	//  String_ node (e.g. operator name in some contexts)
	// ------------------------------------------------------------------ //
	case node.GetString_() != nil:
		sval := node.GetString_().GetSval()
		return &irv1.Expr{
			Node:   &irv1.Expr_RawSql{RawSql: sval},
			NodeId: id.String(),
		}

	// ------------------------------------------------------------------ //
	//  Fallback: deparse or marker
	// ------------------------------------------------------------------ //
	default:
		return &irv1.Expr{
			Node:   &irv1.Expr_RawSql{RawSql: deparseNode(node)},
			NodeId: id.String(),
		}
	}
}

// ---------------------------------------------------------------------------
// ColumnRef helpers
// ---------------------------------------------------------------------------

func mapColumnRef(cr *pg.ColumnRef, id nodeid.Builder) *irv1.Expr {
	fields := cr.GetFields()
	if len(fields) == 0 {
		return &irv1.Expr{Node: &irv1.Expr_RawSql{RawSql: "<empty_column_ref>"}, NodeId: id.String()}
	}

	// Check if the last field is a wildcard A_Star → StarExpr
	last := fields[len(fields)-1]
	if last.GetAStar() != nil {
		qualifier := ""
		if len(fields) >= 2 {
			qualifier = fields[len(fields)-2].GetString_().GetSval()
		}
		return &irv1.Expr{
			Node:   &irv1.Expr_Star{Star: &irv1.StarExpr{Qualifier: qualifier}},
			NodeId: id.String(),
		}
	}

	// Normal ColumnRef
	column := last.GetString_().GetSval()
	qualifier := ""
	if len(fields) >= 2 {
		qualifier = fields[len(fields)-2].GetString_().GetSval()
	}
	return &irv1.Expr{
		Node: &irv1.Expr_ColumnRef{ColumnRef: &irv1.ColumnRef{
			Qualifier: qualifier,
			Column:    column,
		}},
		NodeId: id.String(),
	}
}

// ---------------------------------------------------------------------------
// A_Const helper
// ---------------------------------------------------------------------------

func mapAConst(ac *pg.A_Const, id nodeid.Builder) *irv1.Expr {
	var lit *irv1.Literal

	switch {
	case ac.GetIsnull():
		lit = &irv1.Literal{Value: &irv1.Literal_NullValue{NullValue: true}}

	case ac.GetIval() != nil:
		lit = &irv1.Literal{Value: &irv1.Literal_IntValue{IntValue: int64(ac.GetIval().GetIval())}}

	case ac.GetFval() != nil:
		// Fval is a string representation of the float literal; try to
		// parse it to float64; if that fails store as NumericValue.
		fstr := ac.GetFval().GetFval()
		if f, err := strconv.ParseFloat(fstr, 64); err == nil {
			lit = &irv1.Literal{Value: &irv1.Literal_FloatValue{FloatValue: f}}
		} else {
			lit = &irv1.Literal{Value: &irv1.Literal_NumericValue{NumericValue: fstr}}
		}

	case ac.GetSval() != nil:
		lit = &irv1.Literal{Value: &irv1.Literal_StringValue{StringValue: ac.GetSval().GetSval()}}

	case ac.GetBoolval() != nil:
		lit = &irv1.Literal{Value: &irv1.Literal_BoolValue{BoolValue: ac.GetBoolval().GetBoolval()}}

	default:
		// Unexpected A_Const shape — best effort.
		lit = &irv1.Literal{Value: &irv1.Literal_NullValue{NullValue: true}}
	}

	return &irv1.Expr{
		Node:   &irv1.Expr_Literal{Literal: lit},
		NodeId: id.String(),
	}
}

// ---------------------------------------------------------------------------
// FuncCall helper
// ---------------------------------------------------------------------------

func mapFuncCall(fc *pg.FuncCall, id nodeid.Builder) *irv1.Expr {
	qname := funcNameToQualifiedName(fc.GetFuncname())

	// Map arguments.  When agg_star is set (count(*)) there are no regular
	// args, but we include a StarExpr as the single argument so callers can
	// tell it was a wildcard.
	var args []*irv1.Expr
	if fc.GetAggStar() {
		args = []*irv1.Expr{{
			Node:   &irv1.Expr_Star{Star: &irv1.StarExpr{}},
			NodeId: id.Child("arg").Index(0).String(),
		}}
	} else {
		for i, a := range fc.GetArgs() {
			args = append(args, MapExpr(a, id.Child("arg").Index(i)))
		}
	}

	call := &irv1.FunctionCall{
		Name:      qname,
		Arguments: args,
		Distinct:  fc.GetAggDistinct(),
		Variadic:  fc.GetFuncVariadic(),
	}

	// Best-effort: map filter if present.
	if f := fc.GetAggFilter(); f != nil {
		call.Filter = MapExpr(f, id.Child("filter"))
	}

	for i, order := range fc.GetAggOrder() {
		call.OrderBy = append(call.OrderBy, mapSortBy(order, id.Child("orderby").Index(i)))
	}
	call.Over = mapWindowSpec(fc.GetOver(), id.Child("over"))

	return &irv1.Expr{
		Node:   &irv1.Expr_FunctionCall{FunctionCall: call},
		NodeId: id.String(),
	}
}

// funcNameToQualifiedName converts the funcname node list to a QualifiedName.
// Last element → Name, preceding element → Schema.
func funcNameToQualifiedName(nodes []*pg.Node) *irv1.QualifiedName {
	if len(nodes) == 0 {
		return &irv1.QualifiedName{}
	}
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		parts = append(parts, n.GetString_().GetSval())
	}
	qn := &irv1.QualifiedName{}
	switch len(parts) {
	case 1:
		qn.Name = parts[0]
	case 2:
		qn.Schema = parts[0]
		qn.Name = parts[1]
	default:
		qn.Catalog = parts[0]
		qn.Schema = parts[1]
		qn.Name = parts[len(parts)-1]
	}
	return qn
}

// ---------------------------------------------------------------------------
// A_Expr helper
// ---------------------------------------------------------------------------

func mapAExpr(ae *pg.A_Expr, id nodeid.Builder) *irv1.Expr {
	symbol := aExprSymbol(ae)

	var operands []*irv1.Expr

	if ae.GetLexpr() != nil {
		operands = append(operands, MapExpr(ae.GetLexpr(), id.Child("op").Index(0)))
	}

	// For IN/NOT IN the rexpr is a List node.  We map it directly (it will
	// become a ListExpr containing the in-list elements).
	if ae.GetRexpr() != nil {
		operands = append(operands, MapExpr(ae.GetRexpr(), id.Child("op").Index(len(operands))))
	}

	return &irv1.Expr{
		Node: &irv1.Expr_Operator{Operator: &irv1.OperatorExpr{
			Symbol:   symbol,
			Operands: operands,
		}},
		NodeId: id.String(),
	}
}

// aExprSymbol derives the operator symbol string from an A_Expr.
func aExprSymbol(ae *pg.A_Expr) string {
	switch ae.GetKind() {
	case pg.A_Expr_Kind_AEXPR_IN:
		// Name list may hold "=" (IN) or "<>" (NOT IN).
		for _, n := range ae.GetName() {
			if n.GetString_().GetSval() == "<>" {
				return "NOT IN"
			}
		}
		return "IN"

	case pg.A_Expr_Kind_AEXPR_LIKE:
		return "LIKE"

	case pg.A_Expr_Kind_AEXPR_ILIKE:
		return "ILIKE"

	case pg.A_Expr_Kind_AEXPR_SIMILAR:
		return "SIMILAR TO"

	case pg.A_Expr_Kind_AEXPR_BETWEEN:
		return "BETWEEN"

	case pg.A_Expr_Kind_AEXPR_NOT_BETWEEN:
		return "NOT BETWEEN"

	case pg.A_Expr_Kind_AEXPR_BETWEEN_SYM:
		return "BETWEEN SYMMETRIC"

	case pg.A_Expr_Kind_AEXPR_NOT_BETWEEN_SYM:
		return "NOT BETWEEN SYMMETRIC"

	case pg.A_Expr_Kind_AEXPR_DISTINCT:
		return "IS DISTINCT FROM"

	case pg.A_Expr_Kind_AEXPR_NOT_DISTINCT:
		return "IS NOT DISTINCT FROM"

	case pg.A_Expr_Kind_AEXPR_NULLIF:
		return "NULLIF"

	case pg.A_Expr_Kind_AEXPR_OP,
		pg.A_Expr_Kind_AEXPR_OP_ANY,
		pg.A_Expr_Kind_AEXPR_OP_ALL:
		// Normal operator — take the first Name node's sval.
		for _, n := range ae.GetName() {
			sym := n.GetString_().GetSval()
			if sym != "" {
				if ae.GetKind() == pg.A_Expr_Kind_AEXPR_OP_ANY {
					return sym + " ANY"
				}
				if ae.GetKind() == pg.A_Expr_Kind_AEXPR_OP_ALL {
					return sym + " ALL"
				}
				return sym
			}
		}
		return "<op>"

	default:
		// Unrecognised kind — still try to get the name.
		for _, n := range ae.GetName() {
			sym := n.GetString_().GetSval()
			if sym != "" {
				return sym
			}
		}
		return "<op>"
	}
}

// ---------------------------------------------------------------------------
// BoolExpr helper
// ---------------------------------------------------------------------------

func mapBoolExpr(be *pg.BoolExpr, id nodeid.Builder) *irv1.Expr {
	var symbol string
	switch be.GetBoolop() {
	case pg.BoolExprType_AND_EXPR:
		symbol = "AND"
	case pg.BoolExprType_OR_EXPR:
		symbol = "OR"
	case pg.BoolExprType_NOT_EXPR:
		symbol = "NOT"
	default:
		symbol = "AND"
	}

	operands := make([]*irv1.Expr, 0, len(be.GetArgs()))
	for i, a := range be.GetArgs() {
		operands = append(operands, MapExpr(a, id.Child("bool").Index(i)))
	}

	return &irv1.Expr{
		Node: &irv1.Expr_Operator{Operator: &irv1.OperatorExpr{
			Symbol:   symbol,
			Operands: operands,
		}},
		NodeId: id.String(),
	}
}

// ---------------------------------------------------------------------------
// NullTest helper
// ---------------------------------------------------------------------------

func mapNullTest(nt *pg.NullTest, id nodeid.Builder) *irv1.Expr {
	symbol := "IS NULL"
	if nt.GetNulltesttype() == pg.NullTestType_IS_NOT_NULL {
		symbol = "IS NOT NULL"
	}

	var operands []*irv1.Expr
	if nt.GetArg() != nil {
		operands = []*irv1.Expr{MapExpr(nt.GetArg(), id.Child("null_arg"))}
	}

	return &irv1.Expr{
		Node: &irv1.Expr_Operator{Operator: &irv1.OperatorExpr{
			Symbol:   symbol,
			Operands: operands,
		}},
		NodeId: id.String(),
	}
}

// ---------------------------------------------------------------------------
// CaseExpr helper
// ---------------------------------------------------------------------------

func mapCaseExpr(ce *pg.CaseExpr, id nodeid.Builder) *irv1.Expr {
	var operand *irv1.Expr
	if ce.GetArg() != nil {
		operand = MapExpr(ce.GetArg(), id.Child("case_operand"))
	}

	whens := make([]*irv1.CaseWhen, 0, len(ce.GetArgs()))
	for i, whenNode := range ce.GetArgs() {
		cw := whenNode.GetCaseWhen()
		if cw == nil {
			continue
		}
		whens = append(whens, &irv1.CaseWhen{
			Condition: MapExpr(cw.GetExpr(), id.Child("when").Index(i).Child("cond")),
			Result:    MapExpr(cw.GetResult(), id.Child("when").Index(i).Child("result")),
		})
	}

	var elseResult *irv1.Expr
	if ce.GetDefresult() != nil {
		elseResult = MapExpr(ce.GetDefresult(), id.Child("else"))
	}

	return &irv1.Expr{
		Node: &irv1.Expr_CaseExpr{CaseExpr: &irv1.CaseExpr{
			Operand:    operand,
			Whens:      whens,
			ElseResult: elseResult,
		}},
		NodeId: id.String(),
	}
}

// ---------------------------------------------------------------------------
// List / RowExpr helpers
// ---------------------------------------------------------------------------

func mapList(lst *pg.List, id nodeid.Builder, isRow bool) *irv1.Expr {
	elems := make([]*irv1.Expr, 0, len(lst.GetItems()))
	for i, item := range lst.GetItems() {
		elems = append(elems, MapExpr(item, id.Child("elem").Index(i)))
	}
	return &irv1.Expr{
		Node:   &irv1.Expr_List{List: &irv1.ListExpr{Elements: elems, IsRow: isRow}},
		NodeId: id.String(),
	}
}

func mapRowExpr(re *pg.RowExpr, id nodeid.Builder) *irv1.Expr {
	elems := make([]*irv1.Expr, 0, len(re.GetArgs()))
	for i, a := range re.GetArgs() {
		elems = append(elems, MapExpr(a, id.Child("elem").Index(i)))
	}
	return &irv1.Expr{
		Node:   &irv1.Expr_List{List: &irv1.ListExpr{Elements: elems, IsRow: true}},
		NodeId: id.String(),
	}
}

// ---------------------------------------------------------------------------
// SubLink / SubqueryExpr helper
// ---------------------------------------------------------------------------

// mapSubLink converts a pg SubLink node to an irv1.Expr_Subquery.
//
// SubLinkType to SubqueryKind mapping:
//
//	EXPR_SUBLINK  → SCALAR  (plain scalar subquery)
//	EXISTS_SUBLINK → EXISTS
//	ANY_SUBLINK   → IN / ANY depending on oper_name
//	ALL_SUBLINK   → ALL
//
// The Testexpr (LHS of IN/ANY/ALL) maps to Operand; the OperName first element
// maps to Symbol (e.g. "=" for IN, "<>" for !=, etc.).
func mapSubLink(sl *pg.SubLink, id nodeid.Builder) *irv1.Expr {
	kind := subLinkKind(sl)

	sq := &irv1.SubqueryExpr{Kind: kind}

	// Inner query
	if inner := sl.GetSubselect().GetSelectStmt(); inner != nil {
		sq.Query = mapSelectStmt(inner, id.Child("sublink_sel"))
	}

	// Operand (LHS for IN/ANY/ALL)
	if sl.GetTestexpr() != nil {
		sq.Operand = MapExpr(sl.GetTestexpr(), id.Child("sublink_lhs"))
	}

	// Operator symbol (ANY/ALL only)
	for _, opNode := range sl.GetOperName() {
		if s := opNode.GetString_().GetSval(); s != "" {
			sq.Symbol = s
			break
		}
	}

	return &irv1.Expr{
		Node:   &irv1.Expr_Subquery{Subquery: sq},
		NodeId: id.String(),
	}
}

// subLinkKind maps pg SubLinkType to irv1 SubqueryKind.
func subLinkKind(sl *pg.SubLink) irv1.SubqueryKind {
	switch sl.GetSubLinkType() {
	case pg.SubLinkType_EXISTS_SUBLINK:
		return irv1.SubqueryKind_SUBQUERY_KIND_EXISTS
	case pg.SubLinkType_ALL_SUBLINK:
		return irv1.SubqueryKind_SUBQUERY_KIND_ALL
	case pg.SubLinkType_ANY_SUBLINK:
		// ANY_SUBLINK is used for both x IN (SELECT ...) and x op ANY (SELECT ...).
		// When OperName is empty or contains "=" it's IN semantics; when OperName
		// contains a different operator it's a generic ANY.
		hasNonEqOp := false
		for _, n := range sl.GetOperName() {
			sym := n.GetString_().GetSval()
			if sym != "" && sym != "=" {
				hasNonEqOp = true
				break
			}
		}
		if hasNonEqOp {
			return irv1.SubqueryKind_SUBQUERY_KIND_ANY
		}
		return irv1.SubqueryKind_SUBQUERY_KIND_IN
	case pg.SubLinkType_EXPR_SUBLINK:
		return irv1.SubqueryKind_SUBQUERY_KIND_SCALAR
	default:
		// ROWCOMPARE, MULTIEXPR, ARRAY, CTE — best effort: treat as scalar
		return irv1.SubqueryKind_SUBQUERY_KIND_SCALAR
	}
}

// ---------------------------------------------------------------------------
// Deparse fallback
// ---------------------------------------------------------------------------

// deparseNode attempts to produce a SQL string from a single *pg.Node by
// wrapping it in a minimal SELECT and calling pg.Deparse.  On any error it
// returns "<raw_sql_unavailable>".
func deparseNode(node *pg.Node) string {
	if node == nil {
		return "<nil>"
	}
	// Wrap in a SELECT statement so Deparse has a full parse tree.
	tree := &pg.ParseResult{
		Stmts: []*pg.RawStmt{
			{
				Stmt: &pg.Node{
					Node: &pg.Node_SelectStmt{
						SelectStmt: &pg.SelectStmt{
							TargetList: []*pg.Node{
								{
									Node: &pg.Node_ResTarget{
										ResTarget: &pg.ResTarget{
											Val: node,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	sql, err := pgquery.Deparse(tree)
	if err != nil {
		return "<raw_sql_unavailable>"
	}
	// Strip the surrounding "SELECT " prefix to get just the expression text.
	if len(sql) > 7 {
		return sql[7:] // len("SELECT ") == 7
	}
	return sql
}
