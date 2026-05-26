// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aarondl/opt/null"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/expr"
	"github.com/yaroher/sqld/example/gen/db"
)

// KitchenSink is an object representing the database table.
type KitchenSink struct {
	CInt2          null.Val[int16]                                           `db:"c_int2" `
	CInt4          null.Val[int32]                                           `db:"c_int4" `
	CInt8          null.Val[int64]                                           `db:"c_int8" `
	CSerial        null.Val[int32]                                           `db:"c_serial" `
	CNumeric       null.Val[string]                                          `db:"c_numeric" `
	CFloat4        null.Val[float32]                                         `db:"c_float4" `
	CFloat8        null.Val[float64]                                         `db:"c_float8" `
	CBool          null.Val[bool]                                            `db:"c_bool" `
	CText          null.Val[string]                                          `db:"c_text" `
	CVarchar       null.Val[string]                                          `db:"c_varchar" `
	CChar          null.Val[string]                                          `db:"c_char" `
	CUUID          null.Val[uuid.UUID]                                       `db:"c_uuid" `
	CBytea         null.Val[[]byte]                                          `db:"c_bytea" `
	CJsonb         null.Val[map[string]any]                                  `db:"c_jsonb" `
	CJSON          null.Val[json.RawMessage]                                 `db:"c_json" `
	CInet          null.Val[string]                                          `db:"c_inet" `
	CDate          null.Val[time.Time]                                       `db:"c_date" `
	CTime          null.Val[time.Time]                                       `db:"c_time" `
	CTimestamp     null.Val[time.Time]                                       `db:"c_timestamp" `
	CTimestamptz   null.Val[time.Time]                                       `db:"c_timestamptz" `
	CInterval      null.Val[pgtype.Interval]                                 `db:"c_interval" `
	CPoint         null.Val[pgtype.Point]                                    `db:"c_point" `
	CTags          null.Val[pgtype.Hstore]                                   `db:"c_tags" `
	CLtree         null.Val[string]                                          `db:"c_ltree" `
	CIntArray      null.Val[[]int32]                                         `db:"c_int_array" `
	CTextArray     null.Val[[]string]                                        `db:"c_text_array" `
	CStatus        null.Val[db.AppUserStatus]                                `db:"c_status" `
	CAddress       null.Val[db.AppAddress]                                   `db:"c_address" `
	CEmail         null.Val[string]                                          `db:"c_email" `
	CInt4range     null.Val[pgtype.Range[pgtype.Int4]]                       `db:"c_int4range" `
	CNummultirange null.Val[pgtype.Multirange[pgtype.Range[pgtype.Numeric]]] `db:"c_nummultirange" `
	CGenerated     null.Val[int32]                                           `db:"c_generated,generated" `
	CIdentity      int32                                                     `db:"c_identity" `
}

// KitchenSinkSlice is an alias for a slice of pointers to KitchenSink.
// This should almost always be used instead of []*KitchenSink.
type KitchenSinkSlice []*KitchenSink

// KitchenSinks contains methods to work with the kitchen_sink view
var KitchenSinks = psql.NewViewx[*KitchenSink, KitchenSinkSlice]("app", "kitchen_sink", buildKitchenSinkColumns("kitchen_sink"))

// KitchenSinksQuery is a query on the kitchen_sink view
type KitchenSinksQuery = *psql.ViewQuery[*KitchenSink, KitchenSinkSlice]

