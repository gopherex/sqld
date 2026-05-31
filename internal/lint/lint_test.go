package lint

import (
	"strings"
	"testing"

	"github.com/gopherex/sqld/pkg/migrate"
)

// findOne returns the first finding matching rule, or nil.
func findOne(findings []Finding, rule string) *Finding {
	for i := range findings {
		if findings[i].Rule == rule {
			return &findings[i]
		}
	}
	return nil
}

// countRule counts findings with the given rule id.
func countRule(findings []Finding, rule string) int {
	n := 0
	for _, f := range findings {
		if f.Rule == rule {
			n++
		}
	}
	return n
}

// mig is a small helper to build a migration with a down section so that
// missing-down does not pollute rule-specific assertions.
func mig(version, up string) migrate.Migration {
	return migrate.Migration{Version: version, Name: "t", UpSQL: up, DownSQL: "-- noop"}
}

func TestDropTable(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("001", "DROP TABLE x;")})
	f := findOne(findings, RuleDestructiveDrop)
	if f == nil {
		t.Fatalf("expected a %s finding, got %+v", RuleDestructiveDrop, findings)
	}
	if f.Severity != SeverityError {
		t.Errorf("expected error severity, got %q", f.Severity)
	}
	if f.Version != "001" {
		t.Errorf("expected version 001, got %q", f.Version)
	}
	if !strings.Contains(f.Message, "x") {
		t.Errorf("expected message to name the table x, got %q", f.Message)
	}
}

func TestAddNotNullColumnNoDefault(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("002", "ALTER TABLE t ADD COLUMN c int NOT NULL;")})
	f := findOne(findings, RuleNotNullNoDefault)
	if f == nil {
		t.Fatalf("expected a %s finding, got %+v", RuleNotNullNoDefault, findings)
	}
	if f.Severity != SeverityError {
		t.Errorf("expected error severity, got %q", f.Severity)
	}
	if !strings.Contains(f.Message, "c") {
		t.Errorf("expected message to name column c, got %q", f.Message)
	}
}

func TestAddNotNullColumnWithDefault(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("003", "ALTER TABLE t ADD COLUMN c int NOT NULL DEFAULT 0;")})
	if f := findOne(findings, RuleNotNullNoDefault); f != nil {
		t.Errorf("expected no %s finding when a default is present, got %+v", RuleNotNullNoDefault, *f)
	}
}

func TestColumnTypeChange(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("004", "ALTER TABLE t ALTER COLUMN c TYPE bigint;")})
	f := findOne(findings, RuleColumnTypeChange)
	if f == nil {
		t.Fatalf("expected a %s finding, got %+v", RuleColumnTypeChange, findings)
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %q", f.Severity)
	}
}

func TestCreateIndexNotConcurrent(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("005", "CREATE INDEX i ON t(c);")})
	f := findOne(findings, RuleIndexNotConcurr)
	if f == nil {
		t.Fatalf("expected a %s finding, got %+v", RuleIndexNotConcurr, findings)
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %q", f.Severity)
	}
}

func TestCreateIndexConcurrent(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("006", "CREATE INDEX CONCURRENTLY i ON t(c);")})
	if f := findOne(findings, RuleIndexNotConcurr); f != nil {
		t.Errorf("expected no %s finding for a concurrent index, got %+v", RuleIndexNotConcurr, *f)
	}
}

func TestMissingDown(t *testing.T) {
	// No down section -> missing-down warning.
	withoutDown := migrate.Migration{Version: "007", Name: "t", UpSQL: "CREATE TABLE t (id int);"}
	findings := Lint([]migrate.Migration{withoutDown})
	f := findOne(findings, RuleMissingDown)
	if f == nil {
		t.Fatalf("expected a %s finding, got %+v", RuleMissingDown, findings)
	}
	if f.Severity != SeverityWarning {
		t.Errorf("expected warning severity, got %q", f.Severity)
	}

	// With a down section -> no missing-down.
	withDown := migrate.Migration{Version: "008", Name: "t", UpSQL: "CREATE TABLE t (id int);", DownSQL: "DROP TABLE t;"}
	findings = Lint([]migrate.Migration{withDown})
	if f := findOne(findings, RuleMissingDown); f != nil {
		t.Errorf("expected no %s finding when down SQL is present, got %+v", RuleMissingDown, *f)
	}
}

func TestCleanCreateTableNoFindings(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("009", "CREATE TABLE t (id int PRIMARY KEY, name text NOT NULL);")})
	if len(findings) != 0 {
		t.Errorf("expected no findings for a clean CREATE TABLE, got %+v", findings)
	}
}

func TestTruncate(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("010", "TRUNCATE TABLE t;")})
	f := findOne(findings, RuleDestructiveTrunc)
	if f == nil {
		t.Fatalf("expected a %s finding, got %+v", RuleDestructiveTrunc, findings)
	}
	if f.Severity != SeverityError {
		t.Errorf("expected error severity, got %q", f.Severity)
	}
}

func TestDropColumnAndConstraint(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("011", "ALTER TABLE t DROP COLUMN c; ALTER TABLE t DROP CONSTRAINT ck;")})
	if n := countRule(findings, RuleDestructiveDrop); n != 2 {
		t.Fatalf("expected 2 %s findings (column + constraint), got %d: %+v", RuleDestructiveDrop, n, findings)
	}
}

func TestParseError(t *testing.T) {
	findings := Lint([]migrate.Migration{mig("012", "THIS IS NOT SQL;;;")})
	f := findOne(findings, RuleParseError)
	if f == nil {
		t.Fatalf("expected a %s finding, got %+v", RuleParseError, findings)
	}
	if f.Severity != SeverityError {
		t.Errorf("expected error severity, got %q", f.Severity)
	}
}
