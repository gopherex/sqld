package parse

import (
	pg "github.com/wasilibs/go-pgquery"

	pganalyze "github.com/pganalyze/pg_query_go/v6"
)

// Parse parses one or more SQL statements into the libpg_query parse tree.
func Parse(sql string) (*pganalyze.ParseResult, error) {
	return pg.Parse(sql)
}

// Stmt is one parsed statement with its 0-based ordinal.
type Stmt struct {
	Index int
	Node  *pganalyze.Node // the inner statement node (RawStmt.Stmt)
}

// Statements parses SQL and returns each top-level statement.
func Statements(sql string) ([]Stmt, error) {
	res, err := Parse(sql)
	if err != nil {
		return nil, err
	}
	out := make([]Stmt, 0, len(res.GetStmts()))
	for i, raw := range res.GetStmts() {
		out = append(out, Stmt{Index: i, Node: raw.GetStmt()})
	}
	return out, nil
}
