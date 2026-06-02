package spec

import (
	"fmt"
	"strings"

	"github.com/seeruk/morph/internal/mapsx"
)

type Output struct {
	Strategy *OutputStrategy `json:"strategy"`
	Path     string          `json:"path"`
	Package  string          `json:"package"`
	Filename string          `json:"filename"`
}

type OutputStrategy uint

const (
	OutputStrategySinglePackage OutputStrategy = iota
	OutputStrategySourcePackage
	OutputStrategyTargetPackage
	outputStrategyMax
)

var outputStrategyNames = map[OutputStrategy]string{
	OutputStrategySinglePackage: "single_package",
	OutputStrategySourcePackage: "source_package",
	OutputStrategyTargetPackage: "target_package",
}

var outputStrategiesByName = mapsx.Invert(outputStrategyNames)

func (s OutputStrategy) MarshalText() ([]byte, error) {
	return []byte(s.String()), nil
}

func (s *OutputStrategy) UnmarshalText(text []byte) error {
	value, ok := outputStrategiesByName[strings.ToLower(string(text))]
	if !ok {
		return fmt.Errorf("unknown output strategy: %q", string(text))
	}

	*s = value
	return nil
}

func (s OutputStrategy) String() string {
	return outputStrategyNames[s]
}
