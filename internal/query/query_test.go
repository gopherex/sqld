package query

import (
	"testing"

	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

func TestParseQueriesHeader(t *testing.T) {
	qs, err := ParseQueries("-- name: GetUser :one\nSELECT id, email FROM users WHERE id = $1;\n", "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 1 {
		t.Fatalf("queries=%d", len(qs))
	}
	if qs[0].GetName() != "GetUser" {
		t.Fatalf("name=%q", qs[0].GetName())
	}
	if qs[0].GetCommand() != pluginv1.QueryCommand_QUERY_COMMAND_ONE {
		t.Fatalf("cmd=%v", qs[0].GetCommand())
	}
	if qs[0].GetAst().GetSelect() == nil {
		t.Fatal("ast not a select")
	}
}

func TestParseQueriesCommand(t *testing.T) {
	cases := []struct {
		suffix  string
		command pluginv1.QueryCommand
	}{
		{"one", pluginv1.QueryCommand_QUERY_COMMAND_ONE},
		{"many", pluginv1.QueryCommand_QUERY_COMMAND_MANY},
		{"exec", pluginv1.QueryCommand_QUERY_COMMAND_EXEC},
		{"execrows", pluginv1.QueryCommand_QUERY_COMMAND_EXEC_ROWS},
		{"execresult", pluginv1.QueryCommand_QUERY_COMMAND_EXEC_RESULT},
		{"execlastid", pluginv1.QueryCommand_QUERY_COMMAND_EXEC_LASTID},
		{"batchexec", pluginv1.QueryCommand_QUERY_COMMAND_BATCH_EXEC},
		{"copyfrom", pluginv1.QueryCommand_QUERY_COMMAND_COPY_FROM},
		{"unknown", pluginv1.QueryCommand_QUERY_COMMAND_UNSPECIFIED},
	}
	for _, tc := range cases {
		sql := "-- name: Q :" + tc.suffix + "\nSELECT 1;\n"
		qs, err := ParseQueries(sql, "f.sql")
		if err != nil {
			t.Fatalf("suffix=%s: %v", tc.suffix, err)
		}
		if len(qs) != 1 {
			t.Fatalf("suffix=%s: queries=%d", tc.suffix, len(qs))
		}
		if qs[0].GetCommand() != tc.command {
			t.Fatalf("suffix=%s: want=%v got=%v", tc.suffix, tc.command, qs[0].GetCommand())
		}
	}
}

func TestParseQueriesMultiple(t *testing.T) {
	sql := `-- name: GetAll :many
SELECT id FROM users;
-- name: GetOne :one
SELECT id FROM users WHERE id = $1;
`
	qs, err := ParseQueries(sql, "q.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("queries=%d", len(qs))
	}
	if qs[0].GetName() != "GetAll" {
		t.Fatalf("name0=%q", qs[0].GetName())
	}
	if qs[1].GetName() != "GetOne" {
		t.Fatalf("name1=%q", qs[1].GetName())
	}
}

func TestParseQueriesSourceFile(t *testing.T) {
	qs, _ := ParseQueries("-- name: Q :exec\nDELETE FROM t;\n", "myfile.sql")
	if len(qs) != 1 {
		t.Fatal("expected 1 query")
	}
	if qs[0].GetSourceFile() != "myfile.sql" {
		t.Fatalf("source_file=%q", qs[0].GetSourceFile())
	}
}

func TestParseQueriesComment(t *testing.T) {
	sql := "-- This is a doc comment\n-- name: Q :exec\nDELETE FROM t;\n"
	qs, _ := ParseQueries(sql, "f.sql")
	if len(qs) != 1 {
		t.Fatal("expected 1 query")
	}
	if qs[0].GetComment() == "" {
		t.Fatal("expected comment to be set")
	}
}

func TestParseQueriesInvalidBodySkipped(t *testing.T) {
	// second query has invalid SQL body; should return only the first
	sql := "-- name: Q1 :one\nSELECT 1;\n-- name: Q2 :exec\nnot valid sql !!!;\n"
	qs, err := ParseQueries(sql, "f.sql")
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 1 {
		t.Fatalf("queries=%d", len(qs))
	}
	if qs[0].GetName() != "Q1" {
		t.Fatalf("name=%q", qs[0].GetName())
	}
}
