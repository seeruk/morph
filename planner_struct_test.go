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

func TestPlannerPlanBidirectionalPackageContext(t *testing.T) {
	out := planWithConfig(t, "lab/planner", config.Config{
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
	})

	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 2)

	forward := requireRootByTargetName(t, out.OutputGroups[0].Roots, "RecipeDifficulty")
	inverse := requireRootByTargetName(t, out.OutputGroups[0].Roots, "Difficulty")

	assert.Equal(t, "github.com/seeruk/morph/lab/planner/from", forward.Source.ImportPath)
	assert.Equal(t, "github.com/seeruk/morph/lab/planner/to", forward.Target.ImportPath)
	assert.Equal(t, "github.com/seeruk/morph/lab/planner/to", inverse.Source.ImportPath)
	assert.Equal(t, "github.com/seeruk/morph/lab/planner/from", inverse.Target.ImportPath)
}

func TestPlannerPlanRootVisibility(t *testing.T) {
	t.Run("allows unexported root types from the generated package", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilitySourcePackage,
				Output: config.Output{Strategy: new(spec.OutputStrategySourcePackage)},
				Types: []config.Type{{
					Name: "samePackageUnexportedRoot",
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
	})

	t.Run("rejects unexported source types from another generated package", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategyTargetPackage)},
				Types: []config.Type{{
					Source: "unexportedSourceRoot",
					Target: "UnexportedSourceRoot",
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`mapper "MapSourceunexportedSourceRootToTargetUnexportedSourceRoot" source signature references unexported type "unexportedSourceRoot" from package "github.com/seeruk/morph/testdata/visibility/source", but generated package "github.com/seeruk/morph/testdata/visibility/target" cannot name it; unexported types can only be named from their declaring package`,
		)
	})

	t.Run("rejects unexported target types from another generated package", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategySourcePackage)},
				Types: []config.Type{{
					Name: "unexportedBothRoot",
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`mapper "MapSourceunexportedBothRootToTargetunexportedBothRoot" target signature references unexported type "unexportedBothRoot" from package "github.com/seeruk/morph/testdata/visibility/target", but generated package "github.com/seeruk/morph/testdata/visibility/source" cannot name it; unexported types can only be named from their declaring package`,
		)
	})
}

func TestPlannerPlanFieldVisibility(t *testing.T) {
	t.Run("allows unexported source fields from the generated package", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategySourcePackage)},
				Types: []config.Type{{
					Name: "LowerSourceFieldContainer",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"secret": {Target: "Secret"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		field := requirePlanField(t, root.StructPlan, "Secret")

		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.Equal(t, "secret", field.SourceField.Name)
	})

	t.Run("allows unexported target fields from the generated package", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategyTargetPackage)},
				Types: []config.Type{{
					Name: "LowerTargetFieldContainer",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"Secret": {Target: "secret"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		field := requirePlanField(t, root.StructPlan, "secret")

		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.Equal(t, "Secret", field.SourceField.Name)
	})

	t.Run("maps accessible unexported fields automatically", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilitySourcePackage,
				Output: config.Output{Strategy: new(spec.OutputStrategySourcePackage)},
				Types: []config.Type{{
					Source: "SamePackageSecretSource",
					Target: "SamePackageSecretTarget",
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		field := requirePlanField(t, root.StructPlan, "secret")

		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.Equal(t, "secret", field.SourceField.Name)
	})

	t.Run("skips inaccessible unexported fields during automatic matching", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategySourcePackage)},
				Types: []config.Type{{
					Name: "LowerTargetFieldContainer",
				}},
			}},
		})

		root := requireSingleRoot(t, out)

		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.Empty(t, root.StructPlan.Fields)
	})

	t.Run("rejects explicit unexported source fields from another generated package", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategyTargetPackage)},
				Types: []config.Type{{
					Name: "LowerSourceFieldContainer",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"secret": {Target: "Secret"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`source field "secret" is not accessible from generated package "github.com/seeruk/morph/testdata/visibility/target"; unexported fields can only be mapped from their declaring package "github.com/seeruk/morph/testdata/visibility/source"`,
		)
	})

	t.Run("rejects explicit unexported target fields from another generated package", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategySourcePackage)},
				Types: []config.Type{{
					Name: "LowerTargetFieldContainer",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"Secret": {Target: "secret"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`target field "secret" is not accessible from generated package "github.com/seeruk/morph/testdata/visibility/source"; unexported fields can only be mapped from their declaring package "github.com/seeruk/morph/testdata/visibility/target"`,
		)
	})

	t.Run("continues to reject embedded fields", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategySourcePackage)},
				Types: []config.Type{{
					Name: "EmbeddedFieldContainer",
					Struct: &config.Struct{Fields: map[string]config.Field{
						"ID": {Target: "ID"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		assertFatalDiagnosticMessages(t, root.Diagnostics, `source field "ID" is embedded; embedded fields are not supported`)
	})
}

func TestPlannerPlanOutputScopedVariants(t *testing.T) {
	out := planWithConfig(t, "lab/planner", config.Config{
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
	})

	require.Len(t, out.OutputGroups, 2)

	var functionNames []string
	for _, outputGroup := range out.OutputGroups {
		require.Len(t, outputGroup.Roots, 1)
		functionNames = append(functionNames, outputGroup.Roots[0].FunctionName)
	}
	assert.ElementsMatch(t, []string{"MapSingleInMapping", "MapSingleInSource"}, functionNames)
}

func TestPlannerPlanRecursiveNestedStructs(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "Container",
			}},
		}},
	})

	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)
	require.Len(t, out.OutputGroups[0].Nested, 1)

	root := requireSingleRoot(t, out)
	assert.Equal(t, out.OutputGroups[0].Location, root.Location)
	require.NotNil(t, root.StructPlan)
	require.Len(t, root.StructPlan.Fields, 2)

	first := requireFieldPlan(t, root.StructPlan, "First")
	second := requireFieldPlan(t, root.StructPlan, "Second")
	assert.Same(t, first, second)
	assert.Same(t, first, out.OutputGroups[0].Nested[0])
	assert.Equal(t, out.OutputGroups[0].Location, first.Location)
	require.NotNil(t, first.StructPlan)

	next := requirePlanField(t, first.StructPlan, "Next")
	assert.Equal(t, plan.OperationStruct, next.Mapping.Operation)
	assert.Equal(t, []plan.ValueAdaptation{plan.ValueAdaptationDeref}, next.Mapping.SourceAdaptations)
	assert.Equal(t, []plan.ValueAdaptation{plan.ValueAdaptationAddress}, next.Mapping.TargetAdaptations)
	assert.Same(t, first, next.Mapping.Plan)
}

