package mapper

// routine.go — DDL mapper extensions for PostgreSQL routine/type objects:
//   CREATE DOMAIN, CREATE TYPE ... AS (composite), CREATE FUNCTION,
//   CREATE PROCEDURE, CREATE TRIGGER, CREATE MATERIALIZED VIEW.
//
// Accessor paths (empirically verified):
//
// CreateDomainStmt:
//   .GetDomainname()     []*Node  — String_ nodes: [schema, name] or [name]
//   .GetTypeName()       *TypeName
//   .GetCollClause()     *CollateClause → .GetCollname() []*Node → sval
//   .GetConstraints()    []*Node → .GetConstraint()
//     CONSTR_NOTNULL → Nullable=false
//     CONSTR_DEFAULT → DefaultExpr via .GetRawExpr()
//     CONSTR_CHECK   → DomainConstraint{Name:.GetConname(), Check:MapExpr(rawExpr)}
//
// CompositeTypeStmt:
//   .GetTypevar()        *RangeVar  — schema + name via GetSchemaname()/GetRelname()
//   .GetColdeflist()     []*Node → .GetColumnDef() → .GetColname(), .GetTypeName()
//
// CreateFunctionStmt:
//   .GetIsProcedure()    bool
//   .GetFuncname()       []*Node  — String_ nodes: [schema, name] or [name]
//   .GetParameters()     []*Node → .GetFunctionParameter()
//     .GetName()         string
//     .GetArgType()      *TypeName
//     .GetMode()         FunctionParameterMode (FUNC_PARAM_IN/OUT/INOUT/VARIADIC/TABLE/DEFAULT)
//     .GetDefexpr()      *Node
//   .GetReturnType()     *TypeName  (.GetSetof() → SETOF; name "void"/"trigger" → special)
//   .GetOptions()        []*Node → .GetDefElem()
//     defname "language"   → arg.GetString_().GetSval()
//     defname "volatility" → arg.GetString_().GetSval() ("immutable"/"stable"/"volatile")
//     defname "strict"     → STRICT/RETURNS NULL ON NULL INPUT
//     defname "security"   → SECURITY DEFINER if boolval true
//     defname "leakproof"  → bool
//     defname "parallel"   → arg.GetString_().GetSval() ("safe"/"restricted"/"unsafe")
//     defname "as"         → arg.GetList().GetItems()[0].GetString_().GetSval() (body)
//
// CreateTrigStmt:
//   .GetTrigname()       string
//   .GetRelation()       *RangeVar  — table the trigger is on
//   .GetFuncname()       []*Node    — trigger function name
//   .GetArgs()           []*Node    — trigger arguments
//   .GetRow()            bool       — true=ROW, false=STATEMENT
//   .GetTiming()         int32      — bitmask: BEFORE=2, AFTER=0, INSTEAD_OF=64
//   .GetEvents()         int32      — bitmask: INSERT=4, UPDATE=16, DELETE=8, TRUNCATE=32
//   .GetColumns()        []*Node    — UPDATE OF columns
//   .GetWhenClause()     *Node
//   .GetIsconstraint()   bool
//
// CreateTableAsStmt (MATERIALIZED VIEW):
//   .GetObjtype()        ObjectType  — OBJECT_MATVIEW for CREATE MATERIALIZED VIEW
//   .GetQuery()          *Node → .GetSelectStmt()
//   .GetInto()           *IntoClause
//     .GetRel()          *RangeVar   — name
//     .GetColNames()     []*Node     — explicit column aliases
//     .GetSkipData()     bool        — WITH NO DATA → skip_data=true
//     .GetTableSpaceName() string

