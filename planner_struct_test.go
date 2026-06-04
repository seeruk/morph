package morph

import (
	"testing"

	"github.com/seeruk/morph/config"
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
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

	t.Run("should match auto-address result types", func(t *testing.T) {
		got := assessFunctionCompatibility(
			sourceType,
			pointerTestType(targetType),
			testFunctionDecl("MapUser", sourceType, targetType),
		)

		require.True(t, got.Compatible())
		assert.Equal(t, callableInputExact, got.Input)
		assert.Equal(t, callableResultAutoAddress, got.Result)
	})

	t.Run("should match auto-deref result types", func(t *testing.T) {
		got := assessFunctionCompatibility(
			sourceType,
			targetType,
			testFunctionDecl("MapUserPtr", sourceType, pointerTestType(targetType)),
		)

		require.True(t, got.Compatible())
		assert.Equal(t, callableInputExact, got.Input)
		assert.Equal(t, callableResultAutoDeref, got.Result)
	})
}

func TestAssessHigherOrderFunctionCompatibility(t *testing.T) {
	inputParam := typeParamTestType("I")
	outputParam := typeParamTestType("O")
	stringType := basicTestType("string")
	intType := basicTestType("int")
	sourceType := namedTestType("module.test/from", "Optional", stringType)
	targetType := namedTestType("module.test/to", "Optional", intType)
	sourceGeneric := namedTestType("module.test/from", "Optional", inputParam)
	targetGeneric := namedTestType("module.test/to", "Optional", outputParam)

	got := assessHigherOrderFunctionCompatibility(sourceType, targetType, types.FunctionDecl{
		Name: "MapOptional",
		Params: []types.Parameter{
			{Type: sourceGeneric},
			{Type: signatureTestType([]types.Type{inputParam}, outputParam)},
		},
		Results: []types.Parameter{{Type: targetGeneric}},
	})

	require.True(t, got.Compatible())

	t.Run("matches the container signature", func(t *testing.T) {
		assert.Equal(t, callableInputExact, got.Input)
		assert.Equal(t, callableResultGeneric, got.Result)
	})

	t.Run("binds input and output type parameters", func(t *testing.T) {
		assert.Equal(t, stringType, got.TypeBindings["I"])
		assert.Equal(t, intType, got.TypeBindings["O"])
	})

	t.Run("describes the mapper argument", func(t *testing.T) {
		require.Len(t, got.MapperArgs, 1)
		assert.Equal(t, stringType, got.MapperArgs[0].Source)
		assert.Equal(t, intType, got.MapperArgs[0].Target)
		assert.False(t, got.MapperArgs[0].ReturnsError)
	})

	t.Run("matches auto-address result types", func(t *testing.T) {
		got := assessHigherOrderFunctionCompatibility(sourceType, pointerTestType(targetType), types.FunctionDecl{
			Name: "MapOptional",
			Params: []types.Parameter{
				{Type: sourceGeneric},
				{Type: signatureTestType([]types.Type{inputParam}, outputParam)},
			},
			Results: []types.Parameter{{Type: targetGeneric}},
		})

		require.True(t, got.Compatible())
		assert.Equal(t, callableResultGenericAutoAddress, got.Result)
		assert.Equal(t, intType, got.TypeBindings["O"])
	})

	t.Run("matches auto-deref result types", func(t *testing.T) {
		got := assessHigherOrderFunctionCompatibility(sourceType, targetType, types.FunctionDecl{
			Name: "MapOptional",
			Params: []types.Parameter{
				{Type: sourceGeneric},
				{Type: signatureTestType([]types.Type{inputParam}, outputParam)},
			},
			Results: []types.Parameter{{Type: pointerTestType(targetGeneric)}},
		})

		require.True(t, got.Compatible())
		assert.Equal(t, callableResultGenericAutoDeref, got.Result)
		assert.Equal(t, intType, got.TypeBindings["O"])
	})
}

func TestAssessHigherOrderFunctionCompatibilityWithErrors(t *testing.T) {
	inputParam := typeParamTestType("I")
	outputParam := typeParamTestType("O")
	stringType := basicTestType("string")
	intType := basicTestType("int")
	sourceType := namedTestType("module.test/from", "FallibleOptional", stringType)
	targetType := namedTestType("module.test/to", "FallibleOptional", intType)
	sourceGeneric := namedTestType("module.test/from", "FallibleOptional", inputParam)
	targetGeneric := namedTestType("module.test/to", "FallibleOptional", outputParam)

	got := assessHigherOrderFunctionCompatibility(sourceType, targetType, types.FunctionDecl{
		Name: "MapFallibleOptional",
		Params: []types.Parameter{
			{Type: sourceGeneric},
			{Type: signatureTestType([]types.Type{inputParam}, outputParam, errorTestType())},
		},
		Results: []types.Parameter{
			{Type: targetGeneric},
			{Type: errorTestType()},
		},
	})

	require.True(t, got.Compatible())

	t.Run("marks the container callable as erroring", func(t *testing.T) {
		assert.True(t, got.ReturnsError)
	})

	t.Run("binds input and output type parameters", func(t *testing.T) {
		assert.Equal(t, stringType, got.TypeBindings["I"])
		assert.Equal(t, intType, got.TypeBindings["O"])
	})

	t.Run("describes the erroring mapper argument", func(t *testing.T) {
		require.Len(t, got.MapperArgs, 1)
		assert.Equal(t, stringType, got.MapperArgs[0].Source)
		assert.Equal(t, intType, got.MapperArgs[0].Target)
		assert.True(t, got.MapperArgs[0].ReturnsError)
	})
}

