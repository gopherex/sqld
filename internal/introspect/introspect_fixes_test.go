package introspect

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/diff"
	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/pkg/devdb"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	"github.com/jackc/pgx/v5"
)

// introspectDDL applies ddl to a fresh ephemeral PG and returns the
// introspected catalog for the given schemas (defaults to public).
func introspectDDL(t *testing.T, ddl string, schemas []string) *irv1.Catalog {
	t.Helper()
	ctx := context.Background()
	d, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	t.Cleanup(func() { _ = d.Close() })

	if err := d.Apply(ctx, ddl); err != nil {
		t.Fatalf("apply ddl: %v", err)
	}
	conn, err := pgx.Connect(ctx, d.URL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(func() { conn.Close(ctx) })

	cat, err := Introspect(ctx, conn, schemas)
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	return cat
}

// parseDDL builds a desired (parsed) catalog from declarative DDL.
func parseDDL(t *testing.T, ddl string) *irv1.Catalog {
	t.Helper()
	stmts, err := parse.Statements(ddl)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	c, _ := catalog.Build(stmts)
	return c
}

func findSchemaCat(cat *irv1.Catalog, name string) *irv1.Schema {
	for _, s := range cat.GetSchemas() {
		if s.GetName() == name {
			return s
		}
	}
	return nil
}

func tableIn(s *irv1.Schema, name string) *irv1.Table {
	if s == nil {
		return nil
	}
	for _, tb := range s.GetTables() {
		if tb.GetName().GetName() == name {
			return tb
		}
	}
	return nil
}

func colIn(tb *irv1.Table, name string) *irv1.Column {
	if tb == nil {
		return nil
	}
	for _, c := range tb.GetColumns() {
		if c.GetName() == name {
			return c
		}
	}
	return nil
}

// renderColType is a thin DB-free helper that renders a column type the way the
// diff engine would (PgName + modifier + array brackets), without importing the
// diff package's unexported renderer. It mirrors what the generated DDL emits.
func renderColType(c *irv1.Column) string {
	tr := c.GetType()
	base := tr.GetPgName()
	mod := tr.GetModifier()
	if tr.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY && tr.GetElement() != nil {
		base = tr.GetElement().GetPgName()
		mod = tr.GetElement().GetModifier()
	}
	suffix := ""
	switch {
	case mod.GetNumeric() != nil:
		n := mod.GetNumeric()
		if n.GetScale() > 0 {
			suffix = fmt.Sprintf("(%d,%d)", n.GetPrecision(), n.GetScale())
		} else if n.GetPrecision() > 0 {
			suffix = fmt.Sprintf("(%d)", n.GetPrecision())
		}
	case mod.GetText() != nil:
		if l := mod.GetText().GetLength(); l > 0 {
			suffix = fmt.Sprintf("(%d)", l)
		}
	case mod.GetDateTime() != nil:
		if p := mod.GetDateTime().GetPrecision(); p > 0 {
			suffix = fmt.Sprintf("(%d)", p)
		}
	}
	return base + suffix
}

// TestIntrospectTypeModifiers (C3) verifies that numeric(10,2) and varchar(50)
// columns introspect with their precision/length preserved, so generated DDL
// reproduces the modifier. Before the fix, PgName was the bare typname and the
// modifier was dropped (numeric, varchar).
func TestIntrospectTypeModifiers(t *testing.T) {
	cat := introspectDDL(t, `
CREATE TABLE m (
  a numeric(10,2),
  b varchar(50),
  c timestamptz(3),
  d numeric(8)
);`, []string{"public"})

	tbl := tableIn(findSchemaCat(cat, "public"), "m")
	if tbl == nil {
		t.Fatalf("table m not introspected")
	}

	if got := renderColType(colIn(tbl, "a")); got != "numeric(10,2)" {
		t.Fatalf("column a type = %q, want numeric(10,2)", got)
	}
	if got := renderColType(colIn(tbl, "b")); got != "varchar(50)" {
		t.Fatalf("column b type = %q, want varchar(50)", got)
	}
	if got := renderColType(colIn(tbl, "c")); got != "timestamptz(3)" {
		t.Fatalf("column c type = %q, want timestamptz(3)", got)
	}
	if got := renderColType(colIn(tbl, "d")); got != "numeric(8)" {
		t.Fatalf("column d type = %q, want numeric(8)", got)
	}
}

// TestIntrospectModifiersNoSpuriousDiff (C3) builds the SAME schema from parsed
// DDL (desired) and from introspection (current), then diffs them; the plan
// must be EMPTY. A dropped modifier would surface as a spurious
// ALTER COLUMN ... TYPE change.
func TestIntrospectModifiersNoSpuriousDiff(t *testing.T) {
	const ddl = `CREATE TABLE m (a numeric(10,2) NOT NULL, b varchar(50), c timestamptz(3));`
	current := introspectDDL(t, ddl, []string{"public"})
	desired := parseDDL(t, ddl)

	p, err := diff.Diff(current, desired)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Empty() {
		t.Fatalf("expected empty diff for identical schema, got up:\n%s", p.UpSQL())
	}
}

// TestIntrospectConstraintBackedIndexExcluded (C1) verifies a UNIQUE constraint
// does not also surface a standalone unique-backing index, and that a
// generate-style diff of two such introspected states applies cleanly on a real
// PG (no "relation already exists").
func TestIntrospectConstraintBackedIndexExcluded(t *testing.T) {
	const ddl = `CREATE TABLE u (id bigint PRIMARY KEY, email text, CONSTRAINT u_email UNIQUE (email));`
	cat := introspectDDL(t, ddl, []string{"public"})
	tbl := tableIn(findSchemaCat(cat, "public"), "u")
	if tbl == nil {
		t.Fatalf("table u not introspected")
	}
	// No standalone index should be recorded (PK + UNIQUE are constraint-owned).
	for _, idx := range tbl.GetIndexes() {
		t.Fatalf("constraint-backed index %q should be excluded; got indexes %v",
			idx.GetName(), indexNames(tbl))
	}
	// Exactly the two constraints (PK + UNIQUE) should be present.
	var nPK, nUniq int
	for _, c := range tbl.GetConstraints() {
		switch c.GetType() {
		case irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY:
			nPK++
		case irv1.ConstraintType_CONSTRAINT_TYPE_UNIQUE:
			nUniq++
		}
	}
	if nPK != 1 || nUniq != 1 {
		t.Fatalf("want 1 PK + 1 UNIQUE constraint, got pk=%d uniq=%d", nPK, nUniq)
	}

	// generate-style: diff from empty -> introspected, render and apply on a
	// fresh PG. Before the fix this emitted both ADD CONSTRAINT ... UNIQUE and
	// CREATE UNIQUE INDEX for the same relation and failed to apply.
	p, err := diff.Diff(&irv1.Catalog{}, cat)
	if err != nil {
		t.Fatal(err)
	}
	up := p.UpSQL()
	if strings.Contains(up, "CREATE UNIQUE INDEX") {
		t.Fatalf("generated DDL must not CREATE a constraint-backed index:\n%s", up)
	}
	applyOnFreshPG(t, up)
}

func TestIntrospectNullsNotDistinct(t *testing.T) {
	const ddl = `
CREATE TABLE event_rsvps (
  event_id bigint,
  user_id bigint,
  occurrence_at timestamptz,
  CONSTRAINT event_rsvps_pk
    UNIQUE NULLS NOT DISTINCT (event_id, user_id, occurrence_at)
);
CREATE UNIQUE INDEX event_rsvps_lookup_uidx
  ON event_rsvps (user_id, occurrence_at) NULLS NOT DISTINCT;`

	cat := introspectDDL(t, ddl, []string{"public"})
	tbl := tableIn(findSchemaCat(cat, "public"), "event_rsvps")
	if tbl == nil {
		t.Fatal("table event_rsvps not introspected")
	}

	var uniqueConstraint *irv1.UniqueConstraint
	for _, c := range tbl.GetConstraints() {
		if c.GetName() == "event_rsvps_pk" {
			uniqueConstraint = c.GetUnique()
			break
		}
	}
	if uniqueConstraint == nil {
		t.Fatal("unique constraint event_rsvps_pk not introspected")
	}
	if !uniqueConstraint.GetNullsNotDistinct() {
		t.Fatal("unique constraint lost NULLS NOT DISTINCT")
	}

	var uniqueIndex *irv1.Index
	for _, idx := range tbl.GetIndexes() {
		if idx.GetName() == "event_rsvps_lookup_uidx" {
			uniqueIndex = idx
			break
		}
	}
	if uniqueIndex == nil {
		t.Fatal("unique index event_rsvps_lookup_uidx not introspected")
	}
	if !uniqueIndex.GetNullsNotDistinct() {
		t.Fatal("unique index lost NULLS NOT DISTINCT")
	}

	plan, err := diff.Diff(&irv1.Catalog{}, cat)
	if err != nil {
		t.Fatal(err)
	}
	up := plan.UpSQL()
	if !strings.Contains(up, `UNIQUE NULLS NOT DISTINCT ("event_id", "user_id", "occurrence_at")`) {
		t.Fatalf("generated constraint lost NULLS NOT DISTINCT:\n%s", up)
	}
	if !strings.Contains(up, `CREATE UNIQUE INDEX "event_rsvps_lookup_uidx" ON "public"."event_rsvps" ("user_id", "occurrence_at") NULLS NOT DISTINCT;`) {
		t.Fatalf("generated index lost NULLS NOT DISTINCT:\n%s", up)
	}
	applyOnFreshPG(t, up)
}

// TestIntrospectExclusionConstraint (C4) verifies an EXCLUSION constraint
// introspects to a non-empty, valid EXCLUDE body (not "EXCLUDE ()") whose
// generated DDL applies on a real PG.
func TestIntrospectExclusionConstraint(t *testing.T) {
	const ddl = `
CREATE EXTENSION IF NOT EXISTS btree_gist;
CREATE TABLE booking (
  room int,
  during tsrange,
  EXCLUDE USING gist (room WITH =, during WITH &&)
);`
	cat := introspectDDL(t, ddl, []string{"public"})
	tbl := tableIn(findSchemaCat(cat, "public"), "booking")
	if tbl == nil {
		t.Fatalf("table booking not introspected")
	}
	var ex *irv1.Constraint
	for _, c := range tbl.GetConstraints() {
		if c.GetType() == irv1.ConstraintType_CONSTRAINT_TYPE_EXCLUSION {
			ex = c
		}
	}
	if ex == nil {
		t.Fatalf("exclusion constraint not introspected; constraints=%v", tbl.GetConstraints())
	}
	if len(ex.GetExclusion().GetElements()) == 0 {
		t.Fatalf("exclusion constraint has no elements (would render EXCLUDE ()): %v", ex.GetExclusion())
	}
	if ex.GetExclusion().GetIndexMethod() != "gist" {
		t.Fatalf("exclusion method = %q, want gist", ex.GetExclusion().GetIndexMethod())
	}

	// Generated DDL must be valid: apply against a fresh PG that has btree_gist.
	p, err := diff.Diff(&irv1.Catalog{}, cat)
	if err != nil {
		t.Fatal(err)
	}
	up := p.UpSQL()
	if strings.Contains(up, "EXCLUDE ()") {
		t.Fatalf("invalid empty EXCLUDE rendered:\n%s", up)
	}
	if !strings.Contains(up, "EXCLUDE USING gist") {
		t.Fatalf("expected EXCLUDE USING gist in generated DDL:\n%s", up)
	}
	applyOnFreshPG(t, "CREATE EXTENSION IF NOT EXISTS btree_gist;\n"+up)
}

// TestIntrospectExcludesExtensionObjects (I2) verifies that objects owned by an
// installed extension (btree_gist pulls in many C functions and support types)
// are NOT introspected into the catalog.
func TestIntrospectExcludesExtensionObjects(t *testing.T) {
	cat := introspectDDL(t, `CREATE EXTENSION IF NOT EXISTS btree_gist;`, []string{"public"})
	pub := findSchemaCat(cat, "public")
	if pub == nil {
		t.Fatalf("public schema missing")
	}
	if n := len(pub.GetFunctions()); n != 0 {
		var names []string
		for _, f := range pub.GetFunctions() {
			names = append(names, f.GetName().GetName())
		}
		t.Fatalf("extension functions leaked into catalog: %d (%v)", n, names)
	}
	if n := len(pub.GetProcedures()); n != 0 {
		t.Fatalf("extension procedures leaked into catalog: %d", n)
	}
}

// applyOnFreshPG applies sql to a brand-new ephemeral PG and fails the test if
// the apply errors. This catches generated DDL that does not actually run.
func applyOnFreshPG(t *testing.T, sql string) {
	t.Helper()
	ctx := context.Background()
	d, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer d.Close()
	if err := d.Apply(ctx, sql); err != nil {
		t.Fatalf("generated DDL failed to apply on a fresh PG: %v\n--- SQL ---\n%s", err, sql)
	}
}
