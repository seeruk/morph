package morph

import (
	"cmp"
	"fmt"
	"go/ast"
	"slices"
	"strings"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
)

type visibilityRequirementSite struct {
	From       string
	Path       string
	Reason     string
	Identifier string
	Package    types.PackageRef
	Type       *plan.Type
	Value      *plan.Value
}

func typeSignatureVisibilitySites(ctx walkContext) []visibilityRequirementSite {
	if ctx.Type == nil {
		return nil
	}

	sites := typeVisibilityRequirementSites(
		ctx.Location.ImportPath,
		ctx.Path,
		fmt.Sprintf("mapper %q source signature", ctx.Type.FunctionName),
		ctx.Type.SourceType,
		ctx.Type,
		nil,
	)

	sites = append(sites, typeVisibilityRequirementSites(
		ctx.Location.ImportPath,
		ctx.Path,
		fmt.Sprintf("mapper %q target signature", ctx.Type.FunctionName),
		ctx.Type.TargetType,
		ctx.Type,
		nil,
	)...)

	return sites
}

func valueVisibilitySites(ctx walkContext) []visibilityRequirementSite {
	value := ctx.Value
	if value == nil {
		return nil
	}

	var sites []visibilityRequirementSite

	if len(value.SourceAdaptations) > 0 || len(value.TargetAdaptations) > 0 {
		sites = append(sites, typeVisibilityRequirementSites(
			ctx.Location.ImportPath,
			ctx.Path,
			"adapted target type",
			value.Target,
			ctx.Owner,
			value,
		)...)
	}

	switch value.Operation {
	case plan.OperationConvert:
		sites = append(sites, typeVisibilityRequirementSites(
			ctx.Location.ImportPath,
			ctx.Path,
			"conversion target type",
			value.Target,
			ctx.Owner,
			value,
		)...)
	case plan.OperationSlice, plan.OperationArray, plan.OperationMap:
		sites = append(sites, typeVisibilityRequirementSites(
			ctx.Location.ImportPath,
			ctx.Path,
			"container target type",
			value.Target,
			ctx.Owner,
			value,
		)...)
	}

	for i := range value.CallableMapperArgs {
		arg := &value.CallableMapperArgs[i]
		argPath := callableMapperArgPath(ctx.Path, i)
		sites = append(sites, typeVisibilityRequirementSites(
			ctx.Location.ImportPath,
			argPath,
			fmt.Sprintf("callable argument %d source type", i+1),
			arg.Mapping.Source,
			ctx.Owner,
			&arg.Mapping,
		)...)
		sites = append(sites, typeVisibilityRequirementSites(
			ctx.Location.ImportPath,
			argPath,
			fmt.Sprintf("callable argument %d target type", i+1),
			arg.Mapping.Target,
			ctx.Owner,
			&arg.Mapping,
		)...)
	}

	return sites
}

func typeVisibilityRequirementSites(
	from string,
	path string,
	reason string,
	typ types.Type,
	owner *plan.Type,
	value *plan.Value,
) []visibilityRequirementSite {
	var sites []visibilityRequirementSite
	collectTypeVisibilityRequirementSites(from, path, reason, typ, owner, value, &sites)
	return sites
}

