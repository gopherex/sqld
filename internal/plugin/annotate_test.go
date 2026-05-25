package plugin

import (
	"strings"
	"testing"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

func testSchema() *pluginv1.AnnotationSchema {
	return &pluginv1.AnnotationSchema{
		Sigil: "@",
		Annotations: []*pluginv1.AnnotationDef{
			{Name: "deprecated", Value: &pluginv1.AnnotationValueSpec{Form: pluginv1.AnnotationForm_ANNOTATION_FORM_FLAG}},
			{Name: "cache", Value: &pluginv1.AnnotationValueSpec{
				Form: pluginv1.AnnotationForm_ANNOTATION_FORM_KEYED,
				Fields: []*pluginv1.FieldSpec{
					{Name: "ttl", Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_DURATION},
					{Name: "region", Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_STRING},
				},
			}},
			{Name: "index", Value: &pluginv1.AnnotationValueSpec{
				Form:    pluginv1.AnnotationForm_ANNOTATION_FORM_LIST,
				Element: &pluginv1.FieldSpec{Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_IDENT},
			}},
		},
	}
}

func testSchemaWithBlock() *pluginv1.AnnotationSchema {
	return &pluginv1.AnnotationSchema{
		Sigil:         "@",
		CommentStyles: []string{"--", "/* */"},
		Annotations: []*pluginv1.AnnotationDef{
			{Name: "deprecated", Value: &pluginv1.AnnotationValueSpec{Form: pluginv1.AnnotationForm_ANNOTATION_FORM_FLAG}},
			{Name: "cache", Value: &pluginv1.AnnotationValueSpec{
				Form: pluginv1.AnnotationForm_ANNOTATION_FORM_KEYED,
				Fields: []*pluginv1.FieldSpec{
					{Name: "ttl", Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_DURATION},
					{Name: "region", Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_STRING},
				},
			}},
			{Name: "index", Value: &pluginv1.AnnotationValueSpec{
				Form:    pluginv1.AnnotationForm_ANNOTATION_FORM_LIST,
				Element: &pluginv1.FieldSpec{Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_IDENT},
			}},
			{Name: "if", Value: &pluginv1.AnnotationValueSpec{
				Form: pluginv1.AnnotationForm_ANNOTATION_FORM_POSITIONAL,
				Fields: []*pluginv1.FieldSpec{
					{Name: "condition", Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_IDENT},
				},
			}},
		},
	}
}

func TestAnnotateKeyedAndFlag(t *testing.T) {
	sql := "-- @cache ttl=30s region=eu\n-- @deprecated\nSELECT 1;\n"
	vals, err := Annotate(sql, "q.sql", testSchema())
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 2 {
		t.Fatalf("vals=%d", len(vals))
	}
	cache := vals[0]
	if cache.GetName() != "cache" || len(cache.GetArgs()) != 2 {
		t.Fatalf("cache=%+v", cache)
	}
	var ttl *irv1.AnnotationArg
	for _, a := range cache.GetArgs() {
		if a.GetName() == "ttl" {
			ttl = a
		}
	}
	if ttl == nil || ttl.GetDurationNanos() != int64(30)*1e9 {
		t.Fatalf("ttl=%+v", ttl)
	}
	if vals[1].GetName() != "deprecated" || !vals[1].GetPresent() {
		t.Fatalf("flag=%+v", vals[1])
	}
}

func TestAnnotateList(t *testing.T) {
	vals, _ := Annotate("-- @index a,b,c\nSELECT 1;\n", "q.sql", testSchema())
	if len(vals) != 1 || len(vals[0].GetArgs()) != 3 {
		t.Fatalf("index=%+v", vals)
	}
	if vals[0].GetArgs()[0].GetStringValue() != "a" {
		t.Fatalf("arg0=%+v", vals[0].GetArgs()[0])
	}
}

// TestAnnotateLineCommentByteOffsets verifies that line comment AnnotationValues
// have non-zero StartOffset and EndOffset, and that the slice sql[start:end]
// contains the comment sigil+name.
func TestAnnotateLineCommentByteOffsets(t *testing.T) {
	sql := "SELECT 1;\n-- @deprecated\nSELECT 2;\n"
	vals, err := Annotate(sql, "q.sql", testSchema())
	if err != nil {
		t.Fatal(err)
	}
	if len(vals) != 1 {
		t.Fatalf("expected 1 val, got %d", len(vals))
	}
	src := vals[0].GetSource()
	if src == nil {
		t.Fatal("Source is nil")
	}
	start := src.GetStartOffset()
	end := src.GetEndOffset()
	if end <= start {
		t.Fatalf("EndOffset(%d) <= StartOffset(%d)", end, start)
	}
	slice := sql[start:end]
	if !strings.Contains(slice, "@deprecated") {
		t.Fatalf("sql[%d:%d] = %q does not contain @deprecated", start, end, slice)
	}
}

// TestAnnotateBlockComment verifies that a block comment /*@if name*/ produces
// an AnnotationValue with name=="if", one IDENT arg "name", and correct byte
// offsets such that sql[StartOffset:EndOffset] contains "@if".
func TestAnnotateBlockComment(t *testing.T) {
	sql := "SELECT * FROM users WHERE /*@if active*/ status = 'active' /*@endif*/;"
	schema := testSchemaWithBlock()
	// Add endif as FLAG.
	schema.Annotations = append(schema.Annotations, &pluginv1.AnnotationDef{
		Name:  "endif",
		Value: &pluginv1.AnnotationValueSpec{Form: pluginv1.AnnotationForm_ANNOTATION_FORM_FLAG},
	})

	vals, err := Annotate(sql, "q.sql", schema)
	if err != nil {
		t.Fatal(err)
	}

	// We expect @if and @endif.
	if len(vals) != 2 {
		t.Fatalf("expected 2 vals, got %d: %v", len(vals), vals)
	}

	ifVal := vals[0]
	if ifVal.GetName() != "if" {
		t.Fatalf("expected name='if', got %q", ifVal.GetName())
	}
	if len(ifVal.GetArgs()) != 1 {
		t.Fatalf("expected 1 arg for @if, got %d", len(ifVal.GetArgs()))
	}
	condArg := ifVal.GetArgs()[0]
	if condArg.GetStringValue() != "active" {
		t.Fatalf("expected condition='active', got %q", condArg.GetStringValue())
	}

	src := ifVal.GetSource()
	if src == nil {
		t.Fatal("Source is nil for @if")
	}
	start := src.GetStartOffset()
	end := src.GetEndOffset()
	if end <= start {
		t.Fatalf("EndOffset(%d) <= StartOffset(%d)", end, start)
	}
	slice := sql[start:end]
	if !strings.Contains(slice, "@if") {
		t.Fatalf("sql[%d:%d] = %q does not contain @if", start, end, slice)
	}
	// The slice should be the full block comment "/*@if active*/".
	if slice != "/*@if active*/" {
		t.Fatalf("expected sql[start:end]=%q, got %q", "/*@if active*/", slice)
	}

	// Verify @endif has valid offsets too.
	endifVal := vals[1]
	if endifVal.GetName() != "endif" {
		t.Fatalf("expected name='endif', got %q", endifVal.GetName())
	}
	endifSrc := endifVal.GetSource()
	if endifSrc == nil {
		t.Fatal("Source is nil for @endif")
	}
	endifSlice := sql[endifSrc.GetStartOffset():endifSrc.GetEndOffset()]
	if endifSlice != "/*@endif*/" {
		t.Fatalf("expected endif slice=%q, got %q", "/*@endif*/", endifSlice)
	}
}

// TestAnnotateBlockCommentOnlyStyle verifies that when CommentStyles contains
// only "/* */", line comments are NOT scanned.
func TestAnnotateBlockCommentOnlyStyle(t *testing.T) {
	sql := "-- @deprecated\n/*@deprecated*/"
	schema := &pluginv1.AnnotationSchema{
		Sigil:         "@",
		CommentStyles: []string{"/* */"},
		Annotations: []*pluginv1.AnnotationDef{
			{Name: "deprecated", Value: &pluginv1.AnnotationValueSpec{Form: pluginv1.AnnotationForm_ANNOTATION_FORM_FLAG}},
		},
	}
	vals, err := Annotate(sql, "q.sql", schema)
	if err != nil {
		t.Fatal(err)
	}
	// Only the block comment should match.
	if len(vals) != 1 {
		t.Fatalf("expected 1 val (block only), got %d", len(vals))
	}
	src := vals[0].GetSource()
	slice := sql[src.GetStartOffset():src.GetEndOffset()]
	if slice != "/*@deprecated*/" {
		t.Fatalf("expected block comment slice, got %q", slice)
	}
}
