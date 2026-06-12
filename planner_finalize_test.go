package morph

import (
	"testing"

	"github.com/seeruk/morph/config"
	"github.com/seeruk/morph/plan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPlannerFinalizesRecursiveGeneratedMapperErrability(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
		Defaults: config.Defaults{
			Packages: config.PackagesDefaults{
				Types: config.TypesDefaults{
					Optionality: pointerErrorOptionalityDefaults(),
				},
			},
		},
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "Container",
			}},
		}},
	})

	root := requireSingleRoot(t, out)
	first := requirePropertyPlan(t, root.StructPlan, "First")
	children := requirePlanProperty(t, first.StructPlan, "Children")
	childrenElem := requireValueElem(t, children.Mapping)
	required := requirePlanProperty(t, first.StructPlan, "Required")

	t.Run("propagates recursive errability", func(t *testing.T) {
		assert.True(t, root.CanError)
		assert.True(t, first.CanError)
		assert.True(t, children.Mapping.CanError)
		assert.True(t, childrenElem.CanError)
		assert.True(t, required.Mapping.CanError)
	})

	t.Run("keeps recursive element adaptations local", func(t *testing.T) {
		assert.Empty(t, childrenElem.SourceAdaptations)
		assert.Empty(t, childrenElem.TargetAdaptations)
	})
}

func TestPlannerRetriesHigherOrderFunctionWhenRecursiveArgCanError(t *testing.T) {
	out := planWithConfig(t, ".", config.Config{
		Defaults: config.Defaults{
			Packages: config.PackagesDefaults{
				Types: config.TypesDefaults{
					Optionality: pointerErrorOptionalityDefaults(),
				},
			},
		},
		Discovery: config.Discovery{
			Packages: []string{
				"github.com/seeruk/morph/testdata/planner/from",
			},
		},
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "OptionalRecursiveNode",
			}},
		}},
	})

	require.False(t, out.HasFatalDiagnostics())

	root := requireSingleRoot(t, out)
	maybe := requirePlanProperty(t, root.StructPlan, "Maybe")
	arg := requireCallableArg(t, maybe.Mapping, 0)
	elem := requireValueElem(t, arg.Mapping)

	t.Run("selects the erroring higher-order callable", func(t *testing.T) {
		assert.True(t, root.CanError)
		assert.Equal(t, "MapOptionalWithError", maybe.Mapping.Callable.Name)
		assert.True(t, maybe.Mapping.Callable.ReturnsError)
		assert.True(t, maybe.Mapping.CanError)
	})

	t.Run("replans the callable argument with error support", func(t *testing.T) {
		assert.True(t, arg.ReturnsError)
		assert.True(t, arg.Mapping.CanError)
		assert.Equal(t, plan.OperationSlice, arg.Mapping.Operation)
		assert.True(t, elem.CanError)
	})

	t.Run("keeps recursive element adaptations local", func(t *testing.T) {
		assert.Empty(t, elem.SourceAdaptations)
		assert.Empty(t, elem.TargetAdaptations)
	})
}
