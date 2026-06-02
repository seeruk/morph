package plan

import (
	"testing"

	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallableRefFromMethod(t *testing.T) {
	sourceType := types.Type{
		Kind:    types.TypeKindNamed,
		Name:    "UserID",
		Package: types.PackageRef{Name: "from", ImportPath: "module.test/from"},
	}
	targetType := types.Type{
		Kind:   types.TypeKindBasic,
		Name:   "string",
		String: "string",
	}

	tests := []struct {
		name   string
		method types.Method
	}{
		{
			name: "should reject methods without receivers",
			method: types.Method{
				Name:    "String",
				Results: []types.Parameter{{Type: targetType}},
			},
		},
		{
			name: "should reject methods with explicit params",
			method: types.Method{
				Owner:    sourceType,
				Receiver: &types.Parameter{Type: sourceType},
				Name:     "Format",
				Params:   []types.Parameter{{Type: targetType}},
				Results:  []types.Parameter{{Type: targetType}},
			},
		},
		{
			name: "should reject methods without results",
			method: types.Method{
				Owner:    sourceType,
				Receiver: &types.Parameter{Type: sourceType},
				Name:     "Clear",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, ok := CallableRefFromMethod(tt.method, CallableSourceUser)
			assert.False(t, ok)
		})
	}

	t.Run("should use the receiver as the source type", func(t *testing.T) {
		callable, ok := CallableRefFromMethod(types.Method{
			Owner:    sourceType,
			Receiver: &types.Parameter{Type: sourceType},
			Name:     "String",
			Results:  []types.Parameter{{Type: targetType}},
		}, CallableSourceUser)

		require.True(t, ok)
		assert.Equal(t, CallableKindMethod, callable.Kind)
		assert.Equal(t, CallableSourceUser, callable.Source)
		assert.Equal(t, "String", callable.Name)
		assert.Equal(t, TypeRefFromType(sourceType), callable.SourceType)
		assert.Equal(t, TypeRefFromType(targetType), callable.TargetType)
		assert.Equal(t, sourceType.Package, callable.Package)
	})

	t.Run("should use the owner as the callable package for pointer receivers", func(t *testing.T) {
		pointerSource := types.Type{
			Kind: types.TypeKindPointer,
			Elem: &sourceType,
		}

		callable, ok := CallableRefFromMethod(types.Method{
			Owner:    sourceType,
			Receiver: &types.Parameter{Type: pointerSource},
			Name:     "String",
			Results:  []types.Parameter{{Type: targetType}},
		}, CallableSourceUser)

		require.True(t, ok)
		assert.Equal(t, TypeRefFromType(pointerSource), callable.SourceType)
		assert.Equal(t, sourceType.Package, callable.Package)
	})
}

func TestCallableRefFromFunctionDecl(t *testing.T) {
	sourceType := types.Type{
		Kind:    types.TypeKindNamed,
		Name:    "Optional",
		Package: types.PackageRef{Name: "from", ImportPath: "module.test/from"},
	}
	targetType := types.Type{
		Kind:    types.TypeKindNamed,
		Name:    "Optional",
		Package: types.PackageRef{Name: "to", ImportPath: "module.test/to"},
	}
	mapperType := types.Type{
		Kind: types.TypeKindSignature,
		Params: []types.Parameter{{
			Type: sourceType,
		}},
		Results: []types.Parameter{{
			Type: targetType,
		}},
	}

	t.Run("should allow additional parameters", func(t *testing.T) {
		callable, ok := CallableRefFromFunctionDecl(types.FunctionDecl{
			Name:    "MapOptional",
			Package: types.PackageRef{Name: "mapping", ImportPath: "module.test/mapping"},
			Params: []types.Parameter{
				{Type: sourceType},
				{Type: mapperType},
			},
			Results: []types.Parameter{{Type: targetType}},
		}, CallableSourceUser)

		require.True(t, ok)
		assert.Equal(t, CallableKindFunction, callable.Kind)
		assert.Equal(t, CallableSourceUser, callable.Source)
		assert.Equal(t, "MapOptional", callable.Name)
		assert.Equal(t, TypeRefFromType(sourceType), callable.SourceType)
		assert.Equal(t, TypeRefFromType(targetType), callable.TargetType)
	})

	t.Run("should reject functions without parameters", func(t *testing.T) {
		_, ok := CallableRefFromFunctionDecl(types.FunctionDecl{
			Name:    "MapOptional",
			Results: []types.Parameter{{Type: targetType}},
		}, CallableSourceUser)

		assert.False(t, ok)
	})

	t.Run("should reject additional non-callable parameters", func(t *testing.T) {
		_, ok := CallableRefFromFunctionDecl(types.FunctionDecl{
			Name: "MapOptional",
			Params: []types.Parameter{
				{Type: sourceType},
				{Type: targetType},
			},
			Results: []types.Parameter{{Type: targetType}},
		}, CallableSourceUser)

		assert.False(t, ok)
	})
}
