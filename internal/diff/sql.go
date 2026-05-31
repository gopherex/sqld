package diff

import (
	"fmt"
	"strconv"
	"strings"

	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

// quoteIdent double-quotes a SQL identifier, escaping embedded quotes.
func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// quoteString single-quotes a SQL string literal, escaping embedded quotes.
func quoteString(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// qualified renders a possibly schema-qualified, quoted object name. The
// "public" schema is still qualified explicitly for determinism.
func qualified(schema, name string) string {
	if schema == "" {
		return quoteIdent(name)
	}
	return quoteIdent(schema) + "." + quoteIdent(name)
}

// qname renders a *irv1.QualifiedName.
func qname(n *irv1.QualifiedName) string {
	return qualified(n.GetSchema(), n.GetName())
}

// tableQualified renders a table's schema-qualified name.
func tableQualified(t *irv1.Table) string {
	return qname(t.GetName())
}

// renderType renders a TypeRef to PostgreSQL DDL: the canonical pg_name plus
// any modifier, with "[]" appended per array dimension.
func renderType(t *irv1.TypeRef) string {
	if t == nil {
		return ""
	}
	// Arrays: render the element type and append [] per dimension. The
	// catalog stores the element pg_name on the TypeRef itself (with the
	// element TypeRef also populated), so prefer Element when present.
	if t.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
		elem := t.GetElement()
		var base string
		if elem != nil {
			base = renderType(elem)
		} else {
			base = baseTypeName(typeBaseName(t), t.GetModifier())
		}
		dims := int(t.GetArrayDimensions())
		if dims < 1 {
			dims = 1
		}
		return base + strings.Repeat("[]", dims)
	}
	return baseTypeName(typeBaseName(t), t.GetModifier())
}

// typeBaseName returns the bare type name to render. For user-defined types
// (enum/domain/composite/range) the introspector populates Udt with the
// schema-qualified name; that qualified name is rendered so the emitted DDL is
// valid regardless of search_path (the referenced type rarely lives in a
// search-path schema). The parse path leaves Udt nil and stores the bare name
// in PgName, so it renders unqualified exactly as before — keeping diff
// equality between a parsed and introspected catalog intact for built-ins and
// for user types referenced by bare name.
func typeBaseName(t *irv1.TypeRef) string {
	if udt := t.GetUdt(); udt != nil {
		if n := udt.GetName(); n != nil && n.GetSchema() != "" && n.GetSchema() != "pg_catalog" {
			return qname(n)
		}
	}
	return t.GetPgName()
}

// baseTypeName renders a scalar type name plus its modifier.
func baseTypeName(name string, mod *irv1.TypeModifier) string {
	if m := renderModifier(mod); m != "" {
		name += m
	}
	return name
}

// renderModifier renders a type modifier suffix, e.g. "(10,2)", "(20)".
func renderModifier(m *irv1.TypeModifier) string {
	if m == nil {
		return ""
	}
	switch {
	case m.GetNumeric() != nil:
		n := m.GetNumeric()
		if n.GetScale() > 0 {
			return fmt.Sprintf("(%d,%d)", n.GetPrecision(), n.GetScale())
		}
		if n.GetPrecision() > 0 {
			return fmt.Sprintf("(%d)", n.GetPrecision())
		}
	case m.GetText() != nil:
		if l := m.GetText().GetLength(); l > 0 {
			return fmt.Sprintf("(%d)", l)
		}
	case m.GetDateTime() != nil:
		// with_timezone is already reflected in the pg_name (e.g. timestamptz);
		// only the fractional-seconds precision needs rendering.
		if p := m.GetDateTime().GetPrecision(); p > 0 {
			return fmt.Sprintf("(%d)", p)
		}
	case m.GetInterval() != nil:
		iv := m.GetInterval()
		s := ""
		if f := iv.GetFields(); f != "" {
			s += " " + f
		}
		if p := iv.GetPrecision(); p > 0 {
			s += fmt.Sprintf("(%d)", p)
		}
		return s
	}
	return ""
}

// renderExpr renders an expression for use as a DEFAULT or similar. It prefers
// the raw SQL form and falls back to a best-effort literal rendering.
func renderExpr(e *irv1.Expr) string {
	if e == nil {
		return ""
	}
	if raw := e.GetRawSql(); raw != "" {
		return raw
	}
	if lit := e.GetLiteral(); lit != nil {
		return renderLiteral(lit)
	}
	return ""
}

// renderLiteral renders a literal constant.
func renderLiteral(l *irv1.Literal) string {
	switch v := l.GetValue().(type) {
	case *irv1.Literal_NullValue:
		return "NULL"
	case *irv1.Literal_IntValue:
		return strconv.FormatInt(v.IntValue, 10)
	case *irv1.Literal_FloatValue:
		return strconv.FormatFloat(v.FloatValue, 'g', -1, 64)
	case *irv1.Literal_NumericValue:
		return v.NumericValue
	case *irv1.Literal_StringValue:
		return quoteString(v.StringValue)
	case *irv1.Literal_BoolValue:
		if v.BoolValue {
			return "TRUE"
		}
		return "FALSE"
	}
	return ""
}

// renderColumn renders a column definition: `"col" type [NOT NULL] [DEFAULT expr]`.
func renderColumn(c *irv1.Column) string {
	var b strings.Builder
	b.WriteString(quoteIdent(c.GetName()))
	b.WriteString(" ")
	b.WriteString(renderType(c.GetType()))
	if !c.GetNullable() {
		b.WriteString(" NOT NULL")
	}
	if d := renderExpr(c.GetDefaultExpr()); d != "" {
		b.WriteString(" DEFAULT ")
		b.WriteString(d)
	}
	return b.String()
}

// --- schema ------------------------------------------------------------------

func renderCreateSchema(s *irv1.Schema) string {
	return "CREATE SCHEMA " + quoteIdent(s.GetName()) + ";"
}

func renderDropSchema(s *irv1.Schema) string {
	return "DROP SCHEMA " + quoteIdent(s.GetName()) + ";"
}

// --- types -------------------------------------------------------------------

func renderDropType(n *irv1.QualifiedName) string {
	return "DROP TYPE " + qname(n) + ";"
}

func renderCreateEnum(e *irv1.EnumType) string {
	labels := make([]string, 0, len(e.GetLabels()))
	for _, l := range e.GetLabels() {
		labels = append(labels, quoteString(l))
	}
	return "CREATE TYPE " + qname(e.GetName()) + " AS ENUM (" + strings.Join(labels, ", ") + ");"
}

func renderCreateDomain(d *irv1.DomainType) string {
	var b strings.Builder
	b.WriteString("CREATE DOMAIN ")
	b.WriteString(qname(d.GetName()))
	b.WriteString(" AS ")
	b.WriteString(renderType(d.GetBaseType()))
	if c := d.GetCollation(); c != "" {
		b.WriteString(" COLLATE ")
		b.WriteString(quoteIdent(c))
	}
	if !d.GetNullable() {
		b.WriteString(" NOT NULL")
	}
	if def := renderExpr(d.GetDefaultExpr()); def != "" {
		b.WriteString(" DEFAULT ")
		b.WriteString(def)
	}
	b.WriteString(";")
	return b.String()
}

func renderCreateComposite(c *irv1.CompositeType) string {
	fields := make([]string, 0, len(c.GetFields()))
	for _, f := range c.GetFields() {
		fields = append(fields, quoteIdent(f.GetName())+" "+renderType(f.GetType()))
	}
	return "CREATE TYPE " + qname(c.GetName()) + " AS (" + strings.Join(fields, ", ") + ");"
}

func renderCreateRange(r *irv1.RangeType) string {
	parts := []string{"subtype = " + renderType(r.GetSubtype())}
	if v := r.GetSubtypeOpclass(); v != "" {
		parts = append(parts, "subtype_opclass = "+v)
	}
	if v := r.GetCollation(); v != "" {
		parts = append(parts, "collation = "+v)
	}
	if v := r.GetCanonical(); v != "" {
		parts = append(parts, "canonical = "+v)
	}
	if v := r.GetSubtypeDiff(); v != "" {
		parts = append(parts, "subtype_diff = "+v)
	}
	// A range type created with a multirange_type_name owns an associated
	// multirange type; it must be re-stated so the multirange (referenced by
	// columns) exists. Qualify it into the range's own schema, mirroring how
	// PostgreSQL creates the multirange alongside the range.
	if v := r.GetMultirange(); v != "" {
		parts = append(parts, "multirange_type_name = "+qualified(r.GetName().GetSchema(), v))
	}
	return "CREATE TYPE " + qname(r.GetName()) + " AS RANGE (" + strings.Join(parts, ", ") + ");"
}

// --- sequences ---------------------------------------------------------------

func renderCreateSequence(s *irv1.Sequence) string {
	var b strings.Builder
	b.WriteString("CREATE SEQUENCE ")
	b.WriteString(qname(s.GetName()))
	if dt := s.GetDataType(); dt != nil && dt.GetPgName() != "" {
		b.WriteString(" AS ")
		b.WriteString(renderType(dt))
	}
	if v := s.GetIncrement(); v != 0 {
		b.WriteString(" INCREMENT BY ")
		b.WriteString(strconv.FormatInt(v, 10))
	}
	if v := s.GetMinValue(); v != 0 {
		b.WriteString(" MINVALUE ")
		b.WriteString(strconv.FormatInt(v, 10))
	}
	if v := s.GetMaxValue(); v != 0 {
		b.WriteString(" MAXVALUE ")
		b.WriteString(strconv.FormatInt(v, 10))
	}
	if v := s.GetStart(); v != 0 {
		b.WriteString(" START WITH ")
		b.WriteString(strconv.FormatInt(v, 10))
	}
	if v := s.GetCache(); v > 0 {
		b.WriteString(" CACHE ")
		b.WriteString(strconv.FormatInt(v, 10))
	}
	if s.GetCycle() {
		b.WriteString(" CYCLE")
	}
	b.WriteString(";")
	return b.String()
}

func renderDropSequence(s *irv1.Sequence) string {
	return "DROP SEQUENCE " + qname(s.GetName()) + ";"
}

// --- tables ------------------------------------------------------------------

// renderCreateTable renders a CREATE TABLE with columns and an inline PRIMARY
// KEY clause when the table has a primary-key constraint. Other constraints and
// indexes are handled by the follow-up task.
func renderCreateTable(t *irv1.Table) string {
	var lines []string
	for _, c := range t.GetColumns() {
		lines = append(lines, "  "+renderColumn(c))
	}
	if pk := primaryKey(t); pk != nil {
		cols := make([]string, 0, len(pk.GetColumns()))
		for _, c := range pk.GetColumns() {
			cols = append(cols, quoteIdent(c))
		}
		lines = append(lines, "  PRIMARY KEY ("+strings.Join(cols, ", ")+")")
	}
	return "CREATE TABLE " + tableQualified(t) + " (\n" + strings.Join(lines, ",\n") + "\n);"
}

func renderDropTable(t *irv1.Table) string {
	return "DROP TABLE " + tableQualified(t) + ";"
}

// primaryKey returns the table's primary-key constraint body, if any.
func primaryKey(t *irv1.Table) *irv1.PrimaryKey {
	for _, c := range t.GetConstraints() {
		if c.GetType() == irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY {
			if pk := c.GetPrimaryKey(); pk != nil {
				return pk
			}
		}
	}
	return nil
}

// --- constraints -------------------------------------------------------------

// quotedCols quotes a list of column names and joins them with ", ".
func quotedCols(cols []string) string {
	out := make([]string, 0, len(cols))
	for _, c := range cols {
		out = append(out, quoteIdent(c))
	}
	return strings.Join(out, ", ")
}

// referentialAction renders an ON DELETE/UPDATE action clause. NO ACTION (the
// PostgreSQL default, and what the parser fills in for an omitted clause) and
// UNSPECIFIED both render empty so the emitted DDL stays minimal.
func referentialAction(a irv1.ReferentialAction) string {
	switch a {
	case irv1.ReferentialAction_REFERENTIAL_ACTION_RESTRICT:
		return "RESTRICT"
	case irv1.ReferentialAction_REFERENTIAL_ACTION_CASCADE:
		return "CASCADE"
	case irv1.ReferentialAction_REFERENTIAL_ACTION_SET_NULL:
		return "SET NULL"
	case irv1.ReferentialAction_REFERENTIAL_ACTION_SET_DEFAULT:
		return "SET DEFAULT"
	default:
		return ""
	}
}

// matchType renders a non-default MATCH clause, or "" for MATCH SIMPLE.
func matchType(m irv1.MatchType) string {
	switch m {
	case irv1.MatchType_MATCH_TYPE_FULL:
		return "FULL"
	case irv1.MatchType_MATCH_TYPE_PARTIAL:
		return "PARTIAL"
	default:
		return ""
	}
}

// renderConstraintBody renders the constraint-defining clause without the
// leading CONSTRAINT name, e.g. `PRIMARY KEY ("id")` or
// `FOREIGN KEY ("a") REFERENCES "s"."t" ("id") ON DELETE CASCADE`.
func renderConstraintBody(c *irv1.Constraint) string {
	var b strings.Builder
	switch c.GetType() {
	case irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY:
		b.WriteString("PRIMARY KEY (")
		b.WriteString(quotedCols(c.GetPrimaryKey().GetColumns()))
		b.WriteString(")")

	case irv1.ConstraintType_CONSTRAINT_TYPE_UNIQUE:
		u := c.GetUnique()
		b.WriteString("UNIQUE")
		if u.GetNullsNotDistinct() {
			b.WriteString(" NULLS NOT DISTINCT")
		}
		b.WriteString(" (")
		b.WriteString(quotedCols(u.GetColumns()))
		b.WriteString(")")

	case irv1.ConstraintType_CONSTRAINT_TYPE_FOREIGN_KEY:
		fk := c.GetForeignKey()
		b.WriteString("FOREIGN KEY (")
		b.WriteString(quotedCols(fk.GetColumns()))
		b.WriteString(") REFERENCES ")
		b.WriteString(refTableName(fk.GetReferencedTable()))
		if cols := fk.GetReferencedColumns(); len(cols) > 0 {
			b.WriteString(" (")
			b.WriteString(quotedCols(cols))
			b.WriteString(")")
		}
		if m := matchType(fk.GetMatchType()); m != "" {
			b.WriteString(" MATCH ")
			b.WriteString(m)
		}
		if a := referentialAction(fk.GetOnDelete()); a != "" {
			b.WriteString(" ON DELETE ")
			b.WriteString(a)
		}
		if a := referentialAction(fk.GetOnUpdate()); a != "" {
			b.WriteString(" ON UPDATE ")
			b.WriteString(a)
		}

	case irv1.ConstraintType_CONSTRAINT_TYPE_CHECK:
		cc := c.GetCheck()
		b.WriteString("CHECK (")
		b.WriteString(renderSelectExpr(cc.GetExpression()))
		b.WriteString(")")
		if cc.GetNoInherit() {
			b.WriteString(" NO INHERIT")
		}

	case irv1.ConstraintType_CONSTRAINT_TYPE_EXCLUSION:
		ex := c.GetExclusion()
		b.WriteString("EXCLUDE")
		if m := ex.GetIndexMethod(); m != "" {
			b.WriteString(" USING ")
			b.WriteString(m)
		}
		elems := make([]string, 0, len(ex.GetElements()))
		for _, e := range ex.GetElements() {
			var s string
			if col := e.GetColumn(); col != "" {
				s = quoteIdent(col)
			} else {
				s = renderSelectExpr(e.GetExpr())
			}
			if op := e.GetOperator(); op != "" {
				s += " WITH " + op
			}
			elems = append(elems, s)
		}
		b.WriteString(" (")
		b.WriteString(strings.Join(elems, ", "))
		b.WriteString(")")
		if pred := renderSelectExpr(ex.GetPredicate()); pred != "" {
			b.WriteString(" WHERE (")
			b.WriteString(pred)
			b.WriteString(")")
		}

	case irv1.ConstraintType_CONSTRAINT_TYPE_NOT_NULL:
		// NOT NULL is rendered inline on the column, never as a named
		// constraint; emit nothing here.
		return ""

	default:
		return ""
	}

	if c.GetDeferrable() {
		b.WriteString(" DEFERRABLE")
		if c.GetInitiallyDeferred() {
			b.WriteString(" INITIALLY DEFERRED")
		}
	}
	return b.String()
}

// refTableName renders a foreign-key referenced table from its ObjectRef,
// preferring the qualified name carried on the ref.
func refTableName(r *irv1.ObjectRef) string {
	if r == nil {
		return ""
	}
	if n := r.GetName(); n != nil {
		return qname(n)
	}
	return quoteIdent(r.GetId())
}

// renderAddConstraint renders an ALTER TABLE .. ADD CONSTRAINT statement.
func renderAddConstraint(t *irv1.Table, c *irv1.Constraint) string {
	body := renderConstraintBody(c)
	if body == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString("ALTER TABLE ")
	b.WriteString(tableQualified(t))
	b.WriteString(" ADD ")
	if name := c.GetName(); name != "" {
		b.WriteString("CONSTRAINT ")
		b.WriteString(quoteIdent(name))
		b.WriteString(" ")
	}
	b.WriteString(body)
	b.WriteString(";")
	return b.String()
}

// renderDropConstraint renders an ALTER TABLE .. DROP CONSTRAINT statement. The
// parser does not synthesize names for constraints declared without one (e.g.
// inline `REFERENCES ...`); PostgreSQL assigns those at execution time, so they
// cannot be dropped by name here. Such drops degrade to a warning comment.
func renderDropConstraint(t *irv1.Table, c *irv1.Constraint) string {
	name := c.GetName()
	if name == "" {
		return "-- WARNING: cannot drop unnamed constraint on " + tableQualified(t) +
			" (" + c.GetType().String() + "); name unknown without a database connection"
	}
	return "ALTER TABLE " + tableQualified(t) + " DROP CONSTRAINT " + quoteIdent(name) + ";"
}

// --- indexes -----------------------------------------------------------------

// renderIndexElement renders one index key element.
func renderIndexElement(e *irv1.IndexElement) string {
	var b strings.Builder
	if col := e.GetColumn(); col != "" {
		b.WriteString(quoteIdent(col))
	} else if ex := e.GetExpr(); ex != nil {
		b.WriteString("(")
		b.WriteString(renderSelectExpr(ex))
		b.WriteString(")")
	}
	if c := e.GetCollation(); c != "" {
		b.WriteString(" COLLATE ")
		b.WriteString(quoteIdent(c))
	}
	if oc := e.GetOpclass(); oc != "" {
		b.WriteString(" ")
		b.WriteString(oc)
	}
	switch e.GetOrder() {
	case irv1.SortOrder_SORT_ORDER_ASC:
		b.WriteString(" ASC")
	case irv1.SortOrder_SORT_ORDER_DESC:
		b.WriteString(" DESC")
	}
	switch e.GetNulls() {
	case irv1.NullsOrder_NULLS_ORDER_FIRST:
		b.WriteString(" NULLS FIRST")
	case irv1.NullsOrder_NULLS_ORDER_LAST:
		b.WriteString(" NULLS LAST")
	}
	return b.String()
}

// renderCreateIndex renders a CREATE [UNIQUE] INDEX on the given table.
func renderCreateIndex(idx *irv1.Index, table *irv1.Table) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if idx.GetUnique() {
		b.WriteString("UNIQUE ")
	}
	b.WriteString("INDEX ")
	if n := idx.GetName(); n != "" {
		b.WriteString(quoteIdent(n))
		b.WriteString(" ")
	}
	b.WriteString("ON ")
	b.WriteString(tableQualified(table))
	if m := idx.GetMethod(); m != "" && m != "btree" {
		b.WriteString(" USING ")
		b.WriteString(m)
	}
	elems := make([]string, 0, len(idx.GetElements()))
	for _, e := range idx.GetElements() {
		elems = append(elems, renderIndexElement(e))
	}
	b.WriteString(" (")
	b.WriteString(strings.Join(elems, ", "))
	b.WriteString(")")
	if inc := idx.GetInclude(); len(inc) > 0 {
		b.WriteString(" INCLUDE (")
		b.WriteString(quotedCols(inc))
		b.WriteString(")")
	}
	if idx.GetNullsNotDistinct() {
		b.WriteString(" NULLS NOT DISTINCT")
	}
	if pred := renderSelectExpr(idx.GetPredicate()); pred != "" {
		b.WriteString(" WHERE ")
		b.WriteString(pred)
	}
	b.WriteString(";")
	return b.String()
}

