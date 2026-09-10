package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gopherex/sqld/pkg/devdb"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	"github.com/jackc/pgx/v5"
)

func TestMigrateGenerateIndexOrdering(t *testing.T) {
	ctx := context.Background()
	dev, err := devdb.Start(ctx)
	if err != nil {
		t.Skipf("docker unavailable: %v", err)
	}
	defer dev.Close()

	root := t.TempDir()
	var initOut, initErr bytes.Buffer
	if code := run([]string{"init", root}, &initOut, &initErr); code != 0 {
		t.Fatalf("init: exit %d\nstdout=%s\nstderr=%s", code, &initOut, &initErr)
	}
	t.Chdir(root)
	schemaFile := filepath.Join(root, "schema.sql")
	schema := `CREATE TABLE audit_log (tenant_id bigint NOT NULL, id bigint PRIMARY KEY);
CREATE TABLE tenant_invites (tenant_id bigint NOT NULL, email text NOT NULL, status text NOT NULL);
CREATE INDEX audit_log_tenant_idx ON audit_log (tenant_id, id DESC);
CREATE UNIQUE INDEX tenant_invites_pending_idx ON tenant_invites (tenant_id, lower(email)) WHERE status = 'pending';
CREATE INDEX audit_log_nulls ON audit_log (tenant_id DESC NULLS LAST, id ASC NULLS FIRST);
`
	// Assert the introspection boundary itself, independently of SQL rendering
	// and comparisons between two catalogs produced by the same code.
	desired, err := buildCatalog(ctx, dev.URL(), schema, []string{"public"})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgx.Connect(ctx, dev.URL())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	seen := 0
	for _, table := range desired.GetSchemas()[0].GetTables() {
		for _, index := range table.GetIndexes() {
			if index.GetName() != "audit_log_tenant_idx" && index.GetName() != "tenant_invites_pending_idx" {
				continue
			}
			seen++
			var definition string
			if err := conn.QueryRow(ctx, "SELECT pg_get_indexdef($1::regclass)", "public."+index.GetName()).Scan(&definition); err != nil {
				t.Fatal(err)
			}
			t.Logf("PostgreSQL: %s\nIR: %s", definition, index)
			elements := index.GetElements()
			if len(elements) != 2 || elements[0].GetColumn() != "tenant_id" {
				t.Fatalf("incorrect index keys: %s", index)
			}
			switch index.GetName() {
			case "audit_log_tenant_idx":
				if !strings.Contains(definition, "id DESC") || elements[1].GetColumn() != "id" || elements[1].GetOrder() != irv1.SortOrder_SORT_ORDER_DESC {
					t.Fatalf("DESC lost: PostgreSQL=%s, IR=%s", definition, index)
				}
			case "tenant_invites_pending_idx":
				call := elements[1].GetExpr().GetFunctionCall()
				if !strings.Contains(definition, "lower(email)") || len(call.GetArguments()) != 1 || call.GetArguments()[0].GetColumnRef().GetColumn() != "email" || !index.GetUnique() || index.GetPredicate() == nil {
					t.Fatalf("expression or partial unique metadata lost: PostgreSQL=%s, IR=%s", definition, index)
				}
			}
		}
	}
	if seen != 2 {
		t.Fatalf("introspected %d reported indexes, want 2", seen)
	}
	writeSchema := func(sql string) {
		t.Helper()
		if err := os.WriteFile(schemaFile, []byte(sql), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeSchema(schema)
	cfgPath := filepath.Join(root, "sqld.yaml")
	generate := func(name string) string {
		t.Helper()
		var out, errb bytes.Buffer
		if code := run([]string{"migrate", "generate", name, "-c", cfgPath, "--dev-url", dev.URL()}, &out, &errb); code != 0 {
			t.Fatalf("generate: exit %d\nstdout=%s\nstderr=%s", code, &out, &errb)
		}
		return strings.TrimSpace(out.String())
	}
	readMigration := func(path string) string {
		t.Helper()
		body, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	initial := readMigration(generate("create_indexes"))
	for _, want := range []string{
		`CREATE INDEX "audit_log_tenant_idx" ON "public"."audit_log" ("tenant_id", "id" DESC);`,
		`CREATE UNIQUE INDEX "tenant_invites_pending_idx" ON "public"."tenant_invites" ("tenant_id", (lower("email"))) WHERE (status = 'pending'::text);`,
		`CREATE INDEX "audit_log_nulls" ON "public"."audit_log" ("tenant_id" DESC NULLS LAST, "id" NULLS FIRST);`,
	} {
		if !strings.Contains(initial, want) {
			t.Errorf("missing %s\n--- migration ---\n%s", want, initial)
		}
	}
	if got := generate("unchanged"); got != "no changes" {
		t.Fatalf("unchanged schema: %s", got)
	}

	writeSchema(strings.Replace(schema, "(tenant_id, id DESC)", "(tenant_id, id ASC)", 1))
	changed := readMigration(generate("change_direction"))
	up, down, ok := strings.Cut(changed, "-- sqld:down")
	if !ok {
		t.Fatalf("missing down migration: %s", changed)
	}
	if !strings.Contains(up, `DROP INDEX "public"."audit_log_tenant_idx";`) ||
		!strings.Contains(up, `("tenant_id", "id");`) || strings.Contains(up, "DESC") {
		t.Errorf("direction change not rendered correctly: %s", up)
	}
	if !strings.Contains(down, `("tenant_id", "id" DESC);`) {
		t.Errorf("down migration does not restore DESC: %s", down)
	}
	if got := generate("unchanged_again"); got != "no changes" {
		t.Fatalf("changed schema: %s", got)
	}
}
