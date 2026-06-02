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
	Defaults  spec.Defaults  `json:"defaults"`
	Discovery spec.Discovery `json:"discovery"`
	Packages  []spec.Package `json:"packages"`
	Presets   []spec.Preset  `json:"presets"`
}

// OutputFile is an in-memory representation of a generated file, ready to be written, detailing
// where the file should be written to, and its contents.
type OutputFile struct {
	LogicalPath string
	PackageName string // Used for debugging
	Source      []byte
}

// Workspace is a representation of the filesystem state that Morph is operating within. This is
// used to resolve file paths for things like output locations.
type Workspace struct {
	// ModuleDir is the logical filesystem location of the current main module
	ModuleDir string
	// ModulePath is the root "import path" of the current main module
	ModulePath string
	// WorkingDir is the current "working directory" of Morph, this is the directory Morph is told
	// it is running in, and is not the same as the process's working directory.
	WorkingDir string
}
