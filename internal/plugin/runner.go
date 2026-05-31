package plugin

import (
	"context"
	"fmt"

	"github.com/gopherex/sqld/pkg/config"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

// Method tags written as the first stdin byte when invoking a plugin process.
// The plugin reads this byte to dispatch to the correct handler.
// Task 16 (wasm) must speak the same tag values via its own transport.
const (
	methodGetInfo  byte = 0
	methodGenerate byte = 1
)

// Runner drives a single generator plugin regardless of transport.
type Runner interface {
	// GetInfo retrieves metadata about the plugin (name, version, capabilities).
	GetInfo(ctx context.Context) (*pluginv1.GetInfoResponse, error)

	// Generate invokes the plugin's code-generation logic.
	Generate(ctx context.Context, req *pluginv1.GenerateRequest) (*pluginv1.GenerateResponse, error)

	// Close releases any resources held by the runner.
	Close() error
}

// Open constructs a Runner for the plugin described by pc.
// Transport is selected based on which of pc.Binary/Command/Wasm is non-empty:
//   - Binary  → stdio-framed protobuf over an exec'd process (binaryRunner).
//   - Command → same as Binary but the field name differs.
//   - Wasm    → WASI module via wazero (wasmRunner).
func Open(pc config.PluginConfig) (Runner, error) {
	switch {
	case pc.Binary != "":
		return newBinaryRunner(pc.Binary, pc.Args, pc.Env), nil
	case pc.Command != "":
		return newBinaryRunner(pc.Command, pc.Args, pc.Env), nil
	case pc.Wasm != "":
		return newWasmRunner(pc.Wasm, pc.Args, pc.Env)
	default:
		return nil, fmt.Errorf("plugin %q: no source location set (set binary, command, or wasm)", pc.Name)
	}
}
