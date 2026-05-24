package plan

import (
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

// ValuePlan describes how to map one value to another. Compound mappings point to child mappings
// for elements, keys, or values (i.e. for nested types).
type ValuePlan struct {
	Kind          OperationKind
	Source        types.Type
	Target        types.Type
	Callable      *CallableRef
	Plan          *Type
	Elem          *ValuePlan
	Key           *ValuePlan
	Value         *ValuePlan
	SourcePointer bool
	TargetPointer bool
	CanError      bool
	Diagnostics   []Diagnostic
}

// OperationKind describes the operation used for a mapping node.
type OperationKind string

const (
	OperationUnsupported OperationKind = "unsupported"
	OperationAssign      OperationKind = "assign"
	OperationConversion  OperationKind = "conversion"
	OperationMethod      OperationKind = "method"
	OperationConvert     OperationKind = "convert"
	OperationStruct      OperationKind = "struct"
	OperationEnum        OperationKind = "enum"
	OperationPointer     OperationKind = "pointer"
	OperationSlice       OperationKind = "slice"
	OperationArray       OperationKind = "array"
	OperationMap         OperationKind = "map"
)

// MapperSignature represents the planned signature of a mapping function, it differs from the type
// found in the spec package in that values here are explicit and always defined (i.e. not nil).
type MapperSignature struct {
	Accepts spec.ParameterKind
	Returns spec.ParameterKind
}

// Diagnostic is a generalized type used for presenting helpful messages to Morph consumers to help
// them find and fix issues found during planning.
type Diagnostic struct {
	Path    string
	Message string
}
