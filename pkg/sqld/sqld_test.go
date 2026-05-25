package sqld_test

import (
	"testing"

	"github.com/yaroher/sqld/pkg/config"
	"github.com/yaroher/sqld/pkg/sqld"
)

func inlineConfig(schema string) *config.Config {
	return &config.Config{
		Engine: config.EnginePostgreSQL,
		SQL: []config.SQLSource{{
			Inline: schema,
			Kind:   config.SQLSchema,
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
	cfg := &config.Config{
		Engine: config.EnginePostgreSQL,
		SQL: []config.SQLSource{
			{
				Inline: "CREATE TABLE users(id bigint primary key, email text not null);",
				Kind:   config.SQLSchema,
			},
			{
				Inline: "-- name: GetUser :one\nSELECT id, email FROM users WHERE id = $1;\n",
				Kind:   config.SQLQuery,
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
