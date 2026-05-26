package introspect

import (
	"context"
	"strings"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
)

// loadIndexes reads pg_index for the relations in the wanted schemas and
// attaches Index entries to their owning Table.
//
// Indexes that merely back a PrimaryKey or UNIQUE constraint are still
// recorded, but flagged: indisprimary sets Index.Primary, and constraint-owned
// unique indexes set Index.Unique. The diff engine can decide whether to treat
// them as redundant; we keep them so an introspected index list is complete.
func (b *builder) loadIndexes(ctx context.Context, schemas []string) error {
	// Column names are resolved in SQL. keycol_names holds one entry per key
	// attribute (in order); a NULL entry marks an expression key (attnum 0).
	// inc_names holds the INCLUDE (covering) column names.
	const q = `
SELECT i.indrelid                                AS table_oid,
       ic.relname                                AS index_name,
       am.amname                                 AS method,
       i.indisunique                             AS is_unique,
       i.indisprimary                            AS is_primary,
       (SELECT array_agg(a.attname ORDER BY k.ord)
        FROM unnest((string_to_array(i.indkey::text,' ')::int2[])[1:i.indnkeyatts])
             WITH ORDINALITY AS k(attnum, ord)
        LEFT JOIN pg_catalog.pg_attribute a
          ON a.attrelid = i.indrelid AND a.attnum = k.attnum) AS keycol_names,
       (SELECT array_agg(a.attname ORDER BY k.ord)
        FROM unnest((string_to_array(i.indkey::text,' ')::int2[])[i.indnkeyatts+1:i.indnatts])
             WITH ORDINALITY AS k(attnum, ord)
        JOIN pg_catalog.pg_attribute a
          ON a.attrelid = i.indrelid AND a.attnum = k.attnum) AS inc_names,
       pg_catalog.pg_get_expr(i.indpred, i.indrelid) AS predicate,
       pg_catalog.pg_get_indexdef(ic.oid)            AS indexdef
FROM pg_catalog.pg_index i
JOIN pg_catalog.pg_class ic ON ic.oid = i.indexrelid
JOIN pg_catalog.pg_class tc ON tc.oid = i.indrelid
JOIN pg_catalog.pg_namespace n ON n.oid = tc.relnamespace
JOIN pg_catalog.pg_am am ON am.oid = ic.relam
WHERE n.nspname = ANY($1)
  AND tc.relkind = 'r'
ORDER BY n.nspname, tc.relname, ic.relname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			tableOID    uint32
			indexName   string
			method      string
			isUnique    bool
			isPrimary   bool
			keycolNames []*string
			incNames    []string
			predicate   *string
			indexdef    string
		)
		if err := rows.Scan(&tableOID, &indexName, &method, &isUnique,
			&isPrimary, &keycolNames, &incNames, &predicate, &indexdef); err != nil {
			return err
		}

		tbl := b.relTables[tableOID]
		if tbl == nil {
			continue
		}

		idx := &irv1.Index{
			Id:      tbl.GetId() + "." + indexName,
			Name:    indexName,
			Method:  method,
			Unique:  isUnique,
			Primary: isPrimary,
			Include: incNames,
		}
		if predicate != nil {
			idx.Predicate = rawExpr(*predicate)
		}

		// A NULL key name marks an expression element; its text comes from the
		// index definition's key list.
		for pos, name := range keycolNames {
			if name == nil {
				idx.Elements = append(idx.Elements, &irv1.IndexElement{
					Target: &irv1.IndexElement_Expr{Expr: rawExpr(indexElementExpr(indexdef, pos))},
				})
				continue
			}
			idx.Elements = append(idx.Elements, &irv1.IndexElement{
				Target: &irv1.IndexElement_Column{Column: *name},
			})
		}

		tbl.Indexes = append(tbl.Indexes, idx)
	}
	return rows.Err()
}

// loadViews reads relkind 'v' (views) and 'm' (materialized views) and stores
// the view definition as a raw SQL SelectStmt escape hatch.
func (b *builder) loadViews(ctx context.Context, schemas []string) error {
	const q = `
SELECT n.nspname,
       c.relname,
       c.relkind::text,
       pg_catalog.pg_get_viewdef(c.oid, true) AS def
FROM pg_catalog.pg_class c
JOIN pg_catalog.pg_namespace n ON n.oid = c.relnamespace
WHERE c.relkind IN ('v','m')
  AND n.nspname = ANY($1)
ORDER BY n.nspname, c.relname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			schema  string
			name    string
			relkind string
			def     string
		)
		if err := rows.Scan(&schema, &name, &relkind, &def); err != nil {
			return err
		}
		query := &irv1.SelectStmt{RawSql: strings.TrimSpace(def)}
		sch := b.getSchema(schema)
		if relkind == "m" {
			sch.MaterializedViews = append(sch.MaterializedViews, &irv1.MaterializedView{
				Id:    schema + "." + name,
				Name:  qname(schema, name),
				Query: query,
			})
		} else {
			sch.Views = append(sch.Views, &irv1.View{
				Id:    schema + "." + name,
				Name:  qname(schema, name),
				Query: query,
			})
		}
	}
	return rows.Err()
}

