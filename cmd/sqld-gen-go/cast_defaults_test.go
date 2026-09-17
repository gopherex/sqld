package main

import (
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/internal/query"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

// These query shapes cover the 14 remaining cast-only parameter sites in
// Komeet, along with nullable inputs that must survive the new default.
func TestGenerateCastParameterDefaults(t *testing.T) {
	stmts, err := parse.Statements(`
CREATE TABLE users(id uuid PRIMARY KEY, handle text NOT NULL, kind text NOT NULL,
    nickname text, verified_at timestamptz);
CREATE TABLE news(id uuid PRIMARY KEY, pinned bool NOT NULL, published_at timestamptz);
`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	qs, err := query.ParseQueries(`
-- name: ListPublishedNewsIds :many
SELECT id FROM news WHERE published_at IS NOT NULL
AND (@after_id::uuid IS NULL OR (pinned, published_at, id) <
    (@after_pinned::bool, @after_at::timestamptz, @after_id::uuid));
-- name: ListNewsAdminIds :many
SELECT id FROM news WHERE (@status::text = ''
    OR (@status = 'draft' AND published_at IS NULL)
    OR (@status = 'published' AND published_at <= @now))
    AND (@after::uuid IS NULL OR id < @after);
-- name: ListNotifications :many
SELECT id FROM users WHERE NOT @unread_only::bool OR verified_at IS NULL;
-- name: MatchHandleReservation :one
SELECT id FROM users WHERE handle_reservation_matches(handle, kind, @handle::text);
-- name: ListHandleEntries :many
SELECT id FROM users WHERE (@prefix::text = '' OR handle LIKE @prefix)
    AND (@holder_kind::text = '' OR @holder_kind = 'user')
    AND (@verified::text = '' OR (@verified = 'yes' AND verified_at IS NOT NULL))
    AND (NOT @reserved_only::bool OR handle_reservation_matches(handle, kind, handle));
-- name: FindUserIds :many
SELECT id FROM users WHERE (@query::text = '' OR handle LIKE @prefix::text)
    AND (@kind::text = '' OR @kind = 'human' OR @kind = 'bot')
    AND (@verified::text = '' OR (@verified = 'yes' AND verified_at IS NOT NULL))
    AND (@name_pattern::text = '' OR nickname LIKE @name_pattern);
-- name: FindChatIds :many
SELECT id FROM users WHERE (@query::text = '' OR handle LIKE @prefix::text)
    AND (@kind::text = '' OR kind = @kind)
    AND (@verified::text = '' OR (@verified = 'yes' AND verified_at IS NOT NULL))
    AND (NOT @public_only::bool OR handle <> '');
-- name: OptionalFunction :one
SELECT id FROM users WHERE handle_reservation_matches(handle, kind, @handle?::text);
-- name: NullableUpdate :exec
UPDATE users SET handle = COALESCE(@handle::text, handle), nickname = @nickname::text
WHERE id = @id;
`, "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	var diagnostics catalog.Diagnostics
	for _, q := range qs {
		query.Infer(q, cat, &diagnostics)
	}
	if len(diagnostics.Items) != 0 {
		t.Fatalf("inference diagnostics: %v", diagnostics.Items)
	}
	resp, err := Generate(&pluginv1.GenerateRequest{
		Catalog: cat, Queries: qs,
		Options: []byte(`{"nullMode":"pointer","overrides":{"uuid":"github.com/google/uuid.UUID"}}`),
	})
	if err != nil {
		t.Fatal(err)
	}
	var source []byte
	for _, file := range resp.GetFiles() {
		if file.GetPath() == "queries.go" {
			source = file.GetContents()
		}
	}
	got := generatedGoTypes(t, source)
	for field, want := range map[string]string{
		"ListPublishedNewsIdsParams.AfterPinned": "bool",
		"ListPublishedNewsIdsParams.AfterAt":     "time.Time",
		"ListNewsAdminIdsParams.Status":          "string",
		"ListNotifications.unreadOnly":           "bool",
		"MatchHandleReservation.handle":          "string",
		"ListHandleEntriesParams.HolderKind":     "string",
		"ListHandleEntriesParams.Verified":       "string",
		"ListHandleEntriesParams.ReservedOnly":   "bool",
		"FindUserIdsParams.Query":                "string",
		"FindUserIdsParams.Kind":                 "string",
		"FindUserIdsParams.Verified":             "string",
		"FindChatIdsParams.Query":                "string",
		"FindChatIdsParams.Verified":             "string",
		"FindChatIdsParams.PublicOnly":           "bool",
		// Guards and nullable-column contexts still require pointers.
		"ListPublishedNewsIdsParams.AfterID": "*uuid.UUID",
		"ListNewsAdminIdsParams.After":       "*uuid.UUID",
		"ListNewsAdminIdsParams.Now":         "*time.Time",
		"FindUserIdsParams.NamePattern":      "*string",
		"OptionalFunctionParams.Handle":      "*string",
		"NullableUpdateParams.Handle":        "*string",
		"NullableUpdateParams.Nickname":      "*string",
	} {
		if got[field] != want {
			t.Errorf("%s: got %q, want %q", field, got[field], want)
		}
	}
}
