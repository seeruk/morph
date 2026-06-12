package morph

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/seeruk/morph/config"
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGeneratorGenerate(t *testing.T) {
	golden := goldie.New(t, goldie.WithFixtureDir("testdata/golden/generator"))

	t.Run("generates simple struct mapper", func(t *testing.T) {
		files, err := NewGenerator().Generate(generatorPlan(generatorSimpleStructRoot()))

		require.NoError(t, err)
		require.Len(t, files, 1)
		golden.Assert(t, "simple_struct_mapper", files[0].Source)
	})

	t.Run("generates enum mappers", func(t *testing.T) {
		files, err := NewGenerator().Generate(generatorPlan(
			generatorEnumRoot("MapStatus", spec.EnumFailureModeError),
			generatorEnumRoot("MapStatusOrZero", spec.EnumFailureModeZero),
		))

		require.NoError(t, err)
		require.Len(t, files, 1)
		golden.Assert(t, "enum_mappers", files[0].Source)
	})

	t.Run("generates container mappings", func(t *testing.T) {
		files, err := NewGenerator().Generate(generatorPlan(generatorContainerRoot()))

		require.NoError(t, err)
		require.Len(t, files, 1)
		golden.Assert(t, "container_mappings", files[0].Source)
	})

	t.Run("generates callables and adaptations", func(t *testing.T) {
		files, err := NewGenerator().Generate(generatorPlan(generatorCallableRoot()))

		require.NoError(t, err)
		require.Len(t, files, 1)
		golden.Assert(t, "callables_and_adaptations", files[0].Source)
	})

	t.Run("generates pointer mapper signatures", func(t *testing.T) {
		files, err := NewGenerator().Generate(generatorPlan(generatorPointerSignatureRoot()))

		require.NoError(t, err)
		require.Len(t, files, 1)
		golden.Assert(t, "pointer_mapper_signatures", files[0].Source)
	})

	t.Run("generates generated mapper calls and higher order arguments", func(t *testing.T) {
		root, nested := generatorHigherOrderRoot()
		files, err := NewGenerator().Generate(Plan{OutputGroups: []plan.OutputGroup{{
			Location: generatorLocation(),
			Roots:    []*plan.Type{root},
			Nested:   []*plan.Type{nested},
		}}})

		require.NoError(t, err)
		require.Len(t, files, 1)
		golden.Assert(t, "generated_mappers_and_higher_order_arguments", files[0].Source)
	})

	t.Run("generates import aliases", func(t *testing.T) {
		files, err := NewGenerator().Generate(generatorPlan(generatorImportAliasRoot()))

		require.NoError(t, err)
		require.Len(t, files, 1)
		golden.Assert(t, "import_aliases", files[0].Source)
	})

	t.Run("generates reflect fallback for non-comparable zero checks", func(t *testing.T) {
		files, err := NewGenerator().Generate(generatorPlan(generatorReflectFallbackRoot()))

		require.NoError(t, err)
		require.Len(t, files, 1)
		golden.Assert(t, "reflect_zero_fallback", files[0].Source)
	})

	t.Run("generates comparable struct zero checks", func(t *testing.T) {
		files, err := NewGenerator().Generate(generatorPlan(generatorComparableZeroRoot()))

		require.NoError(t, err)
		require.Len(t, files, 1)
		golden.Assert(t, "comparable_struct_zero_check", files[0].Source)
	})

	t.Run("generates multiple output groups", func(t *testing.T) {
		files, err := NewGenerator().Generate(Plan{OutputGroups: []plan.OutputGroup{
			{
				Location: plan.OutputLocation{
					LogicalPath: "/repo/one/morph.gen.go",
					ImportPath:  "module.test/one",
					PackageName: "one",
				},
				Roots: []*plan.Type{generatorSimpleStructRoot()},
			},
			{
				Location: plan.OutputLocation{
					LogicalPath: "/repo/two/morph.gen.go",
					ImportPath:  "module.test/two",
					PackageName: "two",
				},
				Roots: []*plan.Type{generatorSimpleStructRoot()},
			},
		}})

		require.NoError(t, err)
		require.Len(t, files, 2)
		golden.Assert(t, "multiple_output_groups_one", files[0].Source)
		golden.Assert(t, "multiple_output_groups_two", files[1].Source)
	})
}

