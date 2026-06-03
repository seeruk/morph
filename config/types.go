package config

import "github.com/seeruk/morph/spec"

// Config is Morph's user-written configuration model. Pointer fields distinguish omitted values
// from explicit zero values so resolution can apply defaults and presets correctly.
type Config struct {
	Defaults  Defaults          `json:"defaults"`
	Discovery Discovery         `json:"discovery"`
	Packages  []Package         `json:"packages"`
	Presets   map[string]Preset `json:"presets"`
}

// Defaults represents available options for configuring Morph defaults.
type Defaults struct {
	Packages PackagesDefaults `json:"packages"`
}

// PackagesDefaults represents available options for configuring package-level defaults.
type PackagesDefaults struct {
	Types  TypesDefaults `json:"types"`
	Output Output        `json:"output"`
}

// TypesDefaults represents available options for configuring type-pair-level defaults.
type TypesDefaults struct {
	Enum          *EnumDefaults        `json:"enum"`
	Mappers       *MappersDefaults     `json:"mappers"`
	Optionality   *OptionalityDefaults `json:"optionality"`
	Bidirectional *bool                `json:"bidirectional"`
}

// EnumDefaults represents available options for configuring enum mapping defaults.
type EnumDefaults struct {
	FailureMode *spec.EnumFailureMode `json:"failureMode"`
}

// Discovery represents available options for configuring auto-discovery of mapping functions.
type Discovery struct {
	Packages   []string           `json:"packages"`
	Functions  []spec.CallableRef `json:"functions"`
	Exclusions []spec.CallableRef `json:"exclusions"`
}

// Preset provides a repeatable, easily referenced set of type defaults to apply.
type Preset struct {
	Enum          *EnumDefaults        `json:"enum"`
	Mappers       *MappersDefaults     `json:"mappers"`
	Optionality   *OptionalityDefaults `json:"optionality"`
	Bidirectional *bool                `json:"bidirectional"`
}

// Package represents the mapping configuration of a pair of packages, and types within them.
type Package struct {
	Source        string               `json:"source"`
	Target        string               `json:"target"`
	Preset        string               `json:"preset"`
	Types         []Type               `json:"types"`
	Output        Output               `json:"output"`
	Enum          *EnumDefaults        `json:"enum"`
	Mappers       *MappersDefaults     `json:"mappers"`
	Optionality   *OptionalityDefaults `json:"optionality"`
	Bidirectional *bool                `json:"bidirectional"`
}

// Type represents the mapping configuration for a specific pair of types.
type Type struct {
	Name          string               `json:"name"`
	Source        string               `json:"source"`
	Target        string               `json:"target"`
	Preset        string               `json:"preset"`
	Enum          *Enum                `json:"enum"`
	Struct        *Struct              `json:"struct"`
	Mappers       *MappersDefaults     `json:"mappers"`
	Optionality   *OptionalityDefaults `json:"optionality"`
	Bidirectional *bool                `json:"bidirectional"`
}

// Output represents defaultable output configuration.
type Output struct {
	Strategy *spec.OutputStrategy `json:"strategy"`
	Path     string               `json:"path"`
	Package  string               `json:"package"`
	Filename string               `json:"filename"`
}

// Enum represents defaultable enum mapping configuration.
type Enum struct {
	FailureMode *spec.EnumFailureMode `json:"failureMode"`
	Patterns    *EnumPatterns         `json:"patterns"`
	Values      map[string]string     `json:"values"`
}

// EnumPatterns allows patterns to be configured for matching enums.
type EnumPatterns struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Struct holds configuration for how a struct should be mapped.
type Struct struct {
	Fields map[string]Field `json:"fields"`
}

// Field holds configuration for how a specific field should be mapped.
type Field struct {
	Target      string               `json:"target"`
	Optionality *OptionalityDefaults `json:"optionality"`
}

// MappersDefaults configures partial mapper defaults that can be layered with other defaults.
type MappersDefaults struct {
	Forward *MapperDefaults `json:"forward"`
	Inverse *MapperDefaults `json:"inverse"`
}

// MapperDefaults represents partial configuration for how a mapper function should be generated.
type MapperDefaults struct {
	Name      *string                  `json:"name"`
	Signature *MapperSignatureDefaults `json:"signature"`
}

// MapperSignatureDefaults configures partial mapper function signature defaults.
type MapperSignatureDefaults struct {
	Accepts *spec.ParameterKind `json:"accepts"`
	Returns *spec.ParameterKind `json:"returns"`
}

// OptionalityDefaults configures partial optionality defaults.
type OptionalityDefaults struct {
	OnNilSourcePointer *spec.PointerOptionality `json:"onNilSourcePointer"`
	OnZeroSourceValue  *spec.ValueOptionality   `json:"onZeroSourceValue"`
}
