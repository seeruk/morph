package plan

import (
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
)

// Diagnostic is a generalized type used for presenting helpful messages to Morph consumers to help
// them find and fix issues found during planning.
type Diagnostic struct {
	Path    string
	Message string
}

// OutputGroup contains information about one logical file to generate.
type OutputGroup struct {
	Location OutputLocation
	// Roots is a list of the plans for explicitly requested types found within this OutputGroup.
	Roots []*Type
}

// OutputLocation describes where to generate one logical output file, and how file-level
// information should be generated for that file.
type OutputLocation struct {
	LogicalPath string
	ImportPath  string
	PackageName string
}

// Type represents the output of the planning process for a type pair.
type Type struct {
	// Type information
	Source     TypeRef
	Target     TypeRef
	SourceDecl types.TypeDecl
	TargetDecl types.TypeDecl
	SourceType types.Type
	TargetType types.Type
	// Function information
	FunctionName string
	TypeParams   []types.TypeParam
	Signature    MapperSignature
	CanError     bool
	// Plan
	Enum       *EnumPlan
	StructPlan *StructPlan
	// Debugging information
	Diagnostics []Diagnostic
}

type EnumPlan struct {
	FailureMode spec.EnumFailureMode
	Values      []EnumValuePlan
}

// EnumValuePlan describes the plan for mapping a single enum value.
type EnumValuePlan struct {
	Source types.ConstantDecl
	Target types.ConstantDecl
}

type StructPlan struct {
	Fields []FieldPlan
}

type FieldPlan struct {
	SourceField types.Field
	TargetField types.Field
	Mapping     ValuePlan
}

type ValuePlan struct {
	// TODO: Fill in...
}

// MapperSignature represents the planned signature of a mapping function, it differs from the type
// found in the spec package in that values here are explicit and always defined (i.e. not nil).
type MapperSignature struct {
	Accepts spec.ParameterKind
	Returns spec.ParameterKind
}

type TypeRef struct {
	ImportPath string
	Name       string
	Key        string
}