func TestGeneratorGenerate_OutputCompilesWithRuntimeValueImportAndNameCollision(t *testing.T) {
	files, err := NewGenerator().Generate(generatorPlan(generatorComparableZeroRoot()))

	require.NoError(t, err)
	require.Len(t, files, 1)
	compileGeneratedPackage(t, files[0], map[string]string{
		"from/from.go": `package from

type Comparable struct {
	Name string
	Age  int
}

type ComparableSource struct {
	Value Comparable
}
`,
		"to/to.go": `package to

import "module.test/from"

type ComparableTarget struct {
	Value *from.Comparable
}
`,
	})
}

func TestGeneratorGenerate_OutputCompilesWithSamePackageUnexportedFields(t *testing.T) {
	sourceDecl := generatorStructDecl("module.test/out", "Source", map[string]types.Field{
		"secret": {Name: "secret", Type: basicTestType("string")},
	})
	targetDecl := generatorStructDecl("module.test/out", "Target", map[string]types.Field{
		"secret": {Name: "secret", Type: basicTestType("string")},
	})
	root := generatorRoot("MapSourceToTarget", sourceDecl, targetDecl)
	root.Location = plan.OutputLocation{
		LogicalPath: "/repo/out/morph.gen.go",
		ImportPath:  "module.test/out",
		PackageName: "out",
	}
	root.StructPlan = &plan.Struct{Properties: []plan.Property{{
		Source: generatorFieldMember(sourceDecl.Fields["secret"]),
		Target: generatorFieldMember(targetDecl.Fields["secret"]),
		Mapping: plan.Value{
			Operation: plan.OperationAssign,
			Source:    basicTestType("string"),
			Target:    basicTestType("string"),
		},
	}}}

	files, err := NewGenerator().Generate(Plan{OutputGroups: []plan.OutputGroup{{
		Location: root.Location,
		Roots:    []*plan.Type{root},
	}}})

	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Contains(t, string(files[0].Source), "source.secret")
	assert.Contains(t, string(files[0].Source), "target.secret")
	compileGeneratedPackage(t, files[0], map[string]string{
		"out/types.go": `package out

type Source struct {
	secret string
}

type Target struct {
	secret string
}
`,
	})
}

func TestGeneratorGenerate_OutputCompilesWithMethodAccessors(t *testing.T) {
	sourceDecl := generatorStructDecl("module.test/from", "Source", nil)
	targetDecl := generatorStructDecl("module.test/to", "Target", nil)
	root := generatorRoot("MapSourceToTarget", sourceDecl, targetDecl)
	root.StructPlan = &plan.Struct{Properties: []plan.Property{{
		Source: generatorMethodMember("Name", "Name", basicTestType("string"), false),
		Target: generatorMethodMember("Name", "SetName", basicTestType("string"), false),
		Mapping: plan.Value{
			Operation: plan.OperationAssign,
			Source:    basicTestType("string"),
			Target:    basicTestType("string"),
		},
	}}}

	files, err := NewGenerator().Generate(generatorPlan(root))

	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Contains(t, string(files[0].Source), "target.SetName(source.Name())")
	compileGeneratedPackage(t, files[0], map[string]string{
		"from/from.go": `package from

type Source struct{}

func (Source) Name() string { return "" }
`,
		"to/to.go": `package to

type Target struct{}

func (*Target) SetName(string) {}
`,
	})
}

