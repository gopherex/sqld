package db

import (
	"context"
	_ "embed"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
)

//go:embed sqlc_schema.sql
var sqlcSchema string

func TestSQLCBindings(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("SQLD_DYNAMIC_TEST_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, sqlcSchema+"INSERT INTO profiles VALUES (1,'alice','bio'),(2,'bob','bio');"); err != nil {
		t.Fatal(err)
	}
	q := New(conn)
	summary := int64(42)
	if err := q.InsertHeads(ctx, InsertHeadsParams{Kinds: []string{"a", "b"}, SummaryTypes: []*int64{nil, &summary}}); err != nil {
		t.Fatal(err)
	}
	rowsHeads, err := conn.Query(ctx, "SELECT kind, summary_type FROM heads ORDER BY kind")
	if err != nil {
		t.Fatal(err)
	}
	var summaries []*int64
	for rowsHeads.Next() {
		var kind string
		var value *int64
		if err := rowsHeads.Scan(&kind, &value); err != nil {
			t.Fatal(err)
		}
		summaries = append(summaries, value)
	}
	rowsHeads.Close()
	if err := rowsHeads.Err(); err != nil {
		t.Fatal(err)
	}
	if len(summaries) != 2 || summaries[0] != nil || summaries[1] == nil || *summaries[1] != summary {
		t.Fatalf("nullable elements did not round-trip: %v", summaries)
	}
	kind := "b"
	kinds, err := q.FindKinds(ctx, FindKindsParams{Kinds: []*string{nil, &kind}})
	if err != nil || len(kinds) != 1 || kinds[0].Kind != kind {
		t.Fatalf("nullable text elements: %v %v", kinds, err)
	}
	for _, filter := range [][]string{nil, {}, {"b"}} {
		kinds, err := q.OptionalKinds(ctx, OptionalKindsParams{Kinds: filter})
		want := len(filter)
		if filter == nil {
			want = 2
		}
		if err != nil || len(kinds) != want {
			t.Fatalf("optional array %v: %v %v", filter, kinds, err)
		}
	}
	name := "alice"
	rows, err := q.FindName(ctx, nil)
	if err != nil || len(rows) != 0 {
		t.Fatalf("nil must retain predicate: rows=%v err=%v", rows, err)
	}
	rows, err = q.FindName(ctx, &name)
	if err != nil || len(rows) != 1 || rows[0].ID != 1 {
		t.Fatalf("value: rows=%v err=%v", rows, err)
	}
	dynamic, err := q.DynamicFind(ctx, DynamicFindParams{Name: nil})
	if err != nil || len(dynamic) != 0 {
		t.Fatalf("dynamic nil must retain narg predicate: %v %v", dynamic, err)
	}
	dynamic, err = q.DynamicFind(ctx, DynamicFindParams{Name: &name})
	if err != nil || len(dynamic) != 1 || dynamic[0].ID != 1 {
		t.Fatalf("dynamic value: %v %v", dynamic, err)
	}
	for _, ids := range [][]int64{nil, {}, {1}} {
		rows, err := q.FindIds(ctx, FindIdsParams{Ids: ids})
		if err != nil || len(rows) != len(ids) {
			t.Fatalf("array %v: rows=%v err=%v", ids, rows, err)
		}
	}
	repeated, err := q.RepeatName(ctx, nil)
	if err != nil || !repeated.Missing || repeated.Value != "fallback" {
		t.Fatalf("repeat nil: %v %v", repeated, err)
	}
	repeated, err = q.RepeatName(ctx, &name)
	if err != nil || repeated.Missing || repeated.Value != name {
		t.Fatalf("repeat value: %v %v", repeated, err)
	}
	n, err := q.UpdateName(ctx, UpdateNameParams{ID: 1})
	if err != nil || n != 1 {
		t.Fatalf("coalesce nil: %d %v", n, err)
	}
	var stored string
	if err := conn.QueryRow(ctx, "SELECT name FROM profiles WHERE id=1").Scan(&stored); err != nil || stored != "alice" {
		t.Fatalf("coalesce kept name: %q %v", stored, err)
	}
	if err := q.ClearBio(ctx, ClearBioParams{ID: 1}); err != nil {
		t.Fatal(err)
	}
	var cleared bool
	if err := conn.QueryRow(ctx, "SELECT bio IS NULL FROM profiles WHERE id=1").Scan(&cleared); err != nil || !cleared {
		t.Fatalf("NULL assignment: %v %v", cleared, err)
	}
	created, err := q.CreateProfile(ctx, CreateProfileParams{ID: 3})
	if err != nil || created.ID != 3 || created.Name != "default" || created.Bio != nil {
		t.Fatalf("CTE nil: %v %v", created, err)
	}
	mixed, err := q.MixedPositional(ctx, MixedPositionalParams{Value: 5, Amount: 7})
	if err != nil || mixed.Value == nil || *mixed.Value != 12 {
		t.Fatalf("mixed bindings: %v %v", mixed, err)
	}
	bio := "new bio"
	created, err = q.CreateProfile(ctx, CreateProfileParams{ID: 4, Name: &name, Bio: &bio})
	if err != nil || created.Name != name || created.Bio == nil || *created.Bio != bio {
		t.Fatalf("CTE values: %v %v", created, err)
	}
}
