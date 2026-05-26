package main

import "testing"

func TestParseBobOptionsDefaults(t *testing.T) {
	o, err := parseBobOptions([]byte(`{"typesPackage":"example.com/app/db"}`))
	if err != nil {
		t.Fatal(err)
	}
	if o.Package != "models" {
		t.Errorf("package = %q; want models", o.Package)
	}
	if o.TypesPackage != "example.com/app/db" {
		t.Errorf("typesPackage = %q", o.TypesPackage)
	}
	if !o.on(o.Models) || !o.on(o.Factories) || !o.on(o.WhereLoadersJoins) {
		t.Errorf("optional flags should default to on")
	}
}

func TestParseBobOptionsExplicit(t *testing.T) {
	o, err := parseBobOptions([]byte(`{"package":"orm","nullMode":"opt","models":false,"factories":false}`))
	if err != nil {
		t.Fatal(err)
	}
	if o.Package != "orm" {
		t.Errorf("package = %q; want orm", o.Package)
	}
	if o.NullMode != "opt" {
		t.Errorf("nullMode = %q; want opt", o.NullMode)
	}
	if o.on(o.Models) || o.on(o.Factories) {
		t.Errorf("models/factories should be off")
	}
}

func TestParseBobOptionsEmpty(t *testing.T) {
	o, err := parseBobOptions(nil)
	if err != nil {
		t.Fatal(err)
	}
	if o.Package != "models" {
		t.Errorf("package = %q; want models", o.Package)
	}
}
