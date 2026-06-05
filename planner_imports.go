package morph

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
)

func (p *attemptPlanner) prepareImportGraph(outputGroups map[plan.OutputLocation]plan.OutputGroup) error {
	graph := p.baseImportGraph()

	for _, outputGroup := range sortedOutputGroups(outputGroups) {
		for _, root := range outputGroup.Roots {
			// A generated mapper in the output package must refer to both mapped packages. Seeding
			// each root signature edge up front prevents obviously cyclic root plans; the final plan
			// is validated again once nested helpers, callables, and generated mapper calls are known.
			if err := graph.AddGeneratedImport(outputGroup.Location.ImportPath, root.Source.ImportPath); err != nil {
				return fmt.Errorf("output %q: %w", outputGroup.Location.ImportPath, err)
			}
			if err := graph.AddGeneratedImport(outputGroup.Location.ImportPath, root.Target.ImportPath); err != nil {
				return fmt.Errorf("output %q: %w", outputGroup.Location.ImportPath, err)
			}
		}
	}

	p.importGraph = graph
	return nil
}

func (p *attemptPlanner) baseImportGraph() importGraph {
	graph := make(importGraph)
	if p.loader == nil {
		return graph
	}
	for _, pkg := range p.loader.Packages() {
		for _, importPath := range pkg.Imports {
			graph.AddEdge(pkg.ImportPath, importPath)
		}
	}
	return graph
}

type importRequirement struct {
	From   string
	To     string
	Path   string
	Reason string
}

func (r importRequirement) IsNoop() bool {
	return r.From == "" || r.To == "" || r.From == r.To
}

func (r importRequirement) Key() string {
	return strings.Join([]string{r.From, r.To, r.Path, r.Reason}, "\x00")
}

type importRequirementSite struct {
	Requirement importRequirement
	Type        *plan.Type
	Value       *plan.Value
}

// importGraph is an adjacency list of direct package imports:
//
//	importer package -> imported package -> present
//
// Existing imports are loaded from Go packages. During shallow planning, Morph seeds imports for
// root mapper signatures; deeper planning records additional generated imports as selected
// callables, generated mapper calls, and nested helpers become known.
type importGraph map[string]map[string]struct{}

// AddGeneratedImport records an import that Morph-generated code would add. It rejects the edge if
// the imported package already reaches the importer, because adding importer -> imported would then
// complete a cycle.
func (g importGraph) AddGeneratedImport(from, to string) error {
	if p := g.path(to, from); len(p) > 0 {
		return fmt.Errorf(
			"generated import %q -> %q would create an import cycle; existing path: %s",
			from,
			to,
			strings.Join(p, " -> "),
		)
	}
	g.AddEdge(from, to)
	return nil
}

// CanAddEdge reports whether a direct import from -> to can be added without creating a cycle.
func (g importGraph) CanAddEdge(from, to string) bool {
	return len(g.path(to, from)) == 0
}

func (g importGraph) AddEdge(from, to string) {
	if from == "" || to == "" || from == to {
		return
	}
	if _, ok := g[from]; !ok {
		g[from] = make(map[string]struct{})
	}
	g[from][to] = struct{}{}
}

// path returns one import path from `from` to `to` by following existing direct imports. An empty
// slice means no path exists. Self-imports and empty package paths are ignored because Go does not
// emit imports for references within the generated file's own package.
func (g importGraph) path(from, to string) []string {
	if from == "" || to == "" {
		return nil
	}
	if from == to {
		return nil
	}

	seen := make(map[string]struct{})
	var walk func(string) []string
	walk = func(current string) []string {
		if current == to {
			return []string{current}
		}
		if _, ok := seen[current]; ok {
			return nil
		}
		seen[current] = struct{}{}

		for next := range g[current] {
			if p := walk(next); len(p) > 0 {
				return append([]string{current}, p...)
			}
		}
		return nil
	}

	return walk(from)
}