func TestAssessHigherOrderFunctionCompatibilityWithMultipleMapperArgs(t *testing.T) {
	leftInputParam := typeParamTestType("LI")
	rightInputParam := typeParamTestType("RI")
	leftOutputParam := typeParamTestType("LO")
	rightOutputParam := typeParamTestType("RO")
	leftSourceType := namedTestType("module.test/from", "Left")
	rightSourceType := namedTestType("module.test/from", "Right")
	leftTargetType := namedTestType("module.test/to", "Left")
	rightTargetType := namedTestType("module.test/to", "Right")
	sourceType := namedTestType("module.test/from", "Either", leftSourceType, rightSourceType)
	targetType := namedTestType("module.test/to", "Either", leftTargetType, rightTargetType)
	sourceGeneric := namedTestType("module.test/from", "Either", leftInputParam, rightInputParam)
	targetGeneric := namedTestType("module.test/to", "Either", leftOutputParam, rightOutputParam)

	got := assessHigherOrderFunctionCompatibility(sourceType, targetType, types.FunctionDecl{
		Name: "MapEither",
		Params: []types.Parameter{
			{Type: sourceGeneric},
			{Type: signatureTestType([]types.Type{leftInputParam}, leftOutputParam)},
			{Type: signatureTestType([]types.Type{rightInputParam}, rightOutputParam)},
		},
		Results: []types.Parameter{{Type: targetGeneric}},
	})

	require.True(t, got.Compatible())

	t.Run("matches the container signature", func(t *testing.T) {
		assert.Equal(t, callableInputExact, got.Input)
		assert.Equal(t, callableResultGeneric, got.Result)
	})

	t.Run("binds all input and output type parameters", func(t *testing.T) {
		assert.Equal(t, leftSourceType, got.TypeBindings["LI"])
		assert.Equal(t, rightSourceType, got.TypeBindings["RI"])
		assert.Equal(t, leftTargetType, got.TypeBindings["LO"])
		assert.Equal(t, rightTargetType, got.TypeBindings["RO"])
	})

	require.Len(t, got.MapperArgs, 2)
	t.Run("describes the left mapper argument", func(t *testing.T) {
		assert.Equal(t, leftSourceType, got.MapperArgs[0].Source)
		assert.Equal(t, leftTargetType, got.MapperArgs[0].Target)
		assert.False(t, got.MapperArgs[0].ReturnsError)
	})

	t.Run("describes the right mapper argument", func(t *testing.T) {
		assert.Equal(t, rightSourceType, got.MapperArgs[1].Source)
		assert.Equal(t, rightTargetType, got.MapperArgs[1].Target)
		assert.False(t, got.MapperArgs[1].ReturnsError)
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

		best, _, ok := bestCallableCandidate(functionCandidates(sourceType, targetType, genericFn, exactFn))
		require.True(t, ok)

		assert.Equal(t, "ExactResult", best.Name)
	})

	t.Run("should prefer functions that do not error", func(t *testing.T) {
		errorFn := testFunctionDecl("MapWithError", sourceGeneric, targetType, errorTestType())
		noErrorFn := testFunctionDecl("MapWithoutError", sourceGeneric, targetType)

		best, _, ok := bestCallableCandidate(functionCandidates(sourceType, targetType, errorFn, noErrorFn))
		require.True(t, ok)

		assert.Equal(t, "MapWithoutError", best.Name)
	})

	t.Run("should tie-break consistently", func(t *testing.T) {
		alphaFn := testFunctionDecl("Alpha", sourceGeneric, targetType)
		betaFn := testFunctionDecl("Beta", sourceGeneric, targetType)

		for range 10 {
			best, _, ok := bestCallableCandidate(functionCandidates(sourceType, targetType, alphaFn, betaFn))
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
	userFn := testFunctionDecl("UserMap", sourceType, targetType, errorTestType())
	userRef := spec.CallableRefFromFunctionDecl(userFn)
	registry := newFunctionRegistry()
	require.True(t, registry.Register(testFunctionDecl("DiscoveredMap", sourceType, targetType), plan.CallableSourceDiscovered))

	planner := &Planner{
		registry: registry,
		callables: map[spec.CallableRef]registeredCallable{
			userRef: {Function: &userFn},
		},
	}

	got := planner.planValue(
		sourceType,
		targetType,
		"User",
		defaultOptionality(),
		defaultConversionsPolicy(),
		[]spec.TieredCallables{{
			Tier:      spec.CallableTierType,
			Callables: []spec.CallableRef{userRef},
		}},
	)
	require.NotNil(t, got.Callable)

	assert.Equal(t, plan.OperationFunction, got.Operation)
	assert.Equal(t, "UserMap", got.Callable.Name)
	assert.True(t, got.CanError)
}

func TestPlannerPlanValueCallableTiers(t *testing.T) {
	sourceType := basicTestType("string")
	targetType := basicTestType("int")

	t.Run("uses specificity before ranking", func(t *testing.T) {
		typeFn := testFunctionDecl("TypeStringToInt", sourceType, targetType, errorTestType())
		packageFn := testFunctionDecl("PackageStringToInt", sourceType, targetType)
		planner := plannerWithCallables(typeFn, packageFn)

		got := planner.planValue(
			sourceType,
			targetType,
			"Value",
			defaultOptionality(),
			defaultConversionsPolicy(),
			[]spec.TieredCallables{
				callableTier(spec.CallableTierType, typeFn),
				callableTier(spec.CallableTierPackage, packageFn),
			},
		)

		require.NotNil(t, got.Callable)
		assert.Equal(t, "TypeStringToInt", got.Callable.Name)
	})

	t.Run("ranks candidates within one tier", func(t *testing.T) {
		errorFn := testFunctionDecl("ErrorStringToInt", sourceType, targetType, errorTestType())
		noErrorFn := testFunctionDecl("NoErrorStringToInt", sourceType, targetType)
		planner := plannerWithCallables(errorFn, noErrorFn)

		got := planner.planValue(
			sourceType,
			targetType,
			"Value",
			defaultOptionality(),
			defaultConversionsPolicy(),
			[]spec.TieredCallables{callableTier(spec.CallableTierType, errorFn, noErrorFn)},
		)

		require.NotNil(t, got.Callable)
		assert.Equal(t, "NoErrorStringToInt", got.Callable.Name)
	})

	t.Run("falls back from incompatible scoped callables", func(t *testing.T) {
		badFn := testFunctionDecl("StringToBool", sourceType, basicTestType("bool"))
		goodFn := testFunctionDecl("DiscoveredStringToInt", sourceType, targetType)
		registry := newFunctionRegistry()
		require.True(t, registry.Register(goodFn, plan.CallableSourceDiscovered))
		planner := plannerWithCallables(badFn)
		planner.registry = registry

		got := planner.planValue(
			sourceType,
			targetType,
			"Value",
			defaultOptionality(),
			defaultConversionsPolicy(),
			[]spec.TieredCallables{callableTier(spec.CallableTierType, badFn)},
		)

		require.NotNil(t, got.Callable)
		assert.Equal(t, "DiscoveredStringToInt", got.Callable.Name)
	})
}

func TestPlannerPlanStructFieldCallable(t *testing.T) {
	sourceDecl := testStructDecl("module.test/from", "Thing", map[string]types.Field{
		"Value": testField("Value", basicTestType("string")),
	})
	targetDecl := testStructDecl("module.test/to", "Thing", map[string]types.Field{
		"Value": testField("Value", basicTestType("int")),
	})

	t.Run("uses the explicit field callable", func(t *testing.T) {
		fn := testFunctionDecl("StringToInt", basicTestType("string"), basicTestType("int"))
		ref := spec.CallableRefFromFunctionDecl(fn)
		planner := plannerWithCallables(fn)
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{
			Fields: map[string]spec.Field{
				"Value": {Target: "Value", Callable: &ref},
			},
		})

		planner.planStruct(&typ)

		field := requirePlanField(t, typ.StructPlan, "Value")
		require.NotNil(t, field.Mapping.Callable)
		assert.Equal(t, "StringToInt", field.Mapping.Callable.Name)
	})

	t.Run("fails strictly when the field callable is incompatible", func(t *testing.T) {
		fn := testFunctionDecl("StringToBool", basicTestType("string"), basicTestType("bool"))
		ref := spec.CallableRefFromFunctionDecl(fn)
		planner := plannerWithCallables(fn)
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{
			Fields: map[string]spec.Field{
				"Value": {Target: "Value", Callable: &ref},
			},
		})

		planner.planStruct(&typ)

		field := requirePlanField(t, typ.StructPlan, "Value")
		assert.Equal(t, plan.OperationUnsupported, field.Mapping.Operation)
		require.Len(t, field.Mapping.Diagnostics, 1)
		assert.Equal(t, plan.DiagnosticLevelFatal, field.Mapping.Diagnostics[0].Level)
		assert.Equal(t, plan.FieldPath(typ.SourceType, typ.TargetType, field.SourceField, field.TargetField), field.Mapping.Diagnostics[0].Path)
		assert.Contains(t, field.Mapping.Diagnostics[0].Message, `configured field callable "module.test/mapping.StringToBool" is not compatible`)
		assert.Contains(t, field.Mapping.Diagnostics[0].Message, "expected callable to accept string and return int")
	})
}

func TestPlannerPlanValueOptionality(t *testing.T) {
	stringType := basicTestType("string")
	intType := basicTestType("int")
	errorOptionality := spec.Optionality{
		OnNilSourcePointer: spec.PointerOptionalityError,
		OnZeroSourceValue:  spec.ValueOptionalityAddress,
	}
	conversionsPolicy := defaultConversionsPolicy()

	t.Run("marks pointer to value mappings as erroring when nil source pointers error", func(t *testing.T) {
		planner := &Planner{registry: newFunctionRegistry()}

		got := planner.planValue(pointerTestType(stringType), stringType, "Name", errorOptionality, conversionsPolicy, nil)

		assert.Equal(t, plan.OperationPointer, got.Operation)
		assert.True(t, got.SourcePointer)
		assert.False(t, got.TargetPointer)
		assert.Equal(t, errorOptionality, got.Optionality)
		assert.True(t, got.CanError)
	})

	t.Run("keeps pointer to value mappings non-erroring when nil source pointers become zero", func(t *testing.T) {
		planner := &Planner{registry: newFunctionRegistry()}
		optionality := defaultOptionality()

		got := planner.planValue(pointerTestType(stringType), stringType, "Name", optionality, conversionsPolicy, nil)

		assert.Equal(t, plan.OperationPointer, got.Operation)
		assert.False(t, got.CanError)
	})

	t.Run("records auto-deref callable input adaptation", func(t *testing.T) {
		registry := newFunctionRegistry()
		require.True(t, registry.Register(testFunctionDecl("StringToInt", stringType, intType), plan.CallableSourceDiscovered))
		planner := &Planner{registry: registry}

		got := planner.planValue(pointerTestType(stringType), intType, "Name", errorOptionality, conversionsPolicy, nil)

		require.NotNil(t, got.Callable)
		assert.Equal(t, plan.OperationFunction, got.Operation)
		assert.Equal(t, plan.ValueAdaptationDeref, got.CallableParameterAdaptation)
		assert.Equal(t, plan.ValueAdaptationNone, got.CallableResultAdaptation)
		assert.Equal(t, errorOptionality, got.Optionality)
		assert.True(t, got.CanError)
	})

	t.Run("records auto-address callable input adaptation", func(t *testing.T) {
		registry := newFunctionRegistry()
		require.True(t, registry.Register(testFunctionDecl("StringPtrToInt", pointerTestType(stringType), intType), plan.CallableSourceDiscovered))
		planner := &Planner{registry: registry}
		optionality := spec.Optionality{
			OnNilSourcePointer: spec.PointerOptionalityZero,
			OnZeroSourceValue:  spec.ValueOptionalityNil,
		}

		got := planner.planValue(stringType, intType, "Name", optionality, conversionsPolicy, nil)

		require.NotNil(t, got.Callable)
		assert.Equal(t, plan.OperationFunction, got.Operation)
		assert.Equal(t, plan.ValueAdaptationAddress, got.CallableParameterAdaptation)
		assert.Equal(t, plan.ValueAdaptationNone, got.CallableResultAdaptation)
		assert.Equal(t, optionality, got.Optionality)
		assert.False(t, got.CanError)
	})

	t.Run("records auto-deref callable result adaptation", func(t *testing.T) {
		registry := newFunctionRegistry()
		require.True(t, registry.Register(testFunctionDecl("StringToIntPtr", stringType, pointerTestType(intType)), plan.CallableSourceDiscovered))
		planner := &Planner{registry: registry}

		got := planner.planValue(stringType, intType, "Name", errorOptionality, conversionsPolicy, nil)

		require.NotNil(t, got.Callable)
		assert.Equal(t, plan.OperationFunction, got.Operation)
		assert.Equal(t, plan.ValueAdaptationNone, got.CallableParameterAdaptation)
		assert.Equal(t, plan.ValueAdaptationDeref, got.CallableResultAdaptation)
		assert.Equal(t, errorOptionality, got.Optionality)
		assert.True(t, got.CanError)
	})

	t.Run("records auto-address callable result adaptation", func(t *testing.T) {
		registry := newFunctionRegistry()
		require.True(t, registry.Register(testFunctionDecl("StringToInt", stringType, intType), plan.CallableSourceDiscovered))
		planner := &Planner{registry: registry}
		optionality := spec.Optionality{
			OnNilSourcePointer: spec.PointerOptionalityZero,
			OnZeroSourceValue:  spec.ValueOptionalityNil,
		}

		got := planner.planValue(stringType, pointerTestType(intType), "Name", optionality, conversionsPolicy, nil)

		require.NotNil(t, got.Callable)
		assert.Equal(t, plan.OperationFunction, got.Operation)
		assert.Equal(t, plan.ValueAdaptationNone, got.CallableParameterAdaptation)
		assert.Equal(t, plan.ValueAdaptationAddress, got.CallableResultAdaptation)
		assert.Equal(t, optionality, got.Optionality)
		assert.False(t, got.CanError)
	})
}

func TestPlannerPlanStruct(t *testing.T) {
	t.Run("uses explicit field mappings as source to target", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Recipe", map[string]types.Field{
			"RecipeId":    testField("RecipeId", basicTestType("string")),
			"DisplayName": testField("DisplayName", basicTestType("string")),
		})
		targetDecl := testStructDecl("module.test/to", "Recipe", map[string]types.Field{
			"ID":   testField("ID", basicTestType("string")),
			"Name": testField("Name", basicTestType("string")),
		})
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{
			Fields: map[string]spec.Field{
				"RecipeId":    {Target: "ID"},
				"DisplayName": {Target: "Name"},
			},
		})
		planner := &Planner{registry: newFunctionRegistry()}

		planner.planStruct(&typ)

		require.Len(t, typ.StructPlan.Fields, 2)
		assert.Equal(t, "RecipeId", typ.StructPlan.Fields[0].SourceField.Name)
		assert.Equal(t, "ID", typ.StructPlan.Fields[0].TargetField.Name)
		assert.Equal(t, "DisplayName", typ.StructPlan.Fields[1].SourceField.Name)
		assert.Equal(t, "Name", typ.StructPlan.Fields[1].TargetField.Name)
	})

	t.Run("does not reuse explicitly remapped source fields by fallback matching", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Recipe", map[string]types.Field{
			"RecipeId": testField("RecipeId", basicTestType("string")),
		})
		targetDecl := testStructDecl("module.test/to", "Recipe", map[string]types.Field{
			"ID":       testField("ID", basicTestType("string")),
			"RecipeId": testField("RecipeId", basicTestType("string")),
		})
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{
			Fields: map[string]spec.Field{
				"RecipeId": {Target: "ID"},
			},
		})
		planner := &Planner{registry: newFunctionRegistry()}

		planner.planStruct(&typ)

		require.Len(t, typ.StructPlan.Fields, 1)
		assert.Equal(t, "RecipeId", typ.StructPlan.Fields[0].SourceField.Name)
		assert.Equal(t, "ID", typ.StructPlan.Fields[0].TargetField.Name)
		require.Len(t, typ.Diagnostics, 1)
		assert.Contains(t, typ.Diagnostics[0].Message, `target field "RecipeId"`)
	})

	t.Run("bubbles unsupported field diagnostics to the root plan and planner", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Stats", map[string]types.Field{
			"Count": testField("Count", basicTestType("string")),
		})
		targetDecl := testStructDecl("module.test/to", "Stats", map[string]types.Field{
			"Count": testField("Count", basicTestType("bool")),
		})
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{})
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		planner.registry = newFunctionRegistry()

		planner.planType(&typ)

		require.Len(t, typ.StructPlan.Fields, 1)
		assert.Equal(t, plan.OperationUnsupported, typ.StructPlan.Fields[0].Mapping.Operation)
		require.Len(t, typ.Diagnostics, 1)
		require.Len(t, planner.diagnostics, 1)
		assert.Equal(t, typ.Diagnostics[0], planner.diagnostics[0])
		assert.Equal(t, plan.DiagnosticLevelFatal, planner.diagnostics[0].Level)
	})

	t.Run("orders planned fields by target field name", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Thing", map[string]types.Field{
			"Beta":  testField("Beta", basicTestType("string")),
			"Alpha": testField("Alpha", basicTestType("string")),
		})
		targetDecl := testStructDecl("module.test/to", "Thing", map[string]types.Field{
			"Beta":  testField("Beta", basicTestType("string")),
			"Alpha": testField("Alpha", basicTestType("string")),
		})
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{})
		planner := &Planner{registry: newFunctionRegistry()}

		planner.planStruct(&typ)

		require.Len(t, typ.StructPlan.Fields, 2)
		assert.Equal(t, "Alpha", typ.StructPlan.Fields[0].TargetField.Name)
		assert.Equal(t, "Beta", typ.StructPlan.Fields[1].TargetField.Name)
	})

	t.Run("applies field optionality overrides", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Profile", map[string]types.Field{
			"Nickname": testField("Nickname", pointerTestType(basicTestType("string"))),
		})
		targetDecl := testStructDecl("module.test/to", "Profile", map[string]types.Field{
			"Nickname": testField("Nickname", basicTestType("string")),
		})
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{
			Fields: map[string]spec.Field{
				"Nickname": {
					Optionality: spec.Optionality{
						OnNilSourcePointer: spec.PointerOptionalityError,
						OnZeroSourceValue:  spec.ValueOptionalityAddress,
					},
				},
			},
		})
		planner := &Planner{registry: newFunctionRegistry()}

		planner.planStruct(&typ)

		field := requirePlanField(t, typ.StructPlan, "Nickname")
		assert.Equal(t, plan.OperationPointer, field.Mapping.Operation)
		assert.Equal(t, spec.PointerOptionalityError, field.Mapping.Optionality.OnNilSourcePointer)
		assert.Equal(t, spec.ValueOptionalityAddress, field.Mapping.Optionality.OnZeroSourceValue)
		assert.True(t, field.Mapping.CanError)
		assert.True(t, typ.CanError)
	})
}

