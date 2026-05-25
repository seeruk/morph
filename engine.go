package morph

// Engine acts as the library entrypoint to Morph, exposing functionality found in Morph's
// sub-packages from a convenient location.
type Engine struct{}

func New() *Engine {
	return &Engine{}
}

func (e *Engine) Plan(spec Spec) (Plan, error) {
	return NewPlanner(spec).Plan()
}
