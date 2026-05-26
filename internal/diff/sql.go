package diff

import (
	"fmt"
	"strconv"
	"strings"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
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
			base = baseTypeName(t.GetPgName(), t.GetModifier())
		}
		dims := int(t.GetArrayDimensions())
		if dims < 1 {
			dims = 1
		}
		return base + strings.Repeat("[]", dims)
	}
	return baseTypeName(t.GetPgName(), t.GetModifier())
}

// baseTypeName renders a scalar pg_name plus its modifier.
func baseTypeName(pg string, mod *irv1.TypeModifier) string {
	name := pg
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
