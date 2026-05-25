package parse

import (
	pg "github.com/wasilibs/go-pgquery"

	pganalyze "github.com/pganalyze/pg_query_go/v6"
)

// Parse parses one or more SQL statements into the libpg_query parse tree.
func Parse(sql string) (*pganalyze.ParseResult, error) {
	return pg.Parse(sql)
}
