package query

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gopherex/sqld/internal/parse"
	pg "github.com/pganalyze/pg_query_go/v6"
	pgquery "github.com/wasilibs/go-pgquery"
	"google.golang.org/protobuf/reflect/protopath"
	"google.golang.org/protobuf/reflect/protorange"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// rewriteNamedParams shares one positional binding across @name, sqlc.arg(name)
// and sqlc.narg(name). PostgreSQL's scanner protects strings, quoted identifiers,
// nested comments and operators. Source text outside parameter spans is retained.
// Nullable inputs (narg) and optional predicates (@name?) are distinct contracts.
func rewriteNamedParams(sql string) (rewritten string, names map[uint32]string, optional, nullable map[uint32]bool, err error) {
	names = make(map[uint32]string)
	optional = make(map[uint32]bool)
	nullable = make(map[uint32]bool)
	scanned, err := pgquery.Scan(sql)
	if err != nil {
		return "", nil, nil, nil, err
	}
	var tokens []*pg.ScanToken
	var next uint32 = 1
	for _, tok := range scanned.GetTokens() {
		if tok.GetToken() == pg.Token_SQL_COMMENT || tok.GetToken() == pg.Token_C_COMMENT {
			continue
		}
		tokens = append(tokens, tok)
		// Reserve explicit positional bindings before allocating named parameters.
		if tok.GetToken() == pg.Token_PARAM {
			n, e := strconv.ParseUint(sql[tok.Start+1:tok.End], 10, 31)
			if e != nil || n == 0 {
				return "", nil, nil, nil, fmt.Errorf("invalid positional parameter %s", sql[tok.Start:tok.End])
			}
			if uint32(n) >= next {
				next = uint32(n) + 1
			}
		}
	}
	raw := func(n int) string { return sql[tokens[n].Start:tokens[n].End] }
	positions := make(map[string]uint32)
	var out strings.Builder
	last := 0
	for n := 0; n < len(tokens); n++ {
		tok := tokens[n]
		start, end := int(tok.Start), int(tok.End)
		name := ""
		isOptional, isNullable := false, false
		word := raw(n)
		if tok.GetToken() == pg.Token_Op && strings.HasSuffix(word, "@") && !strings.HasSuffix(word, "@@") && end < len(sql) && isIdentStart(sql[end]) {
			start = end - 1
			for end < len(sql) && isIdentCont(sql[end]) {
				end++
			}
			name = sql[start+1 : end]
			if end < len(sql) && sql[end] == '?' {
				isOptional = true
				end++
			}
		} else if sqlIdentifier(word) == "sqlc" && (n == 0 || raw(n-1) != ".") && n+3 < len(tokens) && raw(n+1) == "." && raw(n+3) == "(" {
			macro := sqlIdentifier(raw(n + 2))
			if macro != "arg" && macro != "narg" {
				return "", nil, nil, nil, fmt.Errorf("unsupported sqlc macro sqlc.%s at byte %d; supported macros: sqlc.arg, sqlc.narg", macro, start)
			}
			depth, close := 1, n+4
			for ; close < len(tokens); close++ {
				switch raw(close) {
				case "(":
					depth++
				case ")":
					depth--
				}
				if depth == 0 {
					break
				}
			}
			if close == len(tokens) {
				return "", nil, nil, nil, fmt.Errorf("unterminated sqlc.%s at byte %d", macro, start)
			}
			end = int(tokens[close].End)
			name, err = macroParameterName(sql[start:end])
			if err != nil {
				return "", nil, nil, nil, fmt.Errorf("sqlc.%s at byte %d: %w", macro, start, err)
			}
			isNullable = macro == "narg"
			n = close
		}
		if name == "" {
			continue
		}
		pos, exists := positions[name]
		if !exists {
			if next > 1<<31-1 {
				return "", nil, nil, nil, fmt.Errorf("named parameter %q exceeds PostgreSQL parameter number range", name)
			}
			pos = next
			next++
			positions[name] = pos
			names[pos] = name
		}
		if isOptional {
			optional[pos] = true
		}
		if isNullable {
			nullable[pos] = true
		}
		if optional[pos] && nullable[pos] {
			return "", nil, nil, nil, fmt.Errorf("parameter %q combines sqlc.narg with @name?: nullable binding and optional predicate cannot be combined", name)
		}
		out.WriteString(sql[last:start])
		fmt.Fprintf(&out, "$%d", pos)
		last = end
		// Skip tokens fully consumed by the parameter, leaving a trailing operator
		// (e.g. the '=' in @x?=1) in the original source for the next copy.
		for n+1 < len(tokens) && int(tokens[n+1].End) <= end {
			n++
		}
	}
	if last == 0 {
		return sql, names, optional, nullable, nil
	}
	out.WriteString(sql[last:])
	return out.String(), names, optional, nullable, nil
}

func sqlIdentifier(s string) string {
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") {
		return strings.ReplaceAll(s[1:len(s)-1], "\"\"", "\"")
	}
	return strings.ToLower(s)
}

// Parse the argument with PostgreSQL so escaped strings and quoted names have
// exactly SQL's spelling rules. Expressions and aggregate modifiers are invalid.
func macroParameterName(call string) (string, error) {
	parsed, err := pgquery.Parse("SELECT " + call)
	if err != nil {
		return "", err
	}
	fc := parsed.GetStmts()[0].GetStmt().GetSelectStmt().GetTargetList()[0].GetResTarget().GetVal().GetFuncCall()
	if len(fc.GetArgs()) == 1 && !fc.GetAggStar() && !fc.GetAggDistinct() && !fc.GetAggWithinGroup() && !fc.GetFuncVariadic() && len(fc.GetAggOrder()) == 0 {
		arg := fc.GetArgs()[0]
		if literal := arg.GetAConst(); literal != nil && literal.GetSval() != nil && literal.GetSval().GetSval() != "" {
			return literal.GetSval().GetSval(), nil
		}
		if ref := arg.GetColumnRef(); ref != nil && len(ref.GetFields()) == 1 && ref.GetFields()[0].GetString_() != nil {
			return ref.GetFields()[0].GetString_().GetSval(), nil
		}
	}
	return "", fmt.Errorf("expected one non-empty string literal or identifier as parameter name")
}

// Validate the parsed tree as well: PostgreSQL can spell identifiers using
// Unicode escapes (including UESCAPE), which the textual recognizer does not
// expand. Never let an unexpanded sqlc call reach a generated query as SQL.
func rejectUnexpandedSQLCMacros(stmts []parse.Stmt) error {
	for _, stmt := range stmts {
		err := (protorange.Options{Stable: true}).Range(stmt.Node.ProtoReflect(), func(path protopath.Values) error {
			msg, ok := path.Index(-1).Value.Interface().(protoreflect.Message)
			if !ok {
				return nil
			}
			call, ok := msg.Interface().(*pg.FuncCall)
			if !ok {
				return nil
			}
			name := call.GetFuncname()
			if len(name) == 2 && name[0].GetString_().GetSval() == "sqlc" {
				return fmt.Errorf("unexpanded sqlc.%s at byte %d: unsupported macro or spelling; use sqlc.arg('name') or sqlc.narg('name')", name[1].GetString_().GetSval(), call.GetLocation())
			}
			return nil
		}, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func isIdentStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}
func isIdentCont(b byte) bool { return isIdentStart(b) || (b >= '0' && b <= '9') }
