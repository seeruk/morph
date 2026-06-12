package morph

import (
	"fmt"
	"go/format"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode"

	gotypes "go/types"

	"github.com/danielgtaylor/casing"
	"github.com/seeruk/morph/internal"
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
)

// Generator turns Morph plans into formatted Go source files.
type Generator struct{}

// NewGenerator returns a new Generator.
func NewGenerator() *Generator {
	return &Generator{}
}

// Generate turns a plan into formatted Go source files.
func (g *Generator) Generate(p Plan) ([]OutputFile, error) {
	if p.HasFatalDiagnostics() {
		return nil, fmt.Errorf("cannot generate code for a plan with fatal diagnostics")
	}

	outputGroups := slices.Clone(p.OutputGroups)
	slices.SortFunc(outputGroups, func(a, b plan.OutputGroup) int {
		return compareOutputLocation(a.Location, b.Location)
	})

	out := make([]OutputFile, 0, len(outputGroups))
	for _, group := range outputGroups {
		file, err := newFileGenerator(group).Generate()
		if err != nil {
			return nil, err
		}
		out = append(out, file)
	}

	return out, nil
}

type fileGenerator struct {
	group   plan.OutputGroup
	imports *importNamer
	writer  *sourceWriter
}

func newFileGenerator(group plan.OutputGroup) *fileGenerator {
	return &fileGenerator{
		group:   group,
		imports: newImportNamer(group.Location.ImportPath),
	}
}

func (g *fileGenerator) Generate() (OutputFile, error) {
	body, err := g.renderBodyWithImports()
	if err != nil {
		return OutputFile{}, err
	}

	w := newSourceWriter()
	w.Line(internal.MorphFileHeader)
	w.Line("package %s", g.group.Location.PackageName)
	w.Blank()
	g.renderImports(w)
	w.Write(body)

	source, err := format.Source([]byte(w.String()))
	if err != nil {
		return OutputFile{}, fmt.Errorf("format generated source %q: %w\n%s", g.group.Location.LogicalPath, err, w.String())
	}

	return OutputFile{
		LogicalPath: g.group.Location.LogicalPath,
		PackageName: g.group.Location.PackageName,
		Source:      source,
	}, nil
}

func (g *fileGenerator) renderBodyWithImports() (string, error) {
	// The first render pass discovers every import used by the file body. Import aliases can change
	// after later references introduce package-name collisions, so this body is intentionally
	// discarded.
	if _, err := g.renderBody(); err != nil {
		return "", err
	}

	// Render again once importNamer has the full import set and final aliases. A separate
	// import-discovery walker would duplicate much of the rendering logic, so the two-pass render
	// keeps import discovery tied to the code paths that actually emit references.
	return g.renderBody()
}

func (g *fileGenerator) renderBody() (string, error) {
	g.writer = newSourceWriter()

	var rendered bool
	renderMapper := func(typ *plan.Type) error {
		if rendered {
			g.writer.Blank()
		}
		rendered = true
		return g.renderMapper(typ)
	}

	for _, root := range g.group.Roots {
		if err := renderMapper(root); err != nil {
			return "", err
		}
	}
	for _, nested := range g.group.Nested {
		if err := renderMapper(nested); err != nil {
			return "", err
		}
	}

	return g.writer.String(), nil
}

func (g *fileGenerator) renderImports(w *sourceWriter) {
	imports := g.imports.Imports()
	if len(imports) == 0 {
		return
	}

	w.Line("import (")
	w.Indent(func() {
		for _, imp := range imports {
			if imp.Alias == "" {
				w.Line("%q", imp.Path)
				continue
			}
			w.Line("%s %q", imp.Alias, imp.Path)
		}
	})
	w.Line(")")
	w.Blank()
}

func (g *fileGenerator) renderMapper(typ *plan.Type) error {
	switch {
	case typ.StructPlan != nil:
		return g.renderStructMapper(typ)
	case typ.EnumPlan != nil:
		return g.renderEnumMapper(typ)
	default:
		return fmt.Errorf("cannot generate mapper %q without a struct or enum plan", typ.FunctionName)
	}
}

func (g *fileGenerator) renderStructMapper(typ *plan.Type) error {
	sourceType := g.renderMapperSourceType(typ)
	targetType := renderType(g.imports, typ.TargetType)
	scope := g.newRenderScope(returnShape{
		ReturnsError: typ.CanError,
		ZeroValue:    g.renderMapperReturnZeroValue(typ),
	})

	g.writer.Line("func %s(source %s) %s {", typ.FunctionName, sourceType, g.renderMapperResults(typ))
	g.writer.Indent(func() {
		g.renderMapperNilSourceGuard(scope, typ)
		g.writer.Line("var target %s", targetType)
	})

	for _, property := range typ.StructPlan.Properties {
		name := renderNameFromProperty(property)
		g.writer.Indent(func() {
			source, err := g.renderMemberRead(scope, "source", property.Source, name)
			if err != nil {
				scope.Err = err
				return
			}
			expr, err := g.renderValue(scope, source, property.Mapping, name)
			if err != nil {
				scope.Err = err
				return
			}
			if err := g.renderMemberWrite(scope, "target", property.Target, expr); err != nil {
				scope.Err = err
			}
		})
		if scope.Err != nil {
			return scope.Err
		}
	}

	g.writer.Indent(func() {
		g.renderReturn(scope, g.renderMapperReturnExpr(typ, "target"))
	})
	g.writer.Line("}")
	return nil
}