func TestGeneratorGenerate_OutputCompilesWithErroringMethodAccessors(t *testing.T) {
	sourceDecl := generatorStructDecl("module.test/from", "Source", nil)
	targetDecl := generatorStructDecl("module.test/to", "Target", nil)
	root := generatorRoot("MapSourceToTarget", sourceDecl, targetDecl)
	root.CanError = true
	root.StructPlan = &plan.Struct{Properties: []plan.Property{{
		Source: generatorMethodMember("Name", "GetName", basicTestType("string"), true),
		Target: generatorMethodMember("Name", "SetName", basicTestType("string"), true),
		Mapping: plan.Value{
			Operation: plan.OperationAssign,
			Source:    basicTestType("string"),
			Target:    basicTestType("string"),
			CanError:  true,
		},
	}}}

	files, err := NewGenerator().Generate(generatorPlan(root))

	require.NoError(t, err)
	require.Len(t, files, 1)
	assert.Contains(t, string(files[0].Source), "nameValue, err := source.GetName()")
	assert.Contains(t, string(files[0].Source), "if err2 := target.SetName(nameValue); err2 != nil {")
	compileGeneratedPackage(t, files[0], map[string]string{
		"from/from.go": `package from

type Source struct{}

func (Source) GetName() (string, error) { return "", nil }
`,
		"to/to.go": `package to

type Target struct{}

func (*Target) SetName(string) error { return nil }
`,
	})
}

func TestGeneratorGenerate_WithFatalDiagnostics(t *testing.T) {
	_, err := NewGenerator().Generate(Plan{Diagnostics: []plan.Diagnostic{{
		Level:   plan.DiagnosticLevelFatal,
		Message: "fatal",
	}}})

	require.Error(t, err)
	assert.ErrorContains(t, err, "fatal diagnostics")
}

func TestGeneratorGenerate_WithUnsupportedMapping(t *testing.T) {
	root := generatorSimpleStructRoot()
	root.StructPlan.Properties[0].Mapping = plan.Value{
		Operation: plan.OperationUnsupported,
		Source:    basicTestType("string"),
		Target:    basicTestType("string"),
	}

	_, err := NewGenerator().Generate(generatorPlan(root))

	require.Error(t, err)
	assert.ErrorContains(t, err, "unsupported mapping")
}

func TestLocalNameBase(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "simple names",
			in:   "Count",
			want: "count",
		},
		{
			name: "trailing initialisms",
			in:   "UserID",
			want: "userID",
		},
		{
			name: "leading initialisms",
			in:   "URL",
			want: "url",
		},
		{
			name: "leading initialisms with words",
			in:   "HTTPClient",
			want: "httpClient",
		},
		{
			name: "numeric initialisms",
			in:   "MP4URL",
			want: "mp4URL",
		},
		{
			name: "numbers inside names",
			in:   "Line2Items",
			want: "line2Items",
		},
		{
			name: "plural initialisms",
			in:   "IDs",
			want: "ids",
		},
		{
			name: "keywords",
			in:   "type",
			want: "typeValue",
		},
		{
			name: "empty names",
			in:   "",
			want: "value",
		},
		{
			name: "leading digits",
			in:   "1Stop",
			want: "value1stop",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, localNameBase(tt.in))
		})
	}
}

func compileGeneratedPackage(t *testing.T, file OutputFile, supportFiles map[string]string) {
	t.Helper()

	repoRoot, err := os.Getwd()
	require.NoError(t, err)

	dir := t.TempDir()
	writeTestFile(t, dir, "go.mod", fmt.Sprintf(`module module.test

go 1.26.3

require github.com/seeruk/morph v0.0.0

replace github.com/seeruk/morph => %s
`, filepath.ToSlash(repoRoot)))
	for name, source := range supportFiles {
		writeTestFile(t, dir, name, source)
	}
	writeTestFile(t, dir, filepath.Join(file.PackageName, "morph.gen.go"), string(file.Source))

	cmd := exec.Command("go", "test", "./"+file.PackageName)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOWORK=off")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, string(out))
}

func writeTestFile(t *testing.T, dir, name, source string) {
	t.Helper()

	path := filepath.Join(dir, name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(source), 0o644))
}

