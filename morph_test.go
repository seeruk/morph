package morph

import "github.com/seeruk/morph/types"

func basicTestType(name string) types.Type {
	return types.Type{
		Kind:   types.TypeKindBasic,
		Name:   name,
		String: name,
	}
}

func namedTestType(importPath, name string, args ...types.Type) types.Type {
	return types.Type{
		Kind:     types.TypeKindNamed,
		Name:     name,
		Package:  types.PackageRef{ImportPath: importPath},
		TypeArgs: args,
	}
}

func pointerTestType(typ types.Type) types.Type {
	return types.Type{
		Kind: types.TypeKindPointer,
		Elem: &typ,
	}
}

func typeParamTestType(name string) types.Type {
	return types.Type{
		Kind: types.TypeKindTypeParam,
		Name: name,
	}
}

func signatureTestType(params []types.Type, results ...types.Type) types.Type {
	out := types.Type{
		Kind: types.TypeKindSignature,
	}
	for _, param := range params {
		out.Params = append(out.Params, types.Parameter{Type: param})
	}
	for _, result := range results {
		out.Results = append(out.Results, types.Parameter{Type: result})
	}
	return out
}

func errorTestType() types.Type {
	return types.Type{
		Kind: types.TypeKindNamed,
		Name: "error",
	}
}

func testFunctionDecl(name string, param, result types.Type, extraResults ...types.Type) types.FunctionDecl {
	results := []types.Parameter{{Type: result}}
	for _, extra := range extraResults {
		results = append(results, types.Parameter{Type: extra})
	}
	return types.FunctionDecl{
		Package:    types.PackageRef{Name: "mapping", ImportPath: "module.test/mapping"},
		Name:       name,
		IsExported: true,
		Params:     []types.Parameter{{Type: param}},
		Results:    results,
	}
}

func testMethodDecl(name string, receiver, result types.Type, extraResults ...types.Type) types.Method {
	results := []types.Parameter{{Type: result}}
	for _, extra := range extraResults {
		results = append(results, types.Parameter{Type: extra})
	}
	return types.Method{
		Name:       name,
		IsExported: true,
		Receiver:   &types.Parameter{Type: receiver},
		Results:    results,
	}
}
