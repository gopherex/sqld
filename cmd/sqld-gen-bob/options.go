package main

import "encoding/json"

// bobOptions is the plugin's options block (parsed from the GenerateRequest's
// JSON options). It selects which bob outputs to generate and how to reference
// the shared leaf-type package emitted by sqld-gen-go.
type bobOptions struct {
	// Package is the Go package name for the generated models. Default "models".
	Package string `json:"package"`
	// TypesPackage is the import path of sqld-gen-go's output package, where the
	// canonical enum/composite Go types live. Required for schemas with UDTs.
	TypesPackage string `json:"typesPackage"`
	// NullMode is "pointer" (default) or "opt"; it MUST match sqld-gen-go so the
	// two generators' structs interoperate.
	NullMode string `json:"nullMode"`
	// Overrides is the same Go-type override table sqld-gen-go accepts (keyed by
	// column id or pg type name). It MUST match sqld-gen-go's overrides so both
	// generators emit identical types for overridden columns.
	Overrides map[string]string `json:"overrides"`

	Models    *bool `json:"models"`
	Factories *bool `json:"factories"`
	// WhereLoadersJoins toggles bob's where/loaders/joins/counts helpers.
	// (Relationships are always generated from FK constraints — bob has no
	// separate toggle — so there is no relationships option.)
	WhereLoadersJoins *bool `json:"whereLoadersJoins"`
}

// parseBobOptions decodes the options JSON and applies defaults.
func parseBobOptions(raw []byte) (bobOptions, error) {
	o := bobOptions{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &o); err != nil {
			return o, err
		}
	}
	if o.Package == "" {
		o.Package = "models"
	}
	return o, nil
}

// on reports whether an optional bool flag is enabled (nil defaults to true).
func (bobOptions) on(p *bool) bool { return p == nil || *p }
