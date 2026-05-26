package introspect

import (
	"context"
	"strings"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// tableKey uniquely identifies a relation by (schema, name).
type tableKey struct {
	schema string
	name   string
}

// loadTables populates Table + Column for every ordinary table (relkind 'r')
// in the wanted schemas. It records relation oids so constraints/indexes can
// be attached afterwards.
func (b *builder) loadTables(ctx context.Context, schemas []string) error {
	// relOID -> table for constraint/index attachment.
	b.relTables = make(map[uint32]*irv1.Table)
	b.tableByKey = make(map[tableKey]*irv1.Table)

	const q = `
SELECT c.oid, n.nspname, c.relname, c.relpersistence::text
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
LEFT JOIN pg_catalog.pg_depend d
       ON d.classid = 'pg_catalog.pg_class'::regclass
      AND d.objid = c.oid
      AND d.deptype = 'e'
WHERE c.relkind = 'r'
  AND n.nspname = ANY($1)
  AND d.objid IS NULL
ORDER BY n.nspname, c.relname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	type relRow struct {
		oid    uint32
		schema string
		name   string
		persis string
	}
	var rels []relRow
	for rows.Next() {
		var r relRow
		if err := rows.Scan(&r.oid, &r.schema, &r.name, &r.persis); err != nil {
			rows.Close()
			return err
		}
		rels = append(rels, r)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, r := range rels {
		tbl := &irv1.Table{
			Id:          r.schema + "." + r.name,
			Name:        qname(r.schema, r.name),
			Persistence: persistence(r.persis),
		}
		sch := b.getSchema(r.schema)
		sch.Tables = append(sch.Tables, tbl)
		b.relTables[r.oid] = tbl
		b.tableByKey[tableKey{r.schema, r.name}] = tbl

		if err := b.loadColumns(ctx, r.oid, tbl); err != nil {
			return err
		}
	}
	return nil
}

// loadColumns populates the columns of a single relation.
func (b *builder) loadColumns(ctx context.Context, relOID uint32, tbl *irv1.Table) error {
	const q = `
SELECT a.attnum,
       a.attname,
       a.atttypid,
       pg_catalog.format_type(a.atttypid, a.atttypmod) AS formatted,
       a.attnotnull,
       a.attidentity::text,
       a.attgenerated::text,
       pg_catalog.pg_get_expr(d.adbin, d.adrelid) AS default_expr,
       COALESCE(co.collname, '') AS collation
FROM pg_catalog.pg_attribute a
LEFT JOIN pg_catalog.pg_attrdef d ON d.adrelid = a.attrelid AND d.adnum = a.attnum
LEFT JOIN pg_catalog.pg_collation co ON co.oid = a.attcollation AND a.attcollation <> 0
WHERE a.attrelid = $1
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY a.attnum`
	rows, err := b.conn.Query(ctx, q, relOID)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			attnum    int16
			attname   string
			atttypid  uint32
			formatted string
			notnull   bool
			identity  string // '', 'a' (always), 'd' (by default)
			generated string // '', 's' (stored)
			defExpr   *string
			collation string
		)
		if err := rows.Scan(&attnum, &attname, &atttypid, &formatted,
			&notnull, &identity, &generated, &defExpr, &collation); err != nil {
			return err
		}

		col := &irv1.Column{
			Id:        tbl.GetId() + "." + attname,
			Name:      attname,
			Position:  uint32(attnum),
			Type:      b.typeRefFromOID(atttypid, formatted),
			Nullable:  !notnull,
			Collation: collation,
		}

		// Default vs generated: attgenerated 's' means the adbin holds the
		// generation expression rather than a plain default.
		if defExpr != nil && *defExpr != "" {
			if generated == "s" {
				col.Generated = &irv1.GeneratedColumn{
					Expression: rawExpr(*defExpr),
					Stored:     true,
				}
			} else {
				col.DefaultExpr = rawExpr(*defExpr)
			}
		}

		switch identity {
		case "a":
			col.Identity = &irv1.Identity{Kind: irv1.IdentityKind_IDENTITY_KIND_ALWAYS}
		case "d":
			col.Identity = &irv1.Identity{Kind: irv1.IdentityKind_IDENTITY_KIND_BY_DEFAULT}
		}

		tbl.Columns = append(tbl.Columns, col)
	}
	return rows.Err()
}

// loadConstraints reads pg_constraint for every relation in the wanted schemas
// and attaches the resulting Constraint to its owning Table.
func (b *builder) loadConstraints(ctx context.Context, schemas []string) error {
	// Column names are resolved inside SQL (correlated subqueries over
	// pg_attribute) so we never issue a nested query on the same connection
	// while iterating this result set.
	const q = `
SELECT con.conrelid,
       con.conname,
       con.contype::text,
       con.condeferrable,
       con.condeferred,
       con.confupdtype::text,
       con.confdeltype::text,
       con.confmatchtype::text,
       pg_catalog.pg_get_constraintdef(con.oid, true) AS def,
       COALESCE((SELECT array_agg(a.attname ORDER BY k.ord)
                 FROM unnest(con.conkey) WITH ORDINALITY AS k(attnum, ord)
                 JOIN pg_catalog.pg_attribute a
                   ON a.attrelid = con.conrelid AND a.attnum = k.attnum), '{}') AS local_cols,
       COALESCE((SELECT array_agg(a.attname ORDER BY k.ord)
                 FROM unnest(con.confkey) WITH ORDINALITY AS k(attnum, ord)
                 JOIN pg_catalog.pg_attribute a
                   ON a.attrelid = con.confrelid AND a.attnum = k.attnum), '{}') AS ref_cols,
       fn.nspname AS ref_schema,
       fc.relname AS ref_name
FROM pg_catalog.pg_constraint con
JOIN pg_catalog.pg_class rc ON rc.oid = con.conrelid
JOIN pg_catalog.pg_namespace rn ON rn.oid = rc.relnamespace
LEFT JOIN pg_catalog.pg_class fc ON fc.oid = con.confrelid
LEFT JOIN pg_catalog.pg_namespace fn ON fn.oid = fc.relnamespace
WHERE rn.nspname = ANY($1)
  AND con.contype IN ('p','f','u','c','x')
  AND rc.relkind = 'r'
ORDER BY rn.nspname, rc.relname, con.conname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			conrelid    uint32
			conname     string
			contype     string
			deferrable  bool
			deferred    bool
			confupdtype string
			confdeltype string
			confmatch   string
			def         string
			localCols   []string
			refCols     []string
			refSchema   *string
			refName     *string
		)
		if err := rows.Scan(&conrelid, &conname, &contype, &deferrable,
			&deferred, &confupdtype, &confdeltype, &confmatch, &def,
			&localCols, &refCols, &refSchema, &refName); err != nil {
			return err
		}

		tbl := b.relTables[conrelid]
		if tbl == nil {
			continue
		}

		c := &irv1.Constraint{
			Id:                tbl.GetId() + "." + conname,
			Name:              conname,
			Deferrable:        deferrable,
			InitiallyDeferred: deferred,
		}

		switch contype {
		case "p":
			c.Type = irv1.ConstraintType_CONSTRAINT_TYPE_PRIMARY_KEY
			c.Body = &irv1.Constraint_PrimaryKey{
				PrimaryKey: &irv1.PrimaryKey{Columns: localCols},
			}
		case "u":
			c.Type = irv1.ConstraintType_CONSTRAINT_TYPE_UNIQUE
			c.Body = &irv1.Constraint_Unique{
				Unique: &irv1.UniqueConstraint{Columns: localCols},
			}
		case "c":
			c.Type = irv1.ConstraintType_CONSTRAINT_TYPE_CHECK
			c.Body = &irv1.Constraint_Check{
				Check: &irv1.CheckConstraint{Expression: rawExpr(checkBody(def))},
			}
		case "x":
			c.Type = irv1.ConstraintType_CONSTRAINT_TYPE_EXCLUSION
			c.Body = &irv1.Constraint_Exclusion{
				Exclusion: parseExclusionDef(def),
			}
		case "f":
			c.Type = irv1.ConstraintType_CONSTRAINT_TYPE_FOREIGN_KEY
			rs, rname := "", ""
			if refSchema != nil {
				rs = *refSchema
			}
			if refName != nil {
				rname = *refName
			}
			fk := &irv1.ForeignKey{
				Columns:           localCols,
				ReferencedColumns: refCols,
				OnUpdate:          referentialAction(confupdtype),
				OnDelete:          referentialAction(confdeltype),
				MatchType:         matchType(confmatch),
			}
			if rname != "" {
				fk.ReferencedTable = &irv1.ObjectRef{
					Id:   rs + "." + rname,
					Kind: irv1.ObjectKind_OBJECT_KIND_TABLE,
					Name: qname(rs, rname),
				}
			}
			c.Body = &irv1.Constraint_ForeignKey{ForeignKey: fk}
		}

		tbl.Constraints = append(tbl.Constraints, c)
	}
	return rows.Err()
}

