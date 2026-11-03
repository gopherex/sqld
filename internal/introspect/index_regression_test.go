package introspect

import (
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/diff"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

// Operator classes survive both paths. The introspector runs with an empty
// search_path, so pg_get_indexdef schema-qualifies extension opclasses
// (public.gin_trgm_ops); parsing and introspection must still agree and the
// generated DDL must run.
func TestIndexOpclassRoundTrip(t *testing.T) {
	const ext = "CREATE EXTENSION IF NOT EXISTS pg_trgm;\n"
	for _, tc := range []struct {
		name, ddl string
	}{
		{name: "bare opclass", ddl: `CREATE TABLE docs(body text, meta jsonb, code text);
			CREATE INDEX docs_body ON docs USING gin (body gin_trgm_ops);
			CREATE INDEX docs_meta ON docs USING gin (meta jsonb_path_ops);
			CREATE INDEX docs_code ON docs (code text_pattern_ops DESC);`},
		{name: "schema-qualified opclass", ddl: `CREATE TABLE docs(body text, meta jsonb, code text);
			CREATE INDEX docs_body ON docs USING gin (body public.gin_trgm_ops);
			CREATE INDEX docs_meta ON docs USING gin (meta pg_catalog.jsonb_path_ops);
			CREATE INDEX docs_code ON docs (code pg_catalog.text_pattern_ops DESC);`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			want := map[string]string{"docs_body": "gin_trgm_ops", "docs_meta": "jsonb_path_ops", "docs_code": "text_pattern_ops"}
			parsed := parseDDL(t, tc.ddl)
			introspected := introspectDDL(t, ext+tc.ddl, []string{"public"})
			for path, cat := range map[string]*irv1.Catalog{"parsed": parsed, "introspected": introspected} {
				for _, idx := range tableIn(findSchemaCat(cat, "public"), "docs").GetIndexes() {
					if got := idx.GetElements()[0].GetOpclass(); got != want[idx.GetName()] {
						t.Errorf("%s %s opclass=%q, want %q", path, idx.GetName(), got, want[idx.GetName()])
					}
				}
			}
			drift, err := diff.Diff(introspected, parsed)
			if err != nil {
				t.Fatal(err)
			}
			if up := drift.UpSQL(); strings.Contains(up, "INDEX") {
				t.Errorf("parsed and introspected indexes differ:\n%s", up)
			}
			plan, err := diff.Diff(&irv1.Catalog{}, parsed)
			if err != nil {
				t.Fatal(err)
			}
			up := plan.UpSQL()
			if !strings.Contains(up, `USING gin ("body" gin_trgm_ops)`) {
				t.Errorf("opclass not rendered:\n%s", up)
			}
			applyOnFreshPG(t, ext+up)
		})
	}
}

func TestIntrospectMixedExpressionIndex(t *testing.T) {
	cat := introspectDDL(t, `CREATE TABLE users(tenant_id bigint, email text, payload text);
		CREATE INDEX users_email ON users (tenant_id DESC NULLS LAST, lower(email) DESC, (length(email) + 1) ASC NULLS FIRST) INCLUDE (payload)
		WHERE email IS NOT NULL;`, []string{"public"})
	tbl := tableIn(findSchemaCat(cat, "public"), "users")
	if tbl == nil || len(tbl.GetIndexes()) != 1 {
		t.Fatalf("missing index: %v", tbl)
	}
	idx := tbl.GetIndexes()[0]
	if len(idx.GetElements()) != 3 {
		t.Fatalf("elements=%v", idx.GetElements())
	}
	if idx.GetElements()[0].GetColumn() != "tenant_id" {
		t.Errorf("first element=%v", idx.GetElements()[0])
	}
	for n, want := range []irv1.SortOrder{irv1.SortOrder_SORT_ORDER_DESC, irv1.SortOrder_SORT_ORDER_DESC, irv1.SortOrder_SORT_ORDER_UNSPECIFIED} {
		if got := idx.GetElements()[n].GetOrder(); got != want {
			t.Errorf("element %d order=%v, want %v", n, got, want)
		}
	}
	if idx.GetElements()[0].GetNulls() != irv1.NullsOrder_NULLS_ORDER_LAST || idx.GetElements()[2].GetNulls() != irv1.NullsOrder_NULLS_ORDER_FIRST {
		t.Errorf("NULLS ordering lost: %v", idx.GetElements())
	}
	plan, err := diff.Diff(&irv1.Catalog{}, cat)
	if err != nil {
		t.Fatal(err)
	}
	up := plan.UpSQL()
	if strings.Contains(up, "(tenant_id,") {
		t.Errorf("entire key list used as one expression:\n%s", up)
	}
	if !strings.Contains(up, `"tenant_id" DESC NULLS LAST`) || !strings.Contains(up, "DESC") || !strings.Contains(up, `INCLUDE ("payload")`) {
		t.Errorf("index metadata missing:\n%s", up)
	}
	applyOnFreshPG(t, up)
}
