package morph

import (
	"testing"

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
			Fields: map[string]string{
				"RecipeId":    "ID",
				"DisplayName": "Name",
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
			Fields: map[string]string{
				"RecipeId": "ID",
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
}

func TestInvertStructSpec(t *testing.T) {
	t.Run("returns nil for nil struct specs", func(t *testing.T) {
		assert.Nil(t, invertStructSpec(nil))
	})

	t.Run("inverts source to target mappings", func(t *testing.T) {
		got := invertStructSpec(&spec.Struct{
			Fields: map[string]string{
				"RecipeId":    "ID",
				"DisplayName": "Name",
			},
		})

		require.NotNil(t, got)
		assert.Equal(t, map[string]string{
			"ID":   "RecipeId",
			"Name": "DisplayName",
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
		key := plan.TypeMapperKey(root.Source, root.Target, root.Signature)
		planner.explicitRoots[key] = root
		planner.mappings[key] = root

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
				Accepts: new(spec.ParameterKindPointer),
				Returns: new(spec.ParameterKindValue),
			},
			StructPlan: &plan.Struct{},
		}
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		valueKey := plan.TypeMapperKey(valueRoot.Source, valueRoot.Target, valueRoot.Signature)
		pointerKey := plan.TypeMapperKey(pointerRoot.Source, pointerRoot.Target, pointerRoot.Signature)
		planner.explicitRoots[pointerKey] = pointerRoot
		planner.explicitRoots[valueKey] = valueRoot
		planner.mappings[pointerKey] = pointerRoot
		planner.mappings[valueKey] = valueRoot

		got := planner.explicitRoot(sourceType, targetType)

		assert.Same(t, valueRoot, got)
	})
}

func TestPlannerPlanRecursiveNestedStructs(t *testing.T) {
	engine := New(".")
	out, err := engine.Plan(Spec{
		Packages: []spec.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []spec.Type{{
				Name: "Container",
			}},
		}},
	}, "morph.yaml")

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

func testStructPlanType(sourceDecl, targetDecl types.TypeDecl, structSpec spec.Struct) plan.Type {
	return plan.Type{
		Source:     plan.TypeRefFromTypeDecl(sourceDecl),
		Target:     plan.TypeRefFromTypeDecl(targetDecl),
		SourceDecl: sourceDecl,
		TargetDecl: targetDecl,
		SourceType: sourceDecl.Type,
		TargetType: targetDecl.Type,
		Signature:  defaultMapperSignature,
		StructSpec: structSpec,
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
