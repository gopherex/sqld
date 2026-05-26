package main

import (
	"github.com/yaroher/sqld/pkg/gotypes"
	irv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/ir"
	pluginv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/plugin"
)

// The pg→Go type mapping lives in the shared pkg/gotypes package so that both
// sqld-gen-go and sqld-gen-bob emit identical Go types. The shims below keep the
// existing call sites in gogen.go (and the package tests) unchanged.
//
// resolveGoType (model + row column fields) honours the configurable goNullMode
// (Pointer default | Opt), set in Generate from the "nullMode" option, so opt
// mode emits null.Val[T] for nullable fields to match sqld-gen-bob.
// resolveGoParamType keeps the historical Pointer mode unconditionally — query
// params are sqld-internal and not consumed by bob.

type udtRegistry = gotypes.Registry

type overrides = gotypes.Overrides

func buildUDTRegistry(c *irv1.Catalog) *udtRegistry { return gotypes.BuildRegistry(c) }

func udtGoTypeName(schema, name string) string { return gotypes.UDTName(schema, name) }

func collectUsedExtensionTypes(c *irv1.Catalog, q []*pluginv1.Query) []string {
	return gotypes.CollectUsedExtensionTypes(c, q)
}

func resolveGoType(reg *udtRegistry, ov overrides, columnID string, t *irv1.TypeRef, nullable bool) (string, []string) {
	return gotypes.NewMapper2(reg, ov, goNullMode).GoType(columnID, t, nullable)
}

func resolveGoParamType(reg *udtRegistry, ov overrides, columnID string, t *irv1.TypeRef, nullable bool) (string, []string) {
	return gotypes.NewMapper2(reg, ov, gotypes.Pointer).ParamType(columnID, t, nullable)
}

func isCompositeType(reg *udtRegistry, t *irv1.TypeRef) bool {
	return gotypes.IsCompositeType(reg, t)
}

func compositeDepName(reg *udtRegistry, t *irv1.TypeRef) string {
	return gotypes.CompositeDepName(reg, t)
}

// ---- name-helper shims (gogen.go calls these directly) ----

func pascal(s string) string { return gotypes.Pascal(s) }

func lowerCamel(s string) string { return gotypes.LowerCamel(s) }

// ---- shims used only by the package tests, which lock the mapping behaviour
// directly against the (now-moved) functions ----

func goType(reg *udtRegistry, t *irv1.TypeRef, nullable bool) (string, []string) {
	return gotypes.NewMapper2(reg, nil, gotypes.Pointer).GoType("", t, nullable)
}

func goParamType(reg *udtRegistry, t *irv1.TypeRef, nullable bool) (string, []string) {
	return gotypes.NewMapper2(reg, nil, gotypes.Pointer).ParamType("", t, nullable)
}

func parseOverrideValue(v string) (string, []string) {
	return gotypes.ParseOverrideValue(v)
}
