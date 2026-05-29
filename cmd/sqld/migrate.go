package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/spf13/cobra"

	"github.com/yaroher/sqld/internal/diff"
	"github.com/yaroher/sqld/internal/introspect"
	"github.com/yaroher/sqld/internal/lint"
	"github.com/yaroher/sqld/internal/parse"
	"github.com/yaroher/sqld/internal/source"
	"github.com/yaroher/sqld/pkg/config"
	"github.com/yaroher/sqld/pkg/devdb"
	"github.com/yaroher/sqld/pkg/migrate"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// newMigrateCmd builds the `sqld migrate` subcommand tree: apply, revert,
// inspect, and generate SQL migrations for a sqld project. The database DSN
// comes from --db or $DATABASE_URL; the config path defaults to sqld.yaml.
func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Apply, revert, inspect, and generate SQL migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return errNoSubcommand
		},
	}
	cmd.AddCommand(
		newMigrateUpCmd(),
		newMigrateDownCmd(),
		newMigrateStatusCmd(),
		newMigrateGenerateCmd(),
		newMigrateHashCmd(),
		newMigrateValidateCmd(),
		newMigrateLintCmd(),
	)
	return cmd
}

func newMigrateUpCmd() *cobra.Command {
	var cfgPath, db, to string
	cmd := &cobra.Command{
		Use:     "up",
		Aliases: []string{"apply"},
		Short:   "Apply pending migrations",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			migs, err := loadMigrations(cfgPath)
			if err != nil {
				return err
			}
			ctx := context.Background()
			pool, err := openPool(ctx, dsn(db))
			if err != nil {
				return err
			}
			defer pool.Close()

			m := migrate.New(pool, migs)
			if to != "" {
				if err := m.To(ctx, to); err != nil {
					return fmt.Errorf("up: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "migrated to %s\n", to)
				return nil
			}
			if err := m.Up(ctx); err != nil {
				return fmt.Errorf("up: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "migrated up")
			return nil
		},
	}
	addConfigFlag(cmd, &cfgPath)
	addDBFlag(cmd, &db)
	cmd.Flags().StringVar(&to, "to", "", "migrate up to this version (inclusive)")
	return cmd
}

func newMigrateDownCmd() *cobra.Command {
	var cfgPath, db, to string
	var steps int
	cmd := &cobra.Command{
		Use:   "down",
		Short: "Revert migrations",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			migs, err := loadMigrations(cfgPath)
			if err != nil {
				return err
			}
			ctx := context.Background()
			pool, err := openPool(ctx, dsn(db))
			if err != nil {
				return err
			}
			defer pool.Close()

			m := migrate.New(pool, migs)
			if to != "" {
				if err := m.To(ctx, to); err != nil {
					return fmt.Errorf("down: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "migrated to %s\n", to)
				return nil
			}
			if err := m.Down(ctx, steps); err != nil {
				return fmt.Errorf("down: %w", err)
			}
			fmt.Fprintf(cmd.OutOrStdout(), "reverted %d migration(s)\n", steps)
			return nil
		},
	}
	addConfigFlag(cmd, &cfgPath)
	addDBFlag(cmd, &db)
	cmd.Flags().IntVar(&steps, "steps", 1, "number of migrations to revert")
	cmd.Flags().StringVar(&to, "to", "", "revert down to this version (inclusive)")
	return cmd
}

func newMigrateStatusCmd() *cobra.Command {
	var cfgPath, db string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show applied/pending/drift",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			migs, err := loadMigrations(cfgPath)
			if err != nil {
				return err
			}
			ctx := context.Background()
			pool, err := openPool(ctx, dsn(db))
			if err != nil {
				return err
			}
			defer pool.Close()

			m := migrate.New(pool, migs)
			st, err := m.Status(ctx)
			if err != nil {
				return fmt.Errorf("status: %w", err)
			}

			out := cmd.OutOrStdout()
			fmt.Fprintf(out, "Applied (%d):\n", len(st.Applied))
			for _, a := range st.Applied {
				fmt.Fprintf(out, "  %s_%s  %s\n", a.Version, a.Name, a.AppliedAt.UTC().Format(time.RFC3339))
			}
			fmt.Fprintf(out, "Pending (%d):\n", len(st.Pending))
			for _, p := range st.Pending {
				fmt.Fprintf(out, "  %s_%s\n", p.Version, p.Name)
			}
			fmt.Fprintf(out, "Drift (%d):\n", len(st.Drift))
			for _, d := range st.Drift {
				fmt.Fprintf(out, "  %s\n", d)
			}
			return nil
		},
	}
	addConfigFlag(cmd, &cfgPath)
	addDBFlag(cmd, &db)
	return cmd
}

func newMigrateHashCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "hash",
		Short: "Print each migration version + checksum",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			migs, err := loadMigrations(cfgPath)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			for _, m := range migs {
				fmt.Fprintf(out, "%s  %s\n", m.Version, m.Checksum)
			}
			return nil
		},
	}
	addConfigFlag(cmd, &cfgPath)
	return cmd
}

