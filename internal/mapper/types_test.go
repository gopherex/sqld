package mapper

import (
	"testing"

	pg "github.com/pganalyze/pg_query_go/v6"
	"github.com/yaroher/sqld/internal/parse"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

func typeNameFor(t *testing.T, typeSQL string) *pg.TypeName {
	t.Helper()
	stmts, err := parse.Statements("CREATE TABLE t(c " + typeSQL + ");")
	if err != nil {
		t.Fatalf("parse %q: %v", typeSQL, err)
	}
	ct := stmts[0].Node.GetCreateStmt()
	col := ct.GetTableElts()[0].GetColumnDef()
	return col.GetTypeName()
}

func TestMapType(t *testing.T) {
	cases := []struct {
		sql    string
		pgName string
		kind   irv1.TypeKind
	}{
		{"int", "int4", irv1.TypeKind_TYPE_KIND_SCALAR},
		{"bigint", "int8", irv1.TypeKind_TYPE_KIND_SCALAR},
		{"varchar(20)", "varchar", irv1.TypeKind_TYPE_KIND_SCALAR},
		{"numeric(10,2)", "numeric", irv1.TypeKind_TYPE_KIND_SCALAR},
		{"int[]", "int4", irv1.TypeKind_TYPE_KIND_ARRAY},
		{"timestamptz", "timestamptz", irv1.TypeKind_TYPE_KIND_SCALAR},
	}
	for _, c := range cases {
		tn := typeNameFor(t, c.sql)
		got := MapType(tn)
		if got.GetKind() != c.kind {
			t.Errorf("%s: kind = %v, want %v", c.sql, got.GetKind(), c.kind)
		}
		name := got.GetPgName()
		if c.kind == irv1.TypeKind_TYPE_KIND_ARRAY {
			name = got.GetElement().GetPgName()
		}
		if name != c.pgName {
			t.Errorf("%s: pgName = %q, want %q", c.sql, name, c.pgName)
		}
	}
}

func TestMapTypeNumericModifier(t *testing.T) {
	got := MapType(typeNameFor(t, "numeric(10,2)"))
	m := got.GetModifier().GetNumeric()
	if m.GetPrecision() != 10 || m.GetScale() != 2 {
		t.Fatalf("numeric mod = %+v", m)
	}
}

func TestMapTypeVarcharLen(t *testing.T) {
	got := MapType(typeNameFor(t, "varchar(20)"))
	if got.GetModifier().GetText().GetLength() != 20 {
		t.Fatalf("varchar len = %+v", got.GetModifier())
	}
}

func TestMapTypeArray(t *testing.T) {
	got := MapType(typeNameFor(t, "int[]"))
	if got.GetKind() != irv1.TypeKind_TYPE_KIND_ARRAY {
		t.Fatalf("array kind = %v", got.GetKind())
	}
	if got.GetArrayDimensions() != 1 {
		t.Fatalf("array dimensions = %d, want 1", got.GetArrayDimensions())
	}
	if got.GetElement().GetPgName() != "int4" {
		t.Fatalf("array element pgName = %q, want int4", got.GetElement().GetPgName())
	}
	if got.GetElement().GetKind() != irv1.TypeKind_TYPE_KIND_SCALAR {
		t.Fatalf("array element kind = %v", got.GetElement().GetKind())
	}
}

func TestMapTypeMultiDimArray(t *testing.T) {
	got := MapType(typeNameFor(t, "int[][][]"))
	if got.GetArrayDimensions() != 3 {
		t.Fatalf("array dimensions = %d, want 3", got.GetArrayDimensions())
	}
}

func TestMapTypeTimestampWithPrecision(t *testing.T) {
	got := MapType(typeNameFor(t, "timestamp(3)"))
	dt := got.GetModifier().GetDateTime()
	if dt == nil {
		t.Fatalf("expected DateTimeModifier, got nil")
	}
	if dt.GetPrecision() != 3 {
		t.Fatalf("timestamp precision = %d, want 3", dt.GetPrecision())
	}
	if dt.GetWithTimezone() {
		t.Fatalf("timestamp should not have timezone")
	}
}

func TestMapTypeTimestamptz(t *testing.T) {
	got := MapType(typeNameFor(t, "timestamptz"))
	dt := got.GetModifier().GetDateTime()
	// No typmods were given, so modifier may be nil — that is OK.
	// But if it's present, WithTimezone must be true.
	if dt != nil && !dt.GetWithTimezone() {
		t.Fatalf("timestamptz should have WithTimezone=true, got %+v", dt)
	}
}

func TestMapTypeBpchar(t *testing.T) {
	got := MapType(typeNameFor(t, "char(5)"))
	if got.GetPgName() != "bpchar" {
		t.Fatalf("char pgName = %q, want bpchar", got.GetPgName())
	}
	if got.GetModifier().GetText().GetLength() != 5 {
		t.Fatalf("char len = %+v", got.GetModifier())
	}
}

func TestMapTypeUnknownDoesNotPanic(t *testing.T) {
	// A synthetic TypeName with an unusual name should not panic, just return SCALAR.
	tn := &pg.TypeName{
		Names: []*pg.Node{
			{Node: &pg.Node_String_{String_: &pg.String{Sval: "myschema"}}},
			{Node: &pg.Node_String_{String_: &pg.String{Sval: "mytype"}}},
		},
	}
	got := MapType(tn)
	if got.GetKind() != irv1.TypeKind_TYPE_KIND_SCALAR {
		t.Fatalf("unknown type kind = %v, want SCALAR", got.GetKind())
	}
	if got.GetPgName() != "mytype" {
		t.Fatalf("unknown type pgName = %q, want mytype", got.GetPgName())
	}
}