// loadSequences reads relkind 'S' relations into Schema.Sequences.
func (b *builder) loadSequences(ctx context.Context, schemas []string) error {
	const q = `
SELECT n.nspname,
       c.relname,
       s.seqtypid,
       pg_catalog.format_type(s.seqtypid, -1) AS formatted,
       s.seqstart,
       s.seqincrement,
       s.seqmin,
       s.seqmax,
       s.seqcache,
       s.seqcycle
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
JOIN pg_catalog.pg_sequence s ON s.seqrelid = c.oid
WHERE c.relkind = 'S'
  AND n.nspname = ANY($1)
ORDER BY n.nspname, c.relname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			schema    string
			name      string
			typoid    uint32
			formatted string
			start     int64
			increment int64
			minv      int64
			maxv      int64
			cache     int64
			cycle     bool
		)
		if err := rows.Scan(&schema, &name, &typoid, &formatted, &start,
			&increment, &minv, &maxv, &cache, &cycle); err != nil {
			return err
		}
		seq := &irv1.Sequence{
			Id:        schema + "." + name,
			Name:      qname(schema, name),
			DataType:  b.typeRefFromOID(typoid, formatted),
			Start:     start,
			Increment: increment,
			MinValue:  minv,
			MaxValue:  maxv,
			Cache:     cache,
			Cycle:     cycle,
		}
		sch := b.getSchema(schema)
		sch.Sequences = append(sch.Sequences, seq)
	}
	return rows.Err()
}

// checkBody strips a leading "CHECK " from a pg_get_constraintdef result so
// the stored expression matches the parsed form (which holds only the
// expression, not the CHECK keyword).
func checkBody(def string) string {
	s := strings.TrimSpace(def)
	const prefix = "CHECK "
	if strings.HasPrefix(s, prefix) {
		s = strings.TrimSpace(s[len(prefix):])
	}
	// Strip one matching pair of outer parens if present.
	if len(s) >= 2 && s[0] == '(' && s[len(s)-1] == ')' {
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// parseExclusionDef parses a pg_get_constraintdef result for an EXCLUSION
// constraint into a structured ExclusionConstraint so renderConstraintBody can
// emit a valid `EXCLUDE USING <method> (<elem> WITH <op>, ...) [WHERE (...)]`.
//
// The def has the shape:
//
//	EXCLUDE USING gist (id WITH =, during WITH &&) [WHERE (<predicate>)]
//
// Element targets are kept as raw expressions (column names included) so that
// even non-trivial expression elements round-trip; the leading "<target> WITH
// <op>" split is performed on the last " WITH " of each element.
func parseExclusionDef(def string) *irv1.ExclusionConstraint {
	ex := &irv1.ExclusionConstraint{}
	s := strings.TrimSpace(def)

	const excl = "EXCLUDE"
	if !strings.HasPrefix(s, excl) {
		return ex
	}
	s = strings.TrimSpace(s[len(excl):])

	// Optional USING <method>.
	if strings.HasPrefix(s, "USING ") {
		s = strings.TrimSpace(s[len("USING "):])
		// method is the token up to the first '('.
		if i := strings.IndexByte(s, '('); i >= 0 {
			ex.IndexMethod = strings.TrimSpace(s[:i])
			s = s[i:]
		}
	}

	// Element list within the first balanced parenthesis group.
	open := strings.IndexByte(s, '(')
	if open < 0 {
		return ex
	}
	depth := 0
	closeIdx := -1
	for i := open; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				closeIdx = i
			}
		}
		if closeIdx >= 0 {
			break
		}
	}
	if closeIdx < 0 {
		return ex
	}
	inner := s[open+1 : closeIdx]
	for _, part := range splitTopLevelCommas(inner) {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		target, op := part, ""
		if idx := strings.LastIndex(part, " WITH "); idx >= 0 {
			target = strings.TrimSpace(part[:idx])
			op = strings.TrimSpace(part[idx+len(" WITH "):])
		}
		ex.Elements = append(ex.Elements, &irv1.ExclusionElement{
			Target:   &irv1.ExclusionElement_Expr{Expr: rawExpr(target)},
			Operator: op,
		})
	}

	// Optional trailing WHERE (predicate).
	rest := strings.TrimSpace(s[closeIdx+1:])
	if strings.HasPrefix(rest, "WHERE ") {
		pred := strings.TrimSpace(rest[len("WHERE "):])
		if len(pred) >= 2 && pred[0] == '(' && pred[len(pred)-1] == ')' {
			pred = strings.TrimSpace(pred[1 : len(pred)-1])
		}
		ex.Predicate = rawExpr(pred)
	}
	return ex
}

// splitTopLevelCommas splits s on commas that are not nested inside parens.
func splitTopLevelCommas(s string) []string {
	var out []string
	depth := 0
	start := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '(':
			depth++
		case ')':
			depth--
		case ',':
			if depth == 0 {
				out = append(out, s[start:i])
				start = i + 1
			}
		}
	}
	out = append(out, s[start:])
	return out
}

// persistence maps relpersistence to TablePersistence.
func persistence(p string) irv1.TablePersistence {
	switch p {
	case "p":
		return irv1.TablePersistence_TABLE_PERSISTENCE_PERMANENT
	case "u":
		return irv1.TablePersistence_TABLE_PERSISTENCE_UNLOGGED
	case "t":
		return irv1.TablePersistence_TABLE_PERSISTENCE_TEMPORARY
	default:
		return irv1.TablePersistence_TABLE_PERSISTENCE_UNSPECIFIED
	}
}

// referentialAction maps a confupdtype/confdeltype char to ReferentialAction.
func referentialAction(c string) irv1.ReferentialAction {
	switch c {
	case "a":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_NO_ACTION
	case "r":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_RESTRICT
	case "c":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_CASCADE
	case "n":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_SET_NULL
	case "d":
		return irv1.ReferentialAction_REFERENTIAL_ACTION_SET_DEFAULT
	default:
		return irv1.ReferentialAction_REFERENTIAL_ACTION_UNSPECIFIED
	}
}

// matchType maps a confmatchtype char to MatchType.
func matchType(c string) irv1.MatchType {
	switch c {
	case "f":
		return irv1.MatchType_MATCH_TYPE_FULL
	case "p":
		return irv1.MatchType_MATCH_TYPE_PARTIAL
	case "s":
		return irv1.MatchType_MATCH_TYPE_SIMPLE
	default:
		return irv1.MatchType_MATCH_TYPE_UNSPECIFIED
	}
}

// basePgName extracts a base type name from a format_type result by dropping
// array brackets and modifier parens. Used only as a fallback when an oid is
// unknown.
func basePgName(formatted string) string {
	s := formatted
	if i := strings.IndexByte(s, '('); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSuffix(strings.TrimSpace(s), "[]")
	if i := strings.LastIndexByte(s, '.'); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSpace(s)
}
