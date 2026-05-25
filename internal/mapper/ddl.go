package mapper

// DDL mapper: translates libpg_query DDL AST nodes to sqld IR schema objects.
//
// Accessor paths used (for Task 10 reference):
//
// CreateStmt:
//   .GetRelation()                  *pg.RangeVar  — table name + persistence
//     .GetSchemaname()              string
//     .GetRelname()                 string
//     .GetRelpersistence()          string  "p"=PERMANENT "u"=UNLOGGED "t"=TEMPORARY
//   .GetTableElts()                 []*pg.Node
//     .GetColumnDef()               *pg.ColumnDef  — column element
//     .GetConstraint()              *pg.Constraint — table-level constraint element
//
// ColumnDef:
//   .GetColname()                   string
//   .GetTypeName()                  *pg.TypeName
//   .GetIsNotNull()                 bool
//   .GetConstraints()               []*pg.Node
//     .GetConstraint().GetContype() ConstrType
//       CONSTR_NOTNULL  (2)  → Nullable=false
//       CONSTR_DEFAULT  (3)  → DefaultExpr via .GetRawExpr()
//       CONSTR_IDENTITY (4)  → Identity via .GetGeneratedWhen() "a"/"d"
//       CONSTR_GENERATED(5)  → GeneratedColumn via .GetRawExpr()/.GetGeneratedWhen()
//       CONSTR_CHECK    (6)  → emitted as table-level check constraint
//       CONSTR_PRIMARY  (7)  → Nullable=false; emitted as table-level PK constraint
//       CONSTR_UNIQUE   (8)  → emitted as table-level unique constraint
//       CONSTR_FOREIGN  (10) → emitted as table-level FK constraint
//   .GetCollClause().GetCollname()  []*pg.Node → .GetString_().GetSval()
//
// Constraint (table-level):
//   .GetContype()                   ConstrType (same enum values)
//   .GetConname()                   string  — constraint name
//   .GetDeferrable()                bool
//   .GetInitdeferred()              bool
//   .GetKeys()                      []*pg.Node → .GetString_().GetSval()   — PK/UNIQUE/NN columns
//   .GetPktable()                   *pg.RangeVar  — FK referenced table
//   .GetFkAttrs()                   []*pg.Node  → local FK columns
//   .GetPkAttrs()                   []*pg.Node  → referenced FK columns
//   .GetFkDelAction()               string  single-char: a=NO_ACTION r=RESTRICT c=CASCADE n=SET_NULL d=SET_DEFAULT
//   .GetFkUpdAction()               string  same chars
//   .GetFkMatchtype()               string  "s"=SIMPLE "f"=FULL "p"=PARTIAL
//   .GetRawExpr()                   *pg.Node  — CHECK expression
//   .GetIsNoInherit()               bool  — CHECK NO INHERIT
//   .GetNullsNotDistinct()          bool  — UNIQUE NULLS NOT DISTINCT
//   .GetIndexname()                 string  — backing index name (PK/UNIQUE)
//
// IndexStmt:
//   .GetIdxname()                   string
//   .GetRelation()                  *pg.RangeVar  — table being indexed
//   .GetAccessMethod()              string
//   .GetUnique()                    bool
//   .GetPrimary()                   bool
//   .GetNullsNotDistinct()          bool
//   .GetIndexParams()               []*pg.Node → .GetIndexElem()
//     .GetName()                    string  — column name (empty for expression index)
//     .GetExpr()                    *pg.Node — expression (when Name=="")
//     .GetOrdering()                SortByDir
//     .GetNullsOrdering()           SortByNulls
//     .GetCollation()               []*pg.Node → .GetString_().GetSval()
//     .GetOpclass()                 []*pg.Node → .GetString_().GetSval()
//   .GetIndexIncludingParams()      []*pg.Node → .GetIndexElem().GetName() (INCLUDE cols)
//   .GetWhereClause()               *pg.Node
//
// ViewStmt:
//   .GetView()                      *pg.RangeVar
//   .GetAliases()                   []*pg.Node → .GetString_().GetSval()
//   .GetQuery()                     *pg.Node → .GetSelectStmt()
//   .GetWithCheckOption()           ViewCheckOption  LOCAL_CHECK_OPTION / CASCADED_CHECK_OPTION
//   .GetOptions()                   []*pg.Node DefElem; "security_barrier" → GetBoolval
//
// CreateEnumStmt:
//   .GetTypeName()                  []*pg.Node → .GetString_().GetSval()
//   .GetVals()                      []*pg.Node → .GetString_().GetSval()
//
// CreateSeqStmt:
//   .GetSequence()                  *pg.RangeVar
//   .GetOptions()                   []*pg.Node DefElem
//     defname "start", "increment", "minvalue", "maxvalue", "cache", "cycle"
//     arg → GetIval().GetIval() for integers
//
// bigserial / serial in the AST:
//   bigserial parses as type "int8" (pg_catalog) with a CONSTR_DEFAULT constraint
//   containing nextval('...') expression, plus an implicit CONSTR_PRIMARY or none.
//   It does NOT produce an Identity or CONSTR_IDENTITY node — that is only for
//   GENERATED ... AS IDENTITY syntax. The serial pseudo-type is sugar for
//   DEFAULT nextval(); the backing sequence is implicit and not in the parse tree.

