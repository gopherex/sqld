// Command sqld is the PostgreSQL toolkit host CLI: it scaffolds projects,
// drives code generation through plugins, collects the semantic IR, and applies
// and generates SQL migrations.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// version is the build version, stamped at release time via
// -ldflags "-X main.version=<tag>". It is "dev" for local builds.
var version = "dev"

// usageError marks a CLI usage problem (bad or missing args/flags). It maps to
// exit code 2; ordinary runtime errors map to 1.
type usageError struct{ err error }

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// errNoSubcommand is returned when sqld is invoked without a subcommand. Help
// has already been printed, so it carries no message.
var errNoSubcommand = &usageError{errors.New("")}

// silentError is a runtime error (exit 1) whose diagnostic has already been
// written by the command itself, so Execute must not print it again.
type silentError struct{ err error }

func (e *silentError) Error() string { return e.err.Error() }
func (e *silentError) Unwrap() error { return e.err }

// Execute builds the root command, runs it against args, and returns a process
// exit code: 0 success, 1 runtime error, 2 usage error.
func Execute(args []string, stdout, stderr io.Writer) int {
	root := newRootCmd()
	root.SetArgs(args)
	root.SetOut(stdout)
	root.SetErr(stderr)

	err := root.Execute()
	if err == nil {
		return 0
	}
	var ue *usageError
	if errors.As(err, &ue) {
		return 2
	}
	var se *silentError
	if errors.As(err, &se) {
		return 1
	}
	// cobra reports an unknown subcommand as a plain error before any RunE
	// runs; treat it as a usage error.
	if strings.HasPrefix(err.Error(), "unknown command") {
		fmt.Fprintf(stderr, "%v\n", err)
		return 2
	}
	fmt.Fprintf(stderr, "%v\n", err)
	return 1
}

// run is a thin alias kept for the test suite.
func run(args []string, stdout, stderr io.Writer) int { return Execute(args, stdout, stderr) }

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "sqld",
		Short:         "sqld — PostgreSQL toolkit: codegen, IR collection, and migrations",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return errNoSubcommand
		},
	}
	// Flag-parsing failures (bad/missing flags) are usage errors. cobra
	// propagates this func to every subcommand.
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		fmt.Fprintf(cmd.ErrOrStderr(), "%v\n", err)
		return &usageError{err}
	})
	root.AddCommand(
		newInitCmd(),
		newGenerateCmd(),
		newCollectCmd(),
		newMigrateCmd(),
	)
	return root
}

func main() {
	os.Exit(Execute(os.Args[1:], os.Stdout, os.Stderr))
}
