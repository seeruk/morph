package morph

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
)

type Planner struct {
	// spec is the spec that this planner is planning for
	spec Spec
	// workingDir is the working directory of this Planner
	workingDir string
	// ident is a value used to identify this run, it should be stable (e.g. it could be the name of
	// the config file used for this run). It doesn't need to be hyper-specific, as it's used to
	// build a hash along with the workspace.
	ident string
	// runHash is a stable run hash for this module / location / spec origin.
	runHash string

	// loader is the initialized type loader for this planner
	loader *types.Loader
	// conversions contains explicitly permitted directional named type conversions.
	conversions map[spec.Conversion]struct{}
	// workspace contains the module and filesystem environment Morph is planning within
	workspace *Workspace
}

// NewPlanner returns a new Planner, set to plan the given Spec.
func NewPlanner(specification Spec, workingDir, ident string) *Planner {
	return &Planner{
		spec:       specification,
		workingDir: workingDir,
		ident:      ident,

		conversions: conversionSet(specification.Conversions),
	}
}

// Plan attempts to produce a Plan for the Spec assigned to this Planner.
func (p *Planner) Plan() (Plan, error) {
	var out Plan
	if err := p.prepare(); err != nil {
		return out, fmt.Errorf("failed to prepare planner: %w", err)
	}

	hash, err := stableRunHash(p.workspace, p.ident)
	if err != nil {
		return out, fmt.Errorf("failed to generate run hash: %w", err)
	}

	p.runHash = hash

	bans := make(map[callableBan]struct{})

	for {
		planner := p.newAttempt(bans)

		attempt, retryBan, err := planner.plan()
		if err != nil {
			return attempt, err
		}

		// If no retry ban is returned, the plan is valid and we're done!
		if retryBan == nil {
			return attempt, nil
		}

		// We've hit a ban we can't avoid, return the attempt, and it should contain a fatal
		// diagnostic which should halt execution.
		if _, exists := bans[*retryBan]; exists {
			return attempt, nil
		}

		// If we've not seen the ban before, we keep track of it and go again...
		bans[*retryBan] = struct{}{}
	}
}

func (p *Planner) newAttempt(bans map[callableBan]struct{}) *attemptPlanner {
	return newAttemptPlanner(
		p.spec,
		p.runHash,
		p.loader,
		p.workspace,
		p.conversions,
		bans,
	)
}

type attemptPlanner struct {
	spec        Spec
	runHash     string
	loader      *types.Loader
	workspace   *Workspace
	conversions map[spec.Conversion]struct{}

	// registry is the functionRegistry used by this planner to find callables that could be used
	// to map between types
	registry *functionRegistry
	// callables contains explicitly referenced functions and methods available to scoped callable
	// selection, keyed by the user-provided callable reference.
	callables map[spec.CallableRef]registeredCallable
	// importGraph contains existing imports plus shallow-planned generated imports.
	importGraph importGraph
	// rootVariantsByTypePair contains requested root mappings grouped by source/target pair.
	// Multiple variants can exist when the same type pair is emitted in different output packages
	// or with different function names.
	rootVariantsByTypePair map[string][]*rootVariant // plan.TypePairKey -> variants
	// rootVariantsByCallable reserves root function names within their output package.
	// Nested generated mappers are not tracked here; stale generated nested functions are excluded
	// from discovery by plannedOutputFiles instead.
	rootVariantsByCallable map[spec.CallableRef]*rootVariant
	// callableBans contains callable selections that were invalidated by final errorability
	// checks and should be skipped by later planning attempts.
	callableBans map[callableBan]struct{}
	// mappings contains all requested root and nested type mappings currently known to the planner.
	mappings map[string]*plan.Type // output-scoped root key or plan.TypeMapperKey -> *plan.Type
	// plannedOutputFiles is a map of the logical paths of all output files Morph is planning to
	// generate. This is useful for discovering functions in files we're about to generate.
	plannedOutputFiles map[string]struct{} // clean logical path -> present
	// planningMappings contains mappings currently being planned. This prevents recursive nested
	// struct mappings from repeatedly attempting to plan themselves.
	planningMappings map[string]struct{} // planner mapping key -> present
	// shallowMappings contains mappings currently shallow planned.
	// As shallow plans are made, they'll be added to this map.
	// As these mappings are fully planned, they will be removed from this map.
	shallowMappings map[string]struct{} // planner mapping key -> present
	// currentOutputLocation is temporary planning context used for output-aware root candidate
	// ranking while an output group is being deeply planned.
	currentOutputLocation *plan.OutputLocation

	diagnostics []plan.Diagnostic
}

