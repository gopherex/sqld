// Command sqld-migrate applies, reverts, inspects, and generates SQL
// migrations for a sqld project.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/yaroher/sqld/internal/devdb"
	"github.com/yaroher/sqld/internal/diff"
	"github.com/yaroher/sqld/internal/introspect"
	"github.com/yaroher/sqld/internal/parse"
	"github.com/yaroher/sqld/internal/source"
	"github.com/yaroher/sqld/pkg/config"
	"github.com/yaroher/sqld/pkg/migrate"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

const usage = `Usage: sqld-migrate <subcommand> [flags]

Subcommands:
  up       [-c f] [--db DSN] [--to V]            Apply pending migrations
  apply    [-c f] [--db DSN] [--to V]            Alias of up
  down     [-c f] [--db DSN] [--steps N|--to V]  Revert migrations
  status   [-c f] [--db DSN]                     Show applied/pending/drift
  generate <name> [-c f] [--dev-url DSN]         Generate a migration by schema diff
  hash     [-c f]                                Print each migration version + checksum
  validate [-c f]                                Parse each migration's up SQL

The database DSN comes from --db or $DATABASE_URL. The config path defaults to
sqld.yaml (override with -c).
`

// run is the testable entry point. It parses args, dispatches to a subcommand,
// writes to stdout/stderr, and returns an exit code.
func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "up", "apply":
		return runUp(rest, stdout, stderr)
	case "down":
		return runDown(rest, stdout, stderr)
	case "status":
		return runStatus(rest, stdout, stderr)
	case "generate":
		return runGenerate(rest, stdout, stderr)
	case "hash":
		return runHash(rest, stdout, stderr)
	case "validate":
		return runValidate(rest, stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown subcommand %q\n\n%s", sub, usage)
		return 2
	}
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

// dsn resolves the database DSN from the --db flag, falling back to
// $DATABASE_URL. It returns "" when neither is set.
func dsn(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv("DATABASE_URL")
}

// splitPositional pulls the first leading non-flag token (a positional
// argument) out of args, returning it plus the remaining args for flag
// parsing. A token starting with "-" is treated as a flag, so a name appearing
// only after the flags is left for fs.Args() to recover.
func splitPositional(args []string) (positional string, rest []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return args[0], args[1:]
	}
	return "", args
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
// schema (public unless overridden) plus any schemas named in the search path.
// Returns nil to introspect every non-system schema when nothing specific is
// configured.
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

// loadConfig loads the config from path, reporting errors to stderr.
func loadConfig(path string, stderr io.Writer) (*config.Config, bool) {
	cfg, err := config.Load(path)
	if err != nil {
		fmt.Fprintf(stderr, "%v\n", err)
		return nil, false
	}
	return cfg, true
}

// connectPool opens a pgxpool against the resolved DSN, reporting errors to
// stderr. The caller must Close the returned pool.
func connectPool(ctx context.Context, d string, stderr io.Writer) (*pgxpool.Pool, bool) {
	if d == "" {
		fmt.Fprintf(stderr, "no database DSN: set --db or $DATABASE_URL\n\n%s", usage)
		return nil, false
	}
	pool, err := pgxpool.New(ctx, d)
	if err != nil {
		fmt.Fprintf(stderr, "connect: %v\n", err)
		return nil, false
	}
	return pool, true
}

