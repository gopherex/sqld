package mapper

import (
	"testing"

	"github.com/gopherex/sqld/internal/parse"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

// ---------------------------------------------------------------------------
// TestMapCreateDomain
// ---------------------------------------------------------------------------

func TestMapCreateDomain(t *testing.T) {
	stmts, _ := parse.Statements("CREATE DOMAIN app.email AS text NOT NULL CHECK (VALUE ~ '@');")
	d := MapCreateDomain(stmts[0].Node.GetCreateDomainStmt())
	if d == nil {
		t.Fatal("MapCreateDomain returned nil")
	}
	if d.GetName().GetName() != "email" || d.GetName().GetSchema() != "app" {
		t.Fatalf("name=%v", d.GetName())
	}
	if d.GetBaseType().GetPgName() != "text" {
		t.Fatalf("base=%v", d.GetBaseType())
	}
	if d.GetNullable() {
		t.Fatal("should be NOT NULL → Nullable=false")
	}
	if len(d.GetConstraints()) != 1 {
		t.Fatalf("constraints=%d", len(d.GetConstraints()))
	}
}

func TestMapCreateDomain_Default(t *testing.T) {
	stmts, _ := parse.Statements("CREATE DOMAIN app.score AS int4 DEFAULT 0;")
	d := MapCreateDomain(stmts[0].Node.GetCreateDomainStmt())
	if d == nil {
		t.Fatal("nil")
	}
	// nullable should be true by default (no NOT NULL constraint)
	if !d.GetNullable() {
		t.Fatal("want Nullable=true by default")
	}
	if d.GetDefaultExpr() == nil {
		t.Fatal("want default expr")
	}
}

func TestMapCreateDomain_Nil(t *testing.T) {
	if MapCreateDomain(nil) != nil {
		t.Error("expected nil for nil CreateDomainStmt")
	}
}

// ---------------------------------------------------------------------------
// TestMapCompositeType
// ---------------------------------------------------------------------------

func TestMapCompositeType(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TYPE app.address AS (street text, zip int4);")
	c := MapCompositeType(stmts[0].Node.GetCompositeTypeStmt())
	if c == nil {
		t.Fatal("MapCompositeType returned nil")
	}
	if c.GetName().GetName() != "address" {
		t.Fatalf("name=%v", c.GetName())
	}
	if c.GetName().GetSchema() != "app" {
		t.Fatalf("schema=%v", c.GetName())
	}
	if len(c.GetFields()) != 2 {
		t.Fatalf("composite fields=%d", len(c.GetFields()))
	}
	if c.GetFields()[0].GetName() != "street" {
		t.Fatalf("field[0].name=%q", c.GetFields()[0].GetName())
	}
	if c.GetFields()[0].GetType().GetPgName() != "text" {
		t.Fatalf("field[0].type=%v", c.GetFields()[0].GetType())
	}
}

func TestMapCompositeType_Nil(t *testing.T) {
	if MapCompositeType(nil) != nil {
		t.Error("expected nil for nil CompositeTypeStmt")
	}
}

// ---------------------------------------------------------------------------
// TestMapCreateFunction
// ---------------------------------------------------------------------------

func TestMapCreateFunction(t *testing.T) {
	stmts, _ := parse.Statements("CREATE FUNCTION app.add(a int4, b int4) RETURNS int4 LANGUAGE sql AS $$ SELECT a + b $$;")
	fn, proc := MapCreateFunction(stmts[0].Node.GetCreateFunctionStmt())
	if proc != nil || fn == nil {
		t.Fatalf("want function, got fn=%v proc=%v", fn, proc)
	}
	if fn.GetName().GetName() != "add" {
		t.Fatalf("fn.name=%q", fn.GetName().GetName())
	}
	if fn.GetName().GetSchema() != "app" {
		t.Fatalf("fn.schema=%q", fn.GetName().GetSchema())
	}
	if len(fn.GetArguments()) != 2 {
		t.Fatalf("fn.args=%d", len(fn.GetArguments()))
	}
	if fn.GetLanguage() != "sql" {
		t.Fatalf("lang=%q", fn.GetLanguage())
	}
	if fn.GetReturns().GetScalar().GetPgName() != "int4" {
		t.Fatalf("ret=%v", fn.GetReturns())
	}
	if fn.GetBody() == "" {
		t.Fatal("body should not be empty")
	}
}

func TestMapCreateFunction_ReturnVoid(t *testing.T) {
	stmts, _ := parse.Statements("CREATE FUNCTION f() RETURNS void LANGUAGE plpgsql AS $$ BEGIN END $$;")
	fn, _ := MapCreateFunction(stmts[0].Node.GetCreateFunctionStmt())
	if fn == nil {
		t.Fatal("nil function")
	}
	if !fn.GetReturns().GetVoidValue() {
		t.Fatalf("want void return, got %v", fn.GetReturns())
	}
}

func TestMapCreateFunction_ReturnTrigger(t *testing.T) {
	stmts, _ := parse.Statements("CREATE FUNCTION f() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;")
	fn, _ := MapCreateFunction(stmts[0].Node.GetCreateFunctionStmt())
	if fn == nil {
		t.Fatal("nil function")
	}
	if !fn.GetReturns().GetTriggerValue() {
		t.Fatalf("want trigger return, got %v", fn.GetReturns())
	}
}

func TestMapCreateFunction_ReturnSetof(t *testing.T) {
	stmts, _ := parse.Statements("CREATE FUNCTION f() RETURNS SETOF int4 LANGUAGE sql AS $$ SELECT 1 $$;")
	fn, _ := MapCreateFunction(stmts[0].Node.GetCreateFunctionStmt())
	if fn == nil {
		t.Fatal("nil function")
	}
	if fn.GetReturns().GetSetof() == nil {
		t.Fatalf("want setof return, got %v", fn.GetReturns())
	}
	if fn.GetReturns().GetSetof().GetType().GetPgName() != "int4" {
		t.Fatalf("setof type=%v", fn.GetReturns().GetSetof().GetType())
	}
}

func TestMapCreateFunction_Volatility(t *testing.T) {
	stmts, _ := parse.Statements("CREATE FUNCTION f() RETURNS void LANGUAGE plpgsql AS $$ BEGIN END $$ IMMUTABLE;")
	fn, _ := MapCreateFunction(stmts[0].Node.GetCreateFunctionStmt())
	if fn == nil {
		t.Fatal("nil function")
	}
	if fn.GetVolatility() != irv1.Volatility_VOLATILITY_IMMUTABLE {
		t.Fatalf("volatility=%v", fn.GetVolatility())
	}
}

func TestMapCreateFunction_ArgModes(t *testing.T) {
	stmts, _ := parse.Statements("CREATE FUNCTION f(INOUT x int4, VARIADIC y text[]) RETURNS SETOF text LANGUAGE sql AS '' ;")
	fn, _ := MapCreateFunction(stmts[0].Node.GetCreateFunctionStmt())
	if fn == nil {
		t.Fatal("nil function")
	}
	args := fn.GetArguments()
	if len(args) != 2 {
		t.Fatalf("args=%d", len(args))
	}
	if args[0].GetMode() != irv1.ArgMode_ARG_MODE_INOUT {
		t.Fatalf("arg[0] mode=%v", args[0].GetMode())
	}
	if args[1].GetMode() != irv1.ArgMode_ARG_MODE_VARIADIC {
		t.Fatalf("arg[1] mode=%v", args[1].GetMode())
	}
}

func TestMapCreateProcedure(t *testing.T) {
	stmts, _ := parse.Statements("CREATE PROCEDURE app.p(x int4) LANGUAGE plpgsql AS $$ BEGIN END $$;")
	fn, proc := MapCreateFunction(stmts[0].Node.GetCreateFunctionStmt())
	if fn != nil || proc == nil {
		t.Fatalf("want procedure, got fn=%v proc=%v", fn, proc)
	}
	if proc.GetName().GetName() != "p" {
		t.Fatalf("proc.name=%q", proc.GetName().GetName())
	}
	if len(proc.GetArguments()) != 1 {
		t.Fatalf("proc.args=%d", len(proc.GetArguments()))
	}
	if proc.GetLanguage() != "plpgsql" {
		t.Fatalf("lang=%q", proc.GetLanguage())
	}
}

func TestMapCreateFunction_Nil(t *testing.T) {
	fn, proc := MapCreateFunction(nil)
	if fn != nil || proc != nil {
		t.Error("expected nil for nil CreateFunctionStmt")
	}
}

// ---------------------------------------------------------------------------
// TestMapCreateTrigger
// ---------------------------------------------------------------------------

func TestMapCreateTrigger(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TRIGGER t BEFORE INSERT OR UPDATE ON app.users FOR EACH ROW EXECUTE FUNCTION app.touch();")
	tr := MapCreateTrigger(stmts[0].Node.GetCreateTrigStmt())
	if tr == nil {
		t.Fatal("MapCreateTrigger returned nil")
	}
	if tr.GetName() != "t" {
		t.Fatalf("name=%q", tr.GetName())
	}
	if tr.GetTable().GetName().GetName() != "users" {
		t.Fatalf("table=%v", tr.GetTable())
	}
	if tr.GetTable().GetName().GetSchema() != "app" {
		t.Fatalf("table.schema=%q", tr.GetTable().GetName().GetSchema())
	}
	if tr.GetTiming() != irv1.TriggerTiming_TRIGGER_TIMING_BEFORE {
		t.Fatalf("timing=%v", tr.GetTiming())
	}
	if len(tr.GetEvents()) != 2 {
		t.Fatalf("events=%v (want INSERT+UPDATE)", tr.GetEvents())
	}
	if tr.GetLevel() != irv1.TriggerLevel_TRIGGER_LEVEL_ROW {
		t.Fatal("level should be ROW")
	}
}

func TestMapCreateTrigger_After(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TRIGGER t AFTER DELETE ON users FOR EACH STATEMENT EXECUTE FUNCTION f();")
	tr := MapCreateTrigger(stmts[0].Node.GetCreateTrigStmt())
	if tr == nil {
		t.Fatal("nil")
	}
	if tr.GetTiming() != irv1.TriggerTiming_TRIGGER_TIMING_AFTER {
		t.Fatalf("timing=%v want AFTER", tr.GetTiming())
	}
	if len(tr.GetEvents()) != 1 || tr.GetEvents()[0] != irv1.TriggerEvent_TRIGGER_EVENT_DELETE {
		t.Fatalf("events=%v want DELETE", tr.GetEvents())
	}
	if tr.GetLevel() != irv1.TriggerLevel_TRIGGER_LEVEL_STATEMENT {
		t.Fatal("level should be STATEMENT")
	}
}

func TestMapCreateTrigger_InsteadOf(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TRIGGER t INSTEAD OF INSERT ON v FOR EACH ROW EXECUTE FUNCTION f();")
	tr := MapCreateTrigger(stmts[0].Node.GetCreateTrigStmt())
	if tr == nil {
		t.Fatal("nil")
	}
	if tr.GetTiming() != irv1.TriggerTiming_TRIGGER_TIMING_INSTEAD_OF {
		t.Fatalf("timing=%v want INSTEAD_OF", tr.GetTiming())
	}
}

func TestMapCreateTrigger_Function(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TRIGGER t AFTER INSERT ON t FOR EACH ROW EXECUTE FUNCTION app.my_func();")
	tr := MapCreateTrigger(stmts[0].Node.GetCreateTrigStmt())
	if tr == nil {
		t.Fatal("nil")
	}
	if tr.GetFunction() == nil {
		t.Fatal("function ref nil")
	}
	if tr.GetFunction().GetName().GetName() != "my_func" {
		t.Fatalf("func name=%v", tr.GetFunction().GetName())
	}
}

func TestMapCreateTrigger_Nil(t *testing.T) {
	if MapCreateTrigger(nil) != nil {
		t.Error("expected nil for nil CreateTrigStmt")
	}
}

// ---------------------------------------------------------------------------
// TestMapMaterializedView
// ---------------------------------------------------------------------------

func TestMapMaterializedView(t *testing.T) {
	stmts, _ := parse.Statements("CREATE MATERIALIZED VIEW app.mv AS SELECT 1 AS x;")
	mv := MapMaterializedView(stmts[0].Node.GetCreateTableAsStmt())
	if mv == nil {
		t.Fatal("MapMaterializedView returned nil")
	}
	if mv.GetName().GetName() != "mv" {
		t.Fatalf("name=%q", mv.GetName().GetName())
	}
	if mv.GetName().GetSchema() != "app" {
		t.Fatalf("schema=%q", mv.GetName().GetSchema())
	}
	if !mv.GetWithData() {
		t.Fatal("want WITH DATA default (skip_data=false → with_data=true)")
	}
	if mv.GetQuery() == nil {
		t.Fatal("query nil")
	}
}

func TestMapMaterializedView_WithNoData(t *testing.T) {
	stmts, _ := parse.Statements("CREATE MATERIALIZED VIEW mv AS SELECT 1 WITH NO DATA;")
	mv := MapMaterializedView(stmts[0].Node.GetCreateTableAsStmt())
	if mv == nil {
		t.Fatal("nil")
	}
	if mv.GetWithData() {
		t.Fatal("want WITH NO DATA → with_data=false")
	}
}

func TestMapMaterializedView_Nil(t *testing.T) {
	if MapMaterializedView(nil) != nil {
		t.Error("expected nil for nil CreateTableAsStmt")
	}
}

// ---------------------------------------------------------------------------
// TestMapCreateRange
// ---------------------------------------------------------------------------

func TestMapCreateRange(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TYPE app.timerange AS RANGE (subtype = timestamptz);")
	r := MapCreateRange(stmts[0].Node.GetCreateRangeStmt())
	if r == nil {
		t.Fatal("MapCreateRange returned nil")
	}
	if r.GetName().GetName() != "timerange" || r.GetName().GetSchema() != "app" {
		t.Fatalf("name=%v", r.GetName())
	}
	if r.GetSubtype().GetPgName() != "timestamptz" {
		t.Fatalf("subtype=%v", r.GetSubtype())
	}
}

func TestMapCreateRange_AllParams(t *testing.T) {
	sql := `CREATE TYPE app.timerange AS RANGE (
		subtype = timestamptz,
		subtype_opclass = timestamptz_ops,
		canonical = mycanon,
		subtype_diff = mydiff,
		multirange_type_name = app.timemultirange
	);`
	stmts, _ := parse.Statements(sql)
	r := MapCreateRange(stmts[0].Node.GetCreateRangeStmt())
	if r == nil {
		t.Fatal("MapCreateRange returned nil")
	}
	if r.GetSubtype().GetPgName() != "timestamptz" {
		t.Fatalf("subtype=%v", r.GetSubtype())
	}
	if r.GetSubtypeOpclass() != "timestamptz_ops" {
		t.Fatalf("subtype_opclass=%q", r.GetSubtypeOpclass())
	}
	if r.GetCanonical() != "mycanon" {
		t.Fatalf("canonical=%q", r.GetCanonical())
	}
	if r.GetSubtypeDiff() != "mydiff" {
		t.Fatalf("subtype_diff=%q", r.GetSubtypeDiff())
	}
	// The multirange name is stored BARE (schema qualifier dropped), consistent
	// with how the range's own Name is stored.
	if r.GetMultirange() != "timemultirange" {
		t.Fatalf("multirange=%q", r.GetMultirange())
	}
}

// TestMapCreateRange_DerivesMultirange verifies that when no explicit
// multirange_type_name is given, the multirange name is auto-derived from the
// range name following PostgreSQL's rule (last "range" → "multirange").
func TestMapCreateRange_DerivesMultirange(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TYPE app.timerange AS RANGE (subtype = timestamptz);")
	r := MapCreateRange(stmts[0].Node.GetCreateRangeStmt())
	if r == nil {
		t.Fatal("MapCreateRange returned nil")
	}
	if r.GetMultirange() != "timemultirange" {
		t.Fatalf("derived multirange=%q; want timemultirange", r.GetMultirange())
	}
}

// TestDeriveMultirangeName covers PostgreSQL's auto-generated multirange naming
// rule directly: replace the LAST "range" with "multirange", else append
// "_multirange".
func TestDeriveMultirangeName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"timerange", "timemultirange"},
		{"myrange", "mymultirange"},
		{"foo", "foo_multirange"},
		{"rangerange", "rangemultirange"}, // only the LAST "range" is replaced
		{"int4range", "int4multirange"},
		{"", ""},
	}
	for _, c := range cases {
		if got := deriveMultirangeName(c.in); got != c.want {
			t.Errorf("deriveMultirangeName(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}

func TestMapCreateRange_NoSchema(t *testing.T) {
	stmts, _ := parse.Statements("CREATE TYPE myrange AS RANGE (subtype = int4);")
	r := MapCreateRange(stmts[0].Node.GetCreateRangeStmt())
	if r == nil {
		t.Fatal("MapCreateRange returned nil")
	}
	if r.GetName().GetName() != "myrange" {
		t.Fatalf("name=%q", r.GetName().GetName())
	}
	if r.GetName().GetSchema() != "" {
		t.Fatalf("schema should be empty, got %q", r.GetName().GetSchema())
	}
	if r.GetSubtype().GetPgName() != "int4" {
		t.Fatalf("subtype=%v", r.GetSubtype())
	}
}

func TestMapCreateRange_Nil(t *testing.T) {
	if MapCreateRange(nil) != nil {
		t.Error("expected nil for nil CreateRangeStmt")
	}
}
