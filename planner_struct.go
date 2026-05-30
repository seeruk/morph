package morph

import (
	"fmt"
	"slices"
	"strings"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
)

func (p *Planner) planStruct(typ *plan.Type) {
	targetFields := plannableFieldsByName(typ.TargetDecl)

	var structPlan plan.Struct
	for _, targetField := range targetFields {
		sourceField, ok := matchingField(targetField, typ.SourceDecl, typ.StructSpec.Fields)
		if !ok {
			p.diagnostics = appendDiagnostic(p.diagnostics, plan.Diagnostic{
				Level:   plan.DiagnosticLevelWarning,
				Path:    plan.TypesPath(typ.SourceType, typ.TargetType),
				Message: fmt.Sprintf("no source field found for target field %q", targetField.Name),
			})
			continue
		}

		fieldPath := plan.FieldPath(typ.SourceType, typ.TargetType, sourceField, targetField)

		valuePlan := p.planValue(sourceField.Type, targetField.Type, fieldPath)
		if valuePlan.CanError {
			// Once set to true by any value mapping, this is never set back to false
			typ.CanError = true
		}

		structPlan.Fields = append(structPlan.Fields, plan.Field{
			SourceField: sourceField,
			TargetField: targetField,
			Mapping:     valuePlan,
		})
	}

	typ.StructPlan = &structPlan
}

func (p *Planner) planValue(sourceType, targetType types.Type, path string) plan.Value {
	sourceType = types.UnwrapAlias(sourceType)
	targetType = types.UnwrapAlias(targetType)

	// If there's a user-supplied, explicit function to use for this pair of types, prefer it.
	if fn, ok := p.registry.Find(sourceType, targetType, plan.CallableSourceUser); ok {
		return plan.Value{
			Operation: operationForCallable(fn),
			Source:    sourceType,
			Target:    targetType,
			Callable:  new(fn),
			CanError:  fn.ReturnsError,
		}
	}

	// If we discovered a suitable function to use for this pair of types, use that.
	if fn, ok := p.registry.Find(sourceType, targetType, plan.CallableSourceDiscovered); ok {
		return plan.Value{
			Operation: operationForCallable(fn),
			Source:    sourceType,
			Target:    targetType,
			Callable:  new(fn),
			CanError:  fn.ReturnsError,
		}
	}

	if explicit, ok := p.planExplicitRoot(sourceType, targetType); ok {
		return explicit
	}

	if sourceType.Kind == types.TypeKindPointer || targetType.Kind == types.TypeKindPointer {
		return p.planPointerMapping(sourceType, targetType, path)
	}

	if callable, ok := p.discoverValidMethodCallable(sourceType, targetType); ok {
		fmt.Println(callable.Name)
	}

	// TODO: Method conversion
	// TODO: Arrays, slices, maps, nested structs (?), same type (assignment), basic type conversion
	//  Any others?

	return plan.Value{}
}

// planExplicitRoot is used to "just-in-time" plan an explicit root, so that if we're going to
// generate a mapping function for a type pair, and we could use it elsewhere, we'll be able to
// refer to it in the plan. We already have the shallow plan, really the key thing we need to know
// is will this explicit root error, which can only identify if we fully plan it.
func (p *Planner) planExplicitRoot(source, target types.Type) (plan.Value, bool) {
	// Find the shallow plan, to do this we can't use the key.
	// TODO: Can we map the same source and target types with different signatures? In which case,
	//  would just prefer the best match here?

	sourceRef := plan.TypeRefFromType(source)
	targetRef := plan.TypeRefFromType(target)

	var typePlan *plan.Type
	for _, root := range p.explicitRoots {
		if root.Source == sourceRef && root.Target == targetRef {
			typePlan = root
			break
		}
	}

	if typePlan == nil {
		return plan.Value{}, false
	}

	// JIT plan type...
	// TODO: Can we run into infinite loops here?!
	p.planType(typePlan)

	operation := plan.OperationStruct
	if len(typePlan.Enum.Values) > 0 || isEnumType(typePlan.SourceDecl) && isEnumType(typePlan.TargetDecl) {
		operation = plan.OperationEnum
	}

	return plan.Value{
		Operation:   operation,
		Source:      source,
		Target:      target,
		Plan:        typePlan,
		CanError:    typePlan.CanError,
		Diagnostics: typePlan.Diagnostics,
	}, true
}

