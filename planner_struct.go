package morph

import (
	"fmt"
	"strings"

	"github.com/seeruk/morph/plan"
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
