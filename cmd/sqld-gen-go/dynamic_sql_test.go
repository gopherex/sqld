package main

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	pgquery "github.com/wasilibs/go-pgquery"
)

// PostgreSQL is the independent oracle for the precedence of the reconstructed
// expression. Production generation remains usable in a WASM plugin.
func TestConditionTreePreservesPostgresExpression(t *testing.T) {
	for _, sql := range []string{
		"a OR b AND c", "(a OR b) AND c", "NOT (a OR b) AND c",
		"NOT NOT a OR b", "x NOT BETWEEN 1 AND 2 AND a OR b",
		"x BETWEEN SYMMETRIC 1 AND 2 AND y BETWEEN 3 AND 4",
		"CASE WHEN a AND b THEN c OR d ELSE false END AND e",
		"CASE WHEN a THEN CASE WHEN b THEN c ELSE d END ELSE false END OR e",
		"EXISTS (SELECT 1 WHERE a OR b) AND c",
		"(SELECT a AND b FROM t WHERE c OR d) AND e",
		"(WITH t AS (SELECT true AS a) SELECT a FROM t) OR b",
		"(ARRAY[a AND b, c OR d])[1] AND e",
		"a IS NOT DISTINCT FROM b AND c IS NOT TRUE",
		"p.end = $1 AND p.tenant_id = $2",
		"p.between = $1 AND p.or = $2 OR p.case",
		"a AND /* OR /* AND */ WHERE */ b OR c -- AND d",
		"note = 'WHERE $1 AND 50%' AND note <> $$OR $2$$",
		`note = E'\' OR $99' AND "WHERE" = $1`,
	} {
		t.Run(sql, func(t *testing.T) {
			tree, err := parseConditionTree(sql, nil, nil, overrides{})
			if err != nil {
				t.Fatal(err)
			}
			var render func(*conditionTree) string
			render = func(tree *conditionTree) string {
				if tree.op == "" {
					return tree.leaf.condSQL
				}
				var parts []string
				for _, child := range tree.children {
					parts = append(parts, render(child))
				}
				if tree.op == "NOT" {
					return "NOT (" + parts[0] + ")"
				}
				return "(" + strings.Join(parts, " "+tree.op+" ") + ")"
			}
			canonical := func(sql string) string {
				t.Helper()
				ast, err := pgquery.Parse("SELECT 1 WHERE " + sql)
				if err != nil {
					t.Fatalf("parse %s: %v", sql, err)
				}
				text, err := pgquery.Deparse(ast)
				if err != nil {
					t.Fatal(err)
				}
				return text
			}
			if got, want := canonical(render(tree)), canonical(sql); got != want {
				t.Fatalf("changed PostgreSQL expression:\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

func TestSQLParameterTokens(t *testing.T) {
	sql := `SELECT 'WHERE $99', "col$8", $$ $7 $$, $tag$ $6 $tag$, E'\' $5', foo$4
/* $3 /* $2 */ */ -- $1
WHERE id = ANY(($12::bigint[])) AND x=$2 AND y=$12`
	var numbers []uint32
	for _, param := range sqlParameters(sql) {
		numbers = append(numbers, param.number)
	}
	if !reflect.DeepEqual(numbers, []uint32{12, 2, 12}) {
		t.Fatalf("parameters=%v", numbers)
	}
	if got := sliceParamNumbers(sql); !reflect.DeepEqual(got, map[uint32]bool{12: true}) {
		t.Fatalf("arrays=%v", got)
	}
	info := buildConditionInfo(sql, nil, nil, overrides{})
	format, indexes := conditionFormat(info)
	var args []any
	for _, index := range indexes {
		args = append(args, index+1)
	}
	got := fmt.Sprintf(format, args...)
	want := strings.ReplaceAll(sql, "$12", "$1")
	if got != want {
		t.Fatalf("rewritten SQL:\n%s\nwant:\n%s", got, want)
	}
}

func TestDynamicSQLRejectsUnclosedLexemes(t *testing.T) {
	for _, sql := range []string{"SELECT 'x", "SELECT $$x", "SELECT /* x", "SELECT E'\\'"} {
		if _, err := lexDynamicSQL(sql); err == nil {
			t.Errorf("accepted %q", sql)
		}
	}
}
