package config

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"

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

	out := spec.Spec{
		Defaults: spec.Defaults{
			Types: resolvedTypeDefaults(defaultTypes),
		},
		Discovery: resolveDiscovery(cfg.Discovery),
	}

	for i, pkg := range cfg.Packages {
		if len(pkg.Types) == 0 {
			continue
		}

		resolved, err := resolvePackage(pkg, defaultOutput, defaultTypes, cfg.Presets, i)
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
	presets map[string]Preset,
	index int,
) (spec.Package, error) {
	if pkg.Source == "" {
		return spec.Package{}, fmt.Errorf("packages[%d]: source package is required", index)
	}
	if pkg.Target == "" {
		return spec.Package{}, fmt.Errorf("packages[%d]: target package is required", index)
	}

	if pkg.Preset != "" {
		preset, ok := presets[pkg.Preset]
		if !ok {
			return spec.Package{}, fmt.Errorf("packages[%d]: preset not found %q", index, pkg.Preset)
		}
		typeDefaults = applyPresetToTypeDefaults(typeDefaults, preset)
	}

	typeDefaults = applyPackageToTypeDefaults(typeDefaults, pkg)

	out := spec.Package{
		Source: pkg.Source,
		Target: pkg.Target,
		Output: resolveOutput(mergeOutput(pkg.Output, outputDefaults)),
	}

	for i, typ := range pkg.Types {
		forward, inverse, err := resolveType(typ, typeDefaults, presets, index, i)
		if err != nil {
			return spec.Package{}, err
		}

		out.Types = append(out.Types, forward)
		if inverse != nil {
			out.Types = append(out.Types, *inverse)
		}
	}

	return out, nil
}

func resolveType(
	typ Type,
	typeDefaults TypesDefaults,
	presets map[string]Preset,
	packageIndex, typeIndex int,
) (spec.Type, *spec.Type, error) {
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
		typeDefaults = applyPresetToTypeDefaults(typeDefaults, preset)
	}

	source, target, err := resolveTypeNames(typ)
	if err != nil {
		return spec.Type{}, nil, fmt.Errorf("packages[%d].types[%d]: %w", packageIndex, typeIndex, err)
	}

	enum := resolveEnum(typ.Enum, typeDefaults.Enum)
	mappers := resolveMappers(mergeMappersDefaults(typ.Mappers, typeDefaults.Mappers))
	optionality := optionalityFromDefaults(mergeOptionalityDefaults(typ.Optionality, typeDefaults.Optionality))
	structure := resolveStruct(typ.Struct, optionality)

	forward := spec.Type{
		Source:      source,
		Target:      target,
		Enum:        enum,
		Struct:      structure,
		Mapper:      mappers.Forward,
		Optionality: optionality,
	}

	if !boolWithDefault(typ.Bidirectional, typeDefaults.Bidirectional) {
		return forward, nil, nil
	}

	inverse := spec.Type{
		Source:      target,
		Target:      source,
		Enum:        invertEnum(enum),
		Struct:      invertStruct(structure),
		Mapper:      mappers.Inverse,
		Optionality: optionality,
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
		Functions:  slices.Clone(discovery.Functions),
		Exclusions: slices.Clone(discovery.Exclusions),
	}
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
		Mappers:     resolveMappers(defaults.Mappers),
		Optionality: optionalityFromDefaults(defaults.Optionality),
	}
}

func resolveEnum(enum *Enum, defaults *EnumDefaults) spec.Enum {
	out := spec.Enum{}
	if defaults != nil && defaults.FailureMode != nil {
		out.FailureMode = *defaults.FailureMode
	}
	if enum != nil {
		if enum.FailureMode != nil {
			out.FailureMode = *enum.FailureMode
		}
		if enum.Patterns != nil {
			out.Patterns = spec.EnumPatterns{
				Source: enum.Patterns.Source,
				Target: enum.Patterns.Target,
			}
		}
		out.Values = maps.Clone(enum.Values)
	}
	return out
}

func mergeEnumDefaults(overrides *EnumDefaults, fallback *EnumDefaults) *EnumDefaults {
	overrides = cmp.Or(overrides, new(EnumDefaults))
	fallback = cmp.Or(fallback, new(EnumDefaults))
	return &EnumDefaults{
		FailureMode: cmp.Or(overrides.FailureMode, fallback.FailureMode),
	}
}

func invertEnum(enum spec.Enum) spec.Enum {
	enum.Values = invertStringMap(enum.Values)
	return enum
}

func resolveStruct(structure *Struct, optionality spec.Optionality) spec.Struct {
	if structure == nil {
		return spec.Struct{}
	}

	out := spec.Struct{
		Fields: make(map[string]spec.Field, len(structure.Fields)),
	}
	for sourceName, field := range structure.Fields {
		targetName := cmp.Or(field.Target, sourceName)
		out.Fields[sourceName] = spec.Field{
			Target:      targetName,
			Optionality: optionalityFromOverrides(field.Optionality, optionality),
		}
	}
	return out
}

func invertStruct(in spec.Struct) spec.Struct {
	if len(in.Fields) == 0 {
		return spec.Struct{}
	}

	out := spec.Struct{
		Fields: make(map[string]spec.Field, len(in.Fields)),
	}
	for sourceName, field := range in.Fields {
		targetName := cmp.Or(field.Target, sourceName)
		out.Fields[targetName] = spec.Field{
			Target:      sourceName,
			Optionality: field.Optionality,
		}
	}
	return out
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
		Mappers:       mergeMappersDefaults(overrides.Mappers, fallback.Mappers),
		Optionality:   mergeOptionalityDefaults(overrides.Optionality, fallback.Optionality),
		Bidirectional: cmp.Or(overrides.Bidirectional, fallback.Bidirectional),
	}
}

func applyPresetToTypeDefaults(defaults TypesDefaults, preset Preset) TypesDefaults {
	return TypesDefaults{
		Enum:          mergeEnumDefaults(preset.Enum, defaults.Enum),
		Mappers:       mergeMappersDefaults(preset.Mappers, defaults.Mappers),
		Optionality:   mergeOptionalityDefaults(preset.Optionality, defaults.Optionality),
		Bidirectional: cmp.Or(preset.Bidirectional, defaults.Bidirectional),
	}
}

func applyPackageToTypeDefaults(defaults TypesDefaults, pkg Package) TypesDefaults {
	return TypesDefaults{
		Enum:          mergeEnumDefaults(pkg.Enum, defaults.Enum),
		Mappers:       mergeMappersDefaults(pkg.Mappers, defaults.Mappers),
		Optionality:   mergeOptionalityDefaults(pkg.Optionality, defaults.Optionality),
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
