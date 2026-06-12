package config

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/seeruk/morph/spec"
)

const (
	defaultForwardMapperName = "Map{{ .Source.Package }}{{ .Source.Type }}To{{ .Target.Package }}{{ .Target.Type }}"
	defaultInverseMapperName = "Map{{ .Source.Package }}{{ .Source.Type }}From{{ .Target.Package }}{{ .Target.Type }}"
)

var defaultTypesDefaults = TypesDefaults{
	Enum: &EnumDefaults{
		FailureMode: new(spec.EnumFailureModeError),
	},
	Mappers: &MappersDefaults{
		Forward: &MapperDefaults{
			Name: new(defaultForwardMapperName),
			Signature: &MapperSignatureDefaults{
				Accepts: new(spec.ParameterKindValue),
				Returns: new(spec.ParameterKindValue),
			},
		},
		Inverse: &MapperDefaults{
			Name: new(defaultInverseMapperName),
			Signature: &MapperSignatureDefaults{
				Accepts: new(spec.ParameterKindValue),
				Returns: new(spec.ParameterKindValue),
			},
		},
	},
	Optionality: &OptionalityDefaults{
		OnNilSourcePointer: new(spec.PointerOptionalityZero),
		OnZeroSourceValue:  new(spec.ValueOptionalityNil),
	},
	Conversions: &ConversionsDefaults{
		Enabled: new(true),
	},
	Bidirectional: new(false),
}

// TODO: Revise?
var defaultOutput = Output{
	Strategy: new(spec.OutputStrategySinglePackage),
	Package:  "mapping",
	Path:     "mapping",
	Filename: "mapping.morph.go",
}

var defaultMapperSignature = spec.MapperSignature{
	Accepts: spec.ParameterKindValue,
	Returns: spec.ParameterKindValue,
}

// Resolve turns user-written config into a fully resolved planner-ready spec.
func Resolve(cfg Config) (spec.Spec, error) {
	defaultOutput := mergeOutput(cfg.Defaults.Packages.Output, defaultOutput)
	defaultTypes := mergeTypeDefaults(cfg.Defaults.Packages.Types, defaultTypesDefaults)
	conversions, err := resolveConversions(cfg.Conversions)
	if err != nil {
		return spec.Spec{}, err
	}

	out := spec.Spec{
		Defaults: spec.Defaults{
			Types: resolvedTypeDefaults(defaultTypes),
		},
		Discovery:   resolveDiscovery(cfg.Discovery),
		Conversions: conversions,
	}

	callables := makeCallablePriorities()
	callables.Add(spec.CallablePriorityDefaults, defaultTypes.Callables)

	for i, pkg := range cfg.Packages {
		if len(pkg.Types) == 0 {
			continue
		}

		resolved, err := resolvePackage(pkg, defaultOutput, defaultTypes, callables, cfg.Presets, i)
		if err != nil {
			return spec.Spec{}, err
		}

		if len(resolved.Types) > 0 {
			out.Packages = append(out.Packages, resolved)
		}
	}

	if countResolvedTypes(out.Packages) == 0 {
		return spec.Spec{}, errors.New("no mappings specified; only found empty packages and/or types")
	}

	return out, nil
}

func resolvePackage(
	pkg Package,
	outputDefaults Output,
	typeDefaults TypesDefaults,
	callables callablePriorities,
	presets map[string]Preset,
	index int,
) (spec.Package, error) {
	if pkg.Source == "" {
		return spec.Package{}, fmt.Errorf("packages[%d]: source package is required", index)
	}
	if pkg.Target == "" {
		return spec.Package{}, fmt.Errorf("packages[%d]: target package is required", index)
	}

	callables = callables.Clone()
	if pkg.Preset != "" {
		preset, ok := presets[pkg.Preset]
		if !ok {
			return spec.Package{}, fmt.Errorf("packages[%d]: preset not found %q", index, pkg.Preset)
		}
		callables.Add(spec.CallablePriorityPackagePreset, preset.Callables)
		typeDefaults = applyPresetToTypeDefaults(typeDefaults, preset)
	}

	callables.Add(spec.CallablePriorityPackage, pkg.Callables)
	typeDefaults = applyPackageToTypeDefaults(typeDefaults, pkg)

	out := spec.Package{
		Source: pkg.Source,
		Target: pkg.Target,
		Output: resolveOutput(mergeOutput(pkg.Output, outputDefaults)),
	}

	for i, typ := range pkg.Types {
		forward, inverse, err := resolveType(typ, typeDefaults, callables, presets, index, i)
		if err != nil {
			return spec.Package{}, err
		}

		forward.SourcePackage = pkg.Source
		forward.TargetPackage = pkg.Target
		out.Types = append(out.Types, forward)
		if inverse != nil {
			inverse.SourcePackage = pkg.Target
			inverse.TargetPackage = pkg.Source
			out.Types = append(out.Types, *inverse)
		}
	}

	return out, nil
}

