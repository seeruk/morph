package morph

import (
	"cmp"

	"github.com/seeruk/morph/spec"
)

const (
	defaultForwardMapperName = "Map{{ .Source.Package }}{{ .Source.Type }}To{{ .Target.Package }}{{ .Target.Type }}"
	defaultInverseMapperName = "Map{{ .Source.Package }}{{ .Source.Type }}From{{ .Target.Package }}{{ .Target.Type }}"
)

var defaultTypesDefaults = spec.TypesDefaults{
	Enum: &spec.EnumDefaults{
		FailureMode: new(spec.EnumFailureModeError),
	},
	Mappers: &spec.Mappers{
		Forward: spec.Mapper{
			Name:      defaultForwardMapperName,
			Signature: defaultMapperSignature,
		},
		Inverse: spec.Mapper{
			Name:      defaultInverseMapperName,
			Signature: defaultMapperSignature,
		},
	},
	Bidirectional: new(false),
}

var defaultMapperSignature = spec.MapperSignature{
	Accepts: new(spec.ParameterKindValue),
	Returns: new(spec.ParameterKindValue),
}

var defaultOutput = spec.Output{
	Strategy: spec.OutputStrategySinglePackage,
	Package:  "morph",
	Path:     "morph",
	Filename: "morph.gen.go",
}

func enumWithDefaults(enum *spec.Enum, defaults *spec.EnumDefaults) *spec.Enum {
	enum = cmp.Or(enum, new(spec.Enum))
	defaults = cmp.Or(defaults, new(spec.EnumDefaults))
	return &spec.Enum{
		FailureMode: cmp.Or(enum.FailureMode, defaults.FailureMode),
		Patterns:    enum.Patterns,
		Values:      enum.Values,
	}
}

func enumDefaultsWithDefaults(enum *spec.EnumDefaults, defaults *spec.EnumDefaults) *spec.EnumDefaults {
	enum = cmp.Or(enum, new(spec.EnumDefaults))
	defaults = cmp.Or(defaults, new(spec.EnumDefaults))
	return &spec.EnumDefaults{
		FailureMode: cmp.Or(enum.FailureMode, defaults.FailureMode),
	}
}

func mappersWithDefaults(mappers *spec.Mappers, defaults *spec.Mappers) *spec.Mappers {
	mappers = cmp.Or(mappers, new(spec.Mappers))
	defaults = cmp.Or(defaults, new(spec.Mappers))
	return &spec.Mappers{
		Forward: mapperWithDefaults(mappers.Forward, defaults.Forward),
		Inverse: mapperWithDefaults(mappers.Inverse, defaults.Inverse),
	}
}

func mapperWithDefaults(mapper spec.Mapper, defaults spec.Mapper) spec.Mapper {
	return spec.Mapper{
		Name:      cmp.Or(mapper.Name, defaults.Name),
		Signature: mapperSignatureWithDefaults(mapper.Signature, defaults.Signature),
	}
}

func mapperSignatureWithDefaults(signature spec.MapperSignature, defaults spec.MapperSignature) spec.MapperSignature {
	return spec.MapperSignature{
		Accepts: cmp.Or(signature.Accepts, defaults.Accepts),
		Returns: cmp.Or(signature.Returns, defaults.Returns),
	}
}

// outputWithDefaults returns the given output with defaults applied, if necessary.
func outputWithDefaults(output spec.Output, defaults spec.Output) spec.Output {
	return spec.Output{
		Strategy: cmp.Or(output.Strategy, defaults.Strategy),
		Path:     cmp.Or(output.Path, defaults.Path),
		Package:  cmp.Or(output.Package, defaults.Package),
		Filename: cmp.Or(output.Filename, defaults.Filename),
	}
}

// typesDefaultsWithDefaults returns the given spec.TypesDefaults with a fallback spec.TypesDefaults
// applied. The original spec.TypesDefaults takes precedence.
func typesDefaultsWithDefaults(td spec.TypesDefaults, defaults spec.TypesDefaults) spec.TypesDefaults {
	return spec.TypesDefaults{
		Enum:          enumDefaultsWithDefaults(td.Enum, defaults.Enum),
		Mappers:       mappersWithDefaults(td.Mappers, defaults.Mappers),
		Bidirectional: cmp.Or(td.Bidirectional, defaults.Bidirectional),
	}
}

// typesDefaultsWithPreset returns the given spec.TypesDefaults with a spec.Preset applied. The
// preset takes precedence.
func typesDefaultsWithPreset(td spec.TypesDefaults, preset spec.Preset) spec.TypesDefaults {
	return spec.TypesDefaults{
		Enum:          enumDefaultsWithDefaults(preset.Enum, td.Enum),
		Mappers:       mappersWithDefaults(preset.Mappers, td.Mappers),
		Bidirectional: cmp.Or(preset.Bidirectional, td.Bidirectional),
	}
}

// typesDefaultsWithPackageSpec applies the overrides specified at a package level to the given
// spec.TypesDefaults. The package-level configuration will take precedence.
func typesDefaultsWithPackageSpec(td spec.TypesDefaults, pkg spec.Package) spec.TypesDefaults {
	return spec.TypesDefaults{
		Enum:          td.Enum,
		Mappers:       td.Mappers,
		Bidirectional: cmp.Or(pkg.Bidirectional, td.Bidirectional),
	}
}

func specTypeWithTypesDefaults(typ spec.Type, defaults spec.TypesDefaults) spec.Type {
	typ.Enum = enumWithDefaults(typ.Enum, defaults.Enum)
	typ.Mappers = mappersWithDefaults(typ.Mappers, defaults.Mappers)
	typ.Bidirectional = cmp.Or(typ.Bidirectional, defaults.Bidirectional)
	return typ
}