func generatorPlan(roots ...*plan.Type) Plan {
	return Plan{OutputGroups: []plan.OutputGroup{{
		Location: generatorLocation(),
		Roots:    roots,
	}}}
}

func generatorLocation() plan.OutputLocation {
	return plan.OutputLocation{
		LogicalPath: "/repo/out/morph.gen.go",
		ImportPath:  "module.test/out",
		PackageName: "out",
	}
}

func generatorSimpleStructRoot() *plan.Type {
	sourceDecl := generatorStructDecl("module.test/from", "User", map[string]types.Field{
		"Name": generatorField("Name", basicTestType("string")),
		"Age":  generatorField("Age", basicTestType("int")),
	})
	targetDecl := generatorStructDecl("module.test/to", "User", map[string]types.Field{
		"Name": generatorField("Name", basicTestType("string")),
		"Age":  generatorField("Age", basicTestType("int")),
	})

	root := generatorRoot("MapUser", sourceDecl, targetDecl)
	root.StructPlan = &plan.Struct{Properties: []plan.Property{
		generatorPlanField(sourceDecl, targetDecl, "Name", plan.Value{
			Operation: plan.OperationAssign,
			Source:    basicTestType("string"),
			Target:    basicTestType("string"),
		}),
		generatorPlanField(sourceDecl, targetDecl, "Age", plan.Value{
			Operation: plan.OperationAssign,
			Source:    basicTestType("int"),
			Target:    basicTestType("int"),
		}),
	}}
	return root
}

func generatorEnumRoot(functionName string, failureMode spec.EnumFailureMode) *plan.Type {
	sourceDecl := generatorEnumDecl(
		"module.test/from", "Status",
		generatorConstant("module.test/from", "Status", "StatusReady", "1"),
		generatorConstant("module.test/from", "Status", "StatusDone", "2"),
	)
	targetDecl := generatorEnumDecl(
		"module.test/to", "Status",
		generatorConstant("module.test/to", "Status", "StatusReady", "1"),
		generatorConstant("module.test/to", "Status", "StatusDone", "2"),
	)

	root := generatorRoot(functionName, sourceDecl, targetDecl)
	root.CanError = failureMode == spec.EnumFailureModeError
	root.EnumPlan = &plan.Enum{
		FailureMode: failureMode,
		Values: []plan.EnumValue{
			{Source: sourceDecl.Constants["StatusReady"], Target: targetDecl.Constants["StatusReady"]},
			{Source: sourceDecl.Constants["StatusDone"], Target: targetDecl.Constants["StatusDone"]},
		},
	}
	return root
}

