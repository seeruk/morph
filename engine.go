package morph

// Engine acts as the library entrypoint to Morph, exposing functionality found in Morph's
// sub-packages from a convenient location.
type Engine struct {
	planner *Planner
}

func New() *Engine {
	return &Engine{
		planner: NewPlanner(),
	}
}

func (e *Engine) Plan(spec Spec) (Plan, error) {
	return e.planner.Plan(spec)
}
