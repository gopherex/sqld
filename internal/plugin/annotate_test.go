package plugin

import (
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
