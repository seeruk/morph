package morph

import "github.com/seeruk/morph/plan"

type walkTypeKey struct {
	Type       *plan.Type
	ImportPath string
}

type walkContext struct {
	Location plan.OutputLocation
	Owner    *plan.Type
	Type     *plan.Type
	Value    *plan.Value
	Path     string
}

type walkCallbacks struct {
	TypePre   func(walkContext)
	TypePost  func(walkContext)
	ValuePre  func(walkContext)
	ValuePost func(walkContext)
}

func walkOutputGroups(outputGroups []plan.OutputGroup, callbacks walkCallbacks) {
	seen := make(map[walkTypeKey]struct{})
	for _, outputGroup := range outputGroups {
		for _, root := range outputGroup.Roots {
			walkType(root, outputGroup.Location, seen, callbacks)
		}
		for _, nested := range outputGroup.Nested {
			walkType(nested, outputGroup.Location, seen, callbacks)
		}
	}
}

func walkType(
	typ *plan.Type,
	fallbackLocation plan.OutputLocation,
	seen map[walkTypeKey]struct{},
	callbacks walkCallbacks,
) {
	if typ == nil {
		return
	}

	location := typ.Location
	if location == (plan.OutputLocation{}) {
		location = fallbackLocation
	}

	key := walkTypeKey{Type: typ, ImportPath: location.ImportPath}
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}

	path := plan.TypesPath(typ.SourceType, typ.TargetType)
	context := walkContext{
		Location: location,
		Owner:    typ,
		Type:     typ,
		Path:     path,
	}

	if callbacks.TypePre != nil {
		callbacks.TypePre(context)
	}

	if typ.StructPlan != nil {
		for i := range typ.StructPlan.Fields {
			field := &typ.StructPlan.Fields[i]
			fieldPath := plan.FieldPath(typ.SourceType, typ.TargetType, field.SourceField, field.TargetField)
			walkValue(&field.Mapping, typ, fieldPath, location, seen, callbacks)
		}
	}

	if callbacks.TypePost != nil {
		callbacks.TypePost(context)
	}
}

func walkValue(
	value *plan.Value,
	owner *plan.Type,
	path string,
	location plan.OutputLocation,
	seen map[walkTypeKey]struct{},
	callbacks walkCallbacks,
) {
	if value == nil {
		return
	}

	context := walkContext{
		Location: location,
		Owner:    owner,
		Type:     owner,
		Value:    value,
		Path:     path,
	}

	if callbacks.ValuePre != nil {
		callbacks.ValuePre(context)
	}

	for i := range value.CallableArgs {
		walkValue(
			&value.CallableArgs[i].Mapping,
			owner,
			callableArgPath(path, i),
			location,
			seen,
			callbacks,
		)
	}

	if value.Elem != nil {
		walkValue(value.Elem, owner, path+"[]", location, seen, callbacks)
	}
	if value.Key != nil {
		walkValue(value.Key, owner, path+"[key]", location, seen, callbacks)
	}
	if value.Value != nil {
		walkValue(value.Value, owner, path+"[value]", location, seen, callbacks)
	}
	if value.Plan != nil {
		walkType(value.Plan, value.Plan.Location, seen, callbacks)
	}

	if callbacks.ValuePost != nil {
		callbacks.ValuePost(context)
	}
}