func (g *fileGenerator) renderEnumMapper(typ *plan.Type) error {
	scope := g.newRenderScope(returnShape{
		ReturnsError: typ.CanError,
		ZeroValue:    g.renderMapperReturnZeroValue(typ),
	})

	g.writer.Line("func %s(source %s) %s {", typ.FunctionName, g.renderMapperSourceType(typ), g.renderMapperResults(typ))
	g.writer.Indent(func() {
		g.renderMapperNilSourceGuard(scope, typ)
		source := "source"
		if typ.Signature.Accepts == spec.ParameterKindPointer {
			source = "*source"
		}
		g.writer.Line("switch %s {", source)
		g.writer.Indent(func() {
			for _, value := range typ.EnumPlan.Values {
				g.writer.Line("case %s:", g.renderConstant(value.Source))
				g.writer.Indent(func() {
					g.renderReturnMappedValue(scope, typ, g.renderConstant(value.Target))
				})
			}
		})
		g.writer.Line("}")

		switch typ.EnumPlan.FailureMode {
		case spec.EnumFailureModeZero:
			g.renderReturn(scope, scope.ZeroValue)
		case spec.EnumFailureModeFallback:
			g.renderReturnMappedValue(scope, typ, g.renderConstant(typ.EnumPlan.FallbackValue))
		case spec.EnumFailureModeError:
			g.renderReturnError(scope, g.errorf("no enum mapping for %v", source))
		default:
			scope.Err = fmt.Errorf("cannot generate enum mapper %q with unknown failure mode %q", typ.FunctionName, typ.EnumPlan.FailureMode.String())
		}
	})
	if scope.Err != nil {
		return scope.Err
	}
	g.writer.Line("}")
	return nil
}

func (g *fileGenerator) renderMapperSourceType(typ *plan.Type) string {
	sourceType := renderType(g.imports, typ.SourceType)
	if typ.Signature.Accepts == spec.ParameterKindPointer {
		return "*" + sourceType
	}
	return sourceType
}

func (g *fileGenerator) renderMapperReturnType(typ *plan.Type) string {
	targetType := renderType(g.imports, typ.TargetType)
	if typ.Signature.Returns == spec.ParameterKindPointer {
		return "*" + targetType
	}
	return targetType
}

func (g *fileGenerator) renderMapperResults(typ *plan.Type) string {
	returnType := g.renderMapperReturnType(typ)
	if typ.CanError {
		return "(" + returnType + ", error)"
	}
	return returnType
}

func (g *fileGenerator) renderMapperReturnZeroValue(typ *plan.Type) string {
	if typ.Signature.Returns == spec.ParameterKindPointer {
		return "nil"
	}
	return renderZeroValue(g.imports, typ.TargetType)
}

func (g *fileGenerator) renderMapperReturnExpr(typ *plan.Type, expr string) string {
	if typ.Signature.Returns == spec.ParameterKindPointer {
		return "&" + expr
	}
	return expr
}

func (g *fileGenerator) renderMapperNilSourceGuard(scope *renderScope, typ *plan.Type) {
	if typ.Signature.Accepts != spec.ParameterKindPointer {
		return
	}

	g.writer.Line("if source == nil {")
	g.writer.Indent(func() {
		if typ.CanError && typ.Optionality.OnNilSourcePointer == spec.PointerOptionalityError {
			g.renderReturnError(scope, g.errorf("nil source pointer"))
			return
		}
		g.renderReturn(scope, scope.ZeroValue)
	})
	g.writer.Line("}")
}

type renderName string

func renderNameFromProperty(property plan.Property) renderName {
	name := property.Target.Name
	if name == "" {
		name = property.Source.Name
	}
	return renderName(localNameBase(name))
}

func (g *fileGenerator) renderMemberRead(
	scope *renderScope,
	source string,
	member plan.Member,
	name renderName,
) (string, error) {
	switch member.Kind {
	case plan.MemberKindField:
		return source + "." + member.Accessor, nil
	case plan.MemberKindMethod:
		call := source + "." + member.Accessor + "()"
		if !member.CanError {
			return call, nil
		}
		valueName := scope.Names.Next(name.Suffix("Value", "sourceValue"))
		errName := scope.Names.Next("err")
		g.writer.Line("%s, %s := %s", valueName, errName, call)
		g.writer.Line("if %s != nil {", errName)
		g.writer.Indent(func() {
			g.renderReturnError(scope, errName)
		})
		g.writer.Line("}")
		return valueName, nil
	default:
		return "", fmt.Errorf("cannot render unknown member kind %q", member.Kind)
	}
}

