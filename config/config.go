package config

import (
	"fmt"
	"os"

	"github.com/ghodss/yaml"
)

// LoadFromFile attempts to load Morph config from a file.
func LoadFromFile(filename string) (Config, error) {
	var cfg Config

	data, err := os.ReadFile(filename)
	if err != nil {
		return cfg, fmt.Errorf("failed to open file %s: %w", filename, err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	return cfg, nil
}