// renderDropIndex renders a DROP INDEX for an index on a table in schema.
func renderDropIndex(idx *irv1.Index, schema string) string {
	return "DROP INDEX " + qualified(schema, idx.GetName()) + ";"
}

// --- views -------------------------------------------------------------------

// renderCreateView renders CREATE [OR REPLACE] VIEW .. AS <query>.
func renderCreateView(v *irv1.View, orReplace bool) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if orReplace {
		b.WriteString("OR REPLACE ")
	}
	b.WriteString("VIEW ")
	b.WriteString(qname(v.GetName()))
	if cols := v.GetColumns(); len(cols) > 0 {
		b.WriteString(" (")
		b.WriteString(quotedCols(cols))
		b.WriteString(")")
	}
	if v.GetSecurityBarrier() {
		b.WriteString(" WITH (security_barrier=true)")
	}
	b.WriteString(" AS ")
	b.WriteString(renderQuery(v.GetQuery()))
	switch v.GetCheckOption() {
	case irv1.CheckOption_CHECK_OPTION_LOCAL:
		b.WriteString(" WITH LOCAL CHECK OPTION")
	case irv1.CheckOption_CHECK_OPTION_CASCADED:
		b.WriteString(" WITH CASCADED CHECK OPTION")
	}
	b.WriteString(";")
	return b.String()
}

