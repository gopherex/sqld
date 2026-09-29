package main

import (
	"fmt"
	"strconv"
	"strings"

	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

// sqlToken retains byte offsets while keeping quoted text and comments out of
// structural detection. This scanner is deliberately dependency-free: the Go
// plugin also runs in WASM, where nesting a PostgreSQL WASM parser is too costly.
type sqlToken struct {
	start, end int
	text       string
}

func sqlTokens(sql string) []sqlToken {
	tokens, _ := lexDynamicSQL(sql) // The host validates the complete SQL first.
	return tokens
}

func lexDynamicSQL(sql string) ([]sqlToken, error) {
	var tokens []sqlToken
	for i := 0; i < len(sql); {
		start := i
		c := sql[i]
		if strings.ContainsRune(" \t\r\n\f", rune(c)) {
			i++
			continue
		}
		if strings.HasPrefix(sql[i:], "--") {
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
			continue
		}
		if strings.HasPrefix(sql[i:], "/*") {
			i += 2
			depth := 1
			for i < len(sql) && depth > 0 {
				switch {
				case strings.HasPrefix(sql[i:], "/*"):
					depth++
					i += 2
				case strings.HasPrefix(sql[i:], "*/"):
					depth--
					i += 2
				default:
					i++
				}
			}
			if depth != 0 {
				return nil, fmt.Errorf("unterminated SQL comment")
			}
			continue
		}
		text := ""
		switch {
		case c == '\'' || c == '"':
			escape := c == '\'' && len(tokens) > 0 && tokens[len(tokens)-1].text == "E" && tokens[len(tokens)-1].end == i
			i++
			closed := false
			for i < len(sql) {
				if escape && sql[i] == '\\' {
					i += 2
					continue
				}
				if sql[i] == c {
					i++
					if i < len(sql) && sql[i] == c {
						i++
						continue
					}
					closed = true
					break
				}
				i++
			}
			if !closed {
				return nil, fmt.Errorf("unterminated SQL quote")
			}
		case c == '$':
			i++
			if i < len(sql) && sql[i] >= '0' && sql[i] <= '9' {
				for i < len(sql) && sql[i] >= '0' && sql[i] <= '9' {
					i++
				}
				text = "$PARAM"
			} else {
				for i < len(sql) && sqlIdentByte(sql[i]) && sql[i] != '$' {
					i++
				}
				if i < len(sql) && sql[i] == '$' {
					i++
					tag := sql[start:i]
					end := strings.Index(sql[i:], tag)
					if end < 0 {
						return nil, fmt.Errorf("unterminated dollar quote")
					}
					i += end + len(tag)
				} else {
					i = start + 1
					text = "$"
				}
			}
		case sqlIdentByte(c):
			i++
			for i < len(sql) && sqlIdentByte(sql[i]) {
				i++
			}
			text = strings.ToUpper(sql[start:i])
			// PostgreSQL allows keywords after a qualification dot (p.end,
			// p.where). They are identifiers here, never expression operators.
			if len(tokens) > 0 && tokens[len(tokens)-1].text == "." {
				text = ""
			}
		default:
			i++
			text = sql[start:i]
		}
		tokens = append(tokens, sqlToken{start, i, text})
	}
	return tokens, nil
}

func sqlIdentByte(c byte) bool {
	return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 128
}

type sqlParameter struct {
	start, end int
	number     uint32
}

func sqlParameters(sql string) []sqlParameter {
	var params []sqlParameter
	for _, token := range sqlTokens(sql) {
		if token.text == "$PARAM" {
			n, _ := strconv.ParseUint(sql[token.start+1:token.end], 10, 32)
			params = append(params, sqlParameter{int(token.start), int(token.end), uint32(n)})
		}
	}
	return params
}

func sliceParamNumbers(sql string) map[uint32]bool {
	result := make(map[uint32]bool)
	tokens := sqlTokens(sql)
	for i, token := range tokens {
		if token.text != "ANY" {
			continue
		}
		j := i + 1
		for j < len(tokens) && sql[tokens[j].start:tokens[j].end] == "(" {
			j++
		}
		if j > i+1 && j < len(tokens) && tokens[j].text == "$PARAM" {
			n, _ := strconv.ParseUint(sql[tokens[j].start+1:tokens[j].end], 10, 32)
			result[uint32(n)] = true
		}
	}
	return result
}

func topLevelToken(sql string, accept func(string) bool) int {
	depth := 0
	for _, token := range sqlTokens(sql) {
		switch sql[token.start:token.end] {
		case "(":
			depth++
		case ")":
			depth--
		default:
			if depth == 0 && accept(token.text) {
				return int(token.start)
			}
		}
	}
	return -1
}

func topLevelClauseStart(sql string) int {
	return topLevelToken(sql, func(token string) bool {
		switch token {
		case "ORDER", "GROUP", "HAVING", "LIMIT",
			"OFFSET", "FETCH", "WINDOW", "UNION",
			"INTERSECT", "EXCEPT", "RETURNING", "ON",
			"FOR":
			return true
		}
		return false
	})
}

func splitWhereSuffix(sql string) (whereBody, suffix string) {
	if i := topLevelClauseStart(sql); i >= 0 {
		return strings.TrimSpace(sql[:i]), strings.TrimSpace(sql[i:])
	}
	return strings.TrimSpace(sql), ""
}

func dynamicSQLParts(sql string, params []*pluginv1.QueryParameter, orderBy bool) (base, body, suffix string, err error) {
	// Ordinary arrays use the existing Params API, but never authorize SQL
	// rewriting. In particular, nil and empty arrays must retain ANY predicates.
	optional := false
	for _, p := range params {
		optional = optional || p.GetOptional()
	}
	if !optional && !orderBy {
		return sql, "", "", nil
	}
	if _, err := lexDynamicSQL(sql); err != nil {
		return "", "", "", fmt.Errorf("parse dynamic SQL: %w", err)
	}
	// Strip only an executable terminator, never punctuation inside a comment.
	tokens := sqlTokens(sql)
	if len(tokens) > 0 {
		last := tokens[len(tokens)-1]
		if sql[last.start:last.end] == ";" {
			sql = sql[:last.start] + sql[last.end:]
		}
	}
	i := topLevelToken(sql, func(token string) bool { return token == "WHERE" })
	if i < 0 {
		if orderBy {
			if j := topLevelToken(sql, func(token string) bool {
				switch token {
				case "ORDER", "LIMIT", "OFFSET", "FETCH", "FOR":
					return true
				}
				return false
			}); j >= 0 {
				return strings.TrimSpace(sql[:j]), "", strings.TrimSpace(sql[j:]), nil
			}
		}
		return strings.TrimSpace(sql), "", "", nil
	}
	body, suffix = splitWhereSuffix(sql[i+len("WHERE"):])
	return strings.TrimSpace(sql[:i]), body, suffix, nil
}

// A leaf is one SQL predicate; AND/OR/NOT remain explicit nodes so omission
// cannot accidentally change operator precedence or split BETWEEN/CASE.
type conditionTree struct {
	op       string
	children []*conditionTree
	leaf     conditionInfo
}

func parseConditionTree(sql string, params map[uint32]*pluginv1.QueryParameter, reg *udtRegistry, ov overrides) (*conditionTree, error) {
	if sql == "" {
		return nil, nil
	}
	tokens, err := lexDynamicSQL(sql)
	if err != nil {
		return nil, err
	}
	if len(tokens) == 0 {
		return nil, fmt.Errorf("empty WHERE expression")
	}
	// Strip parentheses only when they enclose the entire expression.
	subquery := len(tokens) > 1 && (tokens[1].text == "SELECT" || tokens[1].text == "WITH" || tokens[1].text == "VALUES" || tokens[1].text == "TABLE")
	if tokens[0].text == "(" && !subquery {
		depth := 0
		for i, token := range tokens {
			if token.text == "(" {
				depth++
			}
			if token.text == ")" {
				depth--
			}
			if depth == 0 {
				if i == len(tokens)-1 {
					return parseConditionTree(sql[tokens[0].end:token.start], params, reg, ov)
				}
				break
			}
		}
	}
	// SQL precedence: OR < AND < NOT. BETWEEN consumes one AND;
	// CASE expressions and nested scopes are opaque predicates at this level.
	for _, op := range []string{"OR", "AND"} {
		depth, cases, between := 0, 0, 0
		var boundaries []sqlToken
		for _, token := range tokens {
			switch token.text {
			case "(", "[":
				depth++
			case ")", "]":
				depth--
			case "CASE":
				cases++
			case "END":
				cases--
			default:
				if depth != 0 || cases != 0 {
					continue
				}
				if token.text == "BETWEEN" {
					between++
					continue
				}
				if token.text == "AND" && between > 0 {
					between--
					continue
				}
				if token.text == op {
					boundaries = append(boundaries, token)
				}
			}
		}
		if len(boundaries) == 0 {
			continue
		}
		tree := &conditionTree{op: op}
		start := 0
		for _, token := range append(boundaries, sqlToken{start: len(sql), end: len(sql)}) {
			child, err := parseConditionTree(sql[start:token.start], params, reg, ov)
			if err != nil {
				return nil, err
			}
			tree.children = append(tree.children, child)
			start = token.end
		}
		return tree, nil
	}
	if tokens[0].text == "NOT" {
		child, err := parseConditionTree(sql[tokens[0].end:], params, reg, ov)
		if err != nil {
			return nil, err
		}
		return &conditionTree{op: "NOT", children: []*conditionTree{child}}, nil
	}
	// Drop surrounding comments while preserving all executable source bytes,
	// including literal contents and comments inside the expression.
	leafSQL := strings.TrimSpace(sql[tokens[0].start:tokens[len(tokens)-1].end])
	return &conditionTree{leaf: buildConditionInfo(leafSQL, params, reg, ov)}, nil
}

// Place runtime ORDER BY after grouping/window clauses and before pagination
// and locking. The original ORDER BY remains the default when none is chosen.
func splitOrderBySuffix(sql string) (before, defaultOrder, after string) {
	end := topLevelToken(sql, func(token string) bool {
		return token == "LIMIT" || token == "OFFSET" || token == "FETCH" || token == "FOR"
	})
	if end >= 0 {
		after = strings.TrimSpace(sql[end:])
		sql = sql[:end]
	}
	start := topLevelToken(sql, func(token string) bool { return token == "ORDER" })
	if start >= 0 {
		defaultOrder = strings.TrimSpace(sql[start:])
		sql = sql[:start]
	}
	return strings.TrimSpace(sql), defaultOrder, after
}

func collectConditions(tree *conditionTree, conditions *[]conditionInfo) {
	if tree == nil {
		return
	}
	if tree.op == "" {
		*conditions = append(*conditions, tree.leaf)
	}
	for _, child := range tree.children {
		collectConditions(child, conditions)
	}
}

func emitConditionTree(sb *strings.Builder, tree *conditionTree, next *int) string {
	name := fmt.Sprintf("cond%d", *next)
	*next++
	fmt.Fprintf(sb, "\tvar %s string\n", name)
	if tree.op != "" {
		parts := name + "Parts"
		fmt.Fprintf(sb, "\tvar %s []string\n", parts)
		for _, child := range tree.children {
			value := emitConditionTree(sb, child, next)
			fmt.Fprintf(sb, "\tif %s != \"\" { %s = append(%s, %s) }\n", value, parts, parts, value)
		}
		if tree.op == "NOT" {
			fmt.Fprintf(sb, "\tif len(%s) > 0 { %s = \"NOT (\" + %s[0] + \")\" }\n", parts, name, parts)
		} else {
			fmt.Fprintf(sb, "\tif len(%s) > 0 { %s = \"(\" + strings.Join(%s, %q) + \")\" }\n", parts, name, parts, " "+tree.op+" ")
		}
		return name
	}
	var guards []string
	for _, param := range tree.leaf.params {
		if param.isOptional {
			guards = append(guards, "arg."+param.fieldName+" != nil")
		}
	}
	if len(guards) > 0 {
		fmt.Fprintf(sb, "\tif %s {\n", strings.Join(guards, " && "))
	}
	emitConditionParams(sb, tree.leaf.params, true)
	if len(tree.leaf.params) == 0 {
		fmt.Fprintf(sb, "\t%s = %q\n", name, tree.leaf.condSQL)
	} else {
		sql, indexes := conditionFormat(tree.leaf)
		fmt.Fprintf(sb, "\t%s = fmt.Sprintf(%q, %s)\n", name, sql, strings.Join(conditionFormatArgs(tree.leaf.params, indexes), ", "))
	}
	if len(guards) > 0 {
		sb.WriteString("\t}\n")
	}
	return name
}

// Reuse the same bind position across fragments. Besides avoiding duplicated
// arguments, this preserves PostgreSQL type inference for "x=$1 OR $1 IS NULL".
func emitConditionParams(sb *strings.Builder, params []conditionParam, guarded bool) {
	for _, param := range params {
		value := "arg." + param.fieldName
		if guarded && param.isOptional && !param.isSlice {
			value = "*" + value
		}
		fmt.Fprintf(sb, "\tif positions[%d] == 0 {\n", param.paramNum)
		fmt.Fprintf(sb, "\targs = append(args, %s)\n", value)
		fmt.Fprintf(sb, "\tpositions[%d] = len(args)\n\t}\n", param.paramNum)
	}
}