func generatorContainerRoot() *plan.Type {
	sourceID := generatorNamedScalarType("module.test/from", "UserID", "string")
	targetID := basicTestType("string")
	sourceDecl := generatorStructDecl("module.test/from", "Containers", map[string]types.Field{
		"IDs":    generatorField("IDs", sliceTestType(sourceID)),
		"Codes":  generatorField("Codes", arrayTestType(2, basicTestType("int"))),
		"Lookup": generatorField("Lookup", mapTestType(basicTestType("string"), sourceID)),
		"PtrIDs": generatorField("PtrIDs", sliceTestType(sourceID)),
	})
	targetDecl := generatorStructDecl("module.test/to", "Containers", map[string]types.Field{
		"IDs":    generatorField("IDs", sliceTestType(targetID)),
		"Codes":  generatorField("Codes", arrayTestType(2, basicTestType("int64"))),
		"Lookup": generatorField("Lookup", mapTestType(basicTestType("string"), targetID)),
		"PtrIDs": generatorField("PtrIDs", pointerTestType(sliceTestType(targetID))),
	})

	root := generatorRoot("MapContainers", sourceDecl, targetDecl)
	root.StructPlan = &plan.Struct{Properties: []plan.Property{
		generatorPlanField(sourceDecl, targetDecl, "IDs", plan.Value{
			Operation: plan.OperationSlice,
			Source:    sliceTestType(sourceID),
			Target:    sliceTestType(targetID),
			Elem: &plan.Value{
				Operation: plan.OperationConvert,
				Source:    sourceID,
				Target:    targetID,
			},
		}),
		generatorPlanField(sourceDecl, targetDecl, "Codes", plan.Value{
			Operation: plan.OperationArray,
			Source:    arrayTestType(2, basicTestType("int")),
			Target:    arrayTestType(2, basicTestType("int64")),
			Elem: &plan.Value{
				Operation: plan.OperationConvert,
				Source:    basicTestType("int"),
				Target:    basicTestType("int64"),
			},
		}),
		generatorPlanField(sourceDecl, targetDecl, "Lookup", plan.Value{
			Operation: plan.OperationMap,
			Source:    mapTestType(basicTestType("string"), sourceID),
			Target:    mapTestType(basicTestType("string"), targetID),
			Key: &plan.Value{
				Operation: plan.OperationAssign,
				Source:    basicTestType("string"),
				Target:    basicTestType("string"),
			},
			Value: &plan.Value{
				Operation: plan.OperationConvert,
				Source:    sourceID,
				Target:    targetID,
			},
		}),
		generatorPlanField(sourceDecl, targetDecl, "PtrIDs", plan.Value{
			Operation:         plan.OperationSlice,
			Source:            sliceTestType(sourceID),
			Target:            pointerTestType(sliceTestType(targetID)),
			TargetAdaptations: []plan.ValueAdaptation{plan.ValueAdaptationAddress},
			Optionality:       defaultOptionality(),
			Elem: &plan.Value{
				Operation: plan.OperationConvert,
				Source:    sourceID,
				Target:    targetID,
			},
		}),
	}}
	return root
}

func generatorCallableRoot() *plan.Type {
	sourceDecl := generatorStructDecl("module.test/from", "CallableSource", map[string]types.Field{
		"Count":    generatorField("Count", basicTestType("string")),
		"ID":       generatorField("ID", generatorNamedStructType("module.test/from", "MethodID")),
		"Maybe":    generatorField("Maybe", basicTestType("string")),
		"Named":    generatorField("Named", basicTestType("string")),
		"Required": generatorField("Required", pointerTestType(basicTestType("string"))),
		"Optional": generatorField("Optional", basicTestType("string")),
	})
	namedTarget := generatorNamedScalarType("module.test/to", "Stringy", "string")
	targetDecl := generatorStructDecl("module.test/to", "CallableTarget", map[string]types.Field{
		"Count":    generatorField("Count", basicTestType("int")),
		"ID":       generatorField("ID", basicTestType("string")),
		"Maybe":    generatorField("Maybe", basicTestType("string")),
		"Named":    generatorField("Named", pointerTestType(namedTarget)),
		"Required": generatorField("Required", basicTestType("string")),
		"Optional": generatorField("Optional", pointerTestType(basicTestType("string"))),
	})

	root := generatorRoot("MapCallable", sourceDecl, targetDecl)
	root.CanError = true
	errorOptionality := spec.Optionality{
		OnNilSourcePointer: spec.PointerOptionalityError,
		OnZeroSourceValue:  spec.ValueOptionalityNil,
	}
	root.StructPlan = &plan.Struct{Properties: []plan.Property{
		generatorPlanField(sourceDecl, targetDecl, "Count", plan.Value{
			Operation:   plan.OperationFunction,
			Source:      basicTestType("string"),
			Target:      basicTestType("int"),
			Callable:    generatorCallable("strconv", "Atoi", true),
			CanError:    true,
			Optionality: defaultOptionality(),
		}),
		generatorPlanField(sourceDecl, targetDecl, "ID", plan.Value{
			Operation:   plan.OperationMethod,
			Source:      generatorNamedStructType("module.test/from", "MethodID"),
			Target:      basicTestType("string"),
			Callable:    &plan.CallableRef{Kind: plan.CallableKindMethod, Name: "String"},
			Optionality: defaultOptionality(),
		}),
		generatorPlanField(sourceDecl, targetDecl, "Maybe", plan.Value{
			Operation:         plan.OperationFunction,
			Source:            basicTestType("string"),
			Target:            basicTestType("string"),
			Callable:          generatorCallable("module.test/from", "StringPtr", false),
			TargetAdaptations: []plan.ValueAdaptation{plan.ValueAdaptationDeref},
			Optionality:       errorOptionality,
			CanError:          true,
		}),
		generatorPlanField(sourceDecl, targetDecl, "Named", plan.Value{
			Operation:         plan.OperationConvert,
			Source:            basicTestType("string"),
			Target:            pointerTestType(namedTarget),
			TargetAdaptations: []plan.ValueAdaptation{plan.ValueAdaptationAddress},
			Optionality:       defaultOptionality(),
		}),
		generatorPlanField(sourceDecl, targetDecl, "Required", plan.Value{
			Operation:         plan.OperationAssign,
			Source:            pointerTestType(basicTestType("string")),
			Target:            basicTestType("string"),
			SourceAdaptations: []plan.ValueAdaptation{plan.ValueAdaptationDeref},
			Optionality:       errorOptionality,
			CanError:          true,
		}),
		generatorPlanField(sourceDecl, targetDecl, "Optional", plan.Value{
			Operation:         plan.OperationAssign,
			Source:            basicTestType("string"),
			Target:            pointerTestType(basicTestType("string")),
			TargetAdaptations: []plan.ValueAdaptation{plan.ValueAdaptationAddress},
			Optionality:       defaultOptionality(),
		}),
	}}
	return root
}

