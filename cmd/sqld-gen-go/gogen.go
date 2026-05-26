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
					// @orderby col1,col2 — LIST of column idents → runtime ORDER BY restricted
					// to those columns, as a typed enum + direction.
					//
					// Optionality of a WHERE condition is NOT an annotation: it comes from
					// QueryParameter.Optional, set by the host for params written as `@name?`.
					Name: "orderby",
					Value: &pluginv1.AnnotationValueSpec{
						Form: pluginv1.AnnotationForm_ANNOTATION_FORM_LIST,
						Element: &pluginv1.FieldSpec{
							Type: irv1.AnnotationArgType_ANNOTATION_ARG_TYPE_IDENT,
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

	// Go-type overrides live in the plugin's options (Go type paths are
	// plugin-specific), parsed once and threaded through generation.
	ov := parseOverrides(req.GetOptions())

	// Build a UDT registry so goType can resolve enums / domains / composites.
	reg := buildUDTRegistry(req.GetCatalog())

	// Detect which extension types (hstore/ltree/lquery) are actually used, by
	// scanning both the catalog columns and the query columns/params. Only used
	// extension types are registered (LoadType + RegisterType) in RegisterTypes.
	usedExtTypes := collectUsedExtensionTypes(req.GetCatalog(), req.GetQueries())

	var diagnostics []*pluginv1.Diagnostic
	var files []*pluginv1.GeneratedFile

	// models.go
	modelsBytes, diags := generateModels(pkg, req.GetCatalog(), reg, ov, usedExtTypes)
	diagnostics = append(diagnostics, diags...)
	files = append(files, &pluginv1.GeneratedFile{
		Path:     "models.go",
		Contents: modelsBytes,
	})

	// queries.go — only when there are queries
	if qs := req.GetQueries(); len(qs) > 0 {
		qBytes, qDiags := generateQueries(pkg, qs, req.GetAnnotations(), reg, ov)
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
	Package   string            `json:"package"`
	Overrides map[string]string `json:"overrides"`
}

// parseOverrides decodes the "overrides" map from the plugin's options JSON.
// Returns nil when there are no options or no overrides.
func parseOverrides(opts []byte) overrides {
	if len(opts) == 0 {
		return nil
	}
	var o pluginOptions
	if err := json.Unmarshal(opts, &o); err != nil || len(o.Overrides) == 0 {
		return nil
	}
	return overrides(o.Overrides)
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

// compositeReceiver derives a short method-receiver identifier from a Go type
// name (e.g. "AppAddress" → "a"). It uses the first letter, lowercased, and
// falls back to "c" if the type name has no usable leading letter.
func compositeReceiver(typeName string) string {
	for _, r := range typeName {
		if unicode.IsLetter(r) {
			return string(unicode.ToLower(r))
		}
	}
	return "c"
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

func generateModels(pkg string, catalog *irv1.Catalog, reg *udtRegistry, ov overrides, usedExtTypes []string) ([]byte, []*pluginv1.Diagnostic) {
	type fieldDef struct {
		name   string
		goType string
	}
	type structDef struct {
		name   string
		fields []fieldDef
	}

	var allImports []string

	// ---- Collect UDT type definitions (enums + composites) ----
	// We emit them in schema order, preserving declaration order within each schema.
	type enumDef struct {
		typeName string
		labels   []string
		// pgName is the schema-qualified PostgreSQL type name (e.g.
		// "app.user_status"); arrayPgName is its auto-created array type
		// ("app._user_status"). Both are registered on a connection so that
		// columns/params of an ENUM ARRAY ("user_status[]") scan and encode
		// (scalar enums scan as text without registration, but an array of an
		// unknown OID needs the array type registered, whose element must be
		// registered first — exactly what pgx's Conn.LoadType requires).
		pgName      string
		arrayPgName string
	}
	type compositeDef struct {
		typeName string
		// bareName is the unqualified PostgreSQL type name (e.g. "address"),
		// used to build the dependency graph (a field's TypeRef.PgName is the
		// bare name, matching the registry key).
		bareName string
		// pgName is the schema-qualified PostgreSQL type name (e.g. "app.address"),
		// used to load+register the composite on a pgx connection.
		pgName string
		// arrayPgName is the schema-qualified name of the composite's auto-created
		// array type (e.g. "app._address"). PostgreSQL creates one array type per
		// base type, named with a leading underscore in the same schema. pgx's
		// Conn.LoadType resolves it ("_foo" when "foo" is registered), so we
		// register it after the element so []AppAddress columns/params work.
		arrayPgName string
		// depNames are the bare names of other composite types this composite
		// directly depends on (a field whose type — or array element type —
		// resolves to another composite). pgx's Conn.LoadType requires all field
		// types of a composite to be registered first, so the outer composite
		// must be registered AFTER every composite in depNames.
		depNames []string
		fields   []fieldDef
	}

	// rangeDef is a custom CREATE TYPE ... AS RANGE type that needs registering.
	// Like enums/composites, the element type (pgName) is registered before its
	// array type (arrayPgName). pgx's Conn.LoadType resolves a range type once
	// its subtype is registered; builtin subtypes (timestamptz, …) are already
	// registered in pgx's default map, so the range loads directly.
	//
	// PostgreSQL 14+ also auto-creates a MULTIRANGE type for each range
	// (multirangePgName, with its own array multirangeArrayPgName). pgx's
	// Conn.LoadType of a multirange requires the element RANGE to be registered
	// first, so the registration order is: range, _range, multirange,
	// _multirange. multirangePgName is empty when the range has no associated
	// multirange.
	type rangeDef struct {
		pgName                string
		arrayPgName           string
		multirangePgName      string
		multirangeArrayPgName string
	}

	var enumDefs []enumDef
	var compositeDefs []compositeDef
	var rangeDefs []rangeDef

	for _, schema := range catalog.GetSchemas() {
		sName := schema.GetName()

		for _, e := range schema.GetEnums() {
			bareName := e.GetName().GetName()
			typeName := udtGoTypeName(sName, bareName)
			pgName := bareName
			arrayPgName := "_" + bareName
			if sName != "" {
				pgName = sName + "." + bareName
				arrayPgName = sName + "._" + bareName
			}
			enumDefs = append(enumDefs, enumDef{
				typeName:    typeName,
				labels:      e.GetLabels(),
				pgName:      pgName,
				arrayPgName: arrayPgName,
			})
		}

		for _, c := range schema.GetComposites() {
			bareName := c.GetName().GetName()
			typeName := udtGoTypeName(sName, bareName)
			var fields []fieldDef
			var depNames []string
			for _, f := range c.GetFields() {
				// Composite fields have no column id; only a type-name override
				// (or the default mapping) can apply.
				gt, imps := resolveGoType(reg, ov, "", f.GetType(), false)
				allImports = append(allImports, imps...)
				fields = append(fields, fieldDef{name: pascal(f.GetName()), goType: gt})
				if dep := compositeDepName(reg, f.GetType()); dep != "" {
					depNames = append(depNames, dep)
				}
			}
			pgName := bareName
			arrayPgName := "_" + bareName
			if sName != "" {
				pgName = sName + "." + bareName
				arrayPgName = sName + "._" + bareName
			}
			compositeDefs = append(compositeDefs, compositeDef{
				typeName:    typeName,
				bareName:    bareName,
				pgName:      pgName,
				arrayPgName: arrayPgName,
				depNames:    depNames,
				fields:      fields,
			})
		}

		for _, r := range schema.GetRanges() {
			bareName := r.GetName().GetName()
			pgName := bareName
			arrayPgName := "_" + bareName
			if sName != "" {
				pgName = sName + "." + bareName
				arrayPgName = sName + "._" + bareName
			}
			rd := rangeDef{
				pgName:      pgName,
				arrayPgName: arrayPgName,
			}
			// Associated multirange (PG 14+), in the same schema as the range.
			if mr := r.GetMultirange(); mr != "" {
				rd.multirangePgName = mr
				rd.multirangeArrayPgName = "_" + mr
				if sName != "" {
					rd.multirangePgName = sName + "." + mr
					rd.multirangeArrayPgName = sName + "._" + mr
				}
			}
			rangeDefs = append(rangeDefs, rd)
		}
	}

	// RegisterTypes is emitted whenever there are enums, composites, custom
	// ranges, OR used extension types: it loads and registers each type (and its
	// array, where applicable) so that enum-array, composite, custom-range, and
	// extension (hstore/ltree) columns/params scan and encode. It needs context,
	// fmt, and the pgx package. Composites additionally need the pgtype package
	// for the CompositeIndexScanner / CompositeIndexGetter interface assertions.
	needsRegister := len(enumDefs) > 0 || len(compositeDefs) > 0 || len(rangeDefs) > 0 || len(usedExtTypes) > 0
	if needsRegister {
		allImports = append(allImports,
			"context",
			"fmt",
			"github.com/jackc/pgx/v5",
		)
	}
	if len(compositeDefs) > 0 {
		allImports = append(allImports, "github.com/jackc/pgx/v5/pgtype")
	}

	// ---- Collect table struct definitions ----
	var structs []structDef

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
				gt, imps := resolveGoType(reg, ov, col.GetId(), col.GetType(), col.GetNullable())
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

	// Emit enum types + const blocks.
	for _, e := range enumDefs {
		sb.WriteString(fmt.Sprintf("type %s string\n\n", e.typeName))
		if len(e.labels) > 0 {
			sb.WriteString("const (\n")
			for _, lbl := range e.labels {
				constName := e.typeName + pascal(lbl)
				sb.WriteString(fmt.Sprintf("\t%s %s = %q\n", constName, e.typeName, lbl))
			}
			sb.WriteString(")\n\n")
		}
	}

	// Emit composite struct types together with the pgx codec methods.
	//
	// pgx decodes a PostgreSQL composite field-by-field via two interfaces
	// (pgtype.CompositeIndexScanner for decoding, pgtype.CompositeIndexGetter
	// for encoding). We implement both on every generated composite and assert
	// satisfaction at compile time. The receiver is a pointer for the scanner
	// (it mutates fields) and a value for the getter (read-only).
	for _, c := range compositeDefs {
		recv := compositeReceiver(c.typeName)

		sb.WriteString(fmt.Sprintf("type %s struct {\n", c.typeName))
		for _, f := range c.fields {
			sb.WriteString(fmt.Sprintf("\t%s %s\n", f.name, f.goType))
		}
		sb.WriteString("}\n\n")

		// ScanIndex returns a pointer usable as a scan target for field i.
		sb.WriteString(fmt.Sprintf("func (%s *%s) ScanIndex(i int) any {\n", recv, c.typeName))
		sb.WriteString("\tswitch i {\n")
		for i, f := range c.fields {
			sb.WriteString(fmt.Sprintf("\tcase %d:\n\t\treturn &%s.%s\n", i, recv, f.name))
		}
		sb.WriteString("\t}\n\treturn nil\n}\n\n")

		// ScanNull sets the value to SQL NULL by zeroing the struct.
		sb.WriteString(fmt.Sprintf("func (%s *%s) ScanNull() error {\n", recv, c.typeName))
		sb.WriteString(fmt.Sprintf("\t*%s = %s{}\n\treturn nil\n}\n\n", recv, c.typeName))

		// Index returns the value of field i for encoding.
		sb.WriteString(fmt.Sprintf("func (%s %s) Index(i int) any {\n", recv, c.typeName))
		sb.WriteString("\tswitch i {\n")
		for i, f := range c.fields {
			sb.WriteString(fmt.Sprintf("\tcase %d:\n\t\treturn %s.%s\n", i, recv, f.name))
		}
		sb.WriteString("\t}\n\treturn nil\n}\n\n")

		// IsNull always reports false: a value-type composite is never NULL.
		sb.WriteString(fmt.Sprintf("func (%s %s) IsNull() bool { return false }\n\n", recv, c.typeName))

		// Compile-time assertions: these fail the build if the generated methods
		// do not satisfy pgx's real composite interfaces.
		sb.WriteString(fmt.Sprintf("var _ pgtype.CompositeIndexScanner = (*%s)(nil)\n", c.typeName))
		sb.WriteString(fmt.Sprintf("var _ pgtype.CompositeIndexGetter = %s{}\n\n", c.typeName))
	}

	// Emit RegisterTypes: loads and registers each extension, enum, composite,
	// and custom range type (and its array, where applicable) on a connection so
	// those columns/params scan into their Go types.
	if needsRegister {
		// Build the dependency-safe ordered list of PostgreSQL type names.
		//
		// 0. Extension types (hstore, ltree, …) FIRST. They have no element
		//    dependencies and pgx resolves them directly by name once loaded; we
		//    register only the bare type (no array companion is generated).
		// 1. Enums (no deps), each followed by its array type.
		// 2. Composites in topological order (a composite whose field is another
		//    composite comes AFTER that field-composite), each followed by its
		//    array type.
		// 3. Custom ranges last, each followed by its array type. A range's
		//    subtype must be registered first; builtin subtypes (timestamptz, …)
		//    are already in pgx's default map, so ranges come after composites
		//    with no further ordering needed.
		//
		// pgx's Conn.LoadType requires an array type's element to be registered
		// first, a composite's field types to all be registered first, and a
		// range type's subtype to be registered first.
		type regType struct {
			pgName, arrayPgName string
			// noArray is true for types that have no companion array to register
			// (the extension types loaded by bare name).
			noArray bool
		}
		var ordered []regType
		for _, name := range usedExtTypes {
			ordered = append(ordered, regType{pgName: name, noArray: true})
		}
		for _, e := range enumDefs {
			ordered = append(ordered, regType{pgName: e.pgName, arrayPgName: e.arrayPgName})
		}

		// Topologically sort composites: dependencies (field-composites) first.
		// Build adjacency from each composite's depNames (restricted to names
		// that are actually composites in this catalog).
		compByName := make(map[string]int, len(compositeDefs))
		for i, c := range compositeDefs {
			compByName[c.bareName] = i
		}
		const (
			white = 0 // unvisited
			gray  = 1 // on the current DFS stack (cycle marker)
			black = 2 // fully processed
		)
		state := make([]int, len(compositeDefs))
		var topo []int
		cycle := false
		var visit func(i int)
		visit = func(i int) {
			if state[i] == black {
				return
			}
			if state[i] == gray {
				cycle = true
				return
			}
			state[i] = gray
			for _, dep := range compositeDefs[i].depNames {
				if j, ok := compByName[dep]; ok && j != i {
					visit(j)
				}
			}
			state[i] = black
			topo = append(topo, i)
		}
		// Visit in declaration order for deterministic output.
		for i := range compositeDefs {
			visit(i)
		}

		if cycle {
			// PostgreSQL forbids composite cycles, so this should be unreachable.
			// Fall back to declaration order and leave a note in the source.
			sb.WriteString("// NOTE: a cycle was detected in composite type dependencies;\n")
			sb.WriteString("// falling back to declaration order (PostgreSQL forbids such cycles).\n")
			topo = topo[:0]
			for i := range compositeDefs {
				topo = append(topo, i)
			}
		}
		for _, i := range topo {
			c := compositeDefs[i]
			ordered = append(ordered, regType{pgName: c.pgName, arrayPgName: c.arrayPgName})
		}

		// Custom ranges last (subtypes already registered — builtin or via the
		// pgx default map). For each range we register, in order: the range and
		// its array, then (PG 14+) the associated multirange and its array. The
		// element RANGE must be registered before its MULTIRANGE, because pgx's
		// Conn.LoadType of a multirange resolves it via the already-registered
		// range element.
		for _, r := range rangeDefs {
			ordered = append(ordered, regType{pgName: r.pgName, arrayPgName: r.arrayPgName})
			if r.multirangePgName != "" {
				ordered = append(ordered, regType{pgName: r.multirangePgName, arrayPgName: r.multirangeArrayPgName})
			}
		}

		sb.WriteString("// RegisterTypes loads and registers the database's extension (hstore/ltree),\n")
		sb.WriteString("// enum, composite, and custom range types (and their array types) on a\n")
		sb.WriteString("// connection so those columns/params scan and encode into their Go types.\n")
		sb.WriteString("// Wire it into pgxpool.Config.AfterConnect (it runs per connection).\n")
		sb.WriteString("//\n")
		sb.WriteString("// The order is dependency-safe: extension types come first (no deps), each\n")
		sb.WriteString("// enum/composite/range is registered before its array type, a composite is\n")
		sb.WriteString("// registered after every composite it has a field of (topological order), and\n")
		sb.WriteString("// custom ranges come last (their subtypes are already registered), because\n")
		sb.WriteString("// pgx's Conn.LoadType resolves a derived type only once its element/field/\n")
		sb.WriteString("// subtype types are registered.\n")
		sb.WriteString("func RegisterTypes(ctx context.Context, conn *pgx.Conn) error {\n")
		sb.WriteString("\tfor _, name := range []string{\n")
		for _, rt := range ordered {
			// Element first, then its array type — the order LoadType requires.
			// Extension types loaded by bare name have no array companion.
			sb.WriteString(fmt.Sprintf("\t\t%q,\n", rt.pgName))
			if !rt.noArray {
				sb.WriteString(fmt.Sprintf("\t\t%q,\n", rt.arrayPgName))
			}
		}
		sb.WriteString("\t} {\n")
		sb.WriteString("\t\tt, err := conn.LoadType(ctx, name)\n")
		sb.WriteString("\t\tif err != nil {\n")
		sb.WriteString("\t\t\treturn fmt.Errorf(\"load type %s: %w\", name, err)\n")
		sb.WriteString("\t\t}\n")
		sb.WriteString("\t\tconn.TypeMap().RegisterType(t)\n")
		sb.WriteString("\t}\n")
		sb.WriteString("\treturn nil\n}\n\n")
	}

	// Emit table structs.
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

func generateQueries(pkg string, queries []*pluginv1.Query, annotations []*irv1.AnnotationValue, reg *udtRegistry, ov overrides) ([]byte, []*pluginv1.Diagnostic) {
	// Index annotations by query name.
	annotByQuery := make(map[string][]*irv1.AnnotationValue)
	for _, a := range annotations {
		if t := a.GetTarget(); t != nil && t.GetQueryName() != "" {
			qn := t.GetQueryName()
			annotByQuery[qn] = append(annotByQuery[qn], a)
		}
	}

	// Determine if any query is dynamic. A query is dynamic when it has an
	// @orderby annotation, OR any parameter is Optional (declared `@name?`),
	// OR any WHERE condition uses ANY($N) (slice).
	hasDynamic := false
	for _, q := range queries {
		if isDynamicQuery(q, annotByQuery[q.GetName()]) {
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
		isDynamic := isDynamicQuery(q, qAnns)

		qCols := q.GetColumns()
		var cols []qField
		for _, c := range qCols {
			gt, imps := resolveGoType(reg, ov, c.GetSourceColumn().GetId(), c.GetType(), c.GetNullable())
			allImports = append(allImports, imps...)
			cols = append(cols, qField{name: pascal(c.GetName()), goType: gt})
		}

		// Collect parameter-type imports for every query (dynamic ones build
		// their Params struct in writeDynamicQueryCode, which discards imports —
		// so an override import on a dynamic param must be gathered here).
		for _, p := range q.GetParameters() {
			_, imps := resolveGoParamType(reg, ov, p.GetColumn().GetId(), p.GetType(), p.GetNullable())
			allImports = append(allImports, imps...)
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
			gt, imps := resolveGoParamType(reg, ov, p.GetColumn().GetId(), p.GetType(), p.GetNullable())
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

	// Emit shared OrderDir type once if any query has @orderby.
	hasAnyOrderBy := false
	for _, de := range dynamicEntries {
		for _, a := range de.anns {
			if a.GetName() == "orderby" {
				hasAnyOrderBy = true
				break
			}
		}
		if hasAnyOrderBy {
			break
		}
	}
	if hasAnyOrderBy {
		sb.WriteString("type OrderDir string\n\n")
		sb.WriteString("const (\n")
		sb.WriteString("\tOrderAsc  OrderDir = \"ASC\"\n")
		sb.WriteString("\tOrderDesc OrderDir = \"DESC\"\n")
		sb.WriteString(")\n\n")
	}

	for _, se := range staticEntries {
		writeQueryCode(&sb, se.qi)
	}

	for _, de := range dynamicEntries {
		writeDynamicQueryCode(&sb, de.q, de.anns, de.cols, reg, ov)
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

// ---- dynamic query generation (WHERE-aware, named-param model) ----

// whereRe matches the top-level WHERE keyword (word-boundary, case-insensitive).
var whereRe = regexp.MustCompile(`(?i)\bWHERE\b`)

// paramNumRe matches $N placeholders.
var paramNumRe = regexp.MustCompile(`\$(\d+)`)

// anyParamRe detects the ANY($N) pattern (slice parameter).
var anyParamRe = regexp.MustCompile(`(?i)\bANY\(\$\d+\)`)

// trailingCommentRe strips a trailing -- ... or /* ... */ comment from a line.
var trailingCommentRe = regexp.MustCompile(`(?:--[^\n]*|/\*.*?\*/)$`)

// conditionInfo describes one parsed WHERE condition.
type conditionInfo struct {
	// condSQL is the cleaned condition SQL (without leading AND/OR and trailing comment).
	condSQL string
	// isOptional is true if the condition's $N parameter has Optional==true
	// (i.e. it was declared `@name?`). Optional conditions become pointer
	// fields, included only when the caller supplies a non-nil value.
	isOptional bool
	// isSlice is true if the condition contains ANY($N). Slice conditions are
	// inherently optional (included only when the slice is non-empty); no `?`
	// is required.
	isSlice bool
	// paramNum is the $N (first) found in the condition (0 if none).
	paramNum uint32
	// fieldName is the PascalCase Go field name derived from the parameter name.
	fieldName string
	// goType is the Go element type (for slices) or base type (for optionals/required).
	goType string
}

// isDynamicQuery reports whether a query must use the WHERE-aware dynamic
// builder. That is the case when it has an @orderby annotation, OR any
// parameter is Optional (declared `@name?`), OR any WHERE condition uses
// ANY($N) (a slice parameter, which is inherently conditional).
func isDynamicQuery(q *pluginv1.Query, anns []*irv1.AnnotationValue) bool {
	for _, a := range anns {
		if a.GetName() == "orderby" {
			return true
		}
	}
	for _, p := range q.GetParameters() {
		if p.GetOptional() {
			return true
		}
	}
	return anyParamRe.MatchString(q.GetSql())
}

// writeDynamicQueryCode generates a WHERE-aware dynamic query method.
func writeDynamicQueryCode(sb *strings.Builder, q *pluginv1.Query, anns []*irv1.AnnotationValue, cols []qField, reg *udtRegistry, ov overrides) {
	sql := q.GetSql()
	methodName := pascal(q.GetName())

	// Build a map from param number to QueryParameter.
	paramByNum := make(map[uint32]*pluginv1.QueryParameter)
	for _, p := range q.GetParameters() {
		paramByNum[p.GetNumber()] = p
	}

	// Collect @orderby annotations. Optionality is no longer an annotation; it
	// comes from QueryParameter.Optional (set by the host for `@name?` params).
	var orderbyAnns []*irv1.AnnotationValue
	for _, a := range anns {
		if a.GetName() == "orderby" {
			orderbyAnns = append(orderbyAnns, a)
		}
	}

	// ----- Step 1: Locate WHERE keyword -----
	loc := whereRe.FindStringIndex(sql)
	var base string
	var whereBody string
	if loc != nil {
		base = strings.TrimRight(sql[:loc[0]], " \t\n\r")
		rest := sql[loc[1]:]

		// Strip trailing semicolon.
		rest = strings.TrimRight(rest, " \t\n\r")
		rest = strings.TrimSuffix(rest, ";")

		// Remove any @orderby comment lines from whereBody.
		// We identify @orderby directive lines by checking offsets.
		// Build set of byte ranges to remove (orderby annotation source spans within whereBody).
		// The offsets are relative to the whole sql string, so we need to strip them from rest
		// by converting absolute offsets to positions within rest.
		whereStart := uint64(loc[1])
		restLines := strings.Split(rest, "\n")
		var filteredLines []string
		lineStart := whereStart
		for _, line := range restLines {
			lineEnd := lineStart + uint64(len(line)) + 1 // +1 for \n
			isOrderByLine := false
			for _, oa := range orderbyAnns {
				src := oa.GetTarget().GetSource()
				// If the @orderby annotation falls within this line, skip the line.
				if src.GetStartOffset() >= lineStart && src.GetStartOffset() < lineEnd {
					isOrderByLine = true
					break
				}
			}
			if !isOrderByLine {
				filteredLines = append(filteredLines, line)
			}
			lineStart = lineEnd
		}
		whereBody = strings.Join(filteredLines, "\n")
	} else {
		// No WHERE clause — treat whole SQL as base with no conditions.
		base = strings.TrimRight(sql, " \t\n\r")
		base = strings.TrimSuffix(base, ";")
		whereBody = ""
	}

	// ----- Step 2: Split whereBody into per-line conditions -----
	//
	// Safety note: the base SQL was already validated by the host (Collect);
	// an invalid base fails generation upstream. By construction, this builder
	// only ever drops whole AND-condition lines, which keeps the remaining SQL
	// valid — so no runtime re-parsing is needed here. Each condition's
	// optionality is read from its $N parameter (Optional/ANY); a condition
	// with no resolvable $N is emitted as REQUIRED static text and never panics.
	//
	// Parse conditions line-by-line.
	var conditions []conditionInfo
	if whereBody != "" && loc != nil {
		// Recompute line offsets against the original sql so that we can skip
		// @orderby comment lines accurately.
		whereEnd := loc[1] // absolute offset of char after WHERE
		currentOff := uint64(whereEnd)
		rawLines := strings.Split(sql[whereEnd:], "\n")
		for _, rawLine := range rawLines {
			lineLen := uint64(len(rawLine))
			lineStartOff := currentOff
			lineEndOff := currentOff + lineLen

			// Trim the line.
			trimmed := strings.TrimSpace(rawLine)

			// Skip blank lines and @orderby lines.
			if trimmed == "" {
				currentOff = lineEndOff + 1 // +1 for \n
				continue
			}
			isOrderByLine := false
			for _, oa := range orderbyAnns {
				src := oa.GetTarget().GetSource()
				if src.GetStartOffset() >= lineStartOff && src.GetStartOffset() <= lineEndOff {
					isOrderByLine = true
					break
				}
			}
			if isOrderByLine {
				currentOff = lineEndOff + 1
				continue
			}

			// Strip leading AND/OR keyword.
			condSQL := trimmed
			upper := strings.ToUpper(condSQL)
			if strings.HasPrefix(upper, "AND ") {
				condSQL = strings.TrimSpace(condSQL[4:])
			} else if strings.HasPrefix(upper, "OR ") {
				condSQL = strings.TrimSpace(condSQL[3:])
			}

			// Strip trailing -- ... or /* ... */ comment.
			condSQL = strings.TrimSpace(trailingCommentRe.ReplaceAllString(condSQL, ""))

			// Strip a trailing statement terminator so a lone ";" line (or a
			// ";" appended to the last condition) is not treated as a condition.
			condSQL = strings.TrimSpace(strings.TrimSuffix(condSQL, ";"))

			if condSQL == "" {
				currentOff = lineEndOff + 1
				continue
			}

			// Detect slice: condition contains ANY($N). Slices are inherently
			// conditional (included only when non-empty) regardless of `?`.
			isSlice := anyParamRe.MatchString(condSQL)

			// Find first $N in the condition.
			var paramNum uint32
			if m := paramNumRe.FindStringSubmatch(condSQL); m != nil {
				fmt.Sscanf(m[1], "%d", &paramNum)
			}

			// Determine field name, Go type, and optionality from the parameter.
			// Optionality comes from QueryParameter.Optional (the `@name?` suffix),
			// NOT from any annotation. A condition with no resolvable $N is treated
			// as required static text.
			var fieldName, goTypeStr string
			var isOptional bool
			if p, ok := paramByNum[paramNum]; ok {
				isOptional = p.GetOptional()
				pName := p.GetName()
				if pName == "" {
					pName = fmt.Sprintf("arg%d", paramNum)
				}
				fieldName = pascal(pName)
				colID := p.GetColumn().GetId()
				if isSlice {
					// Slice: determine element type. A column-id/type-name
					// override applies to the element type.
					tr := p.GetType()
					if tr != nil && tr.GetKind() == irv1.TypeKind_TYPE_KIND_ARRAY {
						gt, _ := resolveGoType(reg, ov, colID, tr.GetElement(), false)
						goTypeStr = gt
					} else {
						gt, _ := resolveGoType(reg, ov, colID, tr, false)
						goTypeStr = gt
					}
				} else {
					gt, _ := resolveGoParamType(reg, ov, colID, p.GetType(), false)
					goTypeStr = gt
				}
			} else {
				fieldName = fmt.Sprintf("Arg%d", paramNum)
				goTypeStr = "any"
			}

			conditions = append(conditions, conditionInfo{
				condSQL:    condSQL,
				isOptional: isOptional,
				isSlice:    isSlice,
				paramNum:   paramNum,
				fieldName:  fieldName,
				goType:     goTypeStr,
			})

			currentOff = lineEndOff + 1
		}
	}

	// ----- Step 3: Parse @orderby columns -----
	var orderByCols []string
	for _, a := range orderbyAnns {
		for _, arg := range a.GetArgs() {
			col := strings.TrimSpace(arg.GetStringValue())
			if col == "" {
				col = strings.TrimSpace(arg.GetRaw())
			}
			if col != "" {
				orderByCols = append(orderByCols, col)
			}
		}
	}
	hasOrderBy := len(orderByCols) > 0

	// ----- Step 4: Emit typed OrderBy enum for this query -----
	orderByTypeName := methodName + "OrderBy"
	if hasOrderBy {
		sb.WriteString(fmt.Sprintf("type %s string\n\n", orderByTypeName))
		sb.WriteString("const (\n")
		for _, col := range orderByCols {
			constName := orderByTypeName + pascal(col)
			sb.WriteString(fmt.Sprintf("\t%s %s = %q\n", constName, orderByTypeName, col))
		}
		sb.WriteString(")\n\n")
	}

	// ----- Step 5: Emit Params struct -----
	paramsStructName := methodName + "Params"
	sb.WriteString(fmt.Sprintf("type %s struct {\n", paramsStructName))
	for _, cond := range conditions {
		switch {
		case cond.isSlice:
			sb.WriteString(fmt.Sprintf("\t%s []%s\n", cond.fieldName, cond.goType))
		case cond.isOptional:
			sb.WriteString(fmt.Sprintf("\t%s *%s\n", cond.fieldName, cond.goType))
		default:
			sb.WriteString(fmt.Sprintf("\t%s %s\n", cond.fieldName, cond.goType))
		}
	}
	if hasOrderBy {
		sb.WriteString(fmt.Sprintf("\tOrderBy %s\n", orderByTypeName))
		sb.WriteString("\tOrderDir OrderDir\n")
	}
	sb.WriteString("}\n\n")

	// ----- Step 6: Emit Row struct -----
	rowTypeName := methodName + "Row"
	isExec := isExecCommand(q.GetCommand())
	if !isExec && len(cols) > 0 {
		sb.WriteString(fmt.Sprintf("type %s struct {\n", rowTypeName))
		for _, c := range cols {
			sb.WriteString(fmt.Sprintf("\t%s %s\n", c.name, c.goType))
		}
		sb.WriteString("}\n\n")
	}

	// ----- Step 7: Emit method -----
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

	// Build SQL at runtime.
	sb.WriteString("\tvar b strings.Builder\n")
	sb.WriteString(fmt.Sprintf("\tb.WriteString(%q)\n", base))
	sb.WriteString("\tvar args []any\n")
	sb.WriteString("\tvar conds []string\n")

	for _, cond := range conditions {
		// Rewrite the condition SQL: replace $N with $%d (using len(args) after append).
		rewritten := paramNumRe.ReplaceAllString(cond.condSQL, "$%d")

		switch {
		case cond.isSlice:
			// Slice: included when len > 0.
			sb.WriteString(fmt.Sprintf("\tif len(arg.%s) > 0 {\n", cond.fieldName))
			sb.WriteString(fmt.Sprintf("\t\targs = append(args, arg.%s)\n", cond.fieldName))
			sb.WriteString(fmt.Sprintf("\t\tconds = append(conds, fmt.Sprintf(%q, len(args)))\n", rewritten))
			sb.WriteString("\t}\n")
		case cond.isOptional:
			// Optional pointer: included when != nil.
			sb.WriteString(fmt.Sprintf("\tif arg.%s != nil {\n", cond.fieldName))
			sb.WriteString(fmt.Sprintf("\t\targs = append(args, *arg.%s)\n", cond.fieldName))
			sb.WriteString(fmt.Sprintf("\t\tconds = append(conds, fmt.Sprintf(%q, len(args)))\n", rewritten))
			sb.WriteString("\t}\n")
		default:
			// Required: always included.
			sb.WriteString(fmt.Sprintf("\targs = append(args, arg.%s)\n", cond.fieldName))
			sb.WriteString(fmt.Sprintf("\tconds = append(conds, fmt.Sprintf(%q, len(args)))\n", rewritten))
		}
	}

	// Emit WHERE clause only if any conditions are present.
	sb.WriteString("\tif len(conds) > 0 {\n")
	sb.WriteString("\t\tb.WriteString(\" WHERE \" + strings.Join(conds, \" AND \"))\n")
	sb.WriteString("\t}\n")

	// Emit ORDER BY block.
	if hasOrderBy {
		sb.WriteString("\tif arg.OrderBy != \"\" {\n")
		sb.WriteString("\t\tdir := \"ASC\"\n")
		sb.WriteString("\t\tif arg.OrderDir == OrderDesc {\n")
		sb.WriteString("\t\t\tdir = \"DESC\"\n")
		sb.WriteString("\t\t}\n")
		sb.WriteString("\t\tfmt.Fprintf(&b, \" ORDER BY %s %s\", string(arg.OrderBy), dir)\n")
		sb.WriteString("\t}\n")
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

// ensure sort is used (it's used in uniqueSorted)
var _ = sort.Strings
