package plan

import "github.com/seeruk/morph/types"

// CallableRef is a structured reference to a callable used by a mapping.
type CallableRef struct {
	SourceType   TypeRef
	TargetType   TypeRef
	Kind         CallableKind
	Source       CallableSource
	Package      types.PackageRef
	Name         string
	ReturnsError bool
}

// CallableKind describes how a callable is invoked.
type CallableKind string

const (
	CallableKindFunction CallableKind = "function"
	CallableKindMethod   CallableKind = "method"
)

// CallableSource describes where a callable came from, this is used to determine the priority that
// callables have (e.g. explicit user-provided conversions should be preferred over discovered
// conversions).
type CallableSource int

const (
	CallableSourceUser CallableSource = iota
	CallableSourceDiscovered
)

// TypeRef is a structured reference to a particular Go type.
type TypeRef struct {
	ImportPath string
	Name       string
	Key        string
}

// TypeRefFromType returns a TypeRef for the given types.Type.
func TypeRefFromType(t types.Type) TypeRef {
	return TypeRef{
		Name:       t.Name,
		ImportPath: t.Package.ImportPath,
		Key:        TypeKey(t),
	}
}

// TypeRefFromTypeDecl returns a TypeRef for the given types.TypeDecl.
func TypeRefFromTypeDecl(t types.TypeDecl) TypeRef {
	return TypeRef{
		Name:       t.Name,
		ImportPath: t.Package.ImportPath,
		Key:        TypeKey(t.Type),
	}
}
