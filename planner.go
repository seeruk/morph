package morph

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/seeruk/morph/internal/slicesx"
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
)

type Planner struct {
	// spec is the spec that this planner is planning for
	spec Spec
	// workingDir is the working directory of this Planner
	workingDir string

	// loader is the initialized type loader for this planner
	loader *types.Loader
	// registry is the callableRegistry used by this planner to find callables that could be used
	// to map between types
	registry *callableRegistry

	// Prepared state:
	// presetsByName is a map of presets in the spec, by name
	presetsByName map[string]spec.Preset
	// workspace contains the module and filesystem environment Morph is planning within
	workspace *Workspace

	// Planning state:
	// explicitRoots is a map of the planned mapping of explicitly requested types
	explicitRoots map[string]*plan.Type // plan.TypeMapperKey -> *plan.Type
	// mappings contains all planned mappings, and is built as the planner processes the plan
	mappings map[string]*plan.Type // plan.TypeMapperKey -> *plan.Type
	// plannedFunctions is a map of all the functions this planner is planning to generate. It being
	// keyed on spec.CallableRef means that it factors in the package.
	plannedFunctions map[spec.CallableRef]string
	// plannedOutputFiles is a map of the logical paths of all output files Morph is planning to
	// generate. This is useful for discovering functions in files we're about to generate.
	plannedOutputFiles map[string]struct{}
}

// NewPlanner returns a new Planner, set to plan the given Spec.
func NewPlanner(specification Spec, workingDir string) *Planner {
	return &Planner{
		spec:       specification,
		workingDir: workingDir,

		explicitRoots:      make(map[string]*plan.Type),
		mappings:           make(map[string]*plan.Type),
		plannedFunctions:   make(map[spec.CallableRef]string),
		plannedOutputFiles: make(map[string]struct{}),
	}
}

// Plan attempts to produce a Plan for the Spec assigned to this Planner.
func (p *Planner) Plan() (Plan, error) {
	// Filter out empty packages
	p.spec.Packages = slicesx.Filter(p.spec.Packages, func(pkg spec.Package) bool {
		return len(pkg.Types) > 0
	})

	var out Plan
	if err := p.prepare(); err != nil {
		return out, fmt.Errorf("failed to prepare planner: %w", err)
	}

	// Next we'll do a shallow pass over the spec to determine all the mapping functions we're going
	// to generate. This allows us to avoid auto-discovering functions we're about to generate when
	// we're planning value mapping, and also allows Morph to detect some other potential issues
	// earlier, for example, cyclic dependency issues.
	outputGroups, err := p.shallowPlan()
	if err != nil {
		return out, fmt.Errorf("failed shallow planning pass: %w", err)
	}

	out.OutputGroups = sortedOutputGroups(outputGroups)

	// Now the shallow plan is complete; we can safely set up discovery, knowing we're not going to
	// allow discovery to pick up on functions we're about to generate.
	if err := p.registerDiscovery(); err != nil {
		return out, fmt.Errorf("failed to register discovery: %w", err)
	}

	return out, nil
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

	p.preparePresets()
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

	for _, fn := range p.spec.Discovery.Functions {
		add(fn.ImportPath)
	}

	// TODO: Context?
	if err := loader.Load(context.Background(), slices.Collect(maps.Keys(distinct))...); err != nil {
		return fmt.Errorf("preparing primary type loader: loading types: %w", err)
	}

	p.loader = loader
	return nil
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

func (p *Planner) registerDiscovery() error {
	pkgs := p.loader.Packages()

	registry := newCallableRegistry()
	if err := p.registerDiscoveryFunctions(registry, pkgs); err != nil {
		return fmt.Errorf("failed to register discovered functions: %w", err)
	}

	if err := p.registerDiscoveryPackages(registry, pkgs); err != nil {
		return fmt.Errorf("failed to register discovered packages: %w", err)
	}

	p.registry = registry
	return nil
}

func (p *Planner) preparePresets() {
	p.presetsByName = make(map[string]spec.Preset, len(p.spec.Presets))
	for _, preset := range p.spec.Presets {
		p.presetsByName[preset.Name] = preset
	}
}

func (p *Planner) registerDiscoveryFunctions(registry *callableRegistry, pkgs map[string]types.Package) error {
	for _, discoveryFn := range p.spec.Discovery.Functions {
		pkg, ok := pkgs[discoveryFn.ImportPath]
		if !ok {
			return fmt.Errorf("package not found for function %q", discoveryFn.Name)
		}

		if discoveryFn.TypeName != "" {
			// Looking for a method
			typ, ok := pkg.Types[discoveryFn.TypeName]
			if !ok {
				return fmt.Errorf("type not found for function '%s.%s' ", discoveryFn.ImportPath, discoveryFn.Name)
			}

			method, ok := typ.Methods[discoveryFn.Name]
			if !ok {
				return fmt.Errorf("method not found for function '%s.%s.%s' ", discoveryFn.ImportPath, discoveryFn.TypeName, discoveryFn.Name)
			}

			if ok := registry.RegisterMethod(method, plan.CallableSourceUser); !ok {
				return fmt.Errorf("method is not viable for discovery '%s.%s.%s' ", discoveryFn.ImportPath, discoveryFn.TypeName, discoveryFn.Name)
			}
		} else {
			// Looking for a function
			fn, ok := pkg.Functions[discoveryFn.Name]
			if !ok {
				return fmt.Errorf("function not found for function '%s.%s' ", discoveryFn.ImportPath, discoveryFn.Name)
			}

			if ok := registry.RegisterFunction(fn, plan.CallableSourceUser); !ok {
				return fmt.Errorf("function is not viable for discovery '%s.%s' ", discoveryFn.ImportPath, discoveryFn.Name)
			}
		}
	}

	return nil
}

func (p *Planner) registerDiscoveryPackages(registry *callableRegistry, pkgs map[string]types.Package) error {
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

			registry.RegisterFunction(fn, plan.CallableSourceDiscovered)
		}
	}

	return nil
}

