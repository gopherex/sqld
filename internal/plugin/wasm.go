package plugin

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

// wasmRunner invokes a plugin compiled as a WASI command module (.wasm) using
// wazero. It reuses the identical stdio-framed protobuf protocol as binaryRunner:
//
//	stdin  → [1 byte method tag] [serialized proto request]
//	stdout ← [serialized proto response]
//
// Each RPC instantiates a fresh module so the plugin's main() runs from scratch
// every call. The compiled module is cached to avoid redundant decoding.
type wasmRunner struct {
	args     []string
	env      map[string]string
	rt       wazero.Runtime
	compiled wazero.CompiledModule
}

// newWasmRunner reads the .wasm file at path, creates a wazero runtime, and
// pre-compiles the module for reuse across calls.
func newWasmRunner(path string, args []string, env map[string]string) (*wasmRunner, error) {
	wasmBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("wasm runner: read %q: %w", path, err)
	}

	ctx := context.Background()

	rt := wazero.NewRuntime(ctx)

	// Install WASI so the module can call proc_exit, fd_write, etc.
	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	compiled, err := rt.CompileModule(ctx, wasmBytes)
	if err != nil {
		_ = rt.Close(ctx)
		return nil, fmt.Errorf("wasm runner: compile module: %w", err)
	}

	return &wasmRunner{
		args:     args,
		env:      env,
		rt:       rt,
		compiled: compiled,
	}, nil
}

// call runs one RPC by instantiating the compiled module with stdin wired to
// [tag | reqBytes] and capturing stdout as the response.
//
// WASI commands exit via proc_exit(0) on success; wazero surfaces that as a
// *sys.ExitError with ExitCode() == 0, which we treat as success.
func (w *wasmRunner) call(ctx context.Context, tag byte, reqBytes []byte) ([]byte, error) {
	stdin := append([]byte{tag}, reqBytes...)

	var stdout bytes.Buffer

	cfg := wazero.NewModuleConfig().
		WithStdin(bytes.NewReader(stdin)).
		WithStdout(&stdout).
		WithStderr(os.Stderr).
		WithArgs(append([]string{"plugin"}, w.args...)...)

	for k, v := range w.env {
		cfg = cfg.WithEnv(k, v)
	}

	// Use an empty module name so multiple instantiations don't collide.
	cfg = cfg.WithName("")

	mod, err := w.rt.InstantiateModule(ctx, w.compiled, cfg)
	if mod != nil {
		_ = mod.Close(ctx)
	}
	if err != nil {
		var exitErr *sys.ExitError
		if errors.As(err, &exitErr) {
			if exitErr.ExitCode() == 0 {
				// Normal WASI proc_exit(0) — success.
				return stdout.Bytes(), nil
			}
			return nil, fmt.Errorf("wasm plugin exited with code %d", exitErr.ExitCode())
		}
		return nil, fmt.Errorf("wasm plugin instantiate: %w", err)
	}

	return stdout.Bytes(), nil
}

// GetInfo implements Runner.
func (w *wasmRunner) GetInfo(ctx context.Context) (*pluginv1.GetInfoResponse, error) {
	reqBytes, err := proto.Marshal(&pluginv1.GetInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("marshal GetInfoRequest: %w", err)
	}

	out, err := w.call(ctx, methodGetInfo, reqBytes)
	if err != nil {
		return nil, err
	}

	var resp pluginv1.GetInfoResponse
	if err := proto.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal GetInfoResponse: %w", err)
	}
	return &resp, nil
}

// Generate implements Runner.
func (w *wasmRunner) Generate(ctx context.Context, req *pluginv1.GenerateRequest) (*pluginv1.GenerateResponse, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal GenerateRequest: %w", err)
	}

	out, err := w.call(ctx, methodGenerate, reqBytes)
	if err != nil {
		return nil, err
	}

	var resp pluginv1.GenerateResponse
	if err := proto.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal GenerateResponse: %w", err)
	}
	return &resp, nil
}

// Close implements Runner. Closing the runtime also closes the compiled module.
func (w *wasmRunner) Close() error {
	return w.rt.Close(context.Background())
}
