package introspect

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/yaroher/sqld/internal/devdb"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// richDDL exercises every object kind the introspector handles.
const richDDL = `
CREATE SCHEMA app;
CREATE TYPE app.status AS ENUM ('active','inactive');
CREATE TYPE app.addr AS (street text, zip text);
CREATE DOMAIN app.email AS text NOT NULL CHECK (VALUE ~ '@');
CREATE TYPE app.timerange AS RANGE (subtype = timestamptz);

CREATE TABLE app.users (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email app.email NOT NULL,
  status app.status,
  tags text[],
  full_name text GENERATED ALWAYS AS (email) STORED,
  created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE app.users ADD CONSTRAINT users_email_uniq UNIQUE (email);

CREATE TABLE app.orders (
  id bigint PRIMARY KEY,
  user_id bigint NOT NULL REFERENCES app.users(id) ON DELETE CASCADE ON UPDATE RESTRICT,
  amount numeric(10,2) NOT NULL CHECK (amount >= 0)
);
CREATE INDEX idx_orders_user ON app.orders(user_id);
CREATE INDEX idx_orders_lower_amt ON app.orders((amount * 2)) WHERE amount > 100;

CREATE VIEW app.active AS SELECT id FROM app.users WHERE status = 'active';
CREATE MATERIALIZED VIEW app.order_counts AS SELECT user_id, count(*) AS n FROM app.orders GROUP BY user_id;

CREATE SEQUENCE app.counter START 5 INCREMENT 2;

CREATE FUNCTION app.touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;
CREATE FUNCTION app.add(a int, b int) RETURNS int LANGUAGE sql IMMUTABLE STRICT AS $$ SELECT a + b $$;
CREATE PROCEDURE app.noop() LANGUAGE plpgsql AS $$ BEGIN END $$;
CREATE TRIGGER users_touch BEFORE UPDATE ON app.users FOR EACH ROW EXECUTE FUNCTION app.touch();
`

// findSchema returns the schema with the given name or fails the test.
func findSchema(t *testing.T, cat *irv1.Catalog, name string) *irv1.Schema {
	t.Helper()
	for _, s := range cat.GetSchemas() {
		if s.GetName() == name {
			return s
		}
	}
	t.Fatalf("schema %q not found; have %v", name, schemaNames(cat))
	return nil
}

func schemaNames(cat *irv1.Catalog) []string {
	var out []string
	for _, s := range cat.GetSchemas() {
		out = append(out, s.GetName())
	}
	return out
}

func findTable(s *irv1.Schema, name string) *irv1.Table {
	for _, tb := range s.GetTables() {
		if tb.GetName().GetName() == name {
			return tb
		}
	}
	return nil
}

func findColumn(tb *irv1.Table, name string) *irv1.Column {
	for _, c := range tb.GetColumns() {
		if c.GetName() == name {
			return c
		}
	}
	return nil
}

