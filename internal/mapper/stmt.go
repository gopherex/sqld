package mapper

// MapStatement maps a single libpg_query AST Node to an IR Statement.
//
// Accessor paths used (for Tasks 9/11 reference):
//
// SelectStmt:
//
//	.GetTargetList()      []*pg.Node → each .GetResTarget()   → .GetVal()/.GetName()
//	.GetFromClause()      []*pg.Node → RangeVar/JoinExpr/RangeSubselect/RangeFunction
//	.GetWhereClause()     *pg.Node
//	.GetGroupClause()     []*pg.Node
//	.GetHavingClause()    *pg.Node
//	.GetSortClause()      []*pg.Node → each .GetSortBy() → .GetNode()/.GetSortbyDir()/.GetSortbyNulls()
//	.GetLimitCount()      *pg.Node
//	.GetLimitOffset()     *pg.Node
//	.GetDistinctClause()  []*pg.Node (non-empty → DISTINCT)
//	.GetValuesLists()     []*pg.Node (VALUES body)
//	.GetOp()              pg.SetOperation  (SETOP_NONE vs SETOP_UNION/INTERSECT/EXCEPT)
//	.GetAll()             bool
//	.GetLarg()/.GetRarg() *pg.SelectStmt
//	.GetWithClause()      *pg.WithClause → .GetCtes() / .GetRecursive()
//
// InsertStmt:
//
//	.GetRelation()       *pg.RangeVar
//	.GetCols()           []*pg.Node → each .GetResTarget().GetName()
//	.GetSelectStmt()     *pg.Node  (VALUES SelectStmt or INSERT..SELECT)
//	.GetOnConflictClause() *pg.OnConflictClause
//	.GetReturningList()  []*pg.Node
//	.GetWithClause()     *pg.WithClause
//
// UpdateStmt:
//
//	.GetRelation()       *pg.RangeVar
//	.GetTargetList()     []*pg.Node → each .GetResTarget() → .GetName()/.GetVal()
//	.GetWhereClause()    *pg.Node
//	.GetFromClause()     []*pg.Node
//	.GetReturningList()  []*pg.Node
//	.GetWithClause()     *pg.WithClause
//
// DeleteStmt:
//
//	.GetRelation()       *pg.RangeVar
//	.GetUsingClause()    []*pg.Node
//	.GetWhereClause()    *pg.Node
//	.GetReturningList()  []*pg.Node
//	.GetWithClause()     *pg.WithClause
//
// RangeVar: .GetCatalogname()/.GetSchemaname()/.GetRelname() → QualifiedName
//
//	.GetInh() — false → ONLY (note: pg default is inh=true for inherits)
//	.GetAlias().GetAliasname()
//
// JoinExpr: .GetJointype() → JoinType enum; .GetIsNatural(); .GetLarg()/.GetRarg();
//
//	.GetQuals() → ON; .GetUsingClause() → []*pg.Node strings
//
// RangeSubselect: .GetLateral(); .GetSubquery() → inner SelectStmt node; .GetAlias()
//
// Set-op enums: pg.SetOperation_SETOP_UNION/INTERSECT/EXCEPT (vs SETOP_NONE = plain)
// Sort dir enums: pg.SortByDir_SORTBY_ASC / SORTBY_DESC / SORTBY_USING
// Join type enums: pg.JoinType_JOIN_INNER/LEFT/FULL/RIGHT
//
// MergeStmt is mapped to a RawStatement{kind=MERGE} (best-effort fallback).

import (
	"github.com/gopherex/sqld/internal/nodeid"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	pg "github.com/pganalyze/pg_query_go/v6"
	pgquery "github.com/wasilibs/go-pgquery"
)

