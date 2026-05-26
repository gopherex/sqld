package introspect

import (
	"context"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// loadTypes populates enums, domains, composites and ranges for the wanted
// schemas.
func (b *builder) loadTypes(ctx context.Context, schemas []string) error {
	if err := b.loadEnums(ctx, schemas); err != nil {
		return err
	}
	if err := b.loadDomains(ctx, schemas); err != nil {
		return err
	}
	if err := b.loadComposites(ctx, schemas); err != nil {
		return err
	}
	return b.loadRanges(ctx, schemas)
}

// loadEnums reads pg_type typtype='e' plus their ordered labels.
func (b *builder) loadEnums(ctx context.Context, schemas []string) error {
	const q = `
SELECT n.nspname,
       t.typname,
       array_agg(e.enumlabel ORDER BY e.enumsortorder) AS labels
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
JOIN pg_catalog.pg_enum e ON e.enumtypid = t.oid
WHERE t.typtype = 'e'
  AND n.nspname = ANY($1)
GROUP BY n.nspname, t.typname
ORDER BY n.nspname, t.typname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			schema string
			name   string
			labels []string
		)
		if err := rows.Scan(&schema, &name, &labels); err != nil {
			return err
		}
		et := &irv1.EnumType{
			Id:     schema + "." + name,
			Name:   qname(schema, name),
			Labels: labels,
		}
		b.getSchema(schema).Enums = append(b.getSchema(schema).Enums, et)
	}
	return rows.Err()
}

// loadDomains reads pg_type typtype='d' with their base type, nullability,
// default, collation and CHECK constraints.
func (b *builder) loadDomains(ctx context.Context, schemas []string) error {
	const q = `
SELECT n.nspname,
       t.typname,
       t.typbasetype,
       pg_catalog.format_type(t.typbasetype, t.typtypmod) AS base_formatted,
       t.typnotnull,
       pg_catalog.pg_get_expr(t.typdefaultbin, 0) AS default_expr,
       COALESCE(co.collname, '') AS collation
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
LEFT JOIN pg_catalog.pg_collation co ON co.oid = t.typcollation AND t.typcollation <> 0
WHERE t.typtype = 'd'
  AND n.nspname = ANY($1)
ORDER BY n.nspname, t.typname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	type domRow struct {
		schema    string
		name      string
		baseOID   uint32
		baseFmt   string
		notnull   bool
		defExpr   *string
		collation string
	}
	var doms []domRow
	for rows.Next() {
		var d domRow
		if err := rows.Scan(&d.schema, &d.name, &d.baseOID, &d.baseFmt,
			&d.notnull, &d.defExpr, &d.collation); err != nil {
			return err
		}
		doms = append(doms, d)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	for _, d := range doms {
		dt := &irv1.DomainType{
			Id:        d.schema + "." + d.name,
			Name:      qname(d.schema, d.name),
			BaseType:  b.typeRefFromOID(d.baseOID, d.baseFmt),
			Nullable:  !d.notnull,
			Collation: d.collation,
		}
		if d.defExpr != nil {
			dt.DefaultExpr = rawExpr(*d.defExpr)
		}
		cons, err := b.domainConstraints(ctx, d.schema, d.name)
		if err != nil {
			return err
		}
		dt.Constraints = cons
		b.getSchema(d.schema).Domains = append(b.getSchema(d.schema).Domains, dt)
	}
	return nil
}

// domainConstraints reads CHECK constraints attached to a domain.
func (b *builder) domainConstraints(ctx context.Context, schema, name string) ([]*irv1.DomainConstraint, error) {
	const q = `
SELECT con.conname,
       pg_catalog.pg_get_constraintdef(con.oid, true) AS def
FROM pg_catalog.pg_constraint con
JOIN pg_catalog.pg_type t ON t.oid = con.contypid
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
WHERE n.nspname = $1
  AND t.typname = $2
  AND con.contype = 'c'
ORDER BY con.conname`
	rows, err := b.conn.Query(ctx, q, schema, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*irv1.DomainConstraint
	for rows.Next() {
		var cname, def string
		if err := rows.Scan(&cname, &def); err != nil {
			return nil, err
		}
		out = append(out, &irv1.DomainConstraint{
			Name:  cname,
			Check: rawExpr(checkBody(def)),
		})
	}
	return out, rows.Err()
}

// loadComposites reads standalone composite types (typtype='c' whose backing
// pg_class is relkind 'c') and their fields.
func (b *builder) loadComposites(ctx context.Context, schemas []string) error {
	const q = `
SELECT n.nspname, t.typname, t.typrelid
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
JOIN pg_catalog.pg_class c ON c.oid = t.typrelid
WHERE t.typtype = 'c'
  AND c.relkind = 'c'
  AND n.nspname = ANY($1)
ORDER BY n.nspname, t.typname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	type compRow struct {
		schema string
		name   string
		relOID uint32
	}
	var comps []compRow
	for rows.Next() {
		var c compRow
		if err := rows.Scan(&c.schema, &c.name, &c.relOID); err != nil {
			rows.Close()
			return err
		}
		comps = append(comps, c)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	for _, c := range comps {
		fields, err := b.compositeFields(ctx, c.relOID)
		if err != nil {
			return err
		}
		ct := &irv1.CompositeType{
			Id:     c.schema + "." + c.name,
			Name:   qname(c.schema, c.name),
			Fields: fields,
		}
		b.getSchema(c.schema).Composites = append(b.getSchema(c.schema).Composites, ct)
	}
	return nil
}

// compositeFields reads the attributes of a composite type's backing relation.
func (b *builder) compositeFields(ctx context.Context, relOID uint32) ([]*irv1.CompositeField, error) {
	const q = `
SELECT a.attname,
       a.atttypid,
       pg_catalog.format_type(a.atttypid, a.atttypmod) AS formatted,
       COALESCE(co.collname, '') AS collation
FROM pg_catalog.pg_attribute a
LEFT JOIN pg_catalog.pg_collation co ON co.oid = a.attcollation AND a.attcollation <> 0
WHERE a.attrelid = $1
  AND a.attnum > 0
  AND NOT a.attisdropped
ORDER BY a.attnum`
	rows, err := b.conn.Query(ctx, q, relOID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*irv1.CompositeField
	for rows.Next() {
		var (
			name      string
			typoid    uint32
			formatted string
			collation string
		)
		if err := rows.Scan(&name, &typoid, &formatted, &collation); err != nil {
			return nil, err
		}
		out = append(out, &irv1.CompositeField{
			Name:      name,
			Type:      b.typeRefFromOID(typoid, formatted),
			Collation: collation,
		})
	}
	return out, rows.Err()
}

// loadRanges reads range types (typtype='r') with their subtype, opclass,
// canonical/diff functions and associated multirange name.
func (b *builder) loadRanges(ctx context.Context, schemas []string) error {
	const q = `
SELECT n.nspname,
       t.typname,
       r.rngsubtype,
       pg_catalog.format_type(r.rngsubtype, -1) AS sub_formatted,
       COALESCE(opc.opcname, '') AS opclass,
       COALESCE(can.proname, '') AS canonical,
       COALESCE(dif.proname, '') AS subtype_diff,
       COALESCE(mt.typname, '') AS multirange
FROM pg_catalog.pg_range r
JOIN pg_catalog.pg_type t ON t.oid = r.rngtypid
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace
LEFT JOIN pg_catalog.pg_opclass opc ON opc.oid = r.rngsubopc AND r.rngsubopc <> 0
LEFT JOIN pg_catalog.pg_proc can ON can.oid = r.rngcanonical AND r.rngcanonical <> 0
LEFT JOIN pg_catalog.pg_proc dif ON dif.oid = r.rngsubdiff AND r.rngsubdiff <> 0
LEFT JOIN pg_catalog.pg_type mt ON mt.oid = r.rngmultitypid AND r.rngmultitypid <> 0
WHERE t.typtype = 'r'
  AND n.nspname = ANY($1)
ORDER BY n.nspname, t.typname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			schema     string
			name       string
			subOID     uint32
			subFmt     string
			opclass    string
			canonical  string
			subdiff    string
			multirange string
		)
		if err := rows.Scan(&schema, &name, &subOID, &subFmt, &opclass,
			&canonical, &subdiff, &multirange); err != nil {
			return err
		}
		rt := &irv1.RangeType{
			Id:             schema + "." + name,
			Name:           qname(schema, name),
			Subtype:        b.typeRefFromOID(subOID, subFmt),
			SubtypeOpclass: opclass,
			Canonical:      canonical,
			SubtypeDiff:    subdiff,
			Multirange:     multirange,
		}
		b.getSchema(schema).Ranges = append(b.getSchema(schema).Ranges, rt)
	}
	return rows.Err()
}
