package plugin

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

// binaryRunner invokes a plugin executable once per RPC using stdio-framed
// protobuf messages:
//
//	stdin  → [1 byte method tag] [serialized proto request]
//	stdout ← [serialized proto response]
//	stderr ← captured for error diagnostics only
//
// This is stdio-framed protobuf, NOT full gRPC-over-stdio. Task 16 (wasm)
// must use the same method tag values (methodGetInfo=0, methodGenerate=1).
type binaryRunner struct {
	path string
	args []string
	env  map[string]string
}

// newBinaryRunner returns a binaryRunner for the given executable path,
// extra arguments, and environment overrides.
func newBinaryRunner(path string, args []string, env map[string]string) *binaryRunner {
	return &binaryRunner{
		path: path,
		args: args,
		env:  env,
	}
}

// call executes the plugin process for a single RPC.
// It writes [tag | reqBytes] to stdin, collects stdout as the response, and
// captures stderr for error context. The process must exit 0.
func (b *binaryRunner) call(ctx context.Context, tag byte, reqBytes []byte) ([]byte, error) {
	cmd := exec.CommandContext(ctx, b.path, b.args...)

	// Build environment: inherit host env, then apply plugin overrides.
	envBase := os.Environ()
	for k, v := range b.env {
		envBase = append(envBase, k+"="+v)
	}
	cmd.Env = envBase

	// stdin = [tag byte] + serialized request
	stdin := make([]byte, 1+len(reqBytes))
	stdin[0] = tag
	copy(stdin[1:], reqBytes)
	cmd.Stdin = bytes.NewReader(stdin)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrText := strings.TrimSpace(stderr.String())
		if stderrText != "" {
			return nil, fmt.Errorf("plugin %q exited with error: %w\nstderr: %s", b.path, err, stderrText)
		}
		return nil, fmt.Errorf("plugin %q exited with error: %w", b.path, err)
	}

	return stdout.Bytes(), nil
}

// GetInfo implements Runner.
func (b *binaryRunner) GetInfo(ctx context.Context) (*pluginv1.GetInfoResponse, error) {
	reqBytes, err := proto.Marshal(&pluginv1.GetInfoRequest{})
	if err != nil {
		return nil, fmt.Errorf("marshal GetInfoRequest: %w", err)
	}

	out, err := b.call(ctx, methodGetInfo, reqBytes)
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
func (b *binaryRunner) Generate(ctx context.Context, req *pluginv1.GenerateRequest) (*pluginv1.GenerateResponse, error) {
	reqBytes, err := proto.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal GenerateRequest: %w", err)
	}

	out, err := b.call(ctx, methodGenerate, reqBytes)
	if err != nil {
		return nil, err
	}

	var resp pluginv1.GenerateResponse
	if err := proto.Unmarshal(out, &resp); err != nil {
		return nil, fmt.Errorf("unmarshal GenerateResponse: %w", err)
	}
	return &resp, nil
}

// Close implements Runner. Binary plugins are stateless (one process per call),
// so there are no resources to release.
func (b *binaryRunner) Close() error {
	return nil
}