func newAttemptPlanner(
	specification Spec,
	runHash string,
	loader *types.Loader,
	workspace *Workspace,
	conversions map[spec.Conversion]struct{},
	bans map[callableBan]struct{},
) *attemptPlanner {
	return &attemptPlanner{
		spec:        specification,
		runHash:     runHash,
		loader:      loader,
		workspace:   workspace,
		conversions: conversions,

		rootVariantsByTypePair: make(map[string][]*rootVariant),
		rootVariantsByCallable: make(map[spec.CallableRef]*rootVariant),
		mappings:               make(map[string]*plan.Type),
		plannedOutputFiles:     make(map[string]struct{}),
		planningMappings:       make(map[string]struct{}),
		shallowMappings:        make(map[string]struct{}),
		callableBans:           maps.Clone(bans),
		callables:              make(map[spec.CallableRef]registeredCallable),
	}
}

func (p *attemptPlanner) plan() (Plan, *callableBan, error) {
	var out Plan

	// Next we'll do a shallow pass over the spec to determine all the mapping functions we're going
	// to generate. This allows us to avoid auto-discovering functions we're about to generate when
	// we're planning value mapping, and also allows Morph to detect some other potential issues
	// earlier, for example, cyclic dependency issues.
	outputGroups, err := p.shallowPlan()
	if err != nil {
		return out, nil, fmt.Errorf("failed shallow planning pass: %w", err)
	}

	if err := p.prepareImportGraph(outputGroups); err != nil {
		return out, nil, fmt.Errorf("failed import cycle validation: %w", err)
	}

	out.OutputGroups = sortedOutputGroups(outputGroups)

	// Now the shallow plan is complete; we can safely set up discovery, knowing we're not going to
	// allow discovery to pick up on functions we're about to generate.
	if err := p.registerDiscovery(); err != nil {
		return out, nil, fmt.Errorf("failed to register discovery: %w", err)
	}

	// Now we're ready to plan value mappings. This is a deeper pass over the spec, actually based
	// on the shallow plan we've just done, as that only contains explicitly requested mappings.
	for _, og := range out.OutputGroups {
		p.planOutputGroup(og)
	}

	p.finalizePlanErrability(out.OutputGroups)

	retryBan := p.validateCallableErrability(out.OutputGroups)

	out.Diagnostics = p.diagnostics

	return out, retryBan, nil
}

// prepare sets up this planner instance, initializing the type loader, setting up the workspace,
// preparing the callable registry, and loading in caches of data from the current spec.
func (p *Planner) prepare() error {
	// Bail early if there's nothing to do...
	if len(p.spec.Packages) == 0 {
		return errors.New("no mappings specified; only found empty packages and/or types")
	}

	// Set up the type loader, primed with all the packages specified at any point in the spec
	if err := p.prepareTypeLoader(); err != nil {
		return fmt.Errorf("failed to prepare type loader: %w", err)
	}

	// Figure out what environment we're operating within
	if err := p.prepareWorkspace(); err != nil {
		return fmt.Errorf("failed to prepare workspace: %w", err)
	}

	return nil
}

func (p *Planner) prepareTypeLoader() error {
	loader := types.NewLoader(p.workingDir)

	distinct := map[string]struct{}{}

	add := func(pattern string) {
		// TODO: Do the package names used herein need to be validated more than this?
		if pattern == "" {
			return
		}
		if _, ok := distinct[pattern]; ok {
			return
		}
		distinct[pattern] = struct{}{}
	}

	// Pull the various import paths from the spec, from basically everywhere they can be referenced
	for _, pkg := range p.spec.Packages {
		add(pkg.Source)
		add(pkg.Target)
	}

	for _, pkg := range p.spec.Discovery.Packages {
		add(pkg)
	}

	for _, callable := range explicitCallableRefs(p.spec) {
		add(callable.ImportPath)
	}

	// TODO: Context?
	if err := loader.Load(context.Background(), slices.Collect(maps.Keys(distinct))...); err != nil {
		return fmt.Errorf("preparing primary type loader: loading types: %w", err)
	}

	p.loader = loader
	return nil
}

