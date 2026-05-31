package diff

import (
	"context"
	"strings"
	"testing"

	"github.com/gopherex/sqld/pkg/devdb"
	"github.com/jackc/pgx/v5"
)

// TestDiffChangedCheckDownReversesUp is a pure (DB-free) regression for C2:
// when a CHECK constraint changes, the up expands to [Drop(old), Add(new)] in
// the same sortKey bucket, so the down must be the exact reverse —
// DROP the new constraint, THEN re-ADD the old one (in that order). The old
// implementation re-sorted descending, which kept the pair in up-order and so
// emitted the inverse in the wrong order.
func TestDiffChangedCheckDownReversesUp(t *testing.T) {
	from := cat(t, "CREATE TABLE t(a int, CONSTRAINT chk CHECK (a > 0));")
	to := cat(t, "CREATE TABLE t(a int, CONSTRAINT chk CHECK (a > 1));")
	p, err := Diff(from, to)
	if err != nil {
		t.Fatal(err)
	}

	up := p.UpSQL()
	// Up: drop the old check, then add the new one.
	dropUp := strings.Index(up, "DROP CONSTRAINT")
	addUp := strings.Index(up, "ADD CONSTRAINT")
	if dropUp < 0 || addUp < 0 || dropUp > addUp {
		t.Fatalf("up should DROP old then ADD new:\n%s", up)
	}
	if !strings.Contains(up, `"a" > 1`) {
		t.Fatalf("up should add the new check (a > 1):\n%s", up)
	}

	down := p.DownSQL()
	dropDown := strings.Index(down, "DROP CONSTRAINT")
	addDown := strings.Index(down, "ADD CONSTRAINT")
	if dropDown < 0 || addDown < 0 {
		t.Fatalf("down should DROP then ADD a constraint:\n%s", down)
	}
	// Down must DROP (the new) before ADD (the old). Pre-fix, this was reversed.
	if dropDown > addDown {
		t.Fatalf("down must DROP new before re-ADD old (got ADD before DROP):\n%s", down)
	}
	if !strings.Contains(down, `"a" > 0`) {
		t.Fatalf("down should re-add the original check (a > 0):\n%s", down)
	}
}

// TestDiffChangedCheckRoundTripPG (C2) proves on a real Postgres that applying
// the up migration and then the down migration round-trips cleanly: the table
// ends with the original CHECK (a > 0), enforced. A wrongly-ordered down (the
// pre-fix behaviour) fails because it tries to ADD the old constraint before
// DROPping the new one of the same name -> "constraint already exists".
func TestDiffChangedCheckRoundTripPG(t *testing.T) {
	ctx := context.Background()
	d, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer d.Close()

	base := "CREATE TABLE t(a int, CONSTRAINT chk CHECK (a > 0));"
	if err := d.Apply(ctx, base); err != nil {
		t.Fatalf("apply base: %v", err)
	}

	from := cat(t, base)
	to := cat(t, "CREATE TABLE t(a int, CONSTRAINT chk CHECK (a > 1));")
	p, err := Diff(from, to)
	if err != nil {
		t.Fatal(err)
	}

	if err := d.Apply(ctx, p.UpSQL()); err != nil {
		t.Fatalf("apply up: %v\n%s", err, p.UpSQL())
	}
	// After up, the new constraint (a > 1) is enforced: a=1 must be rejected.
	if violatesCheck(ctx, t, d, "INSERT INTO t(a) VALUES (1);") == false {
		t.Fatalf("after up, CHECK (a > 1) should reject a=1")
	}

	if err := d.Apply(ctx, p.DownSQL()); err != nil {
		t.Fatalf("apply down: %v\n%s", err, p.DownSQL())
	}
	// After down, the original constraint (a > 0) is back: a=1 is accepted,
	// a=0 is rejected.
	conn, err := pgx.Connect(ctx, d.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, "INSERT INTO t(a) VALUES (1);"); err != nil {
		t.Fatalf("after down, a=1 should be accepted under CHECK (a > 0): %v", err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO t(a) VALUES (0);"); err == nil {
		t.Fatalf("after down, a=0 should be rejected under CHECK (a > 0)")
	}
}

// violatesCheck reports whether executing stmt fails (a constraint violation).
func violatesCheck(ctx context.Context, t *testing.T, d *devdb.DevDB, stmt string) bool {
	t.Helper()
	conn, err := pgx.Connect(ctx, d.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	_, err = conn.Exec(ctx, stmt)
	return err != nil
}
