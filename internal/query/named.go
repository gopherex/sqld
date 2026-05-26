package query

// rewriteNamedParams replaces @ident named parameters in sql with positional
// $N placeholders that libpg_query understands, and returns the mapping from
// position → original name and position → optional flag.
//
// The lexer tracks context so that @ident inside string literals, identifiers,
// comments, and dollar-quoted strings is left untouched.  Bare operator tokens
// like @>, @@, @-@, @? are also left untouched because the character following
// @ is not an identifier-start character.
//
// A trailing ? immediately after @ident (e.g. @email?) marks the parameter as
// optional. The ? is consumed and NOT emitted into the rewritten SQL. A name
// is considered optional if ANY occurrence uses the ? suffix.
func rewriteNamedParams(sql string) (rewritten string, names map[uint32]string, optional map[uint32]bool) {
	names = make(map[uint32]string)
	optional = make(map[uint32]bool)
	// nameToPos maps a name seen before to its assigned $N position so that
	// repeated occurrences of the same name get the same placeholder.
	nameToPos := make(map[string]uint32)
	var nextPos uint32 = 1

	// Lexer state constants.
	const (
		stateNormal       = iota
		stateLineComment  // after --
		stateBlockComment // inside /* … */
		stateSingleQuote  // inside '…'
		stateDoubleQuote  // inside "…"
		stateDollarQuote  // inside $tag$…$tag$
	)

	state := stateNormal
	out := make([]byte, 0, len(sql))
	s := sql
	var dollarTag string // the full tag including both $ delimiters, e.g. "$fmt$"

	for i := 0; i < len(s); {
		ch := s[i]

		switch state {
		// ------------------------------------------------------------------ //
		// Normal state
		// ------------------------------------------------------------------ //
		case stateNormal:
			// Check for line comment start.
			if ch == '-' && i+1 < len(s) && s[i+1] == '-' {
				out = append(out, '-', '-')
				i += 2
				state = stateLineComment
				continue
			}
			// Check for block comment start.
			if ch == '/' && i+1 < len(s) && s[i+1] == '*' {
				out = append(out, '/', '*')
				i += 2
				state = stateBlockComment
				continue
			}
			// Check for single-quoted string.
			if ch == '\'' {
				out = append(out, '\'')
				i++
				state = stateSingleQuote
				continue
			}
			// Check for double-quoted identifier.
			if ch == '"' {
				out = append(out, '"')
				i++
				state = stateDoubleQuote
				continue
			}
			// Check for dollar-quoted string: $[tag]$
			if ch == '$' {
				if tag, tagLen := scanDollarTag(s, i); tagLen > 0 {
					dollarTag = tag
					out = append(out, s[i:i+tagLen]...)
					i += tagLen
					state = stateDollarQuote
					continue
				}
				// Not a dollar-quote tag; treat as ordinary character (e.g. $1).
				out = append(out, ch)
				i++
				continue
			}
			// Check for named parameter: @ followed by identifier-start char.
			if ch == '@' && i+1 < len(s) && isIdentStart(s[i+1]) {
				// Consume the identifier.
				j := i + 1
				for j < len(s) && isIdentCont(s[j]) {
					j++
				}
				name := s[i+1 : j]
				// Check for optional marker: ? immediately following the identifier.
				isOptional := j < len(s) && s[j] == '?'
				if isOptional {
					j++ // consume the '?', do not emit it
				}
				// Assign or look up position.
				pos, seen := nameToPos[name]
				if !seen {
					pos = nextPos
					nextPos++
					nameToPos[name] = pos
					names[pos] = name
				}
				// A name is optional if ANY occurrence uses the ? suffix.
				if isOptional {
					optional[pos] = true
				}
				// Write $N placeholder.
				out = appendUint(append(out, '$'), pos)
				i = j
				continue
			}
			// Any other character passes through unchanged.
			out = append(out, ch)
			i++

		// ------------------------------------------------------------------ //
		// Line comment: everything until EOL passes through unchanged.
		// ------------------------------------------------------------------ //
		case stateLineComment:
			out = append(out, ch)
			i++
			if ch == '\n' {
				state = stateNormal
			}

		// ------------------------------------------------------------------ //
		// Block comment: pass through until */ is seen.
		// ------------------------------------------------------------------ //
		case stateBlockComment:
			if ch == '*' && i+1 < len(s) && s[i+1] == '/' {
				out = append(out, '*', '/')
				i += 2
				state = stateNormal
				continue
			}
			out = append(out, ch)
			i++

		// ------------------------------------------------------------------ //
		// Single-quoted string: handle '' escape and \ not used in standard SQL
		// but we just need to skip past the closing '.
		// ------------------------------------------------------------------ //
		case stateSingleQuote:
			if ch == '\'' {
				out = append(out, '\'')
				i++
				// '' is an escaped quote, stay in string.
				if i < len(s) && s[i] == '\'' {
					out = append(out, '\'')
					i++
					continue
				}
				state = stateNormal
				continue
			}
			out = append(out, ch)
			i++

		// ------------------------------------------------------------------ //
		// Double-quoted identifier.
		// ------------------------------------------------------------------ //
		case stateDoubleQuote:
			if ch == '"' {
				out = append(out, '"')
				i++
				// "" escape stays inside.
				if i < len(s) && s[i] == '"' {
					out = append(out, '"')
					i++
					continue
				}
				state = stateNormal
				continue
			}
			out = append(out, ch)
			i++

		// ------------------------------------------------------------------ //
		// Dollar-quoted string: scan until the matching closing tag.
		// ------------------------------------------------------------------ //
		case stateDollarQuote:
			// Check if the closing tag starts here.
			if ch == '$' && len(s)-i >= len(dollarTag) && s[i:i+len(dollarTag)] == dollarTag {
				out = append(out, s[i:i+len(dollarTag)]...)
				i += len(dollarTag)
				state = stateNormal
				dollarTag = ""
				continue
			}
			out = append(out, ch)
			i++
		}
	}

	if len(names) == 0 {
		return sql, names, optional
	}
	return string(out), names, optional
}