func explicitCallableRefs(specification Spec) []spec.CallableRef {
	var out []spec.CallableRef
	seen := make(map[spec.CallableRef]struct{})

	add := func(ref spec.CallableRef) {
		if ref.ImportPath == "" && ref.TypeName == "" && ref.Name == "" {
			return
		}
		if _, ok := seen[ref]; ok {
			return
		}
		seen[ref] = struct{}{}
		out = append(out, ref)
	}

	for _, pkg := range specification.Packages {
		for _, typ := range pkg.Types {
			for _, tier := range typ.Callables {
				for _, ref := range tier.Callables {
					add(ref)
				}
			}
			for _, field := range typ.Struct.Fields {
				if field.Callable != nil {
					add(*field.Callable)
				}
			}
		}
	}

	return out
}

func (p *Planner) prepareWorkspace() error {
	workDir, err := filepath.Abs(p.workingDir)
	if err != nil {
		return fmt.Errorf("failed to determine working directory: %w", err)
	}

	for _, pkg := range p.loader.Packages() {
		if pkg.Module == nil || !pkg.Module.Main {
			continue
		}

		p.workspace = &Workspace{
			WorkingDir: workDir,
			ModuleDir:  pkg.Module.Dir,
			ModulePath: pkg.Module.Path,
		}
		return nil
	}

	return fmt.Errorf("failed to determine working directory: couldn't find main module")
}