// shallowPlan does a shallow planning pass over the spec, not attempting to plan any value
// mappings, instead just determining the output groups and the explicitly requested mapping
// functions that need to be generated.
func (p *Planner) shallowPlan() (map[plan.OutputLocation]plan.OutputGroup, error) {
	// Prepare the top-level defaults, based on Morph defaults, overridden by user-specified
	// defaults from within the spec. We'll override these again as we go per-package, per-type...
	defaultOutput := outputWithDefaults(p.spec.Defaults.Packages.Output, defaultOutput)
	defaultTypes := typesDefaultsWithDefaults(p.spec.Defaults.Packages.Types, defaultTypesDefaults)

	outputGroups := make(map[plan.OutputLocation]plan.OutputGroup)
	for _, pkgSpec := range p.spec.Packages {
		if err := p.shallowPackagePlan(outputGroups, pkgSpec, defaultOutput, defaultTypes); err != nil {
			return nil, fmt.Errorf("failed to shallow plan package %q -> %q: %w", pkgSpec.Source, pkgSpec.Target, err)
		}
	}

	return outputGroups, nil
}

// shallowPackagePlan does a shallow planning pass for the given package spec.
func (p *Planner) shallowPackagePlan(
	outputGroups map[plan.OutputLocation]plan.OutputGroup,
	pkgSpec spec.Package,
	outputDefaults spec.Output,
	typeDefaults spec.TypesDefaults,
) error {
	pkgs := p.loader.Packages()

	sourcePkg, ok := pkgs[pkgSpec.Source]
	if !ok {
		return fmt.Errorf("source package not found %q", pkgSpec.Source)
	}

	targetPkg, ok := pkgs[pkgSpec.Target]
	if !ok {
		return fmt.Errorf("target package not found %q", pkgSpec.Target)
	}

	if pkgSpec.Preset != "" {
		// If a preset is set, attempt to find it by name
		preset, ok := p.presetsByName[pkgSpec.Preset]
		if !ok {
			return fmt.Errorf("preset not found %q", pkgSpec.Preset)
		}
		// And if found, apply it to the type defaults
		typeDefaults = typesDefaultsWithPreset(typeDefaults, preset)
	}

	// Always override the lower-level defaults with details from the package, at this point.
	typeDefaults = typesDefaultsWithPackageSpec(typeDefaults, pkgSpec)

	output := outputWithDefaults(pkgSpec.Output, outputDefaults)
	location, err := p.outputLocationForPackages(sourcePkg, targetPkg, output)
	if err != nil {
		return fmt.Errorf("failed to determine output location for packages %q -> %q: %w", pkgSpec.Source, pkgSpec.Target, err)
	}

	for _, typeSpec := range pkgSpec.Types {
		typeSpec.Source = cmp.Or(typeSpec.Source, typeSpec.Name)
		typeSpec.Target = cmp.Or(typeSpec.Target, typeSpec.Name)
		if typeSpec.Source == "" && typeSpec.Target == "" {
			return fmt.Errorf("type %q: either name, or source and target type name is required", typeSpec.Name)
		}

		// Forward should always be set, inverse may not be.
		forward, inverse, err := p.shallowTypePlan(sourcePkg, targetPkg, typeSpec, typeDefaults)
		if err != nil {
			return fmt.Errorf("failed to shallow plan type for packages %q -> %q: %w", pkgSpec.Source, pkgSpec.Target, err)
		}

		if err = p.addExplicitRoot(outputGroups, location, forward); err != nil {
			return fmt.Errorf("failed to add explicit root forward plan for packages %q -> %q: %w", pkgSpec.Source, pkgSpec.Target, err)
		}

		if inverse != nil {
			if err = p.addExplicitRoot(outputGroups, location, inverse); err != nil {
				return fmt.Errorf("failed to add explicit root inverse plan for packages %q -> %q: %w", pkgSpec.Source, pkgSpec.Target, err)
			}
		}
	}

	return nil
}

