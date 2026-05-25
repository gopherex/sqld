// Package query parses named DML query files (sqlc-style annotations) and
// maps each query to a pluginv1.Query IR node.
package query

import (
	"regexp"
	"strings"

	"github.com/yaroher/sqld/internal/mapper"
	"github.com/yaroher/sqld/internal/nodeid"
	"github.com/yaroher/sqld/internal/parse"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// headerRe matches lines of the form:
//
//	-- name: QueryName :command
//
// Group 1 = name, group 2 = command suffix (case-insensitive for "name").
var headerRe = regexp.MustCompile(`(?i)^--\s*name:\s*(\w+)\s+:(\w+)\s*$`)

// commentRe matches a plain comment line (not a header).
var commentRe = regexp.MustCompile(`^--`)

// ParseQueries scans sql for named query blocks, parses each, and returns
// the resulting []*pluginv1.Query.
//
// A query block begins with a header comment:
//
//	-- name: QueryName :command
//
// and extends until the next header (or EOF). If a body fails to parse it is
// silently skipped; the caller may emit a diagnostic.
func ParseQueries(sql, sourceFile string) ([]*pluginv1.Query, error) {
	lines := splitLines(sql)

	type block struct {
		name      string
		command   pluginv1.QueryCommand
		bodyLines []string
		comment   string // non-header comment lines immediately before the header
	}

	var blocks []block
	var pendingComments []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		m := headerRe.FindStringSubmatch(line)
		if m != nil {
			name := m[1]
			cmd := parseCommand(strings.ToLower(m[2]))
			blocks = append(blocks, block{
				name:      name,
				command:   cmd,
				comment:   strings.Join(pendingComments, "\n"),
				bodyLines: nil,
			})
			pendingComments = nil
			continue
		}

		if len(blocks) == 0 {
			// We are before the first header; collect potential leading comments.
			if commentRe.MatchString(strings.TrimSpace(line)) {
				pendingComments = append(pendingComments, strings.TrimSpace(line))
			} else if strings.TrimSpace(line) != "" {
				// non-comment, non-empty line before any header resets comment buffer
				pendingComments = nil
			}
			continue
		}

		// Accumulate into the current (last) block.
		blocks[len(blocks)-1].bodyLines = append(blocks[len(blocks)-1].bodyLines, line)

		// After accumulating, if this line is a comment-only line it's not
		// "pending" for the next header (handled separately above), so we reset
		// pendingComments tracking only when we encounter a non-header line after
		// a header.
		//
		// For the next iteration's pendingComments: only lines immediately
		// BEFORE the header count. We reset here because once we are inside a
		// block, the comment lines are body lines, not pending comments.
		pendingComments = nil
	}

	// Handle comment lines that appear between blocks (before the next header).
	// Re-scan to correctly attribute leading comments to the block they precede.
	blocks = nil
	pendingComments = nil
	currentBlock := -1

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		m := headerRe.FindStringSubmatch(trimmed)
		if m != nil {
			name := m[1]
			cmd := parseCommand(strings.ToLower(m[2]))
			blocks = append(blocks, block{
				name:      name,
				command:   cmd,
				comment:   strings.Join(pendingComments, "\n"),
				bodyLines: nil,
			})
			currentBlock = len(blocks) - 1
			pendingComments = nil
			continue
		}

		if currentBlock < 0 {
			// Before first header
			if commentRe.MatchString(trimmed) && trimmed != "" {
				pendingComments = append(pendingComments, trimmed)
			} else if trimmed != "" {
				pendingComments = nil
			}
			continue
		}

		// Inside a block body
		blocks[currentBlock].bodyLines = append(blocks[currentBlock].bodyLines, line)

		// When a line inside a block is a comment, it might be followed by a new
		// header - track potential pending comments.
		if commentRe.MatchString(trimmed) && trimmed != "" {
			pendingComments = append(pendingComments, trimmed)
		} else if trimmed != "" {
			// Non-comment body line resets the pending comment buffer.
			pendingComments = nil
		}
	}

	var queries []*pluginv1.Query

	for _, b := range blocks {
		body := strings.TrimSpace(strings.Join(b.bodyLines, "\n"))
		if body == "" {
			continue
		}

		// Rewrite @ident named params → $N before parsing, so that
		// libpg_query only sees positional placeholders it understands.
		rewritten, names := rewriteNamedParams(body)

		stmts, err := parse.Statements(rewritten)
		if err != nil || len(stmts) == 0 {
			// Skip unparseable bodies silently.
			continue
		}

		first := stmts[0]
		irStmt := mapper.MapStatement(first.Node, nodeid.New("query:"+b.name))

		// Seed parameters from the name map (sorted by position so that the
		// slice is always in order even before Infer runs).
		var seededParams []*pluginv1.QueryParameter
		if len(names) > 0 {
			var maxPos uint32
			for pos := range names {
				if pos > maxPos {
					maxPos = pos
				}
			}
			for n := uint32(1); n <= maxPos; n++ {
				if name, ok := names[n]; ok {
					seededParams = append(seededParams, &pluginv1.QueryParameter{
						Number: n,
						Name:   name,
					})
				}
			}
		}

		q := &pluginv1.Query{
			Name:       b.name,
			Sql:        rewritten,
			Command:    b.command,
			Ast:        irStmt,
			SourceFile: sourceFile,
			Comment:    b.comment,
			Parameters: seededParams,
		}
		queries = append(queries, q)
	}

	return queries, nil
}

// splitLines splits sql into lines preserving empty lines.
func splitLines(sql string) []string {
	return strings.Split(sql, "\n")
}

// parseCommand maps a lower-cased command suffix to a QueryCommand enum value.
func parseCommand(s string) pluginv1.QueryCommand {
	switch s {
	case "one":
		return pluginv1.QueryCommand_QUERY_COMMAND_ONE
	case "many":
		return pluginv1.QueryCommand_QUERY_COMMAND_MANY
	case "exec":
		return pluginv1.QueryCommand_QUERY_COMMAND_EXEC
	case "execrows":
		return pluginv1.QueryCommand_QUERY_COMMAND_EXEC_ROWS
	case "execresult":
		return pluginv1.QueryCommand_QUERY_COMMAND_EXEC_RESULT
	case "execlastid":
		return pluginv1.QueryCommand_QUERY_COMMAND_EXEC_LASTID
	case "batchexec":
		return pluginv1.QueryCommand_QUERY_COMMAND_BATCH_EXEC
	case "copyfrom":
		return pluginv1.QueryCommand_QUERY_COMMAND_COPY_FROM
	default:
		return pluginv1.QueryCommand_QUERY_COMMAND_UNSPECIFIED
	}
}
