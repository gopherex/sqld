// Code generated . DO NOT EDIT.
// This file is meant to be re-generated in place and/or deleted at any time.

package models

import (
	"context"
	"encoding/json"
	"time"

	"github.com/aarondl/opt/null"
	"github.com/google/uuid"
	"github.com/gopherex/sqld/example/gen/db"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stephenafamo/bob"
	"github.com/stephenafamo/bob/dialect/psql"
	"github.com/stephenafamo/bob/expr"
)

// AppKitchenSink is an object representing the database table.
type AppKitchenSink struct {
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

// AppKitchenSinkSlice is an alias for a slice of pointers to AppKitchenSink.
// This should almost always be used instead of []*AppKitchenSink.
type AppKitchenSinkSlice []*AppKitchenSink

// AppKitchenSinks contains methods to work with the kitchen_sink view
var AppKitchenSinks = psql.NewViewx[*AppKitchenSink, AppKitchenSinkSlice]("app", "kitchen_sink", buildAppKitchenSinkColumns("app.kitchen_sink"))

// AppKitchenSinksQuery is a query on the kitchen_sink view
type AppKitchenSinksQuery = *psql.ViewQuery[*AppKitchenSink, AppKitchenSinkSlice]

func buildAppKitchenSinkColumns(tableName string) appKitchenSinkColumns {
	columnsExpr := expr.NewColumnsExpr(
		"c_int2", "c_int4", "c_int8", "c_serial", "c_numeric", "c_float4", "c_float8", "c_bool", "c_text", "c_varchar", "c_char", "c_uuid", "c_bytea", "c_jsonb", "c_json", "c_inet", "c_date", "c_time", "c_timestamp", "c_timestamptz", "c_interval", "c_point", "c_tags", "c_ltree", "c_int_array", "c_text_array", "c_status", "c_address", "c_email", "c_int4range", "c_nummultirange", "c_generated", "c_identity",
	)

	if tableName != "" {
		columnsExpr = columnsExpr.WithParent(tableName)
	}

	return appKitchenSinkColumns{
		ColumnsExpr:    columnsExpr,
		tableAlias:     tableName,
		CInt2:          buildAppKitchenSinkColumn(tableName, "c_int2"),
		CInt4:          buildAppKitchenSinkColumn(tableName, "c_int4"),
		CInt8:          buildAppKitchenSinkColumn(tableName, "c_int8"),
		CSerial:        buildAppKitchenSinkColumn(tableName, "c_serial"),
		CNumeric:       buildAppKitchenSinkColumn(tableName, "c_numeric"),
		CFloat4:        buildAppKitchenSinkColumn(tableName, "c_float4"),
		CFloat8:        buildAppKitchenSinkColumn(tableName, "c_float8"),
		CBool:          buildAppKitchenSinkColumn(tableName, "c_bool"),
		CText:          buildAppKitchenSinkColumn(tableName, "c_text"),
		CVarchar:       buildAppKitchenSinkColumn(tableName, "c_varchar"),
		CChar:          buildAppKitchenSinkColumn(tableName, "c_char"),
		CUUID:          buildAppKitchenSinkColumn(tableName, "c_uuid"),
		CBytea:         buildAppKitchenSinkColumn(tableName, "c_bytea"),
		CJsonb:         buildAppKitchenSinkColumn(tableName, "c_jsonb"),
		CJSON:          buildAppKitchenSinkColumn(tableName, "c_json"),
		CInet:          buildAppKitchenSinkColumn(tableName, "c_inet"),
		CDate:          buildAppKitchenSinkColumn(tableName, "c_date"),
		CTime:          buildAppKitchenSinkColumn(tableName, "c_time"),
		CTimestamp:     buildAppKitchenSinkColumn(tableName, "c_timestamp"),
		CTimestamptz:   buildAppKitchenSinkColumn(tableName, "c_timestamptz"),
		CInterval:      buildAppKitchenSinkColumn(tableName, "c_interval"),
		CPoint:         buildAppKitchenSinkColumn(tableName, "c_point"),
		CTags:          buildAppKitchenSinkColumn(tableName, "c_tags"),
		CLtree:         buildAppKitchenSinkColumn(tableName, "c_ltree"),
		CIntArray:      buildAppKitchenSinkColumn(tableName, "c_int_array"),
		CTextArray:     buildAppKitchenSinkColumn(tableName, "c_text_array"),
		CStatus:        buildAppKitchenSinkColumn(tableName, "c_status"),
		CAddress:       buildAppKitchenSinkColumn(tableName, "c_address"),
		CEmail:         buildAppKitchenSinkColumn(tableName, "c_email"),
		CInt4range:     buildAppKitchenSinkColumn(tableName, "c_int4range"),
		CNummultirange: buildAppKitchenSinkColumn(tableName, "c_nummultirange"),
		CGenerated:     buildAppKitchenSinkColumn(tableName, "c_generated"),
		CIdentity:      buildAppKitchenSinkColumn(tableName, "c_identity"),
	}
}

type appKitchenSinkColumns struct {
	expr.ColumnsExpr
	tableAlias     string
	CInt2          appKitchenSinkColumn
	CInt4          appKitchenSinkColumn
	CInt8          appKitchenSinkColumn
	CSerial        appKitchenSinkColumn
	CNumeric       appKitchenSinkColumn
	CFloat4        appKitchenSinkColumn
	CFloat8        appKitchenSinkColumn
	CBool          appKitchenSinkColumn
	CText          appKitchenSinkColumn
	CVarchar       appKitchenSinkColumn
	CChar          appKitchenSinkColumn
	CUUID          appKitchenSinkColumn
	CBytea         appKitchenSinkColumn
	CJsonb         appKitchenSinkColumn
	CJSON          appKitchenSinkColumn
	CInet          appKitchenSinkColumn
	CDate          appKitchenSinkColumn
	CTime          appKitchenSinkColumn
	CTimestamp     appKitchenSinkColumn
	CTimestamptz   appKitchenSinkColumn
	CInterval      appKitchenSinkColumn
	CPoint         appKitchenSinkColumn
	CTags          appKitchenSinkColumn
	CLtree         appKitchenSinkColumn
	CIntArray      appKitchenSinkColumn
	CTextArray     appKitchenSinkColumn
	CStatus        appKitchenSinkColumn
	CAddress       appKitchenSinkColumn
	CEmail         appKitchenSinkColumn
	CInt4range     appKitchenSinkColumn
	CNummultirange appKitchenSinkColumn
	CGenerated     appKitchenSinkColumn
	CIdentity      appKitchenSinkColumn
}

// Alias returns the current table alias for the columns set.
func (c appKitchenSinkColumns) Alias() string {
	return c.tableAlias
}

// AliasedAs returns a copy of the columns set qualified by tableName.
func (appKitchenSinkColumns) AliasedAs(tableName string) appKitchenSinkColumns {
	return buildAppKitchenSinkColumns(tableName)
}

// Unqualified returns a copy of the columns set without table qualification.
func (c appKitchenSinkColumns) Unqualified() appKitchenSinkColumns {
	return buildAppKitchenSinkColumns("")
}

func buildAppKitchenSinkColumn(alias, name string) appKitchenSinkColumn {
	return appKitchenSinkColumn{
		Expression: psql.Quote(alias, name),
		alias:      alias,
		name:       name,
	}
}

type appKitchenSinkColumn struct {
	psql.Expression
	alias string
	name  string
}

// Name returns the unqualified column name.
func (c appKitchenSinkColumn) Name() string {
	return c.name
}

// ShouldOmitParens prevents automatic parenthesis wrapping in expression builders.
func (c appKitchenSinkColumn) ShouldOmitParens() bool {
	return true
}

// AfterQueryHook is called after AppKitchenSink is retrieved from the database
func (o *AppKitchenSink) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppKitchenSinks.AfterSelectHooks.RunHooks(ctx, exec, AppKitchenSinkSlice{o})
	}

	return err
}

// AfterQueryHook is called after AppKitchenSinkSlice is retrieved from the database
func (o AppKitchenSinkSlice) AfterQueryHook(ctx context.Context, exec bob.Executor, queryType bob.QueryType) error {
	var err error

	switch queryType {
	case bob.QueryTypeSelect:
		ctx, err = AppKitchenSinks.AfterSelectHooks.RunHooks(ctx, exec, o)
	}

	return err
}

type appKitchenSinkWhere[Q psql.Filterable] struct {
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

func (appKitchenSinkWhere[Q]) AliasedAs(alias string) appKitchenSinkWhere[Q] {
	return buildAppKitchenSinkWhere[Q](buildAppKitchenSinkColumns(alias))
}

func buildAppKitchenSinkWhere[Q psql.Filterable](cols appKitchenSinkColumns) appKitchenSinkWhere[Q] {
	return appKitchenSinkWhere[Q]{
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
