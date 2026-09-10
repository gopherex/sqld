package query

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/gopherex/sqld/internal/catalog"
	"github.com/gopherex/sqld/internal/parse"
)

// Expected parameter types were checked against PostgreSQL 17 PREPARE and
// pg_prepared_statements.parameter_types. An empty type list denotes SQL that
// PostgreSQL rejects; sqld must diagnose it rather than silently guess a type.
func TestInferParameterContexts(t *testing.T) {
	data, err := os.ReadFile("testdata/param_context.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name, SQL string
		Types     []string
		Columns   []string
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	stmts, err := parse.Statements(`CREATE TABLE users(id bigint PRIMARY KEY, col text NOT NULL,
		quantity integer NOT NULL, active boolean NOT NULL, stamp timestamp, duration interval);
		CREATE TABLE events(id uuid, col uuid);`)
	if err != nil {
		t.Fatal(err)
	}
	cat, _ := catalog.Build(stmts)
	for _, tc := range cases {
		t.Run(tc.Name, func(t *testing.T) {
			qs, err := ParseQueries("-- name: Q :one\n"+tc.SQL, "q.sql")
			if err != nil {
				t.Fatal(err)
			}
			var d catalog.Diagnostics
			Infer(qs[0], cat, &d)
			params := qs[0].GetParameters()
			if len(tc.Types) == 0 {
				for _, item := range d.Items {
					if strings.Contains(item.Message, "param") || strings.Contains(item.Message, "column") {
						return
					}
				}
				t.Fatalf("invalid or ambiguous SQL received no parameter/column diagnostic: %v", params)
			}
			if len(params) != len(tc.Types) {
				t.Fatalf("params=%v, want %v", params, tc.Types)
			}
			for i, want := range tc.Types {
				got := params[i].GetType().GetPgName()
				if params[i].GetType().GetElement() != nil {
					got = "_" + params[i].GetType().GetElement().GetPgName()
				}
				if got != want {
					t.Errorf("param %s: type=%q, want %q", params[i].GetName(), got, want)
				}
			}
			if tc.Columns != nil {
				cols := qs[0].GetColumns()
				if len(cols) != len(tc.Columns) {
					t.Fatalf("columns=%v, want %v", cols, tc.Columns)
				}
				for n, want := range tc.Columns {
					if got := cols[n].GetType().GetPgName(); got != want {
						t.Errorf("result column %d: type=%q, want %q", n, got, want)
					}
				}
			}
		})
	}
}
