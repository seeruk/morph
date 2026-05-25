package morph

// Engine acts as the library entrypoint to Morph, exposing functionality found in Morph's
// sub-packages from a convenient location.
type Engine struct {
	workingDir string
}

// New creates a new Morph Engine with the supplied working directory. The working directory is not
// the process's working directory, it's the directory that Morph is operating within. For CLI use,
// this is usually the location of the configuration file.
func New(workingDir string) *Engine {
	return &Engine{
		workingDir: workingDir,
	}
}

// Plan generates a Plan for the supplied Spec using a new Planner.
func (e *Engine) Plan(spec Spec) (Plan, error) {
	return NewPlanner(spec, e.workingDir).Plan()
}