func (p *attemptPlanner) prepareImportGraph(outputGroups map[plan.OutputLocation]plan.OutputGroup) error {
	graph := p.baseImportGraph()

	for _, outputGroup := range sortedOutputGroups(outputGroups) {
		for _, root := range outputGroup.Roots {
			// A generated mapper in the output package must refer to both mapped packages. Adding
			// each generated edge up front prevents Morph from producing a plan that Go could not
			// compile due to import cycles.
			if err := graph.addGeneratedImport(outputGroup.Location.ImportPath, root.Source.ImportPath); err != nil {
				return fmt.Errorf("output %q: %w", outputGroup.Location.ImportPath, err)
			}
			if err := graph.addGeneratedImport(outputGroup.Location.ImportPath, root.Target.ImportPath); err != nil {
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
			graph.addEdge(pkg.ImportPath, importPath)
		}
	}
	return graph
}

// importGraph is an adjacency list of direct package imports:
//
//	importer package -> imported package -> present
//
// Existing imports are loaded from Go packages. During shallow planning, Morph also adds the
// imports that generated files would need, from each output package to the source and target
// packages referenced by roots emitted there.
type importGraph map[string]map[string]struct{}

// addGeneratedImport records an import that Morph-generated code would add. It rejects the edge if
// the imported package already reaches the importer, because adding importer -> imported would then
// complete a cycle.
func (g importGraph) addGeneratedImport(from, to string) error {
	if p := g.path(to, from); len(p) > 0 {
		return fmt.Errorf(
			"generated import %q -> %q would create an import cycle; existing path: %s",
			from,
			to,
			strings.Join(p, " -> "),
		)
	}
	g.addEdge(from, to)
	return nil
}

// canAddEdge reports whether a direct import from -> to can be added without creating a cycle.
func (g importGraph) canAddEdge(from, to string) bool {
	return len(g.path(to, from)) == 0
}

func (g importGraph) addEdge(from, to string) {
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

func (p *attemptPlanner) registerDiscovery() error {
	pkgs := p.loader.Packages()

	registry := newFunctionRegistry()
	callables, err := p.registerExplicitCallables(pkgs)
	if err != nil {
		return fmt.Errorf("failed to register explicit callables: %w", err)
	}
	p.callables = callables

	if err := p.registerDiscoveryPackages(registry, pkgs); err != nil {
		return fmt.Errorf("failed to register discovered packages: %w", err)
	}

	p.registry = registry
	return nil
}

type registeredCallable struct {
	Function *types.FunctionDecl
	Method   *types.Method
}

func (p *attemptPlanner) registerExplicitCallables(pkgs map[string]types.Package) (map[spec.CallableRef]registeredCallable, error) {
	out := make(map[spec.CallableRef]registeredCallable)
	for _, ref := range explicitCallableRefs(p.spec) {
		callable, err := registeredCallableFromRef(pkgs, ref)
		if err != nil {
			return nil, err
		}
		out[ref] = callable
	}
	return out, nil
}

func registeredCallableFromRef(pkgs map[string]types.Package, ref spec.CallableRef) (registeredCallable, error) {
	pkg, ok := pkgs[ref.ImportPath]
	if !ok {
		return registeredCallable{}, fmt.Errorf("package not found for callable %q", ref.String())
	}

	if ref.TypeName == "" {
		fn, ok := pkg.Functions[ref.Name]
		if !ok {
			return registeredCallable{}, fmt.Errorf("function not found for callable %q", ref.String())
		}
		if _, ok := plan.CallableRefFromFunctionDecl(fn, plan.CallableSourceUser); !ok {
			return registeredCallable{}, fmt.Errorf("function is not viable for callable %q", ref.String())
		}
		return registeredCallable{Function: &fn}, nil
	}

	decl, ok := pkg.Types[ref.TypeName]
	if !ok {
		return registeredCallable{}, fmt.Errorf("type not found for callable %q", ref.String())
	}
	method, ok := decl.Methods[ref.Name]
	if !ok {
		return registeredCallable{}, fmt.Errorf("method not found for callable %q", ref.String())
	}
	if _, ok := plan.CallableRefFromMethod(method, plan.CallableSourceUser); !ok {
		return registeredCallable{}, fmt.Errorf("method is not viable for callable %q", ref.String())
	}
	return registeredCallable{Method: &method}, nil
}

func (p *attemptPlanner) registerDiscoveryPackages(registry *functionRegistry, pkgs map[string]types.Package) error {
	// NOTE: Functions added to the registry here are based on code that already exists. In other
	// words, functions previously generated by Morph can be added here, i.e. ones that could be
	// removed by this run of Morph. Therefore, when deciding whether to use a discovered function
	// we must check that these functions would not otherwise be generated by this run of Morph.
	// Otherwise, we can pick one of these functions, and then it would no longer exist after Morph
	// has finished running, resulting in invalid generated code.
	//
	// It's also worth noting, many of these discovered functions may never be used!
	for _, importPath := range p.spec.Discovery.Packages {
		pkg, ok := pkgs[importPath]
		if !ok {
			return fmt.Errorf("package not found for discovery %q", importPath)
		}

		for _, fn := range pkg.Functions {
			ref := spec.CallableRefFromFunctionDecl(fn)

			// Skip unexported, explicitly excluded function declarations, and those that are about
			// to be generated as part of this plan.
			// NOTE: Methods aren't discovered here, their exclusion is handled elsewhere.
			isExcluded := slices.Contains(p.spec.Discovery.Exclusions, ref)
			isPending := p.isFunctionPendingGeneration(fn, ref)
			if !fn.IsExported || isExcluded || isPending {
				continue
			}

			registry.Register(fn, plan.CallableSourceDiscovered)
		}
	}

	return nil
}

// shallowPlan does a shallow planning pass over the spec, not attempting to plan any value
// mappings, instead just determining the output groups and the explicitly requested mapping
// functions that need to be generated.
func (p *attemptPlanner) shallowPlan() (map[plan.OutputLocation]plan.OutputGroup, error) {
	outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
	for _, pkgSpec := range p.spec.Packages {
		if err := p.shallowPackagePlan(outputGroups, pkgSpec); err != nil {
			return nil, fmt.Errorf("failed to shallow plan package %q -> %q: %w", pkgSpec.Source, pkgSpec.Target, err)
		}
	}

	return outputGroups, nil
}

// shallowPackagePlan does a shallow planning pass for the given package spec.
func (p *attemptPlanner) shallowPackagePlan(outputGroups map[plan.OutputLocation]plan.OutputGroup, pkgSpec spec.Package) error {
	pkgs := p.loader.Packages()

	outputSourcePkg, ok := pkgs[pkgSpec.Source]
	if !ok {
		return fmt.Errorf("source package not found %q", pkgSpec.Source)
	}

	outputTargetPkg, ok := pkgs[pkgSpec.Target]
	if !ok {
		return fmt.Errorf("target package not found %q", pkgSpec.Target)
	}

	location, err := p.outputLocationForPackages(outputSourcePkg, outputTargetPkg, pkgSpec.Output)
	if err != nil {
		return fmt.Errorf("failed to determine output location for packages %q -> %q: %w", pkgSpec.Source, pkgSpec.Target, err)
	}

	for _, typeSpec := range pkgSpec.Types {
		sourcePackage := cmp.Or(typeSpec.SourcePackage, pkgSpec.Source)
		targetPackage := cmp.Or(typeSpec.TargetPackage, pkgSpec.Target)

		sourcePkg, ok := pkgs[sourcePackage]
		if !ok {
			return fmt.Errorf("source package not found %q", sourcePackage)
		}

		targetPkg, ok := pkgs[targetPackage]
		if !ok {
			return fmt.Errorf("target package not found %q", targetPackage)
		}

		root, err := p.shallowTypePlan(sourcePkg, targetPkg, typeSpec)
		if err != nil {
			return fmt.Errorf("failed to shallow plan type for packages %q -> %q: %w", pkgSpec.Source, pkgSpec.Target, err)
		}

		if err = p.addRoot(outputGroups, location, root); err != nil {
			return fmt.Errorf("failed to add root plan for packages %q -> %q: %w", pkgSpec.Source, pkgSpec.Target, err)
		}
	}

	return nil
}

func (p *attemptPlanner) shallowTypePlan(sourcePkg, targetPkg types.Package, typeSpec spec.Type) (*plan.Type, error) {
	sourceDecl, ok := sourcePkg.Types[typeSpec.Source]
	if !ok {
		return nil, fmt.Errorf("source type not found %q in package %q", typeSpec.Source, sourcePkg.ImportPath)
	}

	targetDecl, ok := targetPkg.Types[typeSpec.Target]
	if !ok {
		return nil, fmt.Errorf("target type not found %q in package %q", typeSpec.Target, targetPkg.ImportPath)
	}

	root, err := p.shallowRootPlan(sourceDecl, targetDecl, typeSpec)
	if err != nil {
		return nil, fmt.Errorf("type %q -> %q: %w", typeSpec.Source, typeSpec.Target, err)
	}

	return root, nil
}

// shallowRootPlan prepares a shallow plan for a particular type mapping.
func (p *attemptPlanner) shallowRootPlan(sourceDecl, targetDecl types.TypeDecl, typeSpec spec.Type) (*plan.Type, error) {
	nameInput := NameInput{
		Source:    sourceDecl.Type,
		Target:    targetDecl.Type,
		Signature: typeSpec.Mapper.Signature,
	}

	functionName, err := MapperName(nameInput, typeSpec.Mapper.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to determine mapper name: %w", err)
	}

	root := &plan.Type{
		Source:       plan.TypeRefFromTypeDecl(sourceDecl),
		Target:       plan.TypeRefFromTypeDecl(targetDecl),
		SourceDecl:   sourceDecl,
		TargetDecl:   targetDecl,
		SourceType:   sourceDecl.Type,
		TargetType:   targetDecl.Type,
		FunctionName: functionName,
		Signature:    typeSpec.Mapper.Signature,
		EnumSpec:     typeSpec.Enum,
		Callables:    typeSpec.Callables,
		StructSpec:   typeSpec.Struct,
		Optionality:  typeSpec.Optionality,
		Conversions:  typeSpec.Conversions,
	}

	root.Diagnostics = appendDiagnostic(root.Diagnostics, validateStructFieldMappings(root)...)
	root.Diagnostics = appendDiagnostic(root.Diagnostics, validateGenericRoot(root)...)

	return root, nil
}

func validateGenericRoot(typ *plan.Type) []plan.Diagnostic {
	if len(typ.SourceDecl.Type.TypeParams) == 0 && len(typ.TargetDecl.Type.TypeParams) == 0 {
		return nil
	}

	return []plan.Diagnostic{{
		Level:   plan.DiagnosticLevelFatal,
		Path:    plan.TypesPath(typ.SourceType, typ.TargetType),
		Message: "generic root mappings are not supported; map concrete instantiations through containing types or provide a higher-order callable",
	}}
}

func invertStructSpec(in *spec.Struct) *spec.Struct {
	if in == nil {
		return nil
	}

	out := *in
	out.Fields = make(map[string]spec.Field, len(in.Fields))
	for sourceName, field := range in.Fields {
		targetName := structFieldTarget(sourceName, field)
		field.Target = sourceName
		out.Fields[targetName] = field
	}
	return &out
}

func validateStructFieldMappings(typ *plan.Type) []plan.Diagnostic {
	if len(typ.StructSpec.Fields) == 0 {
		return nil
	}

	var out []plan.Diagnostic

	sourceFields := plannableFieldsByName(typ.SourceDecl)
	targetFields := plannableFieldsByName(typ.TargetDecl)

	// Collect and sort source field names so the output of this is stable.
	sourceFieldNames := slices.Collect(maps.Keys(typ.StructSpec.Fields))
	slices.Sort(sourceFieldNames)

	sourcesByTarget := make(map[string][]string)
	targetFieldNames := make([]string, 0, len(sourceFieldNames))

	for _, sourceName := range sourceFieldNames {
		fieldSpec := typ.StructSpec.Fields[sourceName]
		targetName := structFieldTarget(sourceName, fieldSpec)

		if _, ok := sourceFields[sourceName]; !ok {
			out = append(out, plan.Diagnostic{
				Level:   plan.DiagnosticLevelFatal,
				Path:    plan.SourceFieldPath(typ.SourceType, typ.TargetType, sourceName),
				Message: fmt.Sprintf("source field %q does not exist or is not plannable; fields must be exported and non-embedded", sourceName),
			})
		}

		if _, ok := targetFields[targetName]; !ok {
			out = append(out, plan.Diagnostic{
				Level:   plan.DiagnosticLevelFatal,
				Path:    plan.TargetFieldPath(typ.SourceType, typ.TargetType, targetName),
				Message: fmt.Sprintf("target field %q does not exist or is not plannable; fields must be exported and non-embedded", targetName),
			})
		}

		if _, ok := sourcesByTarget[targetName]; !ok {
			targetFieldNames = append(targetFieldNames, targetName)
		}

		sourcesByTarget[targetName] = append(sourcesByTarget[targetName], sourceName)
	}

	slices.Sort(targetFieldNames)

	for _, targetName := range targetFieldNames {
		sourceNames := sourcesByTarget[targetName]
		if len(sourceNames) > 1 {
			out = append(out, plan.Diagnostic{
				Level:   plan.DiagnosticLevelFatal,
				Path:    plan.TargetFieldPath(typ.SourceType, typ.TargetType, targetName),
				Message: fmt.Sprintf("target field %q is mapped from multiple source fields %q", targetName, sourceNames),
			})
		}
	}

	return out
}

type rootVariant struct {
	Key      string
	Location plan.OutputLocation
	Root     *plan.Type
}

// addRoot records a requested root mapper in the shallow plan. It indexes the root as an
// output-scoped variant, reserves the generated function name within its Go package, tracks the
// output file so discovery can ignore stale generated functions, and attaches the root to the
// output group that will emit it.
func (p *attemptPlanner) addRoot(
	outputGroups map[plan.OutputLocation]plan.OutputGroup,
	location plan.OutputLocation,
	root *plan.Type,
) error {
	mapperKey := plan.TypeMapperKey(root.Source, root.Target, root.Signature)
	pairKey := plan.TypePairKey(root.Source, root.Target)
	variantKey := rootVariantKey(location, root)
	ref := spec.CallableRef{ImportPath: location.ImportPath, Name: root.FunctionName}

	if existing, ok := p.rootVariantsByCallable[ref]; ok {
		existingMapperKey := plan.TypeMapperKey(existing.Root.Source, existing.Root.Target, existing.Root.Signature)
		if existing.Key != variantKey {
			return fmt.Errorf("function name %q is planned for both %s and %s", root.FunctionName, existingMapperKey, mapperKey)
		}
		if existing.Location != location {
			return fmt.Errorf("function name %q is planned in multiple output files within package %q", root.FunctionName, location.ImportPath)
		}
		if !sameRootPlanningConfig(existing.Root, root) {
			return fmt.Errorf("conflicting mapper configuration for %s", mapperKey)
		}
		return nil
	}

	variant := &rootVariant{
		Key:      variantKey,
		Location: location,
		Root:     root,
	}

	p.rootVariantsByTypePair[pairKey] = append(p.rootVariantsByTypePair[pairKey], variant)
	p.mappings[variantKey] = root
	p.shallowMappings[variantKey] = struct{}{}
	p.rootVariantsByCallable[ref] = variant
	p.plannedOutputFiles[filepath.Clean(location.LogicalPath)] = struct{}{}

	outputGroup, ok := outputGroups[location]
	if !ok {
		// Initialize a newly found output group at this location.
		outputGroup = plan.OutputGroup{
			Location: location,
		}
	}

	outputGroup.Roots = append(outputGroup.Roots, root)
	outputGroups[location] = outputGroup
	return nil
}

func rootVariantKey(location plan.OutputLocation, root *plan.Type) string {
	return strings.Join([]string{
		plan.TypeMapperKey(root.Source, root.Target, root.Signature),
		location.ImportPath,
		root.FunctionName,
	}, "|")
}

func sameRootPlanningConfig(a, b *plan.Type) bool {
	return sameEnumSpec(a.EnumSpec, b.EnumSpec) &&
		sameTieredCallables(a.Callables, b.Callables) &&
		sameStructSpec(a.StructSpec, b.StructSpec) &&
		a.Optionality == b.Optionality &&
		a.Conversions == b.Conversions
}

func sameEnumSpec(a, b spec.Enum) bool {
	return a.FailureMode == b.FailureMode &&
		a.Patterns == b.Patterns &&
		maps.Equal(a.Values, b.Values)
}

func sameStructSpec(a, b spec.Struct) bool {
	return maps.EqualFunc(a.Fields, b.Fields, sameFieldSpec)
}

func sameFieldSpec(a, b spec.Field) bool {
	return a.Target == b.Target &&
		a.Optionality == b.Optionality &&
		a.Conversions == b.Conversions &&
		sameCallableRefPtr(a.Callable, b.Callable)
}

func sameCallableRefPtr(a, b *spec.CallableRef) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func sameTieredCallables(a, b []spec.TieredCallables) bool {
	return slices.EqualFunc(a, b, func(a, b spec.TieredCallables) bool {
		return a.Tier == b.Tier && slices.Equal(a.Callables, b.Callables)
	})
}

func (p *attemptPlanner) isFunctionPendingGeneration(fn types.FunctionDecl, ref spec.CallableRef) bool {
	_, plannedFunc := p.rootVariantsByCallable[ref]
	_, plannedFile := p.plannedOutputFiles[filepath.Clean(fn.SourceFile)]
	return plannedFunc || plannedFile
}

func (p *attemptPlanner) outputLocationForPackages(
	sourcePkg types.Package,
	targetPkg types.Package,
	output spec.Output,
) (plan.OutputLocation, error) {
	switch output.Strategy {
	case spec.OutputStrategySinglePackage:
		return p.outputLocationForPackage(output)
	case spec.OutputStrategySourcePackage:
		return p.outputLocationForExistingPackage(sourcePkg, output), nil
	case spec.OutputStrategyTargetPackage:
		return p.outputLocationForExistingPackage(targetPkg, output), nil
	default:
		return plan.OutputLocation{}, fmt.Errorf("unknown output strategy %q", output.Strategy.String())
	}
}

func (p *attemptPlanner) outputLocationForPackage(output spec.Output) (plan.OutputLocation, error) {
	pkgs := p.loader.Packages()

	logicalDir := filepath.ToSlash(filepath.Clean(filepath.Join(p.workspace.WorkingDir, output.Path)))

	// Check if the package already exists, and is already loaded
	for _, pkg := range pkgs {
		if pkg.Dir == logicalDir {
			return p.outputLocationForExistingPackage(pkg, output), nil
		}
	}

	// Otherwise, we build up the module information ourselves...
	relativeToModule, err := filepath.Rel(p.workspace.ModuleDir, logicalDir)
	if err != nil {
		return plan.OutputLocation{}, fmt.Errorf("failed to determine output path relative to module root: %w", err)
	}

	if relativeToModule == ".." || strings.HasPrefix(relativeToModule, ".."+string(filepath.Separator)) {
		return plan.OutputLocation{}, fmt.Errorf("output path %q is outside of the module root %q", logicalDir, p.workspace.ModuleDir)
	}

	importPath := p.workspace.ModulePath
	if relativeToModule != "." {
		importPath = path.Join(p.workspace.ModulePath, relativeToModule)
	}

	// We also need to check if the package exists already on disk, and if it does, we need to use
	// the package name already used within that package. If we just put in what the user has asked
	// for, we might generate something that won't compile.
	packageName, ok, err := packageNameFromDir(logicalDir)
	if err != nil {
		return plan.OutputLocation{}, fmt.Errorf("failed to determine package name from output directory: %w", err)
	}
	if !ok {
		// If we couldn't find a package name from the directory, we'll just use package name from
		// the output spec as fallback and pray.
		packageName = output.Package
	}

	return plan.OutputLocation{
		LogicalPath: filepath.ToSlash(filepath.Join(logicalDir, output.Filename)),
		ImportPath:  importPath,
		PackageName: packageName,
	}, nil
}

func (p *attemptPlanner) outputLocationForExistingPackage(pkg types.Package, output spec.Output) plan.OutputLocation {
	return plan.OutputLocation{
		LogicalPath: filepath.ToSlash(filepath.Join(pkg.Dir, output.Filename)),
		ImportPath:  pkg.ImportPath,
		PackageName: pkg.Name,
	}
}

func (p *attemptPlanner) planOutputGroup(outputGroup plan.OutputGroup) {
	previous := p.currentOutputLocation
	p.currentOutputLocation = &outputGroup.Location
	defer func() {
		p.currentOutputLocation = previous
	}()

	for _, typ := range outputGroup.Roots {
		p.planTypeWithKey(rootVariantKey(outputGroup.Location, typ), typ)
	}
}

func (p *attemptPlanner) planType(typ *plan.Type) {
	p.planTypeWithKey(plan.TypeMapperKey(typ.Source, typ.Target, typ.Signature), typ)
}

func (p *attemptPlanner) planTypeWithKey(key string, typ *plan.Type) {
	if _, isPlanning := p.planningMappings[key]; isPlanning {
		return
	}

	_, isMapped := p.mappings[key]
	_, isShallow := p.shallowMappings[key]
	if isMapped && !isShallow {
		// This one is already done.
		return
	}

	p.planningMappings[key] = struct{}{}
	defer delete(p.planningMappings, key)

	switch {
	case isEnumType(typ.SourceDecl) && isEnumType(typ.TargetDecl):
		p.planEnum(typ)
	case isStructType(typ.SourceDecl) && isStructType(typ.TargetDecl):
		p.planStruct(typ)
	default:
		typ.Diagnostics = appendDiagnostic(typ.Diagnostics, plan.Diagnostic{
			Level:   plan.DiagnosticLevelFatal,
			Path:    plan.TypesPath(typ.SourceType, typ.TargetType),
			Message: "unsupported type mapping requested; source and target must both be structs or both be enums",
		})
	}

	p.diagnostics = appendDiagnostic(p.diagnostics, typ.Diagnostics...)

	// Mark this as fully planned.
	delete(p.shallowMappings, key)
	// Ensure it's in the map of mappings.
	p.mappings[key] = typ
}

func (p *attemptPlanner) resolveStructType(typ types.Type) (types.TypeDecl, bool) {
	typ = types.UnwrapAlias(typ)
	if typ.Kind != types.TypeKindNamed {
		return types.TypeDecl{}, false
	}

	typeDecl, ok := p.resolveTypeDeclaration(typ)
	if !ok || !isStructType(typeDecl) {
		return types.TypeDecl{}, false
	}

	return typeDecl, true
}

func (p *attemptPlanner) resolveTypeDeclaration(typ types.Type) (types.TypeDecl, bool) {
	for _, pkg := range p.loader.Packages() {
		if pkg.ImportPath != typ.Package.ImportPath {
			continue
		}

		for _, pkgType := range pkg.Types {
			if pkgType.Name == typ.Name {
				// Type names are unique within a package, we don't need to get fancy.
				return pkgType, true
			}
		}
	}

	return types.TypeDecl{}, false
}

func packageNameFromDir(dir string) (name string, ok bool, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read output dir: %w", err)
	}

	fset := token.NewFileSet()

	var found string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filename := entry.Name()
		if filepath.Ext(filename) != ".go" || strings.HasSuffix(filename, "_test.go") {
			continue
		}

		filePath := filepath.Join(dir, filename)
		file, err := parser.ParseFile(fset, filePath, nil, parser.PackageClauseOnly)
		if err != nil {
			return "", false, fmt.Errorf("parse package clause in %q: %w", filePath, err)
		}

		if found == "" {
			found = file.Name.Name
			continue
		}

		if file.Name.Name != found {
			return "", false, fmt.Errorf("multiple packages found in output dir %q: %q and %q", dir, found, file.Name.Name)
		}
	}

	return found, found != "", nil
}

