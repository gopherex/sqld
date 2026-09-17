package query

import (
	"strings"

	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

// Contexts are ordered by strength, so merging repeated uses is independent
// of AST traversal order. Explicit NULL handling wins over required uses;
// a nullable column alone does not override another NOT NULL column.
type inputNullContext uint8

const (
	inputUnknown inputNullContext = iota
	inputNullableColumn
	inputRequired
	inputAcceptsNull
)

type inputNullability struct {
	context inputNullContext
	cast    bool
}

func (n inputNullability) isNullable() bool {
	switch n.context {
	case inputRequired:
		return false
	case inputNullableColumn, inputAcceptsNull:
		return true
	default:
		// A direct cast provides a value-type API unless SQL supplies a
		// nullable context. Uncast unknown inputs stay conservative.
		return !n.cast
	}
}

func columnInputContext(col *irv1.Column) inputNullContext {
	if col.GetNullable() {
		return inputNullableColumn
	}
	return inputRequired
}

func (i *paramInference) insertSelectInputs(sel *irv1.SelectStmt, columns []*irv1.Column) {
	if sel == nil {
		return
	}
	targets := sel.GetSelect().GetTargets()
	if len(targets) == len(columns) {
		for n, target := range targets {
			i.expect(target.GetExpr(), columns[n].GetType(), columns[n])
		}
	}
	if set := sel.GetSetOperation(); set != nil {
		i.insertSelectInputs(set.GetLeft(), columns)
		i.insertSelectInputs(set.GetRight(), columns)
	}
}

func (i *paramInference) recordInput(number uint32, context inputNullContext) {
	c := i.inputNulls[number]
	if context > c.context {
		c.context = context
	}
	i.inputNulls[number] = c
}

// A context constrains inputs only through NULL-propagating expressions. A
// COALESCE result, for example, can be required while its inputs accept nil.
func (i *paramInference) constrainInput(e *irv1.Expr, context inputNullContext) {
	if e == nil {
		return
	}
	if p := e.GetParameter(); p != nil {
		i.recordInput(p.GetPosition(), context)
		return
	}
	if c := e.GetCast(); c != nil {
		i.constrainInput(c.GetExpr(), context)
		return
	}
	if f := e.GetFunctionCall(); f != nil && strictNullFunction(f) {
		for _, arg := range f.GetArguments() {
			i.constrainInput(arg, context)
		}
	}
	if op := e.GetOperator(); op != nil {
		switch strings.ToUpper(op.GetSymbol()) {
		case "+", "-", "*", "/", "%", "^", "||":
			for _, arg := range op.GetOperands() {
				i.constrainInput(arg, context)
			}
		}
	}
}

func strictNullFunction(f *irv1.FunctionCall) bool {
	if schema := f.GetName().GetSchema(); schema != "" && schema != "pg_catalog" {
		return false
	}
	switch funcName(f) {
	case "lower", "upper", "length", "char_length", "character_length", "octet_length", "bit_length",
		"trim", "ltrim", "rtrim", "btrim", "replace", "initcap", "reverse":
		return true
	}
	return false
}

// Comparisons inherit the input column's declared nullability. The NULL rows
// added by an outer join affect result columns, not the input API contract.
// Keep this provenance local to the join: a nullable derived-table column must
// not be mistaken for its original NOT NULL catalog column.
func (i *paramInference) inputOrigin(e *irv1.Expr, s *paramScope) *irv1.Column {
	if e == nil {
		return nil
	}
	if cr := e.GetColumnRef(); cr != nil {
		col := i.column(cr, s)
		for i.joinSources[col] != nil {
			col = i.joinSources[col]
		}
		return col
	}
	if c := e.GetCast(); c != nil {
		return i.inputOrigin(c.GetExpr(), s)
	}
	if f := e.GetFunctionCall(); f != nil && strictNullFunction(f) {
		var origin *irv1.Column
		for _, arg := range f.GetArguments() {
			if lit := arg.GetLiteral(); lit != nil && !lit.GetNullValue() {
				continue
			}
			col := i.inputOrigin(arg, s)
			if col == nil {
				return nil
			}
			if col.GetNullable() {
				return &irv1.Column{Nullable: true}
			}
			if origin == nil || col.GetId() != "" {
				origin = col
			}
		}
		return origin
	}
	return nil
}
