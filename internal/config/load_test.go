package config

import "testing"

func TestLoad(t *testing.T) {
	cfg, err := Load("testdata/basic.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if cfg.GetEngine().String() != "ENGINE_POSTGRESQL" {
		t.Fatalf("engine: %v", cfg.GetEngine())
	}
	if len(cfg.GetPlugins()) != 1 || cfg.GetPlugins()[0].GetName() != "go" {
		t.Fatalf("plugins: %+v", cfg.GetPlugins())
	}
}