func TestPlannerPlanCompositeDiagnostics(t *testing.T) {
	t.Run("keeps slice mapping supported when the element mapper only warns", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "WarningSliceContainer"})

		values := requirePlanField(t, root.StructPlan, "Values")

		assert.Equal(t, plan.OperationSlice, values.Mapping.Operation)
		require.NotNil(t, values.Mapping.Elem)
		assert.Equal(t, plan.OperationStruct, values.Mapping.Elem.Operation)
		require.Len(t, root.Diagnostics, 1)
		assert.Equal(t, plan.DiagnosticLevelWarning, root.Diagnostics[0].Level)
		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.Contains(t, root.Diagnostics[0].Path, "target field Extra")
	})

	t.Run("marks slice mapping unsupported when the element mapper is fatal", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"})

		values := requirePlanField(t, root.StructPlan, "Values")

		assert.Equal(t, plan.OperationUnsupported, values.Mapping.Operation)
		require.NotNil(t, values.Mapping.Elem)
		assert.Equal(t, plan.OperationUnsupported, values.Mapping.Elem.Operation)
		assert.True(t, plan.HasFatalDiagnostics(values.Mapping.Diagnostics))
		assert.True(t, plan.HasFatalDiagnostics(root.Diagnostics))
	})
}

func TestValidateStructFieldMappings(t *testing.T) {
	t.Run("reports missing source fields", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Recipe", map[string]types.Field{
			"DisplayName": testField("DisplayName", basicTestType("string")),
		})
		targetDecl := testStructDecl("module.test/to", "Recipe", map[string]types.Field{
			"Name": testField("Name", basicTestType("string")),
		})
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{
			Fields: map[string]spec.Field{
				"MissingName": {Target: "Name"},
			},
		})

		got := validateStructFieldMappings(&typ)
		require.Len(t, got, 1)

		assert.Equal(t, plan.DiagnosticLevelFatal, got[0].Level)
		assert.Equal(t, plan.SourceFieldPath(typ.SourceType, typ.TargetType, "MissingName"), got[0].Path)
		assert.Equal(t, `source field "MissingName" does not exist or is not plannable; fields must be exported and non-embedded`, got[0].Message)
	})

	t.Run("reports missing target fields", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Recipe", map[string]types.Field{
			"DisplayName": testField("DisplayName", basicTestType("string")),
		})
		targetDecl := testStructDecl("module.test/to", "Recipe", map[string]types.Field{
			"Name": testField("Name", basicTestType("string")),
		})
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{
			Fields: map[string]spec.Field{
				"DisplayName": {Target: "MissingName"},
			},
		})

		got := validateStructFieldMappings(&typ)
		require.Len(t, got, 1)

		assert.Equal(t, plan.DiagnosticLevelFatal, got[0].Level)
		assert.Equal(t, plan.TargetFieldPath(typ.SourceType, typ.TargetType, "MissingName"), got[0].Path)
		assert.Equal(t, `target field "MissingName" does not exist or is not plannable; fields must be exported and non-embedded`, got[0].Message)
	})

	t.Run("reports duplicate target fields", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Recipe", map[string]types.Field{
			"DisplayName":   testField("DisplayName", basicTestType("string")),
			"SecondaryName": testField("SecondaryName", basicTestType("string")),
		})
		targetDecl := testStructDecl("module.test/to", "Recipe", map[string]types.Field{
			"Name": testField("Name", basicTestType("string")),
		})
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{
			Fields: map[string]spec.Field{
				"DisplayName":   {Target: "Name"},
				"SecondaryName": {Target: "Name"},
			},
		})

		got := validateStructFieldMappings(&typ)
		require.Len(t, got, 1)

		assert.Equal(t, plan.DiagnosticLevelFatal, got[0].Level)
		assert.Equal(t, plan.TargetFieldPath(typ.SourceType, typ.TargetType, "Name"), got[0].Path)
		assert.Equal(t, `target field "Name" is mapped from multiple source fields ["DisplayName" "SecondaryName"]`, got[0].Message)
	})
}

