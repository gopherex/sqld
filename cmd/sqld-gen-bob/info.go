package main

import (
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// Info reports the plugin's identity and capabilities. sqld-gen-bob generates
// the ORM from the schema structure only, so it declares no comment-annotation
// grammar (queries — and their @-annotations — stay with sqld-gen-go).
func Info() *pluginv1.GetInfoResponse {
	return &pluginv1.GetInfoResponse{
		Name:             "bob",
		Version:          "0.1.0",
		SupportedEngines: []irv1.Engine{irv1.Engine_ENGINE_POSTGRESQL},
	}
}
