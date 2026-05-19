package types_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	gotypes "go/types"

	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewLoader_DefaultDir(t *testing.T) {
	loader := types.NewLoader("")

	err := loader.Load(context.Background(), "./testdata/alpha")

	require.NoError(t, err)
	assert.Len(t, loader.Packages(), 1)
}

func TestLoaderLoad_LoadsPackages(t *testing.T) {
	loader := loadFixture(t, "./alpha", "./beta")
	pkgs := loader.Packages()

	require.Len(t, pkgs, 2)
	assert.Equal(t, []string{
		"github.com/seeruk/morph/types/testdata/alpha",
		"github.com/seeruk/morph/types/testdata/beta",
	}, []string{pkgs[0].ImportPath, pkgs[1].ImportPath})

	alpha := findPackage(t, pkgs, "alpha")
	assert.Equal(t, "alpha", alpha.Name)
	assert.NotEmpty(t, alpha.Dir)
	assert.NotEmpty(t, alpha.Constants)
	assert.NotEmpty(t, alpha.Functions)
	assert.NotEmpty(t, alpha.Types)

	beta := findPackage(t, pkgs, "beta")
	assert.Equal(t, "beta", beta.Name)
	assert.NotEmpty(t, beta.Dir)
	assert.NotEmpty(t, beta.Types)
}

func TestLoaderLoad_ReturnsPackageErrors(t *testing.T) {
	loader := types.NewLoader("testdata")

	err := loader.Load(context.Background(), "./invalid")

	require.Error(t, err)
	assert.ErrorContains(t, err, "types: loaded packages")
	assert.ErrorContains(t, err, "DoesNotExist")
}

func TestLoaderLoad_LoadsConstants(t *testing.T) {
	pkg := loadAlphaPackage(t)

	tests := []struct {
		name     string
		exported bool
		value    string
		typeKind types.TypeKind
		typeName string
	}{
		{
			name:     "DifficultyEasy",
			exported: true,
			value:    "0",
			typeKind: types.TypeKindNamed,
			typeName: "Difficulty",
		},
		{
			name:     "difficultyMax",
			exported: false,
			value:    "2",
			typeKind: types.TypeKindNamed,
			typeName: "Difficulty",
		},
		{
			name:     "Message",
			exported: true,
			value:    "\"hello\"",
			typeKind: types.TypeKindBasic,
			typeName: "untyped string",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			constant := findConstant(t, pkg, tt.name)

			assert.Equal(t, tt.name, constant.Name)
			assert.Equal(t, tt.exported, constant.Exported)
			assert.Equal(t, tt.value, constant.Value)
			assert.Equal(t, tt.typeKind, constant.Type.Kind)
			assert.Equal(t, tt.typeName, constant.Type.Name)
			assert.Equal(t, pkg.AsRef(), constant.Package)
		})
	}
}

func TestLoaderLoad_LoadsFunctions(t *testing.T) {
	pkg := loadAlphaPackage(t)

	tests := []struct {
		name          string
		exported      bool
		paramKinds    []types.TypeKind
		resultKinds   []types.TypeKind
		typeParamName string
		variadic      bool
	}{
		{
			name:        "Exported",
			exported:    true,
			paramKinds:  []types.TypeKind{types.TypeKindBasic, types.TypeKindBasic},
			resultKinds: []types.TypeKind{types.TypeKindBasic, types.TypeKindNamed},
		},
		{
			name:          "generic",
			exported:      false,
			paramKinds:    []types.TypeKind{types.TypeKindTypeParam},
			resultKinds:   []types.TypeKind{types.TypeKindTypeParam},
			typeParamName: "T",
		},
		{
			name:        "Variadic",
			exported:    true,
			paramKinds:  []types.TypeKind{types.TypeKindBasic, types.TypeKindSlice},
			resultKinds: []types.TypeKind{types.TypeKindSlice},
			variadic:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fn := findFunction(t, pkg, tt.name)

			assert.Equal(t, tt.name, fn.Name)
			assert.Equal(t, tt.exported, fn.Exported)
			assert.Equal(t, tt.variadic, fn.Variadic)
			assert.Equal(t, tt.paramKinds, parameterKinds(fn.Params))
			assert.Equal(t, tt.resultKinds, parameterKinds(fn.Results))
			assert.Equal(t, pkg.AsRef(), fn.Package)
			assert.Contains(t, filepath.ToSlash(fn.SourceFile), "/types/testdata/alpha/")
			assert.False(t, fn.IsMorphFile)

			if tt.typeParamName != "" {
				require.Len(t, fn.TypeParams, 1)
				assert.Equal(t, tt.typeParamName, fn.TypeParams[0].Name)
				assert.Equal(t, types.TypeKindInterface, fn.TypeParams[0].Constraint.Kind)
			}
		})
	}
}

