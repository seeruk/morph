package morph

import "github.com/seeruk/morph/spec"

// outputWithDefaults returns the given output with defaults applied, if necessary.
func outputWithDefaults(output spec.Output, defaults spec.Output) spec.Output {
	return spec.Output{
		Strategy: orDefault(output.Strategy, defaults.Strategy),
		Path:     orDefault(output.Path, defaults.Path),
		Package:  orDefault(output.Package, defaults.Package),
		Filename: orDefault(output.Filename, defaults.Filename),
	}
}

// orDefault either returns the given first value, or if that's zero, falls back to the provided
// default value.
func orDefault[T comparable](v T, defaultValue T) T {
	var zero T
	if v == zero {
		return defaultValue
	}
	return v
}
