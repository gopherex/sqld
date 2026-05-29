package diff_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/yaroher/sqld/internal/diff"
	"github.com/yaroher/sqld/internal/introspect"
	"github.com/yaroher/sqld/pkg/devdb"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// roundTrip proves the core invariant of migration generation: for a transition
// fromSQL -> toSQL, the generated up migration actually reaches the desired
// catalog and the generated down migration reverts back to the original.
//
// Concretely:
//
//	dev1 := fresh PG; apply fromSQL; from := Introspect(dev1)
//	dev2 := fresh PG; apply toSQL;   desired := Introspect(dev2)
//	plan := Diff(from, desired)
//	apply plan.UpSQL() to dev1; got := Introspect(dev1)
//	ASSERT Diff(got, desired).Empty()   -- up reaches desired
//	apply plan.DownSQL() to dev1; back := Introspect(dev1)
//	ASSERT Diff(back, from).Empty()     -- down reverts to from
//
// Two fresh dev databases are used so the desired-state introspection is never
// polluted by the work-in-progress first database.
func roundTrip(t *testing.T, schemas []string, fromSQL, toSQL string) {
	t.Helper()
	roundTripPreamble(t, schemas, "", fromSQL, toSQL)
}

// roundTripPreamble is roundTrip with a preamble (e.g. CREATE EXTENSION)
// applied to both fresh dev databases before fromSQL/toSQL. The preamble is not
// part of the diff: it sets up objects (extensions and their types) that the
// schemas reference but that the generator never emits.
func roundTripPreamble(t *testing.T, schemas []string, preamble, fromSQL, toSQL string) {
	t.Helper()
	ctx := context.Background()

	dev1, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker: %v", err)
	}
	defer dev1.Close()

	dev2, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker: %v", err)
	}
	defer dev2.Close()

	if preamble != "" {
		if err := dev1.Apply(ctx, preamble); err != nil {
			t.Fatalf("apply preamble to dev1: %v\n%s", err, preamble)
		}
		if err := dev2.Apply(ctx, preamble); err != nil {
			t.Fatalf("apply preamble to dev2: %v\n%s", err, preamble)
		}
	}

	// from := introspect(dev1 with fromSQL applied)
	if fromSQL != "" {
		if err := dev1.Apply(ctx, fromSQL); err != nil {
			t.Fatalf("apply fromSQL: %v\n%s", err, fromSQL)
		}
	}
	from := introspectDB(t, ctx, dev1, schemas)

	// desired := introspect(dev2 with toSQL applied)
	if toSQL != "" {
		if err := dev2.Apply(ctx, toSQL); err != nil {
			t.Fatalf("apply toSQL: %v\n%s", err, toSQL)
		}
	}
	desired := introspectDB(t, ctx, dev2, schemas)

	plan, err := diff.Diff(from, desired)
	if err != nil {
		t.Fatalf("diff(from, desired): %v", err)
	}

	// --- UP: apply the generated forward migration to dev1, then prove it
	// reaches the desired catalog.
	up := plan.UpSQL()
	if up != "" {
		if err := dev1.Apply(ctx, up); err != nil {
			t.Fatalf("apply UpSQL: %v\n--- UpSQL ---\n%s", err, up)
		}
	}
	got := introspectDB(t, ctx, dev1, schemas)
	residual, err := diff.Diff(got, desired)
	if err != nil {
		t.Fatalf("diff(got, desired): %v", err)
	}
	if !residual.Empty() {
		t.Fatalf("UP did not reach desired catalog.\n--- UpSQL ---\n%s\n--- residual diff (got -> desired) UpSQL ---\n%s",
			up, residual.UpSQL())
	}

	// --- DOWN: apply the generated inverse migration to dev1, then prove it
	// reverts back to the original catalog.
	down := plan.DownSQL()
	if down != "" {
		if err := dev1.Apply(ctx, down); err != nil {
			t.Fatalf("apply DownSQL: %v\n--- DownSQL ---\n%s", err, down)
		}
	}
	back := introspectDB(t, ctx, dev1, schemas)
	residualDown, err := diff.Diff(back, from)
	if err != nil {
		t.Fatalf("diff(back, from): %v", err)
	}
	if !residualDown.Empty() {
		t.Fatalf("DOWN did not revert to original catalog.\n--- DownSQL ---\n%s\n--- residual diff (back -> from) UpSQL ---\n%s",
			down, residualDown.UpSQL())
	}
}

