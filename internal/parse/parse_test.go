package parse

import "testing"

func TestParseSelectSmoke(t *testing.T) {
	res, err := Parse("SELECT 1")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(res.GetStmts()) != 1 {
		t.Fatalf("want 1 stmt, got %d", len(res.GetStmts()))
	}
	if res.GetStmts()[0].GetStmt().GetSelectStmt() == nil {
		t.Fatalf("want SelectStmt node")
	}
}
