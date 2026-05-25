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
		AnnotationSchema: &pluginv1.AnnotationSchema{
			Sigil:         "@",
			CommentStyles: []string{"--", "/* */"},
			Annotations: []*pluginv1.AnnotationDef{
				{
					Name: "if",
					Value: &pluginv1.AnnotationValueSpec{
						Form: pluginv1.AnnotationForm_ANNOTATION_FORM_POSITIONAL,
						Fields: []*pluginv1.FieldSpec{
							{Name: "condition", Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_IDENT},
						},
					},
				},
				{
					Name:  "endif",
					Value: &pluginv1.AnnotationValueSpec{Form: pluginv1.AnnotationForm_ANNOTATION_FORM_FLAG},
				},
				{
					Name: "slice",
					Value: &pluginv1.AnnotationValueSpec{
						Form: pluginv1.AnnotationForm_ANNOTATION_FORM_POSITIONAL,
						Fields: []*pluginv1.FieldSpec{
							{Name: "param", Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_IDENT},
						},
					},
				},
				{
					Name: "orderby",
					Value: &pluginv1.AnnotationValueSpec{
						Form: pluginv1.AnnotationForm_ANNOTATION_FORM_KEYED,
						Fields: []*pluginv1.FieldSpec{
							{Name: "allow", Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_STRING},
						},
					},
				},
			},
		},
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
		qBytes, qDiags := generateQueries(pkg, qs, req.GetAnnotations())
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