func newMigrateValidateCmd() *cobra.Command {
	var cfgPath string
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Parse each migration's up SQL",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			migs, err := loadMigrations(cfgPath)
			if err != nil {
				return err
			}
			out, errOut := cmd.OutOrStdout(), cmd.ErrOrStderr()
			failed := 0
			for _, m := range migs {
				name := m.Version + "_" + m.Name
				if _, err := parse.Statements(m.UpSQL); err != nil {
					fmt.Fprintf(errOut, "validate: %s: %v\n", name, err)
					failed++
					continue
				}
				fmt.Fprintf(out, "ok: %s\n", name)
			}
			if failed > 0 {
				return &silentError{fmt.Errorf("validate: %d migration(s) failed", failed)}
			}
			return nil
		},
	}
	addConfigFlag(cmd, &cfgPath)
	return cmd
}

// newMigrateLintCmd reports destructive/risky changes grouped by version. It
// fails when any error-severity finding is present; with --strict, warnings
// fail too.
func newMigrateLintCmd() *cobra.Command {
	var cfgPath string
	var strict bool
	cmd := &cobra.Command{
		Use:   "lint",
		Short: "Report destructive/risky changes",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			migs, err := loadMigrations(cfgPath)
			if err != nil {
				return err
			}

			findings := lint.Lint(migs)
			out := cmd.OutOrStdout()
			if len(findings) == 0 {
				fmt.Fprintln(out, "lint: no findings")
				return nil
			}

			// Print findings grouped by version, in first-appearance order.
			var order []string
			byVersion := map[string][]lint.Finding{}
			for _, f := range findings {
				if _, seen := byVersion[f.Version]; !seen {
					order = append(order, f.Version)
				}
				byVersion[f.Version] = append(byVersion[f.Version], f)
			}

			errors, warnings := 0, 0
			for _, v := range order {
				label := v
				if label == "" {
					label = "(cross-file)"
				}
				for _, f := range byVersion[v] {
					fmt.Fprintf(out, "%s [%s] %s: %s\n", label, f.Severity, f.Rule, f.Message)
					switch f.Severity {
					case lint.SeverityError:
						errors++
					case lint.SeverityWarning:
						warnings++
					}
				}
			}
			fmt.Fprintf(out, "lint: %d error(s), %d warning(s)\n", errors, warnings)

			if errors > 0 || (strict && warnings > 0) {
				return &silentError{fmt.Errorf("lint: %d error(s)", errors)}
			}
			return nil
		},
	}
	addConfigFlag(cmd, &cfgPath)
	cmd.Flags().BoolVar(&strict, "strict", false, "treat warnings as failures too")
	return cmd
}

func newMigrateGenerateCmd() *cobra.Command {
	var cfgPath, devURL string
	cmd := &cobra.Command{
		Use:   "generate <name>",
		Short: "Generate a migration by schema diff",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			cfg, err := loadConfig(cfgPath)
			if err != nil {
				return err
			}
			units, err := source.Resolve(cfg)
			if err != nil {
				return fmt.Errorf("generate: resolve sources: %w", err)
			}

			// Desired state = declarative schema SQL. Current state = the SQL
			// produced by the existing migrations (in version order).
			var schemaSQL []string
			type mig struct {
				version string
				sql     string
			}
			var migUnits []mig
			for _, u := range units {
				switch u.Kind {
				case source.KindSchema:
					if s := strings.TrimSpace(u.SQL); s != "" {
						schemaSQL = append(schemaSQL, s)
					}
				case source.KindMigrationUp:
					migUnits = append(migUnits, mig{version: u.Version, sql: u.SQL})
				}
			}
			sort.Slice(migUnits, func(i, j int) bool { return migUnits[i].version < migUnits[j].version })
			var migSQL []string
			for _, m := range migUnits {
				if s := strings.TrimSpace(m.sql); s != "" {
					migSQL = append(migSQL, s)
				}
			}

			ctx := context.Background()
			schemas := schemasOf(cfg)

			// Desired catalog: a fresh dev DB with the declarative schema applied.
			desired, err := buildCatalog(ctx, devURL, strings.Join(schemaSQL, "\n"), schemas)
			if err != nil {
				return fmt.Errorf("generate: build desired state: %w", err)
			}

			// Current catalog: a SECOND fresh dev DB with existing migrations applied.
			current, err := buildCatalog(ctx, devURL, strings.Join(migSQL, "\n"), schemas)
			if err != nil {
				return fmt.Errorf("generate: build current state: %w", err)
			}

			plan, err := diff.Diff(current, desired)
			if err != nil {
				return fmt.Errorf("generate: diff: %w", err)
			}
			if plan.Empty() {
				fmt.Fprintln(cmd.OutOrStdout(), "no changes")
				return nil
			}

			dir := migrationsDir(cfg)
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return fmt.Errorf("generate: mkdir %s: %w", dir, err)
			}
			version := genVersion(time.Now().UTC())
			outPath := filepath.Join(dir, version+"_"+name+".sql")
			content := "-- sqld:up\n" + plan.UpSQL() + "\n\n-- sqld:down\n" + plan.DownSQL() + "\n"
			if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
				return fmt.Errorf("generate: write %s: %w", outPath, err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), outPath)
			return nil
		},
	}
	addConfigFlag(cmd, &cfgPath)
	cmd.Flags().StringVar(&devURL, "dev-url", "", "dev database DSN (else ephemeral container)")
	return cmd
}

