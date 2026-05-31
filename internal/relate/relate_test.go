package relate

import (
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

func deriveSQL(t *testing.T, sql string) []*irv1.Relationship {
	t.Helper()
	stmts, err := parse.Statements(sql)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	return Derive(cat)
}

func TestDeriveManyToOne(t *testing.T) {
	rels := deriveSQL(t, `
		CREATE TABLE users(id bigint primary key);
		CREATE TABLE orders(id bigint primary key, user_id bigint not null references users(id));
	`)
	var m2o, o2m *irv1.Relationship
	for _, r := range rels {
		if r.GetFromTable().GetId() == "public.orders" && r.GetToTable().GetId() == "public.users" {
			m2o = r
		}
		if r.GetFromTable().GetId() == "public.users" && r.GetToTable().GetId() == "public.orders" {
			o2m = r
		}
	}
	if m2o == nil || m2o.GetKind() != irv1.RelationshipKind_RELATIONSHIP_KIND_MANY_TO_ONE {
		t.Fatalf("m2o=%+v", m2o)
	}
	if o2m == nil || o2m.GetKind() != irv1.RelationshipKind_RELATIONSHIP_KIND_ONE_TO_MANY {
		t.Fatalf("o2m=%+v", o2m)
	}
}

func TestDeriveOptional(t *testing.T) {
	rels := deriveSQL(t, `
		CREATE TABLE users(id bigint primary key);
		CREATE TABLE orders(id bigint primary key, user_id bigint references users(id));
	`)
	var m2o *irv1.Relationship
	for _, r := range rels {
		if r.GetFromTable().GetId() == "public.orders" && r.GetToTable().GetId() == "public.users" {
			m2o = r
		}
	}
	if m2o == nil {
		t.Fatal("no MANY_TO_ONE relationship found")
	}
	if !m2o.GetOptional() {
		t.Fatalf("expected optional=true for nullable FK column, got false")
	}
}

func TestDeriveNotOptional(t *testing.T) {
	rels := deriveSQL(t, `
		CREATE TABLE users(id bigint primary key);
		CREATE TABLE orders(id bigint primary key, user_id bigint not null references users(id));
	`)
	var m2o *irv1.Relationship
	for _, r := range rels {
		if r.GetFromTable().GetId() == "public.orders" && r.GetToTable().GetId() == "public.users" {
			m2o = r
		}
	}
	if m2o == nil {
		t.Fatal("no MANY_TO_ONE relationship found")
	}
	if m2o.GetOptional() {
		t.Fatalf("expected optional=false for NOT NULL FK column, got true")
	}
}

func TestDeriveManyToMany(t *testing.T) {
	rels := deriveSQL(t, `
		CREATE TABLE users(id bigint primary key);
		CREATE TABLE roles(id bigint primary key);
		CREATE TABLE user_roles(
			user_id bigint not null references users(id),
			role_id bigint not null references roles(id),
			primary key (user_id, role_id)
		);
	`)
	var m2m *irv1.Relationship
	for _, r := range rels {
		if r.GetKind() == irv1.RelationshipKind_RELATIONSHIP_KIND_MANY_TO_MANY {
			m2m = r
		}
	}
	if m2m == nil {
		t.Fatalf("no MANY_TO_MANY; got %d rels", len(rels))
	}
	if m2m.GetJoinTable().GetTable().GetId() != "public.user_roles" {
		t.Fatalf("join table=%+v", m2m.GetJoinTable())
	}
}

func TestDeriveManyToManyNoPairEmitted(t *testing.T) {
	rels := deriveSQL(t, `
		CREATE TABLE users(id bigint primary key);
		CREATE TABLE roles(id bigint primary key);
		CREATE TABLE user_roles(
			user_id bigint not null references users(id),
			role_id bigint not null references roles(id),
			primary key (user_id, role_id)
		);
	`)
	// The join table's own FKs must NOT produce plain MANY_TO_ONE/ONE_TO_MANY pairs.
	for _, r := range rels {
		from := r.GetFromTable().GetId()
		to := r.GetToTable().GetId()
		if from == "public.user_roles" || to == "public.user_roles" {
			if r.GetKind() != irv1.RelationshipKind_RELATIONSHIP_KIND_MANY_TO_MANY {
				t.Fatalf("unexpected non-M2M rel involving user_roles: %+v", r)
			}
		}
	}
}

func TestDeriveOneToOne(t *testing.T) {
	rels := deriveSQL(t, `
		CREATE TABLE users(id bigint primary key);
		CREATE TABLE profiles(id bigint primary key references users(id));
	`)
	var o2o *irv1.Relationship
	for _, r := range rels {
		if r.GetFromTable().GetId() == "public.profiles" && r.GetToTable().GetId() == "public.users" {
			o2o = r
		}
	}
	if o2o == nil {
		t.Fatal("no rel profiles->users found")
	}
	// id is also the PK of profiles, so this FK is unique => ONE_TO_ONE
	if o2o.GetKind() != irv1.RelationshipKind_RELATIONSHIP_KIND_ONE_TO_ONE {
		t.Fatalf("expected ONE_TO_ONE, got %v", o2o.GetKind())
	}
}

func TestDeriveViaConstraint(t *testing.T) {
	rels := deriveSQL(t, `
		CREATE TABLE users(id bigint primary key);
		CREATE TABLE orders(id bigint primary key, user_id bigint not null references users(id));
	`)
	var m2o *irv1.Relationship
	for _, r := range rels {
		if r.GetFromTable().GetId() == "public.orders" && r.GetToTable().GetId() == "public.users" {
			m2o = r
		}
	}
	if m2o == nil {
		t.Fatal("no MANY_TO_ONE found")
	}
	if m2o.GetViaConstraint() == nil || m2o.GetViaConstraint().GetId() == "" {
		t.Fatalf("expected ViaConstraint to be set, got %+v", m2o.GetViaConstraint())
	}
}

func TestDeriveSuggestedName(t *testing.T) {
	rels := deriveSQL(t, `
		CREATE TABLE users(id bigint primary key);
		CREATE TABLE orders(id bigint primary key, user_id bigint not null references users(id));
	`)
	var m2o, o2m *irv1.Relationship
	for _, r := range rels {
		if r.GetFromTable().GetId() == "public.orders" && r.GetToTable().GetId() == "public.users" {
			m2o = r
		}
		if r.GetFromTable().GetId() == "public.users" && r.GetToTable().GetId() == "public.orders" {
			o2m = r
		}
	}
	if m2o == nil || o2m == nil {
		t.Fatal("missing relationships")
	}
	// M2O: suggested name = referenced table name
	if m2o.GetSuggestedName() != "users" {
		t.Errorf("m2o suggested name=%q, want 'users'", m2o.GetSuggestedName())
	}
	// O2M: suggested name = referencing table name
	if o2m.GetSuggestedName() != "orders" {
		t.Errorf("o2m suggested name=%q, want 'orders'", o2m.GetSuggestedName())
	}
}

func TestDeriveNilCatalog(t *testing.T) {
	// Should not panic
	rels := Derive(nil)
	if rels == nil {
		t.Fatal("expected non-nil slice")
	}
	if len(rels) != 0 {
		t.Fatalf("expected empty slice, got %d", len(rels))
	}
}

func TestDeriveMissingRefTable(t *testing.T) {
	// Build a catalog with a FK to a table that doesn't exist — should not panic
	// We do this by direct construction rather than SQL (which would fail to parse the constraint)
	cat := &irv1.Catalog{
		DefaultSchema: "public",
		Schemas: []*irv1.Schema{
			{
				Id:   "public",
				Name: "public",
				Tables: []*irv1.Table{
					{
						Id:   "public.orders",
						Name: &irv1.QualifiedName{Schema: "public", Name: "orders"},
						Constraints: []*irv1.Constraint{
							{
								Id:   "public.orders.fk_user",
								Name: "fk_user",
								Type: irv1.ConstraintType_CONSTRAINT_TYPE_FOREIGN_KEY,
								Body: &irv1.Constraint_ForeignKey{
									ForeignKey: &irv1.ForeignKey{
										Columns: []string{"user_id"},
										ReferencedTable: &irv1.ObjectRef{
											Id:   "public.missing",
											Kind: irv1.ObjectKind_OBJECT_KIND_TABLE,
										},
										ReferencedColumns: []string{"id"},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	// Should not panic and return empty
	rels := Derive(cat)
	if len(rels) != 0 {
		t.Fatalf("expected 0 rels for missing ref table, got %d", len(rels))
	}
}
