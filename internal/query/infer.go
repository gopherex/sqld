package query

import (
	"fmt"
	"sort"
	"strings"

	"github.com/yaroher/sqld/internal/catalog"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// Infer mutates q by:
//  1. Walking the AST to collect ParameterRef nodes → QueryParameter entries.
//  2. Attempting shallow type inference for parameters by examining operator
//     operands — if a param appears beside a resolved ColumnRef, the param
//     inherits the column's type.
//  3. For SELECT queries, resolving SelectTarget nodes to QueryColumn entries.
//
// Unresolvable items produce a diagnostic but never a panic.
func Infer(q *pluginv1.Query, cat *irv1.Catalog, d *catalog.Diagnostics) {
	if q == nil || q.Ast == nil {
		return
	}

	// Build alias→table lookup from the catalog for fast resolution.
	catIdx := buildCatalogIndex(cat)

	// ------------------------------------------------------------------ //
	// 1. Collect all parameter refs from the full AST.
	// ------------------------------------------------------------------ //
	paramMap := make(map[uint32]*pluginv1.QueryParameter) // keyed by position
	collectParams(q.Ast, paramMap)

	// ------------------------------------------------------------------ //
	// 2. Shallow type inference via operator context.
	// ------------------------------------------------------------------ //
	// For each operator expr we check whether one side is a ParameterRef and
	// the other is a ColumnRef; if so, resolve the column and assign its type.
	inferParamTypes(q.Ast, paramMap, catIdx, q.GetName(), d)

	// Emit diagnostics for unresolved params and build the ordered param slice.
	// Order by position (1-based).
	ordered := orderedParams(paramMap)
	for _, p := range ordered {
		if p.GetType() == nil {
			d.Add("info", fmt.Sprintf("param $%d type unresolved in query %s", p.GetNumber(), q.GetName()))
		}
	}
	q.Parameters = ordered

	// ------------------------------------------------------------------ //
	// 3. Column inference (SELECT, or RETURNING for INSERT/UPDATE/DELETE).
	// ------------------------------------------------------------------ //
	switch {
	case q.Ast.GetSelect() != nil:
		sel := q.Ast.GetSelect()
		ss := sel.GetSelect()
		if ss == nil {
			return
		}

		// Build a table alias map from the FROM clause.
		aliasMap := buildAliasMap(ss.GetFrom(), catIdx)

		// Collect all tables referenced (for star expansion).
		var fromTables []*irv1.Table
		for _, fi := range ss.GetFrom() {
			tbl := resolveFromItemTable(fi, catIdx)
			if tbl != nil {
				fromTables = append(fromTables, tbl)
			}
		}
		fromTables = collectJoinTables(ss.GetFrom(), catIdx, fromTables)

		for _, target := range ss.GetTargets() {
			expr := target.GetExpr()
			alias := target.GetAlias()
			cols := resolveTarget(expr, alias, aliasMap, fromTables, catIdx, q.GetName(), d)
			q.Columns = append(q.Columns, cols...)
		}

	case q.Ast.GetInsert() != nil:
		ins := q.Ast.GetInsert()
		returning := ins.GetReturning()
		if len(returning) == 0 {
			return
		}
		// Resolve aliasMap from the target table.
		tbl := catIdx.lookupTable(ins.GetTableName().GetName())
		aliasMap := make(map[string]*irv1.Table)
		if tbl != nil {
			aliasMap[strings.ToLower(ins.GetTableName().GetName())] = tbl
			if ins.GetAlias() != "" {
				aliasMap[strings.ToLower(ins.GetAlias())] = tbl
			}
		}
		var fromTables []*irv1.Table
		if tbl != nil {
			fromTables = []*irv1.Table{tbl}
		}
		for _, target := range returning {
			cols := resolveTarget(target.GetExpr(), target.GetAlias(), aliasMap, fromTables, catIdx, q.GetName(), d)
			q.Columns = append(q.Columns, cols...)
		}

		// Also infer INSERT parameter types from the column list + table schema.
		inferInsertParamTypes(ins, paramMap, tbl)

	case q.Ast.GetUpdate() != nil:
		upd := q.Ast.GetUpdate()
		returning := upd.GetReturning()
		if len(returning) == 0 {
			return
		}
		tbl := catIdx.lookupTable(upd.GetTableName().GetName())
		aliasMap := make(map[string]*irv1.Table)
		if tbl != nil {
			aliasMap[strings.ToLower(upd.GetTableName().GetName())] = tbl
			if upd.GetAlias() != "" {
				aliasMap[strings.ToLower(upd.GetAlias())] = tbl
			}
		}
		var fromTables []*irv1.Table
		if tbl != nil {
			fromTables = []*irv1.Table{tbl}
		}
		for _, target := range returning {
			cols := resolveTarget(target.GetExpr(), target.GetAlias(), aliasMap, fromTables, catIdx, q.GetName(), d)
			q.Columns = append(q.Columns, cols...)
		}

	case q.Ast.GetDelete() != nil:
		del := q.Ast.GetDelete()
		returning := del.GetReturning()
		if len(returning) == 0 {
			return
		}
		tbl := catIdx.lookupTable(del.GetTableName().GetName())
		aliasMap := make(map[string]*irv1.Table)
		if tbl != nil {
			aliasMap[strings.ToLower(del.GetTableName().GetName())] = tbl
			if del.GetAlias() != "" {
				aliasMap[strings.ToLower(del.GetAlias())] = tbl
			}
		}
		var fromTables []*irv1.Table
		if tbl != nil {
			fromTables = []*irv1.Table{tbl}
		}
		for _, target := range returning {
			cols := resolveTarget(target.GetExpr(), target.GetAlias(), aliasMap, fromTables, catIdx, q.GetName(), d)
			q.Columns = append(q.Columns, cols...)
		}
	}
}

// inferInsertParamTypes attempts to assign types to INSERT VALUES parameters
// by matching them positionally to the INSERT column list against the table schema.
func inferInsertParamTypes(ins *irv1.InsertStmt, paramMap map[uint32]*pluginv1.QueryParameter, tbl *irv1.Table) {
	if tbl == nil || ins == nil {
		return
	}
	cols := ins.GetColumns()
	if len(cols) == 0 {
		return
	}
	vals := ins.GetValues()
	if vals == nil {
		return
	}
	// Build col→type lookup.
	colType := make(map[string]*irv1.TypeRef, len(tbl.GetColumns()))
	colNullable := make(map[string]bool, len(tbl.GetColumns()))
	for _, c := range tbl.GetColumns() {
		colType[strings.ToLower(c.GetName())] = c.GetType()
		colNullable[strings.ToLower(c.GetName())] = c.GetNullable()
	}
	for _, row := range vals.GetRows() {
		for i, expr := range row.GetValues() {
			if i >= len(cols) {
				break
			}
			param := expr.GetParameter()
			if param == nil {
				continue
			}
			pos := param.GetPosition()
			p, ok := paramMap[pos]
			if !ok || p.GetType() != nil {
				continue
			}
			colName := strings.ToLower(cols[i])
			if t, ok := colType[colName]; ok {
				p.Type = t
				p.Nullable = colNullable[colName]
				if p.Name == "" {
					p.Name = cols[i]
				}
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Catalog index
// ---------------------------------------------------------------------------

// catalogIndex maps lower-cased table name → *irv1.Table (first match wins).
type catalogIndex map[string]*irv1.Table

func buildCatalogIndex(cat *irv1.Catalog) catalogIndex {
	idx := make(catalogIndex)
	if cat == nil {
		return idx
	}
	for _, schema := range cat.GetSchemas() {
		for _, tbl := range schema.GetTables() {
			name := strings.ToLower(tbl.GetName().GetName())
			if _, exists := idx[name]; !exists {
				idx[name] = tbl
			}
		}
	}
	return idx
}

func (ci catalogIndex) lookupTable(name string) *irv1.Table {
	return ci[strings.ToLower(name)]
}

func (ci catalogIndex) lookupColumn(tbl *irv1.Table, colName string) *irv1.Column {
	if tbl == nil {
		return nil
	}
	lower := strings.ToLower(colName)
	for _, col := range tbl.GetColumns() {
		if strings.ToLower(col.GetName()) == lower {
			return col
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Parameter collection
// ---------------------------------------------------------------------------

// collectParams recursively walks an IR Statement and populates paramMap.
func collectParams(stmt *irv1.Statement, paramMap map[uint32]*pluginv1.QueryParameter) {
	if stmt == nil {
		return
	}
	switch {
	case stmt.GetSelect() != nil:
		collectParamsFromSelect(stmt.GetSelect(), paramMap)
	case stmt.GetInsert() != nil:
		ins := stmt.GetInsert()
		// INSERT has no WHERE clause; params come from VALUES / SELECT / RETURNING.
		if ins.GetQuery() != nil {
			collectParamsFromSelect(ins.GetQuery(), paramMap)
		}
		// VALUES
		if ins.GetValues() != nil {
			for _, row := range ins.GetValues().GetRows() {
				for _, e := range row.GetValues() {
					walkExpr(e, paramMap)
				}
			}
		}
		for _, t := range ins.GetReturning() {
			walkExpr(t.GetExpr(), paramMap)
		}
	case stmt.GetUpdate() != nil:
		upd := stmt.GetUpdate()
		collectParamsFromExprs(upd.GetWhere(), paramMap)
		for _, a := range upd.GetSet() {
			walkExpr(a.GetValue(), paramMap)
		}
		for _, t := range upd.GetReturning() {
			walkExpr(t.GetExpr(), paramMap)
		}
	case stmt.GetDelete() != nil:
		del := stmt.GetDelete()
		collectParamsFromExprs(del.GetWhere(), paramMap)
		for _, t := range del.GetReturning() {
			walkExpr(t.GetExpr(), paramMap)
		}
	}
}

func collectParamsFromSelect(sel *irv1.SelectStmt, paramMap map[uint32]*pluginv1.QueryParameter) {
	if sel == nil {
		return
	}
	if ss := sel.GetSelect(); ss != nil {
		collectParamsFromSimpleSelect(ss, paramMap)
	}
	if so := sel.GetSetOperation(); so != nil {
		collectParamsFromSelect(so.GetLeft(), paramMap)
		collectParamsFromSelect(so.GetRight(), paramMap)
	}
	walkExpr(sel.GetLimit(), paramMap)
	walkExpr(sel.GetOffset(), paramMap)
}

func collectParamsFromSimpleSelect(ss *irv1.SimpleSelect, paramMap map[uint32]*pluginv1.QueryParameter) {
	if ss == nil {
		return
	}
	for _, t := range ss.GetTargets() {
		walkExpr(t.GetExpr(), paramMap)
	}
	walkExpr(ss.GetWhere(), paramMap)
	for _, g := range ss.GetGroupBy() {
		walkExpr(g, paramMap)
	}
	walkExpr(ss.GetHaving(), paramMap)
	for _, d := range ss.GetDistinctOn() {
		walkExpr(d, paramMap)
	}
	// Walk FROM items: collect params from JOIN ON conditions and subqueries.
	collectParamsFromFromItems(ss.GetFrom(), paramMap)
}

// collectParamsFromFromItems walks a FROM clause recursively to collect params
// from JOIN ON conditions, FROM-subqueries, and FROM-functions.
func collectParamsFromFromItems(from []*irv1.FromItem, paramMap map[uint32]*pluginv1.QueryParameter) {
	for _, fi := range from {
		if fi == nil {
			continue
		}
		if jc := fi.GetJoin(); jc != nil {
			// Walk the ON condition.
			walkExpr(jc.GetOn(), paramMap)
			// Recurse into both join sides.
			collectParamsFromFromItems([]*irv1.FromItem{jc.GetLeft(), jc.GetRight()}, paramMap)
		}
		if sq := fi.GetSubquery(); sq != nil {
			collectParamsFromSelect(sq.GetQuery(), paramMap)
		}
		if fn := fi.GetFunction(); fn != nil {
			walkExpr(fn.GetCall(), paramMap)
		}
	}
}

func collectParamsFromExprs(e *irv1.Expr, paramMap map[uint32]*pluginv1.QueryParameter) {
	walkExpr(e, paramMap)
}

// walkExpr recursively visits an Expr and records any ParameterRef it finds.
func walkExpr(e *irv1.Expr, paramMap map[uint32]*pluginv1.QueryParameter) {
	if e == nil {
		return
	}
	switch {
	case e.GetParameter() != nil:
		pos := e.GetParameter().GetPosition()
		if pos > 0 {
			if _, exists := paramMap[pos]; !exists {
				paramMap[pos] = &pluginv1.QueryParameter{
					Number: pos,
					Name:   e.GetParameter().GetName(),
				}
			}
		}
	case e.GetOperator() != nil:
		for _, op := range e.GetOperator().GetOperands() {
			walkExpr(op, paramMap)
		}
	case e.GetFunctionCall() != nil:
		for _, arg := range e.GetFunctionCall().GetArguments() {
			walkExpr(arg, paramMap)
		}
		walkExpr(e.GetFunctionCall().GetFilter(), paramMap)
	case e.GetCaseExpr() != nil:
		ce := e.GetCaseExpr()
		walkExpr(ce.GetOperand(), paramMap)
		for _, w := range ce.GetWhens() {
			walkExpr(w.GetCondition(), paramMap)
			walkExpr(w.GetResult(), paramMap)
		}
		walkExpr(ce.GetElseResult(), paramMap)
	case e.GetCast() != nil:
		walkExpr(e.GetCast().GetExpr(), paramMap)
	case e.GetList() != nil:
		for _, item := range e.GetList().GetElements() {
			walkExpr(item, paramMap)
		}
	case e.GetSubquery() != nil:
		collectParamsFromSelect(e.GetSubquery().GetQuery(), paramMap)
	}
	// ColumnRef, Literal, Star, RawSql have no sub-expressions to walk.
}

// ---------------------------------------------------------------------------
// Parameter type inference
// ---------------------------------------------------------------------------

// inferParamTypes walks the statement AST looking for OperatorExpr nodes where
// one operand is a ParameterRef and another is a resolvable ColumnRef.
// When found, the param inherits the column's type.
func inferParamTypes(stmt *irv1.Statement, paramMap map[uint32]*pluginv1.QueryParameter, catIdx catalogIndex, qName string, d *catalog.Diagnostics) {
	if stmt == nil {
		return
	}
	// Build alias map from the statement's FROM clause (for select/update/delete).
	var aliasMap map[string]*irv1.Table
	switch {
	case stmt.GetSelect() != nil:
		if ss := stmt.GetSelect().GetSelect(); ss != nil {
			aliasMap = buildAliasMap(ss.GetFrom(), catIdx)
		}
	case stmt.GetUpdate() != nil:
		aliasMap = buildFromTableAlias(stmt.GetUpdate().GetTableName(), stmt.GetUpdate().GetAlias(), catIdx)
	case stmt.GetDelete() != nil:
		aliasMap = buildFromTableAlias(stmt.GetDelete().GetTableName(), stmt.GetDelete().GetAlias(), catIdx)
	case stmt.GetInsert() != nil:
		aliasMap = buildFromTableAlias(stmt.GetInsert().GetTableName(), stmt.GetInsert().GetAlias(), catIdx)
	}
	if aliasMap == nil {
		aliasMap = make(map[string]*irv1.Table)
	}

	inferParamTypesInStatement(stmt, paramMap, aliasMap, catIdx)
}

func buildFromTableAlias(qn *irv1.QualifiedName, alias string, catIdx catalogIndex) map[string]*irv1.Table {
	m := make(map[string]*irv1.Table)
	if qn == nil {
		return m
	}
	tbl := catIdx.lookupTable(qn.GetName())
	if tbl == nil {
		return m
	}
	tableName := strings.ToLower(qn.GetName())
	m[tableName] = tbl
	if alias != "" {
		m[strings.ToLower(alias)] = tbl
	}
	return m
}

func inferParamTypesInStatement(stmt *irv1.Statement, paramMap map[uint32]*pluginv1.QueryParameter, aliasMap map[string]*irv1.Table, catIdx catalogIndex) {
	if stmt == nil {
		return
	}
	switch {
	case stmt.GetSelect() != nil:
		inferParamTypesInSelect(stmt.GetSelect(), paramMap, aliasMap, catIdx)
	case stmt.GetUpdate() != nil:
		upd := stmt.GetUpdate()
		inferParamTypesInExpr(upd.GetWhere(), paramMap, aliasMap, catIdx)
		for _, a := range upd.GetSet() {
			inferParamTypesInExpr(a.GetValue(), paramMap, aliasMap, catIdx)
		}
	case stmt.GetDelete() != nil:
		inferParamTypesInExpr(stmt.GetDelete().GetWhere(), paramMap, aliasMap, catIdx)
	case stmt.GetInsert() != nil:
		ins := stmt.GetInsert()
		if ins.GetQuery() != nil {
			inferParamTypesInSelect(ins.GetQuery(), paramMap, aliasMap, catIdx)
		}
		if ins.GetValues() != nil {
			for _, row := range ins.GetValues().GetRows() {
				for _, e := range row.GetValues() {
					inferParamTypesInExpr(e, paramMap, aliasMap, catIdx)
				}
			}
		}
	}
}

func inferParamTypesInSelect(sel *irv1.SelectStmt, paramMap map[uint32]*pluginv1.QueryParameter, aliasMap map[string]*irv1.Table, catIdx catalogIndex) {
	if sel == nil {
		return
	}
	if ss := sel.GetSelect(); ss != nil {
		inferParamTypesInExpr(ss.GetWhere(), paramMap, aliasMap, catIdx)
		for _, t := range ss.GetTargets() {
			inferParamTypesInExpr(t.GetExpr(), paramMap, aliasMap, catIdx)
		}
		for _, g := range ss.GetGroupBy() {
			inferParamTypesInExpr(g, paramMap, aliasMap, catIdx)
		}
		inferParamTypesInExpr(ss.GetHaving(), paramMap, aliasMap, catIdx)
		// Walk FROM items for JOIN ON conditions and subqueries.
		inferParamTypesInFromItems(ss.GetFrom(), paramMap, aliasMap, catIdx)
	}
	if so := sel.GetSetOperation(); so != nil {
		inferParamTypesInSelect(so.GetLeft(), paramMap, aliasMap, catIdx)
		inferParamTypesInSelect(so.GetRight(), paramMap, aliasMap, catIdx)
	}
}

// inferParamTypesInFromItems walks FROM items to infer param types from JOIN ON
// conditions and FROM-subqueries.
func inferParamTypesInFromItems(from []*irv1.FromItem, paramMap map[uint32]*pluginv1.QueryParameter, aliasMap map[string]*irv1.Table, catIdx catalogIndex) {
	for _, fi := range from {
		if fi == nil {
			continue
		}
		if jc := fi.GetJoin(); jc != nil {
			inferParamTypesInExpr(jc.GetOn(), paramMap, aliasMap, catIdx)
			inferParamTypesInFromItems([]*irv1.FromItem{jc.GetLeft(), jc.GetRight()}, paramMap, aliasMap, catIdx)
		}
		if sq := fi.GetSubquery(); sq != nil {
			inferParamTypesInSelect(sq.GetQuery(), paramMap, aliasMap, catIdx)
		}
		if fn := fi.GetFunction(); fn != nil {
			inferParamTypesInExpr(fn.GetCall(), paramMap, aliasMap, catIdx)
		}
	}
}

// inferParamTypesInExpr traverses an Expr looking for OperatorExpr nodes
// where one side is a param and the other is a resolvable column. When found,
// it sets the param's Type from the column.
func inferParamTypesInExpr(e *irv1.Expr, paramMap map[uint32]*pluginv1.QueryParameter, aliasMap map[string]*irv1.Table, catIdx catalogIndex) {
	if e == nil {
		return
	}
	if op := e.GetOperator(); op != nil {
		operands := op.GetOperands()
		// Try to find (param, colref) or (colref, param) pairs.
		if len(operands) == 2 {
			tryInferParamFromPair(operands[0], operands[1], paramMap, aliasMap, catIdx)
			tryInferParamFromPair(operands[1], operands[0], paramMap, aliasMap, catIdx)
		}
		// Recurse into all operands regardless.
		for _, operand := range operands {
			inferParamTypesInExpr(operand, paramMap, aliasMap, catIdx)
		}
		return
	}
	// Recurse into sub-expressions.
	if fc := e.GetFunctionCall(); fc != nil {
		for _, arg := range fc.GetArguments() {
			inferParamTypesInExpr(arg, paramMap, aliasMap, catIdx)
		}
		inferParamTypesInExpr(fc.GetFilter(), paramMap, aliasMap, catIdx)
		return
	}
	if ce := e.GetCaseExpr(); ce != nil {
		inferParamTypesInExpr(ce.GetOperand(), paramMap, aliasMap, catIdx)
		for _, w := range ce.GetWhens() {
			inferParamTypesInExpr(w.GetCondition(), paramMap, aliasMap, catIdx)
			inferParamTypesInExpr(w.GetResult(), paramMap, aliasMap, catIdx)
		}
		inferParamTypesInExpr(ce.GetElseResult(), paramMap, aliasMap, catIdx)
		return
	}
	if cast := e.GetCast(); cast != nil {
		inferParamTypesInExpr(cast.GetExpr(), paramMap, aliasMap, catIdx)
		return
	}
	if lst := e.GetList(); lst != nil {
		for _, item := range lst.GetElements() {
			inferParamTypesInExpr(item, paramMap, aliasMap, catIdx)
		}
		return
	}
	if sq := e.GetSubquery(); sq != nil {
		inferParamTypesInSelect(sq.GetQuery(), paramMap, aliasMap, catIdx)
		return
	}
}

// tryInferParamFromPair: if `maybeParam` is a ParameterRef and `maybeCol` is
// a ColumnRef, resolve the column from aliasMap/catIdx and assign its type to
// the parameter (if not already set).
func tryInferParamFromPair(maybeParam, maybeCol *irv1.Expr, paramMap map[uint32]*pluginv1.QueryParameter, aliasMap map[string]*irv1.Table, catIdx catalogIndex) {
	if maybeParam == nil || maybeCol == nil {
		return
	}
	param := maybeParam.GetParameter()
	if param == nil {
		return
	}
	colRef := maybeCol.GetColumnRef()
	if colRef == nil {
		return
	}
	pos := param.GetPosition()
	if pos == 0 {
		return
	}
	p, ok := paramMap[pos]
	if !ok || p.GetType() != nil {
		return // already resolved
	}

	col := resolveColumnRef(colRef, aliasMap, catIdx)
	if col == nil {
		return
	}
	p.Type = col.GetType()
	p.Nullable = col.GetNullable()
	p.Column = &irv1.ObjectRef{
		Id:   col.GetId(),
		Kind: irv1.ObjectKind_OBJECT_KIND_COLUMN,
	}
}

// resolveColumnRef looks up a ColumnRef in the aliasMap/catIdx.
func resolveColumnRef(cr *irv1.ColumnRef, aliasMap map[string]*irv1.Table, catIdx catalogIndex) *irv1.Column {
	if cr == nil {
		return nil
	}
	colName := cr.GetColumn()
	qualifier := cr.GetQualifier()

	if qualifier != "" {
		// Qualified: look up the table by alias/name.
		tbl := aliasMap[strings.ToLower(qualifier)]
		if tbl == nil {
			tbl = catIdx.lookupTable(qualifier)
		}
		return catIdx.lookupColumn(tbl, colName)
	}

	// Unqualified: try each table in aliasMap.
	for _, tbl := range aliasMap {
		col := catIdx.lookupColumn(tbl, colName)
		if col != nil {
			return col
		}
	}

	// Last resort: search all tables in stable (sorted) order to ensure
	// deterministic resolution when the column name is ambiguous.
	tableNames := make([]string, 0, len(catIdx))
	for name := range catIdx {
		tableNames = append(tableNames, name)
	}
	sort.Strings(tableNames)
	for _, name := range tableNames {
		col := catIdx.lookupColumn(catIdx[name], colName)
		if col != nil {
			return col
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Column inference
// ---------------------------------------------------------------------------

// buildAliasMap builds alias → *irv1.Table from a FROM clause.
func buildAliasMap(from []*irv1.FromItem, catIdx catalogIndex) map[string]*irv1.Table {
	m := make(map[string]*irv1.Table)
	for _, fi := range from {
		if fi == nil {
			continue
		}
		if tr := fi.GetTable(); tr != nil {
			tblName := tr.GetName().GetName()
			alias := tr.GetAlias()
			tbl := catIdx.lookupTable(tblName)
			key := strings.ToLower(tblName)
			if tbl != nil {
				m[key] = tbl
			}
			if alias != "" {
				m[strings.ToLower(alias)] = tbl
			}
		}
		// For joins, recurse into left/right.
		if jc := fi.GetJoin(); jc != nil {
			for k, v := range buildAliasMap([]*irv1.FromItem{jc.GetLeft(), jc.GetRight()}, catIdx) {
				m[k] = v
			}
		}
	}
	return m
}

// collectJoinTables appends tables found in JOIN clauses to the provided slice.
// For each join side that is a TableRef it resolves and appends the table;
// for nested joins it recurses so that arbitrarily-deep join trees are handled.
func collectJoinTables(from []*irv1.FromItem, catIdx catalogIndex, out []*irv1.Table) []*irv1.Table {
	for _, fi := range from {
		if fi == nil {
			continue
		}
		if jc := fi.GetJoin(); jc != nil {
			// Resolve left and right sides as concrete tables when possible.
			for _, side := range []*irv1.FromItem{jc.GetLeft(), jc.GetRight()} {
				if side == nil {
					continue
				}
				if side.GetJoin() != nil {
					// Nested join — recurse.
					out = collectJoinTables([]*irv1.FromItem{side}, catIdx, out)
				} else {
					// TableRef, SubqueryRef, etc.
					if tbl := resolveFromItemTable(side, catIdx); tbl != nil {
						out = append(out, tbl)
					}
				}
			}
		}
	}
	return out
}

// resolveFromItemTable returns the *irv1.Table for a TableRef FromItem.
func resolveFromItemTable(fi *irv1.FromItem, catIdx catalogIndex) *irv1.Table {
	if fi == nil {
		return nil
	}
	if tr := fi.GetTable(); tr != nil {
		return catIdx.lookupTable(tr.GetName().GetName())
	}
	return nil
}

// resolveTarget converts one SelectTarget to zero or more QueryColumns.
func resolveTarget(expr *irv1.Expr, alias string, aliasMap map[string]*irv1.Table, fromTables []*irv1.Table, catIdx catalogIndex, qName string, d *catalog.Diagnostics) []*pluginv1.QueryColumn {
	if expr == nil {
		colName := alias
		if colName == "" {
			colName = "column"
		}
		return []*pluginv1.QueryColumn{{Name: colName}}
	}

	// ColumnRef
	if cr := expr.GetColumnRef(); cr != nil {
		colName := cr.GetColumn()
		qualifier := cr.GetQualifier()
		displayName := alias
		if displayName == "" {
			displayName = colName
		}

		col := resolveColumnRef(cr, aliasMap, catIdx)
		if col == nil {
			d.Add("info", fmt.Sprintf("column %q unresolved in query %s", colName, qName))
			return []*pluginv1.QueryColumn{{Name: displayName}}
		}
		return []*pluginv1.QueryColumn{{
			Name:     displayName,
			Type:     col.GetType(),
			Nullable: col.GetNullable(),
			SourceColumn: &irv1.ObjectRef{
				Id:   col.GetId(),
				Kind: irv1.ObjectKind_OBJECT_KIND_COLUMN,
			},
			TableAlias: qualifier,
		}}
	}

	// StarExpr
	if expr.GetStar() != nil {
		qualifier := expr.GetStar().GetQualifier()
		// Determine which table to expand.
		if qualifier != "" {
			tbl := aliasMap[strings.ToLower(qualifier)]
			if tbl == nil {
				tbl = catIdx.lookupTable(qualifier)
			}
			if tbl != nil {
				return expandTableColumns(tbl, qualifier)
			}
			d.Add("info", fmt.Sprintf("star qualifier %q unresolved in query %s", qualifier, qName))
			return nil
		}
		// Unqualified star: expand all from tables.
		if len(fromTables) == 0 {
			d.Add("info", fmt.Sprintf("star with no from tables in query %s", qName))
			return nil
		}
		var cols []*pluginv1.QueryColumn
		for _, tbl := range fromTables {
			cols = append(cols, expandTableColumns(tbl, "")...)
		}
		return cols
	}

	// Literal
	if lit := expr.GetLiteral(); lit != nil {
		colName := alias
		if colName == "" {
			colName = "column"
		}
		return []*pluginv1.QueryColumn{{
			Name: colName,
			Type: lit.GetType(),
		}}
	}

	// Other (function call, cast, operator, etc.) – return a column with alias or "column".
	colName := alias
	if colName == "" {
		colName = "column"
	}
	d.Add("info", fmt.Sprintf("column expression type unresolved in query %s", qName))
	return []*pluginv1.QueryColumn{{Name: colName}}
}

// expandTableColumns produces one QueryColumn per column in tbl.
func expandTableColumns(tbl *irv1.Table, tableAlias string) []*pluginv1.QueryColumn {
	if tbl == nil {
		return nil
	}
	cols := make([]*pluginv1.QueryColumn, 0, len(tbl.GetColumns()))
	for _, col := range tbl.GetColumns() {
		cols = append(cols, &pluginv1.QueryColumn{
			Name:     col.GetName(),
			Type:     col.GetType(),
			Nullable: col.GetNullable(),
			SourceColumn: &irv1.ObjectRef{
				Id:   col.GetId(),
				Kind: irv1.ObjectKind_OBJECT_KIND_COLUMN,
			},
			TableAlias: tableAlias,
		})
	}
	return cols
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// orderedParams returns parameters sorted by position.
func orderedParams(paramMap map[uint32]*pluginv1.QueryParameter) []*pluginv1.QueryParameter {
	if len(paramMap) == 0 {
		return nil
	}
	// Find max position.
	var maxPos uint32
	for pos := range paramMap {
		if pos > maxPos {
			maxPos = pos
		}
	}
	out := make([]*pluginv1.QueryParameter, 0, len(paramMap))
	for i := uint32(1); i <= maxPos; i++ {
		if p, ok := paramMap[i]; ok {
			out = append(out, p)
		}
	}
	return out
}
