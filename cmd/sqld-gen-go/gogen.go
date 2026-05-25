// Package gogen is the pure-function core of the sqld Go code generator.
// It has no I/O; the plugin binary wrapper calls Info and Generate.
package main

import (
	"encoding/json"
	"fmt"
	"go/format"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"unicode"

	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// Info returns static metadata about this generator.
func Info() *pluginv1.GetInfoResponse {
	return &pluginv1.GetInfoResponse{
		Name:             "go",
		Version:          "0.1.0",
		SupportedEngines: []irv1.Engine{irv1.Engine_ENGINE_POSTGRESQL},
	}
}

// Generate produces Go source files from the IR request.
func Generate(req *pluginv1.GenerateRequest) (*pluginv1.GenerateResponse, error) {
	pkg := resolvePackage(req.GetOptions(), req.GetOutDir())

	var diagnostics []*pluginv1.Diagnostic
	var files []*pluginv1.GeneratedFile

	// models.go
	modelsBytes, diags := generateModels(pkg, req.GetCatalog())
	diagnostics = append(diagnostics, diags...)
	files = append(files, &pluginv1.GeneratedFile{
		Path:     "models.go",
		Contents: modelsBytes,
	})

	// queries.go — only when there are queries
	if qs := req.GetQueries(); len(qs) > 0 {
		qBytes, qDiags := generateQueries(pkg, qs)
		diagnostics = append(diagnostics, qDiags...)
		files = append(files, &pluginv1.GeneratedFile{
			Path:     "queries.go",
			Contents: qBytes,
		})
	}

	return &pluginv1.GenerateResponse{
		Files:       files,
		Diagnostics: diagnostics,
	}, nil
}

// ---- package resolution ----

type pluginOptions struct {
	Package string `json:"package"`
}

func resolvePackage(opts []byte, outDir string) string {
	if len(opts) > 0 {
		var o pluginOptions
		if err := json.Unmarshal(opts, &o); err == nil && o.Package != "" {
			return sanitizeIdent(o.Package)
		}
	}
	base := filepath.Base(outDir)
	if base != "" && base != "." && base != "/" {
		if id := sanitizeIdent(base); id != "" {
			return id
		}
	}
	return "db"
}

var nonIdentRe = regexp.MustCompile(`[^a-zA-Z0-9_]`)

// sanitizeIdent turns an arbitrary string into a valid Go identifier.
func sanitizeIdent(s string) string {
	s = nonIdentRe.ReplaceAllString(s, "_")
	if s == "" {
		return "db"
	}
	r := []rune(s)
	if unicode.IsDigit(r[0]) {
		s = "_" + s
	}
	return s
}

// ---- name helpers ----

// initialisms that should be fully uppercased in Go identifiers.
var initialisms = map[string]string{
	"id":   "ID",
	"url":  "URL",
	"api":  "API",
	"sql":  "SQL",
	"http": "HTTP",
	"json": "JSON",
	"uuid": "UUID",
	"ip":   "IP",
	"xml":  "XML",
}

// pascal converts a snake_case name to PascalCase applying Go initialisms.
func pascal(s string) string {
	parts := splitWords(s)
	var b strings.Builder
	for _, p := range parts {
		lower := strings.ToLower(p)
		if up, ok := initialisms[lower]; ok {
			b.WriteString(up)
		} else {
			b.WriteString(capitalize(p))
		}
	}
	return b.String()
}

// lowerCamel converts to lowerCamelCase.
func lowerCamel(s string) string {
	parts := splitWords(s)
	var b strings.Builder
	for i, p := range parts {
		lower := strings.ToLower(p)
		if i == 0 {
			b.WriteString(lower)
		} else {
			if up, ok := initialisms[lower]; ok {
				b.WriteString(up)
			} else {
				b.WriteString(capitalize(p))
			}
		}
	}
	return b.String()
}

// splitWords splits on underscore boundaries and camelCase transitions.
// "get_user" → ["get","user"], "GetUser" → ["Get","User"], "getUserID" → ["get","User","ID"]
func splitWords(s string) []string {
	// first split on underscores
	underParts := strings.Split(s, "_")
	var out []string
	for _, p := range underParts {
		if p == "" {
			continue
		}
		// Then split camelCase
		out = append(out, splitCamel(p)...)
	}
	if len(out) == 0 {
		return []string{s}
	}
	return out
}

// splitCamel splits a camelCase or PascalCase string into words.
// "GetUser" → ["Get","User"], "getUserID" → ["get","User","ID"]
func splitCamel(s string) []string {
	if s == "" {
		return nil
	}
	runes := []rune(s)
	var words []string
	start := 0
	for i := 1; i < len(runes); i++ {
		if unicode.IsUpper(runes[i]) {
			// Check if previous char is lowercase (transition Lo→Up) or
			// next char is lowercase and current run is uppercase (acronym end)
			if unicode.IsLower(runes[i-1]) {
				words = append(words, string(runes[start:i]))
				start = i
			} else if i+1 < len(runes) && unicode.IsLower(runes[i+1]) && i-start > 1 {
				words = append(words, string(runes[start:i]))
				start = i
			}
		}
	}
	words = append(words, string(runes[start:]))
	return words
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	return string(unicode.ToUpper(r[0])) + strings.ToLower(string(r[1:]))
}

// uniqueSorted deduplicates and sorts a string slice.
func uniqueSorted(ss []string) []string {
	seen := make(map[string]bool, len(ss))
	var out []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// ---- models.go generation ----

func generateModels(pkg string, catalog *irv1.Catalog) ([]byte, []*pluginv1.Diagnostic) {
	type fieldDef struct {
		name   string
		goType string
	}
	type structDef struct {
		name   string
		fields []fieldDef
	}

	var structs []structDef
	var allImports []string

	for _, schema := range catalog.GetSchemas() {
		schemaName := schema.GetName()
		for _, table := range schema.GetTables() {
			tableName := table.GetName().GetName()
			var structName string
			if schemaName == "public" || schemaName == "" {
				structName = pascal(tableName)
			} else {
				structName = pascal(schemaName) + pascal(tableName)
			}

			var fields []fieldDef
			for _, col := range table.GetColumns() {
				gt, imps := goType(col.GetType(), col.GetNullable())
				allImports = append(allImports, imps...)
				fields = append(fields, fieldDef{name: pascal(col.GetName()), goType: gt})
			}
			structs = append(structs, structDef{name: structName, fields: fields})
		}
	}

	var sb strings.Builder
	sb.WriteString("// Code generated by sqld-gen-go. DO NOT EDIT.\n\n")
	sb.WriteString(fmt.Sprintf("package %s\n\n", pkg))

	if uniq := uniqueSorted(allImports); len(uniq) > 0 {
		sb.WriteString("import (\n")
		for _, imp := range uniq {
			sb.WriteString(fmt.Sprintf("\t%q\n", imp))
		}
		sb.WriteString(")\n\n")
	}

	for _, s := range structs {
		sb.WriteString(fmt.Sprintf("type %s struct {\n", s.name))
		for _, f := range s.fields {
			sb.WriteString(fmt.Sprintf("\t%s %s\n", f.name, f.goType))
		}
		sb.WriteString("}\n\n")
	}

	return formatSource(sb.String(), "models.go")
}

// ---- queries.go generation ----

type qField struct {
	name   string
	goType string
}

type qParam struct {
	goName string
	goType string
}

type qInfo struct {
	methodName      string
	constName       string
	sql             string
	command         pluginv1.QueryCommand
	params          []qParam
	cols            []qField
	hasParamStruct  bool
	paramStructName string
}

func generateQueries(pkg string, queries []*pluginv1.Query) ([]byte, []*pluginv1.Diagnostic) {
	var allImports []string
	allImports = append(allImports, "context", "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v5/pgconn")

	var qInfos []qInfo
	for _, q := range queries {
		methodName := pascal(q.GetName())
		constName := lowerCamel(q.GetName()) + "SQL"

		qCols := q.GetColumns()
		var params []qParam
		for _, p := range q.GetParameters() {
			pName := p.GetName()
			if pName == "" {
				// Try to infer from the output column at the same 0-based index
				idx := int(p.GetNumber()) - 1
				if idx >= 0 && idx < len(qCols) && qCols[idx].GetName() != "" {
					pName = qCols[idx].GetName()
				} else {
					pName = fmt.Sprintf("arg%d", p.GetNumber())
				}
			}
			gt, imps := goType(p.GetType(), p.GetNullable())
			allImports = append(allImports, imps...)
			params = append(params, qParam{goName: lowerCamel(pName), goType: gt})
		}

		var cols []qField
		for _, c := range q.GetColumns() {
			gt, imps := goType(c.GetType(), c.GetNullable())
			allImports = append(allImports, imps...)
			cols = append(cols, qField{name: pascal(c.GetName()), goType: gt})
		}

		qInfos = append(qInfos, qInfo{
			methodName:      methodName,
			constName:       constName,
			sql:             q.GetSql(),
			command:         q.GetCommand(),
			params:          params,
			cols:            cols,
			hasParamStruct:  len(params) >= 2,
			paramStructName: methodName + "Params",
		})
	}

	var sb strings.Builder
	sb.WriteString("// Code generated by sqld-gen-go. DO NOT EDIT.\n\n")
	sb.WriteString(fmt.Sprintf("package %s\n\n", pkg))

	// Import block with blank line between std and external
	uniqueImports := uniqueSorted(allImports)
	var stdImps, extImps []string
	for _, imp := range uniqueImports {
		if strings.Contains(imp, ".") {
			extImps = append(extImps, imp)
		} else {
			stdImps = append(stdImps, imp)
		}
	}
	sb.WriteString("import (\n")
	for _, imp := range stdImps {
		sb.WriteString(fmt.Sprintf("\t%q\n", imp))
	}
	if len(stdImps) > 0 && len(extImps) > 0 {
		sb.WriteString("\n")
	}
	for _, imp := range extImps {
		sb.WriteString(fmt.Sprintf("\t%q\n", imp))
	}
	sb.WriteString(")\n\n")

	// DBTX interface
	sb.WriteString("type DBTX interface {\n")
	sb.WriteString("\tExec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)\n")
	sb.WriteString("\tQuery(ctx context.Context, sql string, args ...any) (pgx.Rows, error)\n")
	sb.WriteString("\tQueryRow(ctx context.Context, sql string, args ...any) pgx.Row\n")
	sb.WriteString("}\n\n")

	sb.WriteString("type Queries struct {\n\tdb DBTX\n}\n\n")
	sb.WriteString("func New(db DBTX) *Queries { return &Queries{db: db} }\n\n")

	for _, qi := range qInfos {
		writeQueryCode(&sb, qi)
	}

	return formatSource(sb.String(), "queries.go")
}

func writeQueryCode(sb *strings.Builder, qi qInfo) {
	// SQL const — backtick-safe: escape any backtick in SQL
	sqlLit := strings.ReplaceAll(qi.sql, "`", "`+\"`\"+`")
	sb.WriteString(fmt.Sprintf("const %s = `%s`\n\n", qi.constName, sqlLit))

	// Params struct (≥ 2 params)
	if qi.hasParamStruct {
		sb.WriteString(fmt.Sprintf("type %s struct {\n", qi.paramStructName))
		for _, p := range qi.params {
			sb.WriteString(fmt.Sprintf("\t%s %s\n", pascal(p.goName), p.goType))
		}
		sb.WriteString("}\n\n")
	}

	// Determine command category
	isExec := isExecCommand(qi.command)
	isExecRows := qi.command == pluginv1.QueryCommand_QUERY_COMMAND_EXEC_ROWS

	rowTypeName := qi.methodName + "Row"

	// Row struct (for :one and :many with columns)
	if !isExec && len(qi.cols) > 0 {
		sb.WriteString(fmt.Sprintf("type %s struct {\n", rowTypeName))
		for _, c := range qi.cols {
			sb.WriteString(fmt.Sprintf("\t%s %s\n", c.name, c.goType))
		}
		sb.WriteString("}\n\n")
	}

	// Method signature
	paramSig := buildParamSig(qi)

	switch {
	case qi.command == pluginv1.QueryCommand_QUERY_COMMAND_ONE:
		sb.WriteString(fmt.Sprintf("func (q *Queries) %s(ctx context.Context%s) (%s, error) {\n",
			qi.methodName, paramSig, rowTypeName))
		sb.WriteString(fmt.Sprintf("\trow := q.db.QueryRow(ctx, %s%s)\n", qi.constName, buildArgList(qi)))
		sb.WriteString(fmt.Sprintf("\tvar i %s\n", rowTypeName))
		sb.WriteString(fmt.Sprintf("\terr := row.Scan(%s)\n", buildScanList(qi.cols, "i")))
		sb.WriteString("\treturn i, err\n}\n\n")

	case qi.command == pluginv1.QueryCommand_QUERY_COMMAND_MANY:
		sb.WriteString(fmt.Sprintf("func (q *Queries) %s(ctx context.Context%s) ([]%s, error) {\n",
			qi.methodName, paramSig, rowTypeName))
		sb.WriteString(fmt.Sprintf("\trows, err := q.db.Query(ctx, %s%s)\n", qi.constName, buildArgList(qi)))
		sb.WriteString("\tif err != nil {\n\t\treturn nil, err\n\t}\n")
		sb.WriteString("\tdefer rows.Close()\n")
		sb.WriteString(fmt.Sprintf("\tvar items []%s\n", rowTypeName))
		sb.WriteString("\tfor rows.Next() {\n")
		sb.WriteString(fmt.Sprintf("\t\tvar i %s\n", rowTypeName))
		sb.WriteString(fmt.Sprintf("\t\tif err := rows.Scan(%s); err != nil {\n", buildScanList(qi.cols, "i")))
		sb.WriteString("\t\t\treturn nil, err\n\t\t}\n")
		sb.WriteString("\t\titems = append(items, i)\n\t}\n")
		sb.WriteString("\tif err := rows.Err(); err != nil {\n\t\treturn nil, err\n\t}\n")
		sb.WriteString("\treturn items, nil\n}\n\n")

	case isExecRows:
		sb.WriteString(fmt.Sprintf("func (q *Queries) %s(ctx context.Context%s) (int64, error) {\n",
			qi.methodName, paramSig))
		sb.WriteString(fmt.Sprintf("\ttag, err := q.db.Exec(ctx, %s%s)\n", qi.constName, buildArgList(qi)))
		sb.WriteString("\treturn tag.RowsAffected(), err\n}\n\n")

	default: // :exec and others
		sb.WriteString(fmt.Sprintf("func (q *Queries) %s(ctx context.Context%s) error {\n",
			qi.methodName, paramSig))
		sb.WriteString(fmt.Sprintf("\t_, err := q.db.Exec(ctx, %s%s)\n", qi.constName, buildArgList(qi)))
		sb.WriteString("\treturn err\n}\n\n")
	}
}

func isExecCommand(cmd pluginv1.QueryCommand) bool {
	switch cmd {
	case pluginv1.QueryCommand_QUERY_COMMAND_ONE, pluginv1.QueryCommand_QUERY_COMMAND_MANY:
		return false
	default:
		return true
	}
}

// buildParamSig builds the parameter list after ctx (with leading ", ").
func buildParamSig(qi qInfo) string {
	if len(qi.params) == 0 {
		return ""
	}
	if qi.hasParamStruct {
		return fmt.Sprintf(", arg %s", qi.paramStructName)
	}
	// single param
	p := qi.params[0]
	return fmt.Sprintf(", %s %s", p.goName, p.goType)
}

// buildArgList builds the extra arguments passed to db.Exec/Query/QueryRow.
func buildArgList(qi qInfo) string {
	if len(qi.params) == 0 {
		return ""
	}
	if qi.hasParamStruct {
		var parts []string
		for _, p := range qi.params {
			parts = append(parts, "arg."+pascal(p.goName))
		}
		return ", " + strings.Join(parts, ", ")
	}
	// single param
	return ", " + qi.params[0].goName
}

// buildScanList builds "&i.Field1, &i.Field2, ..." for row.Scan.
func buildScanList(cols []qField, varName string) string {
	var parts []string
	for _, c := range cols {
		parts = append(parts, fmt.Sprintf("&%s.%s", varName, c.name))
	}
	return strings.Join(parts, ", ")
}

// formatSource runs gofmt on src. On failure, returns src with a warning diagnostic.
func formatSource(src, filename string) ([]byte, []*pluginv1.Diagnostic) {
	b := []byte(src)
	formatted, err := format.Source(b)
	if err != nil {
		return b, []*pluginv1.Diagnostic{{
			Severity: pluginv1.DiagnosticSeverity_DIAGNOSTIC_SEVERITY_WARNING,
			Message:  fmt.Sprintf("%s: gofmt failed: %v", filename, err),
		}}
	}
	return formatted, nil
}

// ensure sort is used (it's used in uniqueSorted)
var _ = sort.Strings
