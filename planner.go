package morph

import (
	"github.com/seeruk/morph/plan"
)

type Planner struct {
	types map[string]*plan.Type
}

func NewPlanner() *Planner {
	return &Planner{}
}

func (planner *Planner) Plan(spec Spec) (Plan, error) {
	return Plan{}, nil
}
