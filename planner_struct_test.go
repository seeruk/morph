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

		values := requirePlanProperty(t, root.StructPlan, "Values")

		assert.Equal(t, plan.OperationSlice, values.Mapping.Operation)
		require.NotNil(t, values.Mapping.Elem)
		assert.Equal(t, plan.OperationStruct, values.Mapping.Elem.Operation)
		require.Len(t, root.Diagnostics, 1)
		assert.Equal(t, plan.DiagnosticLevelWarning, root.Diagnostics[0].Level)
		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.Contains(t, root.Diagnostics[0].Path, "target property Extra")
	})

	t.Run("marks slice mapping unsupported when the element mapper is fatal", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"})

		values := requirePlanProperty(t, root.StructPlan, "Values")

		assert.Equal(t, plan.OperationUnsupported, values.Mapping.Operation)
		require.NotNil(t, values.Mapping.Elem)
		assert.Equal(t, plan.OperationUnsupported, values.Mapping.Elem.Operation)
		assert.True(t, plan.HasFatalDiagnostics(values.Mapping.Diagnostics))
		assert.True(t, plan.HasFatalDiagnostics(root.Diagnostics))
	})
}

func TestPlannerPlanStructOmissions(t *testing.T) {
	t.Run("warns for unmapped source and target properties by default", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "OmissionContainer"})

		assert.Equal(t, []string{
			`no source property found for target property "TargetOnly"; configure struct.properties to map it explicitly or struct.omit.target to omit it`,
			`no target property found for source property "SourceOnly"; configure struct.properties to map it explicitly or struct.omit.source to omit it`,
		}, diagnosticsMessages(root.Diagnostics))
		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
	})

	t.Run("omits target properties from planning and target coverage warnings", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Target: []string{"TargetOnly"},
			}},
		})

		assert.NotContains(t, planPropertyTargetNames(root.StructPlan), "TargetOnly")
		assert.NotContains(t, diagnosticsMessages(root.Diagnostics), `no source property found for target property "TargetOnly"; configure struct.properties to map it explicitly or struct.omit.target to omit it`)
		assert.Contains(t, diagnosticsMessages(root.Diagnostics), `no target property found for source property "SourceOnly"; configure struct.properties to map it explicitly or struct.omit.source to omit it`)
	})

	t.Run("omits source properties from matching and source coverage warnings", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Source: []string{"SourceOnly"},
			}},
		})

		assert.NotContains(t, diagnosticsMessages(root.Diagnostics), `no target property found for source property "SourceOnly"; configure struct.properties to map it explicitly or struct.omit.source to omit it`)
		assert.Contains(t, diagnosticsMessages(root.Diagnostics), `no source property found for target property "TargetOnly"; configure struct.properties to map it explicitly or struct.omit.target to omit it`)
	})

	t.Run("omits both matched properties from matching and coverage warnings", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Both:   []string{"Shared"},
				Source: []string{"SourceOnly"},
				Target: []string{"TargetOnly"},
			}},
		})

		assert.Empty(t, root.Diagnostics)
		assert.NotContains(t, planPropertyTargetNames(root.StructPlan), "Shared")
	})

	t.Run("warns with both hint for source omissions that match target properties", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Source: []string{"Shared", "SourceOnly"},
				Target: []string{"TargetOnly"},
			}},
		})

		assert.Equal(t, []string{
			`no source property found for target property "Shared"; configure struct.properties to map it explicitly or struct.omit.both to omit a property that exists on both sides`,
		}, diagnosticsMessages(root.Diagnostics))
		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.NotContains(t, planPropertyTargetNames(root.StructPlan), "Shared")
	})

	t.Run("warns with both hint for target omissions that match source properties", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Source: []string{"SourceOnly"},
				Target: []string{"Shared", "TargetOnly"},
			}},
		})

		assert.Equal(t, []string{
			`no target property found for source property "Shared"; configure struct.properties to map it explicitly or struct.omit.both to omit a property that exists on both sides`,
		}, diagnosticsMessages(root.Diagnostics))
		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.NotContains(t, planPropertyTargetNames(root.StructPlan), "Shared")
	})

	t.Run("warns when matched properties are omitted as both source and target", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Source: []string{"Shared", "SourceOnly"},
				Target: []string{"Shared", "TargetOnly"},
			}},
		})

		assert.Equal(t, []string{
			`property "Shared" is omitted as both source and target; configure struct.omit.both to omit a property that exists on both sides`,
		}, diagnosticsMessages(root.Diagnostics))
		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.NotContains(t, planPropertyTargetNames(root.StructPlan), "Shared")
	})

	t.Run("inverts source and target omissions while preserving both omissions", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source:        plannerFromPackage,
				Target:        plannerToPackage,
				Bidirectional: new(true),
				Types: []config.Type{{
					Name: "OmissionContainer",
					Struct: &config.Struct{Omit: config.StructOmissions{
						Both:   []string{"Shared"},
						Source: []string{"SourceOnly"},
						Target: []string{"TargetOnly"},
					}},
				}},
			}},
		})

		require.Len(t, out.OutputGroups, 1)
		require.Len(t, out.OutputGroups[0].Roots, 2)

		forward := requireRootBySourcePackage(t, out.OutputGroups[0].Roots, plannerFromPackage)
		inverse := requireRootBySourcePackage(t, out.OutputGroups[0].Roots, plannerToPackage)

		assert.Empty(t, forward.Diagnostics)
		assert.Empty(t, inverse.Diagnostics)
		assert.NotContains(t, planPropertyTargetNames(forward.StructPlan), "Shared")
		assert.NotContains(t, planPropertyTargetNames(inverse.StructPlan), "Shared")
	})

	t.Run("omits both properties discovered through methods", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "GetterToSetterContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Both: []string{"Name"},
			}},
		})

		assert.Empty(t, root.Diagnostics)
		assert.Empty(t, root.StructPlan.Properties)
	})

	t.Run("allows inverted source omissions for read-only inverse target members", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source:        plannerFromPackage,
				Target:        plannerToPackage,
				Bidirectional: new(true),
				Types: []config.Type{{
					Name: "ReadMethodOnlyContainer",
					Struct: &config.Struct{Omit: config.StructOmissions{
						Source: []string{"Name"},
					}},
				}},
			}},
		})

		require.Len(t, out.OutputGroups, 1)
		require.Len(t, out.OutputGroups[0].Roots, 2)

		inverse := requireRootBySourcePackage(t, out.OutputGroups[0].Roots, plannerToPackage)

		assert.False(t, plan.HasFatalDiagnostics(inverse.Diagnostics))
		assert.NotContains(t, diagnosticsMessages(inverse.Diagnostics), `target property "Name" does not exist`)
	})

	t.Run("rejects invalid omitted properties", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Source: []string{"MissingSource"},
				Target: []string{"MissingTarget"},
			}},
		})

		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`source property "MissingSource" does not exist`,
			`target property "MissingTarget" does not exist`,
		)
	})

	t.Run("rejects both omissions that do not exist", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Both: []string{"Missing"},
			}},
		})

		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`property "Missing" in struct.omit.both does not exist on source or target`,
		)
	})

	t.Run("rejects source-only properties in both omissions", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Both: []string{"SourceOnly"},
			}},
		})

		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`property "SourceOnly" in struct.omit.both does not have a matching target property; use struct.omit.source for source-only properties`,
		)
	})

	t.Run("rejects target-only properties in both omissions", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Both: []string{"TargetOnly"},
			}},
		})

		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`property "TargetOnly" in struct.omit.both does not have a matching source property; use struct.omit.target for target-only properties`,
		)
	})

	t.Run("rejects ambiguous both omissions", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "AmbiguousOmissionContainer",
			Struct: &config.Struct{Omit: config.StructOmissions{
				Both: []string{"APIID"},
			}},
		})

		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`property "APIID" in struct.omit.both does not have a unique source and target match; configure struct.properties for ambiguous matches or use struct.omit.source/target for one-sided properties`,
		)
	})

	t.Run("rejects omitted properties that are explicitly mapped", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{
				Properties: []config.Property{
					{Source: "SourceOnly", Target: "TargetOnly"},
				},
				Omit: config.StructOmissions{
					Source: []string{"SourceOnly"},
					Target: []string{"TargetOnly"},
				},
			},
		})

		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`source property "SourceOnly" cannot be both omitted and explicitly mapped`,
			`target property "TargetOnly" cannot be both omitted and explicitly mapped`,
		)
	})

	t.Run("rejects both omissions that are explicitly mapped", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "OmissionContainer",
			Struct: &config.Struct{
				Properties: []config.Property{
					{Name: "Shared"},
				},
				Omit: config.StructOmissions{
					Both:   []string{"Shared"},
					Source: []string{"SourceOnly"},
					Target: []string{"TargetOnly"},
				},
			},
		})

		assertFatalDiagnosticMessages(
			t,
			root.Diagnostics,
			`property "Shared" cannot be both omitted via struct.omit.both and explicitly mapped`,
		)
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
					Struct: &config.Struct{Properties: []config.Property{
						{Source: "secret", Target: "Secret"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		field := requirePlanProperty(t, root.StructPlan, "Secret")

		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.Equal(t, "secret", field.Source.Name)
	})

	t.Run("allows unexported target fields from the generated package", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategyTargetPackage)},
				Types: []config.Type{{
					Name: "LowerTargetFieldContainer",
					Struct: &config.Struct{Properties: []config.Property{
						{Source: "Secret", Target: "secret"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		field := requirePlanProperty(t, root.StructPlan, "secret")

		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.Equal(t, "Secret", field.Source.Name)
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
		field := requirePlanProperty(t, root.StructPlan, "secret")

		assert.False(t, plan.HasFatalDiagnostics(root.Diagnostics))
		assert.Equal(t, "secret", field.Source.Name)
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
		assert.Empty(t, root.StructPlan.Properties)
	})

	t.Run("rejects explicit unexported source fields from another generated package", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source: visibilitySourcePackage,
				Target: visibilityTargetPackage,
				Output: config.Output{Strategy: new(spec.OutputStrategyTargetPackage)},
				Types: []config.Type{{
					Name: "LowerSourceFieldContainer",
					Struct: &config.Struct{Properties: []config.Property{
						{Source: "secret", Target: "Secret"},
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
					Struct: &config.Struct{Properties: []config.Property{
						{Source: "Secret", Target: "secret"},
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
					Struct: &config.Struct{Properties: []config.Property{
						{Name: "ID"},
					}},
				}},
			}},
		})

		root := requireSingleRoot(t, out)
		assertFatalDiagnosticMessages(t, root.Diagnostics, `source field "ID" is embedded; embedded fields are not supported`)
	})
}

func TestPlannerPlanStructMethodAccessors(t *testing.T) {
	t.Run("keeps case-insensitive property matching", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "CaseInsensitivePropertyContainer"})

		recipeID := requirePlanProperty(t, root.StructPlan, "RecipeId")

		assert.Equal(t, "RecipeID", recipeID.Source.Name)
		assert.Equal(t, "RecipeId", recipeID.Target.Name)
		assert.Equal(t, plan.OperationAssign, recipeID.Mapping.Operation)
	})

	t.Run("maps source fields to target setters automatically", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "FieldToSetterContainer"})

		name := requirePlanProperty(t, root.StructPlan, "Name")

		assert.Equal(t, plan.MemberKindField, name.Source.Kind)
		assert.Equal(t, "Name", name.Source.Accessor)
		assert.Equal(t, plan.MemberKindMethod, name.Target.Kind)
		assert.Equal(t, "SetName", name.Target.Accessor)
		assert.Equal(t, plan.OperationAssign, name.Mapping.Operation)
	})

	t.Run("maps source getters to target fields automatically", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "GetterToFieldContainer"})

		name := requirePlanProperty(t, root.StructPlan, "Name")

		assert.Equal(t, plan.MemberKindMethod, name.Source.Kind)
		assert.Equal(t, "GetName", name.Source.Accessor)
		assert.Equal(t, plan.MemberKindField, name.Target.Kind)
		assert.Equal(t, "Name", name.Target.Accessor)
		assert.Equal(t, plan.OperationAssign, name.Mapping.Operation)
	})

	t.Run("maps getter to setter automatically", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "GetterToSetterContainer"})

		name := requirePlanProperty(t, root.StructPlan, "Name")

		assert.Equal(t, plan.MemberKindMethod, name.Source.Kind)
		assert.Equal(t, "GetName", name.Source.Accessor)
		assert.Equal(t, plan.MemberKindMethod, name.Target.Kind)
		assert.Equal(t, "SetName", name.Target.Accessor)
		assert.Equal(t, plan.OperationAssign, name.Mapping.Operation)
	})

	t.Run("does not infer method-to-method mappings without a target setter", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ReadMethodOnlyContainer"})

		require.NotNil(t, root.StructPlan)
		assert.Empty(t, root.StructPlan.Properties)
		assert.Contains(t, diagnosticsMessages(root.Diagnostics), `no target property found for source property "Name"; configure struct.properties to map it explicitly or struct.omit.source to omit it`)
	})

	t.Run("requires configured properties for accessor-only mappings", func(t *testing.T) {
		automatic := planPlannerRoot(t, config.Type{Name: "ExplicitAccessorContainer"})
		require.NotNil(t, automatic.StructPlan)
		assert.Empty(t, automatic.StructPlan.Properties)

		configured := planPlannerRoot(t, config.Type{
			Name: "ExplicitAccessorContainer",
			Struct: &config.Struct{Properties: []config.Property{{
				Source: "EmailAddress",
				Target: "Email",
				Accessors: config.PropertyAccessors{
					Forward: config.PropertyDirectionAccessors{
						Read:  "FetchEmail",
						Write: "StoreEmail",
					},
				},
			}}},
		})

		email := requirePlanProperty(t, configured.StructPlan, "Email")
		assert.Equal(t, plan.MemberKindMethod, email.Source.Kind)
		assert.Equal(t, "FetchEmail", email.Source.Accessor)
		assert.Equal(t, "EmailAddress", email.Source.Name)
		assert.Equal(t, plan.MemberKindMethod, email.Target.Kind)
		assert.Equal(t, "StoreEmail", email.Target.Accessor)
		assert.Equal(t, "Email", email.Target.Name)
	})

	t.Run("inferMethods false disables automatic method inference but not explicit accessors", func(t *testing.T) {
		inferMethods := false
		automatic := planPlannerRoot(t, config.Type{
			Name:   "GetterToFieldContainer",
			Struct: &config.Struct{InferMethods: &inferMethods},
		})
		require.NotNil(t, automatic.StructPlan)
		assert.Empty(t, automatic.StructPlan.Properties)
		assert.Contains(t, diagnosticsMessages(automatic.Diagnostics), `no source property found for target property "Name"; configure struct.properties to map it explicitly or struct.omit.target to omit it`)

		configured := planPlannerRoot(t, config.Type{
			Name: "GetterToFieldContainer",
			Struct: &config.Struct{
				InferMethods: &inferMethods,
				Properties: []config.Property{{
					Name: "Name",
					Accessors: config.PropertyAccessors{
						Forward: config.PropertyDirectionAccessors{Read: "GetName"},
					},
				}},
			},
		})

		name := requirePlanProperty(t, configured.StructPlan, "Name")
		assert.Equal(t, "GetName", name.Source.Accessor)
		assert.Equal(t, plan.MemberKindMethod, name.Source.Kind)
		assert.Equal(t, "Name", name.Target.Accessor)
		assert.Equal(t, plan.MemberKindField, name.Target.Kind)
	})

	t.Run("uses configured forward and inverse accessors directionally", func(t *testing.T) {
		out := planWithConfig(t, ".", config.Config{
			Packages: []config.Package{{
				Source:        plannerFromPackage,
				Target:        plannerToPackage,
				Bidirectional: new(true),
				Types: []config.Type{{
					Name: "BidirectionalAccessorContainer",
					Struct: &config.Struct{Properties: []config.Property{{
						Source: "EmailAddress",
						Target: "Email",
						Accessors: config.PropertyAccessors{
							Forward: config.PropertyDirectionAccessors{
								Read:  "GetEmailAddress",
								Write: "SetEmail",
							},
							Inverse: config.PropertyDirectionAccessors{
								Read:  "GetEmail",
								Write: "SetEmailAddress",
							},
						},
					}}},
				}},
			}},
		})

		require.Len(t, out.OutputGroups, 1)
		require.Len(t, out.OutputGroups[0].Roots, 2)

		forward := requireRootBySourcePackage(t, out.OutputGroups[0].Roots, plannerFromPackage)
		forwardEmail := requirePlanProperty(t, forward.StructPlan, "Email")
		assert.Equal(t, "GetEmailAddress", forwardEmail.Source.Accessor)
		assert.Equal(t, "SetEmail", forwardEmail.Target.Accessor)

		inverse := requireRootBySourcePackage(t, out.OutputGroups[0].Roots, plannerToPackage)
		inverseEmail := requirePlanProperty(t, inverse.StructPlan, "EmailAddress")
		assert.Equal(t, "GetEmail", inverseEmail.Source.Accessor)
		assert.Equal(t, "SetEmailAddress", inverseEmail.Target.Accessor)
	})

	t.Run("propagates accessor errors", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ErrorAccessorContainer"})

		name := requirePlanProperty(t, root.StructPlan, "Name")

		assert.True(t, root.CanError)
		assert.True(t, name.Source.CanError)
		assert.True(t, name.Target.CanError)
	})

	t.Run("reports invalid explicit accessor signatures", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name: "BidirectionalAccessorContainer",
			Struct: &config.Struct{Properties: []config.Property{{
				Source: "EmailAddress",
				Target: "Email",
				Accessors: config.PropertyAccessors{
					Forward: config.PropertyDirectionAccessors{
						Read:  "SetEmailAddress",
						Write: "SetEmail",
					},
				},
			}}},
		})

		assertFatalDiagnosticMessages(t, root.Diagnostics, `source accessor "SetEmailAddress" is not a readable accessor`)
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
	require.Len(t, root.StructPlan.Properties, 2)

	first := requirePropertyPlan(t, root.StructPlan, "First")
	second := requirePropertyPlan(t, root.StructPlan, "Second")
	assert.Same(t, first, second)
	assert.Same(t, first, out.OutputGroups[0].Nested[0])
	assert.Equal(t, out.OutputGroups[0].Location, first.Location)
	require.NotNil(t, first.StructPlan)

	next := requirePlanProperty(t, first.StructPlan, "Next")
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
	firstNested := requirePropertyPlan(t, firstRoot.StructPlan, "First")
	secondNested := requirePropertyPlan(t, secondRoot.StructPlan, "First")

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
	nestedA := requirePropertyPlan(t, boxContainerA.StructPlan, "Box")
	nestedB := requirePropertyPlan(t, boxContainerB.StructPlan, "Box")

	assert.NotSame(t, nestedA, nestedB)
	assert.Same(t, groupA.Nested[0], nestedA)
	assert.Same(t, groupB.Nested[0], nestedB)
	assert.Equal(t, groupA.Location, nestedA.Location)
	assert.Equal(t, groupB.Location, nestedB.Location)

	valueA := requirePlanProperty(t, nestedA.StructPlan, "Value")
	valueB := requirePlanProperty(t, nestedB.StructPlan, "Value")
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

	intBox := requirePropertyPlan(t, root.StructPlan, "IntBox")
	stringBox := requirePropertyPlan(t, root.StructPlan, "StringBox")
	require.Len(t, out.OutputGroups[0].Nested, 2)

	assert.NotSame(t, stringBox, intBox)
	assert.Contains(t, out.OutputGroups[0].Nested, intBox)
	assert.Contains(t, out.OutputGroups[0].Nested, stringBox)
	assert.Equal(t, out.OutputGroups[0].Location, intBox.Location)
	assert.Equal(t, out.OutputGroups[0].Location, stringBox.Location)
	assert.NotEqual(t, stringBox.FunctionName, intBox.FunctionName)
	assert.NotEqual(t, stringBox.Source.Key, intBox.Source.Key)
	assert.NotEqual(t, stringBox.Target.Key, intBox.Target.Key)

	intValue := requirePlanProperty(t, intBox.StructPlan, "Value")
	assert.Equal(t, plan.OperationConvert, intValue.Mapping.Operation)
	assert.Equal(t, types.TypeKindBasic, intValue.Mapping.Source.Kind)
	assert.Equal(t, "int", intValue.Mapping.Source.Name)
	assert.Equal(t, types.TypeKindBasic, intValue.Mapping.Target.Kind)
	assert.Equal(t, "int64", intValue.Mapping.Target.Name)

	stringValue := requirePlanProperty(t, stringBox.StructPlan, "Value")
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

		count := requirePlanProperty(t, root.StructPlan, "Count")

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
		count := requirePlanProperty(t, inverse.StructPlan, "Count")

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
		count := requirePlanProperty(t, inverse.StructPlan, "Count")

		assert.Equal(t, plan.OperationConvert, count.Mapping.Operation)
		assert.Equal(t, "int64", count.Mapping.Source.Name)
		assert.Equal(t, "int", count.Mapping.Target.Name)
	})

	t.Run("requires registry for named conversions", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"})

		id := requirePlanProperty(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationUnsupported, id.Mapping.Operation)
	})

	t.Run("uses registered named conversions", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"}, userIDToString)

		id := requirePlanProperty(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationConvert, id.Mapping.Operation)
	})

	t.Run("unwraps aliases before matching registry pairs", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"}, userIDToString)

		alias := requirePlanProperty(t, root.StructPlan, "Alias")

		assert.Equal(t, plan.OperationConvert, alias.Mapping.Operation)
	})

	t.Run("uses registry conversions inside slices", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{Name: "ConversionContainer"}, userIDToString)

		values := requirePlanProperty(t, root.StructPlan, "Values")
		require.NotNil(t, values.Mapping.Elem)

		assert.Equal(t, plan.OperationConvert, values.Mapping.Elem.Operation)
	})

	t.Run("disables registry conversions at type scope", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name:        "ConversionContainer",
			Conversions: &config.ConversionsDefaults{Enabled: new(false)},
		}, userIDToString)

		id := requirePlanProperty(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationUnsupported, id.Mapping.Operation)
	})

	t.Run("re-enables registry conversions at property scope", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name:        "ConversionsPolicyContainer",
			Conversions: &config.ConversionsDefaults{Enabled: new(false)},
			Struct: &config.Struct{Properties: []config.Property{
				{Name: "ID", Conversions: &config.ConversionsDefaults{Enabled: new(true)}},
			}},
		}, userIDToString)

		id := requirePlanProperty(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationConvert, id.Mapping.Operation)
	})

	t.Run("keeps disabled registry conversions disabled for other properties", func(t *testing.T) {
		root := planPlannerRoot(t, config.Type{
			Name:        "ConversionsPolicyContainer",
			Conversions: &config.ConversionsDefaults{Enabled: new(false)},
			Struct: &config.Struct{Properties: []config.Property{
				{Name: "ID", Conversions: &config.ConversionsDefaults{Enabled: new(true)}},
			}},
		}, userIDToString)

		other := requirePlanProperty(t, root.StructPlan, "Other")

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

		code := requirePlanProperty(t, root.StructPlan, "Code")

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

	boxA := requirePropertyPlan(t, rootA.StructPlan, "Box")
	boxB := requirePropertyPlan(t, rootB.StructPlan, "Box")

	assert.NotSame(t, boxA, boxB)
	assert.NotEqual(t, boxA.FunctionName, boxB.FunctionName)

	valueA := requirePlanProperty(t, boxA.StructPlan, "Value")
	valueB := requirePlanProperty(t, boxB.StructPlan, "Value")
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
	property := requirePlanProperty(t, root.StructPlan, "Value")

	require.NotNil(t, property.Mapping.Callable)
	assert.Equal(t, "ExplicitStringToInt", property.Mapping.Callable.Name)
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
		property := requirePlanProperty(t, root.StructPlan, "ID")

		assert.Equal(t, plan.OperationUnsupported, property.Mapping.Operation)
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
		property := requirePlanProperty(t, root.StructPlan, "ID")

		require.NotNil(t, property.Mapping.Callable)
		assert.Equal(t, plan.OperationMethod, property.Mapping.Operation)
		assert.Equal(t, "String", property.Mapping.Callable.Name)
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

	maybe := requirePlanProperty(t, root.StructPlan, "Maybe")
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

	t.Run("plans the mapper argument properties", func(t *testing.T) {
		name := requirePlanProperty(t, arg.Mapping.Plan.StructPlan, "Name")
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

	maybe := requirePlanProperty(t, root.StructPlan, "Maybe")
	assert.True(t, plan.HasFatalDiagnostics(root.Diagnostics))
	assert.True(t, plan.HasFatalDiagnostics(maybe.Mapping.Diagnostics))
	assert.Contains(t, diagnosticsMessages(root.Diagnostics), "callable argument 1 could not map github.com/seeruk/morph/testdata/planner/from.OptionalBadThing to github.com/seeruk/morph/testdata/planner/to.OptionalBadThing; configure a compatible callable, explicit mapper, discovery package, or registered conversion")
	assert.Contains(t, diagnosticsPaths(root.Diagnostics), plan.PropertyPath(root.SourceType, root.TargetType, maybe.Source.Name, maybe.Target.Name)+" :: callable argument 1")
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
	maybe := requirePlanProperty(t, root.StructPlan, "Maybe")

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

	maybe := requirePlanProperty(t, root.StructPlan, "Maybe")
	require.NotNil(t, maybe.Mapping.Callable)
	require.Len(t, maybe.Mapping.CallableArgs, 1)
	arg := maybe.Mapping.CallableArgs[0]
	require.NotNil(t, arg.Mapping.Callable)

	t.Run("propagates errors to the root and property mapping", func(t *testing.T) {
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

	result := requirePlanProperty(t, root.StructPlan, "Result")
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

		leftCode := requirePlanProperty(t, leftArg.Mapping.Plan.StructPlan, "Code")
		assert.Equal(t, plan.OperationAssign, leftCode.Mapping.Operation)
	})

	t.Run("plans the right mapper argument", func(t *testing.T) {
		assert.False(t, rightArg.ReturnsError)
		assert.Equal(t, plan.OperationStruct, rightArg.Mapping.Operation)
		assert.Equal(t, "EitherRight", rightArg.Mapping.Source.Name)
		assert.Equal(t, "EitherRight", rightArg.Mapping.Target.Name)

		rightName := requirePlanProperty(t, rightArg.Mapping.Plan.StructPlan, "Name")
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

func requirePlanProperty(t *testing.T, structPlan *plan.Struct, targetName string) plan.Property {
	t.Helper()

	require.NotNil(t, structPlan)
	for _, property := range structPlan.Properties {
		if property.Target.Name == targetName {
			return property
		}
	}

	require.Failf(t, "property not planned", "target property %q was not planned", targetName)
	return plan.Property{}
}

func planPropertyTargetNames(structPlan *plan.Struct) []string {
	if structPlan == nil {
		return nil
	}

	out := make([]string, 0, len(structPlan.Properties))
	for _, property := range structPlan.Properties {
		out = append(out, property.Target.Name)
	}
	return out
}

func requirePropertyPlan(t *testing.T, structPlan *plan.Struct, targetName string) *plan.Type {
	t.Helper()

	property := requirePlanProperty(t, structPlan, targetName)
	require.NotNil(t, property.Mapping.Plan)
	return property.Mapping.Plan
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
