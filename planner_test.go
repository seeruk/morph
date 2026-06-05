package morph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanner_addRoot(t *testing.T) {
	t.Run("should register root state when adding a new mapper", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		mapperKey := plan.TypeMapperKey(root.Source, root.Target, root.Signature)
		pairKey := plan.TypePairKey(root.Source, root.Target)
		variantKey := rootVariantKey(location, root)

		err := planner.addRoot(outputGroups, location, root)
		require.NoError(t, err)
		require.Len(t, outputGroups[location].Roots, 1)
		require.Len(t, planner.rootVariantsByTypePair[pairKey], 1)

		variant := planner.rootVariantsByCallable[spec.CallableRef{
			ImportPath: location.ImportPath,
			Name:       root.FunctionName,
		}]
		require.NotNil(t, variant)

		assert.Same(t, root, outputGroups[location].Roots[0])
		assert.Same(t, root, planner.rootVariantsByTypePair[pairKey][0].Root)
		assert.Same(t, root, planner.mappings[variantKey])
		assert.Equal(t, mapperKey, plan.TypeMapperKey(variant.Root.Source, variant.Root.Target, variant.Root.Signature))
		_, ok := planner.plannedOutputFiles[filepath.Clean(location.LogicalPath)]
		assert.True(t, ok)
	})

	t.Run("should not append a duplicate root when mapper and function match", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		duplicate := testPlanType("source.User", "target.User", "MapUser")

		require.NoError(t, planner.addRoot(outputGroups, location, root))

		err := planner.addRoot(outputGroups, location, duplicate)
		require.NoError(t, err)

		require.Len(t, outputGroups[location].Roots, 1)
		assert.Same(t, root, outputGroups[location].Roots[0])
	})

	t.Run("should treat empty and omitted mapper configuration as matching", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		duplicate := testPlanType("source.User", "target.User", "MapUser")
		duplicate.StructSpec = spec.Struct{Fields: map[string]spec.Field{}}
		duplicate.EnumSpec = spec.Enum{
			Patterns: spec.EnumPatterns{},
			Values:   map[string]string{},
		}

		require.NoError(t, planner.addRoot(outputGroups, location, root))

		err := planner.addRoot(outputGroups, location, duplicate)
		require.NoError(t, err)

		require.Len(t, outputGroups[location].Roots, 1)
		assert.Same(t, root, outputGroups[location].Roots[0])
	})

	t.Run("should error when the same mapper has different configuration", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		conflicting := testPlanType("source.User", "target.User", "MapUser")
		conflicting.StructSpec = spec.Struct{
			Fields: map[string]spec.Field{
				"UserId": {Target: "ID"},
			},
		}

		require.NoError(t, planner.addRoot(outputGroups, location, root))

		err := planner.addRoot(outputGroups, location, conflicting)
		require.Error(t, err)

		assert.ErrorContains(t, err, "conflicting mapper configuration")
	})

	t.Run("should allow the same mapper with different function names", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		variant := testPlanType("source.User", "target.User", "MapUserDifferently")

		require.NoError(t, planner.addRoot(outputGroups, location, root))

		err := planner.addRoot(outputGroups, location, variant)
		require.NoError(t, err)

		require.Len(t, outputGroups[location].Roots, 2)
		assert.Same(t, root, outputGroups[location].Roots[0])
		assert.Same(t, variant, outputGroups[location].Roots[1])
	})

	t.Run("should error when different mappers use the same function name in one package", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapThing")
		conflicting := testPlanType("source.Group", "target.Group", "MapThing")

		require.NoError(t, planner.addRoot(outputGroups, location, root))

		err := planner.addRoot(outputGroups, location, conflicting)
		require.Error(t, err)

		assert.ErrorContains(t, err, "function name \"MapThing\" is planned for both")
	})

	t.Run("should allow the same function name in different packages", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		firstLocation := testOutputLocation("github.com/seeruk/morph/one", "/repo/one/morph.gen.go")
		secondLocation := testOutputLocation("github.com/seeruk/morph/two", "/repo/two/morph.gen.go")
		firstRoot := testPlanType("source.User", "target.User", "MapThing")
		secondRoot := testPlanType("source.Group", "target.Group", "MapThing")

		require.NoError(t, planner.addRoot(outputGroups, firstLocation, firstRoot))

		err := planner.addRoot(outputGroups, secondLocation, secondRoot)
		require.NoError(t, err)

		assert.Len(t, outputGroups[firstLocation].Roots, 1)
		assert.Len(t, outputGroups[secondLocation].Roots, 1)
	})

	t.Run("should plan separate variants when the same mapper is added in different packages", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
		firstLocation := testOutputLocation("github.com/seeruk/morph/one", "/repo/one/morph.gen.go")
		secondLocation := testOutputLocation("github.com/seeruk/morph/two", "/repo/two/morph.gen.go")
		root := testPlanType("source.User", "target.User", "MapUser")
		duplicate := testPlanType("source.User", "target.User", "MapUser")

		require.NoError(t, planner.addRoot(outputGroups, firstLocation, root))

		err := planner.addRoot(outputGroups, secondLocation, duplicate)
		require.NoError(t, err)

		require.Len(t, outputGroups[firstLocation].Roots, 1)
		require.Len(t, outputGroups[secondLocation].Roots, 1)
		assert.Same(t, root, outputGroups[firstLocation].Roots[0])
		assert.Same(t, duplicate, outputGroups[secondLocation].Roots[0])
	})
}

