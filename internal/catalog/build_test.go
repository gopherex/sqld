package catalog

import (
	"testing"

	"github.com/yaroher/sqld/internal/parse"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

const shopSQL = `
CREATE TYPE order_status AS ENUM ('pending','paid');
CREATE TABLE users (
  id bigserial primary key,
  email text not null unique
);
CREATE TABLE orders (
  id bigserial primary key,
  user_id bigint not null references users(id) on delete cascade,
  status order_status not null
);
CREATE VIEW active_users AS SELECT id, email FROM users;
`

func buildShop(t *testing.T) (*irv1.Catalog, *Diagnostics) {
	t.Helper()
	stmts, err := parse.Statements(shopSQL)
	if err != nil {
		t.Fatal(err)
	}
	return Build(stmts)
}

func TestBuildSchemasAndTables(t *testing.T) {
	cat, diag := buildShop(t)
	for _, d := range diag.Items {
		t.Logf("diagnostic [%s]: %s", d.Severity, d.Message)
	}
	if cat.GetDefaultSchema() != "public" {
		t.Fatalf("default schema=%q", cat.GetDefaultSchema())
	}
	if len(cat.GetSchemas()) != 1 {
		t.Fatalf("schemas=%d", len(cat.GetSchemas()))
	}
	pub := cat.GetSchemas()[0]
	if len(pub.GetTables()) != 2 {
		t.Fatalf("tables=%d", len(pub.GetTables()))
	}
	if len(pub.GetEnums()) != 1 {
		t.Fatalf("enums=%d", len(pub.GetEnums()))
	}
	if len(pub.GetViews()) != 1 {
		t.Fatalf("views=%d", len(pub.GetViews()))
	}
}

func TestBuildIdsAndFK(t *testing.T) {
	cat, diag := buildShop(t)
	for _, d := range diag.Items {
		t.Logf("diagnostic [%s]: %s", d.Severity, d.Message)
	}
	pub := cat.GetSchemas()[0]
	var orders *irv1.Table
	for _, tb := range pub.GetTables() {
		if tb.GetName().GetName() == "orders" {
			orders = tb
		}
	}
	if orders == nil || orders.GetId() != "public.orders" {
		t.Fatalf("orders id=%q", orders.GetId())
	}
	var fk *irv1.ForeignKey
	for _, c := range orders.GetConstraints() {
		if c.GetForeignKey() != nil {
			fk = c.GetForeignKey()
		}
	}
	if fk == nil {
		t.Fatal("no FK")
	}
	if fk.GetReferencedTable().GetId() != "public.users" {
		t.Fatalf("fk ref id=%q", fk.GetReferencedTable().GetId())
	}
}

func TestBuildColumnIds(t *testing.T) {
	cat, _ := buildShop(t)
	pub := cat.GetSchemas()[0]
	var users *irv1.Table
	for _, tb := range pub.GetTables() {
		if tb.GetName().GetName() == "users" {
			users = tb
		}
	}
	if users == nil {
		t.Fatal("users table not found")
	}
	for _, col := range users.GetColumns() {
		if col.GetId() == "" {
			t.Fatalf("column %q has empty id", col.GetName())
		}
		// id should be "public.users.<colname>"
		expected := "public.users." + col.GetName()
		if col.GetId() != expected {
			t.Fatalf("column %q id=%q want %q", col.GetName(), col.GetId(), expected)
		}
	}
}

func TestBuildNoDiagnosticErrors(t *testing.T) {
	_, diag := buildShop(t)
	for _, d := range diag.Items {
		if d.Severity == "error" {
			t.Errorf("unexpected error diagnostic: %s", d.Message)
		}
	}
}
