package morph

import (
	"testing"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
)

func TestWalkOutputGroups(t *testing.T) {
	t.Run("walks roots and nested helpers once", func(t *testing.T) {
		location := testOutputLocation("module.test/out", "/repo/out/morph.gen.go")
		nested := testWalkType(location, "NestedSource", "NestedTarget", "MapNested")
		root := testWalkType(location, "RootSource", "RootTarget", "MapRoot")
		root.StructPlan = &plan.Struct{Properties: []plan.Property{{
			Source: testFieldMember("Nested", nested.SourceType),
			Target: testFieldMember("Nested", nested.TargetType),
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
		sourceMember := testFieldMember("Input", basicTestType("string"))
		targetMember := testFieldMember("Output", basicTestType("int"))
		root.StructPlan = &plan.Struct{Properties: []plan.Property{{
			Source: sourceMember,
			Target: targetMember,
			Mapping: plan.Value{
				Operation: plan.OperationMap,
				CallableMapperArgs: []plan.CallableMapperArg{{
					Mapping: plan.Value{Operation: plan.OperationAssign},
				}},
				Key:   &plan.Value{Operation: plan.OperationAssign},
				Value: &plan.Value{Operation: plan.OperationAssign},
			},
		}}}
		propertyPath := plan.PropertyPath(root.SourceType, root.TargetType, sourceMember.Name, targetMember.Name)

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
			propertyPath,
			callableMapperArgPath(propertyPath, 0),
			propertyPath + "[key]",
			propertyPath + "[value]",
		}, paths)
	})

	t.Run("handles recursive generated mapper plans", func(t *testing.T) {
		location := testOutputLocation("module.test/out", "/repo/out/morph.gen.go")
		root := testWalkType(location, "Node", "Node", "MapNode")
		root.StructPlan = &plan.Struct{Properties: []plan.Property{{
			Source: testFieldMember("Next", root.SourceType),
			Target: testFieldMember("Next", root.TargetType),
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

func testFieldMember(name string, typ types.Type) plan.Member {
	return plan.Member{
		Name:     name,
		Accessor: name,
		Kind:     plan.MemberKindField,
		Type:     typ,
	}
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
