package morph

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/seeruk/morph/config"
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlannerPlanStructFieldsFromConfig(t *testing.T) {
	t.Run("uses explicit field mappings", func(t *testing.T) {
		out := planWithConfig(t, "lab/planner", config.Config{
			Conversions: []config.Conversion{labStringyConversion()},
			Packages: []config.Package{{
				Source: "github.com/seeruk/morph/lab/planner/from",
				Target: "github.com/seeruk/morph/lab/planner/to",
				Types: []config.Type{{
					Name: "Single",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"Foo": {Target: "Bla"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		bla := requirePlanField(t, root.StructPlan, "Bla")

		assert.Equal(t, "Foo", bla.SourceField.Name)
		assert.Equal(t, plan.OperationConvert, bla.Mapping.Operation)
	})

	t.Run("reports invalid configured fields", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Types: []config.Type{{
					Name: "ConversionContainer",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"MissingSource": {Target: "ID"},
						"ID":            {Target: "MissingTarget"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		assertFatalDiagnosticMessages(
			t, root.Diagnostics,
			`source field "MissingSource" does not exist or is not plannable; fields must be exported and non-embedded`,
			`target field "MissingTarget" does not exist or is not plannable; fields must be exported and non-embedded`,
		)
	})

	t.Run("reports duplicate target fields", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Types: []config.Type{{
					Name: "ConversionContainer",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"ID":     {Target: "ID"},
						"Secret": {Target: "ID"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)

		assertFatalDiagnosticMessages(t, root.Diagnostics, `target field "ID" is mapped from multiple source fields ["ID" "Secret"]`)
	})

	t.Run("applies field optionality overrides", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Types: []config.Type{{
					Name: "Node",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"Required": {Optionality: pointerErrorOptionalityDefaults()},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		required := requirePlanField(t, root.StructPlan, "Required")

		assert.True(t, root.CanError)
		assert.Equal(t, []plan.ValueAdaptation{plan.ValueAdaptationDeref}, required.Mapping.SourceAdaptations)
		assert.True(t, required.Mapping.CanError)
	})
}

func TestPlannerPlanCallableSelectionFromConfig(t *testing.T) {
	t.Run("uses type callables before defaults", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Defaults: config.Defaults{
				Packages: config.PackagesDefaults{
					Types: config.TypesDefaults{
						Callables: []spec.CallableRef{{ImportPath: plannerFromPackage, Name: "ExplicitStringToInt"}},
					},
				},
			},
			Packages: []config.Package{{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Types: []config.Type{{
					Name:      "ExplicitCallableContainer",
					Callables: []spec.CallableRef{{ImportPath: plannerFromPackage, Name: "TypeAStringToInt"}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		value := requirePlanField(t, root.StructPlan, "Value")
		require.NotNil(t, value.Mapping.Callable)

		assert.Equal(t, "TypeAStringToInt", value.Mapping.Callable.Name)
	})

	t.Run("falls back from incompatible scoped callables to discovery", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Discovery: config.Discovery{Packages: []string{plannerFromPackage}},
			Packages: []config.Package{{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Types: []config.Type{{
					Name:      "ExplicitCallableContainer",
					Callables: []spec.CallableRef{{ImportPath: plannerFromPackage, Name: "MapOptional"}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		value := requirePlanField(t, root.StructPlan, "Value")
		require.NotNil(t, value.Mapping.Callable)

		assert.Equal(t, plan.CallableSourceDiscovered, value.Mapping.Callable.Source)
	})

	t.Run("records callable input adaptations", func(t *testing.T) {
		out := planWithConfig(t, "lab/planner", config.Config{
			Packages: []config.Package{{
				Source:        "github.com/seeruk/morph/lab/planner/from",
				Target:        "github.com/seeruk/morph/lab/planner/to",
				Bidirectional: new(true),
				Types: []config.Type{{
					Name: "Explicit",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"Foo": {
							Callable: &config.FieldCallable{Inverse: &spec.CallableRef{
								ImportPath: "github.com/seeruk/morph/lab/planner/from",
								Name:       "OptionalOfString2",
							}},
						},
					}},
				}},
			}},
		})

		inverse := requireRootBySourcePackage(t, out.OutputGroups[0].Roots, "github.com/seeruk/morph/lab/planner/to")
		foo := requirePlanField(t, inverse.StructPlan, "Foo")

		assert.Equal(t, plan.OperationFunction, foo.Mapping.Operation)
		assert.Equal(t, []plan.ValueAdaptation{plan.ValueAdaptationAddress}, foo.Mapping.SourceAdaptations)
	})
}

func TestPlannerPlanCollectionsFromConfig(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
		Conversions: []config.Conversion{plannerUserIDToStringConversion()},
		Packages: []config.Package{{
			Source: plannerFromPackage,
			Target: plannerToPackage,
			Types:  []config.Type{{Name: "CollectionContainer"}},
		}},
	})

	root := requireSingleRoot(t, out)
	values := requirePlanField(t, root.StructPlan, "Values")
	codes := requirePlanField(t, root.StructPlan, "Codes")
	lookup := requirePlanField(t, root.StructPlan, "Lookup")

	require.NotNil(t, values.Mapping.Elem)
	require.NotNil(t, codes.Mapping.Elem)
	require.NotNil(t, lookup.Mapping.Key)
	require.NotNil(t, lookup.Mapping.Value)

	t.Run("plans slice elements", func(t *testing.T) {
		assert.Equal(t, plan.OperationSlice, values.Mapping.Operation)
		assert.Equal(t, plan.OperationConvert, values.Mapping.Elem.Operation)
	})

	t.Run("plans array elements", func(t *testing.T) {
		assert.Equal(t, plan.OperationArray, codes.Mapping.Operation)
		assert.Equal(t, plan.OperationConvert, codes.Mapping.Elem.Operation)
	})

	t.Run("plans map keys and values", func(t *testing.T) {
		assert.Equal(t, plan.OperationMap, lookup.Mapping.Operation)
		assert.Equal(t, plan.OperationConvert, lookup.Mapping.Key.Operation)
		assert.Equal(t, plan.OperationConvert, lookup.Mapping.Value.Operation)
	})
}

func TestPlannerPlanEnumsFromConfig(t *testing.T) {
	t.Run("maps all normalized source constants", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Types:  []config.Type{{Name: "Status"}},
			}},
		})

		root := requireSingleRoot(t, out)
		require.NotNil(t, root.EnumPlan)
		require.Len(t, root.EnumPlan.Values, 2)

		assert.Equal(t, []string{"OK", "StatusOK"}, enumSourceNames(root.EnumPlan.Values))
		assert.Equal(t, []string{"OK", "OK"}, enumTargetNames(root.EnumPlan.Values))
	})

	t.Run("reports invalid explicit value mappings", func(t *testing.T) {
		out := planWithConfig(t, "lab/planner", config.Config{
			Packages: []config.Package{{
				Source: "github.com/seeruk/morph/lab/planner/from",
				Target: "github.com/seeruk/morph/lab/planner/to",
				Types: []config.Type{{
					Source: "Difficulty",
					Target: "RecipeDifficulty",
					Enum: &config.Enum{Values: map[string]string{
						"DifficultyMissing": "RecipeDifficultyEasy",
						"DifficultyEasy":    "RecipeDifficultyMissing",
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)

		assertFatalDiagnosticMessages(
			t, root.Diagnostics,
			`source enum value "DifficultyMissing" does not exist or is not exported`,
			`target enum value "RecipeDifficultyMissing" configured for source enum value "DifficultyEasy" does not exist or is not exported`,
		)
	})

	t.Run("reports missing inferred target matches", func(t *testing.T) {
		out := planWithConfig(t, "lab/planner", config.Config{
			Packages: []config.Package{{
				Source: "github.com/seeruk/morph/lab/planner/from",
				Target: "github.com/seeruk/morph/lab/planner/to",
				Types: []config.Type{{
					Source: "Difficulty",
					Target: "RecipeDifficulty",
				}},
			}},
		})

		root := requireSingleRoot(t, out)

		assertFatalDiagnosticMessages(t, root.Diagnostics, `no target enum value matched source enum value "DifficultyUltra" normalized as "ULTRA"; configure enum.values or enum.patterns`)
	})

	t.Run("respects zero failure mode", func(t *testing.T) {
		zero := spec.EnumFailureModeZero
		out := planWithConfig(t, "lab/planner", config.Config{
			Packages: []config.Package{{
				Source: "github.com/seeruk/morph/lab/planner/from",
				Target: "github.com/seeruk/morph/lab/planner/to",
				Types: []config.Type{{
					Source: "Difficulty",
					Target: "RecipeDifficulty",
					Enum: &config.Enum{
						FailureMode: &zero,
						Values: map[string]string{
							"DifficultyUltra": "RecipeDifficultyInsane",
						},
					},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		require.NotNil(t, root.EnumPlan)

		assert.False(t, root.CanError)
		assert.Equal(t, spec.EnumFailureModeZero, root.EnumPlan.FailureMode)
	})
}

func TestSortedOutputGroups(t *testing.T) {
	t.Run("orders groups by location", func(t *testing.T) {
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
	t.Run("returns false when directory does not exist", func(t *testing.T) {
		name, ok, err := packageNameFromDir(filepath.Join(t.TempDir(), "missing"))

		require.NoError(t, err)
		assert.Empty(t, name)
		assert.False(t, ok)
	})

	t.Run("returns false when directory has no package files", func(t *testing.T) {
		dir := t.TempDir()
		writePlannerTestFile(t, dir, "README.md", "# no package here\n")

		name, ok, err := packageNameFromDir(dir)

		require.NoError(t, err)
		assert.Empty(t, name)
		assert.False(t, ok)
	})

	t.Run("ignores test package files", func(t *testing.T) {
		dir := t.TempDir()
		writePlannerTestFile(t, dir, "thing_test.go", "package example_test\n")

		name, ok, err := packageNameFromDir(dir)

		require.NoError(t, err)
		assert.Empty(t, name)
		assert.False(t, ok)
	})

	t.Run("returns package name from go files", func(t *testing.T) {
		dir := t.TempDir()
		writePlannerTestFile(t, dir, "thing.go", "package example\n")
		writePlannerTestFile(t, dir, "other.go", "package example\n")

		name, ok, err := packageNameFromDir(dir)

		require.NoError(t, err)
		assert.Equal(t, "example", name)
		assert.True(t, ok)
	})

	t.Run("returns error for invalid go files", func(t *testing.T) {
		dir := t.TempDir()
		writePlannerTestFile(t, dir, "broken.go", "package \n")

		name, ok, err := packageNameFromDir(dir)

		require.Error(t, err)
		assert.Empty(t, name)
		assert.False(t, ok)
		assert.ErrorContains(t, err, "parse package clause")
	})

	t.Run("returns error for multiple package names", func(t *testing.T) {
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

func newTestAttemptPlanner(specification Spec, workingDir, ident string) *attemptPlanner {
	return NewPlanner(specification, workingDir, ident).newAttempt(nil)
}

func planWithConfig(t *testing.T, workingDir string, cfg config.Config) Plan {
	t.Helper()

	planner := NewPlanner(resolveTestConfig(t, cfg), workingDir, "morph.yaml")
	out, err := planner.Plan()
	require.NoError(t, err)
	return out
}

func resolveTestConfig(t *testing.T, cfg config.Config) Spec {
	t.Helper()

	resolved, err := ResolveConfig(cfg)
	require.NoError(t, err)
	return resolved
}

func requireSingleRoot(t *testing.T, out Plan) *plan.Type {
	t.Helper()

	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)
	return out.OutputGroups[0].Roots[0]
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

func plannerUserIDToStringConversion() config.Conversion {
	return config.Conversion{
		Source: spec.TypeRef{ImportPath: plannerFromPackage, Name: "UserID"},
		Targets: []spec.TypeRef{
			{Name: "string"},
		},
	}
}

func labStringyConversion() config.Conversion {
	return config.Conversion{
		Source: spec.TypeRef{ImportPath: "github.com/seeruk/morph/lab/planner/to", Name: "Stringy"},
		Targets: []spec.TypeRef{
			{Name: "string"},
		},
		Bidirectional: true,
	}
}

func enumSourceNames(values []plan.EnumValue) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.Source.Name)
	}
	return out
}

func enumTargetNames(values []plan.EnumValue) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, value.Target.Name)
	}
	return out
}

func assertFatalDiagnosticMessages(t *testing.T, diagnostics []plan.Diagnostic, messages ...string) {
	t.Helper()

	assert.True(t, plan.HasFatalDiagnostics(diagnostics))
	got := diagnosticsMessages(diagnostics)
	for _, message := range messages {
		assert.Contains(t, got, message)
	}
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

func writePlannerTestFile(t *testing.T, dir, name, content string) {
	t.Helper()

	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}
