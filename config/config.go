package config

import (
	"fmt"
	"os"

	"github.com/seeruk/morph"
	"gopkg.in/yaml.v2"
)

// LoadFromFile attempts to load Morph config from a file.
// Currently, a configuration file is expected to match the exact structure morph.Spec.
func LoadFromFile(filename string) (morph.Spec, error) {
	var spec morph.Spec

	file, err := os.Open(filename)
	if err != nil {
		return spec, fmt.Errorf("failed to open file %s: %w", filename, err)
	}

	if err := yaml.NewDecoder(file).Decode(&spec); err != nil {
		return spec, fmt.Errorf("failed to parse file %s: %w", filename, err)
	}

	return spec, nil
}
