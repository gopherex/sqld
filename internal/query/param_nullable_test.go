package query

import (
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
)

func TestInferParameterNullability(t *testing.T) {
	stmts, err := parse.Statements(`CREATE TABLE users(id uuid PRIMARY KEY, handle text NOT NULL, nickname text);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	for _, tc := range []struct {
		name, sql, typ string
		nullable       bool
	}{
		{"column", "SELECT id FROM users WHERE id = @p", "uuid", false},
		{"cast_column", "SELECT id FROM users WHERE id = @p::uuid", "uuid", false},
		{"function_column", "SELECT id FROM users WHERE lower(handle) = lower(@p::text)", "text", false},
		{"nullable_column", "SELECT id FROM users WHERE nickname = @p::text", "text", true},
		{"repeated", "SELECT id FROM users WHERE handle = @p AND length(@p) > 0", "text", false},
		{"repeated_reverse", "SELECT id FROM users WHERE length(@p) > 0 AND handle = @p", "text", false},
		{"literal_guard", "SELECT id FROM users WHERE @p::text = '' OR handle = @p", "text", false},
		{"nullable_literal_guard", "SELECT id FROM users WHERE @p::text = '' OR nickname = @p", "text", true},
		{"mixed_columns", "SELECT id FROM users WHERE handle = @p OR nickname = @p", "text", false},
		{"mixed_columns_reverse", "SELECT id FROM users WHERE nickname = @p OR handle = @p", "text", false},
		{"null_safe_repeated", "SELECT id FROM users WHERE handle = @p AND nickname IS DISTINCT FROM @p", "text", false},
		{"left_join_filter", "SELECT a.id FROM users a LEFT JOIN users b ON b.id = a.id WHERE b.handle ILIKE @p", "text", false},
		{"null_guard", "SELECT id FROM users WHERE @p::uuid IS NULL OR id > @p", "uuid", true},
		{"null_guard_reverse", "SELECT id FROM users WHERE id > @p OR @p::uuid IS NULL", "uuid", true},
		{"optional", "SELECT id FROM users WHERE id = @p?", "uuid", true},
		{"coalesce", "UPDATE users SET handle = COALESCE(@p::text, handle)", "text", true},
		{"coalesce_repeated", "SELECT id FROM users WHERE handle = @p AND COALESCE(@p, '') <> ''", "text", true},
		{"coalesce_fallback", "SELECT COALESCE((SELECT id FROM users WHERE id = @p::uuid), @p::uuid)", "uuid", false},
		{"insert_select", "INSERT INTO users(handle) SELECT @p::text", "text", false},
		{"insert_select_optional", "INSERT INTO users(handle) SELECT COALESCE(@p::text, '')", "text", true},
		{"derived_nullable", "SELECT id FROM (SELECT b.id FROM users a LEFT JOIN users b ON b.id = a.id) v WHERE id = @p", "uuid", true},
		{"derived_required", "SELECT id FROM (SELECT id FROM users) v WHERE id = @p", "uuid", false},
		{"right_join_filter", "SELECT b.id FROM users a RIGHT JOIN users b ON b.id = a.id WHERE a.handle = @p", "text", false},
		{"full_join_filter", "SELECT b.id FROM users a FULL JOIN users b ON b.id = a.id WHERE a.handle = @p", "text", false},
		{"cast_alone", "SELECT @p::text", "text", false},
		{"cast_literal_filter", "SELECT id FROM users WHERE @p::text = '' OR @p = 'human'", "text", false},
		{"cast_literal_filter_reverse", "SELECT id FROM users WHERE @p = 'human' OR @p::text = ''", "text", false},
		{"cast_not", "SELECT id FROM users WHERE NOT @p::bool", "bool", false},
		{"cast_boolean_literal", "SELECT id FROM users WHERE @p::bool = false", "bool", false},
		{"cast_unknown_function", "SELECT app.matches(handle, @p::text) FROM users", "text", false},
		{"cast_optional_function", "SELECT app.matches(handle, @p?::text) FROM users", "text", true},
		{"cast_timestamp_literal", "SELECT @p::timestamptz < now()", "timestamptz", false},
		{"cast_syntax", "SELECT CAST(@p AS text)", "text", false},
		{"cast_nullable_function", "SELECT id FROM users WHERE lower(nickname) = lower(@p::text)", "text", true},
		{"cast_nullable_set", "UPDATE users SET nickname = @p::text", "text", true},
		{"cast_nullable_insert", "INSERT INTO users(nickname) VALUES (@p::text)", "text", true},
		{"cast_nullable_insert_select", "INSERT INTO users(nickname) SELECT @p::text", "text", true},
		{"cast_nullable_repeated", "SELECT id FROM users WHERE nickname = @p AND lower(@p::text) = ''", "text", true},
		{"cast_nullable_repeated_reverse", "SELECT id FROM users WHERE lower(@p::text) = '' AND nickname = @p", "text", true},
		{"cast_mixed_columns", "SELECT id FROM users WHERE handle = @p::text OR nickname = @p", "text", false},
		{"cast_mixed_columns_reverse", "SELECT id FROM users WHERE nickname = @p::text OR handle = @p", "text", false},
		{"cast_unknown_without_cast", "SELECT lower(@p)", "text", true},
		{"cast_result_only", "SELECT length(@p)::bigint", "text", true},
		{"cast_null_safe", "SELECT id FROM users WHERE handle IS NOT DISTINCT FROM @p::text", "text", true},
		{"cast_set", "UPDATE users SET handle = @p::text", "text", false},
		{"cast_insert", "INSERT INTO users(handle) VALUES (@p::text)", "text", false},
		{"limit", "SELECT id FROM users LIMIT @p", "int8", false},
		{"offset", "SELECT id FROM users OFFSET @p", "int8", false},
		{"cast_limit", "SELECT id FROM users LIMIT @p::bigint", "int8", false},
		{"optional_limit", "SELECT id FROM users LIMIT @p?", "int8", true},
		{"coalesce_limit", "SELECT id FROM users LIMIT COALESCE(@p::bigint, 10)", "int8", true},
		{"null_safe", "SELECT id FROM users WHERE handle IS NOT DISTINCT FROM @p", "text", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			qs, err := ParseQueries("-- name: Q :many\n"+tc.sql, "q.sql")
			if err != nil {
				t.Fatal(err)
			}
			var d catalog.Diagnostics
			Infer(qs[0], cat, &d)
			if len(qs[0].GetParameters()) != 1 {
				t.Fatalf("parameters: %v", qs[0].GetParameters())
			}
			p := qs[0].GetParameters()[0]
			if p.GetType().GetPgName() != tc.typ || p.GetNullable() != tc.nullable {
				t.Errorf("parameter = %v, want %s nullable=%v", p, tc.typ, tc.nullable)
			}
		})
	}
}

func TestInferCastKeysetNullability(t *testing.T) {
	stmts, err := parse.Statements(`CREATE TABLE news(
		id uuid PRIMARY KEY, pinned bool NOT NULL, published_at timestamptz);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	for _, predicate := range []string{
		"@after_id::uuid IS NULL OR (pinned, published_at, id) < (@after_pinned::bool, @after_at::timestamptz, @after_id::uuid)",
		"(pinned, published_at, id) < (@after_pinned::bool, @after_at::timestamptz, @after_id::uuid) OR @after_id::uuid IS NULL",
	} {
		for _, positional := range []bool{false, true} {
			sql := "SELECT id FROM news WHERE published_at IS NOT NULL AND (" + predicate + ")"
			if positional {
				sql = strings.NewReplacer("@after_id", "$1", "@after_pinned", "$2", "@after_at", "$3").Replace(sql)
			}
			t.Run(sql, func(t *testing.T) {
				qs, err := ParseQueries("-- name: Q :many\n"+sql, "q.sql")
				if err != nil {
					t.Fatal(err)
				}
				var d catalog.Diagnostics
				Infer(qs[0], cat, &d)
				if len(qs[0].GetParameters()) != 3 || len(d.Items) != 0 {
					t.Fatalf("parameters=%v diagnostics=%v", qs[0].GetParameters(), d.Items)
				}
				for _, p := range qs[0].GetParameters() {
					wantNullable := p.GetName() == "after_id" || (positional && p.GetNumber() == 1)
					if p.GetNullable() != wantNullable {
						t.Errorf("parameter %v: nullable=%v, want %v", p, p.GetNullable(), wantNullable)
					}
				}
			})
		}
	}
}
