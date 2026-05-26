package main

import (
	"context"
	"testing"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// testCatalog builds a one-schema catalog: an enum account_status and a table
// accounts(id int8 pk, email text not null, status account_status not null,
// bio text null). Enum columns are emitted as kind=SCALAR with PgName set to
// the enum name, matching how sqld's parser currently produces them.
func testCatalog() *irv1.Catalog {
	col := func(name, pg string, nullable bool) *irv1.Column {
		return &irv1.Column{
			Name:     name,
			Type:     &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR, PgName: pg},
			Nullable: nullable,
		}
	}
	return &irv1.Catalog{
		DefaultSchema: "public",
		Schemas: []*irv1.Schema{{
			Name: "public",
			Enums: []*irv1.EnumType{{
				Name:   &irv1.QualifiedName{Schema: "public", Name: "account_status"},
				Labels: []string{"active", "suspended", "closed"},
			}},
			Tables: []*irv1.Table{{
				Name: &irv1.QualifiedName{Schema: "public", Name: "accounts"},
				Columns: []*irv1.Column{
					col("id", "int8", false),
					col("email", "text", false),
					col("status", "account_status", false),
					col("bio", "text", true),
				},
				Constraints: []*irv1.Constraint{{
					Type: irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY,
					Body: &irv1.Constraint_PrimaryKey{PrimaryKey: &irv1.PrimaryKey{Columns: []string{"id"}}},
				}},
				Indexes: []*irv1.Index{{
					Name:   "accounts_email_key",
					Unique: true,
					Elements: []*irv1.IndexElement{
						{Target: &irv1.IndexElement_Column{Column: "email"}},
					},
				}},
			}},
		}},
	}
}

func TestAssembleColumnTypes(t *testing.T) {
	cat := testCatalog()
	d := newDriver(cat, "example.com/app/db", nil)
	info, err := d.Assemble(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(info.Tables) != 1 {
		t.Fatalf("tables = %d; want 1", len(info.Tables))
	}
	tbl := info.Tables[0]

	cols := map[string]string{}
	null := map[string]bool{}
	for _, c := range tbl.Columns {
		cols[c.Name] = c.Type
		null[c.Name] = c.Nullable
	}
	// bob owns nullability, so Type is always the non-null base type.
	if cols["id"] != "int64" {
		t.Errorf("id type = %q; want int64", cols["id"])
	}
	if cols["email"] != "string" {
		t.Errorf("email type = %q; want string", cols["email"])
	}
	// the enum resolves to the shared, package-qualified type.
	if cols["status"] != "db.AccountStatus" {
		t.Errorf("status type = %q; want db.AccountStatus", cols["status"])
	}
	if cols["bio"] != "string" || !null["bio"] {
		t.Errorf("bio = %q nullable=%v; want string nullable=true", cols["bio"], null["bio"])
	}

	// the default schema is emitted as empty (not prefixed).
	if tbl.Schema != "" {
		t.Errorf("schema = %q; want empty", tbl.Schema)
	}
	// PK constraint mapped, with a synthesized name.
	if tbl.Constraints.Primary == nil || tbl.Constraints.Primary.Name != "accounts_pkey" {
		t.Errorf("primary key = %+v; want accounts_pkey", tbl.Constraints.Primary)
	}
	// Enums left empty so bob emits no competing enum package.
	if len(info.Enums) != 0 {
		t.Errorf("Enums = %d; want 0", len(info.Enums))
	}
	// the shared types package import is registered for the enum type.
	if !d.Types().Contains("db.AccountStatus") {
		t.Errorf("db.AccountStatus not registered in Types")
	}

	// the unique index is mapped (skipping the PK index).
	if len(tbl.Indexes) != 1 {
		t.Fatalf("indexes = %d; want 1", len(tbl.Indexes))
	}
	if idx := tbl.Indexes[0]; idx.Name != "accounts_email_key" || !idx.Unique ||
		len(idx.Columns) != 1 || idx.Columns[0].Name != "email" {
		t.Errorf("index = %+v; want unique accounts_email_key(email)", idx)
	}
}

// TestTableKeyQualifiesNonDefaultSchema ensures non-default-schema tables get a
// schema-qualified bob key, so same-bare-name tables across schemas (and the FKs
// that reference them) don't collide in bob's table map.
func TestTableKeyQualifiesNonDefaultSchema(t *testing.T) {
	d := &sqldDriver{defaultSchema: "public"}
	if got := d.tableKey("public", "users"); got != "users" {
		t.Errorf("default-schema key = %q; want users", got)
	}
	if got := d.tableKey("audit", "log"); got != "audit.log" {
		t.Errorf("non-default-schema key = %q; want audit.log", got)
	}
}
