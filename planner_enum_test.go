package morph

import (
	"maps"
	"slices"
	"testing"

	"github.com/seeruk/morph/internal/slicesx"
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
)

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