// --- shared flags + helpers -------------------------------------------------

func addConfigFlag(cmd *cobra.Command, p *string) {
	cmd.Flags().StringVarP(p, "config", "c", "sqld.yaml", "path to sqld.yaml config file")
}

func addDBFlag(cmd *cobra.Command, p *string) {
	cmd.Flags().StringVar(p, "db", "", "database DSN (else $DATABASE_URL)")
}

// dsn resolves the database DSN from the --db flag, falling back to
// $DATABASE_URL. It returns "" when neither is set.
func dsn(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv("DATABASE_URL")
}

// loadConfig loads the config from path.
func loadConfig(path string) (*config.Config, error) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// loadMigrations loads the migrations from the directory configured in path.
func loadMigrations(cfgPath string) ([]migrate.Migration, error) {
	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return nil, err
	}
	migs, err := migrate.Load(migrationsDir(cfg))
	if err != nil {
		return nil, fmt.Errorf("load migrations: %w", err)
	}
	return migs, nil
}

// openPool opens a pgxpool against the resolved DSN. A missing DSN is a usage
// error. The caller must Close the returned pool.
func openPool(ctx context.Context, d string) (*pgxpool.Pool, error) {
	if d == "" {
		return nil, &usageError{fmt.Errorf("no database DSN: set --db or $DATABASE_URL")}
	}
	pool, err := pgxpool.New(ctx, d)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	return pool, nil
}

// migrationsDir returns the first configured migrations directory, defaulting
// to "migrations".
func migrationsDir(cfg *config.Config) string {
	if len(cfg.Migrations) > 0 && cfg.Migrations[0].Dir != "" {
		return cfg.Migrations[0].Dir
	}
	return "migrations"
}

// schemasOf returns the schema list to introspect/diff: the configured default
// schema plus any schemas named in the search path. Returns nil to introspect
// every non-system schema when nothing specific is configured.
func schemasOf(cfg *config.Config) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		if s == "" || seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	add(cfg.Options.DefaultSchema)
	for _, s := range cfg.Options.SearchPath {
		add(s)
	}
	// When only the implicit default ("public") is present, introspect all
	// non-system schemas instead so user schemas are not silently dropped.
	if len(out) <= 1 {
		return nil
	}
	sort.Strings(out)
	return out
}

// genVersion renders a migration version stamp at millisecond resolution so two
// generates within the same second do not collide. The "." in the reference
// layout is stripped so the version stays an all-digit, lexicographically
// sortable token (e.g. 20060102150405123).
func genVersion(t time.Time) string {
	return strings.Replace(t.UTC().Format("20060102150405.000"), ".", "", 1)
}

// buildCatalog spins up a fresh dev database, applies sql (when non-empty), and
// introspects it into a catalog. Each call uses an independent dev DB so the
// desired and current states never share schema.
func buildCatalog(ctx context.Context, devURL, sql string, schemas []string) (cat *irv1.Catalog, err error) {
	dev, err := devdb.Open(ctx, devURL)
	if err != nil {
		return nil, err
	}
	defer func() {
		if cerr := dev.Close(); cerr != nil && err == nil {
			err = cerr
		}
	}()

	// An external --dev-url database is reused across calls (Close is a no-op),
	// so each independent state must start from a clean slate. The ephemeral
	// container path gets a brand-new instance per Open and needs no reset.
	if dev.External() {
		if err := dev.Reset(ctx); err != nil {
			return nil, fmt.Errorf("reset dev db: %w", err)
		}
	}

	if strings.TrimSpace(sql) != "" {
		if err := dev.Apply(ctx, sql); err != nil {
			return nil, fmt.Errorf("apply sql: %w", err)
		}
	}

	conn, err := pgx.Connect(ctx, dev.URL())
	if err != nil {
		return nil, fmt.Errorf("connect dev: %w", err)
	}
	defer conn.Close(ctx)

	return introspect.Introspect(ctx, conn, schemas)
}