func resolveType(
	typ Type,
	typeDefaults TypesDefaults,
	callables callablePriorities,
	presets map[string]Preset,
	packageIndex, typeIndex int,
) (spec.Type, *spec.Type, error) {
	callables = callables.Clone()
	if typ.Preset != "" {
		preset, ok := presets[typ.Preset]
		if !ok {
			return spec.Type{}, nil, fmt.Errorf(
				"packages[%d].types[%d]: preset not found %q",
				packageIndex,
				typeIndex,
				typ.Preset,
			)
		}
		callables.Add(spec.CallablePriorityTypePreset, preset.Callables)
		typeDefaults = applyPresetToTypeDefaults(typeDefaults, preset)
	}

	source, target, err := resolveTypeNames(typ)
	if err != nil {
		return spec.Type{}, nil, fmt.Errorf("packages[%d].types[%d]: %w", packageIndex, typeIndex, err)
	}

	enum := resolveEnum(typ.Enum, typeDefaults.Enum)
	mappers := resolveMappers(mergeMappersDefaults(typ.Mappers, typeDefaults.Mappers))
	optionality := optionalityFromDefaults(mergeOptionalityDefaults(typ.Optionality, typeDefaults.Optionality))
	conversion := conversionsFromDefaults(mergeConversionsDefaults(typ.Conversions, typeDefaults.Conversions))
	callables.Add(spec.CallablePriorityType, typ.Callables)
	structure, err := resolveStruct(typ.Struct, optionality, conversion, true)
	if err != nil {
		return spec.Type{}, nil, fmt.Errorf("packages[%d].types[%d].struct: %w", packageIndex, typeIndex, err)
	}

	forward := spec.Type{
		Source:      source,
		Target:      target,
		Enum:        enum,
		Callables:   callables.Ordered(),
		Struct:      structure,
		Mapper:      mappers.Forward,
		Optionality: optionality,
		Conversions: conversion,
	}

	if !boolWithDefault(typ.Bidirectional, typeDefaults.Bidirectional) {
		return forward, nil, nil
	}

	inverseStructure, err := resolveStruct(typ.Struct, optionality, conversion, false)
	if err != nil {
		return spec.Type{}, nil, fmt.Errorf("packages[%d].types[%d].struct: %w", packageIndex, typeIndex, err)
	}

	inverse := spec.Type{
		Source:      target,
		Target:      source,
		Enum:        invertEnum(enum),
		Callables:   callables.Ordered(),
		Struct:      inverseStructure,
		Mapper:      mappers.Inverse,
		Optionality: optionality,
		Conversions: conversion,
	}

	return forward, &inverse, nil
}

func resolveTypeNames(typ Type) (string, string, error) {
	source := cmp.Or(typ.Source, typ.Name)
	target := cmp.Or(typ.Target, typ.Name)
	if source == "" || target == "" {
		return "", "", errors.New("either name, or source and target type names are required")
	}
	return source, target, nil
}

func resolveDiscovery(discovery Discovery) spec.Discovery {
	return spec.Discovery{
		Packages:   slices.Clone(discovery.Packages),
		Exclusions: slices.Clone(discovery.Exclusions),
	}
}

func resolveConversions(conversions []Conversion) ([]spec.Conversion, error) {
	var out []spec.Conversion
	seen := make(map[spec.Conversion]struct{})

	add := func(conversion spec.Conversion) {
		if _, ok := seen[conversion]; ok {
			return
		}
		seen[conversion] = struct{}{}
		out = append(out, conversion)
	}

	var zero spec.TypeRef
	for i, conversion := range conversions {
		if conversion.Source == zero {
			return nil, fmt.Errorf("conversions[%d]: source is required", i)
		}
		if len(conversion.Targets) == 0 {
			return nil, fmt.Errorf("conversions[%d]: at least one target is required", i)
		}

		for j, target := range conversion.Targets {
			if target == zero {
				return nil, fmt.Errorf("conversions[%d].targets[%d]: target is required", i, j)
			}

			resolved := spec.Conversion{
				Source: conversion.Source,
				Target: target,
			}
			add(resolved)

			if conversion.Bidirectional {
				add(spec.Conversion{
					Source: resolved.Target,
					Target: resolved.Source,
				})
			}
		}
	}

	return out, nil
}

