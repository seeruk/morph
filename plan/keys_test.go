package plan

import (
	"testing"

	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
)

func TestTypeKey(t *testing.T) {
	stringType := types.Type{
		Kind:   types.TypeKindBasic,
		Name:   "string",
		String: "string",
	}
	intType := types.Type{
		Kind:   types.TypeKindBasic,
		Name:   "int",
		String: "int",
	}
	userType := types.Type{
		Kind:    types.TypeKindNamed,
		Name:    "User",
		Package: types.PackageRef{ImportPath: "module.test/example"},
	}

	tests := []struct {
		name string
		typ  types.Type
		want string
	}{
		{
			name: "basic uses string",
			typ:  stringType,
			want: "string",
		},
		{
			name: "named includes import path",
			typ:  userType,
			want: "module.test/example.User",
		},
		{
			name: "named generic includes type arguments",
			typ: types.Type{
				Kind:    types.TypeKindNamed,
				Name:    "Pair",
				Package: types.PackageRef{ImportPath: "module.test/example"},
				TypeArgs: []types.Type{
					stringType,
					userType,
				},
			},
			want: "module.test/example.Pair[string, module.test/example.User]",
		},
		{
			name: "type parameter uses name",
			typ: types.Type{
				Kind:   types.TypeKindTypeParam,
				Name:   "T",
				String: "any",
			},
			want: "T",
		},
		{
			name: "alias unwraps to target",
			typ: types.Type{
				Kind: types.TypeKindAlias,
				Name: "UserAlias",
				Elem: new(userType),
			},
			want: "module.test/example.User",
		},
		{
			name: "pointer includes element key",
			typ: types.Type{
				Kind: types.TypeKindPointer,
				Elem: new(userType),
			},
			want: "*module.test/example.User",
		},
		{
			name: "slice includes element key",
			typ: types.Type{
				Kind: types.TypeKindSlice,
				Elem: new(userType),
			},
			want: "[]module.test/example.User",
		},
		{
			name: "array includes length and element key",
			typ: types.Type{
				Kind: types.TypeKindArray,
				Len:  3,
				Elem: new(intType),
			},
			want: "[3]int",
		},
		{
			name: "map includes key and value keys",
			typ: types.Type{
				Kind:  types.TypeKindMap,
				Key:   new(stringType),
				Value: new(userType),
			},
			want: "map[string]module.test/example.User",
		},
		{
			name: "unqualified named uses name",
			typ: types.Type{
				Kind: types.TypeKindNamed,
				Name: "Local",
			},
			want: "Local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, types.TypeKey(tt.typ))
		})
	}
}

func TestTypePairKey(t *testing.T) {
	tests := []struct {
		name   string
		source TypeRef
		target TypeRef
		want   string
	}{
		{
			name:   "combines source and target keys",
			source: TypeRef{Key: "source"},
			target: TypeRef{Key: "target"},
			want:   "source->target",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TypePairKey(tt.source, tt.target))
		})
	}
}

func TestSignatureKey(t *testing.T) {
	tests := []struct {
		name      string
		signature spec.MapperSignature
		want      string
	}{
		{
			name: "value to value",
			signature: spec.MapperSignature{
				Accepts: spec.ParameterKindValue,
				Returns: spec.ParameterKindValue,
			},
			want: "value->value",
		},
		{
			name: "pointer to value",
			signature: spec.MapperSignature{
				Accepts: spec.ParameterKindPointer,
				Returns: spec.ParameterKindValue,
			},
			want: "pointer->value",
		},
		{
			name: "value to pointer",
			signature: spec.MapperSignature{
				Accepts: spec.ParameterKindValue,
				Returns: spec.ParameterKindPointer,
			},
			want: "value->pointer",
		},
		{
			name: "pointer to pointer",
			signature: spec.MapperSignature{
				Accepts: spec.ParameterKindPointer,
				Returns: spec.ParameterKindPointer,
			},
			want: "pointer->pointer",
		},
		{
			name:      "should treat zero-value signature parts as value",
			signature: spec.MapperSignature{},
			want:      "value->value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, SignatureKey(tt.signature))
		})
	}
}

func TestTypeMapperKey(t *testing.T) {
	tests := []struct {
		name      string
		source    TypeRef
		target    TypeRef
		signature spec.MapperSignature
		want      string
	}{
		{
			name:   "combines type pair and signature keys",
			source: TypeRef{Key: "module.test/source.User"},
			target: TypeRef{Key: "module.test/target.User"},
			signature: spec.MapperSignature{
				Accepts: spec.ParameterKindPointer,
				Returns: spec.ParameterKindValue,
			},
			want: "module.test/source.User->module.test/target.User|pointer->value",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TypeMapperKey(tt.source, tt.target, tt.signature))
		})
	}
}
