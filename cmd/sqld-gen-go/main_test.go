package main

import (
	"bytes"
	"strings"
	"testing"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

func TestRunGetInfo(t *testing.T) {
	stdin := bytes.NewReader([]byte{0})
	var stdout bytes.Buffer
	if err := run(stdin, &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	var resp pluginv1.GetInfoResponse
	if err := proto.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal GetInfoResponse: %v", err)
	}
	if resp.GetName() != "go" {
		t.Errorf("GetName() = %q; want %q", resp.GetName(), "go")
	}
}

func TestRunGenerate(t *testing.T) {
	req := &pluginv1.GenerateRequest{
		OutDir: "gen/db",
		Catalog: &irv1.Catalog{
			Schemas: []*irv1.Schema{{
				Name: "public",
				Tables: []*irv1.Table{{
					Name: &irv1.QualifiedName{Name: "users"},
					Columns: []*irv1.Column{
						{Name: "id", Type: &irv1.TypeRef{PgName: "int8"}, Nullable: false},
						{Name: "email", Type: &irv1.TypeRef{PgName: "text"}, Nullable: false},
					},
				}},
			}},
		},
	}
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		t.Fatalf("marshal request: %v", err)
	}
	// Prepend tag byte 1 (Generate)
	input := append([]byte{1}, reqBytes...)
	stdin := bytes.NewReader(input)
	var stdout bytes.Buffer
	if err := run(stdin, &stdout); err != nil {
		t.Fatalf("run: %v", err)
	}
	var resp pluginv1.GenerateResponse
	if err := proto.Unmarshal(stdout.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal GenerateResponse: %v", err)
	}
	files := make(map[string]string)
	for _, f := range resp.GetFiles() {
		files[f.GetPath()] = string(f.GetContents())
	}
	modelsContent, ok := files["models.go"]
	if !ok {
		t.Fatalf("no models.go in response; got files: %v", func() []string {
			ks := make([]string, 0, len(files))
			for k := range files {
				ks = append(ks, k)
			}
			return ks
		}())
	}
	if !strings.Contains(modelsContent, "type Users struct") {
		t.Errorf("models.go does not contain 'type Users struct':\n%s", modelsContent)
	}
}
