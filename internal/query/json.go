package query

import irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"

// These SRFs emit no rows for SQL NULL input. Nullability describes an emitted
// value, not whether the input is nullable. The text element variants convert
// JSON null elements to SQL NULL; the JSON variants preserve them as JSON.
func jsonSetResult(fc *irv1.FunctionCall) (typ *irv1.TypeRef, nullable bool) {
	if fc == nil || (fc.GetName().GetSchema() != "" && fc.GetName().GetSchema() != "pg_catalog") {
		return nil, true
	}
	switch funcName(fc) {
	case "json_object_keys", "jsonb_object_keys":
		return scalarType("text"), false
	case "json_array_elements":
		return scalarType("json"), false
	case "jsonb_array_elements":
		return scalarType("jsonb"), false
	case "json_array_elements_text", "jsonb_array_elements_text":
		return scalarType("text"), true
	}
	return nil, true
}

func isJSONExtraction(symbol string) bool {
	switch symbol {
	case "->", "->>", "#>", "#>>":
		return true
	}
	return false
}

func jsonExtractionType(symbol string, left *irv1.TypeRef) *irv1.TypeRef {
	if typePgName(left) != "json" && typePgName(left) != "jsonb" {
		return nil
	}
	switch symbol {
	case "->", "#>":
		return left
	case "->>", "#>>":
		return scalarType("text")
	}
	return nil
}
