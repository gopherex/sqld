// Package plugin provides host-side utilities for working with sqld plugins.
package plugin

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

// wantLineComments reports whether the schema's CommentStyles includes "--".
// When CommentStyles is empty we default to scanning "--" for backwards
// compatibility.
func wantLineComments(schema *pluginv1.AnnotationSchema) bool {
	styles := schema.GetCommentStyles()
	if len(styles) == 0 {
		return true
	}
	for _, s := range styles {
		if s == "--" {
			return true
		}
	}
	return false
}

// wantBlockComments reports whether the schema's CommentStyles includes "/* */".
func wantBlockComments(schema *pluginv1.AnnotationSchema) bool {
	for _, s := range schema.GetCommentStyles() {
		if s == "/* */" {
			return true
		}
	}
	return false
}

// commentCandidate holds the inner text of a comment together with the byte
// offsets of the comment delimiters in the original sql string.
type commentCandidate struct {
	text        string // inner text, stripped of delimiters and leading whitespace
	startOffset int    // byte index of opening delimiter ("--" or "/*")
	endOffset   int    // byte index just past the closing delimiter
	line        int    // 1-based line number of the opening delimiter
}

// scanComments extracts all comment candidates from sql according to the
// enabled comment styles.
func scanComments(sql string, schema *pluginv1.AnnotationSchema) []commentCandidate {
	var results []commentCandidate

	doLine := wantLineComments(schema)
	doBlock := wantBlockComments(schema)

	// Walk the string byte by byte to find comment openings.  We track the
	// current line number so we can populate SourceSpan.StartLine.
	line := 1
	i := 0
	n := len(sql)
	for i < n {
		// Track newlines for line counting.
		if sql[i] == '\n' {
			line++
			i++
			continue
		}

		// Check for line comment "--".
		if doLine && i+1 < n && sql[i] == '-' && sql[i+1] == '-' {
			startOff := i
			startLine := line
			// Advance past "--"
			i += 2
			// Consume until end of line (but not the newline itself).
			for i < n && sql[i] != '\n' {
				i++
			}
			// raw text is everything from startOff to i.
			endOff := i
			inner := sql[startOff+2 : endOff] // strip "--"
			// Optionally strip a single leading space.
			if len(inner) > 0 && inner[0] == ' ' {
				inner = inner[1:]
			}
			results = append(results, commentCandidate{
				text:        inner,
				startOffset: startOff,
				endOffset:   endOff,
				line:        startLine,
			})
			continue
		}

		// Check for block comment "/* ... */".
		if doBlock && i+1 < n && sql[i] == '/' && sql[i+1] == '*' {
			startOff := i
			startLine := line
			// Advance past "/*"
			i += 2
			// Find closing "*/".
			closeIdx := strings.Index(sql[i:], "*/")
			var inner string
			var endOff int
			if closeIdx < 0 {
				// Unterminated block comment: consume to end of string.
				inner = sql[i:]
				endOff = n
				i = n
			} else {
				inner = sql[i : i+closeIdx]
				endOff = i + closeIdx + 2 // just past "*/"
				// Count newlines inside the block comment for line tracking.
				for _, ch := range sql[i : i+closeIdx+2] {
					if ch == '\n' {
						line++
					}
				}
				i = endOff
			}
			// Strip leading/trailing whitespace from the inner text.
			inner = strings.TrimSpace(inner)
			results = append(results, commentCandidate{
				text:        inner,
				startOffset: startOff,
				endOffset:   endOff,
				line:        startLine,
			})
			continue
		}

		i++
	}

	return results
}

// Annotate scans sql for SQL comments that match entries in schema, and returns
// a slice of typed AnnotationValues in document order.  It never panics;
// malformed comments are skipped or parsed best-effort.
//
// Supported comment styles are controlled by schema.CommentStyles:
//   - "--"    line comments  (default when CommentStyles is empty)
//   - "/* */" block comments
//
// Every produced AnnotationValue has Source.StartOffset/EndOffset set to the
// byte range of the comment within sql (StartOffset = index of "--" or "/*",
// EndOffset = index just past end-of-line or "*/").
func Annotate(sql, sourceFile string, schema *pluginv1.AnnotationSchema) ([]*irv1.AnnotationValue, error) {
	if schema == nil {
		return nil, nil
	}

	sigil := schema.GetSigil()
	var results []*irv1.AnnotationValue

	for _, cand := range scanComments(sql, schema) {
		text := strings.TrimSpace(cand.text)
		if text == "" {
			continue
		}

		// Strip sigil and extract candidate name.
		if sigil != "" {
			if !strings.HasPrefix(text, sigil) {
				continue
			}
			text = text[len(sigil):]
		}

		// Split into name token and remainder.
		name, remainder := splitFirst(text)
		if name == "" {
			continue
		}

		// Match against annotation defs.
		def := matchDef(schema, name)
		if def == nil {
			continue
		}

		source := &irv1.SourceSpan{
			File:        sourceFile,
			StartLine:   uint32(cand.line),
			StartOffset: uint64(cand.startOffset),
			EndOffset:   uint64(cand.endOffset),
		}

		// Determine target kind.
		targetKind := irv1.AnnotationTargetKind_ANNOTATION_TARGET_KIND_QUERY
		if len(def.GetTargets()) > 0 {
			targetKind = def.GetTargets()[0]
		}

		target := &irv1.AnnotationTargetRef{
			Kind:   targetKind,
			Source: source,
		}

		// Full raw text is the original annotation text (with sigil).
		rawText := sigil + name
		if remainder != "" {
			rawText = rawText + " " + remainder
		}

		av := &irv1.AnnotationValue{
			Name:   def.GetName(),
			Target: target,
			Raw:    rawText,
			Source: source,
		}

		// Parse args based on form.
		spec := def.GetValue()
		if spec == nil {
			// No spec: treat as FLAG.
			av.Present = true
			results = append(results, av)
			continue
		}

		args, present := parseArgs(spec, remainder)
		av.Args = args
		av.Present = present

		results = append(results, av)
	}

	return results, nil
}

