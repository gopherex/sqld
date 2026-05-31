// Package lint analyzes SQL migrations for destructive or risky changes.
//
// It parses each migration's up SQL with internal/parse and walks the
// resulting statement nodes, emitting Findings for patterns that can lose data,
// take heavy locks, or otherwise fail on a populated production database. The
// linter is database-free: it operates purely on the parse tree and never
// connects to or executes against a server.
package lint

import (
	"fmt"
	"strings"

	pg "github.com/pganalyze/pg_query_go/v6"

	"github.com/gopherex/sqld/internal/parse"
	"github.com/gopherex/sqld/pkg/migrate"
)

// Severity is the seriousness of a Finding.
type Severity string

const (
	// SeverityError marks a change that loses data or is otherwise unsafe; it
	// should block a migration unless explicitly overridden.
	SeverityError Severity = "error"
	// SeverityWarning marks a change that is risky (e.g. takes a lock) but not
	// inherently destructive.
	SeverityWarning Severity = "warning"
)

// Rule identifiers. These are stable strings suitable for allow-listing.
const (
	RuleParseError       = "parse-error"
	RuleDestructiveDrop  = "destructive-drop"
	RuleDestructiveTrunc = "destructive-truncate"
	RuleColumnTypeChange = "column-type-change"
	RuleNotNullNoDefault = "not-null-no-default"
	RuleIndexNotConcurr  = "index-not-concurrent"
	RuleMissingDown      = "missing-down"
)

// Finding is a single lint result tied to a migration version.
type Finding struct {
	Version  string // migration version ("" for cross-file findings)
	Rule     string // stable rule id, e.g. "destructive-drop"
	Severity Severity
	Message  string
}

// Lint analyzes migrations (parsing each UpSQL) and returns findings in
// migration order. It never panics: a parse failure yields a parse-error
// finding and statements it does not recognize are ignored.
func Lint(migs []migrate.Migration) []Finding {
	var findings []Finding
	for _, mig := range migs {
		findings = append(findings, lintOne(mig)...)
	}
	return findings
}

// lintOne returns the findings for a single migration.
func lintOne(mig migrate.Migration) []Finding {
	var out []Finding

	if strings.TrimSpace(mig.DownSQL) == "" {
		out = append(out, Finding{
			Version:  mig.Version,
			Rule:     RuleMissingDown,
			Severity: SeverityWarning,
			Message:  "migration has no down SQL: no rollback path",
		})
	}

	stmts, err := parse.Statements(mig.UpSQL)
	if err != nil {
		out = append(out, Finding{
			Version:  mig.Version,
			Rule:     RuleParseError,
			Severity: SeverityError,
			Message:  fmt.Sprintf("failed to parse up SQL: %v", err),
		})
		return out
	}

	for _, st := range stmts {
		out = append(out, lintNode(mig.Version, st.Node)...)
	}
	return out
}

// lintNode inspects one top-level statement node and appends any findings.
func lintNode(version string, node *pg.Node) []Finding {
	if node == nil {
		return nil
	}
	switch {
	case node.GetDropStmt() != nil:
		return lintDrop(version, node.GetDropStmt())
	case node.GetTruncateStmt() != nil:
		return lintTruncate(version, node.GetTruncateStmt())
	case node.GetAlterTableStmt() != nil:
		return lintAlterTable(version, node.GetAlterTableStmt())
	case node.GetIndexStmt() != nil:
		return lintIndex(version, node.GetIndexStmt())
	}
	return nil
}

// lintDrop handles DROP of a table/schema/sequence/type/view/materialized view.
func lintDrop(version string, drop *pg.DropStmt) []Finding {
	kind, ok := droppableKind(drop.GetRemoveType())
	if !ok {
		return nil
	}
	var out []Finding
	names := dropObjectNames(drop)
	if len(names) == 0 {
		names = []string{"(unnamed)"}
	}
	for _, name := range names {
		out = append(out, Finding{
			Version:  version,
			Rule:     RuleDestructiveDrop,
			Severity: SeverityError,
			Message:  fmt.Sprintf("drops %s %q: destroys the object and its data", kind, name),
		})
	}
	return out
}

// droppableKind maps a destructive DROP target type to a human label. The bool
// is false for object types the linter does not flag (e.g. DROP INDEX).
func droppableKind(t pg.ObjectType) (string, bool) {
	switch t {
	case pg.ObjectType_OBJECT_TABLE:
		return "table", true
	case pg.ObjectType_OBJECT_SCHEMA:
		return "schema", true
	case pg.ObjectType_OBJECT_SEQUENCE:
		return "sequence", true
	case pg.ObjectType_OBJECT_TYPE:
		return "type", true
	case pg.ObjectType_OBJECT_VIEW:
		return "view", true
	case pg.ObjectType_OBJECT_MATVIEW:
		return "materialized view", true
	default:
		return "", false
	}
}

// dropObjectNames extracts the dotted names of every object in a DROP statement.
// Objects are encoded either as a List of String name parts (tables, views,
// sequences, schemas) or as a TypeName (DROP TYPE).
func dropObjectNames(drop *pg.DropStmt) []string {
	var names []string
	for _, obj := range drop.GetObjects() {
		if obj == nil {
			continue
		}
		if l := obj.GetList(); l != nil {
			names = append(names, joinStringParts(l.GetItems()))
			continue
		}
		if tn := obj.GetTypeName(); tn != nil {
			names = append(names, joinStringParts(tn.GetNames()))
			continue
		}
		if s := obj.GetString_(); s != nil {
			names = append(names, s.GetSval())
		}
	}
	return names
}