func runUp(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("up", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("c", "sqld.yaml", "path to sqld.yaml config file")
	db := fs.String("db", "", "database DSN (else $DATABASE_URL)")
	to := fs.String("to", "", "migrate up to this version (inclusive)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, ok := loadConfig(*cfgPath, stderr)
	if !ok {
		return 1
	}
	dir := migrationsDir(cfg)
	migs, err := migrate.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "load migrations: %v\n", err)
		return 1
	}

	ctx := context.Background()
	pool, ok := connectPool(ctx, dsn(*db), stderr)
	if !ok {
		return 2
	}
	defer pool.Close()

	m := migrate.New(pool, migs)
	if *to != "" {
		if err := m.To(ctx, *to); err != nil {
			fmt.Fprintf(stderr, "up: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "migrated to %s\n", *to)
		return 0
	}
	if err := m.Up(ctx); err != nil {
		fmt.Fprintf(stderr, "up: %v\n", err)
		return 1
	}
	fmt.Fprintln(stdout, "migrated up")
	return 0
}

func runDown(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("down", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("c", "sqld.yaml", "path to sqld.yaml config file")
	db := fs.String("db", "", "database DSN (else $DATABASE_URL)")
	steps := fs.Int("steps", 1, "number of migrations to revert")
	to := fs.String("to", "", "revert down to this version (inclusive)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, ok := loadConfig(*cfgPath, stderr)
	if !ok {
		return 1
	}
	dir := migrationsDir(cfg)
	migs, err := migrate.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "load migrations: %v\n", err)
		return 1
	}

	ctx := context.Background()
	pool, ok := connectPool(ctx, dsn(*db), stderr)
	if !ok {
		return 2
	}
	defer pool.Close()

	m := migrate.New(pool, migs)
	if *to != "" {
		if err := m.To(ctx, *to); err != nil {
			fmt.Fprintf(stderr, "down: %v\n", err)
			return 1
		}
		fmt.Fprintf(stdout, "migrated to %s\n", *to)
		return 0
	}
	if err := m.Down(ctx, *steps); err != nil {
		fmt.Fprintf(stderr, "down: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "reverted %d migration(s)\n", *steps)
	return 0
}

func runStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("c", "sqld.yaml", "path to sqld.yaml config file")
	db := fs.String("db", "", "database DSN (else $DATABASE_URL)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, ok := loadConfig(*cfgPath, stderr)
	if !ok {
		return 1
	}
	dir := migrationsDir(cfg)
	migs, err := migrate.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "load migrations: %v\n", err)
		return 1
	}

	ctx := context.Background()
	pool, ok := connectPool(ctx, dsn(*db), stderr)
	if !ok {
		return 2
	}
	defer pool.Close()

	m := migrate.New(pool, migs)
	st, err := m.Status(ctx)
	if err != nil {
		fmt.Fprintf(stderr, "status: %v\n", err)
		return 1
	}

	fmt.Fprintf(stdout, "Applied (%d):\n", len(st.Applied))
	for _, a := range st.Applied {
		fmt.Fprintf(stdout, "  %s_%s  %s\n", a.Version, a.Name, a.AppliedAt.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(stdout, "Pending (%d):\n", len(st.Pending))
	for _, p := range st.Pending {
		fmt.Fprintf(stdout, "  %s_%s\n", p.Version, p.Name)
	}
	fmt.Fprintf(stdout, "Drift (%d):\n", len(st.Drift))
	for _, d := range st.Drift {
		fmt.Fprintf(stdout, "  %s\n", d)
	}
	return 0
}

func runHash(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("hash", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("c", "sqld.yaml", "path to sqld.yaml config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, ok := loadConfig(*cfgPath, stderr)
	if !ok {
		return 1
	}
	dir := migrationsDir(cfg)
	migs, err := migrate.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "load migrations: %v\n", err)
		return 1
	}

	for _, m := range migs {
		fmt.Fprintf(stdout, "%s  %s\n", m.Version, m.Checksum)
	}
	return 0
}

func runValidate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("validate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("c", "sqld.yaml", "path to sqld.yaml config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	cfg, ok := loadConfig(*cfgPath, stderr)
	if !ok {
		return 1
	}
	dir := migrationsDir(cfg)
	migs, err := migrate.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "load migrations: %v\n", err)
		return 1
	}

	failed := 0
	for _, m := range migs {
		name := m.Version + "_" + m.Name
		if _, err := parse.Statements(m.UpSQL); err != nil {
			fmt.Fprintf(stderr, "validate: %s: %v\n", name, err)
			failed++
			continue
		}
		fmt.Fprintf(stdout, "ok: %s\n", name)
	}
	if failed > 0 {
		fmt.Fprintf(stderr, "validate: %d migration(s) failed\n", failed)
		return 1
	}
	return 0
}

func runGenerate(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("generate", flag.ContinueOnError)
	fs.SetOutput(stderr)
	cfgPath := fs.String("c", "sqld.yaml", "path to sqld.yaml config file")
	devURL := fs.String("dev-url", "", "dev database DSN (else ephemeral container)")
	// The migration name is a positional argument that may appear before or
	// after flags. Reorder so flags parse first; the leading non-flag token is
	// the name.
	name, flagArgs := splitPositional(args)
	if err := fs.Parse(flagArgs); err != nil {
		return 2
	}
	if name == "" {
		// A name may still be trailing after the flags.
		if rest := fs.Args(); len(rest) > 0 {
			name = rest[0]
		}
	}
	if name == "" {
		fmt.Fprintf(stderr, "generate: migration name is required\n\n%s", usage)
		return 2
	}

	cfg, ok := loadConfig(*cfgPath, stderr)
	if !ok {
		return 1
	}
	units, err := source.Resolve(cfg)
	if err != nil {
		fmt.Fprintf(stderr, "generate: resolve sources: %v\n", err)
		return 1
	}

	// Desired state = declarative schema SQL. Current state = the SQL produced
	// by the existing migrations (in version order).
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
	desired, err := buildCatalog(ctx, *devURL, strings.Join(schemaSQL, "\n"), schemas)
	if err != nil {
		fmt.Fprintf(stderr, "generate: build desired state: %v\n", err)
		return 1
	}

	// Current catalog: a SECOND fresh dev DB with the existing migrations applied.
	current, err := buildCatalog(ctx, *devURL, strings.Join(migSQL, "\n"), schemas)
	if err != nil {
		fmt.Fprintf(stderr, "generate: build current state: %v\n", err)
		return 1
	}

	plan, err := diff.Diff(current, desired)
	if err != nil {
		fmt.Fprintf(stderr, "generate: diff: %v\n", err)
		return 1
	}
	if plan.Empty() {
		fmt.Fprintln(stdout, "no changes")
		return 0
	}

	dir := migrationsDir(cfg)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(stderr, "generate: mkdir %s: %v\n", dir, err)
		return 1
	}
	version := genVersion(time.Now().UTC())
	outPath := filepath.Join(dir, version+"_"+name+".sql")
	content := "-- sqld:up\n" + plan.UpSQL() + "\n\n-- sqld:down\n" + plan.DownSQL() + "\n"
	if err := os.WriteFile(outPath, []byte(content), 0o644); err != nil {
		fmt.Fprintf(stderr, "generate: write %s: %v\n", outPath, err)
		return 1
	}

	fmt.Fprintln(stdout, outPath)
	return 0
}

// genVersion renders a migration version stamp at millisecond resolution so
// two generates within the same second do not collide. The "." in the
// reference layout is stripped so the version stays an all-digit,
// lexicographically sortable token (e.g. 20060102150405123).
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