// scanDollarTag checks whether the substring starting at pos in s is a
// dollar-quote opening tag of the form $[identifier]$.  It returns the tag
// string (e.g. "$fmt$") and its byte length.  If pos does not start a valid
// dollar-quote tag, tagLen is 0.
func scanDollarTag(s string, pos int) (tag string, tagLen int) {
	if pos >= len(s) || s[pos] != '$' {
		return "", 0
	}
	j := pos + 1
	// Optional identifier between the two $ signs (may be empty → $$).
	for j < len(s) && s[j] != '$' {
		if !isIdentContOrDigit(s[j]) {
			return "", 0 // not a valid tag character
		}
		j++
	}
	if j >= len(s) {
		return "", 0 // no closing $
	}
	// j points at the second $
	tag = s[pos : j+1]
	return tag, len(tag)
}

// isIdentStart reports whether b can be the first byte of a SQL identifier.
func isIdentStart(b byte) bool {
	return (b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || b == '_'
}

// isIdentCont reports whether b can continue a SQL identifier.
func isIdentCont(b byte) bool {
	return isIdentStart(b) || (b >= '0' && b <= '9')
}

// isIdentContOrDigit is used for dollar-tag scanning (allows digits after the
// first character, same rules as isIdentCont).
func isIdentContOrDigit(b byte) bool {
	return isIdentCont(b)
}

// appendUint appends the decimal representation of n to buf.
func appendUint(buf []byte, n uint32) []byte {
	if n == 0 {
		return append(buf, '0')
	}
	var tmp [10]byte
	pos := len(tmp)
	for n > 0 {
		pos--
		tmp[pos] = byte('0' + n%10)
		n /= 10
	}
	return append(buf, tmp[pos:]...)
}
