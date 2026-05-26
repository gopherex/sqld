package example

import (
	"testing"

	"github.com/yaroher/sqld/bob/example/gen/bob/models"
	"github.com/yaroher/sqld/example/gen/db"
)

// TestBobSqldShareLeafTypes is a compile-level proof of the ORM⊕sqlc symbiosis:
// the bob ORM model, the sqld-gen-go model, and a sqld-gen-go query row all use
// the SAME canonical Go type (db.AppUserStatus) for the status column, so values
// flow between bob and sqld code without conversion. (If the generators emitted
// different types for the column, this file would not compile.)
func TestBobSqldShareLeafTypes(t *testing.T) {
	var s db.AppUserStatus = db.AppUserStatusActive

	// bob ORM model field is db.AppUserStatus.
	var bobModel models.AppUser
	bobModel.Status = s

	// sqld-gen-go model field is db.AppUserStatus.
	var sqldModel db.AppUsers
	sqldModel.Status = s

	// sqld-gen-go query row field is db.AppUserStatus.
	var row db.GetUserRow
	row.Status = s

	// Cross-assign every direction: identical type, no conversion.
	bobModel.Status = sqldModel.Status
	sqldModel.Status = row.Status
	row.Status = bobModel.Status

	if bobModel.Status != db.AppUserStatusActive {
		t.Fatalf("status = %q", bobModel.Status)
	}
}
