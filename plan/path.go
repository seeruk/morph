package plan

import "github.com/seeruk/morph/types"

// TypesPath ...
func TypesPath(source, target types.Type) string {
	return types.TypeKey(source) + "->" + types.TypeKey(target)
}

// FieldPath ...
func FieldPath(source, target types.Type, sourceField types.Field) string {
	return TypesPath(source, target) + " :: " + sourceField.Name
}
