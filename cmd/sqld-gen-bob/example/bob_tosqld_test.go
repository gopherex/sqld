package example

import (
	"testing"

	"github.com/aarondl/opt/null"
	"github.com/gopherex/sqld/cmd/sqld-gen-bob/example/gen/bob/models"
	"github.com/gopherex/sqld/example/gen/db"
)

// TestToSqld exercises the auto-generated bob→sqld bridge (sqld_bridge.go),
// emitted whenever the bob plugin's typesPackage is set. The bob model and the
// sqld-gen-go model are NOT Go-convertible (bob carries R/C ORM fields and wraps
// nullable columns in null.Val[T]), so ToSqld field-copies into the flat db.*
// model, unwrapping null.Val[T] to what sqld-gen-go emits.
func TestToSqld(t *testing.T) {
	var u models.AppUser
	u.Status = db.AppUserStatusActive
	got := u.ToSqld() // returns db.AppUsers (the full-table model)

	var _ db.AppUsers = got // compile-time: return type is the gen-go model

	if got.Status != db.AppUserStatusActive {
		t.Fatalf("status = %v; want %v", got.Status, db.AppUserStatusActive)
	}
	// A NULL bob null.Val[int64] unwraps to a nil *int64 in pointer mode.
	if got.ManagerID != nil {
		t.Fatalf("ManagerID = %v; want nil", got.ManagerID)
	}

	// Set the nullable manager_id and confirm .Ptr() bridges the value through.
	u.ManagerID = null.From[int64](7)
	if mgr := u.ToSqld().ManagerID; mgr == nil || *mgr != 7 {
		t.Fatalf("ManagerID = %v; want *7", mgr)
	}
}

// TestToSqldNilCapable covers the nil-capable branch: bob still wraps these
// nullable columns in null.Val[T], but sqld-gen-go emits the bare value type, so
// the bridge unwraps via .GetOrZero().
func TestToSqldNilCapable(t *testing.T) {
	var k models.AppKitchenSink
	got := k.ToSqld() // returns db.AppKitchenSink

	var _ db.AppKitchenSink = got
	if got.CJsonb != nil {
		t.Fatalf("CJsonb = %v; want nil (zero map from unset null.Val)", got.CJsonb)
	}
	if got.CIntArray != nil {
		t.Fatalf("CIntArray = %v; want nil", got.CIntArray)
	}
}
