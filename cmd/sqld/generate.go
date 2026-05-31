package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/encoding/prototext"

	"github.com/gopherex/sqld/pkg/sqld"
)

func newGenerateCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Run code generation",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := sqld.GenerateFile(cfgPath); err != nil {
				return fmt.Errorf("generate: %w", err)
			}
			fmt.Fprintln(cmd.ErrOrStderr(), "generated")
			return nil
		},
	}
	cmd.Flags().StringVarP(&cfgPath, "config", "c", "", "path to sqld.yaml config file (required)")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}

func newCollectCmd() *cobra.Command {
	var (
		cfgPath string
		format  string
	)
	cmd := &cobra.Command{
		Use:   "collect",
		Short: "Collect the semantic IR (Catalog) and print it to stdout",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			catalog, err := sqld.CollectFile(cfgPath)
			if err != nil {
				return fmt.Errorf("collect: %w", err)
			}

			var out []byte
			switch format {
			case "json", "":
				out, err = protojson.MarshalOptions{Multiline: true, Indent: "  "}.Marshal(catalog)
			case "prototext":
				out, err = prototext.MarshalOptions{Multiline: true}.Marshal(catalog)
			default:
				return &usageError{fmt.Errorf("collect: unknown format %q; want json or prototext", format)}
			}
			if err != nil {
				return fmt.Errorf("collect: marshal: %w", err)
			}

			stdout := cmd.OutOrStdout()
			if _, err := stdout.Write(out); err != nil {
				return fmt.Errorf("collect: write: %w", err)
			}
			// Ensure a trailing newline for readability.
			if len(out) > 0 && out[len(out)-1] != '\n' {
				fmt.Fprintln(stdout)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&cfgPath, "config", "c", "", "path to sqld.yaml config file (required)")
	cmd.Flags().StringVar(&format, "format", "json", "output format: json or prototext")
	_ = cmd.MarkFlagRequired("config")
	return cmd
}
