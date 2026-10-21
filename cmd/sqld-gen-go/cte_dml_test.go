package main

import (
	_ "embed"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/internal/query"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

//go:embed testdata/cte_dml_schema.sql
var cteDMLSchema string

//go:embed testdata/cte_dml_queries.sql
var cteDMLQueries string

//go:embed testdata/cte_dml_runtime_test.go
var cteDMLRuntimeTest []byte

func TestGeneratedDataModifyingCTE(t *testing.T) {
	stmts, err := parse.Statements(cteDMLSchema)
	if err != nil {
		t.Fatal(err)
	}
	cat, d := catalog.Build(stmts)
	if len(d.Items) != 0 {
		t.Fatal(d.Items)
	}
	qs, err := query.ParseQueries(cteDMLQueries, "cte.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 6 {
		t.Fatalf("queries=%d", len(qs))
	}
	for _, q := range qs {
		query.Infer(q, cat, d)
	}
	if len(d.Items) != 0 {
		t.Fatal(d.Items)
	}
	req := &pluginv1.GenerateRequest{Catalog: cat, Queries: qs, Options: []byte(`{"nullMode":"pointer","overrides":{"int8":"uint64","int4":"int","public.feed_snapshots.type":"uint32"}}`)}
	// Exercise the protobuf boundary used by native and WASM generators.
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
	got := generatedGoTypes(t, source)
	for field, want := range map[string]string{
		"CreateFeedParams.Kind": "string", "CreateFeedParams.ID": "[]byte", "CreateFeedParams.ParentKind": "*string", "CreateFeedParams.ParentID": "[]byte",
		"BindDeviceParams.Device": "[]byte", "BindDeviceParams.Owner": "[]byte", "BindDeviceRow.Owner": "[]byte",
		"StoreSnapshotParams.Position": "uint64", "StoreSnapshotParams.Type": "uint32", "StoreSnapshotParams.Payload": "[]byte",
		"SweepDevicesParams.DeadBefore": "time.Time", "SweepDevicesParams.Limit": "uint64", "SweepDevicesRow.Removed": "int",
		"ChangeParentParams.ParentKind": "*string", "ChangeParentParams.ParentID": "[]byte", "ChangeParentRow.ParentKind": "*string",
		"SnapshotTypeParams.Type": "uint32", "SnapshotTypeRow.Type": "uint32",
	} {
		if got[field] != want {
			t.Errorf("%s=%q, want %s", field, got[field], want)
		}
	}
	if t.Failed() {
		t.FailNow()
	}
	testGeneratedPostgres(t, resp, map[string][]byte{"cte_dml_runtime_test.go": cteDMLRuntimeTest, "cte_dml_schema.sql": []byte(cteDMLSchema)})
}