// MapStatement maps a *pg.Node to an IR *irv1.Statement.
// It never panics; unknown nodes fall through to RawStatement.
func MapStatement(node *pg.Node, id nodeid.Builder) *irv1.Statement {
	if node == nil {
		return &irv1.Statement{
			Statement: &irv1.Statement_Raw{Raw: &irv1.RawStatement{
				Kind: irv1.StatementKind_STATEMENT_KIND_OTHER,
				Sql:  "<nil>",
			}},
			NodeId: id.String(),
		}
	}

	switch {
	// ------------------------------------------------------------------ //
	// SELECT
	// ------------------------------------------------------------------ //
	case node.GetSelectStmt() != nil:
		sel := node.GetSelectStmt()
		irSel := mapSelectStmt(sel, id.Child("select"))
		return &irv1.Statement{
			Statement: &irv1.Statement_Select{Select: irSel},
			NodeId:    id.String(),
		}

	// ------------------------------------------------------------------ //
	// INSERT
	// ------------------------------------------------------------------ //
	case node.GetInsertStmt() != nil:
		irIns := mapInsertStmt(node.GetInsertStmt(), id.Child("insert"))
		return &irv1.Statement{
			Statement: &irv1.Statement_Insert{Insert: irIns},
			NodeId:    id.String(),
		}

	// ------------------------------------------------------------------ //
	// UPDATE
	// ------------------------------------------------------------------ //
	case node.GetUpdateStmt() != nil:
		irUpd := mapUpdateStmt(node.GetUpdateStmt(), id.Child("update"))
		return &irv1.Statement{
			Statement: &irv1.Statement_Update{Update: irUpd},
			NodeId:    id.String(),
		}

	// ------------------------------------------------------------------ //
	// DELETE
	// ------------------------------------------------------------------ //
	case node.GetDeleteStmt() != nil:
		irDel := mapDeleteStmt(node.GetDeleteStmt(), id.Child("delete"))
		return &irv1.Statement{
			Statement: &irv1.Statement_Delete{Delete: irDel},
			NodeId:    id.String(),
		}

	// ------------------------------------------------------------------ //
	// MERGE — best-effort: emit as RawStatement{kind=MERGE}
	// ------------------------------------------------------------------ //
	case node.GetMergeStmt() != nil:
		return &irv1.Statement{
			Statement: &irv1.Statement_Raw{Raw: &irv1.RawStatement{
				Kind: irv1.StatementKind_STATEMENT_KIND_MERGE,
				Sql:  deparseStmt(node),
			}},
			NodeId: id.String(),
		}

	// ------------------------------------------------------------------ //
	// Everything else: DDL / TRUNCATE / COPY / TCL / UTILITY / OTHER
	// ------------------------------------------------------------------ //
	default:
		return &irv1.Statement{
			Statement: &irv1.Statement_Raw{Raw: &irv1.RawStatement{
				Kind: classifyRaw(node),
				Sql:  deparseStmt(node),
			}},
			NodeId: id.String(),
		}
	}
}

// ---------------------------------------------------------------------------
// SELECT
// ---------------------------------------------------------------------------

// mapSelectStmt converts a pg.SelectStmt to an irv1.SelectStmt.
// It handles set-operations (UNION/INTERSECT/EXCEPT), plain selects, and
// VALUES-only bodies.
func mapSelectStmt(sel *pg.SelectStmt, id nodeid.Builder) *irv1.SelectStmt {
	if sel == nil {
		return &irv1.SelectStmt{RawSql: "<nil_select>", NodeId: id.String()}
	}

	out := &irv1.SelectStmt{NodeId: id.String()}

	// With clause
	if sel.GetWithClause() != nil {
		with, rec := mapWithClause(sel.GetWithClause(), id.Child("with"))
		out.With = with
		out.WithRecursive = rec
	}

	// ORDER BY (lives at the outer level even for set-ops)
	for i, sortNode := range sel.GetSortClause() {
		out.OrderBy = append(out.OrderBy, mapSortBy(sortNode, id.Child("orderby").Index(i)))
	}

	// LIMIT / OFFSET
	if sel.GetLimitCount() != nil {
		out.Limit = MapExpr(sel.GetLimitCount(), id.Child("limit"))
	}
	if sel.GetLimitOffset() != nil {
		out.Offset = MapExpr(sel.GetLimitOffset(), id.Child("offset"))
	}

	// Body: set-op vs. plain select vs. VALUES
	op := sel.GetOp()
	if op != pg.SetOperation_SETOP_NONE && op != pg.SetOperation_SET_OPERATION_UNDEFINED {
		// Set operation (UNION / INTERSECT / EXCEPT)
		setOp := &irv1.SetOperation{
			Kind:   pgSetOpToIR(op),
			All:    sel.GetAll(),
			NodeId: id.Child("setop").String(),
		}
		if sel.GetLarg() != nil {
			setOp.Left = mapSelectStmt(sel.GetLarg(), id.Child("larg"))
		}
		if sel.GetRarg() != nil {
			setOp.Right = mapSelectStmt(sel.GetRarg(), id.Child("rarg"))
		}
		out.Body = &irv1.SelectStmt_SetOperation{SetOperation: setOp}
		return out
	}

	// VALUES body (e.g. VALUES (1,2), (3,4) as a stand-alone statement)
	if len(sel.GetValuesLists()) > 0 && len(sel.GetTargetList()) == 0 {
		out.Body = &irv1.SelectStmt_Values{Values: mapValuesLists(sel.GetValuesLists(), id.Child("values"))}
		return out
	}

	// Plain SELECT
	ss := mapSimpleSelect(sel, id.Child("ss"))
	out.Body = &irv1.SelectStmt_Select{Select: ss}
	return out
}