func resolveOutput(output Output) spec.Output {
	return spec.Output{
		Strategy: *output.Strategy,
		Path:     output.Path,
		Package:  output.Package,
		Filename: output.Filename,
	}
}

func resolvedTypeDefaults(defaults TypesDefaults) spec.TypeDefaults {
	return spec.TypeDefaults{
		Enum:        resolveEnum(nil, defaults.Enum),
		Callables:   slices.Clone(defaults.Callables),
		Mappers:     resolveMappers(defaults.Mappers),
		Optionality: optionalityFromDefaults(defaults.Optionality),
		Conversions: conversionsFromDefaults(defaults.Conversions),
	}
}

func resolveEnum(enum *Enum, defaults *EnumDefaults) spec.Enum {
	out := spec.Enum{}
	if defaults != nil {
		if defaults.FailureMode != nil {
			out.FailureMode = *defaults.FailureMode
		}
		out.Patterns = enumPatternsFromDefaults(defaults.Patterns)
	}
	if enum != nil {
		if enum.FailureMode != nil {
			out.FailureMode = *enum.FailureMode
		}
		out.Patterns = enumPatternsFromOverrides(enum.Patterns, out.Patterns)
		out.Values = maps.Clone(enum.Values)
	}
	return out
}

func mergeEnumDefaults(overrides *EnumDefaults, fallback *EnumDefaults) *EnumDefaults {
	overrides = cmp.Or(overrides, new(EnumDefaults))
	fallback = cmp.Or(fallback, new(EnumDefaults))
	return &EnumDefaults{
		FailureMode: cmp.Or(overrides.FailureMode, fallback.FailureMode),
		Patterns:    mergeEnumPatterns(overrides.Patterns, fallback.Patterns),
	}
}

func mergeEnumPatterns(overrides *EnumPatterns, fallback *EnumPatterns) *EnumPatterns {
	if overrides == nil && fallback == nil {
		return nil
	}

	var out, zero EnumPatterns
	if fallback != nil {
		out = *fallback
	}
	if overrides != nil {
		out.Source = cmp.Or(overrides.Source, out.Source)
		out.Target = cmp.Or(overrides.Target, out.Target)
	}
	if out == zero {
		return nil
	}
	return &out
}

func enumPatternsFromDefaults(patterns *EnumPatterns) spec.EnumPatterns {
	if patterns == nil {
		return spec.EnumPatterns{}
	}
	return spec.EnumPatterns{
		Source: patterns.Source,
		Target: patterns.Target,
	}
}

func enumPatternsFromOverrides(overrides *EnumPatterns, fallback spec.EnumPatterns) spec.EnumPatterns {
	if overrides == nil {
		return fallback
	}
	return spec.EnumPatterns{
		Source: cmp.Or(overrides.Source, fallback.Source),
		Target: cmp.Or(overrides.Target, fallback.Target),
	}
}

func invertEnum(enum spec.Enum) spec.Enum {
	enum.Values = invertStringMap(enum.Values)
	return enum
}

type callablePriorities map[spec.CallablePriority][]spec.CallableRef

func makeCallablePriorities() callablePriorities {
	return make(callablePriorities)
}

func (c callablePriorities) Add(priority spec.CallablePriority, callables []spec.CallableRef) {
	if len(callables) == 0 {
		return
	}
	c[priority] = append(c[priority], callables...)
}

func (c callablePriorities) Clone() callablePriorities {
	out := makeCallablePriorities()
	for priority, callables := range c {
		out[priority] = slices.Clone(callables)
	}
	return out
}

func (c callablePriorities) Ordered() []spec.PrioritizedCallables {
	out := make([]spec.PrioritizedCallables, 0, spec.CallablePriorityCount())
	for i := range spec.CallablePriorityCount() {
		priority := spec.CallablePriority(i)
		callables := c[priority]
		if len(callables) == 0 {
			continue
		}
		out = append(out, spec.PrioritizedCallables{
			Priority:  priority,
			Callables: slices.Clone(callables),
		})
	}
	return out
}

