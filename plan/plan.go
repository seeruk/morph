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
	// Nested is a list of generated helper mappers emitted within this OutputGroup.
	Nested []*Type
}

// OutputLocation describes where to generate one logical output file, and how file-level
// information should be generated for that file.
type OutputLocation struct {
	LogicalPath string
	ImportPath  string
	PackageName string
}

// Type represents the output of the planning process for a type pair. Morph plans type aliases as
// their underlying types; generators should render underlying types rather than alias names.
type Type struct {
	// Type information
	Source     TypeRef
	Target     TypeRef
	SourceDecl types.TypeDecl
	TargetDecl types.TypeDecl
	SourceType types.Type
	TargetType types.Type
	// Function information
	FunctionName   string
	MapperKindSpec spec.MapperKind
	MapperKind     MapperKind
	// Location is the output location that owns this generated mapper.
	Location   OutputLocation
	TypeParams []types.TypeParam
	Signature  spec.MapperSignature
	CanError   bool
	// Spec
	EnumSpec    spec.Enum
	Callables   []spec.PrioritizedCallables
	StructSpec  spec.Struct
	Optionality spec.Optionality
	Conversions spec.ConversionsPolicy
	// Plan
	EnumPlan   *Enum
	StructPlan *Struct
	// Debugging information
	Diagnostics []Diagnostic
}

// MapperKind describes the concrete declaration kind chosen by the planner.
type MapperKind string

const (
	MapperKindFunction MapperKind = "function"
	MapperKindMethod   MapperKind = "method"
)

type Enum struct {
	FailureMode   spec.EnumFailureMode
	FallbackValue types.ConstantDecl
	Values        []EnumValue
}

// EnumValue describes the plan for mapping a single enum value.
type EnumValue struct {
	Source types.ConstantDecl
	Target types.ConstantDecl
}

type Struct struct {
	Properties []Property
}

type Property struct {
	Source  Member
	Target  Member
	Mapping Value
}

// Member describes a readable or writable struct property member used by a mapping.
type Member struct {
	Name     string
	Accessor string
	Kind     MemberKind
	Type     types.Type
	CanError bool
}

// MemberKind describes how a property member is accessed in generated code.
type MemberKind string

const (
	MemberKindField  MemberKind = "field"
	MemberKindMethod MemberKind = "method"
)

// Value describes how to map one value to another. Compound mappings point to child mappings for
// elements, keys, or values (i.e. for nested types). Generators should apply SourceAdaptations to
// the source expression in order, emit Operation, then apply TargetAdaptations to the operation
// result in order. Empty adaptation slices mean no adaptation. For generated mapper calls, Plan
// holds diagnostics for the referenced mapper; Value diagnostics describe only the call site.
type Value struct {
	Operation           Operation
	Source              types.Type
	Target              types.Type
	Callable            *CallableRef
	CallableContextArgs []CallableContextArg
	CallableMapperArgs  []CallableMapperArg
	SourceAdaptations   []ValueAdaptation
	TargetAdaptations   []ValueAdaptation
	Plan                *Type
	Elem                *Value
	Key                 *Value
	Value               *Value
	Optionality         spec.Optionality
	CanError            bool
	Diagnostics         []Diagnostic
}

// CallableContextArg describes an exact source field or zero-arg method passed alongside the primary
// source value when invoking a callable.
type CallableContextArg struct {
	Source Member
}

// CallableMapperArg describes a mapper function argument for a higher-order callable.
type CallableMapperArg struct {
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
	OperationSlice       Operation = "slice"
	OperationArray       Operation = "array"
	OperationMap         Operation = "map"
)

// ValueAdaptation describes how Morph should adapt a value across a pointer/value boundary.
type ValueAdaptation string

const (
	ValueAdaptationAddress ValueAdaptation = "address"
	ValueAdaptationDeref   ValueAdaptation = "deref"
)

// Diagnostic is a generalized type used for presenting helpful messages to Morph consumers to help
// them find and fix issues found during planning.
type Diagnostic struct {
	Level   DiagnosticLevel
	Path    string
	Message string
}

// HasFatalDiagnostics returns whether the supplied diagnostics contain at least one fatal
// diagnostic.
func HasFatalDiagnostics(diagnostics []Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == DiagnosticLevelFatal {
			return true
		}
	}
	return false
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