// joinStringParts joins the Sval of each String node with ".".
func joinStringParts(items []*pg.Node) string {
	var parts []string
	for _, it := range items {
		if s := it.GetString_(); s != nil {
			parts = append(parts, s.GetSval())
		}
	}
	return strings.Join(parts, ".")
}

// lintTruncate flags TRUNCATE, which empties tables irreversibly.
func lintTruncate(version string, trunc *pg.TruncateStmt) []Finding {
	var names []string
	for _, rel := range trunc.GetRelations() {
		if rv := rel.GetRangeVar(); rv != nil {
			names = append(names, rangeVarName(rv))
		}
	}
	target := strings.Join(names, ", ")
	if target == "" {
		target = "(unnamed)"
	}
	return []Finding{{
		Version:  version,
		Rule:     RuleDestructiveTrunc,
		Severity: SeverityError,
		Message:  fmt.Sprintf("truncates %s: removes all rows irreversibly", target),
	}}
}

// lintAlterTable walks each ALTER TABLE command, flagging drop column, drop
// constraint, column type changes, and NOT NULL columns added without a default.
func lintAlterTable(version string, alter *pg.AlterTableStmt) []Finding {
	var out []Finding
	table := "table"
	if rv := alter.GetRelation(); rv != nil {
		table = rangeVarName(rv)
	}
	for _, c := range alter.GetCmds() {
		cmd := c.GetAlterTableCmd()
		if cmd == nil {
			continue
		}
		switch cmd.GetSubtype() {
		case pg.AlterTableType_AT_DropColumn:
			out = append(out, Finding{
				Version:  version,
				Rule:     RuleDestructiveDrop,
				Severity: SeverityError,
				Message:  fmt.Sprintf("drops column %q from %s: destroys column data", cmd.GetName(), table),
			})
		case pg.AlterTableType_AT_DropConstraint:
			out = append(out, Finding{
				Version:  version,
				Rule:     RuleDestructiveDrop,
				Severity: SeverityError,
				Message:  fmt.Sprintf("drops constraint %q from %s", cmd.GetName(), table),
			})
		case pg.AlterTableType_AT_AlterColumnType:
			out = append(out, Finding{
				Version:  version,
				Rule:     RuleColumnTypeChange,
				Severity: SeverityWarning,
				Message:  fmt.Sprintf("changes type of column %q on %s: may rewrite the table and lose data", cmd.GetName(), table),
			})
		case pg.AlterTableType_AT_AddColumn:
			if col := cmd.GetDef().GetColumnDef(); col != nil {
				if f, ok := checkNotNullNoDefault(version, table, col); ok {
					out = append(out, f)
				}
			}
		}
	}
	return out
}

// checkNotNullNoDefault reports an added column declared NOT NULL with no
// default/generated/identity value, which fails on a non-empty table.
func checkNotNullNoDefault(version, table string, col *pg.ColumnDef) (Finding, bool) {
	var notNull, hasFill bool
	for _, cn := range col.GetConstraints() {
		con := cn.GetConstraint()
		if con == nil {
			continue
		}
		switch con.GetContype() {
		case pg.ConstrType_CONSTR_NOTNULL:
			notNull = true
		case pg.ConstrType_CONSTR_DEFAULT,
			pg.ConstrType_CONSTR_GENERATED,
			pg.ConstrType_CONSTR_IDENTITY:
			hasFill = true
		}
	}
	if notNull && !hasFill {
		return Finding{
			Version:  version,
			Rule:     RuleNotNullNoDefault,
			Severity: SeverityError,
			Message:  fmt.Sprintf("adds NOT NULL column %q to %s without a default: fails on a non-empty table", col.GetColname(), table),
		}, true
	}
	return Finding{}, false
}

// lintIndex flags non-concurrent CREATE INDEX, which takes a lock that blocks
// writes for the duration of the build.
func lintIndex(version string, idx *pg.IndexStmt) []Finding {
	if idx.GetConcurrent() {
		return nil
	}
	name := idx.GetIdxname()
	target := ""
	if rv := idx.GetRelation(); rv != nil {
		target = rangeVarName(rv)
	}
	var on string
	if target != "" {
		on = " on " + target
	}
	var idxLabel string
	if name != "" {
		idxLabel = fmt.Sprintf(" %q", name)
	}
	return []Finding{{
		Version:  version,
		Rule:     RuleIndexNotConcurr,
		Severity: SeverityWarning,
		Message:  fmt.Sprintf("creates index%s%s without CONCURRENTLY: locks the table against writes (CREATE INDEX CONCURRENTLY cannot run inside a transaction)", idxLabel, on),
	}}
}

// rangeVarName renders a RangeVar as an optionally schema-qualified name.
func rangeVarName(rv *pg.RangeVar) string {
	if rv == nil {
		return ""
	}
	if s := rv.GetSchemaname(); s != "" {
		return s + "." + rv.GetRelname()
	}
	return rv.GetRelname()
}