func (p *Planner) shallowTypePlan(
	sourcePkg, targetPkg types.Package,
	typeSpec spec.Type,
	typeDefaults spec.TypesDefaults,
) (*plan.Type, *plan.Type, error) {
	if typeSpec.Preset != "" {
		// If a preset is set, attempt to find it by name
		preset, ok := p.presetsByName[typeSpec.Preset]
		if !ok {
			return nil, nil, fmt.Errorf("preset not found %q", typeSpec.Preset)
		}
		// And if found, apply it to the type defaults
		typeDefaults = typesDefaultsWithPreset(typeDefaults, preset)
	}

	// Resolve the final spec for this type, with all layered defaults. The defaults should not
	// be used from this point on.
	typeSpec = specTypeWithTypesDefaults(typeSpec, typeDefaults)

	sourceDecl, ok := sourcePkg.Types[typeSpec.Source]
	if !ok {
		return nil, nil, fmt.Errorf("source type not found %q in package %q", typeSpec.Source, sourcePkg.ImportPath)
	}

	targetDecl, ok := targetPkg.Types[typeSpec.Target]
	if !ok {
		return nil, nil, fmt.Errorf("target type not found %q in package %q", typeSpec.Target, targetPkg.ImportPath)
	}

	var forward, inverse *plan.Type

	forward, err := p.shallowRootPlan(sourceDecl, targetDecl, typeSpec.Mappers.Forward)
	if err != nil {
		return nil, nil, fmt.Errorf("type %q -> %q: %w", typeSpec.Source, typeSpec.Target, err)
	}

	if typeSpec.Bidirectional != nil && *typeSpec.Bidirectional {
		inverse, err = p.shallowRootPlan(targetDecl, sourceDecl, typeSpec.Mappers.Inverse)
		if err != nil {
			return nil, nil, fmt.Errorf("type %q -> %q inverse: %w", typeSpec.Source, typeSpec.Target, err)
		}
	}

	return forward, inverse, nil
}

