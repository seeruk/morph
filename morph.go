package morph

import (
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
)

// Plan is the output of the planning process, providing an abstract representation of what to
// generate and how. The plan can be particularly useful for consumers of Morph as a library, where
// it can feed into a multi-stage generation process, detailing what will be made available, and
// how, from Morph.
type Plan struct {
	OutputGroups []plan.OutputGroup
	Diagnostics  []plan.Diagnostic
}

// Spec is a description of what Morph should do, and how it should do it.
type Spec struct {
	Defaults    spec.Defaults      `json:"defaults"`
	Discovery   spec.Discovery     `json:"discovery"`
	Conversions []spec.CallableRef `json:"conversions"`
	Packages    []spec.Package     `json:"packages"`
	Presets     []spec.Preset      `json:"presets"`
}