func renderDropView(v *irv1.View) string {
	return "DROP VIEW " + qname(v.GetName()) + ";"
}

// renderCreateMatView renders CREATE MATERIALIZED VIEW .. AS <query>.
func renderCreateMatView(mv *irv1.MaterializedView) string {
	var b strings.Builder
	b.WriteString("CREATE MATERIALIZED VIEW ")
	b.WriteString(qname(mv.GetName()))
	if cols := mv.GetColumns(); len(cols) > 0 {
		b.WriteString(" (")
		b.WriteString(quotedCols(cols))
		b.WriteString(")")
	}
	b.WriteString(" AS ")
	b.WriteString(renderQuery(mv.GetQuery()))
	if mv.GetWithData() {
		b.WriteString(" WITH DATA")
	} else {
		b.WriteString(" WITH NO DATA")
	}
	b.WriteString(";")
	return b.String()
}

func renderDropMatView(mv *irv1.MaterializedView) string {
	return "DROP MATERIALIZED VIEW " + qname(mv.GetName()) + ";"
}

// --- functions and procedures ------------------------------------------------

// renderArgument renders one function/procedure argument.
func renderArgument(a *irv1.Argument) string {
	var b strings.Builder
	switch a.GetMode() {
	case irv1.ArgMode_ARG_MODE_OUT:
		b.WriteString("OUT ")
	case irv1.ArgMode_ARG_MODE_INOUT:
		b.WriteString("INOUT ")
	case irv1.ArgMode_ARG_MODE_VARIADIC:
		b.WriteString("VARIADIC ")
	}
	if n := a.GetName(); n != "" {
		b.WriteString(quoteIdent(n))
		b.WriteString(" ")
	}
	b.WriteString(renderType(a.GetType()))
	if d := renderExpr(a.GetDefaultValue()); d != "" {
		b.WriteString(" DEFAULT ")
		b.WriteString(d)
	}
	return b.String()
}

