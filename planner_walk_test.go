package morph

import (
	"testing"

	"github.com/seeruk/morph/plan"
	"github.com/stretchr/testify/assert"
)

func TestWalkOutputGroups(t *testing.T) {
	t.Run("walks roots and nested helpers once", func(t *testing.T) {
		location := testOutputLocation("module.test/out", "/repo/out/morph.gen.go")
		nested := testWalkType(location, "NestedSource", "NestedTarget", "MapNested")
		root := testWalkType(location, "RootSource", "RootTarget", "MapRoot")
		root.StructPlan = &plan.Struct{Fields: []plan.Field{{
			SourceField: testField("Nested", nested.SourceType),
			TargetField: testField("Nested", nested.TargetType),
			Mapping: plan.Value{
				Operation: plan.OperationStruct,
				Plan:      nested,
			},
		}}}

		var names []string
		walkOutputGroups([]plan.OutputGroup{{
			Location: location,
			Roots:    []*plan.Type{root},
			Nested:   []*plan.Type{nested},
		}}, walkCallbacks{
			TypePre: func(ctx walkContext) {
				names = append(names, ctx.Type.FunctionName)
			},
		})

		assert.Equal(t, []string{"MapRoot", "MapNested"}, names)
	})

	t.Run("walks values with stable diagnostic paths", func(t *testing.T) {
		location := testOutputLocation("module.test/out", "/repo/out/morph.gen.go")
		root := testWalkType(location, "Source", "Target", "MapRoot")
		sourceField := testField("Input", basicTestType("string"))
		targetField := testField("Output", basicTestType("int"))
		root.StructPlan = &plan.Struct{Fields: []plan.Field{{
			SourceField: sourceField,
			TargetField: targetField,
			Mapping: plan.Value{
				Operation: plan.OperationMap,
				CallableArgs: []plan.CallableArg{{
					Mapping: plan.Value{Operation: plan.OperationAssign},
				}},
				Key:   &plan.Value{Operation: plan.OperationAssign},
				Value: &plan.Value{Operation: plan.OperationAssign},
			},
		}}}
		fieldPath := plan.FieldPath(root.SourceType, root.TargetType, sourceField, targetField)

		var paths []string
		walkOutputGroups([]plan.OutputGroup{{
			Location: location,
			Roots:    []*plan.Type{root},
		}}, walkCallbacks{
			ValuePre: func(ctx walkContext) {
				paths = append(paths, ctx.Path)
			},
		})

		assert.Equal(t, []string{
			fieldPath,
			callableArgPath(fieldPath, 0),
			fieldPath + "[key]",
			fieldPath + "[value]",
		}, paths)
	})

	t.Run("handles recursive generated mapper plans", func(t *testing.T) {
		location := testOutputLocation("module.test/out", "/repo/out/morph.gen.go")
		root := testWalkType(location, "Node", "Node", "MapNode")
		root.StructPlan = &plan.Struct{Fields: []plan.Field{{
			SourceField: testField("Next", root.SourceType),
			TargetField: testField("Next", root.TargetType),
			Mapping: plan.Value{
				Operation: plan.OperationStruct,
				Plan:      root,
			},
		}}}

		var typeCount, valueCount int
		walkOutputGroups([]plan.OutputGroup{{
			Location: location,
			Roots:    []*plan.Type{root},
		}}, walkCallbacks{
			TypePre: func(walkContext) {
				typeCount++
			},
			ValuePre: func(walkContext) {
				valueCount++
			},
		})

		assert.Equal(t, 1, typeCount)
		assert.Equal(t, 1, valueCount)
	})
}

func testWalkType(location plan.OutputLocation, sourceName, targetName, functionName string) *plan.Type {
	source := namedTestType("module.test/source", sourceName)
	target := namedTestType("module.test/target", targetName)
	return &plan.Type{
		Source:       plan.TypeRefFromType(source),
		Target:       plan.TypeRefFromType(target),
		SourceType:   source,
		TargetType:   target,
		FunctionName: functionName,
		Location:     location,
	}
}
