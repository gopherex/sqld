package query

import (
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	"google.golang.org/protobuf/proto"
)

// An uncast string/NULL literal in a set-operation input is PostgreSQL's
// unknown type, not text. Resolve each pair before its enclosing UNION, as PG
// does: (NULL UNION NULL) UNION uuid must not be treated as NULL UNION uuid.
func setResultType(left, right *irv1.TypeRef, leftUnknown, rightUnknown bool) *irv1.TypeRef {
	switch {
	case leftUnknown && rightUnknown:
		return scalarType("text")
	case leftUnknown:
		return right
	case rightUnknown:
		return left
	case left == nil || right == nil:
		return nil
	case proto.Equal(left, right):
		return left
	}
	if left.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY && right.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
		if elem := setResultType(left.GetElement(), right.GetElement(), false, false); elem != nil {
			return arrayType(elem)
		}
		return nil
	}
	if incompatibleTypes(left, right) {
		return nil
	}
	l, r := left.GetPgName(), right.GetPgName()
	if l == r {
		return left
	}
	if numericRank(l) > 0 && numericRank(r) > 0 {
		if numericRank(r) > numericRank(l) {
			return right
		}
		return left
	}
	dateRank := map[string]int{"date": 1, "timestamp": 2, "timestamptz": 3}
	if dateRank[l] > 0 && dateRank[r] > 0 {
		if dateRank[r] > dateRank[l] {
			return right
		}
		return left
	}
	// text is preferred within the string category; varchar/bpchar otherwise
	// retain the first candidate because both implicit casts are available.
	if (l == "text" || l == "varchar" || l == "bpchar") && (r == "text" || r == "varchar" || r == "bpchar") {
		if r == "text" {
			return right
		}
		return left
	}
	return nil
}