// renderArgs renders a parenthesized argument list.
func renderArgs(args []*irv1.Argument) string {
	parts := make([]string, 0, len(args))
	for _, a := range args {
		parts = append(parts, renderArgument(a))
	}
	return "(" + strings.Join(parts, ", ") + ")"
}

// renderReturnType renders the RETURNS clause body for a function.
func renderReturnType(r *irv1.ReturnType) string {
	switch k := r.GetKind().(type) {
	case *irv1.ReturnType_Scalar:
		return renderType(k.Scalar)
	case *irv1.ReturnType_Setof:
		return "SETOF " + renderType(k.Setof.GetType())
	case *irv1.ReturnType_Table:
		cols := make([]string, 0, len(k.Table.GetColumns()))
		for _, c := range k.Table.GetColumns() {
			cols = append(cols, quoteIdent(c.GetName())+" "+renderType(c.GetType()))
		}
		return "TABLE(" + strings.Join(cols, ", ") + ")"
	case *irv1.ReturnType_VoidValue:
		return "void"
	case *irv1.ReturnType_TriggerValue:
		return "trigger"
	default:
		return ""
	}
}

// bodyDollarQuote wraps a routine body in dollar-quoting, choosing a tag that
// does not collide with the body text.
func bodyDollarQuote(body string) string {
	tag := "$$"
	if strings.Contains(body, "$$") {
		for _, t := range []string{"$body$", "$func$", "$_$", "$sqld$"} {
			if !strings.Contains(body, t) {
				tag = t
				break
			}
		}
	}
	return tag + body + tag
}

