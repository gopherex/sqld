package query

import (
	"fmt"
	"strings"

	"github.com/gopherex/sqld/internal/catalog"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

// Parameter inference has its own lexical scopes. Catalog-wide column fallback
// is not valid here: it can silently give a bind parameter an unrelated type.
type paramScope struct {
	parent *paramScope
	tables map[string]*irv1.Table
	ctes   map[string]*irv1.Table
	order  []string
}

type paramInference struct {
	params   map[uint32]*pluginv1.QueryParameter
	catalog  catalogIndex
	query    string
	diags    *catalog.Diagnostics
	reported map[string]bool
	output   []*irv1.Column
	root     *paramScope
}

func inferParamTypes(stmt *irv1.Statement, params map[uint32]*pluginv1.QueryParameter, cat catalogIndex, name string, d *catalog.Diagnostics) *paramInference {
	i := &paramInference{params: params, catalog: cat, query: name, diags: d, reported: make(map[string]bool)}
	i.statement(stmt)
	return i
}

func (i *paramInference) diagnostic(message string) {
	if !i.reported[message] {
		i.reported[message] = true
		i.diags.Add("warning", message+" in query "+i.query)
	}
}

func (i *paramInference) scope(parent *paramScope, with []*irv1.CommonTableExpr) *paramScope {
	s := &paramScope{parent: parent, tables: make(map[string]*irv1.Table), ctes: make(map[string]*irv1.Table)}
	if parent != nil {
		for k, v := range parent.ctes {
			s.ctes[k] = v
		}
	}
	for _, cte := range with {
		cols := i.selectQuery(cte.GetQuery(), s)
		for n, name := range cte.GetColumns() {
			if n < len(cols) {
				cols[n].Name = name
			}
		}
		s.ctes[strings.ToLower(cte.GetName())] = &irv1.Table{Columns: cols}
	}
	return s
}

func (i *paramInference) column(cr *irv1.ColumnRef, s *paramScope) *irv1.Column {
	if cr == nil {
		return nil
	}
	for scope := s; scope != nil; scope = scope.parent {
		if cr.GetQualifier() != "" {
			if tbl, exists := scope.tables[strings.ToLower(cr.GetQualifier())]; exists {
				col := i.catalog.lookupColumn(tbl, cr.GetColumn())
				if col == nil {
					i.diagnostic("column " + cr.GetQualifier() + "." + cr.GetColumn() + " unresolved")
				}
				return col // A local alias shadows the outer alias even if this column is absent.
			}
			continue
		}
		var found *irv1.Column
		for _, tbl := range scope.tables {
			if col := i.catalog.lookupColumn(tbl, cr.GetColumn()); col != nil {
				if found != nil {
					i.diagnostic("column " + cr.GetColumn() + " is ambiguous")
					return nil
				}
				found = col
			}
		}
		if found != nil {
			return found
		}
	}
	i.diagnostic("column " + cr.GetQualifier() + "." + cr.GetColumn() + " unresolved")
	return nil
}

func (i *paramInference) addTable(s *paramScope, name *irv1.QualifiedName, alias string) *irv1.Table {
	key := strings.ToLower(name.GetName())
	tbl := s.ctes[key]
	if tbl == nil {
		tbl = i.catalog.lookupTable(name.GetName())
	}
	if alias != "" {
		key = strings.ToLower(alias)
	}
	s.tables[key] = tbl // Do not expose the original name when SQL supplies an alias.
	s.order = append(s.order, key)
	return tbl
}

func (i *paramInference) from(items []*irv1.FromItem, s *paramScope) {
	for _, item := range items {
		switch {
		case item.GetTable() != nil:
			t := item.GetTable()
			i.addTable(s, t.GetName(), t.GetAlias())
		case item.GetJoin() != nil:
			j := item.GetJoin()
			start := len(s.order)
			i.from([]*irv1.FromItem{j.GetLeft()}, s)
			middle := len(s.order)
			i.from([]*irv1.FromItem{j.GetRight()}, s)
			i.expr(j.GetOn(), s, scalarType("bool"), nil)
			if j.GetType() == irv1.JoinType_JOIN_TYPE_LEFT || j.GetType() == irv1.JoinType_JOIN_TYPE_FULL {
				nullExtend(s, s.order[middle:])
			}
			if j.GetType() == irv1.JoinType_JOIN_TYPE_RIGHT || j.GetType() == irv1.JoinType_JOIN_TYPE_FULL {
				nullExtend(s, s.order[start:middle])
			}
		case item.GetSubquery() != nil:
			q := item.GetSubquery()
			// A non-LATERAL FROM subquery cannot see siblings in this FROM.
			parent := &paramScope{parent: s.parent, ctes: s.ctes}
			if q.GetLateral() {
				parent = s
			}
			cols := i.selectQuery(q.GetQuery(), parent)
			for n, name := range q.GetColumnAliases() {
				if n < len(cols) {
					cols[n].Name = name
				}
			}
			s.tables[strings.ToLower(q.GetAlias())] = &irv1.Table{Columns: cols}
			s.order = append(s.order, strings.ToLower(q.GetAlias()))
		case item.GetFunction() != nil:
			i.expr(item.GetFunction().GetCall(), s, nil, nil)
		case item.GetValues() != nil:
			v := item.GetValues()
			cols := i.values(v.GetValues(), s, nil)
			for n, name := range v.GetColumnAliases() {
				if n < len(cols) {
					cols[n].Name = name
				}
			}
			s.tables[strings.ToLower(v.GetAlias())] = &irv1.Table{Columns: cols}
			s.order = append(s.order, strings.ToLower(v.GetAlias()))
		}
	}
}

func (i *paramInference) targets(targets []*irv1.SelectTarget, s *paramScope) []*irv1.Column {
	var cols []*irv1.Column
	for _, target := range targets {
		e := target.GetExpr()
		if star := e.GetStar(); star != nil {
			for _, name := range s.order {
				tbl := s.tables[name]
				if star.GetQualifier() == "" || star.GetQualifier() == name {
					for _, col := range tbl.GetColumns() {
						cols = append(cols, &irv1.Column{Id: col.GetId(), Name: col.GetName(), Type: col.GetType(), Nullable: col.GetNullable()})
					}
				}
			}
			continue
		}
		typ := i.expr(e, s, nil, nil)
		name := target.GetAlias()
		if name == "" {
			name = e.GetColumnRef().GetColumn()
		}
		col := &irv1.Column{Name: name, Type: typ, Nullable: exprNullable(e, func(q, col string) *irv1.Column {
			return i.column(&irv1.ColumnRef{Qualifier: q, Column: col}, s)
		})}
		if cr := e.GetColumnRef(); cr != nil {
			if source := i.column(cr, s); source != nil {
				col.Id = source.GetId()
				col.Nullable = source.GetNullable()
			}
		}
		cols = append(cols, col)
	}
	return cols
}

// Outer joins add NULL rows to one or both input sides. Copy the relation so
// another alias of the same table, other queries and the catalog stay intact.
func nullExtend(s *paramScope, names []string) {
	for _, name := range names {
		if tbl := s.tables[name]; tbl != nil {
			copy := proto.Clone(tbl).(*irv1.Table)
			for _, col := range copy.GetColumns() {
				col.Nullable = true
			}
			s.tables[name] = copy
		}
	}
}

func (i *paramInference) selectQuery(sel *irv1.SelectStmt, parent *paramScope) []*irv1.Column {
	if sel == nil {
		return nil
	}
	s := i.scope(parent, sel.GetWith())
	if parent == nil {
		i.root = s
	}
	var cols []*irv1.Column
	if ss := sel.GetSelect(); ss != nil {
		i.from(ss.GetFrom(), s)
		cols = i.targets(ss.GetTargets(), s)
		i.expr(ss.GetWhere(), s, scalarType("bool"), nil)
		for _, e := range ss.GetGroupBy() {
			i.expr(e, s, nil, nil)
		}
		for _, e := range ss.GetDistinctOn() {
			i.expr(e, s, nil, nil)
		}
		i.expr(ss.GetHaving(), s, scalarType("bool"), nil)
		for _, w := range ss.GetWindows() {
			i.window(w.GetSpec(), s)
		}
	}
	if set := sel.GetSetOperation(); set != nil {
		cols = i.selectQuery(set.GetLeft(), s)
		i.selectQuery(set.GetRight(), s)
	}
	if sel.GetValues() != nil {
		cols = i.values(sel.GetValues(), s, nil)
	}
	for _, order := range sel.GetOrderBy() {
		i.expr(order.GetExpr(), s, nil, nil)
	}
	i.expr(sel.GetLimit(), s, scalarType("int8"), nil)
	i.expr(sel.GetOffset(), s, scalarType("int8"), nil)
	if ss := sel.GetSelect(); ss != nil && len(cols) == len(ss.GetTargets()) {
		for n, target := range ss.GetTargets() {
			if p := target.GetExpr().GetParameter(); p != nil {
				cols[n].Type = i.params[p.GetPosition()].GetType()
			}
		}
	}
	return cols
}

func (i *paramInference) statement(stmt *irv1.Statement) {
	if stmt == nil {
		return
	}
	switch {
	case stmt.GetSelect() != nil:
		i.output = i.selectQuery(stmt.GetSelect(), nil)
	case stmt.GetUpdate() != nil:
		u := stmt.GetUpdate()
		s := i.scope(nil, u.GetWith())
		tbl := i.addTable(s, u.GetTableName(), u.GetAlias())
		i.from(u.GetFrom(), s)
		i.assignments(u.GetSet(), tbl, s)
		i.expr(u.GetWhere(), s, scalarType("bool"), nil)
		i.output = i.targets(u.GetReturning(), s)
	case stmt.GetDelete() != nil:
		d := stmt.GetDelete()
		s := i.scope(nil, d.GetWith())
		i.addTable(s, d.GetTableName(), d.GetAlias())
		i.from(d.GetUsing(), s)
		i.expr(d.GetWhere(), s, scalarType("bool"), nil)
		i.output = i.targets(d.GetReturning(), s)
	case stmt.GetInsert() != nil:
		ins := stmt.GetInsert()
		s := i.scope(nil, ins.GetWith())
		tbl := i.addTable(s, ins.GetTableName(), ins.GetAlias())
		columns := tbl.GetColumns()
		if len(ins.GetColumns()) > 0 {
			columns = nil
			for _, name := range ins.GetColumns() {
				columns = append(columns, i.catalog.lookupColumn(tbl, name))
			}
		}
		i.values(ins.GetValues(), s, columns)
		i.selectQuery(ins.GetQuery(), s)
		if conflict := ins.GetOnConflict(); conflict != nil {
			s.tables["excluded"] = tbl
			i.assignments(conflict.GetSet(), tbl, s)
			i.expr(conflict.GetIndexPredicate(), s, scalarType("bool"), nil)
			i.expr(conflict.GetUpdateWhere(), s, scalarType("bool"), nil)
			delete(s.tables, "excluded")
		}
		i.output = i.targets(ins.GetReturning(), s)
	}
}

func (i *paramInference) assignments(assigns []*irv1.Assignment, tbl *irv1.Table, s *paramScope) {
	for _, a := range assigns {
		var col *irv1.Column
		if len(a.GetColumns()) == 1 {
			col = i.catalog.lookupColumn(tbl, a.GetColumns()[0])
		}
		i.expr(a.GetValue(), s, col.GetType(), col)
	}
}

func (i *paramInference) values(values *irv1.Values, s *paramScope, targets []*irv1.Column) []*irv1.Column {
	var cols []*irv1.Column
	for rowN, row := range values.GetRows() {
		for n, e := range row.GetValues() {
			var col *irv1.Column
			if n < len(targets) {
				col = targets[n]
			}
			typ := i.expr(e, s, col.GetType(), col)
			if rowN == 0 {
				cols = append(cols, &irv1.Column{Name: fmt.Sprintf("column%d", n+1), Type: typ, Nullable: true})
			}
		}
	}
	return cols
}

func (i *paramInference) window(w *irv1.WindowSpec, s *paramScope) {
	for _, e := range w.GetPartitionBy() {
		i.expr(e, s, nil, nil)
	}
	for _, order := range w.GetOrderBy() {
		i.expr(order.GetExpr(), s, nil, nil)
	}
}

func (i *paramInference) bind(ref *irv1.ParameterRef, typ *irv1.TypeRef, col *irv1.Column) *irv1.TypeRef {
	p := i.params[ref.GetPosition()]
	if p == nil {
		return nil
	}
	if typ != nil {
		if p.GetType() == nil {
			p.Type, p.Nullable = typ, true
			if col != nil {
				p.Nullable = col.GetNullable()
				if col.GetId() != "" {
					p.Column = &irv1.ObjectRef{Id: col.GetId(), Kind: irv1.ObjectKind_OBJECT_KIND_COLUMN}
				}
			}
		} else if incompatibleTypes(p.GetType(), typ) {
			i.diagnostic(fmt.Sprintf("param $%d type conflict: %s versus %s", p.GetNumber(), p.GetType().GetPgName(), typ.GetPgName()))
		}
		if col == nil {
			p.Nullable = true
		}
	}
	if p.GetName() == "" && col != nil {
		p.Name = col.GetName()
	}
	return p.GetType()
}

// Check only known incompatible categories. PostgreSQL permits cross-type
// numeric operators and text/varchar coercions; differing names alone are not
// a type conflict. Unmodelled overloads stay unresolved instead of being guessed.
func incompatibleTypes(a, b *irv1.TypeRef) bool {
	if a == nil || b == nil || a.GetPgName() == b.GetPgName() && a.GetKind() == b.GetKind() {
		return false
	}
	if a.GetElement() != nil || b.GetElement() != nil {
		if a.GetElement() == nil || b.GetElement() == nil {
			return true
		}
		return incompatibleTypes(a.GetElement(), b.GetElement())
	}
	category := func(t *irv1.TypeRef) string {
		n := t.GetPgName()
		if numericRank(n) > 0 {
			return "numeric"
		}
		switch n {
		case "text", "varchar", "bpchar":
			return "string"
		case "date", "timestamp", "timestamptz":
			return "datetime"
		case "bool", "uuid", "bytea", "interval", "json", "jsonb":
			return n
		}
		return ""
	}
	x, y := category(a), category(b)
	return x != "" && y != "" && x != y
}

func arrayType(elem *irv1.TypeRef) *irv1.TypeRef {
	if elem == nil {
		return nil
	}
	return &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_ARRAY, PgName: elem.GetPgName(), Element: elem, ArrayDimensions: 1}
}