func TestIntrospect(t *testing.T) {
	ctx := context.Background()
	d, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer d.Close()

	if err := d.Apply(ctx, richDDL); err != nil {
		t.Fatalf("apply ddl: %v", err)
	}
	conn, err := pgx.Connect(ctx, d.URL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	cat, err := Introspect(ctx, conn, []string{"app"})
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}

	app := findSchema(t, cat, "app")

	// ---- Tables ----
	if len(app.GetTables()) != 2 {
		t.Fatalf("tables=%d, want 2 (%v)", len(app.GetTables()), tableNames(app))
	}
	users := findTable(app, "users")
	orders := findTable(app, "orders")
	if users == nil || orders == nil {
		t.Fatalf("missing users/orders; have %v", tableNames(app))
	}
	if users.GetId() != "app.users" {
		t.Fatalf("users id=%q", users.GetId())
	}

	// ---- Columns / ids / ordinals ----
	idCol := findColumn(users, "id")
	if idCol == nil || idCol.GetId() != "app.users.id" || idCol.GetPosition() != 1 {
		t.Fatalf("users.id col=%+v", idCol)
	}
	if idCol.GetNullable() {
		t.Fatalf("users.id should be NOT NULL (pk)")
	}
	if idCol.GetIdentity() == nil || idCol.GetIdentity().GetKind() != irv1.IdentityKind_IDENTITY_KIND_ALWAYS {
		t.Fatalf("users.id identity=%v", idCol.GetIdentity())
	}
	// generated column
	fullName := findColumn(users, "full_name")
	if fullName == nil || fullName.GetGenerated() == nil || !fullName.GetGenerated().GetStored() {
		t.Fatalf("users.full_name generated=%v", fullName.GetGenerated())
	}
	// default expr
	createdAt := findColumn(users, "created_at")
	if createdAt == nil || createdAt.GetDefaultExpr() == nil ||
		!strings.Contains(createdAt.GetDefaultExpr().GetRawSql(), "now()") {
		t.Fatalf("users.created_at default=%v", createdAt.GetDefaultExpr())
	}
	// array type
	tags := findColumn(users, "tags")
	if tags == nil || tags.GetType().GetKind() != irv1.TypeKind_TYPE_KIND_ARRAY {
		t.Fatalf("users.tags type=%v", tags.GetType())
	}
	// enum-typed column resolves to the udt
	status := findColumn(users, "status")
	if status == nil || status.GetType().GetKind() != irv1.TypeKind_TYPE_KIND_ENUM {
		t.Fatalf("users.status type=%v", status.GetType())
	}
	if status.GetType().GetUdt().GetId() != "app.status" {
		t.Fatalf("users.status udt=%v", status.GetType().GetUdt())
	}
	// domain-typed column resolves to the udt
	emailCol := findColumn(users, "email")
	if emailCol == nil || emailCol.GetType().GetKind() != irv1.TypeKind_TYPE_KIND_DOMAIN {
		t.Fatalf("users.email type=%v", emailCol.GetType())
	}

	// ---- Constraints ----
	var pk, uniq *irv1.Constraint
	for _, c := range users.GetConstraints() {
		switch c.GetType() {
		case irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY:
			pk = c
		case irv1.ConstraintType_CONSTRAINT_TYPE_UNIQUE:
			uniq = c
		}
	}
	if pk == nil || len(pk.GetPrimaryKey().GetColumns()) != 1 || pk.GetPrimaryKey().GetColumns()[0] != "id" {
		t.Fatalf("users pk=%v", pk)
	}
	if uniq == nil {
		t.Fatalf("users unique constraint missing")
	}

	// FK on orders resolves the referenced table id.
	var fk *irv1.ForeignKey
	var check *irv1.Constraint
	for _, c := range orders.GetConstraints() {
		if c.GetForeignKey() != nil {
			fk = c.GetForeignKey()
		}
		if c.GetType() == irv1.ConstraintType_CONSTRAINT_TYPE_CHECK {
			check = c
		}
	}
	if fk == nil {
		t.Fatal("orders FK missing")
	}
	if fk.GetReferencedTable().GetId() != "app.users" {
		t.Fatalf("fk ref id=%q", fk.GetReferencedTable().GetId())
	}
	if len(fk.GetColumns()) != 1 || fk.GetColumns()[0] != "user_id" {
		t.Fatalf("fk cols=%v", fk.GetColumns())
	}
	if len(fk.GetReferencedColumns()) != 1 || fk.GetReferencedColumns()[0] != "id" {
		t.Fatalf("fk refcols=%v", fk.GetReferencedColumns())
	}
	if fk.GetOnDelete() != irv1.ReferentialAction_REFERENTIAL_ACTION_CASCADE {
		t.Fatalf("fk on delete=%v", fk.GetOnDelete())
	}
	if fk.GetOnUpdate() != irv1.ReferentialAction_REFERENTIAL_ACTION_RESTRICT {
		t.Fatalf("fk on update=%v", fk.GetOnUpdate())
	}
	if check == nil || check.GetCheck().GetExpression() == nil {
		t.Fatalf("orders check constraint missing")
	}

	// ---- Indexes (on orders: idx_orders_user, idx_orders_lower_amt, + pk) ----
	var haveUserIdx, havePartialIdx, havePrimaryIdx bool
	for _, idx := range orders.GetIndexes() {
		switch idx.GetName() {
		case "idx_orders_user":
			haveUserIdx = true
			if len(idx.GetElements()) != 1 || idx.GetElements()[0].GetColumn() != "user_id" {
				t.Fatalf("idx_orders_user elements=%v", idx.GetElements())
			}
		case "idx_orders_lower_amt":
			havePartialIdx = true
			if idx.GetPredicate() == nil {
				t.Fatalf("idx_orders_lower_amt has no predicate")
			}
			if len(idx.GetElements()) != 1 || idx.GetElements()[0].GetExpr() == nil {
				t.Fatalf("idx_orders_lower_amt expr element missing: %v", idx.GetElements())
			}
		}
		if idx.GetPrimary() {
			havePrimaryIdx = true
		}
	}
	if !haveUserIdx {
		t.Fatalf("idx_orders_user not found; have %v", indexNames(orders))
	}
	if !havePartialIdx {
		t.Fatalf("idx_orders_lower_amt not found; have %v", indexNames(orders))
	}
	if !havePrimaryIdx {
		t.Fatalf("primary-key-backing index not flagged; have %v", indexNames(orders))
	}

	// ---- Enum ----
	if len(app.GetEnums()) != 1 {
		t.Fatalf("enums=%d", len(app.GetEnums()))
	}
	en := app.GetEnums()[0]
	if en.GetName().GetName() != "status" || en.GetId() != "app.status" {
		t.Fatalf("enum=%v", en)
	}
	if len(en.GetLabels()) != 2 || en.GetLabels()[0] != "active" || en.GetLabels()[1] != "inactive" {
		t.Fatalf("enum labels=%v", en.GetLabels())
	}

	// ---- Composite ----
	if len(app.GetComposites()) != 1 {
		t.Fatalf("composites=%d", len(app.GetComposites()))
	}
	comp := app.GetComposites()[0]
	if comp.GetName().GetName() != "addr" || len(comp.GetFields()) != 2 {
		t.Fatalf("composite=%v", comp)
	}
	if comp.GetFields()[0].GetName() != "street" {
		t.Fatalf("composite field 0=%v", comp.GetFields()[0])
	}

	// ---- Domain ----
	if len(app.GetDomains()) != 1 {
		t.Fatalf("domains=%d", len(app.GetDomains()))
	}
	dom := app.GetDomains()[0]
	if dom.GetName().GetName() != "email" || dom.GetNullable() {
		t.Fatalf("domain=%v", dom)
	}
	if dom.GetBaseType().GetPgName() != "text" {
		t.Fatalf("domain base=%v", dom.GetBaseType())
	}
	if len(dom.GetConstraints()) != 1 {
		t.Fatalf("domain constraints=%d", len(dom.GetConstraints()))
	}

	// ---- Range ----
	if len(app.GetRanges()) != 1 {
		t.Fatalf("ranges=%d", len(app.GetRanges()))
	}
	rng := app.GetRanges()[0]
	if rng.GetName().GetName() != "timerange" {
		t.Fatalf("range=%v", rng)
	}
	if rng.GetSubtype().GetPgName() != "timestamptz" {
		t.Fatalf("range subtype=%v", rng.GetSubtype())
	}
	if rng.GetMultirange() == "" {
		t.Fatalf("range multirange empty")
	}

	// ---- Views / matviews ----
	if len(app.GetViews()) != 1 {
		t.Fatalf("views=%d", len(app.GetViews()))
	}
	if app.GetViews()[0].GetName().GetName() != "active" || app.GetViews()[0].GetId() != "app.active" {
		t.Fatalf("view=%v", app.GetViews()[0])
	}
	if app.GetViews()[0].GetQuery().GetRawSql() == "" {
		t.Fatalf("view query empty")
	}
	if len(app.GetMaterializedViews()) != 1 {
		t.Fatalf("matviews=%d", len(app.GetMaterializedViews()))
	}
	if app.GetMaterializedViews()[0].GetName().GetName() != "order_counts" {
		t.Fatalf("matview=%v", app.GetMaterializedViews()[0])
	}

	// ---- Sequence (the explicit one; identity-backed sequences are owned). ----
	var counter *irv1.Sequence
	for _, s := range app.GetSequences() {
		if s.GetName().GetName() == "counter" {
			counter = s
		}
	}
	if counter == nil {
		t.Fatalf("sequence counter not found; have %v", seqNames(app))
	}
	if counter.GetStart() != 5 || counter.GetIncrement() != 2 {
		t.Fatalf("counter start=%d incr=%d", counter.GetStart(), counter.GetIncrement())
	}

	// ---- Functions / procedures ----
	var touch, add *irv1.Function
	for _, fn := range app.GetFunctions() {
		switch fn.GetName().GetName() {
		case "touch":
			touch = fn
		case "add":
			add = fn
		}
	}
	if touch == nil || touch.GetLanguage() != "plpgsql" {
		t.Fatalf("function touch=%v", touch)
	}
	if touch.GetReturns().GetTriggerValue() != true {
		t.Fatalf("touch should return trigger; got %v", touch.GetReturns())
	}
	if add == nil {
		t.Fatalf("function add missing")
	}
	if len(add.GetArguments()) != 2 {
		t.Fatalf("add args=%d", len(add.GetArguments()))
	}
	if add.GetArguments()[0].GetName() != "a" {
		t.Fatalf("add arg0=%v", add.GetArguments()[0])
	}
	if add.GetVolatility() != irv1.Volatility_VOLATILITY_IMMUTABLE {
		t.Fatalf("add volatility=%v", add.GetVolatility())
	}
	if add.GetNullInput() != irv1.NullInputBehavior_NULL_INPUT_BEHAVIOR_STRICT {
		t.Fatalf("add null input=%v", add.GetNullInput())
	}
	if add.GetReturns().GetScalar().GetPgName() != "int4" {
		t.Fatalf("add returns=%v", add.GetReturns())
	}
	if len(app.GetProcedures()) != 1 || app.GetProcedures()[0].GetName().GetName() != "noop" {
		t.Fatalf("procedures=%v", app.GetProcedures())
	}

	// ---- Triggers ----
	if len(app.GetTriggers()) != 1 {
		t.Fatalf("triggers=%d", len(app.GetTriggers()))
	}
	trig := app.GetTriggers()[0]
	if trig.GetName() != "users_touch" {
		t.Fatalf("trigger name=%q", trig.GetName())
	}
	if trig.GetTable().GetId() != "app.users" {
		t.Fatalf("trigger table id=%q", trig.GetTable().GetId())
	}
	if trig.GetFunction().GetId() != "app.touch" {
		t.Fatalf("trigger fn id=%q", trig.GetFunction().GetId())
	}
	if trig.GetTiming() != irv1.TriggerTiming_TRIGGER_TIMING_BEFORE {
		t.Fatalf("trigger timing=%v", trig.GetTiming())
	}
	if trig.GetLevel() != irv1.TriggerLevel_TRIGGER_LEVEL_ROW {
		t.Fatalf("trigger level=%v", trig.GetLevel())
	}
	if len(trig.GetEvents()) != 1 || trig.GetEvents()[0] != irv1.TriggerEvent_TRIGGER_EVENT_UPDATE {
		t.Fatalf("trigger events=%v", trig.GetEvents())
	}
	// Table back-ref.
	if len(users.GetTriggers()) != 1 || users.GetTriggers()[0].GetKind() != irv1.ObjectKind_OBJECT_KIND_TRIGGER {
		t.Fatalf("users trigger back-ref=%v", users.GetTriggers())
	}
}