func TestPlannerPlanPackageOwnedNestedStructsInSameOutputPackage(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
		Packages: []config.Package{
			{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Output: config.Output{Filename: "a.morph.go"},
				Types: []config.Type{{
					Name: "Container",
					Mappers: &config.MappersDefaults{
						Forward: &config.MapperDefaults{Name: new("MapContainerA")},
					},
				}},
			},
			{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Output: config.Output{Filename: "b.morph.go"},
				Types: []config.Type{{
					Name: "Container",
					Mappers: &config.MappersDefaults{
						Forward: &config.MapperDefaults{Name: new("MapContainerB")},
					},
				}},
			},
		},
	})

	require.Len(t, out.OutputGroups, 2)
	assert.Contains(t, out.OutputGroups[0].Location.LogicalPath, "a.morph.go")
	assert.Contains(t, out.OutputGroups[1].Location.LogicalPath, "b.morph.go")
	require.Len(t, out.OutputGroups[0].Roots, 1)
	require.Len(t, out.OutputGroups[1].Roots, 1)
	require.Len(t, out.OutputGroups[0].Nested, 1)
	assert.Empty(t, out.OutputGroups[1].Nested)

	firstRoot := out.OutputGroups[0].Roots[0]
	secondRoot := out.OutputGroups[1].Roots[0]
	firstNested := requireFieldPlan(t, firstRoot.StructPlan, "First")
	secondNested := requireFieldPlan(t, secondRoot.StructPlan, "First")

	assert.Same(t, firstNested, secondNested)
	assert.Same(t, out.OutputGroups[0].Nested[0], firstNested)
	assert.Equal(t, out.OutputGroups[0].Location, firstNested.Location)
	assert.Equal(t, out.OutputGroups[0].Location, firstRoot.Location)
	assert.Equal(t, out.OutputGroups[1].Location, secondRoot.Location)
}

