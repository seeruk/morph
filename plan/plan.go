package plan

import (
	"fmt"

	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
)

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
	Signature    spec.MapperSignature
	CanError     bool
	// Spec
	EnumSpec   spec.Enum
	StructSpec spec.Struct
	// Plan
	Enum       *Enum
	StructPlan *Struct
	// Debugging information
	Diagnostics []Diagnostic
}

type Enum struct {
	FailureMode spec.EnumFailureMode
	Values      []EnumValue
}

// EnumValue describes the plan for mapping a single enum value.
type EnumValue struct {
	Source types.ConstantDecl
	Target types.ConstantDecl
}

type Struct struct {
	Fields []Field
}

type Field struct {
	SourceField types.Field
	TargetField types.Field
	Mapping     Value
}

// Value describes how to map one value to another. Compound mappings point to child mappings for
// elements, keys, or values (i.e. for nested types).
type Value struct {
	Operation     Operation
	Source        types.Type
	Target        types.Type
	Callable      *CallableRef
	CallableArgs  []CallableArg
	Plan          *Type
	Elem          *Value
	Key           *Value
	Value         *Value
	SourcePointer bool
	TargetPointer bool
	CanError      bool
	Diagnostics   []Diagnostic
}

// CallableArg describes an argument passed alongside a source value when invoking a callable.
type CallableArg struct {
	Mapping      Value
	ReturnsError bool
}

// Operation describes the operation used for a mapping node.
type Operation string

const (
	OperationUnsupported Operation = "unsupported"
	OperationAssign      Operation = "assign"
	OperationFunction    Operation = "function"
	OperationMethod      Operation = "method"
	OperationConvert     Operation = "convert"
	OperationStruct      Operation = "struct"
	OperationEnum        Operation = "enum"
	OperationPointer     Operation = "pointer"
	OperationSlice       Operation = "slice"
	OperationArray       Operation = "array"
	OperationMap         Operation = "map"
)

// Diagnostic is a generalized type used for presenting helpful messages to Morph consumers to help
// them find and fix issues found during planning.
type Diagnostic struct {
	Level   DiagnosticLevel
	Path    string
	Message string
}

// String returns this Diagnostic as a string.
func (d Diagnostic) String() string {
	if d.Path == "" {
		return fmt.Sprintf("%s: %s", d.Level, d.Message)
	}
	return fmt.Sprintf("%s: %s: %s", d.Level, d.Path, d.Message)
}

// DiagnosticLevel enumerates the possible levels of diagnostics, which can be used to determine
// whether a plan failed.
type DiagnosticLevel uint

const (
	DiagnosticLevelFatal DiagnosticLevel = iota
	DiagnosticLevelWarning
	diagnosticLevelMax
)

var diagnosticLevelNames = map[DiagnosticLevel]string{
	DiagnosticLevelFatal:   "fatal",
	DiagnosticLevelWarning: "warning",
}

// String returns this DiagnosticLevel as a string.
func (d DiagnosticLevel) String() string {
	if s, ok := diagnosticLevelNames[d]; ok {
		return s
	}
	return "unknown"
}
