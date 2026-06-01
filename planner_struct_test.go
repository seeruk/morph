package morph

import (
	"testing"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAssessFunctionCompatibility(t *testing.T) {
	sourceType := namedTestType("module.test/from", "User")
	targetType := namedTestType("module.test/to", "User")

	t.Run("should match exact parameter and result types", func(t *testing.T) {
		got := assessFunctionCompatibility(sourceType, targetType, testFunctionDecl("MapUser", sourceType, targetType))

		require.True(t, got.Compatible())
		assert.Equal(t, callableInputExact, got.Input)
		assert.Equal(t, callableResultExact, got.Result)
	})

	t.Run("should match generic parameter and result types", func(t *testing.T) {
		typeParam := typeParamTestType("T")
		stringType := basicTestType("string")
		sourceConcrete := namedTestType("module.test/from", "Optional", stringType)
		targetConcrete := namedTestType("module.test/to", "Optional", stringType)
		sourceGeneric := namedTestType("module.test/from", "Optional", typeParam)
		targetGeneric := namedTestType("module.test/to", "Optional", typeParam)

		got := assessFunctionCompatibility(
			sourceConcrete,
			targetConcrete,
			testFunctionDecl("MapOptional", sourceGeneric, targetGeneric),
		)

		require.True(t, got.Compatible())
		assert.Equal(t, callableInputExact, got.Input)
		assert.Equal(t, callableResultGeneric, got.Result)
		assert.Equal(t, stringType, got.TypeBindings["T"])
	})

	t.Run("should reject concrete generic mismatches", func(t *testing.T) {
		stringType := basicTestType("string")
		intType := basicTestType("int")
		sourceString := namedTestType("module.test/from", "Optional", stringType)
		targetString := namedTestType("module.test/to", "Optional", stringType)
		sourceInt := namedTestType("module.test/from", "Optional", intType)
		targetInt := namedTestType("module.test/to", "Optional", intType)

		got := assessFunctionCompatibility(
			sourceString,
			targetString,
			testFunctionDecl("MapOptionalInt", sourceInt, targetInt),
		)

		assert.False(t, got.Compatible())
	})

	t.Run("should match auto-address parameter types", func(t *testing.T) {
		got := assessFunctionCompatibility(
			sourceType,
			targetType,
			testFunctionDecl("MapUserPtr", pointerTestType(sourceType), targetType),
		)

		require.True(t, got.Compatible())
		assert.Equal(t, callableInputAutoAddress, got.Input)
		assert.Equal(t, callableResultExact, got.Result)
	})

	t.Run("should match auto-deref parameter types", func(t *testing.T) {
		got := assessFunctionCompatibility(
			pointerTestType(sourceType),
			targetType,
			testFunctionDecl("MapUser", sourceType, targetType),
		)

		require.True(t, got.Compatible())
		assert.Equal(t, callableInputAutoDeref, got.Input)
		assert.Equal(t, callableResultExact, got.Result)
	})
}

func TestBestCallableCandidate(t *testing.T) {
	typeParam := typeParamTestType("T")
	stringType := basicTestType("string")
	sourceType := namedTestType("module.test/from", "Optional", stringType)
	targetType := namedTestType("module.test/to", "Optional", stringType)
	sourceGeneric := namedTestType("module.test/from", "Optional", typeParam)
	targetGeneric := namedTestType("module.test/to", "Optional", typeParam)

	t.Run("should prefer exact result over generic result", func(t *testing.T) {
		genericFn := testFunctionDecl("GenericResult", sourceGeneric, targetGeneric)
		exactFn := testFunctionDecl("ExactResult", sourceGeneric, targetType)

		best, ok := bestCallableCandidate(functionCandidates(sourceType, targetType, genericFn, exactFn))
		require.True(t, ok)

		assert.Equal(t, "ExactResult", best.Name)
	})

	t.Run("should prefer functions that do not error", func(t *testing.T) {
		errorFn := testFunctionDecl("MapWithError", sourceGeneric, targetType, errorTestType())
		noErrorFn := testFunctionDecl("MapWithoutError", sourceGeneric, targetType)

		best, ok := bestCallableCandidate(functionCandidates(sourceType, targetType, errorFn, noErrorFn))
		require.True(t, ok)

		assert.Equal(t, "MapWithoutError", best.Name)
	})

	t.Run("should tie-break consistently", func(t *testing.T) {
		alphaFn := testFunctionDecl("Alpha", sourceGeneric, targetType)
		betaFn := testFunctionDecl("Beta", sourceGeneric, targetType)

		for range 10 {
			best, ok := bestCallableCandidate(functionCandidates(sourceType, targetType, alphaFn, betaFn))
			require.True(t, ok)

			assert.Equal(t, "Beta", best.Name)
		}
	})
}

func TestFunctionCompatibilityMatchesMethodCompatibility(t *testing.T) {
	typeParam := typeParamTestType("T")
	stringType := basicTestType("string")
	sourceType := namedTestType("module.test/from", "User")
	targetType := namedTestType("module.test/to", "User")
	sourceGeneric := namedTestType("module.test/from", "Optional", typeParam)
	targetGeneric := namedTestType("module.test/to", "Optional", typeParam)
	sourceConcrete := namedTestType("module.test/from", "Optional", stringType)
	targetConcrete := namedTestType("module.test/to", "Optional", stringType)

	tests := []struct {
		name   string
		source types.Type
		target types.Type
		input  types.Type
		result types.Type
	}{
		{
			name:   "exact",
			source: sourceType,
			target: targetType,
			input:  sourceType,
			result: targetType,
		},
		{
			name:   "auto-address",
			source: sourceType,
			target: targetType,
			input:  pointerTestType(sourceType),
			result: targetType,
		},
		{
			name:   "auto-deref",
			source: pointerTestType(sourceType),
			target: targetType,
			input:  sourceType,
			result: targetType,
		},
		{
			name:   "generic",
			source: sourceConcrete,
			target: targetConcrete,
			input:  sourceGeneric,
			result: targetGeneric,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fnCompatibility := assessFunctionCompatibility(tt.source, tt.target, testFunctionDecl("Map", tt.input, tt.result))
			methodCompatibility := assessMethodCompatibility(tt.source, tt.target, testMethodDecl("Map", tt.input, tt.result))

			fnRank, fnOK := callableCompatibilityRank(fnCompatibility)
			methodRank, methodOK := callableCompatibilityRank(methodCompatibility)

			require.True(t, fnCompatibility.Compatible())
			require.True(t, methodCompatibility.Compatible())
			assert.Equal(t, fnCompatibility, methodCompatibility)
			assert.Equal(t, fnOK, methodOK)
			assert.Equal(t, fnRank, methodRank)
		})
	}
}