func (g importGraph) Clone() importGraph {
	if g == nil {
		return nil
	}

	out := make(importGraph, len(g))
	for from, imports := range g {
		out[from] = maps.Clone(imports)
	}
	return out
}

func (g importGraph) CanAddImportRequirement(req importRequirement) bool {
	if req.IsNoop() {
		return true
	}
	return g.CanAddEdge(req.From, req.To)
}

func (g importGraph) AddImportRequirement(req importRequirement) error {
	if req.IsNoop() {
		return nil
	}
	return g.AddGeneratedImport(req.From, req.To)
}

func (p *attemptPlanner) canUseGeneratedImport(req importRequirement) bool {
	if req.IsNoop() || p.importGraph == nil {
		return true
	}
	return p.importGraph.CanAddImportRequirement(req)
}

func (p *attemptPlanner) recordGeneratedImport(req importRequirement) []plan.Diagnostic {
	if req.IsNoop() || p.importGraph == nil {
		return nil
	}
	if err := p.importGraph.AddImportRequirement(req); err != nil {
		return []plan.Diagnostic{importRequirementDiagnostic(req, err)}
	}
	return nil
}

func (p *attemptPlanner) importRequirementDiagnostic(req importRequirement) (plan.Diagnostic, bool) {
	if p.canUseGeneratedImport(req) {
		return plan.Diagnostic{}, false
	}

	graph := p.importGraph.Clone()
	err := graph.AddImportRequirement(req)
	if err == nil {
		err = fmt.Errorf("generated import %q -> %q would create an import cycle", req.From, req.To)
	}
	return importRequirementDiagnostic(req, err), true
}

func (p *attemptPlanner) canUseCallable(callable plan.CallableRef) bool {
	req, ok := p.callableImportRequirement("", callable)
	return !ok || p.canUseGeneratedImport(req)
}

func (p *attemptPlanner) callableImportDiagnostic(
	path string,
	callable plan.CallableRef,
) (plan.Diagnostic, bool) {
	req, ok := p.callableImportRequirement(path, callable)
	if !ok {
		return plan.Diagnostic{}, false
	}
	return p.importRequirementDiagnostic(req)
}

func (p *attemptPlanner) recordCallableImport(
	path string,
	callable plan.CallableRef,
) []plan.Diagnostic {
	req, ok := p.callableImportRequirement(path, callable)
	if !ok {
		return nil
	}
	return p.recordGeneratedImport(req)
}

func (p *attemptPlanner) callableImportRequirement(
	path string,
	callable plan.CallableRef,
) (importRequirement, bool) {
	if callable.Kind != plan.CallableKindFunction || p.currentOutputLocation == nil {
		return importRequirement{}, false
	}

	return importRequirement{
		From:   p.currentOutputLocation.ImportPath,
		To:     callable.Package.ImportPath,
		Path:   path,
		Reason: fmt.Sprintf("function callable %q", callable.Name),
	}, true
}

func (p *attemptPlanner) generatedMapperCallRequirement(
	path string,
	mapper *plan.Type,
) (importRequirement, bool) {
	if mapper == nil || p.currentOutputLocation == nil {
		return importRequirement{}, false
	}

	return importRequirement{
		From:   p.currentOutputLocation.ImportPath,
		To:     mapper.Location.ImportPath,
		Path:   path,
		Reason: fmt.Sprintf("generated mapper %q", mapper.FunctionName),
	}, true
}

func (p *attemptPlanner) recordGeneratedMapperCallImport(
	path string,
	mapper *plan.Type,
) []plan.Diagnostic {
	req, ok := p.generatedMapperCallRequirement(path, mapper)
	if !ok {
		return nil
	}
	return p.recordGeneratedImport(req)
}

