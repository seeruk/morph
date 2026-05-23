package spec

import (
	"github.com/seeruk/morph/types"
)

type Discovery struct{}

type Preset struct {
	Name          string     `json:"name"`
	Enum          PresetEnum `json:"enum"`
	Mappers       Mappers    `json:"mappers"`
	Bidirectional bool       `json:"bidirectional"`
}

// PresetEnum represents enum configuration available within a preset. This type only exists because
// the available enum configuration in a preset is a subset of the configuration available when
// configuring an individual type pair.
type PresetEnum struct {
	FailureMode *EnumFailureMode `json:"failureMode"`
}

// TypePair represents the mapping configuration for a specific pair of types.
type TypePair struct {
	Name    string         `json:"name"`
	Source  types.TypeDecl `json:"source"`
	Target  types.TypeDecl `json:"target"`
	Preset  string         `json:"preset"`
	Enum    *Enum          `json:"enum"`
	Struct  *Struct        `json:"struct"`
	Mappers *Mappers       `json:"mappers"`
}

// EnumFailureMode enumerates the possible failure modes for an enum mapping function.
type EnumFailureMode string

const (
	EnumFailureModeError EnumFailureMode = "error"
	EnumFailureModeZero  EnumFailureMode = "zero"
)

// Enum represents configuration for how an enum mapping function should be generated,
// allowing customization of generated output and explicit clarification of ambiguities that the
// planner may not be able to resolve on its own.
type Enum struct {
	FailureMode *EnumFailureMode `json:"failureMode"`
	// Values is an explicit mapping from source enum value name to target enum value name. Only
	// explicit mappings need be placed in this map, as the planner will attempt to infer mappings
	// for values with similar names.
	Values map[string]string `json:"values"`
}

func (e *Enum) ApplyPreset(preset PresetEnum) {
	if e.FailureMode == nil && preset.FailureMode != nil {
		*e.FailureMode = *preset.FailureMode
	}
}

type Struct struct {
	// Fields is a map from source field name to target field name. Only explicit mappings need to
	// be placed in this map, the planner will attempt to infer mappings for similarly named fields.
	Fields map[string]string `json:"fields"`
}

// Mappers represents configuration for how mapper functions should be generated for a type
// pair. For bidirectional mapping, both forward and inverse mapper configuration may be provided,
// otherwise only forward mapping configuration is necessary.
type Mappers struct {
	Forward Mapper `json:"forward"`
	Inverse Mapper `json:"inverse"`
}

// Mapper represents configuration for how the mapper function should be generated, allowing
// customization of things like naming, and the signature of the function.
type Mapper struct {
	// Name can be used to provide an exact name, but also supports pattern-based naming using
	// predefined placeholders. Available placeholders are:
	// TODO: Define and list placeholders...
	Name string `json:"name"`

	// Signature allows the signature of a generated mapper function to be customized.
	Signature MapperSignature `json:"signature"`
}

// ParameterKind enumerates the different kinds of parameters that can be passed to a mapper
// function; used to adjust the signature of a generated mapper.
type ParameterKind string

const (
	ParameterKindPointer ParameterKind = "pointer"
	ParameterKindValue   ParameterKind = "value"
)

// MapperSignature configures the signature of a generated mapper function, allowing for
// customization of the generated code.
type MapperSignature struct {
	Accepts ParameterKind `json:"accepts"`
	Returns ParameterKind `json:"returns"`
}