func TestValidateGenericRoot(t *testing.T) {
	t.Run("reports source generic to target generic roots", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Box", nil)
		sourceDecl.Type.TypeParams = []types.TypeParam{{Name: "T", Constraint: basicTestType("any")}}
		targetDecl := testStructDecl("module.test/to", "Box", nil)
		targetDecl.Type.TypeParams = []types.TypeParam{{Name: "T", Constraint: basicTestType("any")}}
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{})

		got := validateGenericRoot(&typ)
		require.Len(t, got, 1)

		assert.Equal(t, plan.DiagnosticLevelFatal, got[0].Level)
		assert.Equal(t, plan.TypesPath(typ.SourceType, typ.TargetType), got[0].Path)
		assert.Equal(t, "generic root mappings are not supported; map concrete instantiations through containing types or provide a higher-order callable", got[0].Message)
	})

	t.Run("reports source generic to target non-generic roots", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Box", nil)
		sourceDecl.Type.TypeParams = []types.TypeParam{{Name: "T", Constraint: basicTestType("any")}}
		targetDecl := testStructDecl("module.test/to", "Box", nil)
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{})

		got := validateGenericRoot(&typ)
		require.Len(t, got, 1)

		assert.Equal(t, plan.DiagnosticLevelFatal, got[0].Level)
		assert.Equal(t, plan.TypesPath(typ.SourceType, typ.TargetType), got[0].Path)
		assert.Equal(t, "generic root mappings are not supported; map concrete instantiations through containing types or provide a higher-order callable", got[0].Message)
	})

	t.Run("reports source non-generic to target generic roots", func(t *testing.T) {
		sourceDecl := testStructDecl("module.test/from", "Box", nil)
		targetDecl := testStructDecl("module.test/to", "Box", nil)
		targetDecl.Type.TypeParams = []types.TypeParam{{Name: "T", Constraint: basicTestType("any")}}
		typ := testStructPlanType(sourceDecl, targetDecl, spec.Struct{})

		got := validateGenericRoot(&typ)
		require.Len(t, got, 1)

		assert.Equal(t, plan.DiagnosticLevelFatal, got[0].Level)
		assert.Equal(t, plan.TypesPath(typ.SourceType, typ.TargetType), got[0].Path)
		assert.Equal(t, "generic root mappings are not supported; map concrete instantiations through containing types or provide a higher-order callable", got[0].Message)
	})
}

func TestInvertStructSpec(t *testing.T) {
	t.Run("returns nil for nil struct specs", func(t *testing.T) {
		assert.Nil(t, invertStructSpec(nil))
	})

	t.Run("inverts source to target mappings", func(t *testing.T) {
		got := invertStructSpec(&spec.Struct{
			Fields: map[string]spec.Field{
				"RecipeId":    {Target: "ID"},
				"DisplayName": {Target: "Name"},
			},
		})

		require.NotNil(t, got)
		assert.Equal(t, map[string]spec.Field{
			"ID":   {Target: "RecipeId"},
			"Name": {Target: "DisplayName"},
		}, got.Fields)
	})
}