// renderCreateFunction renders CREATE [OR REPLACE] FUNCTION.
func renderCreateFunction(fn *irv1.Function, orReplace bool) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if orReplace {
		b.WriteString("OR REPLACE ")
	}
	b.WriteString("FUNCTION ")
	b.WriteString(qname(fn.GetName()))
	b.WriteString(renderArgs(fn.GetArguments()))
	if rt := renderReturnType(fn.GetReturns()); rt != "" {
		b.WriteString(" RETURNS ")
		b.WriteString(rt)
	}
	if lang := fn.GetLanguage(); lang != "" {
		b.WriteString(" LANGUAGE ")
		b.WriteString(lang)
	}
	switch fn.GetVolatility() {
	case irv1.Volatility_VOLATILITY_IMMUTABLE:
		b.WriteString(" IMMUTABLE")
	case irv1.Volatility_VOLATILITY_STABLE:
		b.WriteString(" STABLE")
	case irv1.Volatility_VOLATILITY_VOLATILE:
		b.WriteString(" VOLATILE")
	}
	if fn.GetLeakproof() {
		b.WriteString(" LEAKPROOF")
	}
	if fn.GetNullInput() == irv1.NullInputBehavior_NULL_INPUT_BEHAVIOR_STRICT {
		b.WriteString(" STRICT")
	}
	if fn.GetSecurityDefiner() {
		b.WriteString(" SECURITY DEFINER")
	}
	switch fn.GetParallel() {
	case irv1.ParallelSafety_PARALLEL_SAFETY_SAFE:
		b.WriteString(" PARALLEL SAFE")
	case irv1.ParallelSafety_PARALLEL_SAFETY_RESTRICTED:
		b.WriteString(" PARALLEL RESTRICTED")
	case irv1.ParallelSafety_PARALLEL_SAFETY_UNSAFE:
		b.WriteString(" PARALLEL UNSAFE")
	}
	b.WriteString(" AS ")
	b.WriteString(bodyDollarQuote(fn.GetBody()))
	b.WriteString(";")
	return b.String()
}