func mapSimpleSelect(sel *pg.SelectStmt, id nodeid.Builder) *irv1.SimpleSelect {
	ss := &irv1.SimpleSelect{NodeId: id.String()}

	// DISTINCT / DISTINCT ON
	if len(sel.GetDistinctClause()) > 0 {
		ss.Distinct = true
		// If DistinctClause is non-nil but each item is an empty node it means
		// plain DISTINCT; otherwise it is DISTINCT ON (exprs).
		for i, dn := range sel.GetDistinctClause() {
			if dn.GetNode() != nil {
				ss.DistinctOn = append(ss.DistinctOn, MapExpr(dn, id.Child("distinct_on").Index(i)))
			}
		}
	}

	// Target list
	for i, tNode := range sel.GetTargetList() {
		rt := tNode.GetResTarget()
		if rt == nil {
			continue
		}
		ss.Targets = append(ss.Targets, &irv1.SelectTarget{
			Expr:   MapExpr(rt.GetVal(), id.Child("target").Index(i)),
			Alias:  rt.GetName(),
			NodeId: id.Child("target").Index(i).String(),
		})
	}

	// FROM clause
	for i, fNode := range sel.GetFromClause() {
		ss.From = append(ss.From, mapFromItem(fNode, id.Child("from").Index(i)))
	}

	// WHERE
	if sel.GetWhereClause() != nil {
		ss.Where = MapExpr(sel.GetWhereClause(), id.Child("where"))
	}

	// GROUP BY
	for i, gNode := range sel.GetGroupClause() {
		ss.GroupBy = append(ss.GroupBy, MapExpr(gNode, id.Child("groupby").Index(i)))
	}
	ss.GroupByAll = sel.GetGroupDistinct()

	// HAVING
	if sel.GetHavingClause() != nil {
		ss.Having = MapExpr(sel.GetHavingClause(), id.Child("having"))
	}
	for i, node := range sel.GetWindowClause() {
		if window := node.GetWindowDef(); window != nil {
			spec := mapWindowSpec(window, id.Child("window").Index(i))
			// Here Name declares the window, rather than referencing it.
			spec.RefName = window.GetRefname()
			ss.Windows = append(ss.Windows, &irv1.WindowDef{
				Name: window.GetName(),
				Spec: spec,
			})
		}
	}

	return ss
}

// mapWindowSpec preserves expressions in window partitioning and ordering so
// bind parameters there participate in inference just like other expressions.
func mapWindowSpec(window *pg.WindowDef, id nodeid.Builder) *irv1.WindowSpec {
	if window == nil {
		return nil
	}
	spec := &irv1.WindowSpec{RefName: window.GetRefname()}
	if spec.RefName == "" {
		// OVER w uses Name; OVER (w ORDER BY ...) uses Refname.
		spec.RefName = window.GetName()
	}
	for i, expr := range window.GetPartitionClause() {
		spec.PartitionBy = append(spec.PartitionBy, MapExpr(expr, id.Child("partition").Index(i)))
	}
	for i, order := range window.GetOrderClause() {
		spec.OrderBy = append(spec.OrderBy, mapSortBy(order, id.Child("orderby").Index(i)))
	}
	return spec
}