func TestPlannerPlanExplicitRoot(t *testing.T) {
	sourceType := namedTestType("module.test/from", "User")
	targetType := namedTestType("module.test/to", "User")
	sourceRef := plan.TypeRefFromType(sourceType)
	targetRef := plan.TypeRefFromType(targetType)
	sourceDecl := testStructDecl("module.test/from", "User", nil)
	targetDecl := testStructDecl("module.test/to", "User", nil)

	t.Run("does not panic for struct roots without enum plans", func(t *testing.T) {
		root := &plan.Type{
			Source:       sourceRef,
			Target:       targetRef,
			SourceDecl:   sourceDecl,
			TargetDecl:   targetDecl,
			SourceType:   sourceType,
			TargetType:   targetType,
			FunctionName: "MapUser",
			Signature:    defaultMapperSignature,
			StructPlan:   &plan.Struct{},
		}
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		require.NoError(t, planner.addExplicitRoot(outputGroups, testOutputLocation("module.test/mapping", "/repo/mapping/morph.gen.go"), root))

		require.NotPanics(t, func() {
			value, ok := planner.planExplicitRoot(sourceType, targetType)

			require.True(t, ok)
			assert.Equal(t, plan.OperationStruct, value.Operation)
			assert.Same(t, root, value.Plan)
		})
	})

	t.Run("chooses explicit root signatures deterministically", func(t *testing.T) {
		valueRoot := &plan.Type{
			Source:       sourceRef,
			Target:       targetRef,
			SourceDecl:   sourceDecl,
			TargetDecl:   targetDecl,
			SourceType:   sourceType,
			TargetType:   targetType,
			FunctionName: "MapUserValue",
			Signature:    defaultMapperSignature,
			StructPlan:   &plan.Struct{},
		}
		pointerRoot := &plan.Type{
			Source:       sourceRef,
			Target:       targetRef,
			SourceDecl:   sourceDecl,
			TargetDecl:   targetDecl,
			SourceType:   sourceType,
			TargetType:   targetType,
			FunctionName: "MapUserPointer",
			Signature: spec.MapperSignature{
				Accepts: spec.ParameterKindPointer,
				Returns: spec.ParameterKindValue,
			},
			StructPlan: &plan.Struct{},
		}
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("module.test/mapping", "/repo/mapping/morph.gen.go")
		require.NoError(t, planner.addExplicitRoot(outputGroups, location, pointerRoot))
		require.NoError(t, planner.addExplicitRoot(outputGroups, location, valueRoot))

		got := planner.explicitRoot(sourceType, targetType)

		assert.Same(t, valueRoot, got)
	})

	t.Run("prefers explicit roots in the current output package", func(t *testing.T) {
		currentLocation := testOutputLocation("module.test/current", "/repo/current/morph.gen.go")
		otherLocation := testOutputLocation("module.test/other", "/repo/other/morph.gen.go")
		currentRoot := testExplicitRootPlan(sourceRef, targetRef, sourceDecl, targetDecl, sourceType, targetType, "ZZZMapUser")
		otherRoot := testExplicitRootPlan(sourceRef, targetRef, sourceDecl, targetDecl, sourceType, targetType, "AAAMapUser")
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		require.NoError(t, planner.addExplicitRoot(outputGroups, otherLocation, otherRoot))
		require.NoError(t, planner.addExplicitRoot(outputGroups, currentLocation, currentRoot))
		planner.currentOutputLocation = &currentLocation

		got := planner.explicitRoot(sourceType, targetType)

		assert.Same(t, currentRoot, got)
	})

	t.Run("skips explicit roots that would create an import cycle", func(t *testing.T) {
		currentLocation := testOutputLocation("module.test/current", "/repo/current/morph.gen.go")
		unsafeLocation := testOutputLocation("module.test/unsafe", "/repo/unsafe/morph.gen.go")
		safeLocation := testOutputLocation("module.test/safe", "/repo/safe/morph.gen.go")
		unsafeRoot := testExplicitRootPlan(sourceRef, targetRef, sourceDecl, targetDecl, sourceType, targetType, "AAAMapUser")
		safeRoot := testExplicitRootPlan(sourceRef, targetRef, sourceDecl, targetDecl, sourceType, targetType, "ZZZMapUser")
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		require.NoError(t, planner.addExplicitRoot(outputGroups, unsafeLocation, unsafeRoot))
		require.NoError(t, planner.addExplicitRoot(outputGroups, safeLocation, safeRoot))
		planner.importGraph = importGraph{}
		planner.importGraph.addEdge(unsafeLocation.ImportPath, currentLocation.ImportPath)
		planner.currentOutputLocation = &currentLocation

		got := planner.explicitRoot(sourceType, targetType)

		assert.Same(t, safeRoot, got)
	})
}

func TestPlannerPlanBidirectionalPackageContext(t *testing.T) {
	engine := New("lab/planner")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Packages: []config.Package{{
			Source:        "github.com/seeruk/morph/lab/planner/from",
			Target:        "github.com/seeruk/morph/lab/planner/to",
			Bidirectional: new(true),
			Types: []config.Type{{
				Source: "Difficulty",
				Target: "RecipeDifficulty",
				Enum: &config.Enum{
					Values: map[string]string{
						"DifficultyUltra": "RecipeDifficultyInsane",
					},
				},
			}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 2)

	forward := requireRootByTargetName(t, out.OutputGroups[0].Roots, "RecipeDifficulty")
	inverse := requireRootByTargetName(t, out.OutputGroups[0].Roots, "Difficulty")

	assert.Equal(t, "github.com/seeruk/morph/lab/planner/from", forward.Source.ImportPath)
	assert.Equal(t, "github.com/seeruk/morph/lab/planner/to", forward.Target.ImportPath)
	assert.Equal(t, "github.com/seeruk/morph/lab/planner/to", inverse.Source.ImportPath)
	assert.Equal(t, "github.com/seeruk/morph/lab/planner/from", inverse.Target.ImportPath)
}

func TestPlannerPlanOutputScopedVariants(t *testing.T) {
	engine := New("lab/planner")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Packages: []config.Package{
			{
				Source: "github.com/seeruk/morph/lab/planner/from",
				Target: "github.com/seeruk/morph/lab/planner/to",
				Types: []config.Type{{
					Name: "Single",
					Mappers: &config.MappersDefaults{
						Forward: &config.MapperDefaults{Name: new("MapSingleInMapping")},
					},
				}},
			},
			{
				Source: "github.com/seeruk/morph/lab/planner/from",
				Target: "github.com/seeruk/morph/lab/planner/to",
				Output: config.Output{Strategy: new(spec.OutputStrategySourcePackage)},
				Types: []config.Type{{
					Name: "Single",
					Mappers: &config.MappersDefaults{
						Forward: &config.MapperDefaults{Name: new("MapSingleInSource")},
					},
				}},
			},
		},
	}), "morph.yaml")

	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 2)

	var functionNames []string
	for _, outputGroup := range out.OutputGroups {
		require.Len(t, outputGroup.Roots, 1)
		functionNames = append(functionNames, outputGroup.Roots[0].FunctionName)
	}
	assert.ElementsMatch(t, []string{"MapSingleInMapping", "MapSingleInSource"}, functionNames)
}

func TestPlannerPlanRecursiveNestedStructs(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "Container",
			}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	root := out.OutputGroups[0].Roots[0]
	require.NotNil(t, root.StructPlan)
	require.Len(t, root.StructPlan.Fields, 2)

	first := root.StructPlan.Fields[0].Mapping.Plan
	second := root.StructPlan.Fields[1].Mapping.Plan
	require.NotNil(t, first)
	require.NotNil(t, second)
	assert.Same(t, first, second)
	require.NotNil(t, first.StructPlan)

	var next plan.Field
	for _, field := range first.StructPlan.Fields {
		if field.TargetField.Name == "Next" {
			next = field
			break
		}
	}
	require.NotEmpty(t, next.TargetField.Name)
	require.NotNil(t, next.Mapping.Elem)
	assert.Same(t, first, next.Mapping.Elem.Plan)
}