func (p *attemptPlanner) recordMapperSignatureImports(
	from string,
	reasonPrefix string,
	typ *plan.Type,
) []plan.Diagnostic {
	if typ == nil {
		return nil
	}

	path := plan.TypesPath(typ.SourceType, typ.TargetType)

	sites := typeImportRequirementSites(
		from,
		path,
		reasonPrefix+" source signature",
		typ.SourceType,
		typ,
		nil,
	)

	sites = append(sites, typeImportRequirementSites(
		from,
		path,
		reasonPrefix+" target signature",
		typ.TargetType,
		typ,
		nil,
	)...)

	sites = dedupeImportRequirementSites(sites)

	var diagnostics []plan.Diagnostic
	for _, site := range sites {
		diagnostics = appendDiagnostic(diagnostics, p.recordGeneratedImport(site.Requirement)...)
	}

	return diagnostics
}

func (p *attemptPlanner) validateImportRequirements(sites []importRequirementSite) {
	if len(sites) == 0 {
		return
	}

	graph := p.importGraph.Clone()
	if graph == nil {
		graph = p.baseImportGraph()
	}

	for _, site := range sites {
		if err := graph.AddImportRequirement(site.Requirement); err != nil {
			diagnostic := importRequirementDiagnostic(site.Requirement, err)
			if site.Value != nil {
				site.Value.Diagnostics = appendDiagnostic(site.Value.Diagnostics, diagnostic)
			}
			if site.Type != nil {
				site.Type.Diagnostics = appendDiagnostic(site.Type.Diagnostics, diagnostic)
			}
			p.diagnostics = appendDiagnostic(p.diagnostics, diagnostic)
		}
	}
}

func typeSignatureImportSites(ctx walkContext) []importRequirementSite {
	if ctx.Type == nil {
		return nil
	}

	sites := typeImportRequirementSites(
		ctx.Location.ImportPath,
		ctx.Path,
		fmt.Sprintf("mapper %q source signature", ctx.Type.FunctionName),
		ctx.Type.SourceType,
		ctx.Type,
		nil,
	)

	sites = append(sites, typeImportRequirementSites(
		ctx.Location.ImportPath,
		ctx.Path,
		fmt.Sprintf("mapper %q target signature", ctx.Type.FunctionName),
		ctx.Type.TargetType,
		ctx.Type,
		nil,
	)...)

	return sites
}

