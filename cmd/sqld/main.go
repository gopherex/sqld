package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yaroher/sqld/pkg/sqld"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"
)

const usage = `Usage: sqld <subcommand> [flags]

Subcommands:
  init      [dir]                                Scaffold a new sqld project (sqld.yaml + schema/queries/migrations)
  generate  -c <config.yaml>                    Run code generation
  collect   -c <config.yaml> [-format json|prototext]  Collect IR and print to stdout
`

// run is the testable entry point. It parses args, dispatches to subcommands,
// and writes output to stdout/stderr. It returns an exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "init":
		return runInit(rest, stdout, stderr)
	case "generate":
		return runGenerate(rest, stderr)
	case "collect":
		return runCollect(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n\n%s", sub, usage)
		return 2
	}
}

func runGenerate(args []string, stderr io.Writer) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("c", "", "path to sqld.yaml config file (required)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *cfgPath == "" {
		fmt.Fprintf(stderr, "generate: -c is required\n\n%s", usage)
		return 2
	}
	if err := sqld.GenerateFile(*cfgPath); err != nil {
		fmt.Fprintf(stderr, "generate: %v\n", err)
		return 1
	}
	fmt.Fprintln(stderr, "generated")
	return 0
}

func runCollect(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("collect", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("c", "", "path to sqld.yaml config file (required)")
	format := fs.String("format", "json", "output format: json or prototext")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *cfgPath == "" {
		fmt.Fprintf(stderr, "collect: -c is required\n\n%s", usage)
		return 2
	}
	catalog, err := sqld.CollectFile(*cfgPath)
	if err != nil {
		fmt.Fprintf(stderr, "collect: %v\n", err)
		return 1
	}

	var out []byte
	switch *format {
	case "json", "":
		out, err = protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(catalog)
	case "prototext":
		out, err = prototext.MarshalOptions{Multiline: true}.Marshal(catalog)
	default:
		fmt.Fprintf(stderr, "collect: unknown format %q; want json or prototext\n", *format)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "collect: marshal: %v\n", err)
		return 1
	}
	if _, err := stdout.Write(out); err != nil {
		fmt.Fprintf(stderr, "collect: write: %v\n", err)
		return 1
	}
	// Ensure trailing newline for readability
	if len(out) > 0 && out[len(out)-1] != '\n' {
		fmt.Fprintln(stdout)
	}
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