func TestPlannerPlanGenericNestedStructs(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "GenericContainer",
			}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	root := out.OutputGroups[0].Roots[0]
	require.NotNil(t, root.StructPlan)

	intBox := requirePlanField(t, root.StructPlan, "IntBox")
	stringBox := requirePlanField(t, root.StructPlan, "StringBox")
	require.NotNil(t, intBox.Mapping.Plan)
	require.NotNil(t, stringBox.Mapping.Plan)

	assert.NotSame(t, stringBox.Mapping.Plan, intBox.Mapping.Plan)
	assert.NotEqual(t, stringBox.Mapping.Plan.FunctionName, intBox.Mapping.Plan.FunctionName)
	assert.NotEqual(t, stringBox.Mapping.Plan.Source.Key, intBox.Mapping.Plan.Source.Key)
	assert.NotEqual(t, stringBox.Mapping.Plan.Target.Key, intBox.Mapping.Plan.Target.Key)

	intValue := requirePlanField(t, intBox.Mapping.Plan.StructPlan, "Value")
	assert.Equal(t, plan.OperationConvert, intValue.Mapping.Operation)
	assert.Equal(t, types.TypeKindBasic, intValue.Mapping.Source.Kind)
	assert.Equal(t, "int", intValue.Mapping.Source.Name)
	assert.Equal(t, types.TypeKindBasic, intValue.Mapping.Target.Kind)
	assert.Equal(t, "int64", intValue.Mapping.Target.Name)

	stringValue := requirePlanField(t, stringBox.Mapping.Plan.StructPlan, "Value")
	assert.Equal(t, plan.OperationAssign, stringValue.Mapping.Operation)
	assert.Equal(t, types.TypeKindBasic, stringValue.Mapping.Source.Kind)
	assert.Equal(t, "string", stringValue.Mapping.Source.Name)
	assert.Equal(t, types.TypeKindBasic, stringValue.Mapping.Target.Kind)
	assert.Equal(t, "string", stringValue.Mapping.Target.Name)
}

func TestPlannerPlanConversions(t *testing.T) {
	userIDToString := config.Conversion{
		Source: spec.TypeRef{ImportPath: plannerFromPackage, Name: "UserID"},
		Targets: []spec.TypeRef{
			{Name: "string"},
		},
	}
	int64ToInt := spec.Conversion{
		Source: spec.TypeRef{Name: "int64"},
		Target: spec.TypeRef{Name: "int"},
	}

	t.Run("keeps basic conversions automatic", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"})

		count := requirePlanField(t, root.StructPlan, "Count")

		assert.Equal(t, plan.OperationConvert, count.Mapping.Operation)
	})

	t.Run("keeps unregistered lossy numeric conversions unsupported", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		planner.registry = newFunctionRegistry()

		got := planner.planValue(
			basicTestType("int64"),
			basicTestType("int"),
			"Count",
			defaultOptionality(),
			defaultConversionsPolicy(),
			nil,
		)

		assert.Equal(t, plan.OperationUnsupported, got.Operation)
		require.Len(t, got.Diagnostics, 1)
		assert.Equal(t, plan.DiagnosticLevelFatal, got.Diagnostics[0].Level)
	})

	t.Run("uses registered lossy numeric conversions", func(t *testing.T) {
		planner := NewPlanner(Spec{Conversions: []spec.Conversion{int64ToInt}}, ".", "morph.yaml")
		planner.registry = newFunctionRegistry()

		got := planner.planValue(
			basicTestType("int64"),
			basicTestType("int"),
			"Count",
			defaultOptionality(),
			defaultConversionsPolicy(),
			nil,
		)

		assert.Equal(t, plan.OperationConvert, got.Operation)
	})

	t.Run("keeps registered impossible conversions unsupported", func(t *testing.T) {
		planner := NewPlanner(Spec{Conversions: []spec.Conversion{{
			Source: spec.TypeRef{Name: "bool"},
			Target: spec.TypeRef{Name: "int"},
		}}}, ".", "morph.yaml")
		planner.registry = newFunctionRegistry()

		got := planner.planValue(
			basicTestType("bool"),
			basicTestType("int"),
			"Enabled",
			defaultOptionality(),
			defaultConversionsPolicy(),
			nil,
		)

		assert.Equal(t, plan.OperationUnsupported, got.Operation)
	})

	t.Run("uses bidirectional registered lossy numeric conversions", func(t *testing.T) {
		engine := New(".")
		out, err := engine.Plan(resolveTestConfig(t, config.Config{
			Conversions: []config.Conversion{{
				Source:        spec.TypeRef{Name: "int"},
				Targets:       []spec.TypeRef{{Name: "int64"}},
				Bidirectional: true,
			}},
			Packages: []config.Package{{
				Source:        plannerFromPackage,
				Target:        plannerToPackage,
				Bidirectional: new(true),
				Types: []config.Type{{
					Name: "ConversionContainer",
				}},
			}},
		}), "morph.yaml")
		require.NoError(t, err)
		require.Len(t, out.OutputGroups, 1)
		require.Len(t, out.OutputGroups[0].Roots, 2)

		inverse := requireRootBySourcePackage(t, out.OutputGroups[0].Roots, plannerToPackage)
		count := requirePlanField(t, inverse.StructPlan, "Count")

		assert.Equal(t, plan.OperationConvert, count.Mapping.Operation)
		assert.Equal(t, "int64", count.Mapping.Source.Name)
		assert.Equal(t, "int", count.Mapping.Target.Name)
	})

	t.Run("requires registry for named conversions", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"})

		id := requirePlanField(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationUnsupported, id.Mapping.Operation)
	})

	t.Run("uses registered named conversions", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"}, userIDToString)

		id := requirePlanField(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationConvert, id.Mapping.Operation)
	})

	t.Run("unwraps aliases before matching registry pairs", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"}, userIDToString)

		alias := requirePlanField(t, root.StructPlan, "Alias")

		assert.Equal(t, plan.OperationConvert, alias.Mapping.Operation)
	})

	t.Run("uses registry conversions inside slices", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"}, userIDToString)

		values := requirePlanField(t, root.StructPlan, "Values")
		require.NotNil(t, values.Mapping.Elem)

		assert.Equal(t, plan.OperationConvert, values.Mapping.Elem.Operation)
	})

	t.Run("disables registry conversions at type scope", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name:        "ConversionContainer",
			Conversions: &config.ConversionsDefaults{Enabled: new(false)},
		}, userIDToString)

		id := requirePlanField(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationUnsupported, id.Mapping.Operation)
	})

	t.Run("re-enables registry conversions at field scope", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name:        "ConversionsPolicyContainer",
			Conversions: &config.ConversionsDefaults{Enabled: new(false)},
			Struct: &config.Struct{
				Fields: map[string]config.Field{
					"ID": {Conversions: &config.ConversionsDefaults{Enabled: new(true)}},
				},
			},
		}, userIDToString)

		id := requirePlanField(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationConvert, id.Mapping.Operation)
	})

	t.Run("keeps disabled registry conversions disabled for other fields", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name:        "ConversionsPolicyContainer",
			Conversions: &config.ConversionsDefaults{Enabled: new(false)},
			Struct: &config.Struct{
				Fields: map[string]config.Field{
					"ID": {Conversions: &config.ConversionsDefaults{Enabled: new(true)}},
				},
			},
		}, userIDToString)

		other := requirePlanField(t, root.StructPlan, "Other")

		assert.Equal(t, plan.OperationUnsupported, other.Mapping.Operation)
	})

	t.Run("does not raw convert registered struct pairs", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "StructConversionContainer"}, config.Conversion{
			Source: spec.TypeRef{ImportPath: plannerFromPackage, Name: "StructCode"},
			Targets: []spec.TypeRef{{
				ImportPath: plannerToPackage,
				Name:       "StructCode",
			}},
		})

		code := requirePlanField(t, root.StructPlan, "Code")

		assert.Equal(t, plan.OperationStruct, code.Mapping.Operation)
	})
}