func (p *Planner) planPointerMapping(source, target types.Type, path string) plan.Value {
	sourcePointer := source.Kind == types.TypeKindPointer
	targetPointer := target.Kind == types.TypeKindPointer

	sourceElem := source
	if sourcePointer {
		sourceElem = *source.Elem
	}

	targetElem := target
	if targetPointer {
		targetElem = *target.Elem
	}

	elem := p.planValue(sourceElem, targetElem, path)
	operation := plan.OperationPointer
	if len(elem.Diagnostics) > 0 {
		operation = plan.OperationUnsupported
	}

	return plan.Value{
		Operation:     operation,
		Source:        source,
		Target:        target,
		Elem:          &elem,
		SourcePointer: sourcePointer,
		TargetPointer: targetPointer,
		CanError:      elem.CanError,
		Diagnostics:   elem.Diagnostics,
	}
}

func (p *Planner) discoverValidMethodCallable(sourceType, targetType types.Type) (plan.CallableRef, bool) {
	methodTypeDecl, _, ok := p.methodTypeDecl(sourceType)
	if !ok {
		return plan.CallableRef{}, false
	}

	var bestMatch plan.CallableRef
	var foundMatch bool
	for _, method := range methodTypeDecl.Methods {
		callable, ok := callableFromMethod(method, plan.CallableSourceDiscovered)
		if !ok {
			continue
		}

		specCallable := spec.CallableRef{
			ImportPath: callable.Package.ImportPath,
			TypeName:   callable.SourceType.Name,
			Name:       callable.Name,
		}

		// Skip explicitly excluded methods from this form of discovery.
		if slices.Contains(p.spec.Discovery.Exclusions, specCallable) {
			continue
		}

		// Check if the method is compatible for this pair of types, this includes checking if
		// methods with generic type parameters are compatible too.
		if !assessMethodCompatibility(sourceType, targetType, method, callable) {
			continue
		}

		// TODO: Prioritisation...
		bestMatch = callable
		foundMatch = true
	}

	return bestMatch, foundMatch
}

// methodTypeDecl attempts to unwrap a types.Type to the underlying named type (unaliased, not a \
// pointer).
func (p *Planner) methodTypeDecl(typ types.Type) (types.TypeDecl, types.Type, bool) {
	typ = types.Unwrap(typ)
	if typ.Kind != types.TypeKindNamed {
		return types.TypeDecl{}, types.Type{}, false
	}
	// Find the underlying named type in the type loader, and we'll grab the declaration and type.
	for importPath, pkg := range p.loader.Packages() {
		if importPath != typ.Package.ImportPath {
			continue
		}
		if methodType, ok := pkg.Types[typ.Name]; ok {
			return methodType, typ, true
		}
	}
	return types.TypeDecl{}, types.Type{}, false
}

// methodCompatibility is a type used to help prioritize compatible methods, allowing Morph to
// select, in theory, the most appropriate method for a pair of types.
type methodCompatibility struct {
	Receiver     methodReceiverCompatibility
	Result       methodResultCompatibility
	ReturnsError bool

	// Only set when Result is MethodResultGeneric.
	TypeBindings map[string]types.Type
}

// Compatible returns whether this method is compatible at all.
func (c methodCompatibility) Compatible() bool {
	return c.Receiver.Compatible() && c.Result.Compatible()
}

// methodReceiverCompatibility enumerates the possible levels of compatibility between a method's
// receiver and the source type Morph is trying to map from.
type methodReceiverCompatibility uint

const (
	methodReceiverIncompatible methodReceiverCompatibility = iota
	methodReceiverExact
	methodReceiverAutoAddress // source T, receiver *T
	methodReceiverAutoDeref   // source *T, receiver T
)

func (c methodReceiverCompatibility) Compatible() bool {
	return c != methodReceiverIncompatible
}

// methodResultCompatibility enumerates the possible levels of compatibility between a method's
// result and the target type Morph is trying to map to.
type methodResultCompatibility uint

