package plan

import "github.com/seeruk/morph/types"

// TypesPath returns a stable diagnostic path for a source/target type pair.
func TypesPath(source, target types.Type) string {
	return types.TypeKey(source) + "->" + types.TypeKey(target)
}

// PropertyPath returns a stable diagnostic path for a source/target property mapping.
func PropertyPath(source, target types.Type, sourceProperty, targetProperty string) string {
	return TypesPath(source, target) + " :: " + sourceProperty + "->" + targetProperty
}

// SourcePropertyPath returns a stable diagnostic path for a source property within a type pair.
func SourcePropertyPath(source, target types.Type, sourceProperty string) string {
	return TypesPath(source, target) + " :: source property " + sourceProperty
}

// TargetPropertyPath returns a stable diagnostic path for a target property within a type pair.
func TargetPropertyPath(source, target types.Type, targetProperty string) string {
	return TypesPath(source, target) + " :: target property " + targetProperty
}

// SourceEnumValuePath returns a stable diagnostic path for a source enum value within a type pair.
func SourceEnumValuePath(source, target types.Type, value string) string {
	return TypesPath(source, target) + " :: source enum value " + value
}

// TargetEnumValuePath returns a stable diagnostic path for a target enum value within a type pair.
func TargetEnumValuePath(source, target types.Type, value string) string {
	return TypesPath(source, target) + " :: target enum value " + value
}