func TestPlannerPlanGenericRoots(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types:  []config.Type{{Name: "NumberContainer"}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	root := out.OutputGroups[0].Roots[0]
	assert.Empty(t, root.TypeParams)
	assert.True(t, plan.HasFatalDiagnostics(root.Diagnostics))
	require.NotEmpty(t, root.Diagnostics)
	assert.Equal(t, "generic root mappings are not supported; map concrete instantiations through containing types or provide a higher-order callable", root.Diagnostics[0].Message)
}

func TestPlannerPlanNestedStructsWithScopedCallables(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{
				{
					Name: "ScopedStringBoxA",
					Callables: []spec.CallableRef{{
						ImportPath: "github.com/seeruk/morph/testdata/planner/from",
						Name:       "TypeAStringToInt",
					}},
				},
				{
					Name: "ScopedStringBoxB",
					Callables: []spec.CallableRef{{
						ImportPath: "github.com/seeruk/morph/testdata/planner/from",
						Name:       "TypeBStringToInt",
					}},
				},
			},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	rootA := requireRootByTargetName(t, out.OutputGroups[0].Roots, "ScopedStringBoxA")
	rootB := requireRootByTargetName(t, out.OutputGroups[0].Roots, "ScopedStringBoxB")

	boxA := requirePlanField(t, rootA.StructPlan, "Box").Mapping.Plan
	boxB := requirePlanField(t, rootB.StructPlan, "Box").Mapping.Plan
	require.NotNil(t, boxA)
	require.NotNil(t, boxB)

	assert.NotSame(t, boxA, boxB)
	assert.NotEqual(t, boxA.FunctionName, boxB.FunctionName)

	valueA := requirePlanField(t, boxA.StructPlan, "Value")
	valueB := requirePlanField(t, boxB.StructPlan, "Value")
	require.NotNil(t, valueA.Mapping.Callable)
	require.NotNil(t, valueB.Mapping.Callable)
	assert.Equal(t, "TypeAStringToInt", valueA.Mapping.Callable.Name)
	assert.Equal(t, "TypeBStringToInt", valueB.Mapping.Callable.Name)
}

func TestPlannerPlanDefaultCallable(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Defaults: config.Defaults{
			Packages: config.PackagesDefaults{
				Types: config.TypesDefaults{
					Callables: []spec.CallableRef{{
						ImportPath: "github.com/seeruk/morph/testdata/planner/from",
						Name:       "ExplicitStringToInt",
					}},
				},
			},
		},
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types:  []config.Type{{Name: "ExplicitCallableContainer"}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	root := out.OutputGroups[0].Roots[0]
	field := requirePlanField(t, root.StructPlan, "Value")

	require.NotNil(t, field.Mapping.Callable)
	assert.Equal(t, "ExplicitStringToInt", field.Mapping.Callable.Name)
}

func TestPlannerPlanMethodCallable(t *testing.T) {
	t.Run("ignores unreferenced methods", func(t *testing.T) {
		engine := New(".")
		out, err := engine.Plan(resolveTestConfig(t, config.Config{
			Packages: []config.Package{{
				Source: "github.com/seeruk/morph/testdata/planner/from",
				Target: "github.com/seeruk/morph/testdata/planner/to",
				Types:  []config.Type{{Name: "MethodCallableContainer"}},
			}},
		}), "morph.yaml")

		require.NoError(t, err)
		root := out.OutputGroups[0].Roots[0]
		field := requirePlanField(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationUnsupported, field.Mapping.Operation)
	})

	t.Run("uses explicitly referenced methods", func(t *testing.T) {
		engine := New(".")
		out, err := engine.Plan(resolveTestConfig(t, config.Config{
			Defaults: config.Defaults{
				Packages: config.PackagesDefaults{
					Types: config.TypesDefaults{
						Callables: []spec.CallableRef{{
							ImportPath: "github.com/seeruk/morph/testdata/planner/from",
							TypeName:   "MethodID",
							Name:       "String",
						}},
					},
				},
			},
			Packages: []config.Package{{
				Source: "github.com/seeruk/morph/testdata/planner/from",
				Target: "github.com/seeruk/morph/testdata/planner/to",
				Types:  []config.Type{{Name: "MethodCallableContainer"}},
			}},
		}), "morph.yaml")

		require.NoError(t, err)
		root := out.OutputGroups[0].Roots[0]
		field := requirePlanField(t, root.StructPlan, "ID")

		require.NotNil(t, field.Mapping.Callable)
		assert.Equal(t, plan.OperationMethod, field.Mapping.Operation)
		assert.Equal(t, "String", field.Mapping.Callable.Name)
	})
}

func TestPlannerPlanHigherOrderGenericFunction(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Discovery: config.Discovery{
			Packages: []string{
				"github.com/seeruk/morph/testdata/planner/from",
			},
		},
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "OptionalContainer",
			}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	root := out.OutputGroups[0].Roots[0]
	require.NotNil(t, root.StructPlan)

	maybe := requirePlanField(t, root.StructPlan, "Maybe")
	require.NotNil(t, maybe.Mapping.Callable)
	require.Len(t, maybe.Mapping.CallableArgs, 1)
	arg := maybe.Mapping.CallableArgs[0]
	require.NotNil(t, arg.Mapping.Plan)
	require.NotNil(t, arg.Mapping.Plan.StructPlan)

	t.Run("selects the discovered container function", func(t *testing.T) {
		assert.Equal(t, plan.OperationFunction, maybe.Mapping.Operation)
		assert.Equal(t, "MapOptional", maybe.Mapping.Callable.Name)
		assert.False(t, maybe.Mapping.CanError)
	})

	t.Run("plans the mapper argument", func(t *testing.T) {
		assert.False(t, arg.ReturnsError)
		assert.Equal(t, plan.OperationStruct, arg.Mapping.Operation)
		assert.Equal(t, "OptionalThing", arg.Mapping.Source.Name)
		assert.Equal(t, "OptionalThing", arg.Mapping.Target.Name)
	})

	t.Run("plans the mapper argument fields", func(t *testing.T) {
		name := requirePlanField(t, arg.Mapping.Plan.StructPlan, "Name")
		assert.Equal(t, plan.OperationAssign, name.Mapping.Operation)
	})
}

func TestPlannerPlanHigherOrderGenericFunctionWithUnplannableArg(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Discovery: config.Discovery{
			Packages: []string{
				"github.com/seeruk/morph/testdata/planner/from",
			},
		},
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "OptionalBadContainer",
			}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 1)
	root := out.OutputGroups[0].Roots[0]
	require.NotNil(t, root.StructPlan)

	maybe := requirePlanField(t, root.StructPlan, "Maybe")
	assert.True(t, plan.HasFatalDiagnostics(root.Diagnostics))
	assert.True(t, plan.HasFatalDiagnostics(maybe.Mapping.Diagnostics))
	assert.Contains(t, diagnosticsMessages(root.Diagnostics), "callable argument 1 could not map github.com/seeruk/morph/testdata/planner/from.OptionalBadThing to github.com/seeruk/morph/testdata/planner/to.OptionalBadThing; configure a compatible callable, explicit mapper, discovery package, or registered conversion")
	assert.Contains(t, diagnosticsPaths(root.Diagnostics), plan.FieldPath(root.SourceType, root.TargetType, maybe.SourceField, maybe.TargetField)+" :: callable argument 1")
}

func TestPlannerPlanHigherOrderExplicitCallable(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Defaults: config.Defaults{
			Packages: config.PackagesDefaults{
				Types: config.TypesDefaults{
					Callables: []spec.CallableRef{{
						ImportPath: "github.com/seeruk/morph/testdata/planner/from",
						Name:       "MapOptional",
					}},
				},
			},
		},
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types:  []config.Type{{Name: "OptionalContainer"}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	root := out.OutputGroups[0].Roots[0]
	maybe := requirePlanField(t, root.StructPlan, "Maybe")

	require.NotNil(t, maybe.Mapping.Callable)
	assert.Equal(t, "MapOptional", maybe.Mapping.Callable.Name)
	require.Len(t, maybe.Mapping.CallableArgs, 1)
	assert.Equal(t, plan.OperationStruct, maybe.Mapping.CallableArgs[0].Mapping.Operation)
}

func TestPlannerPlanHigherOrderGenericFunctionWithErrors(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Discovery: config.Discovery{
			Packages: []string{
				"github.com/seeruk/morph/testdata/planner/from",
			},
		},
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "FallibleOptionalContainer",
			}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	root := out.OutputGroups[0].Roots[0]
	require.NotNil(t, root.StructPlan)

	maybe := requirePlanField(t, root.StructPlan, "Maybe")
	require.NotNil(t, maybe.Mapping.Callable)
	require.Len(t, maybe.Mapping.CallableArgs, 1)
	arg := maybe.Mapping.CallableArgs[0]
	require.NotNil(t, arg.Mapping.Callable)

	t.Run("propagates errors to the root and field mapping", func(t *testing.T) {
		assert.True(t, root.CanError)
		assert.True(t, maybe.Mapping.CanError)
	})

	t.Run("selects the discovered erroring container function", func(t *testing.T) {
		assert.Equal(t, plan.OperationFunction, maybe.Mapping.Operation)
		assert.Equal(t, "MapFallibleOptional", maybe.Mapping.Callable.Name)
		assert.True(t, maybe.Mapping.Callable.ReturnsError)
	})

	t.Run("plans the erroring mapper argument", func(t *testing.T) {
		assert.True(t, arg.ReturnsError)
		assert.Equal(t, plan.OperationFunction, arg.Mapping.Operation)
		assert.Equal(t, "MapFallibleThing", arg.Mapping.Callable.Name)
		assert.True(t, arg.Mapping.Callable.ReturnsError)
		assert.True(t, arg.Mapping.CanError)
	})
}

