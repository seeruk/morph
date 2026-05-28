package spec

import (
	"fmt"
	"strings"

	"github.com/seeruk/morph/types"
)

// CallableRef is a structured representation of a reference to a particular callable, i.e. a
// function or method.
type CallableRef struct {
	ImportPath string
	TypeName   string
	Name       string
}

// CallableRefFromFunctionDecl returns a CallableRef for a given types.FunctionDecl.
func CallableRefFromFunctionDecl(fn types.FunctionDecl) CallableRef {
	return CallableRef{
		ImportPath: fn.Package.ImportPath,
		Name:       fn.Name,
	}
}

// CallableRefFromMethod returns a CallableRef for a given types.Method.
func CallableRefFromMethod(method types.Method) (CallableRef, bool) {
	owner, ok := method.OwnerType()
	if !ok {
		return CallableRef{}, false
	}

	return CallableRef{
		ImportPath: owner.Package.ImportPath,
		TypeName:   owner.Name,
		Name:       method.Name,
	}, true
}

func (r *CallableRef) UnmarshalText(text []byte) error {
	value := string(text)
	if value == "" {
		return nil
	}

	lastSlash := strings.LastIndex(value, "/")
	lastDot := strings.LastIndex(value, ".")
	if lastDot <= lastSlash+1 || lastDot == len(value)-1 {
		return fmt.Errorf("invalid callable reference %q", value)
	}

	prefix := value[:lastDot]
	name := value[lastDot+1:]
	typeDot := strings.LastIndex(prefix[lastSlash+1:], ".")

	if typeDot == -1 {
		r.ImportPath = prefix
		r.TypeName = ""
		r.Name = name
		return nil
	}

	typeDot += lastSlash + 1
	if typeDot <= lastSlash+1 || typeDot == len(prefix)-1 {
		return fmt.Errorf("invalid callable reference %q", value)
	}

	r.ImportPath = value[:typeDot]
	r.TypeName = value[typeDot+1 : lastDot]
	r.Name = name
	return nil
}

func (r *CallableRef) String() string {
	var sb strings.Builder
	sb.WriteString(r.ImportPath)
	if r.TypeName != "" {
		sb.WriteString(".")
		sb.WriteString(r.TypeName)
	}
	sb.WriteString(".")
	sb.WriteString(r.Name)
	return sb.String()
}

func invertMap[K, V comparable](in map[K]V) map[V]K {
	out := make(map[V]K, len(in))
	for k, v := range in {
		out[v] = k
	}
	return out
}
