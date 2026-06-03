package morph

import "github.com/seeruk/morph/spec"

const defaultNestedMapperName = "mapNested{{ .Source.Package }}{{ .Source.Type }}To{{ .Target.Package }}{{ .Target.Type }}_{{ .RunHash }}"

var defaultMapperSignature = spec.MapperSignature{
	Accepts: spec.ParameterKindValue,
	Returns: spec.ParameterKindValue,
}

func defaultOptionality() spec.Optionality {
	return spec.Optionality{
		OnNilSourcePointer: spec.PointerOptionalityZero,
		OnZeroSourceValue:  spec.ValueOptionalityNil,
	}
}
