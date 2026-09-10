package main

import (
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/internal/query"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

func TestGenerateCoalesceAfterLeftJoin(t *testing.T) {
	stmts, err := parse.Statements(`CREATE TABLE tenants(id bigint PRIMARY KEY);
		CREATE TABLE users(tenant_id bigint NOT NULL, email text NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	qs, err := query.ParseQueries(`-- name: Joined :many
		SELECT u.email AS raw_email, COALESCE(u.email, '') AS email
		FROM tenants t LEFT JOIN users u ON u.tenant_id = t.id;`, "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	var d catalog.Diagnostics
	query.Infer(qs[0], cat, &d)
	if len(d.Items) != 0 {
		t.Fatalf("diagnostics: %v", d.Items)
	}
	resp, err := Generate(&pluginv1.GenerateRequest{Catalog: cat, Queries: qs})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range resp.GetFiles() {
		if f.GetPath() == "queries.go" {
			want := "type JoinedRow struct {\n\tRawEmail *string\n\tEmail    string\n}"
			if !strings.Contains(string(f.GetContents()), want) {
				t.Fatalf("incorrect Go nullability:\n%s", f.GetContents())
			}
			return
		}
	}
	t.Fatal("queries.go missing")
}