// loadRoutines reads pg_proc for functions (prokind 'f') and procedures
// (prokind 'p'). Aggregate ('a') and window ('w') functions are skipped.
func (b *builder) loadRoutines(ctx context.Context, schemas []string) error {
	const q = `
SELECT n.nspname,
       p.proname,
       p.prokind::text,
       l.lanname,
       p.prosrc,
       p.provolatile::text,
       p.proisstrict,
       p.prosecdef,
       p.proleakproof,
       p.proparallel::text,
       p.prorettype,
       pg_catalog.format_type(p.prorettype, -1) AS ret_formatted,
       p.proretset,
       p.proargnames,
       p.proargmodes::text[],
       p.proargtypes::oid[] AS argtypes
FROM pg_catalog.pg_proc p
JOIN pg_catalog.pg_namespace n ON n.oid = p.pronamespace
JOIN pg_catalog.pg_language l ON l.oid = p.prolang
WHERE n.nspname = ANY($1)
  AND p.prokind IN ('f','p')
ORDER BY n.nspname, p.proname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			schema    string
			name      string
			prokind   string
			lang      string
			src       string
			volatile  string
			isstrict  bool
			secdef    bool
			leakproof bool
			parallel  string
			rettype   uint32
			retFmt    string
			retset    bool
			argNames  []string
			argModes  []string
			argTypes  []uint32
		)
		if err := rows.Scan(&schema, &name, &prokind, &lang, &src, &volatile,
			&isstrict, &secdef, &leakproof, &parallel, &rettype, &retFmt,
			&retset, &argNames, &argModes, &argTypes); err != nil {
			return err
		}

		args := b.buildArguments(argTypes, argNames, argModes)
		sch := b.getSchema(schema)

		if prokind == "p" {
			sch.Procedures = append(sch.Procedures, &irv1.Procedure{
				Id:              schema + "." + name,
				Name:            qname(schema, name),
				Arguments:       args,
				Language:        lang,
				SecurityDefiner: secdef,
				Body:            src,
			})
			continue
		}

		fn := &irv1.Function{
			Id:              schema + "." + name,
			Name:            qname(schema, name),
			Arguments:       args,
			Returns:         b.buildReturnType(rettype, retFmt, retset),
			Language:        lang,
			Volatility:      volatility(volatile),
			NullInput:       nullInput(isstrict),
			SecurityDefiner: secdef,
			Leakproof:       leakproof,
			Parallel:        parallelSafety(parallel),
			Body:            src,
		}
		sch.Functions = append(sch.Functions, fn)
	}
	return rows.Err()
}

// buildArguments maps parallel proargtypes/proargnames/proargmodes arrays into
// IR Arguments. proargnames/proargmodes may be nil (all IN, positional).
func (b *builder) buildArguments(types []uint32, names, modes []string) []*irv1.Argument {
	var out []*irv1.Argument
	for i, toid := range types {
		arg := &irv1.Argument{Type: b.scalarOrUserRef(toid), Mode: irv1.ArgMode_ARG_MODE_IN}
		if i < len(names) {
			arg.Name = names[i]
		}
		if i < len(modes) {
			arg.Mode = argMode(modes[i])
		}
		out = append(out, arg)
	}
	return out
}

// buildReturnType classifies a function return type.
func (b *builder) buildReturnType(oid uint32, formatted string, setof bool) *irv1.ReturnType {
	ref := b.scalarOrUserRef(oid)
	if ref.GetPgName() == "" {
		ref.PgName = basePgName(formatted)
	}
	switch {
	case ref.GetPgName() == "void":
		return &irv1.ReturnType{Kind: &irv1.ReturnType_VoidValue{VoidValue: true}}
	case ref.GetPgName() == "trigger" || ref.GetPgName() == "event_trigger":
		return &irv1.ReturnType{Kind: &irv1.ReturnType_TriggerValue{TriggerValue: true}}
	case setof:
		return &irv1.ReturnType{Kind: &irv1.ReturnType_Setof{Setof: &irv1.SetofType{Type: ref}}}
	default:
		return &irv1.ReturnType{Kind: &irv1.ReturnType_Scalar{Scalar: ref}}
	}
}

// loadTriggers reads non-internal triggers (pg_trigger.tgisinternal = false)
// and resolves their table and function ObjectRefs.
func (b *builder) loadTriggers(ctx context.Context, schemas []string) error {
	const q = `
SELECT n.nspname        AS table_schema,
       tc.relname       AS table_name,
       tg.tgname        AS trigger_name,
       tg.tgtype        AS tgtype,
       fn_ns.nspname    AS fn_schema,
       p.proname        AS fn_name,
       tg.tgconstraint <> 0 AS is_constraint,
       pg_catalog.pg_get_expr(tg.tgqual, tg.tgrelid) AS when_expr
FROM pg_catalog.pg_trigger tg
JOIN pg_catalog.pg_class tc ON tc.oid = tg.tgrelid
JOIN pg_catalog.pg_namespace n ON n.oid = tc.relnamespace
JOIN pg_catalog.pg_proc p ON p.oid = tg.tgfoid
JOIN pg_catalog.pg_namespace fn_ns ON fn_ns.oid = p.pronamespace
WHERE n.nspname = ANY($1)
  AND NOT tg.tgisinternal
ORDER BY n.nspname, tc.relname, tg.tgname`
	rows, err := b.conn.Query(ctx, q, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			tableSchema string
			tableName   string
			trigName    string
			tgtype      int16
			fnSchema    string
			fnName      string
			isConstr    bool
			whenExpr    *string
		)
		if err := rows.Scan(&tableSchema, &tableName, &trigName, &tgtype,
			&fnSchema, &fnName, &isConstr, &whenExpr); err != nil {
			return err
		}

		timing, events, level := decodeTgType(uint16(tgtype))
		trig := &irv1.Trigger{
			Id:     tableSchema + "." + trigName,
			Name:   trigName,
			Timing: timing,
			Events: events,
			Level:  level,
			Table: &irv1.ObjectRef{
				Id:   tableSchema + "." + tableName,
				Kind: irv1.ObjectKind_OBJECT_KIND_TABLE,
				Name: qname(tableSchema, tableName),
			},
			Function: &irv1.ObjectRef{
				Id:   fnSchema + "." + fnName,
				Kind: irv1.ObjectKind_OBJECT_KIND_FUNCTION,
				Name: qname(fnSchema, fnName),
			},
			Constraint: isConstr,
		}
		if whenExpr != nil {
			trig.When = rawExpr(*whenExpr)
		}

		// Place the trigger in the table's schema bucket and add a back-ref to
		// the table, mirroring internal/catalog.
		sch := b.getSchema(tableSchema)
		sch.Triggers = append(sch.Triggers, trig)
		if tbl, ok := b.tableByKey[tableKey{tableSchema, tableName}]; ok {
			tbl.Triggers = append(tbl.Triggers, &irv1.ObjectRef{
				Id:   trig.GetId(),
				Kind: irv1.ObjectKind_OBJECT_KIND_TRIGGER,
				Name: qname(tableSchema, trigName),
			})
		}
	}
	return rows.Err()
}

// indexElementExpr is a best-effort extractor that returns the whole index
// definition's expression list when the position-specific text cannot be
// isolated. The diff engine compares index expressions textually, so storing
// the full indexdef fragment is acceptable as a fallback.
func indexElementExpr(indexdef string, _ int) string {
	open := strings.IndexByte(indexdef, '(')
	if open < 0 {
		return indexdef
	}
	// Find the matching close paren of the key list.
	depth := 0
	for i := open; i < len(indexdef); i++ {
		switch indexdef[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return strings.TrimSpace(indexdef[open+1 : i])
			}
		}
	}
	return strings.TrimSpace(indexdef[open+1:])
}

// ---------------------------------------------------------------------------
// tgtype decoding (see PostgreSQL src/include/catalog/pg_trigger.h)
// ---------------------------------------------------------------------------

const (
	triggerTypeRow      = 1 << 0
	triggerTypeBefore   = 1 << 1
	triggerTypeInsert   = 1 << 2
	triggerTypeDelete   = 1 << 3
	triggerTypeUpdate   = 1 << 4
	triggerTypeTruncate = 1 << 5
	triggerTypeInstead  = 1 << 6
)

func decodeTgType(t uint16) (irv1.TriggerTiming, []irv1.TriggerEvent, irv1.TriggerLevel) {
	var timing irv1.TriggerTiming
	switch {
	case t&triggerTypeInstead != 0:
		timing = irv1.TriggerTiming_TRIGGER_TIMING_INSTEAD_OF
	case t&triggerTypeBefore != 0:
		timing = irv1.TriggerTiming_TRIGGER_TIMING_BEFORE
	default:
		timing = irv1.TriggerTiming_TRIGGER_TIMING_AFTER
	}

	var events []irv1.TriggerEvent
	if t&triggerTypeInsert != 0 {
		events = append(events, irv1.TriggerEvent_TRIGGER_EVENT_INSERT)
	}
	if t&triggerTypeUpdate != 0 {
		events = append(events, irv1.TriggerEvent_TRIGGER_EVENT_UPDATE)
	}
	if t&triggerTypeDelete != 0 {
		events = append(events, irv1.TriggerEvent_TRIGGER_EVENT_DELETE)
	}
	if t&triggerTypeTruncate != 0 {
		events = append(events, irv1.TriggerEvent_TRIGGER_EVENT_TRUNCATE)
	}

	level := irv1.TriggerLevel_TRIGGER_LEVEL_STATEMENT
	if t&triggerTypeRow != 0 {
		level = irv1.TriggerLevel_TRIGGER_LEVEL_ROW
	}
	return timing, events, level
}

// ---------------------------------------------------------------------------
// scalar enum mappers for routines
// ---------------------------------------------------------------------------

func volatility(c string) irv1.Volatility {
	switch c {
	case "i":
		return irv1.Volatility_VOLATILITY_IMMUTABLE
	case "s":
		return irv1.Volatility_VOLATILITY_STABLE
	case "v":
		return irv1.Volatility_VOLATILITY_VOLATILE
	default:
		return irv1.Volatility_VOLATILITY_UNSPECIFIED
	}
}

func nullInput(strict bool) irv1.NullInputBehavior {
	if strict {
		return irv1.NullInputBehavior_NULL_INPUT_BEHAVIOR_STRICT
	}
	return irv1.NullInputBehavior_NULL_INPUT_BEHAVIOR_CALLED
}

func parallelSafety(c string) irv1.ParallelSafety {
	switch c {
	case "s":
		return irv1.ParallelSafety_PARALLEL_SAFETY_SAFE
	case "r":
		return irv1.ParallelSafety_PARALLEL_SAFETY_RESTRICTED
	case "u":
		return irv1.ParallelSafety_PARALLEL_SAFETY_UNSAFE
	default:
		return irv1.ParallelSafety_PARALLEL_SAFETY_UNSPECIFIED
	}
}

func argMode(c string) irv1.ArgMode {
	switch c {
	case "i":
		return irv1.ArgMode_ARG_MODE_IN
	case "o":
		return irv1.ArgMode_ARG_MODE_OUT
	case "b":
		return irv1.ArgMode_ARG_MODE_INOUT
	case "v":
		return irv1.ArgMode_ARG_MODE_VARIADIC
	case "t":
		// TABLE column output mode; treat as OUT for IR purposes.
		return irv1.ArgMode_ARG_MODE_OUT
	default:
		return irv1.ArgMode_ARG_MODE_IN
	}
}
