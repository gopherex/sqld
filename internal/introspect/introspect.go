// Package introspect builds an IR Catalog by querying a live PostgreSQL
// instance through pg_catalog. The produced *irv1.Catalog has the same shape
// as the one internal/catalog.Build produces from parsed DDL, so the diff
// engine can compare a parsed (desired) catalog against an introspected
// (actual) one.
//
// Object ids follow the same convention as internal/catalog: "schema.name"
// for schema-scoped objects, "schema.table.column" for columns, and
// "schema.table.constraint" for constraints. Expressions (defaults, check
// bodies, index predicates, generated columns) are captured verbatim as the
// Expr raw_sql escape hatch via pg_get_expr / pg_get_constraintdef /
// pg_get_indexdef, mirroring how the diff compares text.
//
// The introspector never panics: types it cannot classify fall back to a
// best-effort TypeRef{PgName: ...}; rows that fail to scan are skipped.
package introspect

import (
	"context"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// DBTX is the minimal pgx query surface the introspector needs. Both
// *pgx.Conn and *pgxpool.Pool satisfy it.
type DBTX interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// builder accumulates schema state during a single Introspect call. It mirrors
// internal/catalog so ids and placement stay consistent.
type builder struct {
	conn    DBTX
	schemas map[string]*irv1.Schema // keyed by schema name
	order   []string                // schema names, deterministic order
	// oidToType maps a pg_type oid to a resolved kind + qualified name so we
	// can classify columns/args that reference user-defined types (enums,
	// domains, composites, ranges) without an extra round-trip per column.
	oidToType map[uint32]typeInfo

	// Relation lookups populated by loadTables and consumed by
	// loadConstraints / loadIndexes / loadTriggers.
	relTables  map[uint32]*irv1.Table   // relOID -> table
	tableByKey map[tableKey]*irv1.Table // (schema,name) -> table
}

// typeInfo is a cached classification of a pg_type row.
type typeInfo struct {
	kind   irv1.TypeKind
	schema string
	name   string // typname (the libpg_query-style base name)
	isArr  bool   // typcategory 'A' / element type set
	elem   uint32 // typelem when array
}

// getSchema returns (creating if necessary) the IR Schema for name.
func (b *builder) getSchema(name string) *irv1.Schema {
	if s, ok := b.schemas[name]; ok {
		return s
	}
	s := &irv1.Schema{Id: name, Name: name}
	b.schemas[name] = s
	b.order = append(b.order, name)
	return s
}

// Introspect builds an *irv1.Catalog from a live PostgreSQL connection,
// restricted to the given schemas. When schemas is empty it defaults to
// "public" plus every non-system, non-temp schema. It never panics.
func Introspect(ctx context.Context, conn DBTX, schemas []string) (*irv1.Catalog, error) {
	b := &builder{
		conn:      conn,
		schemas:   make(map[string]*irv1.Schema),
		oidToType: make(map[uint32]typeInfo),
	}

	wanted, err := b.discoverSchemas(ctx, schemas)
	if err != nil {
		return nil, fmt.Errorf("introspect: discover schemas: %w", err)
	}
	for _, s := range wanted {
		b.getSchema(s)
	}

	// Classify all user-defined types up front so column/arg type resolution
	// can attach the right kind. (Built-ins stay SCALAR.)
	if err := b.classifyTypes(ctx, wanted); err != nil {
		return nil, fmt.Errorf("introspect: classify types: %w", err)
	}

	if err := b.loadTypes(ctx, wanted); err != nil {
		return nil, fmt.Errorf("introspect: types: %w", err)
	}
	if err := b.loadSequences(ctx, wanted); err != nil {
		return nil, fmt.Errorf("introspect: sequences: %w", err)
	}
	if err := b.loadTables(ctx, wanted); err != nil {
		return nil, fmt.Errorf("introspect: tables: %w", err)
	}
	if err := b.loadConstraints(ctx, wanted); err != nil {
		return nil, fmt.Errorf("introspect: constraints: %w", err)
	}
	if err := b.loadIndexes(ctx, wanted); err != nil {
		return nil, fmt.Errorf("introspect: indexes: %w", err)
	}
	if err := b.loadViews(ctx, wanted); err != nil {
		return nil, fmt.Errorf("introspect: views: %w", err)
	}
	if err := b.loadRoutines(ctx, wanted); err != nil {
		return nil, fmt.Errorf("introspect: routines: %w", err)
	}
	if err := b.loadTriggers(ctx, wanted); err != nil {
		return nil, fmt.Errorf("introspect: triggers: %w", err)
	}

	dbName, dbVer := b.serverInfo(ctx)

	out := make([]*irv1.Schema, 0, len(b.order))
	for _, name := range b.order {
		out = append(out, b.schemas[name])
	}

	return &irv1.Catalog{
		Name:            dbName,
		DatabaseVersion: dbVer,
		DefaultSchema:   "public",
		Schemas:         out,
	}, nil
}

// discoverSchemas resolves the effective schema list. When the caller passes
// an explicit list it is used verbatim (deduped, sorted). Otherwise every
// non-system schema present in the database is returned, with "public" first.
func (b *builder) discoverSchemas(ctx context.Context, requested []string) ([]string, error) {
	if len(requested) > 0 {
		seen := make(map[string]bool, len(requested))
		var out []string
		for _, s := range requested {
			if s == "" || seen[s] {
				continue
			}
			seen[s] = true
			out = append(out, s)
		}
		sort.Strings(out)
		return out, nil
	}

	const q = `
SELECT nspname
FROM pg_catalog.pg_namespace
WHERE nspname NOT LIKE 'pg\_%'
  AND nspname <> 'information_schema'
ORDER BY nspname`
	rows, err := b.conn.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	havePublic := false
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if name == "public" {
			havePublic = true
			continue
		}
		out = append(out, name)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	sort.Strings(out)
	if havePublic {
		out = append([]string{"public"}, out...)
	} else {
		// Always include public so an empty database still yields a bucket.
		out = append([]string{"public"}, out...)
	}
	return out, nil
}

// serverInfo returns the current database name and server version string.
// Failures are non-fatal; empty strings are returned instead.
func (b *builder) serverInfo(ctx context.Context) (name, version string) {
	_ = b.conn.QueryRow(ctx, `SELECT current_database()`).Scan(&name)
	_ = b.conn.QueryRow(ctx, `SELECT version()`).Scan(&version)
	return name, version
}

// classifyTypes loads every pg_type relevant to the wanted schemas (plus the
// built-ins they may reference as element/base types) and records a kind for
// each oid so column resolution can tag user-defined types correctly.
func (b *builder) classifyTypes(ctx context.Context, schemas []string) error {
	const q = `
SELECT t.oid, t.typname, n.nspname, t.typtype::text, t.typcategory::text, t.typelem
FROM pg_catalog.pg_type t
JOIN pg_catalog.pg_namespace n ON n.oid = t.typnamespace`
	rows, err := b.conn.Query(ctx, q)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			oid     uint32
			typname string
			nspname string
			typtype string // b,c,d,e,p,r,m
			typcat  string
			typelem uint32
		)
		if err := rows.Scan(&oid, &typname, &nspname, &typtype, &typcat, &typelem); err != nil {
			return err
		}
		info := typeInfo{schema: nspname, name: typname, elem: typelem}
		switch {
		case typcat == "A" || (typelem != 0 && len(typname) > 0 && typname[0] == '_'):
			info.kind = irv1.TypeKind_TYPE_KIND_ARRAY
			info.isArr = true
		default:
			switch typtype {
			case "e":
				info.kind = irv1.TypeKind_TYPE_KIND_ENUM
			case "d":
				info.kind = irv1.TypeKind_TYPE_KIND_DOMAIN
			case "c":
				info.kind = irv1.TypeKind_TYPE_KIND_COMPOSITE
			case "r", "m":
				info.kind = irv1.TypeKind_TYPE_KIND_RANGE
			case "p":
				info.kind = irv1.TypeKind_TYPE_KIND_PSEUDO
			default:
				info.kind = irv1.TypeKind_TYPE_KIND_SCALAR
			}
		}
		b.oidToType[oid] = info
	}
	return rows.Err()
}