func sortedOutputGroups(outputGroups map[plan.OutputLocation]plan.OutputGroup) []plan.OutputGroup {
	out := slices.Collect(maps.Values(outputGroups))
	slices.SortFunc(out, func(a, b plan.OutputGroup) int {
		return cmp.Or(
			cmp.Compare(a.Location.LogicalPath, b.Location.LogicalPath),
			cmp.Compare(a.Location.ImportPath, b.Location.ImportPath),
			cmp.Compare(a.Location.PackageName, b.Location.PackageName),
		)
	})
	return out
}

// appendDiagnostic appends only distinct diagnostics to the given slice of diagnostics.
func appendDiagnostic(dd []plan.Diagnostic, diagnostics ...plan.Diagnostic) []plan.Diagnostic {
	distinct := make(map[plan.Diagnostic]struct{}, len(dd))
	for _, d := range dd {
		distinct[d] = struct{}{}
	}

	for _, diagnostic := range diagnostics {
		if _, ok := distinct[diagnostic]; !ok {
			dd = append(dd, diagnostic)
			distinct[diagnostic] = struct{}{}
		}
	}

	return dd
}

func conversionSet(conversions []spec.Conversion) map[spec.Conversion]struct{} {
	out := make(map[spec.Conversion]struct{}, len(conversions))
	for _, conversion := range conversions {
		out[conversion] = struct{}{}
	}
	return out
}