func (g *fileGenerator) renderMemberWrite(
	scope *renderScope,
	target string,
	member plan.Member,
	expr string,
) error {
	switch member.Kind {
	case plan.MemberKindField:
		g.writer.Line("%s.%s = %s", target, member.Accessor, expr)
	case plan.MemberKindMethod:
		call := fmt.Sprintf("%s.%s(%s)", target, member.Accessor, expr)
		if !member.CanError {
			g.writer.Line("%s", call)
			return nil
		}
		errName := scope.Names.Next("err")
		g.writer.Line("if %s := %s; %s != nil {", errName, call, errName)
		g.writer.Indent(func() {
			g.renderReturnError(scope, errName)
		})
		g.writer.Line("}")
	default:
		return fmt.Errorf("cannot render unknown member kind %q", member.Kind)
	}
	return nil
}

func (n renderName) Base(fallback string) string {
	if n == "" {
		return fallback
	}
	return string(n)
}

func (n renderName) Suffix(suffix, fallback string) string {
	if n == "" {
		return fallback
	}
	return string(n) + suffix
}

func (n renderName) Suffixed(suffix string) renderName {
	if n == "" {
		return ""
	}
	return renderName(string(n) + suffix)
}

func (g *fileGenerator) renderValue(scope *renderScope, source string, value plan.Value, name renderName) (string, error) {
	if len(value.SourceAdaptations) > 0 {
		return g.renderSourceAdaptation(scope, source, value, name)
	}

	expr, err := g.renderOperation(scope, source, value, name)
	if err != nil {
		return "", err
	}

	return g.renderTargetAdaptations(scope, expr, value, name)
}

func (g *fileGenerator) renderSourceAdaptation(scope *renderScope, source string, value plan.Value, name renderName) (string, error) {
	adaptation := value.SourceAdaptations[0]
	rest := value
	rest.SourceAdaptations = slices.Clone(value.SourceAdaptations[1:])

	switch adaptation {
	case plan.ValueAdaptationAddress:
		sourceValue := scope.Names.Next(name.Suffix("Value", "sourceValue"))
		g.writer.Line("%s := %s", sourceValue, source)
		return g.renderValue(scope, "&"+sourceValue, rest, name)
	case plan.ValueAdaptationDeref:
		mapped := scope.Names.Next(name.Base("mapped"))
		g.writer.Line("var %s %s", mapped, renderType(g.imports, value.Target))
		if value.Optionality.OnNilSourcePointer == spec.PointerOptionalityError {
			g.writer.Line("if %s == nil {", source)
			g.writer.Indent(func() {
				g.renderReturnError(scope, g.errorf("nil source pointer"))
			})
			g.writer.Line("}")
			expr, err := g.renderValue(scope, "*"+source, rest, name)
			if err != nil {
				return "", err
			}
			g.writer.Line("%s = %s", mapped, expr)
		} else {
			g.writer.Line("if %s != nil {", source)
			g.writer.Indent(func() {
				expr, err := g.renderValue(scope, "*"+source, rest, name)
				if err != nil {
					scope.Err = err
					return
				}
				g.writer.Line("%s = %s", mapped, expr)
			})
			g.writer.Line("}")
		}
		if scope.Err != nil {
			return "", scope.Err
		}
		return mapped, nil
	default:
		return "", fmt.Errorf("cannot generate unknown source adaptation %q", adaptation)
	}
}

func (g *fileGenerator) renderTargetAdaptations(scope *renderScope, source string, value plan.Value, name renderName) (string, error) {
	expr := source
	exprType := operationTargetType(value)
	for _, adaptation := range value.TargetAdaptations {
		switch adaptation {
		case plan.ValueAdaptationAddress:
			targetValue := scope.Names.Next(name.Suffix("Value", "targetValue"))
			g.writer.Line("%s := %s", targetValue, expr)
			targetType := types.PointerTo(exprType)
			target := scope.Names.Next(name.Base("mapped"))
			if value.Optionality.OnZeroSourceValue == spec.ValueOptionalityNil {
				g.writer.Line("var %s %s", target, renderType(g.imports, targetType))
				g.writer.Line("if %s {", renderNonZeroCheck(g.imports, targetValue, exprType))
				g.writer.Indent(func() {
					g.writer.Line("%s = &%s", target, targetValue)
				})
				g.writer.Line("}")
			} else {
				g.writer.Line("%s := &%s", target, targetValue)
			}
			expr = target
			exprType = targetType
		case plan.ValueAdaptationDeref:
			targetPointer := scope.Names.Next(name.Suffix("Pointer", "targetPointer"))
			g.writer.Line("%s := %s", targetPointer, expr)
			targetType := derefPointerType(exprType)
			target := scope.Names.Next(name.Base("mapped"))
			g.writer.Line("var %s %s", target, renderType(g.imports, targetType))
			if value.Optionality.OnNilSourcePointer == spec.PointerOptionalityError {
				g.writer.Line("if %s == nil {", targetPointer)
				g.writer.Indent(func() {
					g.renderReturnError(scope, g.errorf("nil source pointer"))
				})
				g.writer.Line("}")
				g.writer.Line("%s = *%s", target, targetPointer)
			} else {
				g.writer.Line("if %s != nil {", targetPointer)
				g.writer.Indent(func() {
					g.writer.Line("%s = *%s", target, targetPointer)
				})
				g.writer.Line("}")
			}
			expr = target
			exprType = targetType
		default:
			return "", fmt.Errorf("cannot generate unknown target adaptation %q", adaptation)
		}
	}
	return expr, nil
}