func (i *paramInference) expr(e *irv1.Expr, s *paramScope, expected *irv1.TypeRef, origin *irv1.Column) *irv1.TypeRef {
	if e == nil {
		return nil
	}
	switch {
	case e.GetParameter() != nil:
		return i.bind(e.GetParameter(), expected, origin)
	case e.GetColumnRef() != nil:
		return i.column(e.GetColumnRef(), s).GetType()
	case e.GetCast() != nil:
		c := e.GetCast()
		i.expr(c.GetExpr(), s, nil, nil)
		return c.GetTargetType()
	case e.GetLiteral() != nil:
		return inferExprType(e, nil)
	case e.GetFunctionCall() != nil:
		return i.function(e.GetFunctionCall(), s)
	case e.GetCaseExpr() != nil:
		c := e.GetCaseExpr()
		results := []*irv1.Expr{c.GetElseResult()}
		if c.GetOperand() != nil {
			conditions := []*irv1.Expr{c.GetOperand()}
			for _, w := range c.GetWhens() {
				conditions = append(conditions, w.GetCondition())
			}
			i.common(conditions, s)
		} else {
			for _, w := range c.GetWhens() {
				i.expr(w.GetCondition(), s, scalarType("bool"), nil)
			}
		}
		for _, w := range c.GetWhens() {
			results = append(results, w.GetResult())
		}
		return i.common(results, s)
	case e.GetOperator() != nil:
		return i.operator(e.GetOperator(), s)
	case e.GetList() != nil:
		for _, item := range e.GetList().GetElements() {
			i.expr(item, s, expected, nil)
		}
	case e.GetSubquery() != nil:
		q := e.GetSubquery()
		cols := i.selectQuery(q.GetQuery(), s)
		var typ *irv1.TypeRef
		if len(cols) == 1 {
			typ = cols[0].GetType()
		}
		i.expr(q.GetOperand(), s, typ, nil)
		if q.GetKind() != irv1.SubqueryKind_SUBQUERY_KIND_SCALAR {
			return scalarType("bool")
		}
		return typ
	}
	return nil
}