func valueImportSites(ctx walkContext) []importRequirementSite {
	value := ctx.Value
	if value == nil {
		return nil
	}

	var sites []importRequirementSite

	switch value.Operation {
	case plan.OperationFunction:
		if value.Callable != nil {
			sites = append(sites, importRequirementSite{
				Requirement: importRequirement{
					From:   ctx.Location.ImportPath,
					To:     value.Callable.Package.ImportPath,
					Path:   ctx.Path,
					Reason: fmt.Sprintf("function callable %q", value.Callable.Name),
				},
				Type:  ctx.Owner,
				Value: value,
			})
		}
	case plan.OperationStruct, plan.OperationEnum:
		if value.Plan != nil {
			sites = append(sites, importRequirementSite{
				Requirement: importRequirement{
					From:   ctx.Location.ImportPath,
					To:     value.Plan.Location.ImportPath,
					Path:   ctx.Path,
					Reason: fmt.Sprintf("generated mapper %q", value.Plan.FunctionName),
				},
				Type:  ctx.Owner,
				Value: value,
			})
		}
	case plan.OperationConvert:
		sites = append(sites, typeImportRequirementSites(
			ctx.Location.ImportPath,
			ctx.Path,
			"conversion target type",
			value.Target,
			ctx.Owner,
			value,
		)...)
	case plan.OperationSlice, plan.OperationArray, plan.OperationMap:
		sites = append(sites, typeImportRequirementSites(
			ctx.Location.ImportPath,
			ctx.Path,
			"container target type",
			value.Target,
			ctx.Owner,
			value,
		)...)
	}

	for i := range value.CallableArgs {
		arg := &value.CallableArgs[i]
		argPath := callableArgPath(ctx.Path, i)
		sites = append(sites, typeImportRequirementSites(
			ctx.Location.ImportPath,
			argPath,
			fmt.Sprintf("callable argument %d source type", i+1),
			arg.Mapping.Source,
			ctx.Owner,
			&arg.Mapping,
		)...)
		sites = append(sites, typeImportRequirementSites(
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

func typeImportRequirementSites(
	from string,
	path string,
	reason string,
	typ types.Type,
	owner *plan.Type,
	value *plan.Value,
) []importRequirementSite {
	var sites []importRequirementSite
	collectTypeImportRequirementSites(from, path, reason, typ, owner, value, &sites)
	return sites
}

func collectTypeImportRequirementSites(
	from string,
	path string,
	reason string,
	typ types.Type,
	owner *plan.Type,
	value *plan.Value,
	refs *[]importRequirementSite,
) {
	typ = types.UnwrapAlias(typ)

	switch typ.Kind {
	case types.TypeKindNamed, types.TypeKindAlias:
		if typ.Package.ImportPath != "" {
			*refs = append(*refs, importRequirementSite{
				Requirement: importRequirement{
					From:   from,
					To:     typ.Package.ImportPath,
					Path:   path,
					Reason: reason,
				},
				Type:  owner,
				Value: value,
			})
		}
		for _, arg := range typ.TypeArgs {
			collectTypeImportRequirementSites(from, path, reason, arg, owner, value, refs)
		}
	case types.TypeKindPointer, types.TypeKindSlice, types.TypeKindArray, types.TypeKindChan:
		if typ.Elem != nil {
			collectTypeImportRequirementSites(from, path, reason, *typ.Elem, owner, value, refs)
		}
	case types.TypeKindMap:
		if typ.Key != nil {
			collectTypeImportRequirementSites(from, path, reason, *typ.Key, owner, value, refs)
		}
		if typ.Value != nil {
			collectTypeImportRequirementSites(from, path, reason, *typ.Value, owner, value, refs)
		}
	case types.TypeKindSignature:
		for _, param := range typ.Params {
			collectTypeImportRequirementSites(from, path, reason, param.Type, owner, value, refs)
		}
		for _, result := range typ.Results {
			collectTypeImportRequirementSites(from, path, reason, result.Type, owner, value, refs)
		}
	case types.TypeKindStruct:
		for _, field := range typ.Fields {
			collectTypeImportRequirementSites(from, path, reason, field.Type, owner, value, refs)
		}
	case types.TypeKindInterface:
		for _, method := range typ.Methods {
			if method.Receiver != nil {
				collectTypeImportRequirementSites(from, path, reason, method.Receiver.Type, owner, value, refs)
			}
			for _, param := range method.Params {
				collectTypeImportRequirementSites(from, path, reason, param.Type, owner, value, refs)
			}
			for _, result := range method.Results {
				collectTypeImportRequirementSites(from, path, reason, result.Type, owner, value, refs)
			}
		}
	}
}

func dedupeImportRequirementSites(
	refs []importRequirementSite,
) []importRequirementSite {
	slices.SortFunc(refs, func(a, b importRequirementSite) int {
		return cmp.Or(
			strings.Compare(a.Requirement.From, b.Requirement.From),
			strings.Compare(a.Requirement.To, b.Requirement.To),
			strings.Compare(a.Requirement.Path, b.Requirement.Path),
			strings.Compare(a.Requirement.Reason, b.Requirement.Reason),
		)
	})

	out := refs[:0]
	var previous string
	for i, ref := range refs {
		if ref.Requirement.IsNoop() {
			continue
		}
		key := ref.Requirement.Key()
		if i > 0 && key == previous {
			continue
		}
		out = append(out, ref)
		previous = key
	}
	return out
}

func importRequirementDiagnostic(req importRequirement, err error) plan.Diagnostic {
	message := fmt.Sprintf(
		"%s requires generated package %q to import %q, but %v",
		req.Reason,
		req.From,
		req.To,
		err,
	)
	return plan.Diagnostic{
		Level:   plan.DiagnosticLevelFatal,
		Path:    req.Path,
		Message: message,
	}
}
