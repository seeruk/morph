package plan

import "github.com/seeruk/morph/types"

// TypesPath returns a stable diagnostic path for a source/target type pair.
func TypesPath(source, target types.Type) string {
	return types.TypeKey(source) + "->" + types.TypeKey(target)
}

// FieldPath returns a stable diagnostic path for a source/target field mapping.
func FieldPath(source, target types.Type, sourceField, targetField types.Field) string {
	return TypesPath(source, target) + " :: " + sourceField.Name + "->" + targetField.Name
}

// SourceFieldPath returns a stable diagnostic path for a source field within a type pair.
func SourceFieldPath(source, target types.Type, sourceField string) string {
	return TypesPath(source, target) + " :: source field " + sourceField
}

// TargetFieldPath returns a stable diagnostic path for a target field within a type pair.
func TargetFieldPath(source, target types.Type, targetField string) string {
	return TypesPath(source, target) + " :: target field " + targetField
}

// SourceEnumValuePath returns a stable diagnostic path for a source enum value within a type pair.
func SourceEnumValuePath(source, target types.Type, value string) string {
	return TypesPath(source, target) + " :: source enum value " + value
}

// TargetEnumValuePath returns a stable diagnostic path for a target enum value within a type pair.
func TargetEnumValuePath(source, target types.Type, value string) string {
	return TypesPath(source, target) + " :: target enum value " + value
}