func generatorPointerSignatureRoot() *plan.Type {
	root := generatorSimpleStructRoot()
	root.FunctionName = "MapUserPointer"
	root.Signature = spec.MapperSignature{
		Accepts: spec.ParameterKindPointer,
		Returns: spec.ParameterKindPointer,
	}
	return root
}

func generatorHigherOrderRoot() (*plan.Type, *plan.Type) {
	sourceThingDecl := generatorStructDecl("module.test/from", "OptionalThing", map[string]types.Field{
		"Name": generatorField("Name", basicTestType("string")),
	})
	targetThingDecl := generatorStructDecl("module.test/to", "OptionalThing", map[string]types.Field{
		"Name": generatorField("Name", basicTestType("string")),
	})
	nested := generatorRoot("mapNestedOptionalThing", sourceThingDecl, targetThingDecl)
	nested.StructPlan = &plan.Struct{Properties: []plan.Property{
		generatorPlanField(sourceThingDecl, targetThingDecl, "Name", plan.Value{
			Operation: plan.OperationAssign,
			Source:    basicTestType("string"),
			Target:    basicTestType("string"),
		}),
	}}
	nested.Location = generatorLocation()

	sourceOptional := generatorNamedType("module.test/from", "Optional", sourceThingDecl.Type)
	targetOptional := generatorNamedType("module.test/to", "Optional", targetThingDecl.Type)
	sourceDecl := generatorStructDecl("module.test/from", "OptionalContainer", map[string]types.Field{
		"Maybe": generatorField("Maybe", sourceOptional),
	})
	targetDecl := generatorStructDecl("module.test/to", "OptionalContainer", map[string]types.Field{
		"Maybe": generatorField("Maybe", targetOptional),
	})
	root := generatorRoot("MapOptionalContainer", sourceDecl, targetDecl)
	root.StructPlan = &plan.Struct{Properties: []plan.Property{
		generatorPlanField(sourceDecl, targetDecl, "Maybe", plan.Value{
			Operation: plan.OperationFunction,
			Source:    sourceOptional,
			Target:    targetOptional,
			Callable:  generatorCallable("module.test/from", "MapOptional", false),
			CallableArgs: []plan.CallableArg{{
				Mapping: plan.Value{
					Operation: plan.OperationStruct,
					Source:    sourceThingDecl.Type,
					Target:    targetThingDecl.Type,
					Plan:      nested,
				},
			}},
			Optionality: defaultOptionality(),
		}),
	}}
	return root, nested
}