// typeRefFromOID builds a TypeRef for a column/arg/field given its base type
// oid and the format_type string (which preserves modifiers + array brackets).
// It uses the up-front classification to set Kind and Udt for user types.
func (b *builder) typeRefFromOID(oid uint32, formatted string) *irv1.TypeRef {
	info, ok := b.oidToType[oid]
	if !ok {
		// Unknown oid: best effort with the formatted name.
		pg := basePgName(formatted)
		return &irv1.TypeRef{
			Kind:     irv1.TypeKind_TYPE_KIND_SCALAR,
			PgName:   pg,
			Modifier: modifierFromFormatted(pg, formatted),
		}
	}

	if info.kind == irv1.TypeKind_TYPE_KIND_ARRAY {
		// Resolve the element type (info.elem) for the array. The column's
		// modifier (e.g. numeric(10,2)[]) applies to the element type, so the
		// formatted string is carried onto the element so its modifier renders.
		elem := b.scalarOrUserRef(info.elem)
		if mod := modifierFromFormatted(elem.GetPgName(), formatted); mod != nil {
			elem.Modifier = mod
		}
		return &irv1.TypeRef{
			Kind:            irv1.TypeKind_TYPE_KIND_ARRAY,
			PgName:          info.name, // e.g. "_int4"
			ArrayDimensions: 1,
			Element:         elem,
		}
	}

	ref := &irv1.TypeRef{
		Kind:     info.kind,
		PgName:   info.name,
		Modifier: modifierFromFormatted(info.name, formatted),
	}
	switch info.kind {
	case irv1.TypeKind_TYPE_KIND_ENUM:
		ref.Udt = b.udtRef(info, irv1.ObjectKind_OBJECT_KIND_ENUM_TYPE)
	case irv1.TypeKind_TYPE_KIND_DOMAIN:
		ref.Udt = b.udtRef(info, irv1.ObjectKind_OBJECT_KIND_DOMAIN_TYPE)
	case irv1.TypeKind_TYPE_KIND_COMPOSITE:
		ref.Udt = b.udtRef(info, irv1.ObjectKind_OBJECT_KIND_COMPOSITE_TYPE)
	case irv1.TypeKind_TYPE_KIND_RANGE:
		ref.Udt = b.udtRef(info, irv1.ObjectKind_OBJECT_KIND_RANGE_TYPE)
	}
	return ref
}