func generateQueries(pkg string, queries []*pluginv1.Query, annotations []*irv1.AnnotationValue) ([]byte, []*pluginv1.Diagnostic) {
	// Index annotations by query name.
	annotByQuery := make(map[string][]*irv1.AnnotationValue)
	for _, a := range annotations {
		if t := a.GetTarget(); t != nil && t.GetQueryName() != "" {
			qn := t.GetQueryName()
			annotByQuery[qn] = append(annotByQuery[qn], a)
		}
	}

	// Determine if any query is dynamic (has annotations).
	hasDynamic := false
	for _, q := range queries {
		if len(annotByQuery[q.GetName()]) > 0 {
			hasDynamic = true
			break
		}
	}

	var allImports []string
	allImports = append(allImports, "context", "github.com/jackc/pgx/v5", "github.com/jackc/pgx/v5/pgconn")
	if hasDynamic {
		allImports = append(allImports, "fmt", "strings")
	}

	// Separate static and dynamic queries. Collect import contributions from both.
	type staticEntry struct {
		qi qInfo
	}
	type dynamicEntry struct {
		q    *pluginv1.Query
		anns []*irv1.AnnotationValue
		cols []qField
	}

	var staticEntries []staticEntry
	var dynamicEntries []dynamicEntry

	for _, q := range queries {
		qAnns := annotByQuery[q.GetName()]
		isDynamic := len(qAnns) > 0

		qCols := q.GetColumns()
		var cols []qField
		for _, c := range qCols {
			gt, imps := goType(c.GetType(), c.GetNullable())
			allImports = append(allImports, imps...)
			cols = append(cols, qField{name: pascal(c.GetName()), goType: gt})
		}

		if isDynamic {
			dynamicEntries = append(dynamicEntries, dynamicEntry{q: q, anns: qAnns, cols: cols})
			continue
		}

		methodName := pascal(q.GetName())
		constName := lowerCamel(q.GetName()) + "SQL"

		var params []qParam
		for _, p := range q.GetParameters() {
			pName := p.GetName()
			if pName == "" {
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

		staticEntries = append(staticEntries, staticEntry{qi: qInfo{
			methodName:      methodName,
			constName:       constName,
			sql:             q.GetSql(),
			command:         q.GetCommand(),
			params:          params,
			cols:            cols,
			hasParamStruct:  len(params) >= 2,
			paramStructName: methodName + "Params",
		}})
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

	for _, se := range staticEntries {
		writeQueryCode(&sb, se.qi)
	}

	for _, de := range dynamicEntries {
		writeDynamicQueryCode(&sb, de.q, de.anns, de.cols)
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

// ---- dynamic query generation ----

// dynSegKind classifies a segment within a dynamic query's SQL.
type dynSegKind int

const (
	dynSegStatic  dynSegKind = iota // verbatim SQL text (may contain $N placeholders)
	dynSegIf                        // @if region
	dynSegSlice                     // @slice region
	dynSegOrderBy                   // @orderby marker
)

type dynSegment struct {
	kind        dynSegKind
	text        string // static: SQL text; conditional: fragment SQL
	fieldName   string // if/slice: Go field name (PascalCase)
	goType      string // slice: element type; if: pointer-base type
	allowList   string // orderby: comma-sep allowed columns
	paramNumber uint32 // for if/slice: the $N inside the fragment
}

// writeDynamicQueryCode generates a query method that builds SQL at runtime.
func writeDynamicQueryCode(sb *strings.Builder, q *pluginv1.Query, anns []*irv1.AnnotationValue, cols []qField) {
	sql := q.GetSql()
	methodName := pascal(q.GetName())
	lowerName := lowerCamel(q.GetName())

	// Build a map from param number to QueryParameter.
	paramByNum := make(map[uint32]*pluginv1.QueryParameter)
	for _, p := range q.GetParameters() {
		paramByNum[p.GetNumber()] = p
	}

	// Sort annotations by StartOffset.
	sorted := make([]*irv1.AnnotationValue, len(anns))
	copy(sorted, anns)
	sort.Slice(sorted, func(i, j int) bool {
		si := sorted[i].GetTarget().GetSource().GetStartOffset()
		sj := sorted[j].GetTarget().GetSource().GetStartOffset()
		return si < sj
	})

	// Pair opens (if/slice) with the next endif, in order.
	// Build a list of "events" over the SQL: open, close, orderby.
	type annEvent struct {
		name     string
		startOff uint64
		endOff   uint64
		argValue string // condition/param name or allow list
	}
	var events []annEvent
	for _, a := range sorted {
		src := a.GetTarget().GetSource()
		argVal := ""
		if len(a.GetArgs()) > 0 {
			argVal = a.GetArgs()[0].GetStringValue()
		}
		events = append(events, annEvent{
			name:     a.GetName(),
			startOff: src.GetStartOffset(),
			endOff:   src.GetEndOffset(),
			argValue: argVal,
		})
	}

	// Walk events and build segments, tracking excluded byte ranges.
	// excluded ranges = all directive comment byte ranges.
	type byteRange struct{ start, end uint64 }
	var excluded []byteRange
	for _, ev := range events {
		excluded = append(excluded, byteRange{ev.startOff, ev.endOff})
	}

	// Build ordered segments by processing events left-to-right.
	// We maintain a cursor over the SQL and pop off each directive.
	var segments []dynSegment
	cursor := uint64(0)

	// Stack of open regions: each entry = index into events of the opener.
	type openEntry struct {
		evIdx     int
		openEvent annEvent
	}
	var openStack []openEntry

	for i, ev := range events {
		switch ev.name {
		case "if", "slice":
			// Emit static text from cursor to start of this directive.
			if ev.startOff > cursor {
				staticText := strings.TrimRight(sql[cursor:ev.startOff], " \t\n\r")
				if staticText != "" {
					segments = append(segments, dynSegment{kind: dynSegStatic, text: staticText})
				}
			}
			cursor = ev.endOff
			openStack = append(openStack, openEntry{evIdx: i, openEvent: ev})

		case "endif":
			if len(openStack) == 0 {
				// Orphan endif — treat as static exclusion.
				cursor = ev.endOff
				continue
			}
			opener := openStack[len(openStack)-1]
			openStack = openStack[:len(openStack)-1]

			// Fragment text: from end of opener directive to start of this endif.
			fragText := ""
			if ev.startOff > opener.openEvent.endOff {
				fragText = strings.TrimSpace(sql[opener.openEvent.endOff:ev.startOff])
			}

			// Find the $N inside the fragment.
			paramNum := findFirstParamNum(fragText)

			var fieldName, goTypeStr string
			switch opener.openEvent.name {
			case "if":
				fieldName = pascal(opener.openEvent.argValue)
				// Get the param's base type (no pointer — we'll add * when generating).
				if p, ok := paramByNum[paramNum]; ok {
					gt, _ := goType(p.GetType(), false)
					goTypeStr = gt
				} else {
					goTypeStr = "any"
				}
				segments = append(segments, dynSegment{
					kind:        dynSegIf,
					text:        fragText,
					fieldName:   fieldName,
					goType:      goTypeStr,
					paramNumber: paramNum,
				})
			case "slice":
				fieldName = pascal(opener.openEvent.argValue)
				// Determine element type.
				if p, ok := paramByNum[paramNum]; ok {
					tr := p.GetType()
					if tr != nil && tr.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
						gt, _ := goType(tr.GetElement(), false)
						goTypeStr = gt
					} else {
						gt, _ := goType(tr, false)
						goTypeStr = gt
					}
				} else {
					goTypeStr = "any"
				}
				segments = append(segments, dynSegment{
					kind:        dynSegSlice,
					text:        fragText,
					fieldName:   fieldName,
					goType:      goTypeStr,
					paramNumber: paramNum,
				})
			}
			cursor = ev.endOff

		case "orderby":
			// Emit static text before this directive.
			if ev.startOff > cursor {
				staticText := strings.TrimRight(sql[cursor:ev.startOff], " \t\n\r")
				if staticText != "" {
					segments = append(segments, dynSegment{kind: dynSegStatic, text: staticText})
				}
			}
			segments = append(segments, dynSegment{
				kind:      dynSegOrderBy,
				allowList: ev.argValue,
			})
			cursor = ev.endOff
		}
	}

	// Emit any trailing static text.
	if int(cursor) < len(sql) {
		staticText := strings.TrimRight(sql[cursor:], " \t\n\r")
		if staticText != "" {
			segments = append(segments, dynSegment{kind: dynSegStatic, text: staticText})
		}
	}

	// Collect Params struct fields.
	type paramField struct {
		name      string
		goType    string // full type including * or []
		kind      dynSegKind
		allowList string
	}
	var fields []paramField
	hasOrderBy := false
	for _, seg := range segments {
		switch seg.kind {
		case dynSegIf:
			fields = append(fields, paramField{name: seg.fieldName, goType: "*" + seg.goType, kind: dynSegIf})
		case dynSegSlice:
			fields = append(fields, paramField{name: seg.fieldName, goType: "[]" + seg.goType, kind: dynSegSlice})
		case dynSegOrderBy:
			if !hasOrderBy {
				fields = append(fields, paramField{name: "OrderBy", goType: "string", kind: dynSegOrderBy, allowList: seg.allowList})
				hasOrderBy = true
			}
		}
	}

	// Emit orderby allowlist map var (before the method).
	if hasOrderBy {
		mapVarName := lowerName + "OrderBy"
		// Collect allowed columns.
		var allowCols []string
		for _, seg := range segments {
			if seg.kind == dynSegOrderBy {
				for _, col := range strings.Split(seg.allowList, ",") {
					col = strings.TrimSpace(col)
					if col != "" {
						allowCols = append(allowCols, col)
					}
				}
				break
			}
		}
		sb.WriteString(fmt.Sprintf("var %s = map[string]string{", mapVarName))
		for i, col := range allowCols {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%q: %q", col, col))
		}
		sb.WriteString("}\n\n")
	}

	// Params struct.
	paramsStructName := methodName + "Params"
	sb.WriteString(fmt.Sprintf("type %s struct {\n", paramsStructName))
	for _, f := range fields {
		sb.WriteString(fmt.Sprintf("\t%s %s\n", f.name, f.goType))
	}
	sb.WriteString("}\n\n")

	// Row struct.
	rowTypeName := methodName + "Row"
	isExec := isExecCommand(q.GetCommand())
	if !isExec && len(cols) > 0 {
		sb.WriteString(fmt.Sprintf("type %s struct {\n", rowTypeName))
		for _, c := range cols {
			sb.WriteString(fmt.Sprintf("\t%s %s\n", c.name, c.goType))
		}
		sb.WriteString("}\n\n")
	}

	// Method signature.
	var retType string
	switch {
	case q.GetCommand() == pluginv1.QueryCommand_QUERY_COMMAND_ONE:
		retType = fmt.Sprintf("(%s, error)", rowTypeName)
	case q.GetCommand() == pluginv1.QueryCommand_QUERY_COMMAND_MANY:
		retType = fmt.Sprintf("([]%s, error)", rowTypeName)
	case q.GetCommand() == pluginv1.QueryCommand_QUERY_COMMAND_EXEC_ROWS:
		retType = "(int64, error)"
	default:
		retType = "error"
	}

	if retType == "error" {
		sb.WriteString(fmt.Sprintf("func (q *Queries) %s(ctx context.Context, arg %s) error {\n",
			methodName, paramsStructName))
	} else {
		sb.WriteString(fmt.Sprintf("func (q *Queries) %s(ctx context.Context, arg %s) %s {\n",
			methodName, paramsStructName, retType))
	}

	// Body: build SQL string at runtime.
	sb.WriteString("\tvar b strings.Builder\n")
	sb.WriteString("\tvar args []any\n")

	mapVarName := lowerName + "OrderBy"

	for _, seg := range segments {
		switch seg.kind {
		case dynSegStatic:
			// Handle $N placeholders within the static text.
			writeStaticSegment(sb, seg.text)

		case dynSegIf:
			// if arg.X != nil { args = append(args, *arg.X); fmt.Fprintf(&b, " ... $%d ...", len(args)) }
			// The fragment is trimmed of surrounding whitespace, so prefix a single
			// leading space to keep the assembled SQL separated (avoid e.g. "trueAND").
			rewritten := " " + rewriteFragment(seg.text)
			sb.WriteString(fmt.Sprintf("\tif arg.%s != nil {\n", seg.fieldName))
			sb.WriteString(fmt.Sprintf("\t\targs = append(args, *arg.%s)\n", seg.fieldName))
			sb.WriteString(fmt.Sprintf("\t\tfmt.Fprintf(&b, %q, len(args))\n", rewritten))
			sb.WriteString("\t}\n")

		case dynSegSlice:
			// Prefix a single leading space (fragment is trimmed) so consecutive
			// conditional fragments stay separated by exactly one space.
			rewritten := " " + rewriteFragment(seg.text)
			sb.WriteString(fmt.Sprintf("\tif len(arg.%s) > 0 {\n", seg.fieldName))
			sb.WriteString(fmt.Sprintf("\t\targs = append(args, arg.%s)\n", seg.fieldName))
			sb.WriteString(fmt.Sprintf("\t\tfmt.Fprintf(&b, %q, len(args))\n", rewritten))
			sb.WriteString("\t}\n")

		case dynSegOrderBy:
			sb.WriteString("\tif arg.OrderBy != \"\" {\n")
			sb.WriteString(fmt.Sprintf("\t\tcol, ok := %s[arg.OrderBy]\n", mapVarName))
			sb.WriteString("\t\tif !ok {\n")
			sb.WriteString("\t\t\treturn " + dynReturnNil(q.GetCommand()) + "fmt.Errorf(\"invalid order by: %s\", arg.OrderBy)\n")
			sb.WriteString("\t\t}\n")
			sb.WriteString("\t\tfmt.Fprintf(&b, \" ORDER BY %s\", col)\n")
			sb.WriteString("\t}\n")
		}
	}

	// Execute the built query.
	switch {
	case q.GetCommand() == pluginv1.QueryCommand_QUERY_COMMAND_ONE:
		sb.WriteString("\trow := q.db.QueryRow(ctx, b.String(), args...)\n")
		sb.WriteString(fmt.Sprintf("\tvar i %s\n", rowTypeName))
		sb.WriteString(fmt.Sprintf("\terr := row.Scan(%s)\n", buildScanList(cols, "i")))
		sb.WriteString("\treturn i, err\n")

	case q.GetCommand() == pluginv1.QueryCommand_QUERY_COMMAND_MANY:
		sb.WriteString("\trows, err := q.db.Query(ctx, b.String(), args...)\n")
		sb.WriteString("\tif err != nil {\n\t\treturn nil, err\n\t}\n")
		sb.WriteString("\tdefer rows.Close()\n")
		sb.WriteString(fmt.Sprintf("\tvar items []%s\n", rowTypeName))
		sb.WriteString("\tfor rows.Next() {\n")
		sb.WriteString(fmt.Sprintf("\t\tvar i %s\n", rowTypeName))
		sb.WriteString(fmt.Sprintf("\t\tif err := rows.Scan(%s); err != nil {\n", buildScanList(cols, "i")))
		sb.WriteString("\t\t\treturn nil, err\n\t\t}\n")
		sb.WriteString("\t\titems = append(items, i)\n\t}\n")
		sb.WriteString("\treturn items, rows.Err()\n")

	case q.GetCommand() == pluginv1.QueryCommand_QUERY_COMMAND_EXEC_ROWS:
		sb.WriteString("\ttag, err := q.db.Exec(ctx, b.String(), args...)\n")
		sb.WriteString("\treturn tag.RowsAffected(), err\n")

	default: // exec
		sb.WriteString("\t_, err := q.db.Exec(ctx, b.String(), args...)\n")
		sb.WriteString("\treturn err\n")
	}

	sb.WriteString("}\n\n")
}

// dynReturnNil returns the prefix to prepend before an error return for dynamic
// queries that have non-error return types (e.g. "nil, ").
func dynReturnNil(cmd pluginv1.QueryCommand) string {
	switch cmd {
	case pluginv1.QueryCommand_QUERY_COMMAND_ONE, pluginv1.QueryCommand_QUERY_COMMAND_MANY:
		return "nil, "
	case pluginv1.QueryCommand_QUERY_COMMAND_EXEC_ROWS:
		return "0, "
	default:
		return ""
	}
}

// findFirstParamNum scans text for the first $N and returns N.
// Returns 0 if not found.
func findFirstParamNum(text string) uint32 {
	re := regexp.MustCompile(`\$(\d+)`)
	m := re.FindStringSubmatch(text)
	if m == nil {
		return 0
	}
	var n uint32
	fmt.Sscanf(m[1], "%d", &n)
	return n
}

// rewriteFragment replaces the first $N in a fragment with $%d (for fmt.Fprintf).
func rewriteFragment(text string) string {
	re := regexp.MustCompile(`\$\d+`)
	return re.ReplaceAllString(text, "$%d")
}

// writeStaticSegment emits b.WriteString / fmt.Fprintf calls for a static
// SQL segment that may contain $N placeholders. Each $N is replaced with
// a runtime-numbered $%d after appending the corresponding argument.
// Since static segments outside any region use positional params that are
// always present, we just emit the text as a WriteString (no args to append —
// static params outside conditional regions are not supported in dynamic queries;
// all params must be inside regions). If the static text has no $N we simply
// emit a WriteString.
func writeStaticSegment(sb *strings.Builder, text string) {
	// For the current design, static text outside regions should not contain
	// $N (those are always inside regions in a dynamic query). We write it
	// literally via b.WriteString.
	sb.WriteString(fmt.Sprintf("\tb.WriteString(%q)\n", text))
}

// ensure sort is used (it's used in uniqueSorted)
var _ = sort.Strings