// ---------------------------------------------------------------------------
// FROM items
// ---------------------------------------------------------------------------

// mapFromItem converts a single from-clause node to a *irv1.FromItem.
// It handles RangeVar, JoinExpr, RangeSubselect, and RangeFunction.
// Unknown node shapes fall through to a best-effort raw raw_sql approach.
func mapFromItem(node *pg.Node, id nodeid.Builder) *irv1.FromItem {
	if node == nil {
		return &irv1.FromItem{NodeId: id.String()}
	}

	switch {
	// ------------------------------------------------ table ref
	case node.GetRangeVar() != nil:
		rv := node.GetRangeVar()
		tableRef := &irv1.TableRef{
			Name:  rangeVarToQualifiedName(rv),
			Alias: rv.GetAlias().GetAliasname(),
			// Inh=false means ONLY; default is inh=true (inherits children)
			Only: !rv.GetInh(),
		}
		return &irv1.FromItem{
			Source: &irv1.FromItem_Table{Table: tableRef},
			NodeId: id.String(),
		}

	// ------------------------------------------------ join
	case node.GetJoinExpr() != nil:
		je := node.GetJoinExpr()
		jc := &irv1.JoinClause{
			Type:    pgJoinTypeToIR(je.GetJointype()),
			Natural: je.GetIsNatural(),
			NodeId:  id.Child("join").String(),
		}
		if je.GetLarg() != nil {
			jc.Left = mapFromItem(je.GetLarg(), id.Child("join_left"))
		}
		if je.GetRarg() != nil {
			jc.Right = mapFromItem(je.GetRarg(), id.Child("join_right"))
		}
		if je.GetQuals() != nil {
			jc.On = MapExpr(je.GetQuals(), id.Child("join_on"))
		}
		for _, uNode := range je.GetUsingClause() {
			if s := uNode.GetString_().GetSval(); s != "" {
				jc.Using = append(jc.Using, s)
			}
		}
		return &irv1.FromItem{
			Source: &irv1.FromItem_Join{Join: jc},
			NodeId: id.String(),
		}

	// ------------------------------------------------ subquery in FROM
	case node.GetRangeSubselect() != nil:
		rs := node.GetRangeSubselect()
		alias := rs.GetAlias().GetAliasname()
		var colAliases []string
		for _, cn := range rs.GetAlias().GetColnames() {
			if s := cn.GetString_().GetSval(); s != "" {
				colAliases = append(colAliases, s)
			}
		}
		var innerSel *irv1.SelectStmt
		if inner := rs.GetSubquery().GetSelectStmt(); inner != nil {
			innerSel = mapSelectStmt(inner, id.Child("subq_sel"))
		}
		subRef := &irv1.SubqueryRef{
			Query:         innerSel,
			Alias:         alias,
			ColumnAliases: colAliases,
			Lateral:       rs.GetLateral(),
		}
		return &irv1.FromItem{
			Source: &irv1.FromItem_Subquery{Subquery: subRef},
			NodeId: id.String(),
		}

	// ------------------------------------------------ range function (TABLE FUNCTION)
	case node.GetRangeFunction() != nil:
		rf := node.GetRangeFunction()
		// Map each function in the functions list
		var callExpr *irv1.Expr
		// Each element is a pg.Node containing a List of [funcCall, colidList];
		// the first item is the actual function call expression. We only map the
		// first function.
		if fns := rf.GetFunctions(); len(fns) > 0 {
			fn := fns[0]
			if lst := fn.GetList(); lst != nil && len(lst.GetItems()) > 0 {
				callExpr = MapExpr(lst.GetItems()[0], id.Child("rfunc").Index(0))
			} else {
				callExpr = MapExpr(fn, id.Child("rfunc").Index(0))
			}
		}
		alias := rf.GetAlias().GetAliasname()
		var colAliases []string
		for _, cn := range rf.GetAlias().GetColnames() {
			if s := cn.GetString_().GetSval(); s != "" {
				colAliases = append(colAliases, s)
			}
		}
		funcRef := &irv1.FunctionRef{
			Call:           callExpr,
			Alias:          alias,
			ColumnAliases:  colAliases,
			Lateral:        rf.GetLateral(),
			WithOrdinality: rf.GetOrdinality(),
		}
		return &irv1.FromItem{
			Source: &irv1.FromItem_Function{Function: funcRef},
			NodeId: id.String(),
		}

	// ------------------------------------------------ fallback
	default:
		// Emit a TableRef with an empty name as a safe fallback — callers can
		// detect the empty name and degrade gracefully.  We also set a raw_sql
		// star expression so the from item carries some information.
		return &irv1.FromItem{NodeId: id.String()}
	}
}