func (i *paramInference) common(args []*irv1.Expr, s *paramScope) *irv1.TypeRef {
	var best *irv1.TypeRef
	for _, arg := range args {
		typ := i.expr(arg, s, nil, nil)
		// Bare string and NULL literals are unknown until context selects a type.
		if lit := arg.GetLiteral(); lit != nil {
			switch lit.GetValue().(type) {
			case *irv1.Literal_StringValue, *irv1.Literal_NullValue:
				continue
			}
		}
		if typ == nil {
			continue
		}
		if incompatibleTypes(best, typ) {
			i.diagnostic("parameter context has incompatible expression types: " + best.GetPgName() + " versus " + typ.GetPgName())
			return nil
		}
		if best == nil || numericRank(typ.GetPgName()) > numericRank(best.GetPgName()) {
			best = typ
		}
	}
	if best == nil {
		best = scalarType("text")
	}
	for _, arg := range args {
		i.expect(arg, best, nil)
	}
	return best
}

func (i *paramInference) function(fc *irv1.FunctionCall, s *paramScope) *irv1.TypeRef {
	args := fc.GetArguments()
	name := funcName(fc)
	builtin := fc.GetName().GetSchema() == "" || fc.GetName().GetSchema() == "pg_catalog"
	if builtin {
		switch name {
		case "coalesce", "greatest", "least", "nullif":
			return i.common(args, s)
		}
	}
	types := make([]*irv1.TypeRef, len(args))
	for n, a := range args {
		types[n] = i.expr(a, s, nil, nil)
	}
	i.expr(fc.GetFilter(), s, scalarType("bool"), nil)
	i.window(fc.GetOver(), s)
	for _, order := range fc.GetOrderBy() {
		i.expr(order.GetExpr(), s, nil, nil)
	}
	// Builtin signatures must not be imposed on a schema-qualified user function.
	if !builtin {
		return nil
	}
	var signature []string
	switch name {
	case "lower", "upper", "initcap", "reverse":
		signature = []string{"text"}
	case "replace":
		signature = []string{"text", "text", "text"}
	case "strpos":
		signature = []string{"text", "text"}
	case "left", "right", "repeat":
		signature = []string{"text", "int4"}
	case "length", "char_length", "character_length", "octet_length", "bit_length":
		if len(types) > 0 && types[0] == nil {
			signature = []string{"text"}
		}
	case "substr", "substring":
		first := "text"
		if len(types) > 0 && typePgName(types[0]) == "bytea" {
			first = "bytea"
		}
		second := "int4"
		if name == "substring" && first == "text" && len(types) > 1 && (types[1] == nil || typePgName(types[1]) == "text") && (len(types) < 3 || types[2] == nil || typePgName(types[2]) == "text") {
			second = "text"
		}
		signature = []string{first, second, second}
	}
	for n, name := range signature {
		if n < len(args) {
			i.expect(args[n], scalarType(name), nil)
		}
	}
	return inferFunctionType(fc, func(q, col string) *irv1.Column { return i.column(&irv1.ColumnRef{Qualifier: q, Column: col}, s) })
}

