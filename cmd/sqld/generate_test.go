package main

import (
	"strings"
	"testing"

	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
)

func TestStableJSON(t *testing.T) {
	t.Parallel()
	cat := &irv1.Catalog{
		IrVersion:  "v1",
		Extensions: []*irv1.Extension{{Name: "pg_trgm", Schema: "public"}},
	}
	got, err := stableJSON(cat)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{
  "irVersion": "v1",
  "extensions": [
    {
      "name": "pg_trgm",
      "schema": "public"
    }
  ]
}`
	if string(got) != want {
		t.Fatalf("stableJSON =\n%s\nwant\n%s", got, want)
	}
	if strings.Contains(string(got), ":  ") {
		t.Fatal("protojson whitespace randomization leaked through")
	}
}