// splitFirst splits s into the first whitespace-delimited token and the rest.
func splitFirst(s string) (token, rest string) {
	s = strings.TrimSpace(s)
	idx := strings.IndexFunc(s, unicode.IsSpace)
	if idx < 0 {
		return s, ""
	}
	return s[:idx], strings.TrimSpace(s[idx+1:])
}

// matchDef finds the AnnotationDef whose Name or Aliases match candidate,
// respecting schema.CaseInsensitive.
func matchDef(schema *pluginv1.AnnotationSchema, candidate string) *pluginv1.AnnotationDef {
	ci := schema.GetCaseInsensitive()
	cmp := candidate
	if ci {
		cmp = strings.ToLower(candidate)
	}
	for _, def := range schema.GetAnnotations() {
		defName := def.GetName()
		if ci {
			defName = strings.ToLower(defName)
		}
		if defName == cmp {
			return def
		}
		for _, alias := range def.GetAliases() {
			a := alias
			if ci {
				a = strings.ToLower(a)
			}
			if a == cmp {
				return def
			}
		}
	}
	return nil
}

// parseArgs parses the remainder string according to the AnnotationValueSpec form.
// Returns the args slice and the present flag (for FLAG form).
func parseArgs(spec *pluginv1.AnnotationValueSpec, remainder string) ([]*irv1.AnnotationArg, bool) {
	switch spec.GetForm() {
	case pluginv1.AnnotationForm_ANNOTATION_FORM_FLAG,
		pluginv1.AnnotationForm_ANNOTATION_FORM_UNSPECIFIED:
		// FLAG: no args, just presence.
		return nil, true

	case pluginv1.AnnotationForm_ANNOTATION_FORM_SCALAR:
		if remainder == "" {
			return nil, false
		}
		arg := coerceArg("", 0, spec.GetScalarType(), remainder, nil)
		return []*irv1.AnnotationArg{arg}, false

	case pluginv1.AnnotationForm_ANNOTATION_FORM_POSITIONAL:
		return parsePositional(spec, remainder), false

	case pluginv1.AnnotationForm_ANNOTATION_FORM_KEYED:
		return parseKeyed(spec, remainder), false

	case pluginv1.AnnotationForm_ANNOTATION_FORM_LIST:
		return parseList(spec, remainder), false

	case pluginv1.AnnotationForm_ANNOTATION_FORM_FREEFORM:
		if remainder == "" {
			return nil, false
		}
		arg := &irv1.AnnotationArg{
			Type:  irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_STRING,
			Value: &irv1.AnnotationArg_StringValue{StringValue: remainder},
			Raw:   remainder,
		}
		return []*irv1.AnnotationArg{arg}, false

	default:
		return nil, false
	}
}

// parsePositional splits remainder by whitespace and assigns each token to
// the corresponding FieldSpec. Variadic fields consume the remainder.
func parsePositional(spec *pluginv1.AnnotationValueSpec, remainder string) []*irv1.AnnotationArg {
	fields := spec.GetFields()
	if len(fields) == 0 || remainder == "" {
		return nil
	}

	tokens := strings.Fields(remainder)
	var args []*irv1.AnnotationArg

	for i, field := range fields {
		if i >= len(tokens) {
			break
		}
		var token string
		if field.GetVariadic() && i < len(fields)-1 {
			// Only the last field should be variadic, but handle gracefully.
			token = strings.Join(tokens[i:], " ")
		} else if field.GetVariadic() {
			token = strings.Join(tokens[i:], " ")
		} else {
			token = tokens[i]
		}
		arg := coerceArg(field.GetName(), uint32(i), field.GetType(), token, field.GetEnumValues())
		args = append(args, arg)
		if field.GetVariadic() {
			break
		}
	}

	return args
}

