package query

import (
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

// Exercise the SQL -> IR -> parameter inference path, including clauses whose
// parameters used to be missed by the hand-written AST traversal.
func TestInferExplicitParamCasts(t *testing.T) {
	stmts, err := parse.Statements("CREATE TABLE users(id bigint PRIMARY KEY, col text NOT NULL);")
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	tests := []struct {
		name, sql, pgType string
		nullable          bool
	}{
		{"coalesce_set", "UPDATE users SET col = COALESCE(@p::text, col)", "text", true},
		{"direct_set", "UPDATE users SET col = @p::text", "text", false},
		{"cast_syntax", "UPDATE users SET col = COALESCE(CAST(@p AS text), col)", "text", true},
		{"select", "SELECT @p::text AS value", "text", true},
		{"where", "SELECT id FROM users WHERE col = @p::text", "text", false},
		{"insert_values", "INSERT INTO users (col) VALUES (@p::text)", "text", false},
		{"insert_select", "INSERT INTO users (col) SELECT @p::text", "text", false},
		{"insert_returning", "INSERT INTO users (col) VALUES ('x') RETURNING @p::text", "text", true},
		{"update_returning", "UPDATE users SET col = 'x' RETURNING @p::text", "text", true},
		{"delete_returning", "DELETE FROM users RETURNING @p::text", "text", true},
		{"case", "UPDATE users SET col = CASE WHEN id > 0 THEN @p::text ELSE col END", "text", true},
		{"function", "UPDATE users SET col = lower(@p::text)", "text", false},
		{"greatest", "UPDATE users SET col = greatest(@p::text, col)", "text", true},
		{"least", "UPDATE users SET col = least(col, @p::text)", "text", true},
		{"nullif", "UPDATE users SET col = nullif(@p::text, col)", "text", true},
		{"operator", "UPDATE users SET col = @p::text || col", "text", false},
		{"nested_cast", "UPDATE users SET col = (@p::integer)::text", "int4", false},
		{"row", "SELECT ROW(@p::text, 1)", "text", true},
		{"subquery", "UPDATE users SET col = (SELECT @p::text)", "text", true},
		{"subquery_operand", "SELECT id FROM users WHERE @p::text IN (SELECT col FROM users)", "text", true},
		{"join", "SELECT u.id FROM users u JOIN users v ON v.col = @p::text", "text", false},
		{"from_function", "SELECT * FROM unnest(@p::text[])", "text[]", true},
		{"from_values", "SELECT v.col FROM (VALUES (@p::text)) AS v(col)", "text", true},
		{"values", "VALUES (@p::text)", "text", true},
		{"union", "SELECT 'x'::text UNION ALL SELECT @p::text", "text", true},
		{"cte_select", "WITH v AS (SELECT @p::text AS col) SELECT col FROM v", "text", true},
		{"cte_insert", "WITH v AS (SELECT @p::text AS col) INSERT INTO users (col) SELECT col FROM v", "text", true},
		{"cte_update", "WITH v AS (SELECT @p::text AS col) UPDATE users SET col = (SELECT col FROM v)", "text", true},
		{"cte_delete", "WITH v AS (SELECT @p::text AS col) DELETE FROM users WHERE col IN (SELECT col FROM v)", "text", true},
		{"conflict_set", "INSERT INTO users (id, col) VALUES (1, 'x') ON CONFLICT (id) DO UPDATE SET col = COALESCE(@p::text, users.col)", "text", true},
		{"conflict_where", "INSERT INTO users (id, col) VALUES (1, 'x') ON CONFLICT (id) DO UPDATE SET col = 'y' WHERE users.col = @p::text", "text", false},
		{"update_from", "UPDATE users SET col = v.col FROM (SELECT @p::text AS col) v", "text", true},
		{"delete_using", "DELETE FROM users USING (SELECT @p::text AS col) v WHERE users.col = v.col", "text", true},
		{"order_by", "SELECT id FROM users ORDER BY @p::text", "text", true},
		{"limit", "SELECT id FROM users LIMIT @p::bigint", "int8", false},
		{"offset", "SELECT id FROM users OFFSET @p::bigint", "int8", false},
		{"distinct_on", "SELECT DISTINCT ON (@p::text) id FROM users", "text", true},
		{"group_by", "SELECT count(*) FROM users GROUP BY @p::text", "text", true},
		{"having", "SELECT count(*) FROM users HAVING count(*) > @p::bigint", "int8", true},
		{"aggregate_filter", "SELECT count(*) FILTER (WHERE col = @p::text) FROM users", "text", false},
		{"aggregate_order", "SELECT array_agg(col ORDER BY @p::text) FROM users", "text", true},
		{"window_inline", "SELECT row_number() OVER (PARTITION BY @p::text ORDER BY id) FROM users", "text", true},
		{"window_named", "SELECT row_number() OVER w FROM users WINDOW w AS (ORDER BY @p::text)", "text", true},
		{"uuid", "SELECT @p::uuid", "uuid", true},
		{"jsonb", "SELECT @p::jsonb", "jsonb", true},
		{"array", "SELECT @p::text[]", "text[]", true},
		{"cast_before_context", "SELECT @p::integer FROM users WHERE id = @p", "int4", false},
		{"context_before_cast", "SELECT id FROM users WHERE id = @p AND @p::integer > 0", "int4", false},
	}
	for _, tt := range tests {
		for _, named := range []bool{true, false} {
			name, sql := tt.name+"/named", tt.sql
			if !named {
				name, sql = tt.name+"/positional", strings.ReplaceAll(sql, "@p", "$1")
			}
			t.Run(name, func(t *testing.T) {
				qs, err := ParseQueries("-- name: Q :one\n"+sql+";", "q.sql")
				if err != nil {
					t.Fatal(err)
				}
				if len(qs) != 1 {
					t.Fatalf("queries=%d, want 1", len(qs))
				}
				var d catalog.Diagnostics
				Infer(qs[0], cat, &d)
				params := qs[0].GetParameters()
				if len(params) != 1 {
					t.Fatalf("params=%v, want one parameter", params)
				}
				p := params[0]
				if p.GetNumber() != 1 || (named && p.GetName() != "p") {
					t.Errorf("parameter identity lost: %v", p)
				}
				if got, want := p.GetType().GetPgName(), strings.TrimSuffix(tt.pgType, "[]"); got != want {
					t.Errorf("type=%q, want %q", got, want)
				}
				if p.GetNullable() != tt.nullable {
					t.Errorf("nullable=%v, want %v", p.GetNullable(), tt.nullable)
				}
				if tt.pgType == "text[]" {
					if p.GetType().GetKind() != irv1.TypeKind_TYPE_KIND_ARRAY || p.GetType().GetElement().GetPgName() != "text" {
						t.Errorf("array type lost: %v", p.GetType())
					}
				}
				for _, item := range d.Items {
					if strings.Contains(item.Message, "param $1 type unresolved") {
						t.Errorf("unexpected diagnostic: %v", item)
					}
				}
			})
		}
	}
}

func TestInferResultCastDoesNotReplaceArgumentType(t *testing.T) {
	for _, sql := range []string{
		"SELECT length($1)::bigint",
		"SELECT CAST(COALESCE($1, $2) AS uuid)",
	} {
		t.Run(sql, func(t *testing.T) {
			qs, err := ParseQueries("-- name: Q :one\n"+sql, "q.sql")
			if err != nil {
				t.Fatal(err)
			}
			var d catalog.Diagnostics
			Infer(qs[0], nil, &d)
			if len(qs[0].GetParameters()) == 0 {
				t.Fatal("parameters were lost")
			}
			for _, p := range qs[0].GetParameters() {
				if p.GetType().GetPgName() != "text" {
					t.Errorf("argument type must come from the function, not its result cast: %v", p)
				}
			}
		})
	}
}

func TestInferCastPreservesOptionalAndRepeatedParameters(t *testing.T) {
	qs, err := ParseQueries("-- name: Q :one\nSELECT @p::text WHERE @p?::text IS NOT NULL", "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	var d catalog.Diagnostics
	Infer(qs[0], nil, &d)
	params := qs[0].GetParameters()
	if len(params) != 1 {
		t.Fatalf("params=%v, want one shared parameter", params)
	}
	p := params[0]
	if p.GetName() != "p" || !p.GetOptional() || !p.GetNullable() || p.GetType().GetPgName() != "text" {
		t.Fatalf("parameter metadata lost: %v", p)
	}
	if p.GetColumn() != nil {
		t.Fatalf("cast-only parameter has no originating column: %v", p)
	}
}