func (i *paramInference) operator(op *irv1.OperatorExpr, s *paramScope) *irv1.TypeRef {
	args, sym := op.GetOperands(), strings.ToUpper(op.GetSymbol())
	if sym == "AND" || sym == "OR" || sym == "NOT" {
		for _, a := range args {
			i.expr(a, s, scalarType("bool"), nil)
		}
		return scalarType("bool")
	}
	if len(args) != 2 {
		for _, a := range args {
			i.expr(a, s, nil, nil)
		}
		if isBoolOperator(sym) {
			return scalarType("bool")
		}
		return nil
	}
	left, right := i.expr(args[0], s, nil, nil), i.expr(args[1], s, nil, nil)
	leftWant, rightWant := right, left
	if sym == "NULLIF" {
		return i.common(args, s)
	}
	if strings.Contains(sym, "ANY") || strings.Contains(sym, "ALL") {
		rightWant = arrayType(left)
		leftWant = right.GetElement()
	} else if isArithmeticOperator(sym) {
		if (typePgName(left) == "date" && right == nil) || (typePgName(right) == "date" && left == nil) {
			i.diagnostic("parameter type for date arithmetic is ambiguous; add an explicit cast")
			return nil
		}
		// Arithmetic signatures are not generally T × T -> T.
		if (sym == "+" || sym == "-") && (typePgName(left) == "timestamp" || typePgName(left) == "timestamptz") && right == nil {
			rightWant = scalarType("interval")
		}
		if sym == "+" && (typePgName(right) == "timestamp" || typePgName(right) == "timestamptz") && left == nil {
			leftWant = scalarType("interval")
		}
		if (sym == "*" || sym == "/") && typePgName(left) == "interval" {
			rightWant = scalarType("float8")
		}
		if sym == "*" && typePgName(right) == "interval" {
			leftWant = scalarType("float8")
		}
	}
	// Direct comparisons retain column identity and existing NOT NULL behaviour.
	var leftCol, rightCol *irv1.Column
	if isBoolOperator(sym) && !strings.Contains(sym, "ANY") && !strings.Contains(sym, "ALL") {
		if args[1].GetColumnRef() != nil {
			leftCol = i.column(args[1].GetColumnRef(), s)
		}
		if args[0].GetColumnRef() != nil {
			rightCol = i.column(args[0].GetColumnRef(), s)
		}
	}
	i.expect(args[0], leftWant, leftCol)
	i.expect(args[1], rightWant, rightCol)
	if isBoolOperator(sym) || strings.Contains(sym, "ANY") || strings.Contains(sym, "ALL") {
		return scalarType("bool")
	}
	if sym == "||" {
		return scalarType("text")
	}
	if isArithmeticOperator(sym) {
		if typePgName(left) == "timestamp" || typePgName(left) == "timestamptz" || typePgName(left) == "interval" {
			return left
		}
		if left == nil {
			return right
		}
		if right == nil {
			return left
		}
		if numericRank(right.GetPgName()) > numericRank(left.GetPgName()) {
			return right
		}
		return left
	}
	return nil
}

