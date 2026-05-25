package sqld_test

import (
	"testing"

	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
	"github.com/yaroher/sqld/pkg/sqld"
)

func inlineConfig(schema string) *configv1.Config {
	return &configv1.Config{
		Engine: 0,
		Sql: []*configv1.SqlSource{{
			Source: &configv1.SqlSource_Inline{Inline: schema},
			Kind:   configv1.SqlKind_SQL_KIND_SCHEMA,
		}},
	}
}

func TestCollect(t *testing.T) {
	cfg := inlineConfig(`
		CREATE TABLE users(id bigint primary key, email text not null);
		CREATE TABLE orders(id bigint primary key, user_id bigint not null references users(id));
	`)
	cat, err := sqld.Collect(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(cat.GetSchemas()) != 1 || len(cat.GetSchemas()[0].GetTables()) != 2 {
		t.Fatalf("schemas/tables wrong: %+v", cat.GetSchemas())
	}
	if len(cat.GetRelationships()) == 0 {
		t.Fatal("expected derived relationships")
	}
}

func TestCollectAll(t *testing.T) {
	cfg := &configv1.Config{
		Sql: []*configv1.SqlSource{
			{
				Source: &configv1.SqlSource_Inline{Inline: "CREATE TABLE users(id bigint primary key, email text not null);"},
				Kind:   configv1.SqlKind_SQL_KIND_SCHEMA,
			},
			{
				Source: &configv1.SqlSource_Inline{Inline: "-- name: GetUser :one\nSELECT id, email FROM users WHERE id = $1;\n"},
				Kind:   configv1.SqlKind_SQL_KIND_QUERY,
			},
		},
	}
	r, err := sqld.CollectAll(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Queries) != 1 || r.Queries[0].GetName() != "GetUser" {
		t.Fatalf("queries=%+v", r.Queries)
	}
	if len(r.Queries[0].GetColumns()) != 2 {
		t.Fatalf("query columns=%d", len(r.Queries[0].GetColumns()))
	}
}