// ---------------------------------------------------------------------------
// INSERT
// ---------------------------------------------------------------------------

func mapInsertStmt(ins *pg.InsertStmt, id nodeid.Builder) *irv1.InsertStmt {
	out := &irv1.InsertStmt{NodeId: id.String()}

	// Table name
	if ins.GetRelation() != nil {
		out.TableName = rangeVarToQualifiedName(ins.GetRelation())
		out.Alias = ins.GetRelation().GetAlias().GetAliasname()
	}

	// Column list
	for _, colNode := range ins.GetCols() {
		rt := colNode.GetResTarget()
		if rt != nil && rt.GetName() != "" {
			out.Columns = append(out.Columns, rt.GetName())
		}
	}

	// WITH clause
	if ins.GetWithClause() != nil {
		with, rec := mapWithClause(ins.GetWithClause(), id.Child("with"))
		out.With = with
		out.WithRecursive = rec
	}

	// Source: VALUES, INSERT..SELECT, or DEFAULT VALUES
	if ins.GetSelectStmt() != nil {
		srcSel := ins.GetSelectStmt().GetSelectStmt()
		if srcSel != nil {
			// VALUES body lives inside a SelectStmt with ValuesLists
			if len(srcSel.GetValuesLists()) > 0 {
				out.Source = &irv1.InsertStmt_Values{
					Values: mapValuesLists(srcSel.GetValuesLists(), id.Child("values")),
				}
			} else {
				out.Source = &irv1.InsertStmt_Query{
					Query: mapSelectStmt(srcSel, id.Child("query")),
				}
			}
		}
	} else {
		out.Source = &irv1.InsertStmt_DefaultValues{DefaultValues: true}
	}

	// ON CONFLICT
	if occ := ins.GetOnConflictClause(); occ != nil {
		out.OnConflict = mapOnConflict(occ, id.Child("on_conflict"))
	}

	// RETURNING
	for i, r := range ins.GetReturningList() {
		out.Returning = append(out.Returning, mapResTargetToSelectTarget(r, id.Child("ret").Index(i)))
	}

	// OVERRIDING SYSTEM VALUE
	out.OverridingSystem = ins.GetOverride() == pg.OverridingKind_OVERRIDING_SYSTEM_VALUE

	return out
}

// ---------------------------------------------------------------------------
// UPDATE
// ---------------------------------------------------------------------------

func mapUpdateStmt(upd *pg.UpdateStmt, id nodeid.Builder) *irv1.UpdateStmt {
	out := &irv1.UpdateStmt{NodeId: id.String()}

	if upd.GetRelation() != nil {
		out.TableName = rangeVarToQualifiedName(upd.GetRelation())
		out.Alias = upd.GetRelation().GetAlias().GetAliasname()
		out.Only = !upd.GetRelation().GetInh()
	}

	// WITH
	if upd.GetWithClause() != nil {
		with, rec := mapWithClause(upd.GetWithClause(), id.Child("with"))
		out.With = with
		out.WithRecursive = rec
	}

	// SET list
	for i, t := range upd.GetTargetList() {
		rt := t.GetResTarget()
		if rt == nil {
			continue
		}
		assign := &irv1.Assignment{
			Columns: []string{rt.GetName()},
			Value:   MapExpr(rt.GetVal(), id.Child("set").Index(i).Child("val")),
			NodeId:  id.Child("set").Index(i).String(),
		}
		out.Set = append(out.Set, assign)
	}

	// FROM
	for i, f := range upd.GetFromClause() {
		out.From = append(out.From, mapFromItem(f, id.Child("from").Index(i)))
	}

	// WHERE
	if upd.GetWhereClause() != nil {
		out.Where = MapExpr(upd.GetWhereClause(), id.Child("where"))
	}

	// RETURNING
	for i, r := range upd.GetReturningList() {
		out.Returning = append(out.Returning, mapResTargetToSelectTarget(r, id.Child("ret").Index(i)))
	}

	return out
}

