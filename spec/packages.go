package spec

import (
	"fmt"
	"strings"

	"github.com/seeruk/morph/internal/mapsx"
)

// Package represents the mapping configuration of a pair of packages, and types within them.
type Package struct {
	Source        string `json:"source"`
	Target        string `json:"target"`
	Preset        string `json:"preset"`
	Types         []Type `json:"types"`
	Output        Output `json:"output"`
	Bidirectional *bool  `json:"bidirectional"`
}

// Type represents the mapping configuration for a specific pair of types.
type Type struct {
	Name          string   `json:"name"`
	Source        string   `json:"source"`
	Target        string   `json:"target"`
	Preset        string   `json:"preset"`
	Enum          *Enum    `json:"enum"`
	Struct        *Struct  `json:"struct"`
	Mappers       *Mappers `json:"mappers"`
	Bidirectional *bool    `json:"bidirectional"`
}

// Enum represents configuration for how an enum mapping function should be generated,
// allowing customization of generated output and explicit clarification of ambiguities that the
// planner may not be able to resolve on its own.
type Enum struct {
	FailureMode *EnumFailureMode `json:"failureMode"`
	Patterns    *EnumPatterns    `json:"patterns"`
	// Values is an explicit mapping from source enum value name to target enum value name. Only
	// explicit mappings need be placed in this map, as the planner will attempt to infer mappings
	// for values with similar names.
	Values map[string]string `json:"values"`
}

func (e *Enum) ApplyDefaults(preset EnumDefaults) {
	if e == nil || e.FailureMode != nil || preset.FailureMode == nil {
		return
	}

	e.FailureMode = new(*preset.FailureMode)
}

// EnumFailureMode enumerates the possible failure modes for an enum mapping function.
type EnumFailureMode uint

const (
	EnumFailureModeError EnumFailureMode = iota
	EnumFailureModeZero
	enumFailureModeMax
)

var enumFailureModeNames = map[EnumFailureMode]string{
	EnumFailureModeError: "error",
	EnumFailureModeZero:  "zero",
}

var enumFailureModesByName = mapsx.Invert(enumFailureModeNames)

func (e EnumFailureMode) MarshalText() ([]byte, error) {
	return []byte(e.String()), nil
}

func (e *EnumFailureMode) UnmarshalText(text []byte) error {
	value, ok := enumFailureModesByName[strings.ToLower(string(text))]
	if !ok {
		return fmt.Errorf("unknown enum failure mode: %q", string(text))
	}
	*e = value
	return nil
}

func (e EnumFailureMode) String() string {
	return enumFailureModeNames[e]
}

// EnumPatterns allows patterns to be configured for matching enums, this can be used to explicitly
// handle difficult to infer mappings.
//
// The format supported is similar how mapping function names can be configured. Available template
// placeholders are based around different casing options for possible elements of the name:
// - <SCREAMING_TYPE>
// - <SCREAMING_VALUE>
// - <PascalType>
// - <PascalValue>
// - <camelValue>
// - <camelValue>
// - <snake_value>
// - <snake_value>
//
// TODO: Do we need more template options? Or something more custom, or lenient?
type EnumPatterns struct {
	Source string `json:"source"`
	Target string `json:"target"`
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
	// Name can be used to provide an exact name, but also supports a template. We use Go's built-in
	// text/template, and the template data is morph.NameInput
	Name string `json:"name"`

	// Signature allows the signature of a generated mapper function to be customized.
	Signature MapperSignature `json:"signature"`
}

// MapperSignature configures the signature of a generated mapper function, allowing for
// customization of the generated code.
type MapperSignature struct {
	Accepts *ParameterKind `json:"accepts"`
	Returns *ParameterKind `json:"returns"`
}

// ParameterKind enumerates the different kinds of parameters that can be passed to a mapper
// function; used to adjust the signature of a generated mapper.
type ParameterKind uint

const (
	ParameterKindValue ParameterKind = iota
	ParameterKindPointer
	parameterKindMax
)

var parameterKindNames = map[ParameterKind]string{
	ParameterKindValue:   "value",
	ParameterKindPointer: "pointer",
}

var parameterKindsByName = mapsx.Invert(parameterKindNames)

func (k ParameterKind) MarshalText() ([]byte, error) {
	return []byte(k.String()), nil
}

func (k *ParameterKind) UnmarshalText(data []byte) error {
	value, ok := parameterKindsByName[strings.ToLower(string(data))]
	if !ok {
		return fmt.Errorf("unknown parameter kind: %q", string(data))
	}

	*k = value
	return nil
}

func (k ParameterKind) String() string {
	return parameterKindNames[k]
}
