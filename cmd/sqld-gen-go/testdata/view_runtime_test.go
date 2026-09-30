package db

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

//go:embed view_schema.sql
var schemaSQL string

func TestViewScanning(t *testing.T) {
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, os.Getenv("SQLD_DYNAMIC_TEST_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	if _, err := conn.Exec(ctx, schemaSQL); err != nil {
		t.Fatal(err)
	}
	if _, err := conn.Exec(ctx, `INSERT INTO backplane.a VALUES ('00000000-0000-0000-0000-000000000001','2026-09-30T12:00:00Z','{"n":1}');
INSERT INTO backplane.b VALUES ('00000000-0000-0000-0000-000000000002','2026-09-30T13:00:00Z','{"n":2}');`); err != nil {
		t.Fatal(err)
	}
	q := New(conn)
	check := func(source string, id uuid.UUID, at time.Time, payload json.RawMessage) {
		t.Helper()
		if source != "a" || id != uuid.MustParse("00000000-0000-0000-0000-000000000001") || !at.Equal(time.Date(2026, 9, 30, 12, 0, 0, 0, time.UTC)) || !json.Valid(payload) {
			t.Fatalf("row: %s %s %s %s", source, id, at, payload)
		}
	}
	plain, err := q.FeedPlain(ctx)
	if err != nil || len(plain) != 2 {
		t.Fatalf("plain: %v %v", plain, err)
	}
	check(plain[0].Source, plain[0].ID, plain[0].T, plain[0].Attributes)
	aliased, err := q.FeedAliased(ctx)
	if err != nil || len(aliased) != 2 {
		t.Fatalf("aliased: %v %v", aliased, err)
	}
	check(aliased[0].Source, aliased[0].ID, aliased[0].T, aliased[0].Attributes)
	star, err := q.FeedStar(ctx)
	if err != nil || len(star) != 2 {
		t.Fatalf("star: %v %v", star, err)
	}
	check(star[0].Source, star[0].ID, star[0].T, star[0].Attributes)
	lateral, err := q.FeedLateral(ctx)
	if err != nil || len(lateral) != 2 || lateral[0].ID != plain[0].ID || !json.Valid(lateral[0].Attributes) {
		t.Fatalf("lateral: %v %v", lateral, err)
	}
	rows, err := q.FeedPayload(ctx)
	if err != nil || len(rows) != 2 {
		t.Fatalf("json: %v %v", rows, err)
	}
	var payload struct {
		ID         uuid.UUID
		Source     string
		Attributes json.RawMessage
	}
	if err := json.Unmarshal(rows[0].Payload, &payload); err != nil || payload.ID != plain[0].ID || payload.Source != "a" || !json.Valid(payload.Attributes) {
		t.Fatalf("json payload: %s %v", rows[0].Payload, err)
	}
	nulls, err := q.FeedNullable(ctx)
	if err != nil || len(nulls) != 2 || nulls[0].ID != nil || nulls[1].ID == nil || *nulls[1].ID != plain[0].ID {
		t.Fatalf("nullable: %v %v", nulls, err)
	}
	source := "b"
	for _, tc := range []struct {
		params FeedOptionalParams
		count  int
	}{
		{FeedOptionalParams{}, 2},
		{FeedOptionalParams{Source: &source}, 1},
		{FeedOptionalParams{Ids: []uuid.UUID{plain[0].ID}}, 1},
		{FeedOptionalParams{Ids: []uuid.UUID{}}, 0},
		{FeedOptionalParams{Source: &source, Ids: []uuid.UUID{plain[0].ID}}, 0},
	} {
		got, err := q.FeedOptional(ctx, tc.params)
		if err != nil || len(got) != tc.count {
			t.Fatalf("optional %+v: rows=%v err=%v", tc.params, got, err)
		}
	}
}
