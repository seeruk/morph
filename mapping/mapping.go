package mapping

import "github.com/seeruk/morph/types"

// PlanInput is the input to the planning process, providing context to the planner around which
// type pairs to plan for, and options for customizing the plan, for example, things like naming of
// functions Morph will emit.
type PlanInput struct{}

// Plan is the output of the planning process, providing an abstract representation of what to
// generate and how. The plan can be particularly useful for consumers of Morph as a library, where
// it can feed into a multi-stage generation process, detailing what will be made available, and
// how, from Morph.
type Plan struct{}

// TypePairConfig represents the mapping configuration for a specific pair of types.
type TypePairConfig struct {
	Source types.TypeDecl
	Target types.TypeDecl
	// Configuration for how to map type pairs supported by Morph:
	Enum   *EnumConfig
	Struct *StructConfig
	// Configuration for customizing what is produced by Morph:
	Mapper *MapperConfig

	// TODO: Evaluate potential for OutputConfig:
	//  - It doesn't make sense necessarily to allow customisation of file, package name, or import
	//    path at this level, because Morph will either place all code in one package, or it'll be
	//    placed near one end of the source or target (usually target, I think).
}

type EnumConfig struct {
	// Values is an explicit mapping from source enum value name to target enum value name. Only
	// explicit mappings need be placed in this map, as the planner will attempt to infer mappings
	// for values with similar names.
	Values map[string]string

	// FailureMode specifies explicit behaviour for this enum for what the generated mapping
	// function should do when an invalid or unknown value is encountered, i.e. should the mapping
	// function return an error as a second return value, or should it return the zero value and no
	// error?
	FailureMode EnumFailureMode
}

// EnumFailureMode enumerates the possible failure modes for an enum mapping function.
type EnumFailureMode uint

const (
	EnumFailureModeError EnumFailureMode = iota
	EnumFailureModeZero
	enumFailureModeMax
)

type StructConfig struct {
	// FieldMappings is a map from source field name to target field name. Only explicit mappings
	// need be placed in this map, as the planner will attempt to infer mappings for fields with
	// similar names.
	FieldMappings map[string]string
}

// MapperConfig is configuration for how the mapper function should be generated, allowing
// customization of things like naming, and the signature of the function.
type MapperConfig struct {
	// Name can be used to provide an exact name, but also supports pattern-based naming using
	// predefined placeholders. Available placeholders are:
	// TODO: Define and list placeholders...
	Name string
	// Some types prefer to be passed around as either pointers or values, for example, ProtoBuf
	// generated types contain mutexes for their own metadata, and as such are typically handled
	// using pointers.
	SourceAsPointer bool
	TargetAsPointer bool
}