// scalarOrUserRef builds a TypeRef for a (typically element) oid.
func (b *builder) scalarOrUserRef(oid uint32) *irv1.TypeRef {
	info, ok := b.oidToType[oid]
	if !ok {
		return &irv1.TypeRef{Kind: irv1.TypeKind_TYPE_KIND_SCALAR}
	}
	ref := &irv1.TypeRef{Kind: info.kind, PgName: info.name}
	switch info.kind {
	case irv1.TypeKind_TYPE_KIND_ENUM:
		ref.Udt = b.udtRef(info, irv1.ObjectKind_OBJECT_KIND_ENUM_TYPE)
	case irv1.TypeKind_TYPE_KIND_DOMAIN:
		ref.Udt = b.udtRef(info, irv1.ObjectKind_OBJECT_KIND_DOMAIN_TYPE)
	case irv1.TypeKind_TYPE_KIND_COMPOSITE:
		ref.Udt = b.udtRef(info, irv1.ObjectKind_OBJECT_KIND_COMPOSITE_TYPE)
	case irv1.TypeKind_TYPE_KIND_RANGE:
		ref.Udt = b.udtRef(info, irv1.ObjectKind_OBJECT_KIND_RANGE_TYPE)
	}
	return ref
}

// udtRef constructs an ObjectRef pointing at a user-defined type.
func (b *builder) udtRef(info typeInfo, kind irv1.ObjectKind) *irv1.ObjectRef {
	return &irv1.ObjectRef{
		Id:   info.schema + "." + info.name,
		Kind: kind,
		Name: &irv1.QualifiedName{Schema: info.schema, Name: info.name},
	}
}

// rawExpr wraps a SQL fragment in an Expr via the raw_sql escape hatch, or nil
// when the fragment is empty.
func rawExpr(sql string) *irv1.Expr {
	if sql == "" {
		return nil
	}
	return &irv1.Expr{Node: &irv1.Expr_RawSql{RawSql: sql}}
}

// qname builds a schema-qualified QualifiedName.
func qname(schema, name string) *irv1.QualifiedName {
	return &irv1.QualifiedName{Schema: schema, Name: name}
}

