package spec

import (
	"fmt"
	"strings"

	"github.com/seeruk/morph/internal/mapsx"
)

type Output struct {
	Strategy OutputStrategy
	Path     string
	Package  string
	Filename string
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
	value, ok := outputStrategyNames[s]
	if !ok {
		return nil, fmt.Errorf("unknown output strategy: %s", s.String())
	}
	return []byte(value), nil
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
	if value, ok := outputStrategyNames[s]; ok {
		return value
	}
	return fmt.Sprintf("OutputStrategy(%d)", s)
}