func generatorImportAliasRoot() *plan.Type {
	sourceDecl := generatorStructDecl("module.test/source/model", "User", nil)
	targetDecl := generatorStructDecl("module.test/target/model", "User", nil)
	root := generatorRoot("MapUser", sourceDecl, targetDecl)
	root.StructPlan = &plan.Struct{}
	return root
}

func generatorReflectFallbackRoot() *plan.Type {
	valueType := generatorNamedSliceStructType("module.test/from", "NonComparable")
	sourceDecl := generatorStructDecl("module.test/from", "ReflectSource", map[string]types.Field{
		"Value": generatorField("Value", valueType),
	})
	targetDecl := generatorStructDecl("module.test/to", "ReflectTarget", map[string]types.Field{
		"Value": generatorField("Value", pointerTestType(valueType)),
	})

	root := generatorRoot("MapReflectFallback", sourceDecl, targetDecl)
	root.StructPlan = &plan.Struct{Properties: []plan.Property{
		generatorPlanField(sourceDecl, targetDecl, "Value", plan.Value{
			Operation:         plan.OperationAssign,
			Source:            valueType,
			Target:            pointerTestType(valueType),
			TargetAdaptations: []plan.ValueAdaptation{plan.ValueAdaptationAddress},
			Optionality:       defaultOptionality(),
		}),
	}}
	return root
}

func generatorComparableZeroRoot() *plan.Type {
	valueType := generatorNamedComparableStructType("module.test/from", "Comparable")
	sourceDecl := generatorStructDecl("module.test/from", "ComparableSource", map[string]types.Field{
		"Value": generatorField("Value", valueType),
	})
	targetDecl := generatorStructDecl("module.test/to", "ComparableTarget", map[string]types.Field{
		"Value": generatorField("Value", pointerTestType(valueType)),
	})

	root := generatorRoot("MapComparableZero", sourceDecl, targetDecl)
	root.StructPlan = &plan.Struct{Properties: []plan.Property{
		generatorPlanField(sourceDecl, targetDecl, "Value", plan.Value{
			Operation:         plan.OperationAssign,
			Source:            valueType,
			Target:            pointerTestType(valueType),
			TargetAdaptations: []plan.ValueAdaptation{plan.ValueAdaptationAddress},
			Optionality:       defaultOptionality(),
		}),
	}}
	return root
}

func generatorRoot(functionName string, sourceDecl, targetDecl types.TypeDecl) *plan.Type {
	return &plan.Type{
		Source:       plan.TypeRefFromTypeDecl(sourceDecl),
		Target:       plan.TypeRefFromTypeDecl(targetDecl),
		SourceDecl:   sourceDecl,
		TargetDecl:   targetDecl,
		SourceType:   sourceDecl.Type,
		TargetType:   targetDecl.Type,
		FunctionName: functionName,
		Location:     generatorLocation(),
		Signature:    defaultMapperSignature,
		Optionality:  defaultOptionality(),
		Conversions:  defaultConversionsPolicy(),
	}
}

func generatorPlanField(sourceDecl, targetDecl types.TypeDecl, name string, mapping plan.Value) plan.Property {
	return plan.Property{
		Source:  generatorFieldMember(sourceDecl.Fields[name]),
		Target:  generatorFieldMember(targetDecl.Fields[name]),
		Mapping: mapping,
	}
}

func generatorFieldMember(field types.Field) plan.Member {
	return plan.Member{
		Name:     field.Name,
		Accessor: field.Name,
		Kind:     plan.MemberKindField,
		Type:     field.Type,
	}
}

func generatorMethodMember(name, accessor string, typ types.Type, canError bool) plan.Member {
	return plan.Member{
		Name:     name,
		Accessor: accessor,
		Kind:     plan.MemberKindMethod,
		Type:     typ,
		CanError: canError,
	}
}