// shallowRootPlan prepares a shallow plan for a particular type mapping.
func (p *Planner) shallowRootPlan(sourceDecl, targetDecl types.TypeDecl, mapper spec.Mapper) (*plan.Type, error) {
	nameInput := NameInput{
		Source:     sourceDecl.Type,
		Target:     targetDecl.Type,
		TypeParams: sourceDecl.Type.TypeParams,
		Signature:  mapper.Signature,
	}

	functionName, err := MapperName(nameInput, mapper.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to determine mapper name: %w", err)
	}

	return &plan.Type{
		Source:       plan.TypeRefFromTypeDecl(sourceDecl),
		Target:       plan.TypeRefFromTypeDecl(targetDecl),
		SourceDecl:   sourceDecl,
		TargetDecl:   targetDecl,
		SourceType:   sourceDecl.Type,
		TargetType:   targetDecl.Type,
		FunctionName: functionName,
		TypeParams:   sourceDecl.Type.TypeParams,
		Signature:    mapper.Signature,
	}, nil
}

// addExplicitRoot records an explicitly requested mapper in the shallow plan. It keeps one
// canonical root per mapper key, reserves the generated function name within its Go package so
// later roots cannot collide, tracks output files that will be regenerated so discovery can ignore
// stale functions from them, and finally attaches the root to the output group that will emit it.
func (p *Planner) addExplicitRoot(
	outputGroups map[plan.OutputLocation]plan.OutputGroup,
	location plan.OutputLocation,
	root *plan.Type,
) error {
	mapperKey := plan.TypeMapperKey(root.Source, root.Target, root.Signature)

	if existing, ok := p.explicitRoots[mapperKey]; ok {
		if existing.FunctionName != root.FunctionName {
			return fmt.Errorf("conflicting mapper names %q and %q for %s", existing.FunctionName, root.FunctionName, mapperKey)
		}
		root = existing
	} else {
		p.explicitRoots[mapperKey] = root
		p.mappings[mapperKey] = root
	}

	ref := spec.CallableRef{ImportPath: location.ImportPath, Name: root.FunctionName}
	if existingKey, ok := p.plannedFunctions[ref]; ok {
		if existingKey == mapperKey {
			return nil
		}
		return fmt.Errorf("function name %q is planned for both %s and %s", root.FunctionName, existingKey, mapperKey)
	}

	p.plannedFunctions[ref] = mapperKey
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

func (p *Planner) isFunctionPendingGeneration(fn types.FunctionDecl, ref spec.CallableRef) bool {
	_, plannedFunc := p.plannedFunctions[ref]
	_, plannedFile := p.plannedOutputFiles[filepath.Clean(fn.SourceFile)]
	return plannedFunc || plannedFile
}

func (p *Planner) outputLocationForPackages(
	sourcePkg types.Package,
	targetPkg types.Package,
	output spec.Output,
) (plan.OutputLocation, error) {
	if output.Strategy == nil {
		return plan.OutputLocation{}, errors.New("missing output strategy")
	}

	switch *output.Strategy {
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

func (p *Planner) outputLocationForPackage(output spec.Output) (plan.OutputLocation, error) {
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

	return plan.OutputLocation{
		LogicalPath: filepath.ToSlash(filepath.Join(logicalDir, output.Filename)),
		ImportPath:  importPath,
		PackageName: output.Package,
	}, nil
}

func (p *Planner) outputLocationForExistingPackage(pkg types.Package, output spec.Output) plan.OutputLocation {
	return plan.OutputLocation{
		LogicalPath: filepath.ToSlash(filepath.Join(pkg.Dir, output.Filename)),
		ImportPath:  pkg.ImportPath,
		PackageName: pkg.Name,
	}
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
// TODO: Do we even need diagnostics? It is useful if something isn't a hard failure?
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
