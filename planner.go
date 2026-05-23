package morph

type Planner struct{}

func NewPlanner() *Planner {
	return &Planner{}
}

func (planner *Planner) Plan(spec Spec) (Plan, error) {
	return Plan{}, nil
}