func resolveStruct(
	structure *Struct,
	optionality spec.Optionality,
	conversion spec.ConversionsPolicy,
	forward bool,
) (spec.Struct, error) {
	inferMethods := true
	if structure == nil {
		return spec.Struct{InferMethods: inferMethods}, nil
	}
	if structure.InferMethods != nil {
		inferMethods = *structure.InferMethods
	}

	out := spec.Struct{
		InferMethods: inferMethods,
		Properties:   make([]spec.Property, 0, len(structure.Properties)),
		Omit:         resolveStructOmissions(structure.Omit),
	}
	if !forward {
		out.Omit = invertStructOmissions(out.Omit)
	}

	sourceIndexes := make(map[string][]int, len(structure.Properties))
	targetIndexes := make(map[string][]int, len(structure.Properties))
	for i, property := range structure.Properties {
		resolved, err := resolveStructProperty(property, optionality, conversion, forward)
		if err != nil {
			return spec.Struct{}, fmt.Errorf("properties[%d]: %w", i, err)
		}
		sourceIndexes[resolved.Source] = append(sourceIndexes[resolved.Source], i)
		targetIndexes[resolved.Target] = append(targetIndexes[resolved.Target], i)
		out.Properties = append(out.Properties, resolved)
	}
	if err := duplicateStructPropertiesError(sourceIndexes, targetIndexes); err != nil {
		return spec.Struct{}, err
	}

	return out, nil
}

func resolveStructProperty(
	property Property,
	optionality spec.Optionality,
	conversion spec.ConversionsPolicy,
	forward bool,
) (spec.Property, error) {
	source, target, err := resolvePropertyNames(property)
	if err != nil {
		return spec.Property{}, err
	}

	accessors := property.Accessors.Forward
	if !forward {
		source, target = target, source
		accessors = property.Accessors.Inverse
	}

	return spec.Property{
		Source:      source,
		Target:      target,
		Accessors:   spec.PropertyAccessors{Read: accessors.Read, Write: accessors.Write},
		Callable:    resolvePropertyCallable(property.Callable, forward),
		Optionality: optionalityFromOverrides(property.Optionality, optionality),
		Conversions: conversionsFromOverrides(property.Conversions, conversion),
	}, nil
}

func resolvePropertyNames(property Property) (string, string, error) {
	if property.Name != "" {
		if property.Source != "" || property.Target != "" {
			return "", "", errors.New("name cannot be combined with source or target")
		}
		return property.Name, property.Name, nil
	}
	if property.Source == "" || property.Target == "" {
		return "", "", errors.New("either name, or source and target property names are required")
	}
	return property.Source, property.Target, nil
}

func duplicateStructPropertiesError(
	sourceIndexes map[string][]int,
	targetIndexes map[string][]int,
) error {
	parts := duplicatePropertyMessages("source", sourceIndexes)
	parts = append(parts, duplicatePropertyMessages("target", targetIndexes)...)
	if len(parts) == 0 {
		return nil
	}
	return fmt.Errorf("duplicate struct property mappings: %s", strings.Join(parts, "; "))
}

func duplicatePropertyMessages(side string, indexesByName map[string][]int) []string {
	names := slices.Collect(maps.Keys(indexesByName))
	slices.Sort(names)

	var messages []string
	for _, name := range names {
		indexes := indexesByName[name]
		if len(indexes) < 2 {
			continue
		}
		messages = append(
			messages,
			fmt.Sprintf("%s property %q appears in %s", side, name, formatPropertyIndexes(indexes)),
		)
	}
	return messages
}

func formatPropertyIndexes(indexes []int) string {
	indexes = slices.Clone(indexes)
	slices.Sort(indexes)

	parts := make([]string, len(indexes))
	for i, index := range indexes {
		parts[i] = fmt.Sprintf("properties[%d]", index)
	}
	return strings.Join(parts, ", ")
}

func resolveStructOmissions(omit StructOmissions) spec.StructOmissions {
	return spec.StructOmissions{
		Source: dedupeStrings(omit.Source),
		Target: dedupeStrings(omit.Target),
	}
}

func dedupeStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	out := slices.Clone(values)
	slices.Sort(out)
	return slices.Compact(out)
}