// TestIntrospectDefaultSchemas verifies that an empty schema list discovers
// non-system schemas (including public).
func TestIntrospectDefaultSchemas(t *testing.T) {
	ctx := context.Background()
	d, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer d.Close()

	if err := d.Apply(ctx, `CREATE TABLE public.widget (id int primary key);`); err != nil {
		t.Fatalf("apply: %v", err)
	}
	conn, err := pgx.Connect(ctx, d.URL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)

	cat, err := Introspect(ctx, conn, nil)
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	if cat.GetDefaultSchema() != "public" {
		t.Fatalf("default schema=%q", cat.GetDefaultSchema())
	}
	pub := findSchema(t, cat, "public")
	if findTable(pub, "widget") == nil {
		t.Fatalf("public.widget not introspected; tables=%v", tableNames(pub))
	}
	if cat.GetDatabaseVersion() == "" {
		t.Fatalf("database version not populated")
	}
}

func tableNames(s *irv1.Schema) []string {
	var out []string
	for _, t := range s.GetTables() {
		out = append(out, t.GetName().GetName())
	}
	return out
}

func indexNames(t *irv1.Table) []string {
	var out []string
	for _, idx := range t.GetIndexes() {
		out = append(out, idx.GetName())
	}
	return out
}

func seqNames(s *irv1.Schema) []string {
	var out []string
	for _, sq := range s.GetSequences() {
		out = append(out, sq.GetName().GetName())
	}
	return out
}
