// Command sqld-gen-bob is an sqld code-generation plugin that drives
// stephenafamo/bob's generator from sqld's protobuf IR (*irv1.Catalog),
// producing a full Go ORM (models, relationships, factories, where/loaders/
// joins) whose Go types are the same canonical types sqld-gen-go emits.
//
// It speaks the sqld plugin stdio transport: a single tag byte (0=GetInfo,
// 1=Generate) followed by the serialized request; the serialized response is
// written to stdout.
package main

import (
	"fmt"
	"io"
	"os"

	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

func run(stdin io.Reader, stdout io.Writer) error {
	in, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if len(in) == 0 {
		return fmt.Errorf("empty request")
	}
	tag, payload := in[0], in[1:]
	var out proto.Message
	switch tag {
	case 0: // GetInfo
		out = Info()
	case 1: // Generate
		req := &pluginv1.GenerateRequest{}
		if err := proto.Unmarshal(payload, req); err != nil {
			return fmt.Errorf("unmarshal request: %w", err)
		}
		resp, err := Generate(req)
		if err != nil {
			return fmt.Errorf("generate: %w", err)
		}
		out = resp
	default:
		return fmt.Errorf("unknown method tag %d", tag)
	}
	b, err := proto.Marshal(out)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	_, err = stdout.Write(b)
	return err
}

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "sqld-gen-bob:", err)
		os.Exit(1)
	}
}
