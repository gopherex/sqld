package query

import (
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
)

func TestInferCoalesceAndOuterJoinNullability(t *testing.T) {
	stmts, err := parse.Statements(`CREATE TABLE tenants(id bigint PRIMARY KEY, name text NOT NULL);
		CREATE TABLE users(tenant_id bigint NOT NULL, email text NOT NULL, nickname text);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	for _, tc := range []struct {
		name, sql string
		nullable  []bool
	}{
		{"left", "SELECT t.name, u.email FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id", []bool{false, true}},
		{"fallback", "SELECT COALESCE(u.email, '') FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id", []bool{false}},
		{"nullable_fallback", "SELECT COALESCE(u.email, u.nickname) FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id", []bool{true}},
		{"left_column_fallback", "SELECT COALESCE(u.email, t.name) FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id", []bool{false}},
		{"cast_fallback", "SELECT COALESCE(u.email, ''::text) FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id", []bool{false}},
		{"cast_null", "SELECT COALESCE(u.email, NULL::text) FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id", []bool{true}},
		{"nested", "SELECT COALESCE(lower(u.email), COALESCE(u.nickname, '')) FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id", []bool{false}},
		{"right", "SELECT t.name, u.email FROM tenants t RIGHT JOIN users u ON u.tenant_id = t.id", []bool{true, false}},
		{"full", "SELECT t.name, u.email, COALESCE(t.name, u.email), COALESCE(t.name, u.email, '') FROM tenants t FULL JOIN users u ON u.tenant_id = t.id", []bool{true, true, true, false}},
		{"inner", "SELECT t.name, u.email FROM tenants t JOIN users u ON u.tenant_id = t.id", []bool{false, false}},
		{"self_join", "SELECT a.email, b.email, COALESCE(b.email, a.email) FROM users a LEFT JOIN users b ON a.tenant_id = b.tenant_id", []bool{false, true, false}},
		{"star", "SELECT u.* FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id", []bool{true, true, true}},
		{"derived", "SELECT COALESCE(v.email, '') FROM (SELECT u.email FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id) v", []bool{false}},
		{"cte", "WITH v AS (SELECT COALESCE(u.email, '') AS email FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id) SELECT email FROM v", []bool{false}},
		{"case_fallback", "SELECT COALESCE(nickname, CASE WHEN tenant_id > 0 THEN '' ELSE 'x' END) FROM users", []bool{false}},
		{"case_no_else", "SELECT COALESCE(nickname, CASE WHEN tenant_id > 0 THEN '' END) FROM users", []bool{true}},
		{"all_null", "SELECT COALESCE(NULL, NULL)", []bool{true}},
		{"parameter", "SELECT COALESCE(@p::text, '')", []bool{false}},
		{"non_null_first", "SELECT COALESCE('', @p::text)", []bool{false}},
		{"comparison", "SELECT t.id = 1, u.tenant_id = 1, u.email IS NULL, COALESCE(u.tenant_id = 1, false) FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id", []bool{false, true, false, false}},
		{"exists", "SELECT EXISTS (SELECT 1 FROM users), NOT EXISTS (SELECT 1 FROM users)", []bool{false, false}},
		{"scalar_count", "SELECT (SELECT count(*) FROM users)::int AS total", []bool{false}},
		{"scalar_count_correlated", "SELECT (SELECT count(*) FROM users u WHERE u.tenant_id = t.id) FROM tenants t", []bool{false}},
		{"scalar_count_having", "SELECT (SELECT count(*) FROM users HAVING count(*) > 0)", []bool{true}},
		{"scalar_count_grouped", "SELECT (SELECT count(*) FROM users GROUP BY tenant_id LIMIT 1)", []bool{true}},
		{"scalar_count_offset", "SELECT (SELECT count(*) FROM users OFFSET 1)", []bool{true}},
		{"scalar_count_filter", "SELECT (SELECT count(*) FILTER (WHERE email <> '') FROM users)", []bool{false}},
		{"scalar_count_empty", "SELECT (SELECT count(*) FROM users WHERE false)", []bool{false}},
		{"scalar_count_limit_zero", "SELECT (SELECT count(*) FROM users LIMIT 0)", []bool{true}},
		{"scalar_window_count", "SELECT (SELECT count(*) OVER () FROM users LIMIT 1)", []bool{true}},
		{"scalar_set_returning", "SELECT (SELECT generate_series(1, 0))", []bool{true}},
		{"scalar_false", "SELECT (SELECT 1 WHERE false)", []bool{true}},
		{"scalar_outer_aggregate", "SELECT (SELECT count(t.id) FROM users) FROM tenants t", []bool{true}},
		{"scalar_column", "SELECT (SELECT email FROM users LIMIT 1)", []bool{true}},
		{"scalar_literal", "SELECT (SELECT 1)", []bool{false}},
		{"union_nullable", "SELECT v.email FROM (SELECT email FROM users UNION ALL SELECT NULL) v", []bool{true}},
		{"union_nonnull", "SELECT v.email FROM (SELECT email FROM users UNION ALL SELECT 'x') v", []bool{false}},
		{"intersect_nonnull", "SELECT v.email FROM (SELECT nickname AS email FROM users INTERSECT SELECT email FROM users) v", []bool{false}},
		{"intersect_nullable", "SELECT v.email FROM (SELECT nickname AS email FROM users INTERSECT SELECT NULL) v", []bool{true}},
		{"except_nonnull", "SELECT v.email FROM (SELECT email FROM users EXCEPT SELECT NULL) v", []bool{false}},
		{"except_nullable", "SELECT v.email FROM (SELECT nickname AS email FROM users EXCEPT SELECT email FROM users) v", []bool{true}},
		{"values_nonnull", "SELECT v.email FROM (VALUES ('a'), ('b')) v(email)", []bool{false}},
		{"values_nullable", "SELECT v.email FROM (VALUES ('a'), (NULL)) v(email)", []bool{true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			qs, err := ParseQueries("-- name: Q :many\n"+tc.sql, "q.sql")
			if err != nil {
				t.Fatal(err)
			}
			var d catalog.Diagnostics
			Infer(qs[0], cat, &d)
			cols := qs[0].GetColumns()
			if len(cols) != len(tc.nullable) {
				t.Fatalf("columns=%v, want %d", cols, len(tc.nullable))
			}
			for n, want := range tc.nullable {
				if got := cols[n].GetNullable(); got != want {
					t.Errorf("column %d nullable=%v, want %v", n, got, want)
				}
			}
		})
	}
	// The same catalog is reused above: outer joins must not mutate its columns.
	for _, tbl := range cat.GetSchemas()[0].GetTables() {
		for _, col := range tbl.GetColumns() {
			if col.GetName() != "nickname" && col.GetNullable() {
				t.Errorf("catalog column mutated: %s", col.GetId())
			}
		}
	}
}
