package spec

import (
	"fmt"
	"strings"

	"github.com/seeruk/morph/internal/mapsx"
)

// Package represents a resolved mapping configuration for a pair of packages, and the directional
// type mappings within them.
type Package struct {
	Source string
	Target string
	Output Output
	Types  []Type
}

// Type represents resolved configuration for one directional type mapping.
type Type struct {
	Source        string
	Target        string
	SourcePackage string
	TargetPackage string
	Enum          Enum
	Callables     []PrioritizedCallables
	Struct        Struct
	Mapper        Mapper
	Optionality   Optionality
	Conversions   ConversionsPolicy
}

// PrioritizedCallables contains explicit callables from one config-derived priority.
type PrioritizedCallables struct {
	Priority  CallablePriority
	Callables []CallableRef
}

// CallablePriority identifies where a set of explicit callables came from. More specific
// priorities are considered before less specific priorities when planning value mappings.
type CallablePriority uint8

const (
	CallablePriorityType CallablePriority = iota
	CallablePriorityTypePreset
	CallablePriorityPackage
	CallablePriorityPackagePreset
	CallablePriorityDefaults
	callablePriorityMax
)

var callablePriorityNames = map[CallablePriority]string{
	CallablePriorityType:          "type",
	CallablePriorityTypePreset:    "type_preset",
	CallablePriorityPackage:       "package",
	CallablePriorityPackagePreset: "package_preset",
	CallablePriorityDefaults:      "defaults",
}

func (p CallablePriority) String() string {
	return callablePriorityNames[p]
}

// CallablePriorityCount returns the number of valid callable priorities.
func CallablePriorityCount() int {
	return int(callablePriorityMax)
}

// Enum represents configuration for how an enum mapping function should be generated,
// allowing customization of generated output and explicit clarification of ambiguities that the
// planner may not be able to resolve on its own.
type Enum struct {
	FailureMode EnumFailureMode
	Patterns    EnumPatterns
	// Values is an explicit mapping from source enum value name to target enum value name. Only
	// explicit mappings need be placed in this map, as the planner will attempt to infer mappings
	// for values with similar names.
	Values map[string]string
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
type EnumPatterns struct {
	Source string
	Target string
}

// Struct holds configuration for how a struct should be mapped.
type Struct struct {
	// Fields is a map from source field name to target field config. Only explicit mappings need to
	// be placed in this map, the planner will attempt to infer mappings for similarly named fields.
	Fields map[string]Field
	// Omit contains source and target fields intentionally left unmapped.
	Omit StructOmissions
}

// Field holds configuration for how a specific field should be mapped, allowing more explicit
// control over mapping behaviour.
type Field struct {
	Target      string
	Callable    *CallableRef
	Optionality Optionality
	Conversions ConversionsPolicy
}

// StructOmissions contains source and target fields intentionally left unmapped.
type StructOmissions struct {
	Source []string
	Target []string
}

// Mappers holds resolved configuration for how mapper functions should be generated for a type pair.
type Mappers struct {
	Forward Mapper
	Inverse Mapper
}

// Mapper represents resolved configuration for how a mapper function should be generated.
type Mapper struct {
	Name      string
	Signature MapperSignature
}

// MapperSignature configures the resolved signature of a generated mapper function.
type MapperSignature struct {
	Accepts ParameterKind
	Returns ParameterKind
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

// Optionality configures how Morph handles pointer/value optionality boundaries after all defaults
// have been resolved.
type Optionality struct {
	OnNilSourcePointer PointerOptionality
	OnZeroSourceValue  ValueOptionality
}

type PointerOptionality uint

const (
	PointerOptionalityZero PointerOptionality = iota
	PointerOptionalityError
	pointerOptionalityMax
)

var pointerOptionalityNames = map[PointerOptionality]string{
	PointerOptionalityZero:  "zero",
	PointerOptionalityError: "error",
}

var pointerOptionalitiesByName = mapsx.Invert(pointerOptionalityNames)

func (p PointerOptionality) MarshalText() ([]byte, error) {
	return []byte(p.String()), nil
}

func (p *PointerOptionality) UnmarshalText(data []byte) error {
	value, ok := pointerOptionalitiesByName[strings.ToLower(string(data))]
	if !ok {
		return fmt.Errorf("unknown pointer optionality: %q", string(data))
	}

	*p = value
	return nil
}

func (p PointerOptionality) String() string {
	return pointerOptionalityNames[p]
}

type ValueOptionality uint

const (
	ValueOptionalityNil ValueOptionality = iota
	ValueOptionalityAddress
	valueOptionalityMax
)

var valueOptionalityNames = map[ValueOptionality]string{
	ValueOptionalityNil:     "nil",
	ValueOptionalityAddress: "address",
}

var valueOptionalitiesByName = mapsx.Invert(valueOptionalityNames)

func (v ValueOptionality) MarshalText() ([]byte, error) {
	return []byte(v.String()), nil
}

func (v *ValueOptionality) UnmarshalText(data []byte) error {
	value, ok := valueOptionalitiesByName[strings.ToLower(string(data))]
	if !ok {
		return fmt.Errorf("unknown value optionality: %q", string(data))
	}

	*v = value
	return nil
}

func (v ValueOptionality) String() string {
	return valueOptionalityNames[v]
}

// ConversionsPolicy configures whether Morph can use registered named type conversions in a scope.
type ConversionsPolicy struct {
	Enabled bool
}