func TestPlannerPlanHigherOrderGenericFunctionWithMultipleTypeArgs(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Discovery: config.Discovery{
			Packages: []string{
				"github.com/seeruk/morph/testdata/planner/from",
			},
		},
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "EitherContainer",
			}},
		}},
	}), "morph.yaml")

	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	root := out.OutputGroups[0].Roots[0]
	require.NotNil(t, root.StructPlan)

	result := requirePlanField(t, root.StructPlan, "Result")
	require.NotNil(t, result.Mapping.Callable)
	require.Len(t, result.Mapping.CallableArgs, 2)

	leftArg := result.Mapping.CallableArgs[0]
	require.NotNil(t, leftArg.Mapping.Plan)
	require.NotNil(t, leftArg.Mapping.Plan.StructPlan)

	rightArg := result.Mapping.CallableArgs[1]
	require.NotNil(t, rightArg.Mapping.Plan)
	require.NotNil(t, rightArg.Mapping.Plan.StructPlan)

	t.Run("selects the discovered container function", func(t *testing.T) {
		assert.Equal(t, plan.OperationFunction, result.Mapping.Operation)
		assert.Equal(t, "MapEither", result.Mapping.Callable.Name)
		assert.False(t, result.Mapping.CanError)
	})

	t.Run("plans the left mapper argument", func(t *testing.T) {
		assert.False(t, leftArg.ReturnsError)
		assert.Equal(t, plan.OperationStruct, leftArg.Mapping.Operation)
		assert.Equal(t, "EitherLeft", leftArg.Mapping.Source.Name)
		assert.Equal(t, "EitherLeft", leftArg.Mapping.Target.Name)

		leftCode := requirePlanField(t, leftArg.Mapping.Plan.StructPlan, "Code")
		assert.Equal(t, plan.OperationAssign, leftCode.Mapping.Operation)
	})

	t.Run("plans the right mapper argument", func(t *testing.T) {
		assert.False(t, rightArg.ReturnsError)
		assert.Equal(t, plan.OperationStruct, rightArg.Mapping.Operation)
		assert.Equal(t, "EitherRight", rightArg.Mapping.Source.Name)
		assert.Equal(t, "EitherRight", rightArg.Mapping.Target.Name)

		rightName := requirePlanField(t, rightArg.Mapping.Plan.StructPlan, "Name")
		assert.Equal(t, plan.OperationAssign, rightName.Mapping.Operation)
	})
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

func plannerWithCallables(fns ...types.FunctionDecl) *Planner {
	callables := make(map[spec.CallableRef]registeredCallable, len(fns))
	for _, fn := range fns {
		callables[spec.CallableRefFromFunctionDecl(fn)] = registeredCallable{Function: &fn}
	}
	return &Planner{
		registry:  newFunctionRegistry(),
		callables: callables,
	}
}

func callableTier(tier spec.CallableTier, fns ...types.FunctionDecl) spec.TieredCallables {
	callables := make([]spec.CallableRef, 0, len(fns))
	for _, fn := range fns {
		callables = append(callables, spec.CallableRefFromFunctionDecl(fn))
	}
	return spec.TieredCallables{
		Tier:      tier,
		Callables: callables,
	}
}

const (
	plannerFromPackage = "github.com/seeruk/morph/testdata/planner/from"
	plannerToPackage   = "github.com/seeruk/morph/testdata/planner/to"
)

func planPlannerRoot(t *testing.T, typ config.Type, conversions ...config.Conversion) *plan.Type {
	t.Helper()

	engine := New(".")
	out, err := engine.Plan(resolveTestConfig(t, config.Config{
		Conversions: conversions,
		Packages: []config.Package{{
			Source: plannerFromPackage,
			Target: plannerToPackage,
			Types:  []config.Type{typ},
		}},
	}), "morph.yaml")
	require.NoError(t, err)
	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	return out.OutputGroups[0].Roots[0]
}

func resolveTestConfig(t *testing.T, cfg config.Config) Spec {
	t.Helper()

	resolved, err := ResolveConfig(cfg)
	require.NoError(t, err)
	return resolved
}

func requireRootByTargetName(t *testing.T, roots []*plan.Type, targetName string) *plan.Type {
	t.Helper()

	for _, root := range roots {
		if root.Target.Name == targetName {
			return root
		}
	}

	require.Failf(t, "root not planned", "target root %q was not planned", targetName)
	return nil
}

func requireRootBySourcePackage(t *testing.T, roots []*plan.Type, sourcePackage string) *plan.Type {
	t.Helper()

	for _, root := range roots {
		if root.Source.ImportPath == sourcePackage {
			return root
		}
	}

	require.Failf(t, "root not planned", "source package root %q was not planned", sourcePackage)
	return nil
}

func requirePlanField(t *testing.T, structPlan *plan.Struct, targetName string) plan.Field {
	t.Helper()

	require.NotNil(t, structPlan)
	for _, field := range structPlan.Fields {
		if field.TargetField.Name == targetName {
			return field
		}
	}

	require.Failf(t, "field not planned", "target field %q was not planned", targetName)
	return plan.Field{}
}

func diagnosticsMessages(diagnostics []plan.Diagnostic) []string {
	out := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, diagnostic.Message)
	}
	return out
}

func diagnosticsPaths(diagnostics []plan.Diagnostic) []string {
	out := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, diagnostic.Path)
	}
	return out
}

func testStructPlanType(sourceDecl, targetDecl types.TypeDecl, structSpec spec.Struct) plan.Type {
	return plan.Type{
		Source:      plan.TypeRefFromTypeDecl(sourceDecl),
		Target:      plan.TypeRefFromTypeDecl(targetDecl),
		SourceDecl:  sourceDecl,
		TargetDecl:  targetDecl,
		SourceType:  sourceDecl.Type,
		TargetType:  targetDecl.Type,
		Signature:   defaultMapperSignature,
		StructSpec:  structSpec,
		Optionality: defaultOptionality(),
		Conversions: defaultConversionsPolicy(),
	}
}

func testExplicitRootPlan(
	sourceRef, targetRef plan.TypeRef,
	sourceDecl, targetDecl types.TypeDecl,
	sourceType, targetType types.Type,
	functionName string,
) *plan.Type {
	return &plan.Type{
		Source:       sourceRef,
		Target:       targetRef,
		SourceDecl:   sourceDecl,
		TargetDecl:   targetDecl,
		SourceType:   sourceType,
		TargetType:   targetType,
		FunctionName: functionName,
		Signature:    defaultMapperSignature,
		StructPlan:   &plan.Struct{},
		Optionality:  defaultOptionality(),
		Conversions:  defaultConversionsPolicy(),
	}
}

func testStructDecl(importPath, name string, fields map[string]types.Field) types.TypeDecl {
	structType := types.Type{
		Kind:   types.TypeKindStruct,
		String: "struct{}",
		Fields: fields,
	}

	return types.TypeDecl{
		Name: name,
		Package: types.PackageRef{
			Name:       "test",
			ImportPath: importPath,
		},
		Type: types.Type{
			Kind:    types.TypeKindNamed,
			Name:    name,
			Package: types.PackageRef{Name: "test", ImportPath: importPath},
			Elem:    &structType,
		},
		Underlying: structType,
		Fields:     fields,
	}
}

func testField(name string, typ types.Type) types.Field {
	return types.Field{
		Name:       name,
		Type:       typ,
		IsExported: true,
	}
}
