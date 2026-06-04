package morph

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
)

func (p *Planner) planStruct(typ *plan.Type) {
	sourceFields := plannableFieldsForType(typ.SourceDecl, typ.SourceType)
	targetFields := plannableFieldsForType(typ.TargetDecl, typ.TargetType)
	typeParams := typeParamScopeFrom(typ.TypeParams)

	var structPlan plan.Struct
	for _, targetField := range targetFields {
		sourceField, fieldSpec, mapped, ok := matchingField(targetField, sourceFields, typ.StructSpec.Fields)
		if !ok {
			diagnostic := plan.Diagnostic{
				Level:   plan.DiagnosticLevelWarning,
				Path:    plan.TypesPath(typ.SourceType, typ.TargetType),
				Message: fmt.Sprintf("no source field found for target field %q", targetField.Name),
			}
			typ.Diagnostics = appendDiagnostic(typ.Diagnostics, diagnostic)
			continue
		}

		fieldPath := plan.FieldPath(typ.SourceType, typ.TargetType, sourceField)
		optionality := typ.Optionality
		conversion := typ.Conversions
		if mapped {
			optionality = fieldSpec.Optionality
			conversion = fieldSpec.Conversions
		}

		var valuePlan plan.Value
		if mapped && fieldSpec.Callable != nil {
			valuePlan = p.planFieldCallable(
				sourceField.Type,
				targetField.Type,
				fieldPath,
				typeParams,
				optionality,
				conversion,
				*fieldSpec.Callable,
				typ.Callables,
			)
		} else {
			valuePlan = p.planValue(
				sourceField.Type,
				targetField.Type,
				fieldPath,
				typeParams,
				optionality,
				conversion,
				typ.Callables,
			)
		}
		typ.Diagnostics = appendDiagnostic(typ.Diagnostics, valuePlan.Diagnostics...)

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

func (p *Planner) planValue(
	sourceType, targetType types.Type,
	path string,
	typeParams typeParamScope,
	optionality spec.Optionality,
	conversion spec.ConversionsPolicy,
	callables []spec.TieredCallables,
) plan.Value {
	sourceType = types.UnwrapAlias(sourceType)
	targetType = types.UnwrapAlias(targetType)

	if value, ok := p.planTieredCallables(sourceType, targetType, path, typeParams, optionality, conversion, callables); ok {
		return value
	}

	if explicit, ok := p.planExplicitRoot(sourceType, targetType); ok {
		return explicit
	}

	// If broad package discovery found a suitable function to use for this pair of types, use that.
	if fn, compatibility, ok := p.discoverFunctionCallable(sourceType, targetType, plan.CallableSourceDiscovered); ok {
		return planCallableValue(sourceType, targetType, fn, compatibility, optionality)
	}

	if value, ok := p.planHigherOrderFunctionCallable(
		sourceType,
		targetType,
		path,
		typeParams,
		optionality,
		conversion,
		plan.CallableSourceDiscovered,
		callables,
	); ok {
		return value
	}

	if sourceType.Kind == types.TypeKindPointer || targetType.Kind == types.TypeKindPointer {
		return p.planPointerMapping(sourceType, targetType, path, typeParams, optionality, conversion, callables)
	}

	switch {
	case sourceType.Kind == types.TypeKindSlice && targetType.Kind == types.TypeKindSlice:
		elemPlan := p.planValue(
			*sourceType.Elem,
			*targetType.Elem,
			path+"[]",
			typeParams,
			optionality,
			conversion,
			callables,
		)

		operation := plan.OperationSlice
		if len(elemPlan.Diagnostics) > 0 {
			operation = plan.OperationUnsupported
		}

		return plan.Value{
			Operation:   operation,
			Source:      sourceType,
			Target:      targetType,
			Elem:        &elemPlan,
			Optionality: optionality,
			CanError:    elemPlan.CanError,
			Diagnostics: elemPlan.Diagnostics,
		}

	case sourceType.Kind == types.TypeKindArray && targetType.Kind == types.TypeKindArray:
		if sourceType.Len != targetType.Len {
			return unsupportedMapping(sourceType, targetType, path, "array lengths differ")
		}

		elemPlan := p.planValue(
			*sourceType.Elem,
			*targetType.Elem,
			path+"[]",
			typeParams,
			optionality,
			conversion,
			callables,
		)

		operation := plan.OperationArray
		if len(elemPlan.Diagnostics) > 0 {
			operation = plan.OperationUnsupported
		}

		return plan.Value{
			Operation:   operation,
			Source:      sourceType,
			Target:      targetType,
			Elem:        &elemPlan,
			Optionality: optionality,
			CanError:    elemPlan.CanError,
			Diagnostics: elemPlan.Diagnostics,
		}

	case sourceType.Kind == types.TypeKindMap && targetType.Kind == types.TypeKindMap:
		key := p.planValue(
			*sourceType.Key,
			*targetType.Key,
			path+"[key]",
			typeParams,
			optionality,
			conversion,
			callables,
		)
		value := p.planValue(
			*sourceType.Value,
			*targetType.Value,
			path+"[value]",
			typeParams,
			optionality,
			conversion,
			callables,
		)

		diagnostics := append([]plan.Diagnostic{}, key.Diagnostics...)
		diagnostics = append(diagnostics, value.Diagnostics...)

		operation := plan.OperationMap
		if len(diagnostics) > 0 {
			operation = plan.OperationUnsupported
		}

		return plan.Value{
			Operation:   operation,
			Source:      sourceType,
			Target:      targetType,
			Key:         &key,
			Value:       &value,
			Optionality: optionality,
			CanError:    key.CanError || value.CanError,
			Diagnostics: diagnostics,
		}
	}

	if nested, ok := p.planNestedStruct(sourceType, targetType, path, typeParams, callables); ok {
		return nested
	}

	if sameType(sourceType, targetType) {
		return plan.Value{
			Operation:   plan.OperationAssign,
			Source:      sourceType,
			Target:      targetType,
			Optionality: optionality,
		}
	}

	if p.canConvert(sourceType, targetType, conversion) {
		return plan.Value{
			Operation:   plan.OperationConvert,
			Source:      sourceType,
			Target:      targetType,
			Optionality: optionality,
		}
	}

	return unsupportedMapping(sourceType, targetType, path, "unable to determine mapping strategy")
}

func (p *Planner) planFieldCallable(
	sourceType, targetType types.Type,
	path string,
	typeParams typeParamScope,
	optionality spec.Optionality,
	conversion spec.ConversionsPolicy,
	ref spec.CallableRef,
	callables []spec.TieredCallables,
) plan.Value {
	sourceType = types.UnwrapAlias(sourceType)
	targetType = types.UnwrapAlias(targetType)

	refs := []spec.CallableRef{ref}
	if callable, compatibility, ok := p.discoverExplicitCallable(sourceType, targetType, refs); ok {
		return planCallableValue(sourceType, targetType, callable, compatibility, optionality)
	}
	if value, ok := p.planHigherOrderExplicitCallable(
		sourceType,
		targetType,
		path,
		typeParams,
		optionality,
		conversion,
		refs,
		callables,
	); ok {
		return value
	}

	return unsupportedMapping(sourceType, targetType, path, fmt.Sprintf("field callable %q is not compatible", ref.String()))
}

func (p *Planner) planTieredCallables(
	sourceType, targetType types.Type,
	path string,
	typeParams typeParamScope,
	optionality spec.Optionality,
	conversion spec.ConversionsPolicy,
	callables []spec.TieredCallables,
) (plan.Value, bool) {
	for _, tier := range callables {
		if callable, compatibility, ok := p.discoverExplicitCallable(sourceType, targetType, tier.Callables); ok {
			return planCallableValue(sourceType, targetType, callable, compatibility, optionality), true
		}
		if value, ok := p.planHigherOrderExplicitCallable(
			sourceType,
			targetType,
			path,
			typeParams,
			optionality,
			conversion,
			tier.Callables,
			callables,
		); ok {
			return value, true
		}
	}
	return plan.Value{}, false
}

func planCallableValue(
	sourceType, targetType types.Type,
	callable plan.CallableRef,
	compatibility callableCompatibility,
	optionality spec.Optionality,
) plan.Value {
	return plan.Value{
		Operation:                   operationForCallable(callable),
		Source:                      sourceType,
		Target:                      targetType,
		Callable:                    &callable,
		CallableParameterAdaptation: callableInputAdaptation(compatibility.Input),
		CallableResultAdaptation:    callableResultAdaptation(compatibility.Result),
		Optionality:                 optionality,
		CanError: callable.ReturnsError ||
			callableInputCanError(compatibility.Input, optionality) ||
			callableResultCanError(compatibility.Result, optionality),
	}
}

// planExplicitRoot is used to "just-in-time" plan an explicit root, so that if we're going to
// generate a mapping function for a type pair, and we could use it elsewhere, we'll be able to
// refer to it in the plan. We already have the shallow plan, really the key thing we need to know
// is will this explicit root error, which can only identify if we fully plan it.
func (p *Planner) planExplicitRoot(source, target types.Type) (plan.Value, bool) {
	variant := p.explicitRootVariant(source, target)
	if variant == nil {
		return plan.Value{}, false
	}

	typePlan := variant.Root
	previous := p.currentOutputLocation
	p.currentOutputLocation = &variant.Location
	p.planTypeWithKey(variant.Key, typePlan)
	p.currentOutputLocation = previous

	operation := plan.OperationStruct
	if typePlan.EnumPlan != nil || isEnumType(typePlan.SourceDecl) && isEnumType(typePlan.TargetDecl) {
		operation = plan.OperationEnum
	}

	return plan.Value{
		Operation:   operation,
		Source:      source,
		Target:      target,
		Plan:        typePlan,
		Optionality: typePlan.Optionality,
		CanError:    typePlan.CanError,
		Diagnostics: typePlan.Diagnostics,
	}, true
}

func (p *Planner) explicitRoot(source, target types.Type) *plan.Type {
	variant := p.explicitRootVariant(source, target)
	if variant == nil {
		return nil
	}
	return variant.Root
}

func (p *Planner) explicitRootVariant(source, target types.Type) *explicitRootVariant {
	sourceRef := plan.TypeRefFromType(source)
	targetRef := plan.TypeRefFromType(target)

	candidates := slices.Clone(p.explicitRootsByTypePair[plan.TypePairKey(sourceRef, targetRef)])
	if p.currentOutputLocation != nil {
		candidates = slices.DeleteFunc(candidates, func(candidate *explicitRootVariant) bool {
			return !p.canUseExplicitRootVariant(*p.currentOutputLocation, candidate)
		})
	}
	if len(candidates) == 0 {
		return nil
	}

	slices.SortFunc(candidates, func(a, b *explicitRootVariant) int {
		aRank := explicitRootSignatureRank(a.Root.Signature)
		bRank := explicitRootSignatureRank(b.Root.Signature)
		if aRank != bRank {
			return aRank - bRank
		}
		if p.currentOutputLocation != nil {
			aRank = explicitRootOutputPackageRank(*p.currentOutputLocation, a)
			bRank = explicitRootOutputPackageRank(*p.currentOutputLocation, b)
			if aRank != bRank {
				return aRank - bRank
			}
		}
		return cmp.Or(
			strings.Compare(a.Root.FunctionName, b.Root.FunctionName),
			strings.Compare(a.Location.ImportPath, b.Location.ImportPath),
			strings.Compare(plan.SignatureKey(a.Root.Signature), plan.SignatureKey(b.Root.Signature)),
			strings.Compare(a.Key, b.Key),
		)
	})

	return candidates[0]
}

func (p *Planner) canUseExplicitRootVariant(current plan.OutputLocation, candidate *explicitRootVariant) bool {
	if current.ImportPath == candidate.Location.ImportPath {
		return true
	}
	if p.importGraph == nil {
		return true
	}
	return p.importGraph.canAddEdge(current.ImportPath, candidate.Location.ImportPath)
}

func explicitRootSignatureRank(signature spec.MapperSignature) int {
	rank := 0
	if signature.Accepts == spec.ParameterKindPointer {
		rank += 1
	}
	if signature.Returns == spec.ParameterKindPointer {
		rank += 2
	}
	return rank
}

func explicitRootOutputPackageRank(current plan.OutputLocation, candidate *explicitRootVariant) int {
	if current.ImportPath == candidate.Location.ImportPath {
		return 0
	}
	return 1
}

func (p *Planner) planNestedStruct(
	source, target types.Type,
	path string,
	typeParams typeParamScope,
	callables []spec.TieredCallables,
) (plan.Value, bool) {
	sourceDecl, sourceOK := p.resolveStructType(source)
	targetDecl, targetOK := p.resolveStructType(target)
	if !sourceOK || !targetOK {
		return plan.Value{}, false
	}

	if sameType(source, target) {
		return plan.Value{}, false
	}

	defaultTypes := p.spec.Defaults.Types

	sourceRef := scopedTypeRef(source, typeParams)
	targetRef := scopedTypeRef(target, typeParams)
	callablesKey := callableContextKey(callables)
	if callablesKey != "" {
		sourceRef.Key += "|callables:" + callablesKey
	}

	nested := plan.Type{
		Source:     sourceRef,
		Target:     targetRef,
		SourceDecl: sourceDecl,
		TargetDecl: targetDecl,
		SourceType: source,
		TargetType: target,
		TypeParams: concreteTypeParams(sourceDecl, source, typeParams),
		Signature: spec.MapperSignature{ // TODO: Could be more granular
			Accepts: spec.ParameterKindValue,
			Returns: spec.ParameterKindValue,
		},
		EnumSpec:    defaultTypes.Enum,
		Callables:   callables,
		Optionality: defaultTypes.Optionality,
		Conversions: defaultTypes.Conversions,
		// We can't set structSpec in this case, because it's just field mapping currently. To have
		// field mapping this type pair would have to be defined explicitly.
	}

	key := plan.TypeMapperKey(nested.Source, nested.Target, nested.Signature)
	if existing, ok := p.mappings[key]; ok {
		p.planType(existing)
		return plan.Value{
			Operation:   plan.OperationStruct,
			Source:      source,
			Target:      target,
			Plan:        existing,
			Optionality: existing.Optionality,
			CanError:    existing.CanError,
			Diagnostics: existing.Diagnostics,
		}, true
	}

	var err error
	nested.FunctionName, err = p.nestedFunctionName(source, target, nested.Source.Key, nested.Target.Key, callablesKey)
	if err != nil {
		return unsupportedMapping(source, target, path, fmt.Sprintf("failed to generate nested function name: %v", err)), false
	}

	p.mappings[key] = &nested
	p.shallowMappings[key] = struct{}{}
	p.planType(&nested)

	return plan.Value{
		Operation:   plan.OperationStruct,
		Source:      source,
		Target:      target,
		Plan:        &nested,
		Optionality: nested.Optionality,
		CanError:    nested.CanError,
		Diagnostics: nested.Diagnostics,
	}, true
}

func (p *Planner) planPointerMapping(
	source, target types.Type,
	path string,
	typeParams typeParamScope,
	optionality spec.Optionality,
	conversion spec.ConversionsPolicy,
	callables []spec.TieredCallables,
) plan.Value {
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

	elem := p.planValue(sourceElem, targetElem, path, typeParams, optionality, conversion, callables)

	operation := plan.OperationPointer
	if len(elem.Diagnostics) > 0 {
		operation = plan.OperationUnsupported
	}

	return plan.Value{
		Operation:     operation,
		Source:        source,
		Target:        target,
		Elem:          &elem,
		Optionality:   optionality,
		SourcePointer: sourcePointer,
		TargetPointer: targetPointer,
		CanError:      elem.CanError || pointerMappingCanError(sourcePointer, targetPointer, optionality),
		Diagnostics:   elem.Diagnostics,
	}
}

func (p *Planner) discoverFunctionCallable(
	sourceType, targetType types.Type,
	source plan.CallableSource,
) (plan.CallableRef, callableCompatibility, bool) {
	candidates := make(map[plan.CallableRef]callableCompatibility)
	for _, fn := range p.registry.Candidates(sourceType, targetType, source) {
		callable, ok := plan.CallableRefFromFunctionDecl(fn, source)
		if !ok {
			continue
		}

		compatibility := assessFunctionCompatibility(sourceType, targetType, fn)
		if !compatibility.Compatible() {
			continue
		}

		candidates[callable] = compatibility
	}

	callable, compatibility, ok := bestCallableCandidate(candidates)
	if !ok {
		return plan.CallableRef{}, callableCompatibility{}, false
	}
	return callable, compatibility, true
}

func (p *Planner) discoverExplicitCallable(
	sourceType, targetType types.Type,
	refs []spec.CallableRef,
) (plan.CallableRef, callableCompatibility, bool) {
	candidates := make(map[plan.CallableRef]callableCompatibility)
	for _, ref := range refs {
		callable, ok := p.callables[ref]
		if !ok {
			continue
		}

		switch {
		case callable.Function != nil:
			fn := *callable.Function
			callableRef, ok := plan.CallableRefFromFunctionDecl(fn, plan.CallableSourceUser)
			if !ok {
				continue
			}
			compatibility := assessFunctionCompatibility(sourceType, targetType, fn)
			if compatibility.Compatible() {
				candidates[callableRef] = compatibility
			}
		case callable.Method != nil:
			method := *callable.Method
			callableRef, ok := plan.CallableRefFromMethod(method, plan.CallableSourceUser)
			if !ok {
				continue
			}
			compatibility := assessMethodCompatibility(sourceType, targetType, method)
			if compatibility.Compatible() {
				candidates[callableRef] = compatibility
			}
		}
	}

	callable, compatibility, ok := bestCallableCandidate(candidates)
	if !ok {
		return plan.CallableRef{}, callableCompatibility{}, false
	}
	return callable, compatibility, true
}

func (p *Planner) planHigherOrderFunctionCallable(
	sourceType, targetType types.Type,
	path string,
	typeParams typeParamScope,
	optionality spec.Optionality,
	conversion spec.ConversionsPolicy,
	source plan.CallableSource,
	callables []spec.TieredCallables,
) (plan.Value, bool) {
	candidates := make(map[plan.CallableRef]callableCompatibility)
	for _, fn := range p.registry.Candidates(sourceType, targetType, source) {
		callable, ok := plan.CallableRefFromFunctionDecl(fn, source)
		if !ok {
			continue
		}

		compatibility := assessHigherOrderFunctionCompatibility(sourceType, targetType, fn)
		if !compatibility.Compatible() {
			continue
		}

		candidates[callable] = compatibility
	}

	for _, candidate := range rankedCallableCandidates(candidates) {
		args, diagnostics, ok := p.planCallableArgs(
			candidate.Compatibility.MapperArgs,
			path,
			typeParams,
			optionality,
			conversion,
			callables,
		)
		if !ok {
			continue
		}

		callable := candidate.Callable
		return plan.Value{
			Operation:                   plan.OperationFunction,
			Source:                      sourceType,
			Target:                      targetType,
			Callable:                    &callable,
			CallableArgs:                args,
			CallableParameterAdaptation: callableInputAdaptation(candidate.Compatibility.Input),
			CallableResultAdaptation:    callableResultAdaptation(candidate.Compatibility.Result),
			Optionality:                 optionality,
			CanError: callable.ReturnsError ||
				callableInputCanError(candidate.Compatibility.Input, optionality) ||
				callableResultCanError(candidate.Compatibility.Result, optionality),
			Diagnostics: diagnostics,
		}, true
	}

	return plan.Value{}, false
}

func (p *Planner) planHigherOrderExplicitCallable(
	sourceType, targetType types.Type,
	path string,
	typeParams typeParamScope,
	optionality spec.Optionality,
	conversion spec.ConversionsPolicy,
	refs []spec.CallableRef,
	callables []spec.TieredCallables,
) (plan.Value, bool) {
	candidates := make(map[plan.CallableRef]callableCompatibility)
	for _, ref := range refs {
		callable, ok := p.callables[ref]
		if !ok || callable.Function == nil {
			continue
		}

		fn := *callable.Function
		callableRef, ok := plan.CallableRefFromFunctionDecl(fn, plan.CallableSourceUser)
		if !ok {
			continue
		}

		compatibility := assessHigherOrderFunctionCompatibility(sourceType, targetType, fn)
		if compatibility.Compatible() {
			candidates[callableRef] = compatibility
		}
	}

	for _, candidate := range rankedCallableCandidates(candidates) {
		args, diagnostics, ok := p.planCallableArgs(
			candidate.Compatibility.MapperArgs,
			path,
			typeParams,
			optionality,
			conversion,
			callables,
		)
		if !ok {
			continue
		}

		callable := candidate.Callable
		return plan.Value{
			Operation:                   plan.OperationFunction,
			Source:                      sourceType,
			Target:                      targetType,
			Callable:                    &callable,
			CallableArgs:                args,
			CallableParameterAdaptation: callableInputAdaptation(candidate.Compatibility.Input),
			CallableResultAdaptation:    callableResultAdaptation(candidate.Compatibility.Result),
			Optionality:                 optionality,
			CanError: callable.ReturnsError ||
				callableInputCanError(candidate.Compatibility.Input, optionality) ||
				callableResultCanError(candidate.Compatibility.Result, optionality),
			Diagnostics: diagnostics,
		}, true
	}

	return plan.Value{}, false
}

func (p *Planner) planCallableArgs(
	args []callableMapperArgCompatibility,
	path string,
	typeParams typeParamScope,
	optionality spec.Optionality,
	conversion spec.ConversionsPolicy,
	callables []spec.TieredCallables,
) ([]plan.CallableArg, []plan.Diagnostic, bool) {
	out := make([]plan.CallableArg, 0, len(args))
	var diagnostics []plan.Diagnostic

	for i, arg := range args {
		argPath := fmt.Sprintf("%s :: callable argument %d", path, i+1)
		mapping := p.planValue(
			arg.Source,
			arg.Target,
			argPath,
			typeParams,
			optionality,
			conversion,
			callables,
		)
		if mapping.Operation == plan.OperationUnsupported || hasFatalDiagnostics(mapping.Diagnostics) {
			return nil, nil, false
		}
		if mapping.CanError && !arg.ReturnsError {
			return nil, nil, false
		}

		out = append(out, plan.CallableArg{
			Mapping:      mapping,
			ReturnsError: arg.ReturnsError,
		})
		diagnostics = appendDiagnostic(diagnostics, mapping.Diagnostics...)
	}

	return out, diagnostics, true
}

// callableCompatibility is a type used to help prioritize compatible callables, allowing Morph to
// select, in theory, the most appropriate callable for a pair of types.
type callableCompatibility struct {
	Input        callableInputCompatibility
	Result       callableResultCompatibility
	ReturnsError bool

	// TypeBindings is populated when generic type parameters are bound while assessing the callable.
	TypeBindings map[string]types.Type
	MapperArgs   []callableMapperArgCompatibility
}

// Compatible returns whether this callable is compatible at all.
func (c callableCompatibility) Compatible() bool {
	return c.Input.Compatible() && c.Result.Compatible()
}

type callableMapperArgCompatibility struct {
	Source       types.Type
	Target       types.Type
	ReturnsError bool
}

type callableCandidate struct {
	Callable      plan.CallableRef
	Compatibility callableCompatibility
}

// callableInputCompatibility enumerates the possible levels of compatibility between a callable's
// input and the source type Morph is trying to map from.
type callableInputCompatibility uint

// Possible callableInputCompatibility values. These are ordered in priority order.
const (
	callableInputIncompatible callableInputCompatibility = iota
	callableInputExact
	callableInputAutoAddress // source T, input *T
	callableInputAutoDeref   // source *T, input T
	callableInputMax
)

func (c callableInputCompatibility) Compatible() bool {
	return c != callableInputIncompatible
}

// callableResultCompatibility enumerates the possible levels of compatibility between a callable's
// result and the target type Morph is trying to map to.
type callableResultCompatibility uint

// Possible callableResultCompatibility values. These are ordered in priority order.
const (
	callableResultIncompatible callableResultCompatibility = iota
	callableResultExact
	callableResultGeneric // e.g. input binds T=string, result T matches target string
	callableResultAutoAddress
	callableResultGenericAutoAddress
	callableResultAutoDeref
	callableResultGenericAutoDeref
	callableResultMax
)

func (c callableResultCompatibility) Compatible() bool {
	return c != callableResultIncompatible
}

func assessFunctionCompatibility(
	sourceType, targetType types.Type,
	fn types.FunctionDecl,
) callableCompatibility {
	returnsError, ok := plan.CallableResults(fn.Results)
	if !ok || fn.IsVariadic || len(fn.Params) != 1 {
		return callableCompatibility{}
	}

	return assessCallableCompatibility(sourceType, targetType, fn.Params[0].Type, fn.Results[0].Type, returnsError)
}

func assessHigherOrderFunctionCompatibility(
	sourceType, targetType types.Type,
	fn types.FunctionDecl,
) callableCompatibility {
	returnsError, ok := plan.CallableResults(fn.Results)
	if !ok || fn.IsVariadic || len(fn.Params) < 2 {
		return callableCompatibility{}
	}

	input := assessCallableInputCompatibility(sourceType, fn.Params[0].Type)
	if !input.Compatible() {
		return callableCompatibility{}
	}

	bindings, ok := callableInputTypeBindings(sourceType, fn.Params[0].Type)
	if !ok {
		return callableCompatibility{}
	}

	result, bindings := assessCallableResultCompatibilityWithBindings(targetType, fn.Results[0].Type, bindings)
	if !result.Compatible() {
		return callableCompatibility{}
	}

	args, ok := callableMapperArgCompatibilities(fn.Params[1:], bindings)
	if !ok {
		return callableCompatibility{}
	}

	return callableCompatibility{
		Input:        input,
		Result:       result,
		ReturnsError: returnsError,
		TypeBindings: bindings,
		MapperArgs:   args,
	}
}

func assessMethodCompatibility(
	sourceType, targetType types.Type,
	method types.Method,
) callableCompatibility {
	if !method.IsExported {
		return callableCompatibility{} // Zero is incompatible.
	}

	returnsError, ok := plan.CallableResults(method.Results)
	if !ok || method.Receiver == nil {
		return callableCompatibility{}
	}

	return assessCallableCompatibility(sourceType, targetType, method.Receiver.Type, method.Results[0].Type, returnsError)
}

func assessCallableCompatibility(
	sourceType, targetType types.Type,
	inputType, resultType types.Type,
	returnsError bool,
) callableCompatibility {
	input := assessCallableInputCompatibility(sourceType, inputType)
	if !input.Compatible() {
		return callableCompatibility{}
	}

	bindings, _ := callableInputTypeBindings(sourceType, inputType)
	result, bindings := assessCallableResultCompatibility(targetType, resultType, bindings)

	return callableCompatibility{
		Input:        input,
		Result:       result,
		ReturnsError: returnsError,
		TypeBindings: bindings,
	}
}

func assessCallableInputCompatibility(sourceType, inputType types.Type) callableInputCompatibility {
	sourceType = types.UnwrapAlias(sourceType)
	inputType = types.UnwrapAlias(inputType)

	if sameCallableInputType(sourceType, inputType) {
		return callableInputExact
	}

	sourceElem, sourcePointer := types.PointerElem(sourceType)
	inputElem, inputPointer := types.PointerElem(inputType)

	if !sourcePointer && inputPointer && sameCallableInputType(sourceType, inputElem) {
		return callableInputAutoAddress
	}

	if sourcePointer && !inputPointer && sameCallableInputType(sourceElem, inputType) {
		return callableInputAutoDeref
	}

	return callableInputIncompatible
}

func sameCallableInputType(sourceType, inputType types.Type) bool {
	if sameType(sourceType, inputType) {
		return true
	}

	_, ok := typeParamBindings(sourceType, inputType)
	return ok
}

func assessCallableResultCompatibility(
	targetType, resultType types.Type,
	bindings map[string]types.Type,
) (callableResultCompatibility, map[string]types.Type) {
	if result := callableResultCompatibilityFor(targetType, resultType, false); result.Compatible() {
		return result, bindings
	}

	if len(bindings) == 0 {
		return callableResultIncompatible, nil
	}

	generic := hasTypeParams(resultType)
	resultType = substituteTypeParams(resultType, bindings)
	if result := callableResultCompatibilityFor(targetType, resultType, generic); result.Compatible() {
		return result, bindings
	}

	return callableResultIncompatible, nil
}

func assessCallableResultCompatibilityWithBindings(
	targetType, resultType types.Type,
	bindings map[string]types.Type,
) (callableResultCompatibility, map[string]types.Type) {
	if result := callableResultCompatibilityFor(targetType, resultType, false); result.Compatible() {
		return result, bindings
	}

	resultType = substituteTypeParams(resultType, bindings)
	if result := callableResultCompatibilityFor(targetType, resultType, hasTypeParams(resultType)); result.Compatible() {
		return result, bindings
	}

	if result, out := bindCallableResultCompatibility(targetType, resultType, bindings); result.Compatible() {
		return result, out
	}

	return callableResultIncompatible, nil
}

func callableResultCompatibilityFor(
	targetType, resultType types.Type,
	generic bool,
) callableResultCompatibility {
	targetType = types.UnwrapAlias(targetType)
	resultType = types.UnwrapAlias(resultType)

	if sameType(targetType, resultType) {
		if generic {
			return callableResultGeneric
		}
		return callableResultExact
	}

	resultElem, resultPointer := types.PointerElem(resultType)
	targetElem, targetPointer := types.PointerElem(targetType)

	if !resultPointer && targetPointer && sameType(resultType, targetElem) {
		if generic {
			return callableResultGenericAutoAddress
		}
		return callableResultAutoAddress
	}

	if resultPointer && !targetPointer && sameType(resultElem, targetType) {
		if generic {
			return callableResultGenericAutoDeref
		}
		return callableResultAutoDeref
	}

	return callableResultIncompatible
}

func bindCallableResultCompatibility(
	targetType, resultType types.Type,
	bindings map[string]types.Type,
) (callableResultCompatibility, map[string]types.Type) {
	if result, out := bindCallableResultTypeParams(targetType, resultType, bindings, callableResultGeneric); result.Compatible() {
		return result, out
	}

	resultElem, resultPointer := types.PointerElem(resultType)
	targetElem, targetPointer := types.PointerElem(targetType)

	if !resultPointer && targetPointer {
		return bindCallableResultTypeParams(targetElem, resultType, bindings, callableResultGenericAutoAddress)
	}

	if resultPointer && !targetPointer {
		return bindCallableResultTypeParams(targetType, resultElem, bindings, callableResultGenericAutoDeref)
	}

	return callableResultIncompatible, nil
}

func bindCallableResultTypeParams(
	targetType, resultType types.Type,
	bindings map[string]types.Type,
	result callableResultCompatibility,
) (callableResultCompatibility, map[string]types.Type) {
	out := maps.Clone(bindings)
	if !bindTypeParams(out, targetType, resultType) {
		return callableResultIncompatible, nil
	}

	resultType = substituteTypeParams(resultType, out)
	if sameType(targetType, resultType) {
		return result, out
	}

	return callableResultIncompatible, nil
}

func callableMapperArgCompatibilities(
	params []types.Parameter,
	bindings map[string]types.Type,
) ([]callableMapperArgCompatibility, bool) {
	args := make([]callableMapperArgCompatibility, 0, len(params))
	for _, param := range params {
		signature := types.UnwrapAlias(param.Type)
		if signature.Kind != types.TypeKindSignature || signature.IsVariadic || len(signature.Params) != 1 {
			return nil, false
		}

		returnsError, ok := plan.CallableResults(signature.Results)
		if !ok {
			return nil, false
		}

		source := substituteTypeParams(signature.Params[0].Type, bindings)
		target := substituteTypeParams(signature.Results[0].Type, bindings)
		if hasTypeParams(source) || hasTypeParams(target) {
			return nil, false
		}

		args = append(args, callableMapperArgCompatibility{
			Source:       source,
			Target:       target,
			ReturnsError: returnsError,
		})
	}

	return args, true
}

func callableInputTypeBindings(sourceType, inputType types.Type) (map[string]types.Type, bool) {
	sourceType, _ = types.PointerElem(sourceType)
	inputType, _ = types.PointerElem(inputType)

	return typeParamBindings(sourceType, inputType)
}

func typeParamBindings(sourceType, templateType types.Type) (map[string]types.Type, bool) {
	bindings := make(map[string]types.Type)
	if !bindTypeParams(bindings, sourceType, templateType) {
		return nil, false
	}

	return bindings, true
}

func bindTypeParams(bindings map[string]types.Type, sourceType, templateType types.Type) bool {
	sourceType = types.UnwrapAlias(sourceType)
	templateType = types.UnwrapAlias(templateType)

	if templateType.Kind == types.TypeKindTypeParam {
		if existing, ok := bindings[templateType.Name]; ok {
			return sameType(existing, sourceType)
		}
		bindings[templateType.Name] = sourceType
		return true
	}

	if sourceType.Kind != templateType.Kind {
		return false
	}

	switch templateType.Kind {
	case types.TypeKindNamed:
		if sourceType.Name != templateType.Name ||
			sourceType.Package.ImportPath != templateType.Package.ImportPath ||
			len(sourceType.TypeArgs) != len(templateType.TypeArgs) {
			return false
		}
		for i, templateArg := range templateType.TypeArgs {
			if !bindTypeParams(bindings, sourceType.TypeArgs[i], templateArg) {
				return false
			}
		}
		return true
	case types.TypeKindPointer, types.TypeKindSlice, types.TypeKindArray:
		if templateType.Kind == types.TypeKindArray && sourceType.Len != templateType.Len {
			return false
		}
		if sourceType.Elem == nil || templateType.Elem == nil {
			return sourceType.Elem == nil && templateType.Elem == nil
		}
		return bindTypeParams(bindings, *sourceType.Elem, *templateType.Elem)
	case types.TypeKindMap:
		if sourceType.Key == nil || sourceType.Value == nil ||
			templateType.Key == nil || templateType.Value == nil {
			return sourceType.Key == nil && sourceType.Value == nil &&
				templateType.Key == nil && templateType.Value == nil
		}
		return bindTypeParams(bindings, *sourceType.Key, *templateType.Key) &&
			bindTypeParams(bindings, *sourceType.Value, *templateType.Value)
	default:
		return sameType(sourceType, templateType)
	}
}

// substituteTypeParams walks a type, substituting generic type parameters with the provided types
// which are bound for this field.
func substituteTypeParams(typ types.Type, bindings map[string]types.Type) types.Type {
	typ = types.UnwrapAlias(typ)

	if typ.Kind == types.TypeKindTypeParam {
		if bound, ok := bindings[typ.Name]; ok {
			return bound
		}
		return typ
	}

	if typ.Elem != nil {
		typ.Elem = new(substituteTypeParams(*typ.Elem, bindings))
		typ.String = ""
	}

	if typ.Key != nil {
		typ.Key = new(substituteTypeParams(*typ.Key, bindings))
		typ.String = ""
	}

	if typ.Value != nil {
		typ.Value = new(substituteTypeParams(*typ.Value, bindings))
		typ.String = ""
	}

	if len(typ.TypeArgs) > 0 {
		args := make([]types.Type, 0, len(typ.TypeArgs))
		for _, arg := range typ.TypeArgs {
			args = append(args, substituteTypeParams(arg, bindings))
		}
		typ.TypeArgs = args
	}

	return typ
}

func hasTypeParams(typ types.Type) bool {
	typ = types.UnwrapAlias(typ)
	if typ.Kind == types.TypeKindTypeParam {
		return true
	}

	if typ.Elem != nil && hasTypeParams(*typ.Elem) {
		return true
	}
	if typ.Key != nil && hasTypeParams(*typ.Key) {
		return true
	}
	if typ.Value != nil && hasTypeParams(*typ.Value) {
		return true
	}
	if slices.ContainsFunc(typ.TypeArgs, hasTypeParams) {
		return true
	}
	for _, param := range typ.Params {
		if hasTypeParams(param.Type) {
			return true
		}
	}
	for _, result := range typ.Results {
		if hasTypeParams(result.Type) {
			return true
		}
	}
	return false
}

func bestCallableCandidate(candidates map[plan.CallableRef]callableCompatibility) (plan.CallableRef, callableCompatibility, bool) {
	var best plan.CallableRef
	var bestCompatibility callableCompatibility
	var bestRank int
	var found bool

	for callable, compatibility := range candidates {
		rank, ok := callableCompatibilityRank(compatibility)
		if !ok {
			continue
		}

		// Lower rank is better, and callableLess is used to tie-break
		if !found || rank < bestRank || rank == bestRank && callableLess(best, callable) {
			best = callable
			bestCompatibility = compatibility
			bestRank = rank
			found = true
		}
	}

	return best, bestCompatibility, found
}

func rankedCallableCandidates(candidates map[plan.CallableRef]callableCompatibility) []callableCandidate {
	out := make([]callableCandidate, 0, len(candidates))
	for callable, compatibility := range candidates {
		if _, ok := callableCompatibilityRank(compatibility); !ok {
			continue
		}
		out = append(out, callableCandidate{
			Callable:      callable,
			Compatibility: compatibility,
		})
	}

	slices.SortFunc(out, func(a, b callableCandidate) int {
		aRank, _ := callableCompatibilityRank(a.Compatibility)
		bRank, _ := callableCompatibilityRank(b.Compatibility)
		if aRank != bRank {
			return aRank - bRank
		}
		switch {
		case callableLess(a.Callable, b.Callable):
			return 1
		case callableLess(b.Callable, a.Callable):
			return -1
		default:
			return 0
		}
	})

	return out
}

const (
	callableInputRankCount = int(callableInputMax - 1)
	callableErrorRankCount = 2
)

// callableCompatibilityRank calculates a rank used to prioritize which callable to select when
// multiple candidates are available for discovery. Exact result matches are preferred, within which
// callables that don't error are preferred, within which callables that have greater input
// compatibility are preferred. The lower the returned rank, the better.
func callableCompatibilityRank(c callableCompatibility) (int, bool) {
	if !c.Compatible() {
		return 0, false
	}

	resultRank := int(c.Result) - 1
	inputRank := int(c.Input) - 1

	errorRank := 0
	if c.ReturnsError {
		errorRank = 1
	}

	return (resultRank * callableErrorRankCount * callableInputRankCount) +
		(errorRank * callableInputRankCount) +
		inputRank, true
}

func (p *Planner) canConvert(
	source types.Type,
	target types.Type,
	conversion spec.ConversionsPolicy,
) bool {
	if canConvertBasicType(source, target) {
		return true
	}
	if !conversion.Enabled || !p.hasRegisteredConversion(source, target) {
		return false
	}
	return p.canUseRegisteredConversion(source, target)
}

func canConvertBasicType(source types.Type, target types.Type) bool {
	source = types.UnwrapAlias(source)
	target = types.UnwrapAlias(target)
	if source.Kind != types.TypeKindBasic || target.Kind != types.TypeKindBasic {
		return false
	}
	if sameType(source, target) {
		return true
	}
	return canConvertNumericLosslessly(source.Name, target.Name)
}

func (p *Planner) hasRegisteredConversion(source types.Type, target types.Type) bool {
	_, ok := p.conversions[spec.Conversion{
		Source: spec.TypeRefFromType(source),
		Target: spec.TypeRefFromType(target),
	}]
	return ok
}

func (p *Planner) canUseRegisteredConversion(source types.Type, target types.Type) bool {
	sourceUnderlying, sourceOK := p.basicUnderlying(source, map[plan.TypeRef]bool{})
	targetUnderlying, targetOK := p.basicUnderlying(target, map[plan.TypeRef]bool{})
	if !sourceOK || !targetOK {
		return false
	}
	if sameType(sourceUnderlying, targetUnderlying) {
		return true
	}
	return canConvertNumeric(sourceUnderlying.Name, targetUnderlying.Name)
}

// basicUnderlying recursively unwraps a type and/or its underlying type to find a basic type. Named
// types only reach this check after an exact registered conversion pair has matched.
func (p *Planner) basicUnderlying(typ types.Type, seen map[plan.TypeRef]bool) (types.Type, bool) {
	typ = types.UnwrapAlias(typ)
	switch typ.Kind {
	case types.TypeKindBasic:
		return typ, true
	case types.TypeKindNamed:
		// TODO: Does this need to be any more granular? I wouldn't expect so...
		ref := plan.TypeRefFromType(typ)
		if seen[ref] {
			return types.Type{}, false
		}
		seen[ref] = true

		typ, ok := p.resolveTypeDeclaration(typ)
		if !ok {
			return types.Type{}, false
		}
		return p.basicUnderlying(typ.Underlying, seen)
	default:
		return types.Type{}, false
	}
}

type numericKind int

const (
	numericInvalid numericKind = iota
	numericSigned
	numericUnsigned
	numericFloat
)

type numericInfo struct {
	kind       numericKind
	sourceBits int
	targetBits int
	precision  int
}

func canConvertNumericLosslessly(source string, target string) bool {
	sourceInfo, sourceOK := numericConversionInfoFor(source)
	targetInfo, targetOK := numericConversionInfoFor(target)
	if !sourceOK || !targetOK {
		return false
	}

	switch {
	case sourceInfo.kind == numericSigned && targetInfo.kind == numericSigned:
		return sourceInfo.sourceBits <= targetInfo.targetBits
	case sourceInfo.kind == numericUnsigned && targetInfo.kind == numericUnsigned:
		return sourceInfo.sourceBits <= targetInfo.targetBits
	case sourceInfo.kind == numericUnsigned && targetInfo.kind == numericSigned:
		return sourceInfo.sourceBits < targetInfo.targetBits
	case sourceInfo.kind == numericFloat && targetInfo.kind == numericFloat:
		return sourceInfo.precision < targetInfo.precision
	case sourceInfo.kind != numericFloat && targetInfo.kind == numericFloat:
		return sourceInfo.sourceBits <= targetInfo.precision
	default:
		return false
	}
}

func canConvertNumeric(source string, target string) bool {
	_, sourceOK := numericConversionInfoFor(source)
	_, targetOK := numericConversionInfoFor(target)
	return sourceOK && targetOK
}

func numericConversionInfoFor(name string) (numericInfo, bool) {
	switch name {
	case "int":
		return numericInfo{kind: numericSigned, sourceBits: 64, targetBits: 32}, true
	case "int8":
		return numericInfo{kind: numericSigned, sourceBits: 8, targetBits: 8}, true
	case "int16":
		return numericInfo{kind: numericSigned, sourceBits: 16, targetBits: 16}, true
	case "int32":
		return numericInfo{kind: numericSigned, sourceBits: 32, targetBits: 32}, true
	case "int64":
		return numericInfo{kind: numericSigned, sourceBits: 64, targetBits: 64}, true
	case "uint":
		return numericInfo{kind: numericUnsigned, sourceBits: 64, targetBits: 32}, true
	case "uint8":
		return numericInfo{kind: numericUnsigned, sourceBits: 8, targetBits: 8}, true
	case "uint16":
		return numericInfo{kind: numericUnsigned, sourceBits: 16, targetBits: 16}, true
	case "uint32":
		return numericInfo{kind: numericUnsigned, sourceBits: 32, targetBits: 32}, true
	case "uint64":
		return numericInfo{kind: numericUnsigned, sourceBits: 64, targetBits: 64}, true
	case "uintptr":
		return numericInfo{kind: numericUnsigned, sourceBits: 64, targetBits: 32}, true
	case "float32":
		return numericInfo{kind: numericFloat, sourceBits: 32, targetBits: 32, precision: 24}, true
	case "float64":
		return numericInfo{kind: numericFloat, sourceBits: 64, targetBits: 64, precision: 53}, true
	default:
		return numericInfo{}, false
	}
}

func (p *Planner) nestedFunctionName(
	source, target types.Type,
	sourceKey, targetKey string,
	callablesKey string,
) (string, error) {
	runHash := p.runHash
	if len(source.TypeArgs) > 0 || len(target.TypeArgs) > 0 || callablesKey != "" {
		typePairHash := stableTypePairKeyHash(sourceKey, targetKey)
		if runHash == "" {
			runHash = typePairHash
		} else {
			runHash += "_" + typePairHash
		}
	}

	input := NameInput{
		Source:    source,
		Target:    target,
		Signature: defaultMapperSignature,
		RunHash:   runHash,
	}

	return MapperName(input, defaultNestedMapperName)
}

func callableContextKey(callables []spec.TieredCallables) string {
	var sb strings.Builder
	for _, tier := range callables {
		if len(tier.Callables) == 0 {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("|")
		}
		fmt.Fprint(&sb, tier.Tier)
		sb.WriteString(":")
		for i, ref := range tier.Callables {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(ref.String())
		}
	}
	return sb.String()
}

// callableLess is used to compare two plan.CallableRef so Morph can produce stable discovery
// results given the same input.
func callableLess(a, b plan.CallableRef) bool {
	if a.Package.ImportPath != b.Package.ImportPath {
		return a.Package.ImportPath < b.Package.ImportPath
	}
	if a.SourceType.Key != b.SourceType.Key {
		return a.SourceType.Key < b.SourceType.Key
	}
	return a.Name < b.Name
}

func sameType(a, b types.Type) bool {
	return types.TypeKey(a) == types.TypeKey(b)
}

type typeParamScope map[string]types.TypeParam

func typeParamScopeFrom(params []types.TypeParam) typeParamScope {
	if len(params) == 0 {
		return nil
	}

	out := make(typeParamScope, len(params))
	for _, param := range params {
		out[param.Name] = param
	}
	return out
}

func scopedTypeRef(typ types.Type, scope typeParamScope) plan.TypeRef {
	ref := plan.TypeRefFromType(typ)
	ref.Key = scopedTypeKey(typ, scope)
	return ref
}

func scopedTypeKey(typ types.Type, scope typeParamScope) string {
	typ = types.UnwrapAlias(typ)
	if len(scope) == 0 {
		return types.TypeKey(typ)
	}

	switch typ.Kind {
	case types.TypeKindNamed, types.TypeKindAlias:
		name := typ.Name
		if typ.Package.ImportPath != "" {
			name = typ.Package.ImportPath + "." + name
		}
		if len(typ.TypeArgs) == 0 {
			return types.TypeKey(typ)
		}

		args := make([]string, 0, len(typ.TypeArgs))
		for _, arg := range typ.TypeArgs {
			args = append(args, scopedTypeKey(arg, scope))
		}
		return name + "[" + strings.Join(args, ", ") + "]"
	case types.TypeKindTypeParam:
		return scopedTypeParamKey(typ, scope)
	case types.TypeKindPointer:
		if typ.Elem != nil {
			return "*" + scopedTypeKey(*typ.Elem, scope)
		}
	case types.TypeKindSlice:
		if typ.Elem != nil {
			return "[]" + scopedTypeKey(*typ.Elem, scope)
		}
	case types.TypeKindArray:
		if typ.Elem != nil {
			return fmt.Sprintf("[%d]%s", typ.Len, scopedTypeKey(*typ.Elem, scope))
		}
	case types.TypeKindMap:
		if typ.Key != nil && typ.Value != nil {
			return fmt.Sprintf("map[%s]%s", scopedTypeKey(*typ.Key, scope), scopedTypeKey(*typ.Value, scope))
		}
	}

	return types.TypeKey(typ)
}

func scopedTypeParamKey(typ types.Type, scope typeParamScope) string {
	name := types.TypeKey(typ)
	param, ok := scope[typ.Name]
	if !ok {
		return name
	}

	constraint := types.TypeKey(param.Constraint)
	if constraint == "" {
		return name
	}

	return name + "{" + constraint + "}"
}

// isStructType returns true if the given type declaration looks like a struct type, i.e. its
// underlying type is a struct.
func isStructType(typ types.TypeDecl) bool {
	return types.UnwrapAlias(typ.Underlying).Kind == types.TypeKindStruct
}

func matchingField(
	needle types.Field,
	fields []types.Field,
	mapping map[string]spec.Field,
) (field types.Field, fieldSpec spec.Field, mapped bool, ok bool) {
	fieldsByFieldName := fieldsByName(fields)

	// Explicit field mappings are source -> target, so search for a source field whose mapped
	// target name matches this target field.
	if len(mapping) > 0 {
		for _, sourceField := range fields {
			fieldSpec, mapped := mapping[sourceField.Name]
			if mapped && structFieldTarget(sourceField.Name, fieldSpec) == needle.Name {
				return sourceField, fieldSpec, true, true
			}
		}
	}

	// Then we'll fall back to exact name matching.
	if field, ok = fieldsByFieldName[needle.Name]; ok {
		if fieldAvailableForTarget(field, needle.Name, mapping) {
			fieldSpec, mapped = mapping[field.Name]
			return field, fieldSpec, mapped, true
		}
	}

	// If that didn't work, we'll try case-insensitive matching.
	for _, field := range fields {
		if strings.EqualFold(field.Name, needle.Name) &&
			fieldAvailableForTarget(field, needle.Name, mapping) {
			fieldSpec, mapped = mapping[field.Name]
			return field, fieldSpec, mapped, true
		}
	}

	// NOTE: We don't try any more strategies here because they can be easily explicitly specified,
	// and any more advanced patterns would be too likely to return false positives, I think.

	// Otherwise, we failed...
	return field, spec.Field{}, false, false
}

func fieldAvailableForTarget(field types.Field, targetName string, mapping map[string]spec.Field) bool {
	fieldSpec, ok := mapping[field.Name]
	return !ok || structFieldTarget(field.Name, fieldSpec) == targetName
}

func structFieldTarget(sourceName string, field spec.Field) string {
	if field.Target != "" {
		return field.Target
	}
	return sourceName
}

func operationForCallable(callable plan.CallableRef) plan.Operation {
	if callable.Kind == plan.CallableKindMethod {
		return plan.OperationMethod
	}
	return plan.OperationFunction
}

func callableInputAdaptation(input callableInputCompatibility) plan.ValueAdaptation {
	switch input {
	case callableInputAutoAddress:
		return plan.ValueAdaptationAddress
	case callableInputAutoDeref:
		return plan.ValueAdaptationDeref
	default:
		return plan.ValueAdaptationNone
	}
}

func callableResultAdaptation(result callableResultCompatibility) plan.ValueAdaptation {
	switch result {
	case callableResultAutoAddress, callableResultGenericAutoAddress:
		return plan.ValueAdaptationAddress
	case callableResultAutoDeref, callableResultGenericAutoDeref:
		return plan.ValueAdaptationDeref
	default:
		return plan.ValueAdaptationNone
	}
}

func callableInputCanError(input callableInputCompatibility, optionality spec.Optionality) bool {
	return input == callableInputAutoDeref &&
		optionality.OnNilSourcePointer == spec.PointerOptionalityError
}

func callableResultCanError(result callableResultCompatibility, optionality spec.Optionality) bool {
	return callableResultAdaptation(result) == plan.ValueAdaptationDeref &&
		optionality.OnNilSourcePointer == spec.PointerOptionalityError
}

func pointerMappingCanError(sourcePointer, targetPointer bool, optionality spec.Optionality) bool {
	return sourcePointer && !targetPointer &&
		optionality.OnNilSourcePointer == spec.PointerOptionalityError
}

func plannableFields(typeDecl types.TypeDecl) []types.Field {
	return plannableFieldsForType(typeDecl, typeDecl.Type)
}

func plannableFieldsForType(typeDecl types.TypeDecl, typ types.Type) []types.Field {
	bindings := concreteTypeParamBindings(typeDecl, typ)
	out := make([]types.Field, 0, len(typeDecl.Fields))
	for _, field := range typeDecl.Fields {
		if !field.IsExported || field.IsEmbedded {
			// We only support regular, exported fields currently.
			continue
		}
		if len(bindings) > 0 {
			field.Type = substituteTypeParams(field.Type, bindings)
		}
		out = append(out, field)
	}

	slices.SortFunc(out, func(a, b types.Field) int {
		return strings.Compare(a.Name, b.Name)
	})

	return out
}

func concreteTypeParamBindings(typeDecl types.TypeDecl, typ types.Type) map[string]types.Type {
	typ = types.UnwrapAlias(typ)
	if len(typeDecl.Type.TypeParams) == 0 || len(typeDecl.Type.TypeParams) != len(typ.TypeArgs) {
		return nil
	}

	bindings := make(map[string]types.Type, len(typeDecl.Type.TypeParams))
	for i, param := range typeDecl.Type.TypeParams {
		bindings[param.Name] = typ.TypeArgs[i]
	}
	return bindings
}

func concreteTypeParams(
	typeDecl types.TypeDecl,
	typ types.Type,
	typeParams typeParamScope,
) []types.TypeParam {
	typ = types.UnwrapAlias(typ)
	if len(typ.TypeArgs) == 0 {
		return typeDecl.Type.TypeParams
	}

	paramsByName := make(map[string]types.TypeParam, len(typeDecl.Type.TypeParams))
	for _, param := range typeDecl.Type.TypeParams {
		paramsByName[param.Name] = param
	}

	var out []types.TypeParam
	seen := make(map[string]struct{})
	for _, arg := range typ.TypeArgs {
		collectTypeParams(&out, seen, paramsByName, typeParams, arg)
	}
	return out
}

func collectTypeParams(
	out *[]types.TypeParam,
	seen map[string]struct{},
	paramsByName map[string]types.TypeParam,
	typeParams typeParamScope,
	typ types.Type,
) {
	typ = types.UnwrapAlias(typ)
	if typ.Kind == types.TypeKindTypeParam {
		if _, ok := seen[typ.Name]; ok {
			return
		}
		seen[typ.Name] = struct{}{}

		param, ok := typeParams[typ.Name]
		if !ok {
			param, ok = paramsByName[typ.Name]
		}
		if !ok {
			param = types.TypeParam{Name: typ.Name}
		}
		*out = append(*out, param)
		return
	}

	if typ.Elem != nil {
		collectTypeParams(out, seen, paramsByName, typeParams, *typ.Elem)
	}
	if typ.Key != nil {
		collectTypeParams(out, seen, paramsByName, typeParams, *typ.Key)
	}
	if typ.Value != nil {
		collectTypeParams(out, seen, paramsByName, typeParams, *typ.Value)
	}
	for _, arg := range typ.TypeArgs {
		collectTypeParams(out, seen, paramsByName, typeParams, arg)
	}
}

func plannableFieldsByName(typeDecl types.TypeDecl) map[string]types.Field {
	return fieldsByName(plannableFields(typeDecl))
}

func fieldsByName(fields []types.Field) map[string]types.Field {
	out := make(map[string]types.Field, len(fields))
	for _, field := range fields {
		out[field.Name] = field
	}
	return out
}

func unsupportedMapping(source, target types.Type, path string, message string) plan.Value {
	return plan.Value{
		Operation: plan.OperationUnsupported,
		Source:    source,
		Target:    target,
		Diagnostics: []plan.Diagnostic{{
			Path:    path,
			Message: fmt.Sprintf("cannot map %s to %s: %s", source.String, target.String, message),
		}},
	}
}

func hasFatalDiagnostics(diagnostics []plan.Diagnostic) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Level == plan.DiagnosticLevelFatal {
			return true
		}
	}
	return false
}