// modifierFromFormatted builds a TypeModifier for a scalar type from its
// format_type output, keyed by the bare pg_type name (typname). It mirrors how
// internal/mapper.MapType represents modifiers from parsed DDL, so an
// introspected column and a parsed column of the same type render identically
// (no spurious modifier diffs).
//
// pgName is the bare typname (e.g. "numeric", "varchar", "timestamptz");
// formatted is the pg_catalog.format_type result (e.g. "numeric(10,2)",
// "character varying(50)", "timestamp(3) with time zone"). The leading
// number(s) inside the first parenthesized group carry the modifier values.
func modifierFromFormatted(pgName, formatted string) *irv1.TypeModifier {
	nums := formatModifierNumbers(formatted)

	switch pgName {
	case "numeric", "float4", "float8":
		if len(nums) == 0 {
			return nil
		}
		m := &irv1.NumericModifier{Precision: uint32(nums[0])}
		if len(nums) > 1 {
			m.Scale = uint32(nums[1])
		}
		return &irv1.TypeModifier{Modifier: &irv1.TypeModifier_Numeric{Numeric: m}}

	case "varchar", "bpchar", "char", "bit", "varbit":
		if len(nums) == 0 {
			return nil
		}
		return &irv1.TypeModifier{
			Modifier: &irv1.TypeModifier_Text{Text: &irv1.StringModifier{Length: uint32(nums[0])}},
		}

	case "timestamp", "timestamptz":
		tz := pgName == "timestamptz"
		if len(nums) == 0 {
			if tz {
				// Bare timestamptz still carries WithTimezone, matching the parser.
				return &irv1.TypeModifier{
					Modifier: &irv1.TypeModifier_DateTime{DateTime: &irv1.DateTimeModifier{WithTimezone: true}},
				}
			}
			return nil
		}
		return &irv1.TypeModifier{
			Modifier: &irv1.TypeModifier_DateTime{DateTime: &irv1.DateTimeModifier{
				WithTimezone: tz,
				Precision:    uint32(nums[0]),
			}},
		}

	case "time", "timetz":
		tz := pgName == "timetz"
		if len(nums) == 0 {
			if tz {
				return &irv1.TypeModifier{
					Modifier: &irv1.TypeModifier_DateTime{DateTime: &irv1.DateTimeModifier{WithTimezone: true}},
				}
			}
			return nil
		}
		return &irv1.TypeModifier{
			Modifier: &irv1.TypeModifier_DateTime{DateTime: &irv1.DateTimeModifier{
				WithTimezone: tz,
				Precision:    uint32(nums[0]),
			}},
		}

	case "interval":
		if len(nums) == 0 {
			return nil
		}
		// interval(p): mapper stores precision and leaves Fields empty.
		return &irv1.TypeModifier{
			Modifier: &irv1.TypeModifier_Interval{Interval: &irv1.IntervalModifier{Precision: uint32(nums[0])}},
		}
	}
	return nil
}

// formatModifierNumbers extracts the integer arguments from the first
// parenthesized group of a format_type string, e.g. "numeric(10,2)" -> {10,2},
// "character varying(50)" -> {50}, "timestamp(3) with time zone" -> {3}. It
// returns nil when there is no parenthesized modifier (e.g. "integer", "text",
// "character varying").
func formatModifierNumbers(formatted string) []int {
	open := -1
	for i := 0; i < len(formatted); i++ {
		if formatted[i] == '(' {
			open = i
			break
		}
	}
	if open < 0 {
		return nil
	}
	close := -1
	for i := open + 1; i < len(formatted); i++ {
		if formatted[i] == ')' {
			close = i
			break
		}
	}
	if close < 0 {
		return nil
	}
	inner := formatted[open+1 : close]
	var out []int
	for _, part := range splitComma(inner) {
		part = trimSpace(part)
		n, ok := atoiSimple(part)
		if !ok {
			return nil
		}
		out = append(out, n)
	}
	return out
}

// splitComma splits on ',' without pulling in strings for a hot path.
func splitComma(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	for len(s) > 0 && (s[len(s)-1] == ' ' || s[len(s)-1] == '\t') {
		s = s[:len(s)-1]
	}
	return s
}

// atoiSimple parses a non-negative base-10 integer, returning ok=false for any
// non-digit content.
func atoiSimple(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, true
}