import (
	pg "github.com/pganalyze/pg_query_go/v6"
	"github.com/yaroher/sqld/internal/nodeid"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// ---------------------------------------------------------------------------
// MapCreateTable
// ---------------------------------------------------------------------------

// MapCreateTable converts a pg.CreateStmt to an IR Table.
// Returns nil for nil input.
func MapCreateTable(cs *pg.CreateStmt) *irv1.Table {
	if cs == nil {
		return nil
	}

	tbl := &irv1.Table{}

	// Name + persistence
	if rv := cs.GetRelation(); rv != nil {
		tbl.Name = rangeVarToQualifiedName(rv)
		tbl.Persistence = mapRelpersistence(rv.GetRelpersistence())
	}

	// Iterate table elements
	var pos uint32
	for _, elt := range cs.GetTableElts() {
		switch {
		case elt.GetColumnDef() != nil:
			cd := elt.GetColumnDef()
			pos++
			col, colConstraints := mapColumnAndConstraints(cd, pos)
			tbl.Columns = append(tbl.Columns, col)
			tbl.Constraints = append(tbl.Constraints, colConstraints...)

		case elt.GetConstraint() != nil:
			c := elt.GetConstraint()
			if ic := MapConstraint(c, ""); ic != nil {
				tbl.Constraints = append(tbl.Constraints, ic)
			}
		}
	}

	return tbl
}

// mapRelpersistence converts the single-character persistence flag.
func mapRelpersistence(s string) irv1.TablePersistence {
	switch s {
	case "u":
		return irv1.TablePersistence_TABLE_PERSISTENCE_UNLOGGED
	case "t":
		return irv1.TablePersistence_TABLE_PERSISTENCE_TEMPORARY
	default: // "p" or empty
		return irv1.TablePersistence_TABLE_PERSISTENCE_PERMANENT
	}
}

// ---------------------------------------------------------------------------
// MapColumn
// ---------------------------------------------------------------------------

// MapColumn converts a pg.ColumnDef to an IR Column (position is 1-based).
// Returns nil for nil input.
func MapColumn(cd *pg.ColumnDef, pos uint32) *irv1.Column {
	if cd == nil {
		return nil
	}
	col := &irv1.Column{
		Name:     cd.GetColname(),
		Position: pos,
		Nullable: !cd.GetIsNotNull(), // default nullable unless IS NOT NULL
	}

	// Type
	if tn := cd.GetTypeName(); tn != nil {
		col.Type = MapType(tn)
	}

	// Collation
	if cc := cd.GetCollClause(); cc != nil {
		for _, cn := range cc.GetCollname() {
			if s := cn.GetString_().GetSval(); s != "" {
				col.Collation = s
				break
			}
		}
	}

	// Walk column-level constraints to set flags/identity/generated/default
	id := nodeid.New("col_expr")
	for _, cNode := range cd.GetConstraints() {
		c := cNode.GetConstraint()
		if c == nil {
			continue
		}
		switch c.GetContype() {
		case pg.ConstrType_CONSTR_NOTNULL:
			col.Nullable = false

		case pg.ConstrType_CONSTR_PRIMARY:
			// PK implies NOT NULL
			col.Nullable = false

		case pg.ConstrType_CONSTR_DEFAULT:
			if raw := c.GetRawExpr(); raw != nil {
				col.DefaultExpr = MapExpr(raw, id.Child("default"))
			}

		case pg.ConstrType_CONSTR_IDENTITY:
			col.Nullable = false
			col.Identity = &irv1.Identity{
				Kind: mapIdentityKind(c.GetGeneratedWhen()),
			}

		case pg.ConstrType_CONSTR_GENERATED:
			if raw := c.GetRawExpr(); raw != nil {
				col.Generated = &irv1.GeneratedColumn{
					Expression: MapExpr(raw, id.Child("gen")),
					Stored:     c.GetGeneratedWhen() == "s",
				}
			}
		}
	}

	return col
}

// mapIdentityKind maps the generated_when char ("a" / "d") to IdentityKind.
func mapIdentityKind(when string) irv1.IdentityKind {
	switch when {
	case "a":
		return irv1.IdentityKind_IDENTITY_KIND_ALWAYS
	case "d":
		return irv1.IdentityKind_IDENTITY_KIND_BY_DEFAULT
	default:
		return irv1.IdentityKind_IDENTITY_KIND_UNSPECIFIED
	}
}

// mapColumnAndConstraints maps a ColumnDef to a Column + any table-level
// constraints derived from column-level constraint declarations (PK, UNIQUE,
// FK, CHECK, NOTNULL).
func mapColumnAndConstraints(cd *pg.ColumnDef, pos uint32) (*irv1.Column, []*irv1.Constraint) {
	col := MapColumn(cd, pos)
	var constraints []*irv1.Constraint

	for _, cNode := range cd.GetConstraints() {
		c := cNode.GetConstraint()
		if c == nil {
			continue
		}
		switch c.GetContype() {
		case pg.ConstrType_CONSTR_PRIMARY,
			pg.ConstrType_CONSTR_UNIQUE,
			pg.ConstrType_CONSTR_FOREIGN,
			pg.ConstrType_CONSTR_CHECK,
			pg.ConstrType_CONSTR_NOTNULL:
			if ic := MapConstraint(c, cd.GetColname()); ic != nil {
				constraints = append(constraints, ic)
			}
		}
	}

	return col, constraints
}

// ---------------------------------------------------------------------------
// MapConstraint
// ---------------------------------------------------------------------------

// MapConstraint converts a pg.Constraint to an IR Constraint.
// fallbackCol is the column name to use when Keys is empty (column-level constraint).
// Returns nil for nil input or for constraint types that produce no IR object.
func MapConstraint(c *pg.Constraint, fallbackCol string) *irv1.Constraint {
	if c == nil {
		return nil
	}

	base := &irv1.Constraint{
		Name:              c.GetConname(),
		Deferrable:        c.GetDeferrable(),
		InitiallyDeferred: c.GetInitdeferred(),
	}

	// Helper: collect string keys (PK/UNIQUE/NN column lists)
	keyCols := func() []string {
		keys := c.GetKeys()
		if len(keys) == 0 {
			if fallbackCol != "" {
				return []string{fallbackCol}
			}
			return nil
		}
		out := make([]string, 0, len(keys))
		for _, k := range keys {
			if s := k.GetString_().GetSval(); s != "" {
				out = append(out, s)
			}
		}
		return out
	}

	switch c.GetContype() {
	case pg.ConstrType_CONSTR_PRIMARY:
		base.Type = irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY
		base.Body = &irv1.Constraint_PrimaryKey{
			PrimaryKey: &irv1.PrimaryKey{
				Columns: keyCols(),
				Index:   c.GetIndexname(),
			},
		}

	case pg.ConstrType_CONSTR_UNIQUE:
		base.Type = irv1.ConstraintType_CONSTRAINT_TYPE_UNIQUE
		base.Body = &irv1.Constraint_Unique{
			Unique: &irv1.UniqueConstraint{
				Columns:          keyCols(),
				NullsNotDistinct: c.GetNullsNotDistinct(),
				Index:            c.GetIndexname(),
			},
		}

	case pg.ConstrType_CONSTR_FOREIGN:
		base.Type = irv1.ConstraintType_CONSTRAINT_TYPE_FOREIGN_KEY
		fk := &irv1.ForeignKey{
			OnDelete:  mapFKAction(c.GetFkDelAction()),
			OnUpdate:  mapFKAction(c.GetFkUpdAction()),
			MatchType: mapFKMatch(c.GetFkMatchtype()),
		}
		// Local columns: FkAttrs (table-level) or fallbackCol (column-level)
		if attrs := c.GetFkAttrs(); len(attrs) > 0 {
			for _, a := range attrs {
				if s := a.GetString_().GetSval(); s != "" {
					fk.Columns = append(fk.Columns, s)
				}
			}
		} else if fallbackCol != "" {
			fk.Columns = []string{fallbackCol}
		}
		// Referenced table
		if pkt := c.GetPktable(); pkt != nil {
			fk.ReferencedTable = &irv1.ObjectRef{
				Name: rangeVarToQualifiedName(pkt),
			}
		}
		// Referenced columns
		for _, a := range c.GetPkAttrs() {
			if s := a.GetString_().GetSval(); s != "" {
				fk.ReferencedColumns = append(fk.ReferencedColumns, s)
			}
		}
		base.Body = &irv1.Constraint_ForeignKey{ForeignKey: fk}

	case pg.ConstrType_CONSTR_CHECK:
		base.Type = irv1.ConstraintType_CONSTRAINT_TYPE_CHECK
		cc := &irv1.CheckConstraint{
			NoInherit: c.GetIsNoInherit(),
		}
		if raw := c.GetRawExpr(); raw != nil {
			id := nodeid.New("check_expr")
			cc.Expression = MapExpr(raw, id)
		}
		base.Body = &irv1.Constraint_Check{Check: cc}

	case pg.ConstrType_CONSTR_NOTNULL:
		base.Type = irv1.ConstraintType_CONSTRAINT_TYPE_NOT_NULL
		col := fallbackCol
		if col == "" {
			if cols := keyCols(); len(cols) > 0 {
				col = cols[0]
			}
		}
		base.Body = &irv1.Constraint_NotNullColumn{NotNullColumn: col}

	default:
		// Unsupported type — return nil so caller can skip.
		return nil
	}

	return base
}

// mapFKAction maps a single-char FK action to IR ReferentialAction.
// a=NO_ACTION, r=RESTRICT, c=CASCADE, n=SET_NULL, d=SET_DEFAULT
func mapFKAction(ch string) irv1.ReferentialAction {
	switch ch {
	case "a":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_NO_ACTION
	case "r":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_RESTRICT
	case "c":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_CASCADE
	case "n":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_SET_NULL
	case "d":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_SET_DEFAULT
	default:
		return irv1.ReferentialAction_REFERENTIAL_ACTION_UNSPECIFIED
	}
}

// mapFKMatch maps FK match-type char to IR MatchType.
// s=SIMPLE, f=FULL, p=PARTIAL (empty → UNSPECIFIED = SIMPLE)
func mapFKMatch(ch string) irv1.MatchType {
	switch ch {
	case "f":
		return irv1.MatchType_MATCH_TYPE_FULL
	case "p":
		return irv1.MatchType_MATCH_TYPE_PARTIAL
	default: // "s" or ""
		return irv1.MatchType_MATCH_TYPE_UNSPECIFIED
	}
}

// ---------------------------------------------------------------------------
// MapIndex
// ---------------------------------------------------------------------------

// MapIndex converts a pg.IndexStmt to an IR Index.
// Returns nil for nil input.
func MapIndex(is *pg.IndexStmt) *irv1.Index {
	if is == nil {
		return nil
	}

	idx := &irv1.Index{
		Name:             is.GetIdxname(),
		Method:           is.GetAccessMethod(),
		Unique:           is.GetUnique(),
		Primary:          is.GetPrimary(),
		NullsNotDistinct: is.GetNullsNotDistinct(),
	}

	// Index elements (key columns)
	id := nodeid.New("idx_expr")
	for i, paramNode := range is.GetIndexParams() {
		ie := paramNode.GetIndexElem()
		if ie == nil {
			continue
		}
		elem := mapIndexElem(ie, id.Index(i))
		idx.Elements = append(idx.Elements, elem)
	}

	// INCLUDE columns
	for _, incNode := range is.GetIndexIncludingParams() {
		ie := incNode.GetIndexElem()
		if ie == nil {
			continue
		}
		if ie.GetName() != "" {
			idx.Include = append(idx.Include, ie.GetName())
		}
	}

	// WHERE predicate
	if wc := is.GetWhereClause(); wc != nil {
		idx.Predicate = MapExpr(wc, id.Child("pred"))
	}

	return idx
}

// mapIndexElem converts a pg.IndexElem to an IR IndexElement.
func mapIndexElem(ie *pg.IndexElem, id nodeid.Builder) *irv1.IndexElement {
	elem := &irv1.IndexElement{
		Order: mapSortByDir(ie.GetOrdering()),
		Nulls: mapNullsOrdering(ie.GetNullsOrdering()),
	}

	// Collation: first sval from collation list
	for _, cn := range ie.GetCollation() {
		if s := cn.GetString_().GetSval(); s != "" {
			elem.Collation = s
			break
		}
	}

	// Opclass: first sval from opclass list
	for _, op := range ie.GetOpclass() {
		if s := op.GetString_().GetSval(); s != "" {
			elem.Opclass = s
			break
		}
	}

	// Column or expression
	if ie.GetName() != "" {
		elem.Target = &irv1.IndexElement_Column{Column: ie.GetName()}
	} else if ie.GetExpr() != nil {
		elem.Target = &irv1.IndexElement_Expr{
			Expr: MapExpr(ie.GetExpr(), id.Child("elem_expr")),
		}
	}

	return elem
}

// mapSortByDir converts pg SortByDir to IR SortOrder.
func mapSortByDir(d pg.SortByDir) irv1.SortOrder {
	switch d {
	case pg.SortByDir_SORTBY_ASC:
		return irv1.SortOrder_SORT_ORDER_ASC
	case pg.SortByDir_SORTBY_DESC:
		return irv1.SortOrder_SORT_ORDER_DESC
	default:
		return irv1.SortOrder_SORT_ORDER_UNSPECIFIED
	}
}

// mapNullsOrdering converts pg SortByNulls to IR NullsOrder.
func mapNullsOrdering(n pg.SortByNulls) irv1.NullsOrder {
	switch n {
	case pg.SortByNulls_SORTBY_NULLS_FIRST:
		return irv1.NullsOrder_NULLS_ORDER_FIRST
	case pg.SortByNulls_SORTBY_NULLS_LAST:
		return irv1.NullsOrder_NULLS_ORDER_LAST
	default:
		return irv1.NullsOrder_NULLS_ORDER_UNSPECIFIED
	}
}

// ---------------------------------------------------------------------------
// MapView
// ---------------------------------------------------------------------------

// MapView converts a pg.ViewStmt to an IR View.
// Returns nil for nil input.
func MapView(vs *pg.ViewStmt) *irv1.View {
	if vs == nil {
		return nil
	}

	v := &irv1.View{}

	// Name
	if rv := vs.GetView(); rv != nil {
		v.Name = rangeVarToQualifiedName(rv)
	}

	// Column aliases
	for _, a := range vs.GetAliases() {
		if s := a.GetString_().GetSval(); s != "" {
			v.Columns = append(v.Columns, s)
		}
	}

	// Query
	id := nodeid.New("view_query")
	if q := vs.GetQuery().GetSelectStmt(); q != nil {
		v.Query = mapSelectStmt(q, id)
	}

	// WITH CHECK OPTION
	switch vs.GetWithCheckOption() {
	case pg.ViewCheckOption_LOCAL_CHECK_OPTION:
		v.CheckOption = irv1.CheckOption_CHECK_OPTION_LOCAL
	case pg.ViewCheckOption_CASCADED_CHECK_OPTION:
		v.CheckOption = irv1.CheckOption_CHECK_OPTION_CASCADED
	}

	// Security barrier option
	for _, optNode := range vs.GetOptions() {
		de := optNode.GetDefElem()
		if de == nil {
			continue
		}
		if de.GetDefname() == "security_barrier" {
			if bv := de.GetArg().GetBoolean(); bv != nil {
				v.SecurityBarrier = bv.GetBoolval()
			} else if iv := de.GetArg().GetAConst(); iv != nil {
				v.SecurityBarrier = iv.GetBoolval() != nil && iv.GetBoolval().GetBoolval()
			}
		}
	}

	return v
}

// ---------------------------------------------------------------------------
// MapCreateEnum
// ---------------------------------------------------------------------------

// MapCreateEnum converts a pg.CreateEnumStmt to an IR EnumType.
// Returns nil for nil input.
func MapCreateEnum(es *pg.CreateEnumStmt) *irv1.EnumType {
	if es == nil {
		return nil
	}

	et := &irv1.EnumType{}

	// Type name — TypeName list: may be [schema, name] or [name]
	typeNodes := es.GetTypeName()
	et.Name = typeNodeListToQualifiedName(typeNodes)

	// Labels
	for _, valNode := range es.GetVals() {
		if s := valNode.GetString_().GetSval(); s != "" {
			et.Labels = append(et.Labels, s)
		}
	}

	return et
}

// typeNodeListToQualifiedName converts a TypeName node list (as used in
// CreateEnumStmt.TypeName) to a QualifiedName.
// Elements are plain String_ nodes; last element is Name, second-to-last is Schema.
func typeNodeListToQualifiedName(nodes []*pg.Node) *irv1.QualifiedName {
	parts := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if s := n.GetString_().GetSval(); s != "" {
			parts = append(parts, s)
		}
	}
	qn := &irv1.QualifiedName{}
	switch len(parts) {
	case 0:
		// empty
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
// MapCreateSequence
// ---------------------------------------------------------------------------

// MapCreateSequence converts a pg.CreateSeqStmt to an IR Sequence.
// Sequence options are parsed best-effort from the DefElem option list.
// Returns nil for nil input.
func MapCreateSequence(ss *pg.CreateSeqStmt) *irv1.Sequence {
	if ss == nil {
		return nil
	}

	seq := &irv1.Sequence{}

	// Name
	if rv := ss.GetSequence(); rv != nil {
		seq.Name = rangeVarToQualifiedName(rv)
	}

	// Options: each option is a DefElem node
	for _, optNode := range ss.GetOptions() {
		de := optNode.GetDefElem()
		if de == nil {
			continue
		}
		name := de.GetDefname()
		arg := de.GetArg()
		switch name {
		case "start":
			seq.Start = defElemInt64(arg)
		case "increment":
			seq.Increment = defElemInt64(arg)
		case "minvalue":
			seq.MinValue = defElemInt64(arg)
		case "maxvalue":
			seq.MaxValue = defElemInt64(arg)
		case "cache":
			seq.Cache = defElemInt64(arg)
		case "cycle":
			// cycle is a boolean; when present as "cycle" without a value it means true
			seq.Cycle = defElemBool(arg)
		case "as":
			// AS <type>: stored in DataType
			if tn := arg.GetTypeName(); tn != nil {
				seq.DataType = MapType(tn)
			}
		case "owned_by":
			// OWNED BY table.col — best effort: leave OwnedBy empty (catalog resolves)
		}
	}

	return seq
}

// defElemInt64 extracts an integer value from a DefElem arg node.
// It handles Integer (IVal) and TypeCast wrapping best-effort.
func defElemInt64(arg *pg.Node) int64 {
	if arg == nil {
		return 0
	}
	if iv := arg.GetInteger(); iv != nil {
		return int64(iv.GetIval())
	}
	if ac := arg.GetAConst(); ac != nil {
		if iv := ac.GetIval(); iv != nil {
			return int64(iv.GetIval())
		}
	}
	return 0
}

// defElemBool extracts a boolean from a DefElem arg node.
func defElemBool(arg *pg.Node) bool {
	if arg == nil {
		// bare "cycle" keyword has no arg — treat as true
		return true
	}
	if iv := arg.GetInteger(); iv != nil {
		return iv.GetIval() != 0
	}
	return false
}