func TestPlannerPlanPackageOwnedNestedStructsSeparateOutputPackages(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
		Packages: []config.Package{
			{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Output: config.Output{Path: "mapping_a", Package: "mappinga"},
				Types: []config.Type{
					{
						Name: "Node",
						Mappers: &config.MappersDefaults{
							Forward: &config.MapperDefaults{Name: new("MapNodeA")},
						},
					},
					{
						Name: "NodeBoxContainer",
						Mappers: &config.MappersDefaults{
							Forward: &config.MapperDefaults{Name: new("MapNodeBoxContainerA")},
						},
					},
				},
			},
			{
				Source: plannerFromPackage,
				Target: plannerToPackage,
				Output: config.Output{Path: "mapping_b", Package: "mappingb"},
				Types: []config.Type{
					{
						Name: "Node",
						Mappers: &config.MappersDefaults{
							Forward: &config.MapperDefaults{Name: new("MapNodeB")},
						},
					},
					{
						Name: "NodeBoxContainer",
						Mappers: &config.MappersDefaults{
							Forward: &config.MapperDefaults{Name: new("MapNodeBoxContainerB")},
						},
					},
				},
			},
		},
	})

	require.Len(t, out.OutputGroups, 2)

	groupA := requireOutputGroupByPackageName(t, out.OutputGroups, "mappinga")
	groupB := requireOutputGroupByPackageName(t, out.OutputGroups, "mappingb")
	require.Len(t, groupA.Nested, 1)
	require.Len(t, groupB.Nested, 1)

	nodeA := requireRootByTargetName(t, groupA.Roots, "Node")
	nodeB := requireRootByTargetName(t, groupB.Roots, "Node")
	boxContainerA := requireRootByTargetName(t, groupA.Roots, "NodeBoxContainer")
	boxContainerB := requireRootByTargetName(t, groupB.Roots, "NodeBoxContainer")
	nestedA := requireFieldPlan(t, boxContainerA.StructPlan, "Box")
	nestedB := requireFieldPlan(t, boxContainerB.StructPlan, "Box")

	assert.NotSame(t, nestedA, nestedB)
	assert.Same(t, groupA.Nested[0], nestedA)
	assert.Same(t, groupB.Nested[0], nestedB)
	assert.Equal(t, groupA.Location, nestedA.Location)
	assert.Equal(t, groupB.Location, nestedB.Location)

	valueA := requirePlanField(t, nestedA.StructPlan, "Value")
	valueB := requirePlanField(t, nestedB.StructPlan, "Value")
	assert.Same(t, nodeA, valueA.Mapping.Plan)
	assert.Same(t, nodeB, valueB.Mapping.Plan)
}

func TestPlannerPlanGenericNestedStructs(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "GenericContainer",
			}},
		}},
	})

	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	root := out.OutputGroups[0].Roots[0]
	require.NotNil(t, root.StructPlan)

	intBox := requireFieldPlan(t, root.StructPlan, "IntBox")
	stringBox := requireFieldPlan(t, root.StructPlan, "StringBox")
	require.Len(t, out.OutputGroups[0].Nested, 2)

	assert.NotSame(t, stringBox, intBox)
	assert.Contains(t, out.OutputGroups[0].Nested, intBox)
	assert.Contains(t, out.OutputGroups[0].Nested, stringBox)
	assert.Equal(t, out.OutputGroups[0].Location, intBox.Location)
	assert.Equal(t, out.OutputGroups[0].Location, stringBox.Location)
	assert.NotEqual(t, stringBox.FunctionName, intBox.FunctionName)
	assert.NotEqual(t, stringBox.Source.Key, intBox.Source.Key)
	assert.NotEqual(t, stringBox.Target.Key, intBox.Target.Key)

	intValue := requirePlanField(t, intBox.StructPlan, "Value")
	assert.Equal(t, plan.OperationConvert, intValue.Mapping.Operation)
	assert.Equal(t, types.TypeKindBasic, intValue.Mapping.Source.Kind)
	assert.Equal(t, "int", intValue.Mapping.Source.Name)
	assert.Equal(t, types.TypeKindBasic, intValue.Mapping.Target.Kind)
	assert.Equal(t, "int64", intValue.Mapping.Target.Name)

	stringValue := requirePlanField(t, stringBox.StructPlan, "Value")
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

	t.Run("keeps basic conversions automatic", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"})

		count := requirePlanField(t, root.StructPlan, "Count")

		assert.Equal(t, plan.OperationConvert, count.Mapping.Operation)
	})

	t.Run("keeps unregistered lossy numeric conversions unsupported", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source:        plannerFromPackage,
				Target:        plannerToPackage,
				Bidirectional: new(true),
				Types: []config.Type{{
					Name: "ConversionContainer",
				}},
			}},
		})
		require.Len(t, out.OutputGroups, 1)
		require.Len(t, out.OutputGroups[0].Roots, 2)

		inverse := requireRootBySourcePackage(t, out.OutputGroups[0].Roots, plannerToPackage)
		count := requirePlanField(t, inverse.StructPlan, "Count")

		assert.Equal(t, plan.OperationUnsupported, count.Mapping.Operation)
	})

	t.Run("uses bidirectional registered lossy numeric conversions", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
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
		})
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
	out := planWithConfig(t, ".", config.Config{
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types:  []config.Type{{Name: "NumberContainer"}},
		}},
	})

	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	root := out.OutputGroups[0].Roots[0]
	assert.Empty(t, root.TypeParams)
	assert.True(t, plan.HasFatalDiagnostics(root.Diagnostics))
	require.NotEmpty(t, root.Diagnostics)
	assert.Equal(t, "generic root mappings are not supported; map concrete instantiations through containing types or provide a higher-order callable", root.Diagnostics[0].Message)
}

