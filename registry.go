package morph

import (
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
)

// callableRegistry is a type used to keep track of, and help find callables that might be used in
// Morph mappings. The registry is most useful for finding callables from different sources, as they
// are likely to have different priorities when planning how they'll be used.
type callableRegistry struct {
	callables []plan.CallableRef
}

// newCallableRegistry returns a new callableRegistry instance.
func newCallableRegistry() *callableRegistry {
	return &callableRegistry{}
}

// Find attempts to find a matching callable for the given type
func (r *callableRegistry) Find(source, target types.Type, sources ...plan.CallableSource) (plan.CallableRef, bool) {
	sourceKey := plan.TypeKey(source)
	targetKey := plan.TypeKey(target)

	// TODO: There could be multiple potential options. Maybe they could be prioritised based on
	//  how well the signature matches what's needed, e.g. pointer / value parameters or results
	for _, source := range sources {
		for _, callable := range r.callables {
			if callable.Source != source {
				continue
			}
			if callable.SourceType.Key == sourceKey && callable.TargetType.Key == targetKey {
				return callable, true
			}
		}
	}

	return plan.CallableRef{}, false
}

// Register registers the given plan.CallableRef into this registry.
func (r *callableRegistry) Register(callable plan.CallableRef) {
	r.callables = append(r.callables, callable)
}

// RegisterFunction registers the given types.FunctionDecl from the given source into this registry.
func (r *callableRegistry) RegisterFunction(fn types.FunctionDecl, source plan.CallableSource) bool {
	callable, ok := callableFromFunctionDecl(fn, source)
	if ok {
		r.Register(callable)
	}
	return ok
}

// RegisterMethod registers the given types.Method from the given source into this registry.
func (r *callableRegistry) RegisterMethod(method types.Method, source plan.CallableSource) bool {
	callable, ok := callableFromMethod(method, source)
	if ok {
		r.Register(callable)
	}
	return ok
}

func callableFromFunctionDecl(fn types.FunctionDecl, source plan.CallableSource) (plan.CallableRef, bool) {
	if fn.IsVariadic || len(fn.TypeParams) > 0 || len(fn.Params) != 1 {
		// TODO: Revisit type params for using generic callables
		return plan.CallableRef{}, false
	}

	returnsError, ok := callableResults(fn.Results)
	if !ok {
		return plan.CallableRef{}, false
	}

	return plan.CallableRef{
		SourceType:   plan.TypeRefFromType(fn.Params[0].Type),
		TargetType:   plan.TypeRefFromType(fn.Results[0].Type),
		Kind:         plan.CallableKindFunction,
		Source:       source,
		Package:      fn.Package,
		Name:         fn.Name,
		ReturnsError: returnsError,
	}, true
}

func callableFromMethod(method types.Method, source plan.CallableSource) (plan.CallableRef, bool) {
	if method.Receiver == nil || method.IsVariadic || len(method.TypeParams) > 0 || len(method.Params) != 0 {
		// TODO: Revisit type params for using generic callables
		return plan.CallableRef{}, false
	}

	owner, ok := method.OwnerType()
	if !ok {
		return plan.CallableRef{}, false
	}

	returnsError, ok := callableResults(method.Results)
	if !ok {
		return plan.CallableRef{}, false
	}

	return plan.CallableRef{
		SourceType:   plan.TypeRefFromType(method.Receiver.Type),
		TargetType:   plan.TypeRefFromType(method.Results[0].Type),
		Kind:         plan.CallableKindMethod,
		Source:       source,
		Package:      owner.Package,
		Name:         method.Name,
		ReturnsError: returnsError,
	}, true
}

// callableResults returns whether a callable returns an error, and whether it's valid.
// Valid callables' signatures must return either 1 or 2 results, and if 2, the second must be an
// error.
func callableResults(results []types.Parameter) (returnsError bool, ok bool) {
	switch len(results) {
	case 1:
		return false, true
	case 2:
		if isErrorType(results[1].Type) {
			return true, true
		}
	}
	return false, false
}

// isErrorType checks if the given type is specifically the standard library built-in named error
// type. It does not support custom error types or aliases. Generally these are never used as return
// values, and it can be problematic to do so, so we don't currently check for them.
func isErrorType(info types.Type) bool {
	return info.Name == "error" && info.Package.ImportPath == ""
}
