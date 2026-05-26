package catalog

import (
	"fmt"
	"testing"

	"github.com/yaroher/sqld/internal/parse"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

const shopSQL = `
CREATE TYPE order_status AS ENUM ('pending','paid');
CREATE TABLE users (
  id bigserial primary key,
  email text not null unique
);
CREATE TABLE orders (
  id bigserial primary key,
  user_id bigint not null references users(id) on delete cascade,
  status order_status not null
);
CREATE VIEW active_users AS SELECT id, email FROM users;
`

func buildShop(t *testing.T) (*irv1.Catalog, *Diagnostics) {
	t.Helper()
	stmts, err := parse.Statements(shopSQL)
	if err != nil {
		t.Fatal(err)
	}
	return Build(stmts)
}

func TestBuildSchemasAndTables(t *testing.T) {
	cat, diag := buildShop(t)
	for _, d := range diag.Items {
		t.Logf("diagnostic [%s]: %s", d.Severity, d.Message)
	}
	if cat.GetDefaultSchema() != "public" {
		t.Fatalf("default schema=%q", cat.GetDefaultSchema())
	}
	if len(cat.GetSchemas()) != 1 {
		t.Fatalf("schemas=%d", len(cat.GetSchemas()))
	}
	pub := cat.GetSchemas()[0]
	if len(pub.GetTables()) != 2 {
		t.Fatalf("tables=%d", len(pub.GetTables()))
	}
	if len(pub.GetEnums()) != 1 {
		t.Fatalf("enums=%d", len(pub.GetEnums()))
	}
	if len(pub.GetViews()) != 1 {
		t.Fatalf("views=%d", len(pub.GetViews()))
	}
}

func TestBuildIdsAndFK(t *testing.T) {
	cat, diag := buildShop(t)
	for _, d := range diag.Items {
		t.Logf("diagnostic [%s]: %s", d.Severity, d.Message)
	}
	pub := cat.GetSchemas()[0]
	var orders *irv1.Table
	for _, tb := range pub.GetTables() {
		if tb.GetName().GetName() == "orders" {
			orders = tb
		}
	}
	if orders == nil || orders.GetId() != "public.orders" {
		t.Fatalf("orders id=%q", orders.GetId())
	}
	var fk *irv1.ForeignKey
	for _, c := range orders.GetConstraints() {
		if c.GetForeignKey() != nil {
			fk = c.GetForeignKey()
		}
	}
	if fk == nil {
		t.Fatal("no FK")
	}
	if fk.GetReferencedTable().GetId() != "public.users" {
		t.Fatalf("fk ref id=%q", fk.GetReferencedTable().GetId())
	}
}

func TestBuildColumnIds(t *testing.T) {
	cat, _ := buildShop(t)
	pub := cat.GetSchemas()[0]
	var users *irv1.Table
	for _, tb := range pub.GetTables() {
		if tb.GetName().GetName() == "users" {
			users = tb
		}
	}
	if users == nil {
		t.Fatal("users table not found")
	}
	for _, col := range users.GetColumns() {
		if col.GetId() == "" {
			t.Fatalf("column %q has empty id", col.GetName())
		}
		// id should be "public.users.<colname>"
		expected := "public.users." + col.GetName()
		if col.GetId() != expected {
			t.Fatalf("column %q id=%q want %q", col.GetName(), col.GetId(), expected)
		}
	}
}

func TestBuildNoDiagnosticErrors(t *testing.T) {
	_, diag := buildShop(t)
	for _, d := range diag.Items {
		if d.Severity == "error" {
			t.Errorf("unexpected error diagnostic: %s", d.Message)
		}
	}
}

// ---------------------------------------------------------------------------
// TestBuildFullIR — new object kinds: schema, domain, composite, function,
// trigger, materialized view.
// ---------------------------------------------------------------------------

const fullSQL = `
CREATE SCHEMA app;
CREATE TYPE app.status AS ENUM ('active','inactive');
CREATE DOMAIN app.email AS text NOT NULL CHECK (VALUE ~ '@');
CREATE TYPE app.address AS (street text, zip int4);
CREATE TABLE app.users (
  id bigserial PRIMARY KEY,
  email app.email NOT NULL,
  status app.status NOT NULL
);
CREATE FUNCTION app.touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;
CREATE TRIGGER users_touch BEFORE UPDATE ON app.users FOR EACH ROW EXECUTE FUNCTION app.touch();
CREATE MATERIALIZED VIEW app.active_users AS SELECT id, email FROM app.users WHERE status = 'active';
`

func schemaNames(cat *irv1.Catalog) []string {
	var names []string
	for _, s := range cat.GetSchemas() {
		names = append(names, s.GetName())
	}
	return names
}

func TestBuildFullIR(t *testing.T) {
	stmts, err := parse.Statements(fullSQL)
	if err != nil {
		t.Fatal(err)
	}
	cat, diag := Build(stmts)
	for _, d := range diag.Items {
		t.Logf("diagnostic [%s]: %s", d.Severity, d.Message)
	}

	var app *irv1.Schema
	for _, s := range cat.GetSchemas() {
		if s.GetName() == "app" {
			app = s
		}
	}
	if app == nil {
		t.Fatalf("no app schema; got %v", schemaNames(cat))
	}
	if len(app.GetEnums()) != 1 {
		t.Fatalf("enums=%d", len(app.GetEnums()))
	}
	if len(app.GetDomains()) != 1 {
		t.Fatalf("domains=%d", len(app.GetDomains()))
	}
	if len(app.GetComposites()) != 1 {
		t.Fatalf("composites=%d", len(app.GetComposites()))
	}
	if len(app.GetTables()) != 1 {
		t.Fatalf("tables=%d", len(app.GetTables()))
	}
	if len(app.GetFunctions()) != 1 {
		t.Fatalf("functions=%d", len(app.GetFunctions()))
	}
	if len(app.GetTriggers()) != 1 {
		t.Fatalf("triggers=%d", len(app.GetTriggers()))
	}
	if len(app.GetMaterializedViews()) != 1 {
		t.Fatalf("matviews=%d", len(app.GetMaterializedViews()))
	}
	tr := app.GetTriggers()[0]
	if tr.GetTable().GetId() != "app.users" {
		t.Fatalf("trigger table id=%q", tr.GetTable().GetId())
	}
	if tr.GetFunction().GetId() != "app.touch" {
		t.Fatalf("trigger fn id=%q", tr.GetFunction().GetId())
	}

	// Domain and composite IDs must be assigned.
	dom := app.GetDomains()[0]
	if dom.GetId() == "" {
		t.Fatal("domain id is empty")
	}
	comp := app.GetComposites()[0]
	if comp.GetId() == "" {
		t.Fatal("composite id is empty")
	}
	fn := app.GetFunctions()[0]
	if fn.GetId() == "" {
		t.Fatal("function id is empty")
	}
	mv := app.GetMaterializedViews()[0]
	if mv.GetId() == "" {
		t.Fatal("matview id is empty")
	}

	// Table.Triggers back-ref must contain the trigger ObjectRef.
	tbl := app.GetTables()[0]
	if len(tbl.GetTriggers()) != 1 {
		t.Fatalf("table triggers=%d", len(tbl.GetTriggers()))
	}
	tref := tbl.GetTriggers()[0]
	if tref.GetKind() != irv1.ObjectKind_OBJECT_KIND_TRIGGER {
		t.Fatalf("table trigger ref kind=%v", tref.GetKind())
	}
	_ = fmt.Sprintf // suppress unused import error in case fmt is only used here
}

// ---------------------------------------------------------------------------
// TestBuildCreateRange
// ---------------------------------------------------------------------------

func TestBuildCreateRange(t *testing.T) {
	sql := `CREATE SCHEMA app; CREATE TYPE app.timerange AS RANGE (subtype = timestamptz);`
	stmts, err := parse.Statements(sql)
	if err != nil {
		t.Fatal(err)
	}
	cat, diag := Build(stmts)
	for _, d := range diag.Items {
		t.Logf("diagnostic [%s]: %s", d.Severity, d.Message)
	}

	var app *irv1.Schema
	for _, s := range cat.GetSchemas() {
		if s.GetName() == "app" {
			app = s
		}
	}
	if app == nil {
		t.Fatalf("no app schema; got %v", schemaNames(cat))
	}
	if len(app.GetRanges()) != 1 {
		t.Fatalf("ranges=%d, want 1", len(app.GetRanges()))
	}
	rt := app.GetRanges()[0]
	if rt.GetId() != "app.timerange" {
		t.Fatalf("range id=%q, want %q", rt.GetId(), "app.timerange")
	}
	if rt.GetName().GetName() != "timerange" {
		t.Fatalf("range name=%q", rt.GetName().GetName())
	}
	if rt.GetName().GetSchema() != "app" {
		t.Fatalf("range schema=%q", rt.GetName().GetSchema())
	}
	if rt.GetSubtype().GetPgName() != "timestamptz" {
		t.Fatalf("range subtype=%v", rt.GetSubtype())
	}
}
