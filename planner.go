package morph

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	"github.com/davecgh/go-spew/spew"
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
	// for the purposes of mapping between types
	registry *callableRegistry
	// workspace contains the module and filesystem environment Morph is planning within
	workspace *Workspace

	// explicitRoots is a map of the planned mapping of explicitly requested types
	explicitRoots map[string]*plan.Type // plan.TypeMapperKey -> *plan.Type
	// mappings contains all planned mappings, and is built as the planner processes the plan
	mappings map[string]*plan.Type // plan.TypeMapperKey -> *plan.Type
	// outputGroups contains output group specific state, things that are useful to keep track of
	// so that packages are generated correctly and efficiently
	outputGroups map[plan.OutputLocation]outputGroupState

	// explicitCallables keeps track of all the mapping functions Morph will generate for the
	// current spec. This allows us to avoid accidentally using a mapping function via discovery
	// that we're about to generate.
	explicitCallables map[spec.CallableRef]struct{}
}

// NewPlanner returns a new Planner, set to plan the given Spec.
func NewPlanner(spec Spec, workingDir string) *Planner {
	return &Planner{
		spec:       spec,
		workingDir: workingDir,
	}
}

// Plan attempts to produce a Plan for the Spec assigned to this Planner.
func (p *Planner) Plan() (Plan, error) {
	// Filter out empty packages
	p.spec.Packages = slicesx.Filter(p.spec.Packages, func(pkg spec.Package) bool {
		return len(pkg.Types) > 0
	})

	var out Plan

	// Bail early if there's nothing to do...
	if len(p.spec.Packages) == 0 {
		return out, errors.New("no mappings specified; only found empty packages and/or types")
	}

	// Set up the type loader, primed with all the packages specified at any point in the spec
	if err := p.prepareTypeLoader(); err != nil {
		return out, fmt.Errorf("failed to prepare type loader: %w", err)
	}

	// Figure out what environment we're operating within
	if err := p.prepareWorkspace(); err != nil {
		return out, fmt.Errorf("failed to prepare workspace: %w", err)
	}

	// Set up the registry, with all available and compatible conversions added
	if err := p.prepareRegistry(); err != nil {
		return out, fmt.Errorf("failed to prepare registry: %w", err)
	}

	// We determine all output locations upfront as part of our strategy for avoiding discovering
	// mapping functions we're about to generate. If we know which files we're going to generate,
	// then we know we can't pull in functions from those files.
	// TODO: Probably should be associated with something that lets us re-use this information
	outputLocations, err := p.determineOutputLocations()
	if err != nil {
		return out, fmt.Errorf("failed to determine output locations: %w", err)
	}

	for _, location := range outputLocations {
		spew.Dump(location)
	}

	return out, nil
}

func (p *Planner) prepareTypeLoader() error {
	loader := types.NewLoader(".") // TODO: Should this be smarter?

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
		add(pkg.ImportPath)
	}

	for _, conversion := range p.spec.Conversions {
		add(conversion.ImportPath)
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
		if !pkg.Module.Main {
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

func (p *Planner) prepareRegistry() error {
	pkgs := p.loader.Packages()

	registry := newCallableRegistry()
	if err := p.registerConversions(registry, pkgs); err != nil {
		return fmt.Errorf("failed to register conversions: %w", err)
	}

	if err := p.registerDiscovery(registry, pkgs); err != nil {
		return fmt.Errorf("failed to register discovered functions: %w", err)
	}

	p.registry = registry
	return nil
}

func (p *Planner) registerConversions(registry *callableRegistry, pkgs map[string]types.Package) error {
	for _, conversion := range p.spec.Conversions {
		pkg, ok := pkgs[conversion.ImportPath]
		if !ok {
			return fmt.Errorf("package not found for conversion %q", conversion.Name)
		}

		if conversion.TypeName != "" {
			// Looking for a method
			typ, ok := pkg.Types[conversion.TypeName]
			if !ok {
				return fmt.Errorf("type not found for conversion '%s.%s' ", conversion.ImportPath, conversion.Name)
			}

			method, ok := typ.Methods[conversion.Name]
			if !ok {
				return fmt.Errorf("method not found for conversion '%s.%s.%s' ", conversion.ImportPath, conversion.TypeName, conversion.Name)
			}

			// TODO: Could offer "debug" logging of failures
			registry.RegisterMethod(method, plan.CallableSourceUser)
		} else {
			// Looking for a function
			fn, ok := pkg.Functions[conversion.Name]
			if !ok {
				return fmt.Errorf("function not found for conversion '%s.%s' ", conversion.ImportPath, conversion.Name)
			}

			// TODO: Could offer "debug" logging of failures
			registry.RegisterFunction(fn, plan.CallableSourceUser)
		}
	}

	return nil
}

func (p *Planner) registerDiscovery(registry *callableRegistry, pkgs map[string]types.Package) error {
	// NOTE: Functions added to the registry here are based on code that already exists. In other
	// words, functions previously generated by Morph can be added here, i.e. ones that could be
	// removed by this run of Morph. Therefore, when deciding whether to use a discovered function
	// we must check that these functions would not otherwise be generated by this run of Morph.
	// Otherwise, we can pick one of these functions, and then it would no longer exist after Morph
	// has finished running, resulting in invalid generated code.
	//
	// It's also worth noting, many of these discovered functions may never be used!
	for _, discovery := range p.spec.Discovery.Packages {
		pkg, ok := pkgs[discovery.ImportPath]
		if !ok {
			return fmt.Errorf("package not found for discovery %q", discovery.ImportPath)
		}

		for _, fn := range pkg.Functions {
			// Skip unexported or explicitly excluded function declarations.
			// NOTE: Methods aren't discovered here, their exclusion is handled elsewhere.
			if !fn.IsExported || slices.Contains(p.spec.Discovery.Exclusions, spec.CallableRefFromFunctionDecl(fn)) {
				continue
			}
			registry.RegisterFunction(fn, plan.CallableSourceDiscovered)
		}
	}

	return nil
}

func (p *Planner) determineOutputLocations() ([]plan.OutputLocation, error) {
	defaultOutput := outputWithDefaults(p.spec.Defaults.Packages.Output, spec.DefaultOutput)

	pkgs := p.loader.Packages()

	var locs []plan.OutputLocation
	for _, pkg := range p.spec.Packages {
		output := outputWithDefaults(pkg.Output, defaultOutput)

		switch output.Strategy {
		case spec.OutputStrategySinglePackage:
			loc, err := p.outputLocationForPackage(pkgs, output)
			if err != nil {
				return nil, fmt.Errorf("failed to determine location for package %q: %w", pkg, err)
			}
			locs = append(locs, loc)
		case spec.OutputStrategySourcePackage:
			locs = append(locs, p.outputLocationForExistingPackage(pkgs[pkg.Source], output))
		case spec.OutputStrategyTargetPackage:
			locs = append(locs, p.outputLocationForExistingPackage(pkgs[pkg.Target], output))
		}
	}

	return locs, nil
}

func (p *Planner) outputLocationForPackage(pkgs map[string]types.Package, output spec.Output) (plan.OutputLocation, error) {
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

	if strings.HasPrefix(relativeToModule, "..") {
		return plan.OutputLocation{}, fmt.Errorf("output path %q is outside of the module root %q", logicalDir, p.workspace.ModuleDir)
	}

	importPath := p.workspace.ModulePath
	if relativeToModule != "." {
		importPath += string(filepath.Separator) + relativeToModule
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

type outputGroupState struct {
	functionNames map[string]string // typeKey -> name
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