func TestPlannerPlanNestedStructsWithScopedCallables(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
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
	})

	rootA := requireRootByTargetName(t, out.OutputGroups[0].Roots, "ScopedStringBoxA")
	rootB := requireRootByTargetName(t, out.OutputGroups[0].Roots, "ScopedStringBoxB")

	boxA := requireFieldPlan(t, rootA.StructPlan, "Box")
	boxB := requireFieldPlan(t, rootB.StructPlan, "Box")

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
	out := planWithConfig(t, ".", config.Config{
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
	})

	root := out.OutputGroups[0].Roots[0]
	field := requirePlanField(t, root.StructPlan, "Value")

	require.NotNil(t, field.Mapping.Callable)
	assert.Equal(t, "ExplicitStringToInt", field.Mapping.Callable.Name)
}

func TestPlannerPlanMethodCallable(t *testing.T) {
	t.Run("ignores unreferenced methods", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: "github.com/seeruk/morph/testdata/planner/from",
				Target: "github.com/seeruk/morph/testdata/planner/to",
				Types:  []config.Type{{Name: "MethodCallableContainer"}},
			}},
		})

		root := out.OutputGroups[0].Roots[0]
		field := requirePlanField(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationUnsupported, field.Mapping.Operation)
	})

	t.Run("uses explicitly referenced methods", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
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
		})

		root := out.OutputGroups[0].Roots[0]
		field := requirePlanField(t, root.StructPlan, "ID")

		require.NotNil(t, field.Mapping.Callable)
		assert.Equal(t, plan.OperationMethod, field.Mapping.Operation)
		assert.Equal(t, "String", field.Mapping.Callable.Name)
	})
}

func TestPlannerPlanHigherOrderGenericFunction(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
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
	})

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
	out := planWithConfig(t, ".", config.Config{
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
	})

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
	out := planWithConfig(t, ".", config.Config{
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
	})

	root := out.OutputGroups[0].Roots[0]
	maybe := requirePlanField(t, root.StructPlan, "Maybe")

	require.NotNil(t, maybe.Mapping.Callable)
	assert.Equal(t, "MapOptional", maybe.Mapping.Callable.Name)
	require.Len(t, maybe.Mapping.CallableArgs, 1)
	assert.Equal(t, plan.OperationStruct, maybe.Mapping.CallableArgs[0].Mapping.Operation)
}

func TestPlannerPlanHigherOrderGenericFunctionWithErrors(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
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
	})

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
	out := planWithConfig(t, ".", config.Config{
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
	})

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

const (
	plannerFromPackage      = "github.com/seeruk/morph/testdata/planner/from"
	plannerToPackage        = "github.com/seeruk/morph/testdata/planner/to"
	visibilitySourcePackage = "github.com/seeruk/morph/testdata/visibility/source"
	visibilityTargetPackage = "github.com/seeruk/morph/testdata/visibility/target"
)

func planPlannerRoot(t *testing.T, typ config.Type, conversions ...config.Conversion) *plan.Type {
	t.Helper()

	out := planWithConfig(t, ".", config.Config{
		Conversions: conversions,
		Packages: []config.Package{{
			Source: plannerFromPackage,
			Target: plannerToPackage,
			Types:  []config.Type{typ},
		}},
	})
	require.Len(t, out.OutputGroups, 1)
	require.Len(t, out.OutputGroups[0].Roots, 1)

	return out.OutputGroups[0].Roots[0]
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

func requireOutputGroupByPackageName(t *testing.T, groups []plan.OutputGroup, packageName string) plan.OutputGroup {
	t.Helper()

	for _, group := range groups {
		if group.Location.PackageName == packageName {
			return group
		}
	}

	require.Failf(t, "output group not planned", "output group %q was not planned", packageName)
	return plan.OutputGroup{}
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

func requireFieldPlan(t *testing.T, structPlan *plan.Struct, targetName string) *plan.Type {
	t.Helper()

	field := requirePlanField(t, structPlan, targetName)
	require.NotNil(t, field.Mapping.Plan)
	return field.Mapping.Plan
}

func requireValueElem(t *testing.T, value plan.Value) *plan.Value {
	t.Helper()

	require.NotNil(t, value.Elem)
	return value.Elem
}

func requireCallableArg(t *testing.T, value plan.Value, index int) plan.CallableArg {
	t.Helper()

	require.NotNil(t, value.Callable)
	require.Greater(t, len(value.CallableArgs), index)
	return value.CallableArgs[index]
}

func pointerErrorOptionalityDefaults() *config.OptionalityDefaults {
	pointerError := spec.PointerOptionalityError
	valueNil := spec.ValueOptionalityNil
	return &config.OptionalityDefaults{
		OnNilSourcePointer: &pointerError,
		OnZeroSourceValue:  &valueNil,
	}
}

func testField(name string, typ types.Type) types.Field {
	return types.Field{
		Name:       name,
		Type:       typ,
		IsExported: true,
	}
}