func TestLoaderLoad_LoadsTypes(t *testing.T) {
	pkg := loadAlphaPackage(t)

	tests := []struct {
		name           string
		alias          bool
		typeKind       types.TypeKind
		underlyingKind types.TypeKind
		elemKind       types.TypeKind
		typeParamName  string
	}{
		{
			name:           "Difficulty",
			typeKind:       types.TypeKindNamed,
			underlyingKind: types.TypeKindBasic,
			elemKind:       types.TypeKindBasic,
		},
		{
			name:           "ID",
			alias:          true,
			typeKind:       types.TypeKindAlias,
			underlyingKind: types.TypeKindBasic,
			elemKind:       types.TypeKindBasic,
		},
		{
			name:           "Pair",
			typeKind:       types.TypeKindNamed,
			underlyingKind: types.TypeKindStruct,
			elemKind:       types.TypeKindStruct,
			typeParamName:  "T",
		},
		{
			name:           "Reader",
			typeKind:       types.TypeKindNamed,
			underlyingKind: types.TypeKindInterface,
			elemKind:       types.TypeKindInterface,
		},
		{
			name:           "FuncType",
			typeKind:       types.TypeKindNamed,
			underlyingKind: types.TypeKindSignature,
			elemKind:       types.TypeKindSignature,
		},
		{
			name:           "Complex",
			typeKind:       types.TypeKindNamed,
			underlyingKind: types.TypeKindStruct,
			elemKind:       types.TypeKindStruct,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decl := findType(t, pkg, tt.name)

			assert.Equal(t, tt.alias, decl.Alias)
			assert.Equal(t, tt.typeKind, decl.Type.Kind)
			assert.Equal(t, tt.underlyingKind, decl.Underlying.Kind)
			assert.Equal(t, tt.name, decl.Type.Name)
			assert.Equal(t, pkg.AsRef(), decl.Package)

			require.NotNil(t, decl.Type.Elem)
			assert.Equal(t, tt.elemKind, decl.Type.Elem.Kind)

			if tt.typeParamName != "" {
				require.Len(t, decl.Type.TypeParams, 1)
				assert.Equal(t, tt.typeParamName, decl.Type.TypeParams[0].Name)
			}
		})
	}
}

func TestLoaderLoad_LoadsStructFieldsAndMethods(t *testing.T) {
	pkg := loadAlphaPackage(t)
	pair := findType(t, pkg, "Pair")

	require.Len(t, pair.Fields, 3)
	assert.Equal(t, "Embedded", pair.Fields[0].Name)
	assert.True(t, pair.Fields[0].Embedded)
	assert.Equal(t, types.TypeKindPointer, pair.Fields[0].Type.Kind)
	require.NotNil(t, pair.Fields[0].Type.Elem)
	assert.Equal(t, "Embedded", pair.Fields[0].Type.Elem.Name)

	assert.Equal(t, "Source", pair.Fields[1].Name)
	assert.True(t, pair.Fields[1].Exported)
	assert.False(t, pair.Fields[1].Embedded)
	assert.Equal(t, `json:"source"`, pair.Fields[1].Tag)
	assert.Equal(t, types.TypeKindTypeParam, pair.Fields[1].Type.Kind)

	require.Len(t, pair.Methods, 2)
	assert.Equal(t, []string{"Invert", "Set"}, []string{pair.Methods[0].Name, pair.Methods[1].Name})

	invert := pair.Methods[0]
	assert.True(t, invert.Exported)
	require.NotNil(t, invert.Receiver)
	assert.Equal(t, types.TypeKindNamed, invert.Receiver.Type.Kind)
	assert.Empty(t, invert.Params)
	assert.Equal(t, []types.TypeKind{types.TypeKindNamed}, parameterKinds(invert.Results))

	set := pair.Methods[1]
	assert.True(t, set.Exported)
	require.NotNil(t, set.Receiver)
	assert.Equal(t, types.TypeKindPointer, set.Receiver.Type.Kind)
	assert.Equal(t, []types.TypeKind{types.TypeKindTypeParam, types.TypeKindTypeParam}, parameterKinds(set.Params))
	assert.Empty(t, set.Results)
}

