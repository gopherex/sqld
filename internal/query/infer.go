package query

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gopherex/sqld/internal/catalog"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/reflect/protopath"
	"google.golang.org/protobuf/reflect/protorange"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Infer mutates q by:
//  1. Walking the AST to collect parameters and their explicit cast types.
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
	// 1. Collect all parameter refs and explicit cast types from the full AST.
	// ------------------------------------------------------------------ //
	// Seed paramMap from any parameters already set on the query (e.g. named
	// params seeded by ParseQueries via rewriteNamedParams).  This ensures
	// that Infer fills in Type/Nullable/Column on the existing entries rather
	// than creating duplicates, so the original Name is preserved.
	paramMap := make(map[uint32]*pluginv1.QueryParameter) // keyed by position
	for _, p := range q.GetParameters() {
		if p != nil && p.GetNumber() > 0 {
			paramMap[p.GetNumber()] = p
		}
	}
	collectParams(q.Ast, paramMap)

	// ------------------------------------------------------------------ //
	// 2. Shallow type inference via operator context.
	// ------------------------------------------------------------------ //
	// For each operator expr we check whether one side is a ParameterRef and
	// the other is a ColumnRef; if so, resolve the column and assign its type
	// and (for positional params) its name.
	inference := inferParamTypes(q.Ast, paramMap, catIdx, q.GetName(), d)
	output := inference.output
	// Keep result types in the same lexical scopes as parameter inference.
	// The existing target resolver still provides names, nullability and metadata.
	defer func() {
		if len(output) != len(q.GetColumns()) {
			return
		}
		for n, col := range output {
			if col.GetType() != nil {
				q.Columns[n].Type = col.GetType()
				if col.GetId() != "" {
					q.Columns[n].SourceColumn = &irv1.ObjectRef{Id: col.GetId(), Kind: irv1.ObjectKind_OBJECT_KIND_COLUMN}
				}
			}
		}
	}()

	// Infer positional param names from UPDATE SET assignments.
	if q.Ast.GetUpdate() != nil {
		inferUpdateSetParamNames(q.Ast.GetUpdate(), paramMap, catIdx)
	}
	if ins := q.Ast.GetInsert(); ins != nil {
		tbl := catIdx.lookupTable(ins.GetTableName().GetName())
		inferInsertParamTypes(ins, paramMap, tbl)
	}

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
		aliasMap := inference.root.tables

		// Collect all tables referenced (for star expansion).
		var fromTables []*irv1.Table
		for _, name := range inference.root.order {
			tbl := aliasMap[name]
			if tbl != nil {
				fromTables = append(fromTables, tbl)
			}
		}

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
	// Build column lookup.
	columns := make(map[string]*irv1.Column, len(tbl.GetColumns()))
	for _, c := range tbl.GetColumns() {
		columns[strings.ToLower(c.GetName())] = c
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
			if col, ok := columns[colName]; ok {
				p.Type = col.GetType()
				p.Nullable = col.GetNullable()
				p.Column = &irv1.ObjectRef{Id: col.GetId(), Kind: irv1.ObjectKind_OBJECT_KIND_COLUMN}
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

// collectParams collects parameters and their explicit casts before contextual
// inference. A protobuf traversal covers every structured clause, including
// CTEs, RETURNING, ON CONFLICT, ORDER BY and window expressions.
func collectParams(stmt *irv1.Statement, paramMap map[uint32]*pluginv1.QueryParameter) {
	if stmt == nil {
		return
	}
	// The IR contains no Any messages, and the visitor never returns an error.
	// Stable field order also makes repeated parameter occurrences deterministic.
	_ = (protorange.Options{Stable: true}).Range(stmt.ProtoReflect(), func(path protopath.Values) error {
		msg, ok := path.Index(-1).Value.Interface().(protoreflect.Message)
		if !ok {
			return nil
		}
		switch node := msg.Interface().(type) {
		case *irv1.ParameterRef:
			collectParam(node, paramMap)
		case *irv1.CastExpr:
			// Only a direct cast constrains a parameter's input type. For
			// ($1::int)::text the inner cast wins; a cast of a function result
			// must not be applied to the function's arguments.
			ref := node.GetExpr().GetParameter()
			if ref != nil && node.GetTargetType() != nil {
				if p := collectParam(ref, paramMap); p != nil && p.GetType() == nil {
					p.Type = node.GetTargetType()
					// A cast alone does not reject NULL input. In particular,
					// COALESCE($1::text, not_null_column) must accept nil.
					p.Nullable = true
				}
			}
		}
		return nil
	}, nil)
}

func collectParam(ref *irv1.ParameterRef, paramMap map[uint32]*pluginv1.QueryParameter) *pluginv1.QueryParameter {
	pos := ref.GetPosition()
	if pos == 0 {
		return nil
	}
	p := paramMap[pos]
	if p == nil {
		p = &pluginv1.QueryParameter{Number: pos, Name: ref.GetName()}
		paramMap[pos] = p
	}
	return p
}

// ---------------------------------------------------------------------------
// Parameter type inference
// ---------------------------------------------------------------------------

// inferUpdateSetParamNames assigns Name (and Type/Nullable when not already set)
// to positional parameters that appear directly as the value in an UPDATE SET
// assignment. For example: UPDATE t SET status = $2 → param $2 gets Name "status".
func inferUpdateSetParamNames(upd *irv1.UpdateStmt, paramMap map[uint32]*pluginv1.QueryParameter, catIdx catalogIndex) {
	if upd == nil {
		return
	}
	tbl := catIdx.lookupTable(upd.GetTableName().GetName())
	for _, assign := range upd.GetSet() {
		cols := assign.GetColumns()
		if len(cols) != 1 {
			continue
		}
		val := assign.GetValue()
		if val == nil {
			continue
		}
		param := val.GetParameter()
		if param == nil {
			continue
		}
		pos := param.GetPosition()
		if pos == 0 {
			continue
		}
		p, ok := paramMap[pos]
		if !ok {
			continue
		}
		colName := cols[0]
		// Assign the name from the SET column only when the param is not already
		// named (named params like @address arrive pre-named via rewriteNamedParams).
		if p.GetName() == "" {
			p.Name = colName
		}
		// Assign type/nullable from the catalog column when not yet set. This runs
		// even for already-named params so that composite/UDT SET targets (e.g.
		// `SET address = @address`) get the column's composite TypeRef.
		if p.GetType() == nil && tbl != nil {
			col := catIdx.lookupColumn(tbl, colName)
			if col != nil {
				p.Type = col.GetType()
				p.Nullable = col.GetNullable()
				p.Column = &irv1.ObjectRef{
					Id:   col.GetId(),
					Kind: irv1.ObjectKind_OBJECT_KIND_COLUMN,
				}
			}
		}
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
			// The ColumnRef didn't resolve against the catalog. Fall back to
			// best-effort expression-type inference before giving up.
			if ty := inferExprType(expr, columnResolver(aliasMap, catIdx)); ty != nil {
				return []*pluginv1.QueryColumn{{
					Name:     displayName,
					Type:     ty,
					Nullable: true,
				}}
			}
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

	// Any other expression (cast, literal, function call, operator, …):
	// run best-effort type inference.
	resolve := columnResolver(aliasMap, catIdx)
	ty := inferExprType(expr, resolve)
	colName := alias
	if colName == "" {
		colName = defaultColumnName(expr)
	}
	if ty == nil {
		d.Add("info", fmt.Sprintf("column expression type unresolved in query %s", qName))
		return []*pluginv1.QueryColumn{{Name: colName}}
	}
	return []*pluginv1.QueryColumn{{
		Name:     colName,
		Type:     ty,
		Nullable: exprNullable(expr),
	}}
}

// columnResolver adapts the alias map / catalog index into the
// (qualifier, col) → *irv1.Column signature used by inferExprType.
func columnResolver(aliasMap map[string]*irv1.Table, catIdx catalogIndex) func(qualifier, col string) *irv1.Column {
	return func(qualifier, col string) *irv1.Column {
		return resolveColumnRef(&irv1.ColumnRef{Qualifier: qualifier, Column: col}, aliasMap, catIdx)
	}
}

// defaultColumnName picks a sensible name for an unaliased non-column target:
// the (lowercased, last) function name for a function call, else "column".
func defaultColumnName(expr *irv1.Expr) string {
	if fc := expr.GetFunctionCall(); fc != nil {
		if name := funcName(fc); name != "" {
			return name
		}
	}
	return "column"
}

// exprNullable returns a best-effort nullability for a non-column target.
// Comparison/logical operators and count(*) are NOT NULL; aggregates and
// everything else default to nullable=true.
func exprNullable(expr *irv1.Expr) bool {
	if op := expr.GetOperator(); op != nil {
		if isBoolOperator(op.GetSymbol()) {
			return false
		}
		return true
	}
	if fc := expr.GetFunctionCall(); fc != nil {
		switch funcName(fc) {
		case "count":
			return false
		}
		return true
	}
	// Casts and literals: conservatively nullable=false for literals,
	// nullable for casts of unknown source.
	if expr.GetLiteral() != nil {
		// A non-null literal cannot be NULL.
		return expr.GetLiteral().GetNullValue()
	}
	return true
}

// ---------------------------------------------------------------------------
// Expression type inference
// ---------------------------------------------------------------------------

// scalarType builds a SCALAR TypeRef for a built-in PG type name.
func scalarType(pgName string) *irv1.TypeRef {
	return &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: pgName}
}

// funcName returns the lowercased, last-component name of a function call,
// e.g. "pg_catalog.count" → "count".
func funcName(fc *irv1.FunctionCall) string {
	if fc == nil {
		return ""
	}
	return strings.ToLower(fc.GetName().GetName())
}

// isBoolOperator reports whether the operator symbol yields a boolean result.
func isBoolOperator(sym string) bool {
	if strings.HasSuffix(strings.ToUpper(sym), " ANY") || strings.HasSuffix(strings.ToUpper(sym), " ALL") {
		return true
	}
	switch strings.ToUpper(strings.TrimSpace(sym)) {
	case "=", "<", ">", "<=", ">=", "<>", "!=",
		"AND", "OR", "NOT",
		"IS NULL", "IS NOT NULL", "IS TRUE", "IS FALSE",
		"IS DISTINCT FROM", "IS NOT DISTINCT FROM",
		"LIKE", "NOT LIKE", "ILIKE", "NOT ILIKE",
		"SIMILAR TO", "NOT SIMILAR TO",
		"IN", "NOT IN", "BETWEEN", "NOT BETWEEN", "EXISTS",
		"@>", "<@", "&&", "?", "?|", "?&", "~", "~*", "!~", "!~*", "@@":
		return true
	}
	return false
}

// isArithmeticOperator reports whether the operator symbol is arithmetic.
func isArithmeticOperator(sym string) bool {
	switch strings.TrimSpace(sym) {
	case "+", "-", "*", "/", "%", "^":
		return true
	}
	return false
}

// numericRank orders numeric-ish PG types from narrowest to widest so that
// arithmetic over two operands picks the wider type. Non-numeric types rank 0.
func numericRank(pgName string) int {
	switch pgName {
	case "int2":
		return 1
	case "int4":
		return 2
	case "int8":
		return 3
	case "numeric":
		return 4
	case "float4":
		return 5
	case "float8":
		return 6
	}
	return 0
}

// inferExprType performs best-effort type inference for a SELECT/RETURNING
// target expression that is not a bare, catalog-resolvable column. It returns
// the inferred TypeRef, or nil when the type cannot be determined (the caller
// then leaves QueryColumn.Type nil, preserving the legacy `any` behavior).
//
// `resolve` maps a (qualifier, column) pair to its catalog *irv1.Column and may
// be nil. inferExprType never panics.
func inferExprType(e *irv1.Expr, resolve func(qualifier, col string) *irv1.Column) *irv1.TypeRef {
	if e == nil {
		return nil
	}

	switch {
	// ---------------------------------------------------------------- //
	// Cast — highest confidence: the target type is explicit.
	// ---------------------------------------------------------------- //
	case e.GetCast() != nil:
		return e.GetCast().GetTargetType()

	// ---------------------------------------------------------------- //
	// Literal — derive from the value kind.
	// ---------------------------------------------------------------- //
	case e.GetLiteral() != nil:
		lit := e.GetLiteral()
		if lit.GetType() != nil {
			return lit.GetType()
		}
		switch lit.GetValue().(type) {
		case *irv1.Literal_IntValue:
			return scalarType("int4")
		case *irv1.Literal_FloatValue, *irv1.Literal_NumericValue:
			return scalarType("numeric")
		case *irv1.Literal_StringValue:
			return scalarType("text")
		case *irv1.Literal_BoolValue:
			return scalarType("bool")
		case *irv1.Literal_ByteaValue:
			return scalarType("bytea")
		case *irv1.Literal_NullValue:
			return nil
		}
		return nil

	// ---------------------------------------------------------------- //
	// ColumnRef — resolve against the catalog if a resolver is provided.
	// ---------------------------------------------------------------- //
	case e.GetColumnRef() != nil:
		if resolve == nil {
			return nil
		}
		cr := e.GetColumnRef()
		if col := resolve(cr.GetQualifier(), cr.GetColumn()); col != nil {
			return col.GetType()
		}
		return nil

	// ---------------------------------------------------------------- //
	// FunctionCall — map by (lowercased, last-part) function name.
	// ---------------------------------------------------------------- //
	case e.GetFunctionCall() != nil:
		return inferFunctionType(e.GetFunctionCall(), resolve)
	case e.GetCaseExpr() != nil:
		c := e.GetCaseExpr()
		args := []*irv1.Expr{c.GetElseResult()}
		for _, w := range c.GetWhens() {
			args = append(args, w.GetResult())
		}
		return commonExprType(args, resolve)

	// ---------------------------------------------------------------- //
	// OperatorExpr — boolean for comparisons/logical; recurse for arithmetic.
	// ---------------------------------------------------------------- //
	case e.GetOperator() != nil:
		op := e.GetOperator()
		sym := op.GetSymbol()
		if isBoolOperator(sym) {
			return scalarType("bool")
		}
		if isArithmeticOperator(sym) {
			return inferArithmeticType(op, resolve)
		}
		return nil
	}

	return nil
}

// inferFunctionType maps a function call to its result type.
func inferFunctionType(fc *irv1.FunctionCall, resolve func(qualifier, col string) *irv1.Column) *irv1.TypeRef {
	name := funcName(fc)
	args := fc.GetArguments()

	// arg0Type resolves the type of the first argument, if present.
	arg0Type := func() *irv1.TypeRef {
		if len(args) == 0 {
			return nil
		}
		return inferExprType(args[0], resolve)
	}

	switch name {
	// --- Aggregates ---------------------------------------------------- //
	case "count":
		return scalarType("int8")
	case "sum":
		t := arg0Type()
		switch typePgName(t) {
		case "int2", "int4":
			return scalarType("int8")
		case "int8", "numeric":
			return scalarType("numeric")
		case "float4":
			return scalarType("float4")
		case "float8":
			return scalarType("float8")
		}
		return scalarType("numeric")
	case "avg":
		switch typePgName(arg0Type()) {
		case "float4", "float8":
			return scalarType("float8")
		}
		return scalarType("numeric")
	case "min", "max":
		// min/max return the argument's own type.
		return arg0Type()
	case "bool_and", "bool_or":
		return scalarType("bool")
	case "string_agg":
		return scalarType("text")
	case "array_agg":
		elem := arg0Type()
		if elem == nil {
			return nil
		}
		return &irv1.TypeRef{
			Kind:            irv1.TypeKind_TYPE_KIND_ARRAY,
			PgName:          elem.GetPgName(),
			ArrayDimensions: 1,
			Element:         elem,
		}
	case "jsonb_agg":
		return scalarType("jsonb")
	case "json_agg":
		return scalarType("json")

	// --- Common scalar functions → text ------------------------------- //
	case "lower", "upper", "trim", "ltrim", "rtrim", "btrim",
		"concat", "concat_ws", "md5", "to_char", "substr", "substring",
		"replace", "initcap", "repeat", "reverse", "left", "right":
		return scalarType("text")

	// --- Length / position → int4 -------------------------------------- //
	case "length", "char_length", "character_length",
		"octet_length", "bit_length", "position", "strpos",
		"cardinality", "array_length":
		return scalarType("int4")

	// --- Date/time ----------------------------------------------------- //
	case "now", "current_timestamp", "clock_timestamp",
		"statement_timestamp", "transaction_timestamp":
		return scalarType("timestamptz")
	case "current_date":
		return scalarType("date")
	case "current_time", "localtime":
		return scalarType("time")
	case "localtimestamp":
		return scalarType("timestamp")

	// --- Pass-through over first argument ------------------------------ //
	case "coalesce", "greatest", "least":
		return commonExprType(args, resolve)
	case "nullif", "abs", "ceil", "ceiling", "floor", "round", "trunc", "sign", "mod":
		return arg0Type()

	// --- Misc ---------------------------------------------------------- //
	case "gen_random_uuid", "uuid_generate_v4":
		return scalarType("uuid")
	}

	// Unknown function → nil (stays `any`).
	return nil
}

// inferArithmeticType recurses into an arithmetic operator's operands and
// returns the wider numeric-ish operand type. Falls back to int4 when both
// operands are integers but their exact width is unknown.
func inferArithmeticType(op *irv1.OperatorExpr, resolve func(qualifier, col string) *irv1.Column) *irv1.TypeRef {
	var best *irv1.TypeRef
	bestRank := 0
	known := false
	for _, operand := range op.GetOperands() {
		t := inferExprType(operand, resolve)
		if t == nil {
			continue
		}
		known = true
		if r := numericRank(t.GetPgName()); r > bestRank {
			bestRank = r
			best = t
		}
	}
	if best != nil {
		return best
	}
	if known {
		// Operands resolved but none are numeric-ish; default to int4.
		return scalarType("int4")
	}
	return nil
}

// typePgName safely returns a TypeRef's PgName ("" when nil).
func typePgName(t *irv1.TypeRef) string {
	if t == nil {
		return ""
	}
	return t.GetPgName()
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
