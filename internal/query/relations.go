package query

import (
	"strings"

	"github.com/gopherex/sqld/internal/catalog"
	irv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/gopherex/sqld/pkg/proto/sqld/v1/plugin"
)

type relationName struct{ schema, name string }

// Views share relation/alias resolution with tables. Their projected columns
// are inferred lazily so dependencies need not appear in catalog order.
type catalogIndex struct {
	tables    map[string]*irv1.Table
	qualified map[relationName]*irv1.Table
	views     map[*irv1.Table]*viewDefinition
	diags     *catalog.Diagnostics
}

type viewDefinition struct {
	query               *irv1.SelectStmt
	aliases             []string
	resolving, resolved bool
}

func buildCatalogIndex(cat *irv1.Catalog) catalogIndex {
	idx := catalogIndex{tables: make(map[string]*irv1.Table), qualified: make(map[relationName]*irv1.Table), views: make(map[*irv1.Table]*viewDefinition), diags: &catalog.Diagnostics{}}
	add := func(schema string, tbl *irv1.Table) {
		name := strings.ToLower(tbl.GetName().GetName())
		idx.qualified[relationName{strings.ToLower(schema), name}] = tbl
		if idx.tables[name] == nil {
			idx.tables[name] = tbl
		}
	}
	for _, schema := range cat.GetSchemas() {
		for _, tbl := range schema.GetTables() {
			add(schema.GetName(), tbl)
		}
		view := func(name *irv1.QualifiedName, query *irv1.SelectStmt, aliases []string) {
			tbl := &irv1.Table{Id: schema.GetName() + "." + name.GetName(), Name: name}
			add(schema.GetName(), tbl)
			idx.views[tbl] = &viewDefinition{query: query, aliases: aliases}
		}
		for _, v := range schema.GetViews() {
			view(v.GetName(), v.GetQuery(), v.GetColumns())
		}
		for _, v := range schema.GetMaterializedViews() {
			view(v.GetName(), v.GetQuery(), v.GetColumns())
		}
	}
	return idx
}

func (ci catalogIndex) lookupRelation(name *irv1.QualifiedName) *irv1.Table {
	if name.GetSchema() == "" {
		return ci.lookupTable(name.GetName())
	}
	return ci.resolveView(ci.qualified[relationName{strings.ToLower(name.GetSchema()), strings.ToLower(name.GetName())}])
}

func (ci catalogIndex) lookupTable(name string) *irv1.Table {
	return ci.resolveView(ci.tables[strings.ToLower(name)])
}

func (ci catalogIndex) resolveView(tbl *irv1.Table) *irv1.Table {
	v := ci.views[tbl]
	if v == nil || v.resolved {
		return tbl
	}
	if v.resolving {
		ci.diags.Add("warning", "cyclic view dependency: "+tbl.GetId())
		return nil
	}
	v.resolving = true
	defer func() { v.resolving = false; v.resolved = true }()
	query := v.query
	// Introspection stores PostgreSQL's deparsed SELECT in RawSql.
	if query.GetBody() == nil && query.GetRawSql() != "" {
		qs, err := ParseQueries("-- name: view_definition :many\n"+query.GetRawSql(), tbl.GetId())
		if err != nil || len(qs) != 1 {
			ci.diags.Add("warning", "cannot parse view definition: "+tbl.GetId())
			return tbl
		}
		query = qs[0].GetAst().GetSelect()
	}
	if query.GetBody() == nil {
		ci.diags.Add("warning", "view definition unavailable: "+tbl.GetId())
		return tbl
	}
	stmt := &irv1.Statement{Statement: &irv1.Statement_Select{Select: query}}
	inf := inferParamTypes(stmt, map[uint32]*pluginv1.QueryParameter{}, nil, ci, tbl.GetId(), ci.diags)
	for n, col := range inf.output {
		if n < len(v.aliases) {
			col.Name = v.aliases[n]
		}
		col.Id = tbl.GetId() + "." + col.GetName()
		col.Position = uint32(n + 1)
		tbl.Columns = append(tbl.Columns, col)
	}
	return tbl
}
