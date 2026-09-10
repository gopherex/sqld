package introspect

import (
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/diff"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

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
