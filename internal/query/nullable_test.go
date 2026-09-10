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
