package config

import (
	"fmt"
	"os"

	"github.com/yaroher/sqld/internal/utils/protoyaml"
	configv1 "github.com/yaroher/sqld/pkg/proto/sqld/v1/config"
)

// Load reads a YAML config file into a Config message.
func Load(path string) (*configv1.Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	cfg := &configv1.Config{}
	if err := protoyaml.Unmarshal(b, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}
