package morph

import "github.com/seeruk/morph/spec"

// Plan is the output of the planning process, providing an abstract representation of what to
// generate and how. The plan can be particularly useful for consumers of Morph as a library, where
// it can feed into a multi-stage generation process, detailing what will be made available, and
// how, from Morph.
type Plan struct{}

// Spec is a description of what Morph should do, and how it should do it.
type Spec struct {
	Presets []spec.Presets `json:"presets"`
}