const (
	methodResultIncompatible methodResultCompatibility = iota
	methodResultExact
	methodResultGeneric // e.g. receiver binds T=string, result T matches target string
)

func (c methodResultCompatibility) Compatible() bool {
	return c != methodResultIncompatible
}

func assessMethodCompatibility(
	sourceType, targetType types.Type,
	method types.Method,
	callable plan.CallableRef,
) methodCompatibility {
	if !method.IsExported {
		return methodCompatibility{} // Zero is incompatible.
	}

	// At this point, we already know this is a method on the source type, so we don't need to check
	// anything like that. So we can start with a very basic check; is the target type the same as
	// the first result of the method?
	if _, ok := callableResults(method.Results); !ok {
		return methodCompatibility{}
	}

	receiverCompat := assessMethodReceiverCompatibility(sourceType, method)
	if !receiverCompat.Compatible() {
		return methodCompatibility{}
	}

	// If the result is the same, then we have a compatible method!
	if sameType(targetType, method.Results[0].Type) {
		return methodCompatibility{
			Result:       methodResultExact,
			ReturnsError: callable.ReturnsError,
		}
	}

	return methodCompatibility{}
}

func assessMethodReceiverCompatibility(sourceType types.Type, method types.Method) methodReceiverCompatibility {
	if method.Receiver == nil {
		return methodReceiverIncompatible
	}

	sourceType = types.UnwrapAlias(sourceType)
	receiverType := types.UnwrapAlias(method.Receiver.Type)

	if sameType(sourceType, receiverType) {
		return methodReceiverExact
	}

	sourceElem, sourcePointer := pointerElem(sourceType)
	receiverElem, receiverPointer := pointerElem(receiverType)

	if !sourcePointer && receiverPointer && sameType(sourceType, receiverElem) {
		return methodReceiverAutoAddress
	}

	if sourcePointer && !receiverPointer && sameType(sourceElem, receiverType) {
		return methodReceiverAutoDeref
	}

	return methodReceiverIncompatible
}

// pointerElem unwraps aliased types, and returns the type and whether it's a pointer.
func pointerElem(typ types.Type) (types.Type, bool) {
	typ = types.UnwrapAlias(typ)
	if typ.Kind != types.TypeKindPointer || typ.Elem == nil {
		return typ, false
	}
	return types.UnwrapAlias(*typ.Elem), true
}

func sameType(a, b types.Type) bool {
	return plan.TypeKey(a) == plan.TypeKey(b)
}

// isStructType returns true if the given type declaration looks like a struct type, i.e. its
// underlying type is a struct.
func isStructType(typ types.TypeDecl) bool {
	return types.UnwrapAlias(typ.Underlying).Kind == types.TypeKindStruct
}

func matchingField(
	needle types.Field,
	typeDecl types.TypeDecl,
	mapping map[string]string,
) (field types.Field, ok bool) {
	fields := plannableFieldsByName(typeDecl)

	// Explicit field mapping takes precedence, as it's user-specified.
	if mappedName, ok := mapping[needle.Name]; ok {
		if field, ok = fields[mappedName]; ok {
			return field, ok
		}
	}

	// Then we'll fall back to exact name matching.
	if field, ok = fields[needle.Name]; ok {
		return field, ok
	}

	// If that didn't work, we'll try case-insensitive matching.
	for _, field := range fields {
		if strings.EqualFold(field.Name, needle.Name) {
			return field, true
		}
	}

	// NOTE: We don't try any more strategies here because they can be easily explicitly specified,
	// and any more advanced patterns would be too likely to retunr false positives, I think.

	// Otherwise, we failed...
	return field, false
}

func operationForCallable(callable plan.CallableRef) plan.Operation {
	if callable.Kind == plan.CallableKindMethod {
		return plan.OperationMethod
	}
	return plan.OperationFunction
}

func plannableFieldsByName(typeDecl types.TypeDecl) map[string]types.Field {
	out := make(map[string]types.Field)
	for _, field := range typeDecl.Fields {
		if !field.IsExported || field.IsEmbedded {
			// We only support regular, exported fields currently.
			continue
		}
		out[field.Name] = field
	}
	return out
}