func resolvePropertyCallable(callable *PropertyCallable, forward bool) *spec.CallableRef {
	if callable == nil {
		return nil
	}
	if forward {
		return cloneCallableRef(callable.Forward)
	}
	return cloneCallableRef(callable.Inverse)
}

func cloneCallableRef(ref *spec.CallableRef) *spec.CallableRef {
	if ref == nil {
		return nil
	}
	out := *ref
	return &out
}

func structOmissionsEmpty(omit spec.StructOmissions) bool {
	return len(omit.Source) == 0 && len(omit.Target) == 0
}

func invertStructOmissions(in spec.StructOmissions) spec.StructOmissions {
	return spec.StructOmissions{
		Source: slices.Clone(in.Target),
		Target: slices.Clone(in.Source),
	}
}

func invertStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}

	out := make(map[string]string, len(in))
	for source, target := range in {
		out[target] = source
	}
	return out
}

func mergeMappersDefaults(overrides *MappersDefaults, fallback *MappersDefaults) *MappersDefaults {
	overrides = cmp.Or(overrides, new(MappersDefaults))
	fallback = cmp.Or(fallback, new(MappersDefaults))
	return &MappersDefaults{
		Forward: mergeMapperDefaults(overrides.Forward, fallback.Forward),
		Inverse: mergeMapperDefaults(overrides.Inverse, fallback.Inverse),
	}
}

func mergeMapperDefaults(overrides *MapperDefaults, fallback *MapperDefaults) *MapperDefaults {
	overrides = cmp.Or(overrides, new(MapperDefaults))
	fallback = cmp.Or(fallback, new(MapperDefaults))
	return &MapperDefaults{
		Name:      cmp.Or(overrides.Name, fallback.Name),
		Signature: mergeMapperSignatureDefaults(overrides.Signature, fallback.Signature),
	}
}

func mergeMapperSignatureDefaults(
	overrides *MapperSignatureDefaults,
	fallback *MapperSignatureDefaults,
) *MapperSignatureDefaults {
	overrides = cmp.Or(overrides, new(MapperSignatureDefaults))
	fallback = cmp.Or(fallback, new(MapperSignatureDefaults))
	return &MapperSignatureDefaults{
		Accepts: cmp.Or(overrides.Accepts, fallback.Accepts),
		Returns: cmp.Or(overrides.Returns, fallback.Returns),
	}
}

func resolveMappers(mappers *MappersDefaults) spec.Mappers {
	return spec.Mappers{
		Forward: resolveMapper(mapperDefaults(mappers, true), defaultMapper(defaultForwardMapperName)),
		Inverse: resolveMapper(mapperDefaults(mappers, false), defaultMapper(defaultInverseMapperName)),
	}
}

func mapperDefaults(mappers *MappersDefaults, forward bool) *MapperDefaults {
	if mappers == nil {
		return nil
	}
	if forward {
		return mappers.Forward
	}
	return mappers.Inverse
}

func defaultMapper(name string) spec.Mapper {
	return spec.Mapper{
		Name:      name,
		Signature: defaultMapperSignature,
	}
}

func resolveMapper(mapper *MapperDefaults, fallback spec.Mapper) spec.Mapper {
	if mapper == nil {
		return fallback
	}
	if mapper.Name != nil {
		fallback.Name = *mapper.Name
	}
	fallback.Signature = resolveMapperSignature(mapper.Signature, fallback.Signature)
	return fallback
}

func resolveMapperSignature(
	signature *MapperSignatureDefaults,
	fallback spec.MapperSignature,
) spec.MapperSignature {
	if signature == nil {
		return fallback
	}
	if signature.Accepts != nil {
		fallback.Accepts = *signature.Accepts
	}
	if signature.Returns != nil {
		fallback.Returns = *signature.Returns
	}
	return fallback
}

func mergeOptionalityDefaults(
	overrides *OptionalityDefaults,
	fallback *OptionalityDefaults,
) *OptionalityDefaults {
	overrides = cmp.Or(overrides, new(OptionalityDefaults))
	fallback = cmp.Or(fallback, new(OptionalityDefaults))
	return &OptionalityDefaults{
		OnNilSourcePointer: cmp.Or(overrides.OnNilSourcePointer, fallback.OnNilSourcePointer),
		OnZeroSourceValue:  cmp.Or(overrides.OnZeroSourceValue, fallback.OnZeroSourceValue),
	}
}