// renderDropFunction renders DROP FUNCTION with its argument signature.
func renderDropFunction(fn *irv1.Function) string {
	return "DROP FUNCTION " + qname(fn.GetName()) + renderArgs(fn.GetArguments()) + ";"
}

// renderCreateProcedure renders CREATE [OR REPLACE] PROCEDURE.
func renderCreateProcedure(p *irv1.Procedure, orReplace bool) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if orReplace {
		b.WriteString("OR REPLACE ")
	}
	b.WriteString("PROCEDURE ")
	b.WriteString(qname(p.GetName()))
	b.WriteString(renderArgs(p.GetArguments()))
	if lang := p.GetLanguage(); lang != "" {
		b.WriteString(" LANGUAGE ")
		b.WriteString(lang)
	}
	if p.GetSecurityDefiner() {
		b.WriteString(" SECURITY DEFINER")
	}
	b.WriteString(" AS ")
	b.WriteString(bodyDollarQuote(p.GetBody()))
	b.WriteString(";")
	return b.String()
}

func renderDropProcedure(p *irv1.Procedure) string {
	return "DROP PROCEDURE " + qname(p.GetName()) + renderArgs(p.GetArguments()) + ";"
}

// --- triggers ----------------------------------------------------------------

// renderCreateTrigger renders CREATE [CONSTRAINT] TRIGGER.
func renderCreateTrigger(tr *irv1.Trigger) string {
	var b strings.Builder
	b.WriteString("CREATE ")
	if tr.GetConstraint() {
		b.WriteString("CONSTRAINT ")
	}
	b.WriteString("TRIGGER ")
	b.WriteString(quoteIdent(tr.GetName()))
	switch tr.GetTiming() {
	case irv1.TriggerTiming_TRIGGER_TIMING_BEFORE:
		b.WriteString(" BEFORE")
	case irv1.TriggerTiming_TRIGGER_TIMING_AFTER:
		b.WriteString(" AFTER")
	case irv1.TriggerTiming_TRIGGER_TIMING_INSTEAD_OF:
		b.WriteString(" INSTEAD OF")
	}
	events := make([]string, 0, len(tr.GetEvents()))
	for _, e := range tr.GetEvents() {
		switch e {
		case irv1.TriggerEvent_TRIGGER_EVENT_INSERT:
			events = append(events, "INSERT")
		case irv1.TriggerEvent_TRIGGER_EVENT_DELETE:
			events = append(events, "DELETE")
		case irv1.TriggerEvent_TRIGGER_EVENT_TRUNCATE:
			events = append(events, "TRUNCATE")
		case irv1.TriggerEvent_TRIGGER_EVENT_UPDATE:
			ev := "UPDATE"
			if cols := tr.GetUpdateColumns(); len(cols) > 0 {
				ev += " OF " + quotedCols(cols)
			}
			events = append(events, ev)
		}
	}
	if len(events) > 0 {
		b.WriteString(" ")
		b.WriteString(strings.Join(events, " OR "))
	}
	b.WriteString(" ON ")
	b.WriteString(refTableName(tr.GetTable()))
	switch tr.GetLevel() {
	case irv1.TriggerLevel_TRIGGER_LEVEL_ROW:
		b.WriteString(" FOR EACH ROW")
	case irv1.TriggerLevel_TRIGGER_LEVEL_STATEMENT:
		b.WriteString(" FOR EACH STATEMENT")
	}
	if w := renderExpr(tr.GetWhen()); w != "" {
		b.WriteString(" WHEN (")
		b.WriteString(w)
		b.WriteString(")")
	}
	b.WriteString(" EXECUTE FUNCTION ")
	b.WriteString(triggerFuncName(tr.GetFunction()))
	b.WriteString("(")
	args := make([]string, 0, len(tr.GetArguments()))
	for _, a := range tr.GetArguments() {
		args = append(args, quoteString(a))
	}
	b.WriteString(strings.Join(args, ", "))
	b.WriteString(");")
	return b.String()
}

