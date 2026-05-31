package example

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	bobpgx "github.com/stephenafamo/bob/drivers/pgx"

	"github.com/gopherex/sqld/cmd/sqld-gen-bob/example/gen/bob/models"
	"github.com/gopherex/sqld/example/gen/db"
	"github.com/gopherex/sqld/pkg/devdb"
)

func strptr(s string) *string                        { return &s }
func statusptr(s db.AppUserStatus) *db.AppUserStatus { return &s }

// usersDDL is the minimal slice of the example schema needed for the app.users
// model: the enum, the email domain, and the table. It avoids the extension
// types (hstore/ltree) so the round-trip isolates the bob⊕sqld symbiosis. The
// enum is a string-kind type and needs no codec registration.
const usersDDL = `
CREATE SCHEMA app;
CREATE TYPE app.user_status AS ENUM ('active', 'inactive', 'banned');
CREATE DOMAIN app.email AS text NOT NULL CHECK (VALUE ~ '@');
CREATE TABLE app.users (
  id          bigserial PRIMARY KEY,
  email       app.email NOT NULL UNIQUE,
  status      app.user_status NOT NULL DEFAULT 'active',
  manager_id  bigint REFERENCES app.users(id) ON DELETE SET NULL,
  created_at  timestamptz NOT NULL DEFAULT now()
);`

// TestBobSqldRuntimeSymbiosis proves the runtime half of the ORM⊕sqlc symbiosis:
// a row written through bob's ORM is read back identically through raw pgx, on
// ONE *pgxpool.Pool, sharing the same db.AppUserStatus type. Docker-gated.
func TestBobSqldRuntimeSymbiosis(t *testing.T) {
	ctx := context.Background()

	dev, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer dev.Close()

	if err := dev.Apply(ctx, usersDDL); err != nil {
		t.Fatalf("apply schema: %v", err)
	}

	pool, err := pgxpool.New(ctx, dev.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	exec := bobpgx.NewPool(pool) // bob ORM over the SAME pool

	// Write via bob's ORM, using the shared db.AppUserStatus type.
	inserted, err := models.AppUsers.Insert(&models.AppUserSetter{
		Email:  strptr("ada@example.com"),
		Status: statusptr(db.AppUserStatusInactive),
	}).One(ctx, exec)
	if err != nil {
		t.Fatalf("bob insert: %v", err)
	}
	if inserted.Status != db.AppUserStatusInactive {
		t.Fatalf("bob inserted.Status = %q; want inactive", inserted.Status)
	}

	// Read back via raw pgx on the same pool, scanning into the shared type.
	var got db.AppUserStatus
	if err := pool.QueryRow(ctx,
		"SELECT status FROM app.users WHERE id = $1", inserted.ID,
	).Scan(&got); err != nil {
		t.Fatalf("raw read: %v", err)
	}
	if got != db.AppUserStatusInactive {
		t.Fatalf("raw read status = %q; want inactive (written by bob)", got)
	}
}