// introspectDB opens a fresh connection to dev and introspects it.
func introspectDB(t *testing.T, ctx context.Context, dev *devdb.DevDB, schemas []string) *irv1.Catalog {
	t.Helper()
	conn, err := pgx.Connect(ctx, dev.URL())
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer conn.Close(ctx)
	cat, err := introspect.Introspect(ctx, conn, schemas)
	if err != nil {
		t.Fatalf("introspect: %v", err)
	}
	return cat
}

// roundTripUpOnly proves only the UP half of the invariant (up reaches desired)
// without asserting the down fully reverts. It is used for transitions whose
// down is inherently lossy (e.g. DROP COLUMN); the down SQL is still applied so
// any structural revert errors are caught, and the structural revert is checked
// by the caller-provided predicate.
func roundTripUpOnly(t *testing.T, schemas []string, fromSQL, toSQL string, checkDown func(t *testing.T, back, from *irv1.Catalog)) {
	t.Helper()
	ctx := context.Background()

	dev1, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker: %v", err)
	}
	defer dev1.Close()

	dev2, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker: %v", err)
	}
	defer dev2.Close()

	if fromSQL != "" {
		if err := dev1.Apply(ctx, fromSQL); err != nil {
			t.Fatalf("apply fromSQL: %v\n%s", err, fromSQL)
		}
	}
	from := introspectDB(t, ctx, dev1, schemas)

	if toSQL != "" {
		if err := dev2.Apply(ctx, toSQL); err != nil {
			t.Fatalf("apply toSQL: %v\n%s", err, toSQL)
		}
	}
	desired := introspectDB(t, ctx, dev2, schemas)

	plan, err := diff.Diff(from, desired)
	if err != nil {
		t.Fatalf("diff(from, desired): %v", err)
	}

	up := plan.UpSQL()
	if up != "" {
		if err := dev1.Apply(ctx, up); err != nil {
			t.Fatalf("apply UpSQL: %v\n--- UpSQL ---\n%s", err, up)
		}
	}
	got := introspectDB(t, ctx, dev1, schemas)
	residual, err := diff.Diff(got, desired)
	if err != nil {
		t.Fatalf("diff(got, desired): %v", err)
	}
	if !residual.Empty() {
		t.Fatalf("UP did not reach desired catalog.\n--- UpSQL ---\n%s\n--- residual diff (got -> desired) UpSQL ---\n%s",
			up, residual.UpSQL())
	}

	down := plan.DownSQL()
	if down != "" {
		// Strip warning comments out so the (possibly lossy) re-create body
		// actually executes. The harness wraps lossy re-creates with a
		// "-- WARNING:" line which is a no-op comment, fine to apply as-is.
		if err := dev1.Apply(ctx, down); err != nil {
			t.Fatalf("apply DownSQL: %v\n--- DownSQL ---\n%s", err, down)
		}
	}
	back := introspectDB(t, ctx, dev1, schemas)
	if checkDown != nil {
		checkDown(t, back, from)
	}
}

func TestRoundTripFullCreate(t *testing.T) {
	data, err := os.ReadFile("../../example/schema.sql")
	if err != nil {
		t.Fatalf("read example/schema.sql: %v", err)
	}
	// The example schema lives in schemas app + audit (and public is implicit).
	// It references the hstore + ltree extension types, which must be installed
	// in every dev database first. Extensions are objects we do not diff (their
	// types are extension-owned and filtered out by the introspector), so they
	// belong in a preamble applied to both the from and desired databases.
	const ext = "CREATE EXTENSION IF NOT EXISTS hstore; CREATE EXTENSION IF NOT EXISTS ltree;"
	roundTripPreamble(t, []string{"public", "app", "audit"}, ext, "", string(data))
}

func TestRoundTripAddColumn(t *testing.T) {
	roundTrip(t, []string{"public"},
		`CREATE TABLE t(id bigint primary key);`,
		`CREATE TABLE t(id bigint primary key, email text not null);`,
	)
}

func TestRoundTripDropColumn(t *testing.T) {
	// Down is lossy: the dropped column's data cannot be restored. We assert UP
	// reaches desired, and that DOWN structurally re-adds the column.
	roundTripUpOnly(t, []string{"public"},
		`CREATE TABLE t(id bigint primary key, email text not null);`,
		`CREATE TABLE t(id bigint primary key);`,
		func(t *testing.T, back, from *irv1.Catalog) {
			t.Helper()
			col := findColumnInCat(back, "public", "t", "email")
			if col == nil {
				t.Fatalf("DOWN did not re-add the dropped column 'email'")
			}
		},
	)
}