// triggerFuncName renders the trigger function reference (unquoted-name form so
// the call site looks like func()).
func triggerFuncName(r *irv1.ObjectRef) string {
	if r == nil {
		return ""
	}
	if n := r.GetName(); n != nil {
		return qname(n)
	}
	return quoteIdent(r.GetId())
}

// renderDropTrigger renders DROP TRIGGER .. ON <table>.
func renderDropTrigger(tr *irv1.Trigger) string {
	return "DROP TRIGGER " + quoteIdent(tr.GetName()) + " ON " + refTableName(tr.GetTable()) + ";"
}

// --- query rendering (views / matviews) --------------------------------------

// renderQuery renders a SelectStmt back to SQL text. It prefers the raw_sql
// escape hatch (populated by the introspection path, which stores view bodies
// verbatim) and falls back to a best-effort render of the structured tree
// produced by the parser. If neither yields anything, a placeholder comment is
// returned so the surrounding DDL stays syntactically obvious in golden output.
func renderQuery(s *irv1.SelectStmt) string {
	if s == nil {
		return "/* missing query */"
	}
	if raw := strings.TrimSpace(s.GetRawSql()); raw != "" {
		return raw
	}
	if sel := s.GetSelect(); sel != nil {
		return renderSimpleSelect(sel)
	}
	if so := s.GetSetOperation(); so != nil {
		return renderSetOperation(so)
	}
	return "/* unrenderable query */"
}

func renderSetOperation(so *irv1.SetOperation) string {
	op := "UNION"
	switch so.GetKind() {
	case irv1.SetOpKind_SET_OP_KIND_INTERSECT:
		op = "INTERSECT"
	case irv1.SetOpKind_SET_OP_KIND_EXCEPT:
		op = "EXCEPT"
	}
	if so.GetAll() {
		op += " ALL"
	}
	return renderQuery(so.GetLeft()) + " " + op + " " + renderQuery(so.GetRight())
}

// renderSimpleSelect renders a single SELECT block best-effort.
func renderSimpleSelect(sel *irv1.SimpleSelect) string {
	var b strings.Builder
	b.WriteString("SELECT ")
	if sel.GetDistinct() {
		b.WriteString("DISTINCT ")
	}
	targets := make([]string, 0, len(sel.GetTargets()))
	for _, t := range sel.GetTargets() {
		s := renderSelectExpr(t.GetExpr())
		if a := t.GetAlias(); a != "" {
			s += " AS " + quoteIdent(a)
		}
		targets = append(targets, s)
	}
	if len(targets) == 0 {
		b.WriteString("*")
	} else {
		b.WriteString(strings.Join(targets, ", "))
	}
	if from := sel.GetFrom(); len(from) > 0 {
		parts := make([]string, 0, len(from))
		for _, f := range from {
			parts = append(parts, renderFromItem(f))
		}
		b.WriteString(" FROM ")
		b.WriteString(strings.Join(parts, ", "))
	}
	if w := renderSelectExpr(sel.GetWhere()); w != "" {
		b.WriteString(" WHERE ")
		b.WriteString(w)
	}
	if gb := sel.GetGroupBy(); len(gb) > 0 {
		parts := make([]string, 0, len(gb))
		for _, g := range gb {
			parts = append(parts, renderSelectExpr(g))
		}
		b.WriteString(" GROUP BY ")
		b.WriteString(strings.Join(parts, ", "))
	}
	if h := renderSelectExpr(sel.GetHaving()); h != "" {
		b.WriteString(" HAVING ")
		b.WriteString(h)
	}
	return b.String()
}

