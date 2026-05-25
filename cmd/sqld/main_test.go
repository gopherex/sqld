package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConfigYAML = `sql:
  - inline: "CREATE TABLE users(id bigint primary key, email text not null);"
    kind: SQL_KIND_SCHEMA
`

func TestCollectCommand(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "sqld.yaml")
	if err := os.WriteFile(cfgPath, []byte(testConfigYAML), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var out, errBuf bytes.Buffer
	code := run([]string{"collect", "-c", cfgPath}, &out, &errBuf)
	if code != 0 {
		t.Fatalf("run returned %d; stderr: %s", code, errBuf.String())
	}
	outStr := out.String()
	if !strings.Contains(outStr, "users") {
		t.Errorf("stdout does not contain 'users':\n%s", outStr)
	}
}

func TestUsageOnNoArgs(t *testing.T) {
	var out, errBuf bytes.Buffer
	code := run(nil, &out, &errBuf)
	if code != 2 {
		t.Errorf("run(nil) returned %d; want 2", code)
	}
}