func collectTypeVisibilityRequirementSites(
	from string,
	path string,
	reason string,
	typ types.Type,
	owner *plan.Type,
	value *plan.Value,
	refs *[]visibilityRequirementSite,
) {
	typ = types.UnwrapAlias(typ)

	switch typ.Kind {
	case types.TypeKindNamed, types.TypeKindAlias:
		if !isIdentifierAccessibleFrom(typ.Package, typ.Name, from) {
			*refs = append(*refs, visibilityRequirementSite{
				From:       from,
				Path:       path,
				Reason:     reason,
				Identifier: typ.Name,
				Package:    typ.Package,
				Type:       owner,
				Value:      value,
			})
		}
		for _, arg := range typ.TypeArgs {
			collectTypeVisibilityRequirementSites(from, path, reason, arg, owner, value, refs)
		}
	case types.TypeKindPointer, types.TypeKindSlice, types.TypeKindArray, types.TypeKindChan:
		if typ.Elem != nil {
			collectTypeVisibilityRequirementSites(from, path, reason, *typ.Elem, owner, value, refs)
		}
	case types.TypeKindMap:
		if typ.Key != nil {
			collectTypeVisibilityRequirementSites(from, path, reason, *typ.Key, owner, value, refs)
		}
		if typ.Value != nil {
			collectTypeVisibilityRequirementSites(from, path, reason, *typ.Value, owner, value, refs)
		}
	case types.TypeKindSignature:
		for _, param := range typ.Params {
			collectTypeVisibilityRequirementSites(from, path, reason, param.Type, owner, value, refs)
		}
		for _, result := range typ.Results {
			collectTypeVisibilityRequirementSites(from, path, reason, result.Type, owner, value, refs)
		}
	case types.TypeKindStruct:
		for _, field := range typ.Fields {
			collectTypeVisibilityRequirementSites(from, path, reason, field.Type, owner, value, refs)
		}
	case types.TypeKindInterface:
		for _, method := range typ.Methods {
			if method.Receiver != nil {
				collectTypeVisibilityRequirementSites(from, path, reason, method.Receiver.Type, owner, value, refs)
			}
			for _, param := range method.Params {
				collectTypeVisibilityRequirementSites(from, path, reason, param.Type, owner, value, refs)
			}
			for _, result := range method.Results {
				collectTypeVisibilityRequirementSites(from, path, reason, result.Type, owner, value, refs)
			}
		}
	}
}

func dedupeVisibilityRequirementSites(
	refs []visibilityRequirementSite,
) []visibilityRequirementSite {
	slices.SortFunc(refs, func(a, b visibilityRequirementSite) int {
		return cmp.Or(
			strings.Compare(a.From, b.From),
			strings.Compare(a.Package.ImportPath, b.Package.ImportPath),
			strings.Compare(a.Identifier, b.Identifier),
			strings.Compare(a.Path, b.Path),
			strings.Compare(a.Reason, b.Reason),
		)
	})

	out := refs[:0]
	var previous string
	for i, ref := range refs {
		key := visibilityRequirementKey(ref)
		if i > 0 && key == previous {
			continue
		}
		out = append(out, ref)
		previous = key
	}
	return out
}

func visibilityRequirementKey(ref visibilityRequirementSite) string {
	return strings.Join([]string{
		ref.From,
		ref.Package.ImportPath,
		ref.Identifier,
		ref.Path,
		ref.Reason,
	}, "\x00")
}

func (p *attemptPlanner) validateVisibilityRequirements(sites []visibilityRequirementSite) {
	for _, site := range sites {
		diagnostic := visibilityRequirementDiagnostic(site)
		if site.Value != nil {
			site.Value.Diagnostics = appendDiagnostic(site.Value.Diagnostics, diagnostic)
		}
		if site.Type != nil {
			site.Type.Diagnostics = appendDiagnostic(site.Type.Diagnostics, diagnostic)
		}
		p.diagnostics = appendDiagnostic(p.diagnostics, diagnostic)
	}
}

func visibilityRequirementDiagnostic(site visibilityRequirementSite) plan.Diagnostic {
	return plan.Diagnostic{
		Level: plan.DiagnosticLevelFatal,
		Path:  site.Path,
		Message: fmt.Sprintf(
			"%s references unexported type %q from package %q, but generated package %q cannot name it; unexported types can only be named from their declaring package",
			site.Reason,
			site.Identifier,
			site.Package.ImportPath,
			site.From,
		),
	}
}

func isIdentifierAccessibleFrom(pkg types.PackageRef, name string, from string) bool {
	return pkg.ImportPath == "" || ast.IsExported(name) || pkg.ImportPath == from
}

func isFieldAccessibleFrom(typeDecl types.TypeDecl, field types.Field, from string) bool {
	return field.IsExported || typeDecl.Package.ImportPath == from
}
