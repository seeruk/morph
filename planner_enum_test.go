package morph

import (
	"maps"
	"slices"
	"testing"

	"github.com/seeruk/morph/internal/slicesx"
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlanner_planEnumValues(t *testing.T) {
	t.Run("does not mark normalized names as seen source constants", func(t *testing.T) {
		sourceDecl := testEnumTypeDecl(
			"Status",
			testConstantDecl("Status", "OK", "ok"),
			testConstantDecl("Status", "StatusOK", "ok"),
		)
		targetDecl := testEnumTypeDecl(
			"Status",
			testConstantDecl("Status", "OK", "ok"),
		)
		typ := plan.Type{
			SourceDecl: sourceDecl,
			TargetDecl: targetDecl,
			SourceType: sourceDecl.Type,
			TargetType: targetDecl.Type,
		}
		planner := &Planner{}

		values, diagnostics := planner.planEnumValues(&typ)

		require.Empty(t, diagnostics)
		require.Len(t, values, 2)

		sourceNames := make([]string, 0, len(values))
		for _, value := range values {
			sourceNames = append(sourceNames, value.Source.Name)
			assert.Equal(t, "OK", value.Target.Name)
		}

		assert.Equal(t, []string{"OK", "StatusOK"}, sourceNames)
	})

	t.Run("reports invalid explicit enum value mappings as fatal", func(t *testing.T) {
		sourceDecl := testEnumTypeDecl(
			"Status",
			testConstantDecl("Status", "StatusOK", "ok"),
		)
		targetDecl := testEnumTypeDecl(
			"Status",
			testConstantDecl("Status", "StatusOK", "ok"),
		)
		typ := plan.Type{
			SourceDecl: sourceDecl,
			TargetDecl: targetDecl,
			SourceType: sourceDecl.Type,
			TargetType: targetDecl.Type,
			EnumSpec: spec.Enum{
				Values: map[string]string{
					"StatusMissing": "StatusOK",
					"StatusOK":      "StatusMissing",
				},
			},
		}
		planner := &Planner{}

		_, diagnostics := planner.planEnumValues(&typ)

		require.Len(t, diagnostics, 2)
		assert.True(t, plan.HasFatalDiagnostics(diagnostics))
		assert.Contains(t, diagnosticsPaths(diagnostics), plan.SourceEnumValuePath(typ.SourceType, typ.TargetType, "StatusMissing"))
		assert.Contains(t, diagnosticsPaths(diagnostics), plan.TargetEnumValuePath(typ.SourceType, typ.TargetType, "StatusMissing"))
		assert.Contains(t, diagnosticsMessages(diagnostics), `source enum value "StatusMissing" does not exist or is not exported`)
		assert.Contains(t, diagnosticsMessages(diagnostics), `target enum value "StatusMissing" configured for source enum value "StatusOK" does not exist or is not exported`)
	})

	t.Run("reports missing inferred target matches as actionable fatal diagnostics", func(t *testing.T) {
		sourceDecl := testEnumTypeDecl(
			"Status",
			testConstantDecl("Status", "StatusOK", "ok"),
		)
		targetDecl := testEnumTypeDecl(
			"Status",
			testConstantDecl("Status", "StatusReady", "ready"),
		)
		typ := plan.Type{
			SourceDecl: sourceDecl,
			TargetDecl: targetDecl,
			SourceType: sourceDecl.Type,
			TargetType: targetDecl.Type,
		}
		planner := &Planner{}

		_, diagnostics := planner.planEnumValues(&typ)

		require.Len(t, diagnostics, 1)
		assert.Equal(t, plan.DiagnosticLevelFatal, diagnostics[0].Level)
		assert.Equal(t, plan.SourceEnumValuePath(typ.SourceType, typ.TargetType, "StatusOK"), diagnostics[0].Path)
		assert.Equal(t, `no target enum value matched source enum value "StatusOK" normalized as "OK"; configure enum.values or enum.patterns`, diagnostics[0].Message)
	})
}

func Test_normalizeEnumConstants(t *testing.T) {
	tt := []struct {
		name      string
		pattern   string
		in        []types.ConstantDecl
		out       []string
		ambiguous map[string][]string
		diags     []plan.Diagnostic
	}{
		{
			name: "supported auto cases",
			in: []types.ConstantDecl{
				testConstantDecl("Example", "ExampleFoo", "foo"),
				testConstantDecl("Example", "Example_BAR", "bar"),
				testConstantDecl("TestExample", "TestExample_TEST_EXAMPLE_EnumValue", "enum_value"),
				testConstantDecl("TestExample", "TestExample_TEST_EXAMPLE_extremelyWeird-value_kind", "unrelated_actual_value"),
			},
			out: []string{
				"FOO",
				"BAR",
				"ENUM_VALUE",
				"EXTREMELY_WEIRD_VALUE_KIND",
			},
		},
		{
			name:    "protobuf-style pattern cases",
			pattern: "{{ .Type.Pascal }}_{{ .Type.Screaming }}_{{ .Value.Screaming }}",
			in: []types.ConstantDecl{
				testConstantDecl("TestExample", "TestExample_TEST_EXAMPLE_SOME_VALUE", "some_value"),
				testConstantDecl("TestExample", "TestExample_TEST_EXAMPLE_ANOTHER_VALUE", "value_another"),
			},
			out: []string{
				"SOME_VALUE",
				"ANOTHER_VALUE",
			},
		},
		{
			name:    "multi-value pattern cases",
			pattern: "{{ .Type.Pascal }}_{{ .Value.Screaming }}_{{ .Value.Camel }}",
			in: []types.ConstantDecl{
				testConstantDecl("TestExample", "TestExample_SOME_VALUE_someValue", "some_value"),
				testConstantDecl("TestExample", "TestExample_ANOTHER_VALUE_anotherValue", "value_another"),
			},
			out: []string{
				"SOME_VALUE",
				"ANOTHER_VALUE",
			},
		},
		{
			name:    "invalid screaming-screaming pattern",
			pattern: "{{ .Type.Pascal }}_{{ .Value.Screaming }}_{{ .Value.Screaming }}",
			in: []types.ConstantDecl{
				testConstantDecl("TestExample", "TestExample_SOME_VALUE_SOME_VALUE", "some_value"),
				testConstantDecl("TestExample", "TestExample_ANOTHER_VALUE_ANOTHER_VALUE", "value_another"),
			},
			diags: []plan.Diagnostic{
				{
					Level:   plan.DiagnosticLevelFatal,
					Path:    "TestExample :: enum value TestExample_SOME_VALUE_SOME_VALUE",
					Message: "failed to normalize enum constant name: enum pattern template validation failed: ambiguous enum value boundary between Screaming and Screaming using \"_\"; configure enum.patterns or enum.values",
				},
				{
					Level:   plan.DiagnosticLevelFatal,
					Path:    "TestExample :: enum value TestExample_ANOTHER_VALUE_ANOTHER_VALUE",
					Message: "failed to normalize enum constant name: enum pattern template validation failed: ambiguous enum value boundary between Screaming and Screaming using \"_\"; configure enum.patterns or enum.values",
				},
			},
		},
		{
			name: "ambiguous enum values",
			in: []types.ConstantDecl{
				testConstantDecl("TestExample", "TestExample_TestValue", "test_value"),
				testConstantDecl("TestExample", "TestExample_TEST_VALUE", "test_value"),
			},
			ambiguous: map[string][]string{
				"TEST_VALUE": {"TestExample_TEST_VALUE", "TestExample_TestValue"},
			},
		},
		{
			name: "ambiguous auto type name and value",
			in: []types.ConstantDecl{
				testConstantDecl("Test", "Test_TestValue", "test_value"),
			},
			out: []string{
				"TEST_VALUE",
			},
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			input := make(map[string]types.ConstantDecl, len(tc.in))
			input = slicesx.Reduce(tc.in, input, func(acc map[string]types.ConstantDecl, decl types.ConstantDecl) map[string]types.ConstantDecl {
				acc[decl.Name] = decl
				return acc
			})

			out, ambiguous, diags := constantsByNormalizedName(input, tc.pattern, nil, "")
			if len(tc.out) > 0 {
				assert.ElementsMatch(t, tc.out, slices.Collect(maps.Keys(out)))
			} else {
				assert.Empty(t, out)
			}

			if len(tc.ambiguous) > 0 {
				assert.Equal(t, tc.ambiguous, ambiguous)
			} else {
				assert.Empty(t, ambiguous)
			}

			if len(tc.diags) > 0 {
				assert.ElementsMatch(t, tc.diags, diags)
			} else {
				assert.Empty(t, diags)
			}
		})
	}
}

func testEnumTypeDecl(name string, constants ...types.ConstantDecl) types.TypeDecl {
	typ := basicTestType(name)
	constantsByName := make(map[string]types.ConstantDecl, len(constants))
	for _, constant := range constants {
		constantsByName[constant.Name] = constant
	}

	return types.TypeDecl{
		Name:       name,
		Type:       typ,
		Underlying: typ,
		Constants:  constantsByName,
	}
}

func Test_casingScreamingSnake(t *testing.T) {
	tt := map[string]string{
		"example_value":    "EXAMPLE_VALUE",
		"ExampleValue":     "EXAMPLE_VALUE",
		"single":           "SINGLE",
		"multi-word-value": "MULTI_WORD_VALUE",
		"mp3-player":       "MP3_PLAYER",
	}

	for input, expected := range tt {
		t.Run(input, func(t *testing.T) {
			assert.Equal(t, expected, casingScreamingSnake(input))
		})
	}
}