func generatorStructDecl(importPath, name string, fields map[string]types.Field) types.TypeDecl {
	underlying := types.Type{
		Kind:   types.TypeKindStruct,
		String: "struct{}",
		Fields: fields,
	}
	typ := generatorNamedType(importPath, name)
	typ.Elem = &underlying
	return types.TypeDecl{
		Name:       name,
		Package:    generatorPackage(importPath),
		Type:       typ,
		Underlying: underlying,
		Fields:     fields,
	}
}

func generatorEnumDecl(importPath, name string, constants ...types.ConstantDecl) types.TypeDecl {
	underlying := basicTestType("int")
	typ := generatorNamedType(importPath, name)
	typ.Elem = &underlying

	constantsByName := make(map[string]types.ConstantDecl, len(constants))
	for _, constant := range constants {
		constantsByName[constant.Name] = constant
	}

	return types.TypeDecl{
		Name:       name,
		Package:    generatorPackage(importPath),
		Type:       typ,
		Underlying: underlying,
		Constants:  constantsByName,
	}
}

func generatorConstant(importPath, typeName, name, value string) types.ConstantDecl {
	typ := generatorNamedType(importPath, typeName)
	typ.Elem = new(types.Type)
	*typ.Elem = basicTestType("int")
	return types.ConstantDecl{
		Package:    generatorPackage(importPath),
		Name:       name,
		Type:       typ,
		Value:      value,
		IsExported: true,
	}
}

func generatorField(name string, typ types.Type) types.Field {
	return types.Field{
		Name:       name,
		Type:       typ,
		IsExported: true,
	}
}

func generatorNamedType(importPath, name string, args ...types.Type) types.Type {
	return types.Type{
		Kind:     types.TypeKindNamed,
		Name:     name,
		Package:  generatorPackage(importPath),
		TypeArgs: args,
	}
}

func generatorNamedScalarType(importPath, name, underlying string) types.Type {
	typ := generatorNamedType(importPath, name)
	typ.Elem = new(types.Type)
	*typ.Elem = basicTestType(underlying)
	return typ
}

func generatorNamedStructType(importPath, name string) types.Type {
	typ := generatorNamedType(importPath, name)
	typ.Elem = &types.Type{Kind: types.TypeKindStruct, String: "struct{}"}
	return typ
}

func generatorNamedSliceStructType(importPath, name string) types.Type {
	fieldType := sliceTestType(basicTestType("string"))
	typ := generatorNamedType(importPath, name)
	typ.Elem = &types.Type{
		Kind: types.TypeKindStruct,
		Fields: map[string]types.Field{
			"Values": generatorField("Values", fieldType),
		},
		String: "struct{ Values []string }",
	}
	return typ
}

func generatorNamedComparableStructType(importPath, name string) types.Type {
	typ := generatorNamedType(importPath, name)
	typ.Elem = &types.Type{
		Kind: types.TypeKindStruct,
		Fields: map[string]types.Field{
			"Name": generatorField("Name", basicTestType("string")),
			"Age":  generatorField("Age", basicTestType("int")),
		},
		String: "struct{ Name string; Age int }",
	}
	return typ
}

func generatorPackage(importPath string) types.PackageRef {
	return types.PackageRef{
		Name:       path.Base(importPath),
		ImportPath: importPath,
	}
}

func generatorCallable(importPath, name string, returnsError bool) *plan.CallableRef {
	return &plan.CallableRef{
		Kind:         plan.CallableKindFunction,
		Package:      generatorPackage(importPath),
		Name:         name,
		ReturnsError: returnsError,
	}
}

func configWithUnsupportedMapping() config.Config {
	return config.Config{
		Packages: []config.Package{{
			Source: "github.com/seeruk/morph/testdata/planner/from",
			Target: "github.com/seeruk/morph/testdata/planner/to",
			Types: []config.Type{{
				Name: "OptionalBadContainer",
			}},
		}},
	}
}