func TestPlannerPlanValuePrefersUserFunctionCandidates(t *testing.T) {
	sourceType := namedTestType("module.test/from", "User")
	targetType := namedTestType("module.test/to", "User")
	registry := newFunctionRegistry()
	require.True(t, registry.Register(testFunctionDecl("UserMap", sourceType, targetType, errorTestType()), plan.CallableSourceUser))
	require.True(t, registry.Register(testFunctionDecl("DiscoveredMap", sourceType, targetType), plan.CallableSourceDiscovered))

	planner := &Planner{registry: registry}

	got := planner.planValue(sourceType, targetType, "User")
	require.NotNil(t, got.Callable)

	assert.Equal(t, plan.OperationFunction, got.Operation)
	assert.Equal(t, "UserMap", got.Callable.Name)
	assert.True(t, got.CanError)
}

func functionCandidates(sourceType, targetType types.Type, fns ...types.FunctionDecl) map[plan.CallableRef]callableCompatibility {
	out := make(map[plan.CallableRef]callableCompatibility, len(fns))
	for _, fn := range fns {
		callable, ok := plan.CallableRefFromFunctionDecl(fn, plan.CallableSourceDiscovered)
		if !ok {
			continue
		}
		out[callable] = assessFunctionCompatibility(sourceType, targetType, fn)
	}
	return out
}
