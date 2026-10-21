package db

import (
	"bytes"
	"context"
	_ "embed"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

//go:embed cte_dml_schema.sql
var cteSchema string

func TestDataModifyingCTERuntime(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("SQLD_DYNAMIC_TEST_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, cteSchema); err != nil {
		t.Fatal(err)
	}
	q := New(conn)
	id := []byte("feed")
	created, err := q.CreateFeed(ctx, CreateFeedParams{Kind: "chat", ID: id})
	if err != nil || created.Purged {
		t.Fatalf("create: %v %v", created, err)
	}
	var isNull bool
	if err := conn.QueryRow(ctx, "SELECT parent_kind IS NULL AND parent_id IS NULL FROM feeds").Scan(&isNull); err != nil || !isNull {
		t.Fatalf("nullable parents: %v %v", isNull, err)
	}
	parent := "root"
	changed, err := q.ChangeParent(ctx, ChangeParentParams{Kind: "chat", ID: id, ParentKind: &parent, ParentID: []byte("parent")})
	if err != nil || changed.ParentKind == nil || *changed.ParentKind != parent {
		t.Fatalf("update CTE: %v %v", changed, err)
	}
	changed, err = q.ChangeParent(ctx, ChangeParentParams{Kind: "chat", ID: id})
	if err != nil || changed.ParentKind != nil || changed.ParentID != nil {
		t.Fatalf("clear parent: %v %v", changed, err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO purged_feeds VALUES ('chat',$1)", []byte("purged")); err != nil {
		t.Fatal(err)
	}
	purged, err := q.CreateFeed(ctx, CreateFeedParams{Kind: "chat", ID: []byte("purged")})
	if err != nil || !purged.Purged {
		t.Fatalf("purged: %v %v", purged, err)
	}
	for _, owner := range [][]byte{[]byte("alice"), []byte("bob")} {
		bound, err := q.BindDevice(ctx, BindDeviceParams{Device: []byte("device"), Owner: owner})
		if err != nil || !bytes.Equal(bound.Owner, []byte("alice")) {
			t.Fatalf("bind: %v %v", bound, err)
		}
	}
	if _, err := conn.Exec(ctx, "INSERT INTO device_delivery VALUES ($1,3)", []byte("device")); err != nil {
		t.Fatal(err)
	}
	swept, err := q.SweepDevices(ctx, SweepDevicesParams{DeadBefore: time.Now().Add(time.Hour), Limit: 10})
	if err != nil || swept.Removed != 1 {
		t.Fatalf("sweep: %v %v", swept, err)
	}
	var count int
	if err := conn.QueryRow(ctx, "SELECT count(*) FROM device_delivery").Scan(&count); err != nil || count != 0 {
		t.Fatalf("delivery CTE did not execute: %d %v", count, err)
	}
	if _, err := conn.Exec(ctx, "INSERT INTO feed_items VALUES ('chat',$1,1,'infinity')", id); err != nil {
		t.Fatal(err)
	}
	if err := q.StoreSnapshot(ctx, StoreSnapshotParams{Kind: "chat", ID: id, Position: 2, Type: 42, Payload: []byte("snapshot")}); err != nil {
		t.Fatal(err)
	}
	var expired bool
	if err := conn.QueryRow(ctx, "SELECT expires < 'infinity' FROM feed_items").Scan(&expired); err != nil || !expired {
		t.Fatalf("snapshot CTE: expired=%v %v", expired, err)
	}
	row, err := q.SnapshotType(ctx, SnapshotTypeParams{Kind: "other", ID: id, Type: 43})
	if err != nil || row.Type != 43 {
		t.Fatalf("returning override: %v %v", row, err)
	}
}
