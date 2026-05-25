// Package plugin provides host-side utilities for working with sqld plugins.
package plugin

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// Annotate scans sql line by line for SQL comments that match entries in
// schema, and returns a slice of typed AnnotationValues in document order.
// It never panics; malformed comments are skipped or parsed best-effort.
func Annotate(sql, sourceFile string, schema *pluginv1.AnnotationSchema) ([]*irv1.AnnotationValue, error) {
	if schema == nil {
		return nil, nil
	}

	var results []*irv1.AnnotationValue

	lines := splitLines(sql)
	for lineIdx, line := range lines {
		lineNo := lineIdx + 1 // 1-based

		commentText, ok := extractLineComment(line)
		if !ok {
			continue
		}

		commentText = strings.TrimSpace(commentText)
		if commentText == "" {
			continue
		}

		// Strip sigil and extract candidate name.
		sigil := schema.GetSigil()
		if sigil != "" {
			if !strings.HasPrefix(commentText, sigil) {
				continue
			}
			commentText = commentText[len(sigil):]
		}

		// Split into name token and remainder.
		name, remainder := splitFirst(commentText)
		if name == "" {
			continue
		}

		// Match against annotation defs.
		def := matchDef(schema, name)
		if def == nil {
			continue
		}

		source := &irv1.SourceSpan{
			File:      sourceFile,
			StartLine: uint32(lineNo),
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

		// Full raw text is the original comment text (with sigil included if any).
		rawText := schema.GetSigil() + name
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

// splitLines splits sql into lines, preserving empty trailing lines.
func splitLines(sql string) []string {
	return strings.Split(sql, "\n")
}

// extractLineComment checks if the trimmed line starts with "--" and returns
// the text after "-- " (or after "--"), along with a boolean ok.
func extractLineComment(line string) (string, bool) {
	trimmed := strings.TrimLeftFunc(line, unicode.IsSpace)
	if !strings.HasPrefix(trimmed, "--") {
		return "", false
	}
	rest := trimmed[2:] // strip "--"
	// Optionally strip a single leading space.
	if len(rest) > 0 && rest[0] == ' ' {
		rest = rest[1:]
	}
	return rest, true
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
