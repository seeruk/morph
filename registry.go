package morph

import (
	"fmt"
	"maps"
	"slices"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
)

// functionRegistry is a type used to keep track of, and help find functions that might be used in
// Morph mappings. The registry is most useful for finding functions from different sources, as they
// are likely to have different priorities when planning how they'll be used.
type functionRegistry struct {
	functions map[functionKey][]types.FunctionDecl
}

// newFunctionRegistry returns a new functionRegistry instance.
func newFunctionRegistry() *functionRegistry {
	return &functionRegistry{
		functions: make(map[functionKey][]types.FunctionDecl),
	}
}

// Candidates returns functions that may be compatible with the given types, from the given source.
func (r *functionRegistry) Candidates(
	sourceType, targetType types.Type,
	source plan.CallableSource,
) []types.FunctionDecl {
	var out []types.FunctionDecl
	for _, key := range createFunctionKeys(sourceType, targetType, source) {
		out = append(out, r.functions[key]...)
	}
	return out
}

// Register registers the given types.FunctionDecl from the given source into this registry.
func (r *functionRegistry) Register(fn types.FunctionDecl, source plan.CallableSource) bool {
	_, ok := plan.CallableRefFromFunctionDecl(fn, source)
	if !ok {
		return false // Not compatible
	}

	key, ok := createFunctionKey(fn.Params[0].Type, fn.Results[0].Type, source)
	if !ok {
		return false
	}

	if r.functions == nil {
		r.functions = make(map[functionKey][]types.FunctionDecl)
	}
	r.functions[key] = append(r.functions[key], fn)
	return true
}

// functionKey is a type used as a key for looking up functions in the functionRegistry. It's
// similar to a plan.TypeRef, but critically does not contain a types.TypeKey, as that would
// separate types with different generic arguments.
type functionKey struct {
	SourceKey string
	TargetKey string
	Source    plan.CallableSource
}

// createFunctionKeys returns a slice of all possible function keys that could be used for the given
// source and target types, and callable source.
func createFunctionKeys(sourceType, targetType types.Type, source plan.CallableSource) []functionKey {
	distinct := make(map[functionKey]struct{}, 2)

	add := func(sourceType types.Type) {
		key, ok := createFunctionKey(sourceType, targetType, source)
		if !ok {
			return
		}
		distinct[key] = struct{}{}
	}

	add(sourceType)

	sourceElem, sourcePointer := types.PointerElem(sourceType)
	if sourcePointer {
		add(sourceElem)
	} else {
		add(types.PointerTo(sourceType))
	}

	return slices.Collect(maps.Keys(distinct))
}

// createFunctionKey returns a functionKey for the given source and target types, and callable
// source.
func createFunctionKey(sourceType, targetType types.Type, source plan.CallableSource) (functionKey, bool) {
	sourceKey, sourceOK := functionTypeKey(sourceType)
	targetKey, targetOK := functionTypeKey(targetType)
	if !sourceOK || !targetOK {
		return functionKey{}, false
	}

	return functionKey{
		SourceKey: sourceKey,
		TargetKey: targetKey,
		Source:    source,
	}, true
}

// functionTypeKey creates a string key for the given type which contains just enough information
// to correctly key functions in the registry to make them discoverable later when we're assessing
// candidate functions returned by the registry. For example, a generic `Optional[T]` type should
// not be returned with the type parameter information in for this purpose.
func functionTypeKey(typ types.Type) (string, bool) {
	typ = types.UnwrapAlias(typ)

	switch typ.Kind {
	case types.TypeKindBasic:
		return typ.Name, true
	case types.TypeKindNamed:
		if typ.Package.ImportPath != "" {
			return typ.Package.ImportPath + "." + typ.Name, true
		}
		return typ.Name, typ.Name != ""
	case types.TypeKindPointer:
		if typ.Elem == nil {
			return "", false
		}
		elem, ok := functionTypeKey(*typ.Elem)
		return "*" + elem, ok
	case types.TypeKindSlice:
		if typ.Elem == nil {
			return "", false
		}
		elem, ok := functionTypeKey(*typ.Elem)
		return "[]" + elem, ok
	case types.TypeKindArray:
		if typ.Elem == nil {
			return "", false
		}
		elem, ok := functionTypeKey(*typ.Elem)
		return fmt.Sprintf("[%d]%s", typ.Len, elem), ok
	case types.TypeKindMap:
		if typ.Key == nil || typ.Value == nil {
			return "", false
		}
		key, keyOK := functionTypeKey(*typ.Key)
		value, valueOK := functionTypeKey(*typ.Value)
		return fmt.Sprintf("map[%s]%s", key, value), keyOK && valueOK
	case types.TypeKindTypeParam:
		return "", false
	default:
		if typ.String != "" {
			return typ.String, true
		}
		return typ.Name, typ.Name != ""
	}
}
