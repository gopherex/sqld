package plugin

import (
	"context"
	"fmt"

	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
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
// Transport is selected based on pc.Source.Location:
//   - Binary / Command → stdio framing over an exec'd process (binaryRunner).
//   - Wasm             → not yet implemented (Task 16).
func Open(pc *configv1.PluginConfig) (Runner, error) {
	src := pc.GetSource()
	if src == nil {
		return nil, fmt.Errorf("plugin %q: source is nil", pc.GetName())
	}
	switch loc := src.GetLocation().(type) {
	case *configv1.PluginSource_Binary:
		return newBinaryRunner(loc.Binary, src.GetArgs(), pc.GetEnv()), nil
	case *configv1.PluginSource_Command:
		return newBinaryRunner(loc.Command, src.GetArgs(), pc.GetEnv()), nil
	case *configv1.PluginSource_Wasm:
		return nil, fmt.Errorf("plugin %q: wasm transport not yet implemented (Task 16)", pc.GetName())
	default:
		return nil, fmt.Errorf("plugin %q: no source location set", pc.GetName())
	}
}
