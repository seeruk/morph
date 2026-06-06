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
func (e *Engine) Plan(spec Spec, ident string) (Plan, error) {
	return NewPlanner(spec, e.workingDir, ident).Plan()
}

// GeneratePlan generates output files for the supplied Plan.
func (e *Engine) GeneratePlan(plan Plan) ([]OutputFile, error) {
	files, err := NewGenerator().Generate(plan)
	if err != nil {
		return nil, err
	}

	return files, nil
}

// Generate generates a Plan for the supplied Spec, then generates output for that Plan.
func (e *Engine) Generate(spec Spec, ident string) ([]OutputFile, Plan, error) {
	plan, err := e.Plan(spec, ident)
	if err != nil {
		return nil, plan, err
	}
	if plan.HasFatalDiagnostics() {
		return nil, plan, nil
	}

	files, err := e.GeneratePlan(plan)
	if err != nil {
		return nil, plan, err
	}

	return files, plan, nil
}