func TestPlanner_isFunctionPendingGeneration(t *testing.T) {
	planner := NewPlanner(Spec{}, ".", "morph.yaml")
	outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
	location := testOutputLocation("github.com/seeruk/morph/out", "/repo/out/morph.gen.go")
	root := testPlanType("source.User", "target.User", "MapUser")

	require.NoError(t, planner.addRoot(outputGroups, location, root))

	tests := []struct {
		name string
		fn   types.FunctionDecl
		ref  spec.CallableRef
		want bool
	}{
		{
			name: "should return true when the root function ref is planned",
			fn:   types.FunctionDecl{SourceFile: "/repo/other.go"},
			ref:  spec.CallableRef{ImportPath: location.ImportPath, Name: root.FunctionName},
			want: true,
		},
		{
			name: "should return true when the source file is planned for output, including nested mappers",
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

func TestPlanner_prepareImportGraph(t *testing.T) {
	t.Run("should allow single package and source package outputs for the same package pair", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		mappingLocation := testOutputLocation("module.test/mapping", "/repo/mapping/morph.gen.go")
		sourceLocation := testOutputLocation("module.test/source", "/repo/source/morph.gen.go")
		root := testPlanTypeWithPackages("module.test/source", "User", "module.test/target", "User", "MapUser")

		outputGroups := map[plan.OutputLocation]plan.OutputGroup{
			mappingLocation: {Location: mappingLocation, Roots: []*plan.Type{root}},
			sourceLocation:  {Location: sourceLocation, Roots: []*plan.Type{root}},
		}

		err := planner.prepareImportGraph(outputGroups)

		require.NoError(t, err)
	})

	t.Run("should reject source package and target package outputs for the same package pair", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		sourceLocation := testOutputLocation("module.test/source", "/repo/source/morph.gen.go")
		targetLocation := testOutputLocation("module.test/target", "/repo/target/morph.gen.go")
		root := testPlanTypeWithPackages("module.test/source", "User", "module.test/target", "User", "MapUser")

		outputGroups := map[plan.OutputLocation]plan.OutputGroup{
			sourceLocation: {Location: sourceLocation, Roots: []*plan.Type{root}},
			targetLocation: {Location: targetLocation, Roots: []*plan.Type{root}},
		}

		err := planner.prepareImportGraph(outputGroups)

		require.Error(t, err)
		assert.ErrorContains(t, err, "would create an import cycle")
		assert.ErrorContains(t, err, "existing path: module.test/source -> module.test/target")
	})

	t.Run("should reject output that reverses an existing import", func(t *testing.T) {
		planner := NewPlanner(Spec{}, ".", "morph.yaml")
		planner.importGraph = importGraph{}
		graph := importGraph{}
		graph.addEdge("module.test/source", "module.test/target")

		err := graph.addGeneratedImport("module.test/target", "module.test/source")

		require.Error(t, err)
		assert.ErrorContains(t, err, "would create an import cycle")
		assert.ErrorContains(t, err, "existing path: module.test/source -> module.test/target")
	})
}

func TestSortedOutputGroups(t *testing.T) {
	t.Run("should order groups by location", func(t *testing.T) {
		locationB := testOutputLocation("module.test/b", "/repo/b/morph.gen.go")
		locationA := testOutputLocation("module.test/a", "/repo/a/morph.gen.go")

		got := sortedOutputGroups(map[plan.OutputLocation]plan.OutputGroup{
			locationB: {Location: locationB},
			locationA: {Location: locationA},
		})

		require.Len(t, got, 2)
		assert.Equal(t, locationA, got[0].Location)
		assert.Equal(t, locationB, got[1].Location)
	})
}

func TestPackageNameFromDir(t *testing.T) {
	t.Run("should return false when directory does not exist", func(t *testing.T) {
		name, ok, err := packageNameFromDir(filepath.Join(t.TempDir(), "missing"))

		require.NoError(t, err)
		assert.Empty(t, name)
		assert.False(t, ok)
	})

	t.Run("should return false when directory has no package files", func(t *testing.T) {
		dir := t.TempDir()
		writePlannerTestFile(t, dir, "README.md", "# no package here\n")

		name, ok, err := packageNameFromDir(dir)

		require.NoError(t, err)
		assert.Empty(t, name)
		assert.False(t, ok)
	})

	t.Run("should ignore test package files", func(t *testing.T) {
		dir := t.TempDir()
		writePlannerTestFile(t, dir, "thing_test.go", "package example_test\n")

		name, ok, err := packageNameFromDir(dir)

		require.NoError(t, err)
		assert.Empty(t, name)
		assert.False(t, ok)
	})

	t.Run("should return package name from go files", func(t *testing.T) {
		dir := t.TempDir()
		writePlannerTestFile(t, dir, "thing.go", "package example\n")
		writePlannerTestFile(t, dir, "other.go", "package example\n")

		name, ok, err := packageNameFromDir(dir)

		require.NoError(t, err)
		assert.Equal(t, "example", name)
		assert.True(t, ok)
	})

	t.Run("should return error for invalid go files", func(t *testing.T) {
		dir := t.TempDir()
		writePlannerTestFile(t, dir, "broken.go", "package \n")

		name, ok, err := packageNameFromDir(dir)

		require.Error(t, err)
		assert.Empty(t, name)
		assert.False(t, ok)
		assert.ErrorContains(t, err, "parse package clause")
	})

	t.Run("should return error for multiple package names", func(t *testing.T) {
		dir := t.TempDir()
		writePlannerTestFile(t, dir, "one.go", "package one\n")
		writePlannerTestFile(t, dir, "two.go", "package two\n")

		name, ok, err := packageNameFromDir(dir)

		require.Error(t, err)
		assert.Empty(t, name)
		assert.False(t, ok)
		assert.ErrorContains(t, err, "multiple packages found")
	})
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
		Optionality:  defaultOptionality(),
		Conversions:  defaultConversionsPolicy(),
	}
}

func testPlanTypeWithPackages(sourcePackage, sourceName, targetPackage, targetName, functionName string) *plan.Type {
	root := testPlanType(sourcePackage+"."+sourceName, targetPackage+"."+targetName, functionName)
	root.Source.ImportPath = sourcePackage
	root.Source.Name = sourceName
	root.Target.ImportPath = targetPackage
	root.Target.Name = targetName
	return root
}

func testMapperSignature() spec.MapperSignature {
	return spec.MapperSignature{
		Accepts: spec.ParameterKindValue,
		Returns: spec.ParameterKindValue,
	}
}

func writePlannerTestFile(t *testing.T, dir, name, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}