import (
	pg "github.com/pganalyze/pg_query_go/v6"
	"github.com/yaroher/sqld/internal/nodeid"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// Trigger timing and event bitmasks — empirically verified values from
// libpq_query parsing (see comments in triggerTimingFromInt32/triggerEventsFromInt32).
const (
	triggerTimingBefore    = int32(2)  // TRIGGER_TYPE_BEFORE
	triggerTimingInsteadOf = int32(64) // TRIGGER_TYPE_INSTEAD
	// AFTER == neither BEFORE nor INSTEAD set (value 0)

	triggerEventInsert   = int32(4)  // TRIGGER_TYPE_INSERT
	triggerEventDelete   = int32(8)  // TRIGGER_TYPE_DELETE
	triggerEventUpdate   = int32(16) // TRIGGER_TYPE_UPDATE
	triggerEventTruncate = int32(32) // TRIGGER_TYPE_TRUNCATE
)

// ---------------------------------------------------------------------------
// MapCreateDomain
// ---------------------------------------------------------------------------

// MapCreateDomain converts a pg.CreateDomainStmt to an IR DomainType.
// Returns nil for nil input.
func MapCreateDomain(ds *pg.CreateDomainStmt) *irv1.DomainType {
	if ds == nil {
		return nil
	}

	dt := &irv1.DomainType{
		Nullable: true, // default: nullable unless CONSTR_NOTNULL
	}

	// Name — domainname is a list of String_ nodes: [schema, name] or [name]
	dt.Name = typeNodeListToQualifiedName(ds.GetDomainname())

	// Base type
	if tn := ds.GetTypeName(); tn != nil {
		dt.BaseType = MapType(tn)
	}

	// Collation
	if cc := ds.GetCollClause(); cc != nil {
		for _, cn := range cc.GetCollname() {
			if s := cn.GetString_().GetSval(); s != "" {
				dt.Collation = s
				break
			}
		}
	}

	// Constraints
	id := nodeid.New("domain_expr")
	for _, cNode := range ds.GetConstraints() {
		c := cNode.GetConstraint()
		if c == nil {
			continue
		}
		switch c.GetContype() {
		case pg.ConstrType_CONSTR_NOTNULL:
			dt.Nullable = false

		case pg.ConstrType_CONSTR_DEFAULT:
			if raw := c.GetRawExpr(); raw != nil {
				dt.DefaultExpr = MapExpr(raw, id.Child("default"))
			}

		case pg.ConstrType_CONSTR_CHECK:
			dc := &irv1.DomainConstraint{
				Name: c.GetConname(),
			}
			if raw := c.GetRawExpr(); raw != nil {
				dc.Check = MapExpr(raw, id.Child("check"))
			}
			dt.Constraints = append(dt.Constraints, dc)
		}
	}

	return dt
}

// ---------------------------------------------------------------------------
// MapCompositeType
// ---------------------------------------------------------------------------

// MapCompositeType converts a pg.CompositeTypeStmt to an IR CompositeType.
// Returns nil for nil input.
func MapCompositeType(cs *pg.CompositeTypeStmt) *irv1.CompositeType {
	if cs == nil {
		return nil
	}

	ct := &irv1.CompositeType{}

	// Name — typevar is a RangeVar with schema + relname
	if tv := cs.GetTypevar(); tv != nil {
		ct.Name = &irv1.QualifiedName{
			Schema: tv.GetSchemaname(),
			Name:   tv.GetRelname(),
		}
	}

	// Fields
	for _, fNode := range cs.GetColdeflist() {
		cd := fNode.GetColumnDef()
		if cd == nil {
			continue
		}
		f := &irv1.CompositeField{
			Name: cd.GetColname(),
		}
		if tn := cd.GetTypeName(); tn != nil {
			f.Type = MapType(tn)
		}
		// Collation
		if cc := cd.GetCollClause(); cc != nil {
			for _, cn := range cc.GetCollname() {
				if s := cn.GetString_().GetSval(); s != "" {
					f.Collation = s
					break
				}
			}
		}
		ct.Fields = append(ct.Fields, f)
	}

	return ct
}

// ---------------------------------------------------------------------------
// MapCreateFunction
// ---------------------------------------------------------------------------

// MapCreateFunction converts a pg.CreateFunctionStmt to either a Function or
// a Procedure IR object (exactly one will be non-nil).
// Returns (nil, nil) for nil input.
func MapCreateFunction(cf *pg.CreateFunctionStmt) (*irv1.Function, *irv1.Procedure) {
	if cf == nil {
		return nil, nil
	}

	name := typeNodeListToQualifiedName(cf.GetFuncname())
	args := mapFunctionParameters(cf.GetParameters())
	lang, body, volatility, nullInput, secDefiner, leakproof, parallel := parseFunctionOptions(cf.GetOptions())

	if cf.GetIsProcedure() {
		proc := &irv1.Procedure{
			Name:            name,
			Arguments:       args,
			Language:        lang,
			SecurityDefiner: secDefiner,
			Body:            body,
		}
		return nil, proc
	}

	fn := &irv1.Function{
		Name:            name,
		Arguments:       args,
		Language:        lang,
		Body:            body,
		Volatility:      volatility,
		NullInput:       nullInput,
		SecurityDefiner: secDefiner,
		Leakproof:       leakproof,
		Parallel:        parallel,
	}

	// Return type
	if rt := cf.GetReturnType(); rt != nil {
		fn.Returns = mapReturnType(rt)
	}

	return fn, nil
}

// mapFunctionParameters maps a list of FunctionParameter nodes to IR Arguments.
// TABLE-mode params are treated as OUT params (they define the return table).
func mapFunctionParameters(params []*pg.Node) []*irv1.Argument {
	var args []*irv1.Argument
	id := nodeid.New("fn_arg_default")
	for i, pNode := range params {
		fp := pNode.GetFunctionParameter()
		if fp == nil {
			continue
		}
		arg := &irv1.Argument{
			Name: fp.GetName(),
			Mode: mapFuncParamMode(fp.GetMode()),
		}
		if at := fp.GetArgType(); at != nil {
			arg.Type = MapType(at)
		}
		if de := fp.GetDefexpr(); de != nil {
			arg.DefaultValue = MapExpr(de, id.Index(i))
		}
		args = append(args, arg)
	}
	return args
}

// mapFuncParamMode converts pg FunctionParameterMode to IR ArgMode.
func mapFuncParamMode(m pg.FunctionParameterMode) irv1.ArgMode {
	switch m {
	case pg.FunctionParameterMode_FUNC_PARAM_OUT:
		return irv1.ArgMode_ARG_MODE_OUT
	case pg.FunctionParameterMode_FUNC_PARAM_INOUT:
		return irv1.ArgMode_ARG_MODE_INOUT
	case pg.FunctionParameterMode_FUNC_PARAM_VARIADIC:
		return irv1.ArgMode_ARG_MODE_VARIADIC
	default:
		// FUNC_PARAM_IN, FUNC_PARAM_DEFAULT, FUNC_PARAM_TABLE → IN
		return irv1.ArgMode_ARG_MODE_IN
	}
}

// mapReturnType converts a TypeName from CreateFunctionStmt.ReturnType to IR ReturnType.
// Detection order:
//  1. setof=true  → SETOF (even for TABLE returns which also set setof+record)
//  2. name=="void"    → Void
//  3. name=="trigger" → Trigger
//  4. else → Scalar
func mapReturnType(tn *pg.TypeName) *irv1.ReturnType {
	pgName := extractPgName(tn)
	if tn.GetSetof() {
		return &irv1.ReturnType{
			Kind: &irv1.ReturnType_Setof{
				Setof: &irv1.SetofType{Type: MapType(tn)},
			},
		}
	}
	switch pgName {
	case "void":
		return &irv1.ReturnType{Kind: &irv1.ReturnType_VoidValue{VoidValue: true}}
	case "trigger":
		return &irv1.ReturnType{Kind: &irv1.ReturnType_TriggerValue{TriggerValue: true}}
	default:
		return &irv1.ReturnType{Kind: &irv1.ReturnType_Scalar{Scalar: MapType(tn)}}
	}
}

// parseFunctionOptions extracts language, body, volatility, null-input, security,
// leakproof, and parallel settings from the DefElem options list.
func parseFunctionOptions(opts []*pg.Node) (
	lang string,
	body string,
	vol irv1.Volatility,
	nullInput irv1.NullInputBehavior,
	secDefiner bool,
	leakproof bool,
	parallel irv1.ParallelSafety,
) {
	for _, optNode := range opts {
		de := optNode.GetDefElem()
		if de == nil {
			continue
		}
		switch de.GetDefname() {
		case "language":
			lang = de.GetArg().GetString_().GetSval()

		case "as":
			// body is stored as a List with one (or two for C functions) String_ items.
			if lst := de.GetArg().GetList(); lst != nil && len(lst.GetItems()) > 0 {
				body = lst.GetItems()[0].GetString_().GetSval()
			}

		case "volatility":
			switch de.GetArg().GetString_().GetSval() {
			case "immutable":
				vol = irv1.Volatility_VOLATILITY_IMMUTABLE
			case "stable":
				vol = irv1.Volatility_VOLATILITY_STABLE
			case "volatile":
				vol = irv1.Volatility_VOLATILITY_VOLATILE
			}

		case "strict":
			// STRICT / RETURNS NULL ON NULL INPUT
			nullInput = irv1.NullInputBehavior_NULL_INPUT_BEHAVIOR_STRICT

		case "security":
			// SECURITY DEFINER — arg is a Boolean node with boolval=true
			if bv := de.GetArg().GetBoolean(); bv != nil {
				secDefiner = bv.GetBoolval()
			}

		case "leakproof":
			if bv := de.GetArg().GetBoolean(); bv != nil {
				leakproof = bv.GetBoolval()
			}

		case "parallel":
			switch de.GetArg().GetString_().GetSval() {
			case "safe":
				parallel = irv1.ParallelSafety_PARALLEL_SAFETY_SAFE
			case "restricted":
				parallel = irv1.ParallelSafety_PARALLEL_SAFETY_RESTRICTED
			case "unsafe":
				parallel = irv1.ParallelSafety_PARALLEL_SAFETY_UNSAFE
			}
		}
	}
	return
}

// ---------------------------------------------------------------------------
// MapCreateTrigger
// ---------------------------------------------------------------------------

// MapCreateTrigger converts a pg.CreateTrigStmt to an IR Trigger.
// Returns nil for nil input.
//
// Timing bitmask (empirically verified):
//
//	BEFORE    = 2   (bit 1)
//	AFTER     = 0   (no bit set)
//	INSTEAD_OF = 64  (bit 6)
//
// Event bitmask:
//
//	INSERT   = 4   (bit 2)
//	DELETE   = 8   (bit 3)
//	UPDATE   = 16  (bit 4)
//	TRUNCATE = 32  (bit 5)
func MapCreateTrigger(ct *pg.CreateTrigStmt) *irv1.Trigger {
	if ct == nil {
		return nil
	}

	trig := &irv1.Trigger{
		Name:       ct.GetTrigname(),
		Constraint: ct.GetIsconstraint(),
		Timing:     triggerTimingFromInt32(ct.GetTiming()),
		Level:      triggerLevelFromBool(ct.GetRow()),
		Events:     triggerEventsFromInt32(ct.GetEvents()),
	}

	// Table
	if rv := ct.GetRelation(); rv != nil {
		trig.Table = &irv1.ObjectRef{
			Name: rangeVarToQualifiedName(rv),
		}
	}

	// WHEN clause
	if wc := ct.GetWhenClause(); wc != nil {
		id := nodeid.New("trig_when")
		trig.When = MapExpr(wc, id)
	}

	// Trigger function
	if fnName := typeNodeListToQualifiedName(ct.GetFuncname()); fnName != nil {
		trig.Function = &irv1.ObjectRef{Name: fnName}
	}

	// UPDATE OF columns
	for _, colNode := range ct.GetColumns() {
		if s := colNode.GetString_().GetSval(); s != "" {
			trig.UpdateColumns = append(trig.UpdateColumns, s)
		}
	}

	// Arguments (each arg is a String_ node)
	for _, argNode := range ct.GetArgs() {
		if s := argNode.GetString_().GetSval(); s != "" {
			trig.Arguments = append(trig.Arguments, s)
		}
	}

	return trig
}

// triggerTimingFromInt32 decodes the libpg_query timing bitmask.
// BEFORE=2, INSTEAD_OF=64, AFTER=0 (neither bit set).
func triggerTimingFromInt32(t int32) irv1.TriggerTiming {
	switch {
	case t&triggerTimingInsteadOf != 0:
		return irv1.TriggerTiming_TRIGGER_TIMING_INSTEAD_OF
	case t&triggerTimingBefore != 0:
		return irv1.TriggerTiming_TRIGGER_TIMING_BEFORE
	default:
		return irv1.TriggerTiming_TRIGGER_TIMING_AFTER
	}
}

// triggerLevelFromBool converts the row flag to TriggerLevel.
func triggerLevelFromBool(row bool) irv1.TriggerLevel {
	if row {
		return irv1.TriggerLevel_TRIGGER_LEVEL_ROW
	}
	return irv1.TriggerLevel_TRIGGER_LEVEL_STATEMENT
}

// triggerEventsFromInt32 decodes the libpg_query events bitmask to a slice.
// INSERT=4, DELETE=8, UPDATE=16, TRUNCATE=32.
func triggerEventsFromInt32(events int32) []irv1.TriggerEvent {
	var out []irv1.TriggerEvent
	if events&triggerEventInsert != 0 {
		out = append(out, irv1.TriggerEvent_TRIGGER_EVENT_INSERT)
	}
	if events&triggerEventUpdate != 0 {
		out = append(out, irv1.TriggerEvent_TRIGGER_EVENT_UPDATE)
	}
	if events&triggerEventDelete != 0 {
		out = append(out, irv1.TriggerEvent_TRIGGER_EVENT_DELETE)
	}
	if events&triggerEventTruncate != 0 {
		out = append(out, irv1.TriggerEvent_TRIGGER_EVENT_TRUNCATE)
	}
	return out
}

// ---------------------------------------------------------------------------
// MapMaterializedView
// ---------------------------------------------------------------------------

// MapMaterializedView converts a pg.CreateTableAsStmt to an IR MaterializedView.
// Returns nil for nil input or if the statement is not a materialized view.
func MapMaterializedView(cta *pg.CreateTableAsStmt) *irv1.MaterializedView {
	if cta == nil {
		return nil
	}
	// Guard: only handle CREATE MATERIALIZED VIEW statements.
	if cta.GetObjtype() != pg.ObjectType_OBJECT_MATVIEW {
		return nil
	}

	mv := &irv1.MaterializedView{}

	into := cta.GetInto()
	if into != nil {
		// Name
		if rv := into.GetRel(); rv != nil {
			mv.Name = rangeVarToQualifiedName(rv)
		}
		// Explicit column aliases
		for _, cn := range into.GetColNames() {
			if s := cn.GetString_().GetSval(); s != "" {
				mv.Columns = append(mv.Columns, s)
			}
		}
		// WITH DATA vs WITH NO DATA
		mv.WithData = !into.GetSkipData()
		// Tablespace
		mv.Tablespace = into.GetTableSpaceName()
	}

	// Query
	if q := cta.GetQuery(); q != nil {
		if sel := q.GetSelectStmt(); sel != nil {
			id := nodeid.New("matview_query")
			mv.Query = mapSelectStmt(sel, id)
		}
	}

	return mv
}
