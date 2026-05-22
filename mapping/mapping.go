package mapping

// PlanInput is the input to the planning process, providing context to the planner around which
// type pairs to plan for, and options for customizing the plan, for example, things like naming of
// functions Morph will emit.
type PlanInput struct{}

// Plan is the output of the planning process, providing an abstract representation of what to
// generate and how. The plan can be particularly useful for consumers of Morph as a library, where
// it can feed into a multi-stage generation process, detailing what will be made available, and
// how, from Morph.
type Plan struct{}
