package morph

import (
	"testing"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFunctionRegistry(t *testing.T) {
	sourceType := namedTestType("module.test/from", "User")
	targetType := namedTestType("module.test/to", "User")

	t.Run("should initialize and return registered candidates", func(t *testing.T) {
		registry := newFunctionRegistry()

		fn := testFunctionDecl("MapUser", sourceType, targetType)
		require.True(t, registry.Register(fn, plan.CallableSourceUser))

		got := registry.Candidates(sourceType, targetType, plan.CallableSourceUser)
		require.Len(t, got, 1)
		assert.Equal(t, "MapUser", got[0].Name)
	})

	t.Run("should look up candidates without generic arguments", func(t *testing.T) {
		registry := newFunctionRegistry()

		typeParam := typeParamTestType("T")
		stringType := basicTestType("string")
		sourceGeneric := namedTestType("module.test/from", "Optional", typeParam)
		targetGeneric := namedTestType("module.test/to", "Optional", typeParam)
		sourceConcrete := namedTestType("module.test/from", "Optional", stringType)
		targetConcrete := namedTestType("module.test/to", "Optional", stringType)

		require.True(t, registry.Register(testFunctionDecl("MapOptional", sourceGeneric, targetGeneric), plan.CallableSourceDiscovered))

		got := registry.Candidates(sourceConcrete, targetConcrete, plan.CallableSourceDiscovered)
		require.Len(t, got, 1)
		assert.Equal(t, "MapOptional", got[0].Name)
	})

	t.Run("should look up higher-order candidates by first parameter and result", func(t *testing.T) {
		registry := newFunctionRegistry()

		inputParam := typeParamTestType("I")
		outputParam := typeParamTestType("O")
		stringType := basicTestType("string")
		intType := basicTestType("int")
		sourceGeneric := namedTestType("module.test/from", "Optional", inputParam)
		targetGeneric := namedTestType("module.test/to", "Optional", outputParam)
		sourceConcrete := namedTestType("module.test/from", "Optional", stringType)
		targetConcrete := namedTestType("module.test/to", "Optional", intType)
		mapperType := signatureTestType([]types.Type{inputParam}, outputParam)

		require.True(t, registry.Register(types.FunctionDecl{
			Package: types.PackageRef{Name: "mapping", ImportPath: "module.test/mapping"},
			Name:    "MapOptional",
			Params: []types.Parameter{
				{Type: sourceGeneric},
				{Type: mapperType},
			},
			Results: []types.Parameter{{Type: targetGeneric}},
		}, plan.CallableSourceDiscovered))

		got := registry.Candidates(sourceConcrete, targetConcrete, plan.CallableSourceDiscovered)
		require.Len(t, got, 1)
		assert.Equal(t, "MapOptional", got[0].Name)
	})

	t.Run("should only return functions from the requested source", func(t *testing.T) {
		registry := newFunctionRegistry()
		require.True(t, registry.Register(testFunctionDecl("UserMap", sourceType, targetType), plan.CallableSourceUser))
		require.True(t, registry.Register(testFunctionDecl("DiscoveredMap", sourceType, targetType), plan.CallableSourceDiscovered))

		got := registry.Candidates(sourceType, targetType, plan.CallableSourceUser)
		require.Len(t, got, 1)
		assert.Equal(t, "UserMap", got[0].Name)
	})

	t.Run("should distinguish target packages", func(t *testing.T) {
		registry := newFunctionRegistry()
		otherTargetType := namedTestType("module.test/other", "User")
		require.True(t, registry.Register(testFunctionDecl("MapToTarget", sourceType, targetType), plan.CallableSourceDiscovered))
		require.True(t, registry.Register(testFunctionDecl("MapToOther", sourceType, otherTargetType), plan.CallableSourceDiscovered))

		got := registry.Candidates(sourceType, targetType, plan.CallableSourceDiscovered)
		require.Len(t, got, 1)
		assert.Equal(t, "MapToTarget", got[0].Name)
	})

	t.Run("should return pointer input candidates", func(t *testing.T) {
		registry := newFunctionRegistry()
		require.True(t, registry.Register(testFunctionDecl("MapUserPtr", pointerTestType(sourceType), targetType), plan.CallableSourceDiscovered))

		got := registry.Candidates(sourceType, targetType, plan.CallableSourceDiscovered)
		require.Len(t, got, 1)
		assert.Equal(t, "MapUserPtr", got[0].Name)
	})

	t.Run("should return pointer result candidates", func(t *testing.T) {
		registry := newFunctionRegistry()
		require.True(t, registry.Register(testFunctionDecl("MapUserPtrResult", sourceType, pointerTestType(targetType)), plan.CallableSourceDiscovered))

		got := registry.Candidates(sourceType, targetType, plan.CallableSourceDiscovered)
		require.Len(t, got, 1)
		assert.Equal(t, "MapUserPtrResult", got[0].Name)
	})

	t.Run("should reject invalid functions", func(t *testing.T) {
		registry := newFunctionRegistry()

		assert.False(t, registry.Register(types.FunctionDecl{
			Name:    "NoParams",
			Results: []types.Parameter{{Type: targetType}},
		}, plan.CallableSourceUser))

		assert.False(t, registry.Register(types.FunctionDecl{
			Name:   "NoResults",
			Params: []types.Parameter{{Type: sourceType}},
		}, plan.CallableSourceUser))

		assert.False(t, registry.Register(types.FunctionDecl{
			Name:       "Variadic",
			Params:     []types.Parameter{{Type: sourceType}},
			Results:    []types.Parameter{{Type: targetType}},
			IsVariadic: true,
		}, plan.CallableSourceUser))

		assert.False(t, registry.Register(testFunctionDecl("Identity", typeParamTestType("T"), typeParamTestType("T")), plan.CallableSourceUser))
	})
}