func buildKitchenSinkColumns(tableName string) kitchenSinkColumns {
	columnsExpr := expr.NewColumnsExpr(
		"c_int2", "c_int4", "c_int8", "c_serial", "c_numeric", "c_float4", "c_float8", "c_bool", "c_text", "c_varchar", "c_char", "c_uuid", "c_bytea", "c_jsonb", "c_json", "c_inet", "c_date", "c_time", "c_timestamp", "c_timestamptz", "c_interval", "c_point", "c_tags", "c_ltree", "c_int_array", "c_text_array", "c_status", "c_address", "c_email", "c_int4range", "c_nummultirange", "c_generated", "c_identity",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return kitchenSinkColumns{
		ColumnsExpr:    columnsExpr,
		tableAlias:     tableName,
		CInt2:          buildKitchenSinkColumn(tableName, "c_int2"),
		CInt4:          buildKitchenSinkColumn(tableName, "c_int4"),
		CInt8:          buildKitchenSinkColumn(tableName, "c_int8"),
		CSerial:        buildKitchenSinkColumn(tableName, "c_serial"),
		CNumeric:       buildKitchenSinkColumn(tableName, "c_numeric"),
		CFloat4:        buildKitchenSinkColumn(tableName, "c_float4"),
		CFloat8:        buildKitchenSinkColumn(tableName, "c_float8"),
		CBool:          buildKitchenSinkColumn(tableName, "c_bool"),
		CText:          buildKitchenSinkColumn(tableName, "c_text"),
		CVarchar:       buildKitchenSinkColumn(tableName, "c_varchar"),
		CChar:          buildKitchenSinkColumn(tableName, "c_char"),
		CUUID:          buildKitchenSinkColumn(tableName, "c_uuid"),
		CBytea:         buildKitchenSinkColumn(tableName, "c_bytea"),
		CJsonb:         buildKitchenSinkColumn(tableName, "c_jsonb"),
		CJSON:          buildKitchenSinkColumn(tableName, "c_json"),
		CInet:          buildKitchenSinkColumn(tableName, "c_inet"),
		CDate:          buildKitchenSinkColumn(tableName, "c_date"),
		CTime:          buildKitchenSinkColumn(tableName, "c_time"),
		CTimestamp:     buildKitchenSinkColumn(tableName, "c_timestamp"),
		CTimestamptz:   buildKitchenSinkColumn(tableName, "c_timestamptz"),
		CInterval:      buildKitchenSinkColumn(tableName, "c_interval"),
		CPoint:         buildKitchenSinkColumn(tableName, "c_point"),
		CTags:          buildKitchenSinkColumn(tableName, "c_tags"),
		CLtree:         buildKitchenSinkColumn(tableName, "c_ltree"),
		CIntArray:      buildKitchenSinkColumn(tableName, "c_int_array"),
		CTextArray:     buildKitchenSinkColumn(tableName, "c_text_array"),
		CStatus:        buildKitchenSinkColumn(tableName, "c_status"),
		CAddress:       buildKitchenSinkColumn(tableName, "c_address"),
		CEmail:         buildKitchenSinkColumn(tableName, "c_email"),
		CInt4range:     buildKitchenSinkColumn(tableName, "c_int4range"),
		CNummultirange: buildKitchenSinkColumn(tableName, "c_nummultirange"),
		CGenerated:     buildKitchenSinkColumn(tableName, "c_generated"),
		CIdentity:      buildKitchenSinkColumn(tableName, "c_identity"),
	}
}

type kitchenSinkColumns struct {
	expr.ColumnsExpr
	tableAlias     string
	CInt2          kitchenSinkColumn
	CInt4          kitchenSinkColumn
	CInt8          kitchenSinkColumn
	CSerial        kitchenSinkColumn
	CNumeric       kitchenSinkColumn
	CFloat4        kitchenSinkColumn
	CFloat8        kitchenSinkColumn
	CBool          kitchenSinkColumn
	CText          kitchenSinkColumn
	CVarchar       kitchenSinkColumn
	CChar          kitchenSinkColumn
	CUUID          kitchenSinkColumn
	CBytea         kitchenSinkColumn
	CJsonb         kitchenSinkColumn
	CJSON          kitchenSinkColumn
	CInet          kitchenSinkColumn
	CDate          kitchenSinkColumn
	CTime          kitchenSinkColumn
	CTimestamp     kitchenSinkColumn
	CTimestamptz   kitchenSinkColumn
	CInterval      kitchenSinkColumn
	CPoint         kitchenSinkColumn
	CTags          kitchenSinkColumn
	CLtree         kitchenSinkColumn
	CIntArray      kitchenSinkColumn
	CTextArray     kitchenSinkColumn
	CStatus        kitchenSinkColumn
	CAddress       kitchenSinkColumn
	CEmail         kitchenSinkColumn
	CInt4range     kitchenSinkColumn
	CNummultirange kitchenSinkColumn
	CGenerated     kitchenSinkColumn
	CIdentity      kitchenSinkColumn
}

// Alias returns the current table alias for the columns set.
func (c kitchenSinkColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (kitchenSinkColumns) AliasedAs(tableName string) kitchenSinkColumns {
	return buildKitchenSinkColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c kitchenSinkColumns) Unqualified() kitchenSinkColumns {
	return buildKitchenSinkColumns("")
}

func buildKitchenSinkColumn(alias, name string) kitchenSinkColumn {
	return kitchenSinkColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type kitchenSinkColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c kitchenSinkColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c kitchenSinkColumn) ShouldOmitParens() bool {
	return true
}

// AfterQueryHook is called after KitchenSink is retrieved from the database
func (o *KitchenSink) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = KitchenSinks.AfterSelectHooks.RunHooks(ctx, exec, KitchenSinkSlice{o})
	}

	return err
}

// AfterQueryHook is called after KitchenSinkSlice is retrieved from the database
func (o KitchenSinkSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = KitchenSinks.AfterSelectHooks.RunHooks(ctx, exec, o)
	}

	return err
}

