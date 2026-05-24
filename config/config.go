package config

import (
	"fmt"
	"os"

	"github.com/ghodss/yaml"
	"github.com/seeruk/morph"
)

// LoadFromFile attempts to load Morph config from a file.
// Currently, a configuration file is expected to match the exact structure morph.Spec.
func LoadFromFile(filename string) (morph.Spec, error) {
	var spec morph.Spec

	data, err := os.ReadFile(filename)
	if err != nil {
		return spec, fmt.Errorf("failed to open file %s: %w", filename, err)
	}

	if err := yaml.Unmarshal(data, &spec); err != nil {
		return spec, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	return spec, nil
}