func TestLoaderLoad_LoadsCompositeTypeShapes(t *testing.T) {
	pkg := loadAlphaPackage(t)
	complexType := findType(t, pkg, "Complex")

	tests := []struct {
		name      string
		kind      types.TypeKind
		elemKind  types.TypeKind
		keyKind   types.TypeKind
		valueKind types.TypeKind
		arrayLen  int64
		chanDir   gotypes.ChanDir
	}{
		{name: "Ptr", kind: types.TypeKindPointer, elemKind: types.TypeKindNamed},
		{name: "Slice", kind: types.TypeKindSlice, elemKind: types.TypeKindBasic},
		{name: "Array", kind: types.TypeKindArray, elemKind: types.TypeKindBasic, arrayLen: 2},
		{name: "Map", kind: types.TypeKindMap, keyKind: types.TypeKindBasic, valueKind: types.TypeKindNamed},
		{name: "SendOnly", kind: types.TypeKindChan, elemKind: types.TypeKindBasic, chanDir: gotypes.SendOnly},
		{name: "RecvOnly", kind: types.TypeKindChan, elemKind: types.TypeKindBasic, chanDir: gotypes.RecvOnly},
		{name: "Both", kind: types.TypeKindChan, elemKind: types.TypeKindBasic, chanDir: gotypes.SendRecv},
		{name: "Reader", kind: types.TypeKindNamed, elemKind: types.TypeKindInterface},
		{name: "Handler", kind: types.TypeKindNamed, elemKind: types.TypeKindSignature},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field := findField(t, complexType, tt.name)

			assert.Equal(t, tt.kind, field.Type.Kind)
			if tt.arrayLen != 0 {
				assert.Equal(t, tt.arrayLen, field.Type.Len)
			}
			if tt.chanDir != 0 {
				assert.Equal(t, tt.chanDir, field.Type.ChanDir)
			}
			if tt.elemKind != "" {
				require.NotNil(t, field.Type.Elem)
				assert.Equal(t, tt.elemKind, field.Type.Elem.Kind)
			}
			if tt.keyKind != "" {
				require.NotNil(t, field.Type.Key)
				assert.Equal(t, tt.keyKind, field.Type.Key.Kind)
			}
			if tt.valueKind != "" {
				require.NotNil(t, field.Type.Value)
				assert.Equal(t, tt.valueKind, field.Type.Value.Kind)
			}
		})
	}
}

func TestLoaderLoad_MarksMorphGeneratedFunctions(t *testing.T) {
	pkg := loadAlphaPackage(t)

	generated := findFunction(t, pkg, "GeneratedMapper")
	normal := findFunction(t, pkg, "Exported")

	assert.True(t, generated.IsMorphFile)
	assert.Contains(t, filepath.Base(generated.SourceFile), "generated_morph.go")

	assert.False(t, normal.IsMorphFile)
	assert.Contains(t, filepath.Base(normal.SourceFile), "alpha.go")
}

func loadFixture(t *testing.T, patterns ...string) *types.Loader {
	t.Helper()

	loader := types.NewLoader("testdata")
	require.NoError(t, loader.Load(context.Background(), patterns...))

	return loader
}

func loadAlphaPackage(t *testing.T) types.Package {
	t.Helper()

	return findPackage(t, loadFixture(t, "./alpha").Packages(), "alpha")
}

func findPackage(t *testing.T, pkgs []types.Package, name string) types.Package {
	t.Helper()

	for _, pkg := range pkgs {
		if pkg.Name == name {
			return pkg
		}
	}

	require.Failf(t, "package not found", "package %q not found in %v", name, packageNames(pkgs))
	return types.Package{}
}

func findConstant(t *testing.T, pkg types.Package, name string) types.ConstantDecl {
	t.Helper()

	for _, constant := range pkg.Constants {
		if constant.Name == name {
			return constant
		}
	}

	require.Failf(t, "constant not found", "constant %q not found", name)
	return types.ConstantDecl{}
}

func findFunction(t *testing.T, pkg types.Package, name string) types.FunctionDecl {
	t.Helper()

	for _, fn := range pkg.Functions {
		if fn.Name == name {
			return fn
		}
	}

	require.Failf(t, "function not found", "function %q not found", name)
	return types.FunctionDecl{}
}

func findType(t *testing.T, pkg types.Package, name string) types.TypeDecl {
	t.Helper()

	for _, typ := range pkg.Types {
		if typ.Name == name {
			return typ
		}
	}

	require.Failf(t, "type not found", "type %q not found", name)
	return types.TypeDecl{}
}

func findField(t *testing.T, typ types.TypeDecl, name string) types.Field {
	t.Helper()

	for _, field := range typ.Fields {
		if field.Name == name {
			return field
		}
	}

	require.Failf(t, "field not found", "field %q not found", name)
	return types.Field{}
}

func packageNames(pkgs []types.Package) []string {
	names := make([]string, len(pkgs))
	for i, pkg := range pkgs {
		names[i] = pkg.Name
	}
	slices.Sort(names)
	return names
}

func parameterKinds(params []types.Parameter) []types.TypeKind {
	kinds := make([]types.TypeKind, len(params))
	for i, param := range params {
		kinds[i] = param.Type.Kind
	}
	return kinds
}
