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
	sourceKey := types.TypeKey(source)
	targetKey := types.TypeKey(target)

	// TODO: Big changes here for function discovery...

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
	callable, ok := plan.CallableRefFromFunctionDecl(fn, source)
	if ok {
		r.Register(callable)
	}
	return ok
}
