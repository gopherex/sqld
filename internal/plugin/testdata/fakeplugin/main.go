// fakeplugin is a test helper binary that implements the sqld stdio plugin
// transport. It reads all of stdin, interprets the first byte as a method tag,
// deserializes the request, and writes the serialized response to stdout.
package main

import (
	"io"
	"os"

	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
	"google.golang.org/protobuf/proto"
)

func main() {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		os.Stderr.WriteString("fakeplugin: read stdin: " + err.Error() + "\n")
		os.Exit(1)
	}
	if len(data) == 0 {
		os.Stderr.WriteString("fakeplugin: empty stdin\n")
		os.Exit(1)
	}

	tag := data[0]
	payload := data[1:]

	var out []byte
	switch tag {
	case 0: // GetInfo
		var req pluginv1.GetInfoRequest
		if err := proto.Unmarshal(payload, &req); err != nil {
			os.Stderr.WriteString("fakeplugin: unmarshal GetInfoRequest: " + err.Error() + "\n")
			os.Exit(1)
		}
		resp := &pluginv1.GetInfoResponse{
			Name:    "fake",
			Version: "0.0.1",
		}
		out, err = proto.Marshal(resp)
		if err != nil {
			os.Stderr.WriteString("fakeplugin: marshal GetInfoResponse: " + err.Error() + "\n")
			os.Exit(1)
		}
	case 1: // Generate
		var req pluginv1.GenerateRequest
		if err := proto.Unmarshal(payload, &req); err != nil {
			os.Stderr.WriteString("fakeplugin: unmarshal GenerateRequest: " + err.Error() + "\n")
			os.Exit(1)
		}
		resp := &pluginv1.GenerateResponse{
			Files: []*pluginv1.GeneratedFile{
				{
					Path:     "out.txt",
					Contents: []byte("hi"),
				},
			},
		}
		out, err = proto.Marshal(resp)
		if err != nil {
			os.Stderr.WriteString("fakeplugin: marshal GenerateResponse: " + err.Error() + "\n")
			os.Exit(1)
		}
	default:
		os.Stderr.WriteString("fakeplugin: unknown tag\n")
		os.Exit(1)
	}

	if _, err := os.Stdout.Write(out); err != nil {
		os.Stderr.WriteString("fakeplugin: write stdout: " + err.Error() + "\n")
		os.Exit(1)
	}
}