// parseKeyed splits remainder into key=value pairs separated by PairSeparator
// (default: whitespace), then for each pair finds the matching FieldSpec.
func parseKeyed(spec *pluginv1.AnnotationValueSpec, remainder string) []*irv1.AnnotationArg {
	if remainder == "" {
		return nil
	}

	kvSep := spec.GetKvSeparator()
	if kvSep == "" {
		kvSep = "="
	}

	// Build a field lookup map.
	fieldMap := make(map[string]*pluginv1.FieldSpec)
	for _, f := range spec.GetFields() {
		fieldMap[strings.ToLower(f.GetName())] = f
	}

	// Split into pairs. PairSeparator default = whitespace.
	var pairs []string
	pairSep := spec.GetPairSeparator()
	if pairSep == "" {
		pairs = strings.Fields(remainder)
	} else {
		pairs = strings.Split(remainder, pairSep)
	}

	var args []*irv1.AnnotationArg
	for _, pair := range pairs {
		pair = strings.TrimSpace(pair)
		if pair == "" {
			continue
		}
		sepIdx := strings.Index(pair, kvSep)
		if sepIdx < 0 {
			// No kv separator found; treat whole token as a flag-style bare key with empty value.
			arg := &irv1.AnnotationArg{
				Name:  pair,
				Type:  irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_STRING,
				Value: &irv1.AnnotationArg_StringValue{StringValue: ""},
				Raw:   pair,
			}
			args = append(args, arg)
			continue
		}
		key := pair[:sepIdx]
		val := pair[sepIdx+len(kvSep):]

		// Find matching field (case-insensitive key lookup).
		field := fieldMap[strings.ToLower(key)]
		var argType irv1.AnnotationArgType
		var enumValues []string
		if field != nil {
			argType = field.GetType()
			enumValues = field.GetEnumValues()
		} else {
			argType = irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_STRING
		}

		arg := coerceArg(key, 0, argType, val, enumValues)
		args = append(args, arg)
	}

	return args
}

// parseList splits remainder by ItemSeparator (default: ",") and coerces each
// element to Element.Type.
func parseList(spec *pluginv1.AnnotationValueSpec, remainder string) []*irv1.AnnotationArg {
	if remainder == "" {
		return nil
	}

	sep := spec.GetItemSeparator()
	if sep == "" {
		sep = ","
	}

	elem := spec.GetElement()
	var elemType irv1.AnnotationArgType
	var enumValues []string
	if elem != nil {
		elemType = elem.GetType()
		enumValues = elem.GetEnumValues()
	} else {
		elemType = irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_STRING
	}

	items := strings.Split(remainder, sep)
	var args []*irv1.AnnotationArg
	for i, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		arg := coerceArg("", uint32(i), elemType, item, enumValues)
		args = append(args, arg)
	}

	return args
}

// coerceArg converts a raw string token to the appropriate AnnotationArg
// typed value. On coercion failure it falls back to StringValue.
func coerceArg(name string, index uint32, argType irv1.AnnotationArgType, raw string, enumValues []string) *irv1.AnnotationArg {
	arg := &irv1.AnnotationArg{
		Name:  name,
		Index: index,
		Type:  argType,
		Raw:   raw,
	}

	switch argType {
	case irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_INT:
		v, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			arg.Value = &irv1.AnnotationArg_IntValue{IntValue: v}
			return arg
		}
		// fallback
		arg.Value = &irv1.AnnotationArg_StringValue{StringValue: raw}

	case irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_FLOAT:
		v, err := strconv.ParseFloat(raw, 64)
		if err == nil {
			arg.Value = &irv1.AnnotationArg_FloatValue{FloatValue: v}
			return arg
		}
		arg.Value = &irv1.AnnotationArg_StringValue{StringValue: raw}

	case irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_BOOL:
		v, err := strconv.ParseBool(raw)
		if err == nil {
			arg.Value = &irv1.AnnotationArg_BoolValue{BoolValue: v}
			return arg
		}
		arg.Value = &irv1.AnnotationArg_StringValue{StringValue: raw}

	case irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_DURATION:
		d, err := time.ParseDuration(raw)
		if err == nil {
			arg.Value = &irv1.AnnotationArg_DurationNanos{DurationNanos: d.Nanoseconds()}
			return arg
		}
		arg.Value = &irv1.AnnotationArg_StringValue{StringValue: raw}

	case irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_ENUM:
		// Validate against enum values; still emit StringValue either way.
		_ = enumValues // validation is informational in v1; emit regardless
		arg.Value = &irv1.AnnotationArg_StringValue{StringValue: raw}

	case irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_STRING,
		irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_IDENT,
		irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_UNSPECIFIED:
		fallthrough
	default:
		arg.Value = &irv1.AnnotationArg_StringValue{StringValue: raw}
	}

	return arg
}
