package config

import (
	"encoding/json"

	"github.com/seeruk/morph/spec"
)

// Config is Morph's user-written configuration model. Pointer fields distinguish omitted values
// from explicit zero values so resolution can apply defaults and presets correctly.
type Config struct {
	Defaults    Defaults          `json:"defaults"`
	Discovery   Discovery         `json:"discovery"`
	Conversions []Conversion      `json:"conversions"`
	Packages    []Package         `json:"packages"`
	Presets     map[string]Preset `json:"presets"`
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
	Enum          *EnumDefaults              `json:"enum"`
	Callables     []spec.CallableRef         `json:"callables"`
	Mappers       *DirectionalMapperDefaults `json:"mappers"`
	Optionality   *OptionalityDefaults       `json:"optionality"`
	Conversions   *ConversionsDefaults       `json:"conversions"`
	Bidirectional *bool                      `json:"bidirectional"`
}

// EnumDefaults represents available options for configuring enum mapping defaults.
type EnumDefaults struct {
	FailureMode *spec.EnumFailureMode `json:"failureMode"`
	Patterns    *EnumPatterns         `json:"patterns"`
}

// DirectionalMapperDefaults configures partial mapper defaults that can be layered with other defaults.
type DirectionalMapperDefaults struct {
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
	UseIsZeroMethod    *bool                    `json:"useIsZeroMethod"`
}

// ConversionsDefaults configures scoped conversion policy defaults.
type ConversionsDefaults struct {
	Enabled *bool `json:"enabled"`
}

// Output represents defaultable output configuration.
type Output struct {
	Strategy *spec.OutputStrategy `json:"strategy"`
	Path     string               `json:"path"`
	Package  string               `json:"package"`
	Filename string               `json:"filename"`
}

// Discovery represents available options for configuring broad package auto-discovery.
type Discovery struct {
	Packages   []string           `json:"packages"`
	Exclusions []spec.CallableRef `json:"exclusions"`
}

// Conversion represents grouped user-written conversion configuration.
type Conversion struct {
	Source        spec.TypeRef   `json:"source"`
	Targets       []spec.TypeRef `json:"targets"`
	Bidirectional bool           `json:"bidirectional"`
}

// Package represents the mapping configuration of a pair of packages, and types within them.
type Package struct {
	Source        string                     `json:"source"`
	Target        string                     `json:"target"`
	Preset        string                     `json:"preset"`
	Callables     []spec.CallableRef         `json:"callables"`
	Types         []Type                     `json:"types"`
	Output        Output                     `json:"output"`
	Enum          *EnumDefaults              `json:"enum"`
	Mappers       *DirectionalMapperDefaults `json:"mappers"`
	Optionality   *OptionalityDefaults       `json:"optionality"`
	Conversions   *ConversionsDefaults       `json:"conversions"`
	Bidirectional *bool                      `json:"bidirectional"`
}

// Type represents the mapping configuration for a specific pair of types.
type Type struct {
	Name          string                     `json:"name"`
	Source        string                     `json:"source"`
	Target        string                     `json:"target"`
	Preset        string                     `json:"preset"`
	Callables     []spec.CallableRef         `json:"callables"`
	Enum          *Enum                      `json:"enum"`
	Struct        *Struct                    `json:"struct"`
	Mappers       *DirectionalMapperDefaults `json:"mappers"`
	Optionality   *OptionalityDefaults       `json:"optionality"`
	Conversions   *ConversionsDefaults       `json:"conversions"`
	Bidirectional *bool                      `json:"bidirectional"`
}

// Enum represents defaultable enum mapping configuration.
type Enum struct {
	FailureMode *spec.EnumFailureMode `json:"failureMode"`
	Fallback    *EnumFallback         `json:"fallback"`
	Patterns    *EnumPatterns         `json:"patterns"`
	Values      map[string]string     `json:"values"`
}

// EnumFallback configures directional fallback target constants for fallback enum mappings.
type EnumFallback struct {
	Forward string `json:"forward"`
	Inverse string `json:"inverse"`
}

// EnumPatterns allows patterns to be configured for matching enums.
type EnumPatterns struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// Struct holds configuration for how a struct should be mapped.
type Struct struct {
	InferMethods *bool           `json:"inferMethods"`
	Properties   []Property      `json:"properties"`
	Omit         StructOmissions `json:"omit"`
}

// Property holds configuration for how a specific logical property should be mapped.
type Property struct {
	Name        string                        `json:"name"`
	Source      string                        `json:"source"`
	Target      string                        `json:"target"`
	Accessors   DirectionalPropertyAccessors  `json:"accessors"`
	Callable    *DirectionalPropertyCallables `json:"callable"`
	Conversions *ConversionsDefaults          `json:"conversions"`
	Optionality *OptionalityDefaults          `json:"optionality"`
}

// DirectionalPropertyAccessors configures exact accessors for each generated mapping direction.
type DirectionalPropertyAccessors struct {
	Forward PropertyAccessors `json:"forward"`
	Inverse PropertyAccessors `json:"inverse"`
}

// PropertyAccessors configures exact read and write accessor names.
type PropertyAccessors struct {
	Read  string `json:"read"`
	Write string `json:"write"`
}

// DirectionalPropertyCallables configures the explicit callable to use for each mapping direction.
type DirectionalPropertyCallables struct {
	Forward *PropertyCallableInvocation `json:"forward"`
	Inverse *PropertyCallableInvocation `json:"inverse"`
}

// PropertyCallableInvocation configures a callable reference and the context source arguments to pass
// to it. It supports a shorthand string form, which is equivalent to setting Ref with no Args.
type PropertyCallableInvocation struct {
	Ref  spec.CallableRef             `json:"ref"`
	Args []PropertyCallableContextArg `json:"args"`
}

func (i *PropertyCallableInvocation) UnmarshalJSON(data []byte) error {
	var ref spec.CallableRef
	if err := json.Unmarshal(data, &ref); err == nil {
		*i = PropertyCallableInvocation{Ref: ref}
		return nil
	}

	type propertyCallableInvocation PropertyCallableInvocation
	var out propertyCallableInvocation
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	*i = PropertyCallableInvocation(out)
	return nil
}

func (i PropertyCallableInvocation) MarshalJSON() ([]byte, error) {
	if len(i.Args) == 0 {
		return json.Marshal(i.Ref)
	}

	type propertyCallableInvocation PropertyCallableInvocation
	return json.Marshal(propertyCallableInvocation(i))
}

// PropertyCallableContextArg configures one exact source field or zero-arg method to pass as
// callable context.
type PropertyCallableContextArg struct {
	Source string `json:"source"`
}

// StructOmissions configures properties intentionally omitted from a mapping.
type StructOmissions struct {
	Both   []string `json:"both"`
	Source []string `json:"source"`
	Target []string `json:"target"`
}

// Preset provides a repeatable, easily referenced set of type defaults to apply.
type Preset struct {
	Enum          *EnumDefaults              `json:"enum"`
	Callables     []spec.CallableRef         `json:"callables"`
	Mappers       *DirectionalMapperDefaults `json:"mappers"`
	Optionality   *OptionalityDefaults       `json:"optionality"`
	Conversions   *ConversionsDefaults       `json:"conversions"`
	Bidirectional *bool                      `json:"bidirectional"`
}