// Common result type for CASE/COALESCE and related expressions. Parameter
// inference supplies the input constraints separately; unknown literal inputs
// do not force text when a typed sibling is available.
func commonExprType(args []*irv1.Expr, resolve func(string, string) *irv1.Column) *irv1.TypeRef {
	var best *irv1.TypeRef
	for _, arg := range args {
		if lit := arg.GetLiteral(); lit != nil {
			switch lit.GetValue().(type) {
			case *irv1.Literal_StringValue, *irv1.Literal_NullValue:
				continue
			}
		}
		typ := inferExprType(arg, resolve)
		if typ == nil {
			continue
		}
		if incompatibleTypes(best, typ) {
			return nil
		}
		if best == nil || numericRank(typ.GetPgName()) > numericRank(best.GetPgName()) {
			best = typ
		}
	}
	if best == nil {
		best = scalarType("text")
	}
	return best
}

// Children have already been analysed before contextual types are propagated.
// Only bind parameters (and IN/row list items) accept that expected input type;
// re-evaluating nested functions here would make traversal exponential.
func (i *paramInference) expect(e *irv1.Expr, typ *irv1.TypeRef, col *irv1.Column) {
	if e == nil {
		return
	}
	if p := e.GetParameter(); p != nil {
		i.bind(p, typ, col)
	}
	if list := e.GetList(); list != nil {
		for _, item := range list.GetElements() {
			i.expect(item, typ, nil)
		}
	}
}