type kitchenSinkWhere[Q psql.Filterable] struct {
	CInt2          psql.WhereNullMod[Q, int16]
	CInt4          psql.WhereNullMod[Q, int32]
	CInt8          psql.WhereNullMod[Q, int64]
	CSerial        psql.WhereNullMod[Q, int32]
	CNumeric       psql.WhereNullMod[Q, string]
	CFloat4        psql.WhereNullMod[Q, float32]
	CFloat8        psql.WhereNullMod[Q, float64]
	CBool          psql.WhereNullMod[Q, bool]
	CText          psql.WhereNullMod[Q, string]
	CVarchar       psql.WhereNullMod[Q, string]
	CChar          psql.WhereNullMod[Q, string]
	CUUID          psql.WhereNullMod[Q, uuid.UUID]
	CBytea         psql.WhereNullMod[Q, []byte]
	CJsonb         psql.WhereNullMod[Q, map[string]any]
	CJSON          psql.WhereNullMod[Q, json.RawMessage]
	CInet          psql.WhereNullMod[Q, string]
	CDate          psql.WhereNullMod[Q, time.Time]
	CTime          psql.WhereNullMod[Q, time.Time]
	CTimestamp     psql.WhereNullMod[Q, time.Time]
	CTimestamptz   psql.WhereNullMod[Q, time.Time]
	CInterval      psql.WhereNullMod[Q, pgtype.Interval]
	CPoint         psql.WhereNullMod[Q, pgtype.Point]
	CTags          psql.WhereNullMod[Q, pgtype.Hstore]
	CLtree         psql.WhereNullMod[Q, string]
	CIntArray      psql.WhereNullMod[Q, []int32]
	CTextArray     psql.WhereNullMod[Q, []string]
	CStatus        psql.WhereNullMod[Q, db.AppUserStatus]
	CAddress       psql.WhereNullMod[Q, db.AppAddress]
	CEmail         psql.WhereNullMod[Q, string]
	CInt4range     psql.WhereNullMod[Q, pgtype.Range[pgtype.Int4]]
	CNummultirange psql.WhereNullMod[Q, pgtype.Multirange[pgtype.Range[pgtype.Numeric]]]
	CGenerated     psql.WhereNullMod[Q, int32]
	CIdentity      psql.WhereMod[Q, int32]
}

func (kitchenSinkWhere[Q]) AliasedAs(alias string) kitchenSinkWhere[Q] {
	return buildKitchenSinkWhere[Q](buildKitchenSinkColumns(alias))
}

func buildKitchenSinkWhere[Q psql.Filterable](cols kitchenSinkColumns) kitchenSinkWhere[Q] {
	return kitchenSinkWhere[Q]{
		CInt2:          psql.WhereNull[Q, int16](cols.CInt2.Expression),
		CInt4:          psql.WhereNull[Q, int32](cols.CInt4.Expression),
		CInt8:          psql.WhereNull[Q, int64](cols.CInt8.Expression),
		CSerial:        psql.WhereNull[Q, int32](cols.CSerial.Expression),
		CNumeric:       psql.WhereNull[Q, string](cols.CNumeric.Expression),
		CFloat4:        psql.WhereNull[Q, float32](cols.CFloat4.Expression),
		CFloat8:        psql.WhereNull[Q, float64](cols.CFloat8.Expression),
		CBool:          psql.WhereNull[Q, bool](cols.CBool.Expression),
		CText:          psql.WhereNull[Q, string](cols.CText.Expression),
		CVarchar:       psql.WhereNull[Q, string](cols.CVarchar.Expression),
		CChar:          psql.WhereNull[Q, string](cols.CChar.Expression),
		CUUID:          psql.WhereNull[Q, uuid.UUID](cols.CUUID.Expression),
		CBytea:         psql.WhereNull[Q, []byte](cols.CBytea.Expression),
		CJsonb:         psql.WhereNull[Q, map[string]any](cols.CJsonb.Expression),
		CJSON:          psql.WhereNull[Q, json.RawMessage](cols.CJSON.Expression),
		CInet:          psql.WhereNull[Q, string](cols.CInet.Expression),
		CDate:          psql.WhereNull[Q, time.Time](cols.CDate.Expression),
		CTime:          psql.WhereNull[Q, time.Time](cols.CTime.Expression),
		CTimestamp:     psql.WhereNull[Q, time.Time](cols.CTimestamp.Expression),
		CTimestamptz:   psql.WhereNull[Q, time.Time](cols.CTimestamptz.Expression),
		CInterval:      psql.WhereNull[Q, pgtype.Interval](cols.CInterval.Expression),
		CPoint:         psql.WhereNull[Q, pgtype.Point](cols.CPoint.Expression),
		CTags:          psql.WhereNull[Q, pgtype.Hstore](cols.CTags.Expression),
		CLtree:         psql.WhereNull[Q, string](cols.CLtree.Expression),
		CIntArray:      psql.WhereNull[Q, []int32](cols.CIntArray.Expression),
		CTextArray:     psql.WhereNull[Q, []string](cols.CTextArray.Expression),
		CStatus:        psql.WhereNull[Q, db.AppUserStatus](cols.CStatus.Expression),
		CAddress:       psql.WhereNull[Q, db.AppAddress](cols.CAddress.Expression),
		CEmail:         psql.WhereNull[Q, string](cols.CEmail.Expression),
		CInt4range:     psql.WhereNull[Q, pgtype.Range[pgtype.Int4]](cols.CInt4range.Expression),
		CNummultirange: psql.WhereNull[Q, pgtype.Multirange[pgtype.Range[pgtype.Numeric]]](cols.CNummultirange.Expression),
		CGenerated:     psql.WhereNull[Q, int32](cols.CGenerated.Expression),
		CIdentity:      psql.Where[Q, int32](cols.CIdentity.Expression),
	}
}