func (g *fileGenerator) renderOperation(scope *renderScope, source string, value plan.Value, name renderName) (string, error) {
	switch value.Operation {
	case plan.OperationAssign:
		return source, nil
	case plan.OperationConvert:
		return fmt.Sprintf("%s(%s)", renderType(g.imports, operationTargetType(value)), source), nil
	case plan.OperationFunction, plan.OperationMethod:
		return g.renderCallable(scope, source, value, name)
	case plan.OperationStruct, plan.OperationEnum:
		return g.renderGeneratedMapperCall(scope, source, value, name)
	case plan.OperationSlice:
		return g.renderSlice(scope, source, value, name)
	case plan.OperationArray:
		return g.renderArray(scope, source, value, name)
	case plan.OperationMap:
		return g.renderMap(scope, source, value, name)
	case plan.OperationUnsupported:
		return "", fmt.Errorf("cannot generate unsupported mapping from %s to %s", types.TypeKey(value.Source), types.TypeKey(value.Target))
	default:
		return "", fmt.Errorf("cannot generate unknown mapping operation %q", value.Operation)
	}
}

func (g *fileGenerator) renderCallable(scope *renderScope, source string, value plan.Value, name renderName) (string, error) {
	if value.Callable == nil {
		return "", fmt.Errorf("cannot generate callable operation without a callable")
	}

	args, err := g.renderCallableArgs(scope, value.CallableArgs)
	if err != nil {
		return "", err
	}

	var call string
	switch value.Operation {
	case plan.OperationFunction:
		callArgs := append([]string{source}, args...)
		call = fmt.Sprintf("%s(%s)", g.renderCallableName(*value.Callable), strings.Join(callArgs, ", "))
	case plan.OperationMethod:
		call = fmt.Sprintf("%s.%s(%s)", source, value.Callable.Name, strings.Join(args, ", "))
	default:
		return "", fmt.Errorf("cannot generate callable operation %q", value.Operation)
	}

	if !value.Callable.ReturnsError {
		return call, nil
	}

	return g.renderErroringCall(scope, name, call), nil
}