func optionalityFromOverrides(overrides *OptionalityDefaults, fallback spec.Optionality) spec.Optionality {
	if overrides == nil {
		return fallback
	}
	if overrides.OnNilSourcePointer != nil {
		fallback.OnNilSourcePointer = *overrides.OnNilSourcePointer
	}
	if overrides.OnZeroSourceValue != nil {
		fallback.OnZeroSourceValue = *overrides.OnZeroSourceValue
	}
	return fallback
}

func optionalityFromDefaults(optionality *OptionalityDefaults) spec.Optionality {
	return optionalityFromOverrides(optionality, defaultOptionality())
}

func defaultOptionality() spec.Optionality {
	return spec.Optionality{
		OnNilSourcePointer: spec.PointerOptionalityZero,
		OnZeroSourceValue:  spec.ValueOptionalityNil,
	}
}

func mergeConversionsDefaults(
	overrides *ConversionsDefaults,
	fallback *ConversionsDefaults,
) *ConversionsDefaults {
	overrides = cmp.Or(overrides, new(ConversionsDefaults))
	fallback = cmp.Or(fallback, new(ConversionsDefaults))
	return &ConversionsDefaults{
		Enabled: cmp.Or(overrides.Enabled, fallback.Enabled),
	}
}

func conversionsFromOverrides(overrides *ConversionsDefaults, fallback spec.ConversionsPolicy) spec.ConversionsPolicy {
	if overrides == nil {
		return fallback
	}
	if overrides.Enabled != nil {
		fallback.Enabled = *overrides.Enabled
	}
	return fallback
}

func conversionsFromDefaults(conversion *ConversionsDefaults) spec.ConversionsPolicy {
	return conversionsFromOverrides(conversion, defaultConversionsPolicy())
}

func defaultConversionsPolicy() spec.ConversionsPolicy {
	return spec.ConversionsPolicy{
		Enabled: true,
	}
}

func mergeOutput(overrides Output, fallback Output) Output {
	return Output{
		Strategy: cmp.Or(overrides.Strategy, fallback.Strategy),
		Path:     cmp.Or(overrides.Path, fallback.Path),
		Package:  cmp.Or(overrides.Package, fallback.Package),
		Filename: cmp.Or(overrides.Filename, fallback.Filename),
	}
}

func mergeTypeDefaults(overrides TypesDefaults, fallback TypesDefaults) TypesDefaults {
	return TypesDefaults{
		Enum:          mergeEnumDefaults(overrides.Enum, fallback.Enum),
		Callables:     append(slices.Clone(fallback.Callables), overrides.Callables...),
		Mappers:       mergeMappersDefaults(overrides.Mappers, fallback.Mappers),
		Optionality:   mergeOptionalityDefaults(overrides.Optionality, fallback.Optionality),
		Conversions:   mergeConversionsDefaults(overrides.Conversions, fallback.Conversions),
		Bidirectional: cmp.Or(overrides.Bidirectional, fallback.Bidirectional),
	}
}

func applyPresetToTypeDefaults(defaults TypesDefaults, preset Preset) TypesDefaults {
	return TypesDefaults{
		Enum:          mergeEnumDefaults(preset.Enum, defaults.Enum),
		Mappers:       mergeMappersDefaults(preset.Mappers, defaults.Mappers),
		Optionality:   mergeOptionalityDefaults(preset.Optionality, defaults.Optionality),
		Conversions:   mergeConversionsDefaults(preset.Conversions, defaults.Conversions),
		Bidirectional: cmp.Or(preset.Bidirectional, defaults.Bidirectional),
	}
}

func applyPackageToTypeDefaults(defaults TypesDefaults, pkg Package) TypesDefaults {
	return TypesDefaults{
		Enum:          mergeEnumDefaults(pkg.Enum, defaults.Enum),
		Mappers:       mergeMappersDefaults(pkg.Mappers, defaults.Mappers),
		Optionality:   mergeOptionalityDefaults(pkg.Optionality, defaults.Optionality),
		Conversions:   mergeConversionsDefaults(pkg.Conversions, defaults.Conversions),
		Bidirectional: cmp.Or(pkg.Bidirectional, defaults.Bidirectional),
	}
}

func boolWithDefault(value *bool, def *bool) bool {
	if value != nil {
		return *value
	}
	if def != nil {
		return *def
	}
	return false
}

func countResolvedTypes(packages []spec.Package) int {
	var count int
	for _, pkg := range packages {
		count += len(pkg.Types)
	}
	return count
}
