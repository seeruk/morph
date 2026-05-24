package plan

import (
	"fmt"
	"strings"

	"github.com/seeruk/morph/types"
)

// typeKey returns a string key that uniquely identifies a type, including its structure and type
// arguments. This is used for caching and comparison purposes.
//
// We can't just rely on the String field from types.TypeInfo, which is provided by go/types,
// because Morph may substitute type arguments or unwrap aliases after loading.
func typeKey(t types.Type) string {
	t = types.UnwrapAlias(t)

	switch t.Kind {
	case types.TypeKindNamed, types.TypeKindAlias:
		name := t.Name
		if t.Package.ImportPath != "" {
			name = t.Package.ImportPath + "." + name
		}
		if len(t.TypeArgs) > 0 {
			args := make([]string, 0, len(t.TypeArgs))
			for _, arg := range t.TypeArgs {
				args = append(args, typeKey(arg))
			}
			return name + "[" + strings.Join(args, ", ") + "]"
		}
	case types.TypeKindTypeParam:
		if t.Name != "" {
			return t.Name
		}
	}

	if t.String != "" {
		return t.String
	}

	switch t.Kind {
	case types.TypeKindPointer:
		if t.Elem != nil {
			return "*" + typeKey(*t.Elem)
		}
	case types.TypeKindSlice:
		if t.Elem != nil {
			return "[]" + typeKey(*t.Elem)
		}
	case types.TypeKindArray:
		if t.Elem != nil {
			return fmt.Sprintf("[%d]%s", t.Len, typeKey(*t.Elem))
		}
	case types.TypeKindMap:
		if t.Key != nil && t.Value != nil {
			return fmt.Sprintf("map[%s]%s", typeKey(*t.Key), typeKey(*t.Value))
		}
	}

	if t.Package.ImportPath != "" && t.Name != "" {
		return t.Package.ImportPath + "." + t.Name
	}

	return t.Name
}

// typePairKey returns a combination of the keys of two given TypeRef. This key doesn't need to be
// recreated here, we just take the "cached" key on the TypeRef.
func typePairKey(source TypeRef, target TypeRef) string {
	return source.Key + "->" + target.Key
}

// signatureKey returns a string that identifies the signature structure of a mapper function.
// Typically, this will be used in combination with another key function to uniquely identify a
// mapper.
func signatureKey(signature MapperSignature) string {
	return signature.Accepts.String() + "->" + signature.Returns.String()
}

// typeMapperKey returns a key that uniquely identifies an individual mapper function. This is
// particularly useful for caching planned mappers and looking them up again later, as the types
// used to create this key are unfortunately not able to be comparable (e.g. would contain slices).
func typeMapperKey(source TypeRef, target TypeRef, signature MapperSignature) string {
	return typePairKey(source, target) + "|" + signatureKey(signature)
}