// ---------------------------------------------------------------------------
// DELETE
// ---------------------------------------------------------------------------

func mapDeleteStmt(del *pg.DeleteStmt, id nodeid.Builder) *irv1.DeleteStmt {
	out := &irv1.DeleteStmt{NodeId: id.String()}

	if del.GetRelation() != nil {
		out.TableName = rangeVarToQualifiedName(del.GetRelation())
		out.Alias = del.GetRelation().GetAlias().GetAliasname()
		out.Only = !del.GetRelation().GetInh()
	}

	// WITH
	if del.GetWithClause() != nil {
		with, rec := mapWithClause(del.GetWithClause(), id.Child("with"))
		out.With = with
		out.WithRecursive = rec
	}

	// USING
	for i, u := range del.GetUsingClause() {
		out.Using = append(out.Using, mapFromItem(u, id.Child("using").Index(i)))
	}

	// WHERE
	if del.GetWhereClause() != nil {
		out.Where = MapExpr(del.GetWhereClause(), id.Child("where"))
	}

	// RETURNING
	for i, r := range del.GetReturningList() {
		out.Returning = append(out.Returning, mapResTargetToSelectTarget(r, id.Child("ret").Index(i)))
	}

	return out
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

// rangeVarToQualifiedName converts a pg.RangeVar to an irv1.QualifiedName.
func rangeVarToQualifiedName(rv *pg.RangeVar) *irv1.QualifiedName {
	if rv == nil {
		return &irv1.QualifiedName{}
	}
	return &irv1.QualifiedName{
		Catalog: rv.GetCatalogname(),
		Schema:  rv.GetSchemaname(),
		Name:    rv.GetRelname(),
	}
}

// mapResTargetToSelectTarget converts a RETURNING / target list node.
func mapResTargetToSelectTarget(node *pg.Node, id nodeid.Builder) *irv1.SelectTarget {
	if node == nil {
		return &irv1.SelectTarget{NodeId: id.String()}
	}
	rt := node.GetResTarget()
	if rt == nil {
		// Might be a bare expression (e.g. *).
		return &irv1.SelectTarget{
			Expr:   MapExpr(node, id.Child("expr")),
			NodeId: id.String(),
		}
	}
	return &irv1.SelectTarget{
		Expr:   MapExpr(rt.GetVal(), id.Child("expr")),
		Alias:  rt.GetName(),
		NodeId: id.String(),
	}
}

// mapSortBy converts a pg SortBy node (wrapped in a pg.Node) to an OrderByItem.
func mapSortBy(node *pg.Node, id nodeid.Builder) *irv1.OrderByItem {
	sb := node.GetSortBy()
	if sb == nil {
		return &irv1.OrderByItem{
			Expr:   MapExpr(node, id.Child("expr")),
			NodeId: id.String(),
		}
	}
	var order irv1.SortOrder
	switch sb.GetSortbyDir() {
	case pg.SortByDir_SORTBY_ASC:
		order = irv1.SortOrder_SORT_ORDER_ASC
	case pg.SortByDir_SORTBY_DESC:
		order = irv1.SortOrder_SORT_ORDER_DESC
	default:
		order = irv1.SortOrder_SORT_ORDER_UNSPECIFIED
	}
	var nullsOrder irv1.NullsOrder
	switch sb.GetSortbyNulls() {
	case pg.SortByNulls_SORTBY_NULLS_FIRST:
		nullsOrder = irv1.NullsOrder_NULLS_ORDER_FIRST
	case pg.SortByNulls_SORTBY_NULLS_LAST:
		nullsOrder = irv1.NullsOrder_NULLS_ORDER_LAST
	}
	item := &irv1.OrderByItem{
		Expr:   MapExpr(sb.GetNode(), id.Child("expr")),
		Order:  order,
		Nulls:  nullsOrder,
		NodeId: id.String(),
	}
	// USING operator
	for _, uOp := range sb.GetUseOp() {
		if s := uOp.GetString_().GetSval(); s != "" {
			item.UsingOperator = s
			break
		}
	}
	return item
}

// mapValuesLists converts the ValuesLists of a SelectStmt to an irv1.Values.
func mapValuesLists(lists []*pg.Node, id nodeid.Builder) *irv1.Values {
	vals := &irv1.Values{}
	for i, rowNode := range lists {
		row := &irv1.ValuesRow{}
		lst := rowNode.GetList()
		if lst != nil {
			for j, item := range lst.GetItems() {
				row.Values = append(row.Values, MapExpr(item, id.Child("row").Index(i).Child("val").Index(j)))
			}
		} else {
			// Fallback: treat the whole node as a single value.
			row.Values = []*irv1.Expr{MapExpr(rowNode, id.Child("row").Index(i).Child("val").Index(0))}
		}
		vals.Rows = append(vals.Rows, row)
	}
	return vals
}

// mapWithClause converts a pg.WithClause to CTE slice + recursive flag.
func mapWithClause(wc *pg.WithClause, id nodeid.Builder) ([]*irv1.CommonTableExpr, bool) {
	if wc == nil {
		return nil, false
	}
	var ctes []*irv1.CommonTableExpr
	for i, cteNode := range wc.GetCtes() {
		pgCte := cteNode.GetCommonTableExpr()
		if pgCte == nil {
			continue
		}
		cid := id.Index(i)
		irCte := &irv1.CommonTableExpr{
			Name:   pgCte.GetCtename(),
			NodeId: cid.String(),
		}
		// Column aliases
		for _, cn := range pgCte.GetAliascolnames() {
			if s := cn.GetString_().GetSval(); s != "" {
				irCte.Columns = append(irCte.Columns, s)
			}
		}
		// Query — must be a SelectStmt node
		if pgCte.GetCtequery() != nil {
			if inner := pgCte.GetCtequery().GetSelectStmt(); inner != nil {
				irCte.Query = mapSelectStmt(inner, cid.Child("query"))
			}
		}
		// Materialization
		switch pgCte.GetCtematerialized() {
		case pg.CTEMaterialize_CTEMaterializeAlways:
			irCte.Materialization = irv1.CteMaterialization_CTE_MATERIALIZATION_MATERIALIZED
		case pg.CTEMaterialize_CTEMaterializeNever:
			irCte.Materialization = irv1.CteMaterialization_CTE_MATERIALIZATION_NOT_MATERIALIZED
		}
		ctes = append(ctes, irCte)
	}
	return ctes, wc.GetRecursive()
}

// mapOnConflict converts a pg.OnConflictClause to irv1.OnConflict.
func mapOnConflict(occ *pg.OnConflictClause, id nodeid.Builder) *irv1.OnConflict {
	if occ == nil {
		return nil
	}
	oc := &irv1.OnConflict{}
	switch occ.GetAction() {
	case pg.OnConflictAction_ONCONFLICT_NOTHING:
		oc.Action = irv1.OnConflictAction_ON_CONFLICT_ACTION_DO_NOTHING
	case pg.OnConflictAction_ONCONFLICT_UPDATE:
		oc.Action = irv1.OnConflictAction_ON_CONFLICT_ACTION_DO_UPDATE
	}
	// Infer clause
	if inf := occ.GetInfer(); inf != nil {
		for _, ie := range inf.GetIndexElems() {
			// Best effort: extract column name from index element
			if ie.GetIndexElem() != nil && ie.GetIndexElem().GetName() != "" {
				oc.TargetColumns = append(oc.TargetColumns, ie.GetIndexElem().GetName())
			}
		}
		oc.TargetConstraint = inf.GetConname()
	}
	// SET assignments
	for i, t := range occ.GetTargetList() {
		rt := t.GetResTarget()
		if rt == nil {
			continue
		}
		oc.Set = append(oc.Set, &irv1.Assignment{
			Columns: []string{rt.GetName()},
			Value:   MapExpr(rt.GetVal(), id.Child("set").Index(i).Child("val")),
			NodeId:  id.Child("set").Index(i).String(),
		})
	}
	// WHERE
	if occ.GetWhereClause() != nil {
		oc.UpdateWhere = MapExpr(occ.GetWhereClause(), id.Child("oc_where"))
	}
	return oc
}

// ---------------------------------------------------------------------------
// Enum converters
// ---------------------------------------------------------------------------

func pgSetOpToIR(op pg.SetOperation) irv1.SetOpKind {
	switch op {
	case pg.SetOperation_SETOP_UNION:
		return irv1.SetOpKind_SET_OP_KIND_UNION
	case pg.SetOperation_SETOP_INTERSECT:
		return irv1.SetOpKind_SET_OP_KIND_INTERSECT
	case pg.SetOperation_SETOP_EXCEPT:
		return irv1.SetOpKind_SET_OP_KIND_EXCEPT
	default:
		return irv1.SetOpKind_SET_OP_KIND_UNSPECIFIED
	}
}

func pgJoinTypeToIR(jt pg.JoinType) irv1.JoinType {
	switch jt {
	case pg.JoinType_JOIN_INNER:
		return irv1.JoinType_JOIN_TYPE_INNER
	case pg.JoinType_JOIN_LEFT:
		return irv1.JoinType_JOIN_TYPE_LEFT
	case pg.JoinType_JOIN_FULL:
		return irv1.JoinType_JOIN_TYPE_FULL
	case pg.JoinType_JOIN_RIGHT:
		return irv1.JoinType_JOIN_TYPE_RIGHT
	default:
		return irv1.JoinType_JOIN_TYPE_UNSPECIFIED
	}
}

// ---------------------------------------------------------------------------
// Raw statement classification
// ---------------------------------------------------------------------------

func classifyRaw(node *pg.Node) irv1.StatementKind {
	switch {
	// DDL
	case node.GetCreateStmt() != nil,
		node.GetAlterTableStmt() != nil,
		node.GetIndexStmt() != nil,
		node.GetDropStmt() != nil,
		node.GetViewStmt() != nil,
		node.GetCreateSeqStmt() != nil,
		node.GetAlterSeqStmt() != nil,
		node.GetCreateSchemaStmt() != nil,
		node.GetCreateFunctionStmt() != nil,
		node.GetAlterFunctionStmt() != nil,
		node.GetRuleStmt() != nil,
		node.GetCompositeTypeStmt() != nil,
		node.GetCreateEnumStmt() != nil,
		node.GetCreateRangeStmt() != nil,
		node.GetCreateDomainStmt() != nil,
		node.GetCreateExtensionStmt() != nil,
		node.GetAlterExtensionStmt() != nil,
		node.GetDropOwnedStmt() != nil,
		node.GetCreateTableAsStmt() != nil,
		node.GetCreateForeignTableStmt() != nil,
		node.GetGrantStmt() != nil,
		node.GetGrantRoleStmt() != nil,
		node.GetCreateTrigStmt() != nil,
		node.GetAlterObjectDependsStmt() != nil,
		node.GetRenameStmt() != nil,
		node.GetCommentStmt() != nil:
		return irv1.StatementKind_STATEMENT_KIND_DDL

	// TRUNCATE
	case node.GetTruncateStmt() != nil:
		return irv1.StatementKind_STATEMENT_KIND_TRUNCATE

	// COPY
	case node.GetCopyStmt() != nil:
		return irv1.StatementKind_STATEMENT_KIND_COPY

	// TCL
	case node.GetTransactionStmt() != nil,
		node.GetDeclareCursorStmt() != nil:
		return irv1.StatementKind_STATEMENT_KIND_TCL

	// UTILITY
	case node.GetVacuumStmt() != nil,
		node.GetVariableSetStmt() != nil,
		node.GetExplainStmt() != nil,
		node.GetClusterStmt() != nil,
		node.GetReindexStmt() != nil:
		return irv1.StatementKind_STATEMENT_KIND_UTILITY

	default:
		return irv1.StatementKind_STATEMENT_KIND_OTHER
	}
}

// deparseStmt deparsed a whole statement node back to SQL.
// On failure it returns a marker string. Never panics.
func deparseStmt(node *pg.Node) string {
	if node == nil {
		return "<nil>"
	}
	tree := &pg.ParseResult{
		Stmts: []*pg.RawStmt{{Stmt: node}},
	}
	sql, err := pgquery.Deparse(tree)
	if err != nil {
		return "<deparse_failed>"
	}
	return sql
}
