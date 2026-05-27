package morph

import (
	"path/filepath"
	"testing"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanner_addExplicitRoot(t *testing.T) {
	t.Run("should register root state when adding a new mapper", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		mapperKey := plan.TypeMapperKey(root.Source, root.Target, root.Signature)

		err := planner.addExplicitRoot(outputGroups, location, root)
		require.NoError(t, err)
		require.Len(t, outputGroups[location].Roots, 1)

		assert.Same(t, root, outputGroups[location].Roots[0])
		assert.Same(t, root, planner.explicitRoots[mapperKey])
		assert.Same(t, root, planner.mappings[mapperKey])
		assert.Equal(t, mapperKey, planner.plannedFunctions[spec.CallableRef{
			ImportPath: location.ImportPath,
			Name:       root.FunctionName,
		}])
		_, ok := planner.plannedOutputFiles[filepath.Clean(location.LogicalPath)]
		assert.True(t, ok)
	})

	t.Run("should not append a duplicate root when mapper and function match", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		duplicate := testPlanType("source.User", "target.User", "MapUser")

		require.NoError(t, planner.addExplicitRoot(outputGroups, location, root))

		err := planner.addExplicitRoot(outputGroups, location, duplicate)
		require.NoError(t, err)

		require.Len(t, outputGroups[location].Roots, 1)
		assert.Same(t, root, outputGroups[location].Roots[0])
	})

	t.Run("should error when the same mapper has different function names", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		conflicting := testPlanType("source.User", "target.User", "MapUserDifferently")

		require.NoError(t, planner.addExplicitRoot(outputGroups, location, root))

		err := planner.addExplicitRoot(outputGroups, location, conflicting)
		require.Error(t, err)

		assert.ErrorContains(t, err, "conflicting mapper names")
	})

	t.Run("should error when different mappers use the same function name in one package", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapThing")
		conflicting := testPlanType("source.Group", "target.Group", "MapThing")

		require.NoError(t, planner.addExplicitRoot(outputGroups, location, root))

		err := planner.addExplicitRoot(outputGroups, location, conflicting)
		require.Error(t, err)

		assert.ErrorContains(t, err, "function name \"MapThing\" is planned for both")
	})

	t.Run("should allow the same function name in different packages", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		firstLocation := testOutputLocation("github.com/seeruk/morph/one", "/repo/one/morph.gen.go")
		secondLocation := testOutputLocation("github.com/seeruk/morph/two", "/repo/two/morph.gen.go")
		firstRoot := testPlanType("source.User", "target.User", "MapThing")
		secondRoot := testPlanType("source.Group", "target.Group", "MapThing")

		require.NoError(t, planner.addExplicitRoot(outputGroups, firstLocation, firstRoot))

		err := planner.addExplicitRoot(outputGroups, secondLocation, secondRoot)
		require.NoError(t, err)

		assert.Len(t, outputGroups[firstLocation].Roots, 1)
		assert.Len(t, outputGroups[secondLocation].Roots, 1)
	})

	t.Run("should reuse the canonical root when the same mapper is added in different packages", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		firstLocation := testOutputLocation("github.com/seeruk/morph/one", "/repo/one/morph.gen.go")
		secondLocation := testOutputLocation("github.com/seeruk/morph/two", "/repo/two/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		duplicate := testPlanType("source.User", "target.User", "MapUser")

		require.NoError(t, planner.addExplicitRoot(outputGroups, firstLocation, root))

		err := planner.addExplicitRoot(outputGroups, secondLocation, duplicate)
		require.NoError(t, err)

		require.Len(t, outputGroups[firstLocation].Roots, 1)
		require.Len(t, outputGroups[secondLocation].Roots, 1)
		assert.Same(t, root, outputGroups[firstLocation].Roots[0])
		assert.Same(t, root, outputGroups[secondLocation].Roots[0])
	})
}

func TestPlanner_isFunctionPendingGeneration(t *testing.T) {
	planner := NewPlanner(Spec{}, ".")
	outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
	location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
	root := testPlanType("source.User", "target.User", "MapUser")

	require.NoError(t, planner.addExplicitRoot(outputGroups, location, root))

	tests := []struct {
		name string
		fn   types.FunctionDecl
		ref  spec.CallableRef
		want bool
	}{
		{
			name: "should return true when the function ref is planned",
			fn:   types.FunctionDecl{SourceFile: "/repo/other.go"},
			ref:  spec.CallableRef{ImportPath: location.ImportPath, Name: root.FunctionName},
			want: true,
		},
		{
			name: "should return true when the source file is planned for output",
			fn:   types.FunctionDecl{SourceFile: location.LogicalPath},
			ref:  spec.CallableRef{ImportPath: location.ImportPath, Name: "OtherFunction"},
			want: true,
		},
		{
			name: "should return false when the function and source file are not planned",
			fn:   types.FunctionDecl{SourceFile: "/repo/other.go"},
			ref:  spec.CallableRef{ImportPath: location.ImportPath, Name: "OtherFunction"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, planner.isFunctionPendingGeneration(tt.fn, tt.ref))
		})
	}
}

func testOutputLocation(importPath, logicalPath string) plan.OutputLocation {
	return plan.OutputLocation{
		LogicalPath: logicalPath,
		ImportPath:  importPath,
		PackageName: "out",
	}
}

func testPlanType(sourceKey, targetKey, functionName string) *plan.Type {
	return &plan.Type{
		Source: plan.TypeRef{
			Name: sourceKey,
			Key:  sourceKey,
		},
		Target: plan.TypeRef{
			Name: targetKey,
			Key:  targetKey,
		},
		FunctionName: functionName,
		Signature:    testMapperSignature(),
	}
}

func testMapperSignature() spec.MapperSignature {
	accepts := spec.ParameterKindValue
	returns := spec.ParameterKindValue
	return spec.MapperSignature{
		Accepts: &accepts,
		Returns: &returns,
	}
}
