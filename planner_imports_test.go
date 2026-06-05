package morph

import (
	"testing"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratedImportRequirements(t *testing.T) {
	t.Run("dedupes repeated requirements and ignores same-package imports", func(t *testing.T) {
		external := namedTestType("module.test/external", "Thing")

		refs := typeImportRequirementSites(
			"module.test/out",
			"Values",
			"container target type",
			mapTestType(external, external),
			nil,
			nil,
		)
		refs = dedupeImportRequirementSites(refs)

		require.Len(t, refs, 1)
		assert.Equal(t, "module.test/out", refs[0].Requirement.From)
		assert.Equal(t, "module.test/external", refs[0].Requirement.To)

		refs = typeImportRequirementSites(
			"module.test/external",
			"Values",
			"container target type",
			external,
			nil,
			nil,
		)
		refs = dedupeImportRequirementSites(refs)

		assert.Empty(t, refs)
	})

	t.Run("reports import cycles against the owning generated type", func(t *testing.T) {
		location := testOutputLocation("module.test/out", "/repo/out/morph.gen.go")
		source := namedTestType("module.test/external", "Thing")
		target := basicTestType("string")
		root := &plan.Type{
			SourceType:   source,
			TargetType:   target,
			FunctionName: "MapThing",
			Location:     location,
		}
		planner := newTestAttemptPlanner(Spec{}, ".", "morph.yaml")
		planner.importGraph = importGraph{}
		planner.importGraph.AddEdge("module.test/external", "module.test/out")

		planner.validateFinalPlan([]plan.OutputGroup{{
			Location: location,
			Roots:    []*plan.Type{root},
		}})

		require.Len(t, planner.diagnostics, 1)
		require.Len(t, root.Diagnostics, 1)
		assert.Equal(t, planner.diagnostics[0], root.Diagnostics[0])
		assert.Contains(t, planner.diagnostics[0].Message, `mapper "MapThing" source signature`)
		assert.Contains(t, planner.diagnostics[0].Message, `generated package "module.test/out" to import "module.test/external"`)
		assert.Contains(t, planner.diagnostics[0].Message, "would create an import cycle")
	})

	t.Run("validates nested mapper signature imports", func(t *testing.T) {
		location := testOutputLocation("module.test/out", "/repo/out/morph.gen.go")
		source := namedTestType("module.test/external", "Nested")
		target := basicTestType("string")
		nested := &plan.Type{
			SourceType:   source,
			TargetType:   target,
			FunctionName: "MapNested",
			Location:     location,
		}
		planner := newTestAttemptPlanner(Spec{}, ".", "morph.yaml")
		planner.importGraph = importGraph{}
		planner.importGraph.AddEdge("module.test/external", "module.test/out")

		planner.validateFinalPlan([]plan.OutputGroup{{
			Location: location,
			Nested:   []*plan.Type{nested},
		}})

		require.Len(t, planner.diagnostics, 1)
		require.Len(t, nested.Diagnostics, 1)
		assert.Contains(t, planner.diagnostics[0].Message, `mapper "MapNested" source signature`)
	})

	t.Run("validates nested mapper target signature imports", func(t *testing.T) {
		location := testOutputLocation("module.test/out", "/repo/out/morph.gen.go")
		source := basicTestType("string")
		target := namedTestType("module.test/external", "Nested")
		nested := &plan.Type{
			SourceType:   source,
			TargetType:   target,
			FunctionName: "MapNested",
			Location:     location,
		}
		planner := newTestAttemptPlanner(Spec{}, ".", "morph.yaml")
		planner.importGraph = importGraph{}
		planner.importGraph.AddEdge("module.test/external", "module.test/out")

		planner.validateFinalPlan([]plan.OutputGroup{{
			Location: location,
			Nested:   []*plan.Type{nested},
		}})

		require.Len(t, planner.diagnostics, 1)
		require.Len(t, nested.Diagnostics, 1)
		assert.Contains(t, planner.diagnostics[0].Message, `mapper "MapNested" target signature`)
	})
}

func TestGeneratedImportValidationForValues(t *testing.T) {
	location := testOutputLocation("module.test/out", "/repo/out/morph.gen.go")
	sourceRoot := namedTestType("module.test/source", "Root")
	targetRoot := namedTestType("module.test/target", "Root")
	argSource := namedTestType("module.test/argsource", "Thing")
	argTarget := namedTestType("module.test/argtarget", "Thing")
	callable := plan.CallableRef{
		Kind:    plan.CallableKindFunction,
		Package: argSource.Package,
		Name:    "MapOptional",
	}
	value := plan.Value{
		Operation: plan.OperationFunction,
		Source:    basicTestType("string"),
		Target:    basicTestType("string"),
		Callable:  &callable,
		CallableArgs: []plan.CallableArg{{
			Mapping: plan.Value{
				Operation: plan.OperationAssign,
				Source:    argSource,
				Target:    argTarget,
			},
		}},
	}
	root := &plan.Type{
		SourceType:   sourceRoot,
		TargetType:   targetRoot,
		FunctionName: "MapRoot",
		Location:     location,
		StructPlan: &plan.Struct{Fields: []plan.Field{{
			SourceField: testField("Value", basicTestType("string")),
			TargetField: testField("Value", basicTestType("string")),
			Mapping:     value,
		}}},
	}
	planner := newTestAttemptPlanner(Spec{}, ".", "morph.yaml")
	planner.importGraph = importGraph{}
	planner.importGraph.AddEdge("module.test/argsource", "module.test/out")
	planner.importGraph.AddEdge("module.test/argtarget", "module.test/out")

	planner.validateFinalPlan([]plan.OutputGroup{{
		Location: location,
		Roots:    []*plan.Type{root},
	}})

	require.Len(t, planner.diagnostics, 3)
	messages := diagnosticsMessages(planner.diagnostics)
	assert.Contains(t, messages, `function callable "MapOptional" requires generated package "module.test/out" to import "module.test/argsource", but generated import "module.test/out" -> "module.test/argsource" would create an import cycle; existing path: module.test/argsource -> module.test/out`)
	assert.Contains(t, messages, `callable argument 1 source type requires generated package "module.test/out" to import "module.test/argsource", but generated import "module.test/out" -> "module.test/argsource" would create an import cycle; existing path: module.test/argsource -> module.test/out`)
	assert.Contains(t, messages, `callable argument 1 target type requires generated package "module.test/out" to import "module.test/argtarget", but generated import "module.test/out" -> "module.test/argtarget" would create an import cycle; existing path: module.test/argtarget -> module.test/out`)
}

func testFunctionDeclInPackage(
	importPath string,
	packageName string,
	name string,
	param types.Type,
	result types.Type,
	extraResults ...types.Type,
) types.FunctionDecl {
	fn := testFunctionDecl(name, param, result, extraResults...)
	fn.Package = types.PackageRef{Name: packageName, ImportPath: importPath}
	return fn
}