func TestRoundTripAddForeignKey(t *testing.T) {
	roundTrip(t, []string{"public"},
		`CREATE TABLE a(id bigint primary key);
		 CREATE TABLE b(id bigint primary key, a_id bigint);`,
		`CREATE TABLE a(id bigint primary key);
		 CREATE TABLE b(id bigint primary key, a_id bigint references a(id));`,
	)
}

func TestRoundTripAddIndex(t *testing.T) {
	roundTrip(t, []string{"public"},
		`CREATE TABLE t(id bigint primary key, email text);`,
		`CREATE TABLE t(id bigint primary key, email text);
		 CREATE INDEX idx_t_email ON t(email);`,
	)
}

func TestRoundTripAddUniqueConstraint(t *testing.T) {
	roundTrip(t, []string{"public"},
		`CREATE TABLE t(id bigint primary key, email text);`,
		`CREATE TABLE t(id bigint primary key, email text, CONSTRAINT t_email_key UNIQUE (email));`,
	)
}

func TestRoundTripAddCheck(t *testing.T) {
	roundTrip(t, []string{"public"},
		`CREATE TABLE t(id bigint primary key, n int);`,
		`CREATE TABLE t(id bigint primary key, n int, CONSTRAINT t_n_check CHECK (n >= 0));`,
	)
}

func TestRoundTripEnumAddValue(t *testing.T) {
	// DOWN is inherently irreversible: PostgreSQL cannot drop an enum label, so
	// the generated down is a documented no-op warning comment. We assert UP
	// reaches the desired enum, and that DOWN leaves the added value in place
	// (i.e. it does not error and the catalog still has 'b').
	//
	// TODO: PostgreSQL has no DROP-enum-value primitive; a fully reversible down
	// would require recreating the type and rewriting every dependent column,
	// which the generator does not (and arguably should not) attempt.
	roundTripUpOnly(t, []string{"public"},
		`CREATE TYPE s AS ENUM ('a');`,
		`CREATE TYPE s AS ENUM ('a','b');`,
		func(t *testing.T, back, from *irv1.Catalog) {
			t.Helper()
			labels := findEnumLabels(back, "public", "s")
			if len(labels) != 2 || labels[0] != "a" || labels[1] != "b" {
				t.Fatalf("after down, enum s labels = %v, want [a b] (drop-enum-value is unsupported in PG)", labels)
			}
		},
	)
}

func TestRoundTripAddView(t *testing.T) {
	roundTrip(t, []string{"public"},
		`CREATE TABLE t(id bigint primary key, n int);`,
		`CREATE TABLE t(id bigint primary key, n int);
		 CREATE VIEW v AS SELECT id, n FROM t WHERE n > 0;`,
	)
}

func TestRoundTripAddFunctionAndTrigger(t *testing.T) {
	roundTrip(t, []string{"public"},
		`CREATE TABLE t(id bigint primary key, n int);`,
		`CREATE TABLE t(id bigint primary key, n int);
		 CREATE FUNCTION touch() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RETURN NEW; END $$;
		 CREATE TRIGGER t_touch BEFORE UPDATE ON t FOR EACH ROW EXECUTE FUNCTION touch();`,
	)
}

func TestRoundTripAddEnumType(t *testing.T) {
	roundTrip(t, []string{"public"},
		``,
		`CREATE TYPE mood AS ENUM ('happy','sad');`,
	)
}

func TestRoundTripAddDomainType(t *testing.T) {
	roundTrip(t, []string{"public"},
		``,
		`CREATE DOMAIN email AS text NOT NULL CHECK (VALUE ~ '@');`,
	)
}

func TestRoundTripAddCompositeType(t *testing.T) {
	roundTrip(t, []string{"public"},
		``,
		`CREATE TYPE addr AS (street text, city text, zip text);`,
	)
}

// findEnumLabels returns the labels of an enum type by schema.name, or nil.
func findEnumLabels(c *irv1.Catalog, schema, name string) []string {
	for _, s := range c.GetSchemas() {
		if s.GetName() != schema {
			continue
		}
		for _, e := range s.GetEnums() {
			if e.GetName().GetName() == name {
				return e.GetLabels()
			}
		}
	}
	return nil
}

// findColumnInCat locates a column by schema.table.column or returns nil.
func findColumnInCat(c *irv1.Catalog, schema, table, column string) *irv1.Column {
	for _, s := range c.GetSchemas() {
		if s.GetName() != schema {
			continue
		}
		for _, tb := range s.GetTables() {
			if tb.GetName().GetName() != table {
				continue
			}
			for _, col := range tb.GetColumns() {
				if col.GetName() == column {
					return col
				}
			}
		}
	}
	return nil
}
