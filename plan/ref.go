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

// CallableRefFromFunctionDecl returns a CallableRef for the given function declaration, from the
// given source. Additional parameters may be accepted by higher-level planner compatibility checks,
// but the first parameter is always considered the source value for registry lookups.
func CallableRefFromFunctionDecl(fn types.FunctionDecl, source CallableSource) (CallableRef, bool) {
	if fn.IsVariadic || len(fn.Params) == 0 {
		return CallableRef{}, false
	}

	returnsError, ok := CallableResults(fn.Results)
	if !ok {
		return CallableRef{}, false
	}

	return CallableRef{
		SourceType:   TypeRefFromType(fn.Params[0].Type),
		TargetType:   TypeRefFromType(fn.Results[0].Type),
		Kind:         CallableKindFunction,
		Source:       source,
		Package:      fn.Package,
		Name:         fn.Name,
		ReturnsError: returnsError,
	}, true
}

// CallableRefFromMethod returns a CallableRef for the given method, from the given source.
func CallableRefFromMethod(method types.Method, source CallableSource) (CallableRef, bool) {
	if method.Receiver == nil || method.IsVariadic || len(method.Params) != 0 {
		return CallableRef{}, false
	}

	owner, ok := method.OwnerType()
	if !ok {
		return CallableRef{}, false
	}

	returnsError, ok := CallableResults(method.Results)
	if !ok {
		return CallableRef{}, false
	}

	return CallableRef{
		SourceType:   TypeRefFromType(method.Receiver.Type),
		TargetType:   TypeRefFromType(method.Results[0].Type),
		Kind:         CallableKindMethod,
		Source:       source,
		Package:      owner.Package,
		Name:         method.Name,
		ReturnsError: returnsError,
	}, true
}

// CallableResults returns whether a callable returns an error, and whether it's valid.
// Valid callables' signatures must return either 1 or 2 results, and if 2, the second must be an
// error.
func CallableResults(results []types.Parameter) (returnsError bool, ok bool) {
	switch len(results) {
	case 1:
		return false, true
	case 2:
		if isErrorType(results[1].Type) {
			return true, true
		}
	}
	return false, false
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
		Key:        types.TypeKey(t),
	}
}

// TypeRefFromTypeDecl returns a TypeRef for the given types.TypeDecl.
func TypeRefFromTypeDecl(t types.TypeDecl) TypeRef {
	return TypeRef{
		Name:       t.Name,
		ImportPath: t.Package.ImportPath,
		Key:        types.TypeKey(t.Type),
	}
}

// isErrorType checks if the given type is specifically the standard library built-in named error
// type. It does not support custom error types or aliases. Generally these are never used as return
// values, and it can be problematic to do so, so we don't currently check for them.
func isErrorType(info types.Type) bool {
	return info.Name == "error" && info.Package.ImportPath == ""
}