// renderFromItem renders a FROM item (table, subquery, or join) best-effort.
func renderFromItem(f *irv1.FromItem) string {
	switch src := f.GetSource().(type) {
	case *irv1.FromItem_Table:
		s := qname(src.Table.GetName())
		if a := src.Table.GetAlias(); a != "" {
			s += " " + quoteIdent(a)
		}
		return s
	case *irv1.FromItem_Subquery:
		s := "(" + renderQuery(src.Subquery.GetQuery()) + ")"
		if a := src.Subquery.GetAlias(); a != "" {
			s += " " + quoteIdent(a)
		}
		return s
	case *irv1.FromItem_Join:
		return renderJoin(src.Join)
	default:
		return "/* unrenderable from */"
	}
}

func renderJoin(j *irv1.JoinClause) string {
	var b strings.Builder
	b.WriteString(renderFromItem(j.GetLeft()))
	if j.GetNatural() {
		b.WriteString(" NATURAL")
	}
	b.WriteString(" ")
	b.WriteString(joinKeyword(j.GetType()))
	b.WriteString(" ")
	b.WriteString(renderFromItem(j.GetRight()))
	if on := renderSelectExpr(j.GetOn()); on != "" {
		b.WriteString(" ON ")
		b.WriteString(on)
	} else if using := j.GetUsing(); len(using) > 0 {
		b.WriteString(" USING (")
		b.WriteString(quotedCols(using))
		b.WriteString(")")
	}
	return b.String()
}

func joinKeyword(t irv1.JoinType) string {
	switch t {
	case irv1.JoinType_JOIN_TYPE_LEFT:
		return "LEFT JOIN"
	case irv1.JoinType_JOIN_TYPE_RIGHT:
		return "RIGHT JOIN"
	case irv1.JoinType_JOIN_TYPE_FULL:
		return "FULL JOIN"
	case irv1.JoinType_JOIN_TYPE_CROSS:
		return "CROSS JOIN"
	default:
		return "JOIN"
	}
}

// renderSelectExpr renders an expression within a query body. Unlike renderExpr
// (which targets DEFAULT/CHECK contexts and prefers raw_sql), this walks the
// common structured shapes so parser-produced view bodies render readably.
func renderSelectExpr(e *irv1.Expr) string {
	if e == nil {
		return ""
	}
	switch n := e.GetNode().(type) {
	case *irv1.Expr_RawSql:
		return n.RawSql
	case *irv1.Expr_Literal:
		return renderLiteral(n.Literal)
	case *irv1.Expr_ColumnRef:
		if q := n.ColumnRef.GetQualifier(); q != "" {
			return quoteIdent(q) + "." + quoteIdent(n.ColumnRef.GetColumn())
		}
		return quoteIdent(n.ColumnRef.GetColumn())
	case *irv1.Expr_Star:
		if q := n.Star.GetQualifier(); q != "" {
			return quoteIdent(q) + ".*"
		}
		return "*"
	case *irv1.Expr_Parameter:
		if nm := n.Parameter.GetName(); nm != "" {
			return ":" + nm
		}
		return fmt.Sprintf("$%d", n.Parameter.GetPosition())
	case *irv1.Expr_Operator:
		return renderOperator(n.Operator)
	case *irv1.Expr_FunctionCall:
		return renderFunctionCall(n.FunctionCall)
	case *irv1.Expr_Cast:
		return renderSelectExpr(n.Cast.GetExpr()) + "::" + renderType(n.Cast.GetTargetType())
	case *irv1.Expr_List:
		parts := make([]string, 0, len(n.List.GetElements()))
		for _, el := range n.List.GetElements() {
			parts = append(parts, renderSelectExpr(el))
		}
		return "(" + strings.Join(parts, ", ") + ")"
	case *irv1.Expr_Subquery:
		return "(" + renderQuery(n.Subquery.GetQuery()) + ")"
	default:
		// Fall back to the DEFAULT/CHECK renderer for anything not modeled here.
		return renderExpr(e)
	}
}

func renderOperator(o *irv1.OperatorExpr) string {
	ops := o.GetOperands()
	sym := o.GetSymbol()
	switch len(ops) {
	case 1:
		// Unary / postfix predicates (IS NULL, NOT, etc.).
		if strings.HasPrefix(sym, "IS ") || sym == "NOTNULL" || sym == "ISNULL" {
			return renderSelectExpr(ops[0]) + " " + sym
		}
		return sym + " " + renderSelectExpr(ops[0])
	case 2:
		return renderSelectExpr(ops[0]) + " " + sym + " " + renderSelectExpr(ops[1])
	default:
		parts := make([]string, 0, len(ops))
		for _, op := range ops {
			parts = append(parts, renderSelectExpr(op))
		}
		return strings.Join(parts, " "+sym+" ")
	}
}

func renderFunctionCall(fc *irv1.FunctionCall) string {
	name := fc.GetName().GetName()
	if sch := fc.GetName().GetSchema(); sch != "" && sch != "pg_catalog" {
		name = quoteIdent(sch) + "." + name
	}
	args := make([]string, 0, len(fc.GetArguments()))
	for _, a := range fc.GetArguments() {
		args = append(args, renderSelectExpr(a))
	}
	inner := strings.Join(args, ", ")
	if fc.GetDistinct() {
		inner = "DISTINCT " + inner
	}
	return name + "(" + inner + ")"
}
