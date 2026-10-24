package main

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/internal/query"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

//go:embed testdata/sqlc_schema.sql
var sqlcSchema string

//go:embed testdata/sqlc_queries.sql
var sqlcQueries string

//go:embed testdata/sqlc_runtime_test.go
var sqlcRuntimeTest []byte

func TestGeneratedSQLCParameters(t *testing.T) {
	stmts, err := parse.Statements(sqlcSchema)
	if err != nil {
		t.Fatal(err)
	}
	cat, d := catalog.Build(stmts)
	qs, err := query.ParseQueries(sqlcQueries, "sqlc_queries.sql")
	if err != nil || len(qs) != 11 {
		t.Fatalf("queries=%d err=%v", len(qs), err)
	}
	for _, q := range qs {
		query.Infer(q, cat, d)
	}
	if len(d.Items) != 0 {
		t.Fatal(d.Items)
	}
	req := &pluginv1.GenerateRequest{Catalog: cat, Queries: qs, Options: []byte(`{"nullMode":"pointer"}`)}
	wire, err := proto.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}
	req = &pluginv1.GenerateRequest{}
	if err := proto.Unmarshal(wire, req); err != nil {
		t.Fatal(err)
	}
	resp, err := Generate(req)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Diagnostics) != 0 {
		t.Fatal(resp.Diagnostics)
	}
	var source []byte
	for _, f := range resp.Files {
		if f.Path == "queries.go" {
			source = f.Contents
		}
	}
	if strings.Contains(string(source), "sqlc.") || strings.Contains(string(source), "sqld.") {
		t.Fatalf("unexpanded macro in generated SQL:\n%s", source)
	}
	got := generatedGoTypes(t, source)
	for field, want := range map[string]string{
		"InsertHeadsParams.SummaryTypes": "[]*int64", "InsertHeadsParams.Kinds": "[]string",
		"FindKindsParams.Kinds": "[]*string", "OptionalKindsParams.Kinds": "[]string",
		"UpdateNameParams.Name": "*string", "UpdateNameParams.ID": "int64",
		"ClearBioParams.Bio": "*string", "ClearBioParams.ID": "int64",
		"CreateProfileParams.ID": "int64", "CreateProfileParams.Name": "*string", "CreateProfileParams.Bio": "*string",
	} {
		if got[field] != want {
			t.Errorf("%s=%s want %s", field, got[field], want)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	testGeneratedPostgres(t, resp, map[string][]byte{"sqlc_runtime_test.go": sqlcRuntimeTest, "sqlc_schema.sql": []byte(sqlcSchema)})
}

func TestGeneratedNullElementsModes(t *testing.T) {
	previousMode := goNullMode
	t.Cleanup(func() { goNullMode = previousMode })
	for _, mode := range []string{"pointer", "opt"} {
		t.Run(mode, func(t *testing.T) {
			qs, err := query.ParseQueries(`-- name: ArrayValues :exec
SELECT sqld.nullelems(texts)::text[], sqld.nullelems(times)::timestamptz[],
       sqld.nullelems(blobs)::bytea[], sqld.nullelems(documents)::jsonb[];
-- name: FilterTimes :many
SELECT 1 WHERE now() = ANY(sqld.nullelems(times)::timestamptz[]);
`, "arrays.sql")
			if err != nil {
				t.Fatal(err)
			}
			d := &catalog.Diagnostics{}
			for _, q := range qs {
				query.Infer(q, nil, d)
			}
			if len(d.Items) != 0 {
				t.Fatal(d.Items)
			}
			resp, err := Generate(&pluginv1.GenerateRequest{Queries: qs, Options: []byte(`{"nullMode":"` + mode + `"}`)})
			if err != nil {
				t.Fatal(err)
			}
			if len(resp.Diagnostics) != 0 {
				t.Fatal(resp.Diagnostics)
			}
			for _, f := range resp.Files {
				if f.Path != "queries.go" {
					continue
				}
				got := generatedGoTypes(t, f.Contents)
				for field, want := range map[string]string{
					"ArrayValuesParams.Texts": "[]*string", "ArrayValuesParams.Times": "[]*time.Time",
					"ArrayValuesParams.Blobs": "[][]byte", "ArrayValuesParams.Documents": "[]json.RawMessage",
					"FilterTimesParams.Times": "[]*time.Time",
				} {
					if got[field] != want {
						t.Errorf("%s=%s want %s", field, got[field], want)
					}
				}
			}
		})
	}
}
