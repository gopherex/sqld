// Package gotypes maps PostgreSQL types to Go types for sqld code generators.
// It is the single source of truth shared by sqld-gen-go and sqld-gen-bob so
// both plugins emit identical Go types for the same catalog.
package gotypes

import (
	"strings"
	"unicode"
)

// initialisms that should be fully uppercased in Go identifiers.
var initialisms = map[string]string{
	"id":   "ID",
	"url":  "URL",
	"api":  "API",
	"sql":  "SQL",
	"http": "HTTP",
	"json": "JSON",
	"uuid": "UUID",
	"ip":   "IP",
	"xml":  "XML",
}

// Pascal converts a snake_case name to PascalCase applying Go initialisms.
func Pascal(s string) string {
	parts := splitWords(s)
	var b strings.Builder
	for _, p := range parts {
		lower := strings.ToLower(p)
		if up, ok := initialisms[lower]; ok {
			b.WriteString(up)
		} else {
			b.WriteString(capitalize(p))
		}
	}
	return b.String()
}

// LowerCamel converts to lowerCamelCase.
func LowerCamel(s string) string {
	parts := splitWords(s)
	var b strings.Builder
	for i, p := range parts {
		lower := strings.ToLower(p)
		if i == 0 {
			b.WriteString(lower)
		} else {
			if up, ok := initialisms[lower]; ok {
				b.WriteString(up)
			} else {
				b.WriteString(capitalize(p))
			}
		}
	}
	return b.String()
}

// splitWords splits on underscore boundaries and camelCase transitions.
// "get_user" → ["get","user"], "GetUser" → ["Get","User"], "getUserID" → ["get","User","ID"]
func splitWords(s string) []string {
	// first split on underscores
	underParts := strings.Split(s, "_")
	var out []string
	for _, p := range underParts {
		if p == "" {
			continue
		}
		// Then split camelCase
		out = append(out, splitCamel(p)...)
	}
	if len(out) == 0 {
		return []string{s}
	}
	return out
}

// splitCamel splits a camelCase or PascalCase string into words.
// "GetUser" → ["Get","User"], "getUserID" → ["get","User","ID"]
func splitCamel(s string) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	var words []string
	start := 0
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) {
			// Check if previous char is lowercase (transition Lo→Up) or
			// next char is lowercase and current run is uppercase (acronym end)
			if unicode.IsLower(runes[i-1]) {
				words = append(words, string(runes[start:i]))
				start = i
			} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) && i-start > 1 {
				words = append(words, string(runes[start:i]))
				start = i
			}
		}
	}
	words = append(words, string(runes[start:]))
	return words
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(unicode.ToUpper(r[0])) + strings.ToLower(string(r[1:]))
}
