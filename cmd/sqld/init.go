package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const initSqldYAML = `version: "1"
engine: postgresql
options:
  defaultSchema: public
schema:
  - file: schema.sql
queries:
  - dir: queries
migrations:
  - dir: migrations
plugins:
  - name: go
    # build the plugin (make build) or ` + "`go install ./cmd/sqld-gen-go`" + `, then point at it:
    command: sqld-gen-go
    out: gen/db
    options:
      package: db
`

const initSchemaSQL = `-- Declarative schema (source of truth). Edit, then ` + "`sqld generate`" + `.
CREATE TABLE authors (
  id   bigserial PRIMARY KEY,
  name text NOT NULL,
  bio  text
);
`

const initQueriesAuthorsSQL = `-- name: GetAuthor :one
SELECT id, name, bio FROM authors WHERE id = @id;

-- name: ListAuthors :many
SELECT id, name, bio FROM authors;
`

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init [dir]",
		Short: "Scaffold a new sqld project (sqld.yaml + schema/queries/migrations)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if code := runInit(args, cmd.OutOrStdout(), cmd.ErrOrStderr()); code != 0 {
				// runInit already wrote a diagnostic; surface a non-zero exit.
				return errInit
			}
			return nil
		},
	}
}

// errInit carries a runtime (exit 1) failure out of runInit, which reports its
// own diagnostic to stderr. It is silent so Execute does not print it twice.
var errInit = &silentError{errors.New("init failed")}

func runInit(args []string, stdout, stderr io.Writer) int {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}

	// Create the target directory if it doesn't exist.
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "init: create dir %s: %v\n", dir, err)
		return 1
	}

	// Refuse to clobber an existing sqld.yaml.
	cfgPath := filepath.Join(dir, "sqld.yaml")
	if _, err := os.Stat(cfgPath); !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(stderr, "init: %s already exists; not overwriting\n", cfgPath)
		return 1
	}

	// Create queries/ directory.
	queriesDir := filepath.Join(dir, "queries")
	if err := os.MkdirAll(queriesDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "init: create dir %s: %v\n", queriesDir, err)
		return 1
	}

	// Create migrations/ directory.
	migrationsDir := filepath.Join(dir, "migrations")
	if err := os.MkdirAll(migrationsDir, 0o755); err != nil {
		fmt.Fprintf(stderr, "init: create dir %s: %v\n", migrationsDir, err)
		return 1
	}

	// Write sqld.yaml.
	if err := os.WriteFile(cfgPath, []byte(initSqldYAML), 0o644); err != nil {
		fmt.Fprintf(stderr, "init: write %s: %v\n", cfgPath, err)
		return 1
	}

	// Write schema.sql.
	schemaPath := filepath.Join(dir, "schema.sql")
	if err := os.WriteFile(schemaPath, []byte(initSchemaSQL), 0o644); err != nil {
		fmt.Fprintf(stderr, "init: write %s: %v\n", schemaPath, err)
		return 1
	}

	// Write queries/authors.sql.
	queriesFile := filepath.Join(queriesDir, "authors.sql")
	if err := os.WriteFile(queriesFile, []byte(initQueriesAuthorsSQL), 0o644); err != nil {
		fmt.Fprintf(stderr, "init: write %s: %v\n", queriesFile, err)
		return 1
	}

	// Write migrations/.gitkeep (empty sentinel file).
	gitkeepPath := filepath.Join(migrationsDir, ".gitkeep")
	if err := os.WriteFile(gitkeepPath, []byte{}, 0o644); err != nil {
		fmt.Fprintf(stderr, "init: write %s: %v\n", gitkeepPath, err)
		return 1
	}

	fmt.Fprintf(stdout, "created %s\n", cfgPath)
	fmt.Fprintf(stdout, "created %s\n", schemaPath)
	fmt.Fprintf(stdout, "created %s\n", queriesFile)
	fmt.Fprintf(stdout, "created %s\n", gitkeepPath)
	fmt.Fprintf(stdout, "\nnext: sqld generate -c %s\n", cfgPath)
	return 0
}