func (g *fileGenerator) renderCallableArgs(scope *renderScope, args []plan.CallableArg) ([]string, error) {
	out := make([]string, 0, len(args))
	for i := range args {
		base := "mapValue"
		if len(args) > 1 {
			base = fmt.Sprintf("mapValue%d", i+1)
		}
		name := scope.Names.Next(base)
		if err := g.renderCallableArg(scope, name, args[i]); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, nil
}

func (g *fileGenerator) renderCallableArg(scope *renderScope, name string, arg plan.CallableArg) error {
	sourceType := renderType(g.imports, arg.Mapping.Source)
	targetType := renderType(g.imports, arg.Mapping.Target)
	shape := returnShape{
		ReturnsError: arg.ReturnsError,
		ZeroValue:    renderZeroValue(g.imports, arg.Mapping.Target),
	}
	child := g.newRenderScope(shape)

	results := targetType
	if arg.ReturnsError {
		results = "(" + targetType + ", error)"
	}

	g.writer.Line("%s := func(source %s) %s {", name, sourceType, results)
	g.writer.Indent(func() {
		expr, err := g.renderValue(child, "source", arg.Mapping, "")
		if err != nil {
			child.Err = err
			return
		}
		g.renderReturn(child, expr)
	})
	g.writer.Line("}")

	return child.Err
}

func (g *fileGenerator) renderGeneratedMapperCall(scope *renderScope, source string, value plan.Value, name renderName) (string, error) {
	if value.Plan == nil {
		return "", fmt.Errorf("cannot generate generated mapper call without a mapper plan")
	}

	call := fmt.Sprintf("%s(%s)", g.renderGeneratedMapperName(value.Plan), source)
	if !value.Plan.CanError {
		return call, nil
	}

	return g.renderErroringCall(scope, name, call), nil
}

func (g *fileGenerator) renderErroringCall(scope *renderScope, name renderName, call string) string {
	mapped := scope.Names.Next(name.Base("mapped"))
	errName := scope.Names.Next("err")
	g.writer.Line("%s, %s := %s", mapped, errName, call)
	g.writer.Line("if %s != nil {", errName)
	g.writer.Indent(func() {
		g.renderReturnError(scope, errName)
	})
	g.writer.Line("}")
	return mapped
}

func (g *fileGenerator) renderSlice(scope *renderScope, source string, value plan.Value, name renderName) (string, error) {
	if value.Elem == nil {
		return "", fmt.Errorf("cannot generate slice mapping without an element mapping")
	}

	sourceValues := scope.Names.Next(name.Suffix("Values", "sourceValues"))
	target := scope.Names.Next(name.Base("mapped"))
	sourceValue := scope.Names.Next(name.Suffix("Value", "sourceValue"))
	targetType := renderType(g.imports, operationTargetType(value))
	g.writer.Line("%s := %s", sourceValues, source)
	g.writer.Line("var %s %s", target, targetType)
	g.writer.Line("if %s != nil {", sourceValues)
	g.writer.Indent(func() {
		g.writer.Line("%s = make(%s, len(%s))", target, targetType, sourceValues)
		g.writer.Line("for i, %s := range %s {", sourceValue, sourceValues)
		g.writer.Indent(func() {
			expr, err := g.renderValue(scope, sourceValue, *value.Elem, name.Suffixed("Value"))
			if err != nil {
				scope.Err = err
				return
			}
			g.writer.Line("%s[i] = %s", target, expr)
		})
		g.writer.Line("}")
	})
	g.writer.Line("}")
	if scope.Err != nil {
		return "", scope.Err
	}
	return target, nil
}

func (g *fileGenerator) renderArray(scope *renderScope, source string, value plan.Value, name renderName) (string, error) {
	if value.Elem == nil {
		return "", fmt.Errorf("cannot generate array mapping without an element mapping")
	}

	target := scope.Names.Next(name.Base("mapped"))
	sourceValue := scope.Names.Next(name.Suffix("Value", "sourceValue"))
	g.writer.Line("var %s %s", target, renderType(g.imports, operationTargetType(value)))
	g.writer.Line("for i, %s := range %s {", sourceValue, source)
	g.writer.Indent(func() {
		expr, err := g.renderValue(scope, sourceValue, *value.Elem, name.Suffixed("Value"))
		if err != nil {
			scope.Err = err
			return
		}
		g.writer.Line("%s[i] = %s", target, expr)
	})
	g.writer.Line("}")
	if scope.Err != nil {
		return "", scope.Err
	}
	return target, nil
}

func (g *fileGenerator) renderMap(scope *renderScope, source string, value plan.Value, name renderName) (string, error) {
	if value.Key == nil || value.Value == nil {
		return "", fmt.Errorf("cannot generate map mapping without key and value mappings")
	}

	sourceValues := scope.Names.Next(name.Suffix("Values", "sourceValues"))
	target := scope.Names.Next(name.Base("mapped"))
	sourceKey := scope.Names.Next(name.Suffix("Key", "sourceKey"))
	sourceValue := scope.Names.Next(name.Suffix("Value", "sourceValue"))
	targetType := renderType(g.imports, operationTargetType(value))
	g.writer.Line("%s := %s", sourceValues, source)
	g.writer.Line("var %s %s", target, targetType)
	g.writer.Line("if %s != nil {", sourceValues)
	g.writer.Indent(func() {
		g.writer.Line("%s = make(%s, len(%s))", target, targetType, sourceValues)
		g.writer.Line("for %s, %s := range %s {", sourceKey, sourceValue, sourceValues)
		g.writer.Indent(func() {
			key, err := g.renderValue(scope, sourceKey, *value.Key, name.Suffixed("Key"))
			if err != nil {
				scope.Err = err
				return
			}
			mappedValue, err := g.renderValue(scope, sourceValue, *value.Value, name.Suffixed("Value"))
			if err != nil {
				scope.Err = err
				return
			}
			g.writer.Line("%s[%s] = %s", target, key, mappedValue)
		})
		g.writer.Line("}")
	})
	g.writer.Line("}")
	if scope.Err != nil {
		return "", scope.Err
	}
	return target, nil
}

func (g *fileGenerator) renderReturn(scope *renderScope, expr string) {
	if scope.ReturnsError {
		g.writer.Line("return %s, nil", expr)
		return
	}
	g.writer.Line("return %s", expr)
}

func (g *fileGenerator) renderReturnMappedValue(scope *renderScope, typ *plan.Type, expr string) {
	if typ.Signature.Returns != spec.ParameterKindPointer {
		g.renderReturn(scope, expr)
		return
	}

	target := scope.Names.Next("target")
	g.writer.Line("%s := %s", target, expr)
	g.renderReturn(scope, "&"+target)
}

func (g *fileGenerator) renderReturnError(scope *renderScope, errExpr string) {
	if scope.ReturnsError {
		g.writer.Line("return %s, %s", scope.ZeroValue, errExpr)
		return
	}
	g.writer.Line("return %s", scope.ZeroValue)
}

func (g *fileGenerator) renderCallableName(callable plan.CallableRef) string {
	if callable.Package.ImportPath == "" || callable.Package.ImportPath == g.group.Location.ImportPath {
		return callable.Name
	}
	return g.imports.Name(callable.Package) + "." + callable.Name
}

func (g *fileGenerator) renderGeneratedMapperName(mapper *plan.Type) string {
	if mapper.Location.ImportPath == "" || mapper.Location.ImportPath == g.group.Location.ImportPath {
		return mapper.FunctionName
	}
	return g.imports.Name(types.PackageRef{
		Name:       mapper.Location.PackageName,
		ImportPath: mapper.Location.ImportPath,
	}) + "." + mapper.FunctionName
}

func (g *fileGenerator) renderConstant(constant types.ConstantDecl) string {
	if constant.Package.ImportPath == "" || constant.Package.ImportPath == g.group.Location.ImportPath {
		return constant.Name
	}
	return g.imports.Name(constant.Package) + "." + constant.Name
}

func (g *fileGenerator) errorf(message string, args ...string) string {
	fmtName := g.imports.Name(types.PackageRef{Name: "fmt", ImportPath: "fmt"})
	parts := make([]string, 0, len(args)+1)
	parts = append(parts, strconv.Quote("morph: "+message))
	parts = append(parts, args...)
	return fmt.Sprintf("%s.Errorf(%s)", fmtName, strings.Join(parts, ", "))
}

type renderScope struct {
	Names *nameAllocator
	returnShape
	Err error
}

func (g *fileGenerator) newRenderScope(shape returnShape) *renderScope {
	names := newNameAllocator()
	names.Reserve("source", "target")
	names.Reserve(g.imports.LocalNames()...)
	return &renderScope{
		Names:       names,
		returnShape: shape,
	}
}

type returnShape struct {
	ReturnsError bool
	ZeroValue    string
}

type sourceWriter struct {
	sb     strings.Builder
	indent int
}

func newSourceWriter() *sourceWriter {
	return &sourceWriter{}
}

func (w *sourceWriter) Line(format string, args ...any) {
	if format != "" {
		w.sb.WriteString(strings.Repeat("\t", w.indent))
		fmt.Fprintf(&w.sb, format, args...)
	}
	w.sb.WriteByte('\n')
}

func (w *sourceWriter) Blank() {
	w.Line("")
}

func (w *sourceWriter) Indent(fn func()) {
	w.indent++
	defer func() {
		w.indent--
	}()
	fn()
}

func (w *sourceWriter) Write(value string) {
	w.sb.WriteString(value)
}

func (w *sourceWriter) String() string {
	return w.sb.String()
}

type importNamer struct {
	currentImportPath string
	byPath            map[string]*importName
}

type importName struct {
	Path        string
	PackageName string
	Alias       string
}

func newImportNamer(currentImportPath string) *importNamer {
	return &importNamer{
		currentImportPath: currentImportPath,
		byPath:            make(map[string]*importName),
	}
}

func (n *importNamer) Name(pkg types.PackageRef) string {
	if pkg.ImportPath == "" || pkg.ImportPath == n.currentImportPath {
		return pkg.Name
	}

	name := pkg.Name
	if name == "" {
		name = importPathPackageName(pkg.ImportPath)
	}

	imp, ok := n.byPath[pkg.ImportPath]
	if !ok {
		imp = &importName{
			Path:        pkg.ImportPath,
			PackageName: name,
		}
		n.byPath[pkg.ImportPath] = imp
		n.updateAliases()
	}

	if imp.Alias != "" {
		return imp.Alias
	}
	return imp.PackageName
}

func (n *importNamer) Imports() []importName {
	out := make([]importName, 0, len(n.byPath))
	for _, imp := range n.byPath {
		out = append(out, *imp)
	}
	slices.SortFunc(out, func(a, b importName) int {
		return strings.Compare(a.Path, b.Path)
	})
	return out
}

func (n *importNamer) LocalNames() []string {
	imports := n.Imports()
	out := make([]string, 0, len(imports))
	for _, imp := range imports {
		if imp.Alias != "" {
			out = append(out, imp.Alias)
			continue
		}
		out = append(out, imp.PackageName)
	}
	return out
}

func (n *importNamer) updateAliases() {
	byName := make(map[string][]*importName)
	for _, imp := range n.byPath {
		byName[imp.PackageName] = append(byName[imp.PackageName], imp)
		imp.Alias = ""
	}

	for _, imports := range byName {
		if len(imports) == 1 {
			continue
		}
		slices.SortFunc(imports, func(a, b *importName) int {
			return strings.Compare(a.Path, b.Path)
		})
		used := make(map[string]struct{}, len(imports))
		for _, imp := range imports {
			alias := importPathAlias(imp.Path)
			base := alias
			for i := 2; ; i++ {
				if _, ok := used[alias]; !ok {
					break
				}
				alias = fmt.Sprintf("%s%d", base, i)
			}
			used[alias] = struct{}{}
			imp.Alias = alias
		}
	}
}

type nameAllocator struct {
	used map[string]int
}

func newNameAllocator() *nameAllocator {
	return &nameAllocator{used: make(map[string]int)}
}

func (a *nameAllocator) Reserve(names ...string) {
	for _, name := range names {
		if _, ok := a.used[name]; !ok {
			a.used[name] = 1
		}
	}
}

func (a *nameAllocator) Next(base string) string {
	if base == "" {
		base = "value"
	}

	next := a.used[base]
	if next == 0 {
		a.used[base] = 1
		return base
	}

	name := fmt.Sprintf("%s%d", base, next+1)
	a.used[base] = next + 1
	a.used[name] = 1
	return name
}

func localNameBase(name string) string {
	parts := normalizeLocalNameParts(casing.MergeNumbers(casing.Split(name)))
	if len(parts) == 0 {
		return "value"
	}

	var sb strings.Builder
	for i, part := range parts {
		sb.WriteString(localNamePart(part, i == 0))
	}

	out := sb.String()
	if out == "" {
		return "value"
	}
	if startsWithDigit(out) {
		out = "value" + uppercaseFirst(out)
	}
	if token.Lookup(out).IsKeyword() {
		out += "Value"
	}
	return out
}

func normalizeLocalNameParts(parts []string) []string {
	out := make([]string, 0, len(parts))
	for i := 0; i < len(parts); i++ {
		if i+1 < len(parts) {
			candidate := parts[i] + strings.TrimSuffix(parts[i+1], "s")
			if strings.HasSuffix(parts[i+1], "s") && isCommonInitialism(candidate) {
				out = append(out, strings.ToUpper(candidate)+"s")
				i++
				continue
			}
		}
		out = append(out, parts[i])
	}
	return out
}

func localNamePart(part string, first bool) string {
	if initialism, ok := pluralInitialism(part); ok {
		if first {
			return strings.ToLower(initialism + "s")
		}
		return initialism + "s"
	}

	part = strings.ToLower(part)
	initialism := casing.Initialism(part)
	if first {
		return strings.ToLower(initialism)
	}
	if initialism != part {
		return initialism
	}
	return uppercaseFirst(part)
}

func pluralInitialism(part string) (string, bool) {
	if !strings.HasSuffix(part, "s") {
		return "", false
	}
	part = strings.TrimSuffix(part, "s")
	if !isCommonInitialism(part) {
		return "", false
	}
	return strings.ToUpper(part), true
}

func isCommonInitialism(part string) bool {
	part = strings.ToLower(part)
	return casing.Initialism(part) == strings.ToUpper(part)
}

func startsWithDigit(value string) bool {
	for _, r := range value {
		return unicode.IsDigit(r)
	}
	return false
}

func renderType(imports *importNamer, typ types.Type) string {
	switch typ.Kind {
	case types.TypeKindAlias:
		if typ.Elem != nil {
			return renderType(imports, *typ.Elem)
		}
		return renderNamedType(imports, typ)
	case types.TypeKindNamed:
		return renderNamedType(imports, typ)
	case types.TypeKindBasic, types.TypeKindTypeParam:
		return typ.Name
	case types.TypeKindPointer:
		return "*" + renderType(imports, derefType(typ.Elem))
	case types.TypeKindSlice:
		return "[]" + renderType(imports, derefType(typ.Elem))
	case types.TypeKindArray:
		return fmt.Sprintf("[%d]%s", typ.Len, renderType(imports, derefType(typ.Elem)))
	case types.TypeKindMap:
		return fmt.Sprintf("map[%s]%s", renderType(imports, derefType(typ.Key)), renderType(imports, derefType(typ.Value)))
	case types.TypeKindSignature:
		return renderSignatureType(imports, typ)
	case types.TypeKindChan:
		return renderChanType(imports, typ)
	case types.TypeKindStruct, types.TypeKindInterface:
		if typ.String != "" {
			return typ.String
		}
	}
	if typ.String != "" {
		return typ.String
	}
	return typ.Name
}

func operationTargetType(value plan.Value) types.Type {
	target := value.Target
	for i := len(value.TargetAdaptations) - 1; i >= 0; i-- {
		switch value.TargetAdaptations[i] {
		case plan.ValueAdaptationAddress:
			if target.Kind == types.TypeKindPointer && target.Elem != nil {
				target = *target.Elem
			}
		case plan.ValueAdaptationDeref:
			target = types.PointerTo(target)
		}
	}
	return target
}

func derefPointerType(typ types.Type) types.Type {
	typ = types.UnwrapAlias(typ)
	if typ.Kind == types.TypeKindPointer && typ.Elem != nil {
		return *typ.Elem
	}
	return typ
}

func renderNonZeroCheck(imports *importNamer, expr string, typ types.Type) string {
	typ = types.UnwrapAlias(typ)

	switch typ.Kind {
	case types.TypeKindBasic:
		return renderBasicNonZeroCheck(expr, typ.Name)
	case types.TypeKindNamed:
		return renderNamedNonZeroCheck(imports, expr, typ)
	case types.TypeKindPointer, types.TypeKindSlice, types.TypeKindMap,
		types.TypeKindSignature, types.TypeKindInterface, types.TypeKindChan:
		return expr + " != nil"
	case types.TypeKindArray, types.TypeKindStruct:
		if typeSupportsComparableZero(typ) {
			return renderComparableNonZeroCheck(imports, expr)
		}
	}

	return renderIncomparableNonZeroCheck(imports, expr)
}

func renderBasicNonZeroCheck(expr, name string) string {
	if name == "bool" {
		return expr
	}
	return expr + " != " + basicZeroValue(name)
}

func renderNamedNonZeroCheck(imports *importNamer, expr string, typ types.Type) string {
	if typ.Elem == nil {
		return renderIncomparableNonZeroCheck(imports, expr)
	}

	underlying := types.UnwrapAlias(*typ.Elem)
	switch underlying.Kind {
	case types.TypeKindBasic:
		return expr + " != " + renderZeroValue(imports, typ)
	case types.TypeKindPointer, types.TypeKindSlice, types.TypeKindMap,
		types.TypeKindSignature, types.TypeKindInterface, types.TypeKindChan:
		return expr + " != nil"
	case types.TypeKindArray, types.TypeKindStruct:
		if typeSupportsComparableZero(underlying) {
			return renderComparableNonZeroCheck(imports, expr)
		}
	}

	return renderIncomparableNonZeroCheck(imports, expr)
}

func renderComparableNonZeroCheck(imports *importNamer, expr string) string {
	return fmt.Sprintf("!%s.IsComparableZero(%s)", renderRuntimeValuePackage(imports), expr)
}

func renderIncomparableNonZeroCheck(imports *importNamer, expr string) string {
	return fmt.Sprintf("!%s.IsIncomparableZero(%s)", renderRuntimeValuePackage(imports), expr)
}

func renderRuntimeValuePackage(imports *importNamer) string {
	return imports.Name(types.PackageRef{
		Name:       "value",
		ImportPath: "github.com/seeruk/morph/runtime/value",
	})
}

func typeSupportsComparableZero(typ types.Type) bool {
	typ = types.UnwrapAlias(typ)

	switch typ.Kind {
	case types.TypeKindBasic, types.TypeKindPointer, types.TypeKindChan:
		return true
	case types.TypeKindNamed:
		return typ.Elem != nil && typeSupportsComparableZero(*typ.Elem)
	case types.TypeKindArray:
		return typ.Elem != nil && typeSupportsComparableZero(*typ.Elem)
	case types.TypeKindStruct:
		for _, field := range typ.Fields {
			if !typeSupportsComparableZero(field.Type) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

func renderNamedType(imports *importNamer, typ types.Type) string {
	name := typ.Name
	if typ.Package.ImportPath != "" && typ.Package.ImportPath != imports.currentImportPath {
		name = imports.Name(typ.Package) + "." + name
	}

	if len(typ.TypeArgs) == 0 {
		return name
	}

	args := make([]string, 0, len(typ.TypeArgs))
	for _, arg := range typ.TypeArgs {
		args = append(args, renderType(imports, arg))
	}
	return name + "[" + strings.Join(args, ", ") + "]"
}

func renderSignatureType(imports *importNamer, typ types.Type) string {
	params := make([]string, 0, len(typ.Params))
	for _, param := range typ.Params {
		params = append(params, renderType(imports, param.Type))
	}

	results := make([]string, 0, len(typ.Results))
	for _, result := range typ.Results {
		results = append(results, renderType(imports, result.Type))
	}

	out := "func(" + strings.Join(params, ", ") + ")"
	switch len(results) {
	case 0:
		return out
	case 1:
		return out + " " + results[0]
	default:
		return out + " (" + strings.Join(results, ", ") + ")"
	}
}

func renderChanType(imports *importNamer, typ types.Type) string {
	elem := renderType(imports, derefType(typ.Elem))
	switch typ.ChanDir {
	case gotypes.SendOnly:
		return "chan<- " + elem
	case gotypes.RecvOnly:
		return "<-chan " + elem
	default:
		return "chan " + elem
	}
}

func renderZeroValue(imports *importNamer, typ types.Type) string {
	typ = types.UnwrapAlias(typ)
	switch typ.Kind {
	case types.TypeKindBasic:
		return basicZeroValue(typ.Name)
	case types.TypeKindNamed:
		if typ.Name == "error" && typ.Package.ImportPath == "" {
			return "nil"
		}
		if typ.Elem == nil {
			return renderType(imports, typ) + "{}"
		}
		return namedZeroValue(imports, typ, *typ.Elem)
	case types.TypeKindPointer, types.TypeKindSlice, types.TypeKindMap,
		types.TypeKindSignature, types.TypeKindInterface, types.TypeKindChan:
		return "nil"
	case types.TypeKindArray, types.TypeKindStruct:
		return renderType(imports, typ) + "{}"
	case types.TypeKindTypeParam:
		return "*new(" + renderType(imports, typ) + ")"
	default:
		if typ.String != "" {
			return typ.String + "{}"
		}
		return "nil"
	}
}

func namedZeroValue(imports *importNamer, typ types.Type, underlying types.Type) string {
	name := renderType(imports, typ)
	underlying = types.UnwrapAlias(underlying)
	switch underlying.Kind {
	case types.TypeKindBasic:
		return name + "(" + basicZeroValue(underlying.Name) + ")"
	case types.TypeKindPointer, types.TypeKindSlice, types.TypeKindMap,
		types.TypeKindSignature, types.TypeKindInterface, types.TypeKindChan:
		return "nil"
	default:
		return name + "{}"
	}
}

func basicZeroValue(name string) string {
	switch name {
	case "bool":
		return "false"
	case "string":
		return `""`
	case "error":
		return "nil"
	default:
		return "0"
	}
}

func derefType(typ *types.Type) types.Type {
	if typ == nil {
		return types.Type{Kind: types.TypeKindInvalid}
	}
	return *typ
}

func importPathPackageName(importPath string) string {
	name := path.Base(importPath)
	return sanitizeIdentifier(name)
}

func importPathAlias(importPath string) string {
	parts := strings.Split(importPath, "/")
	if len(parts) > 2 {
		parts = parts[len(parts)-2:]
	}
	for i, part := range parts {
		parts[i] = sanitizeIdentifier(part)
	}
	return strings.Join(parts, "_")
}

func sanitizeIdentifier(value string) string {
	var sb strings.Builder
	for i, r := range value {
		if r == '_' || unicode.IsLetter(r) || i > 0 && unicode.IsDigit(r) {
			sb.WriteRune(r)
			continue
		}
		if unicode.IsDigit(r) {
			sb.WriteByte('_')
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	out := strings.Trim(sb.String(), "_")
	if out == "" {
		return "pkg"
	}
	return out
}
