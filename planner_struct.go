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

		fieldPath := plan.FieldPath(typ.SourceType, typ.TargetType, sourceField)

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

	if sourceType.Kind == types.TypeKindPointer || targetType.Kind == types.TypeKindPointer {
		return p.planPointerMapping(sourceType, targetType, path)
	}

	// If there's a user-supplied, explicit function to use for this pair of types, prefer it.
	if fn, ok := p.discoverFunctionCallable(sourceType, targetType, plan.CallableSourceUser); ok {
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

	// If we discovered a suitable function to use for this pair of types, use that.
	if fn, ok := p.discoverFunctionCallable(sourceType, targetType, plan.CallableSourceDiscovered); ok {
		return plan.Value{
			Operation: operationForCallable(fn),
			Source:    sourceType,
			Target:    targetType,
			Callable:  new(fn),
			CanError:  fn.ReturnsError,
		}
	}

	if callable, ok := p.discoverMethodCallable(sourceType, targetType); ok {
		return plan.Value{
			Operation: operationForCallable(callable),
			Source:    sourceType,
			Target:    targetType,
			Callable:  new(callable),
			CanError:  callable.ReturnsError,
		}
	}

	switch {
	case sourceType.Kind == types.TypeKindSlice && targetType.Kind == types.TypeKindSlice:
		elemPlan := p.planValue(*sourceType.Elem, *targetType.Elem, path+"[]")

		operation := plan.OperationSlice
		if len(elemPlan.Diagnostics) > 0 {
			operation = plan.OperationUnsupported
		}

		return plan.Value{
			Operation:   operation,
			Source:      sourceType,
			Target:      targetType,
			Elem:        &elemPlan,
			CanError:    elemPlan.CanError,
			Diagnostics: elemPlan.Diagnostics,
		}

	case sourceType.Kind == types.TypeKindArray && targetType.Kind == types.TypeKindArray:
		if sourceType.Len != targetType.Len {
			return unsupportedMapping(sourceType, targetType, path, "array lengths differ")
		}

		elemPlan := p.planValue(*sourceType.Elem, *targetType.Elem, path+"[]")

		operation := plan.OperationMap
		if len(elemPlan.Diagnostics) > 0 {
			operation = plan.OperationUnsupported
		}

		return plan.Value{
			Operation:   operation,
			Source:      sourceType,
			Target:      targetType,
			Elem:        &elemPlan,
			CanError:    elemPlan.CanError,
			Diagnostics: elemPlan.Diagnostics,
		}

	case sourceType.Kind == types.TypeKindMap && targetType.Kind == types.TypeKindMap:
		key := p.planValue(*sourceType.Key, *targetType.Key, path+"[key]")
		value := p.planValue(*sourceType.Value, *targetType.Value, path+"[value]")

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
			CanError:    key.CanError || value.CanError,
			Diagnostics: diagnostics,
		}
	}

	if nested, ok := p.planNestedStruct(sourceType, targetType, path); ok {
		return nested
	}

	if sameType(sourceType, targetType) {
		return plan.Value{
			Operation: plan.OperationAssign,
			Source:    sourceType,
			Target:    targetType,
		}
	}

	if p.canConvertByBasicType(sourceType, targetType) {
		return plan.Value{
			Operation: plan.OperationConvert,
			Source:    sourceType,
			Target:    targetType,
		}
	}

	return unsupportedMapping(sourceType, targetType, path, "unable to determine mapping strategy")
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

func (p *Planner) planNestedStruct(source, target types.Type, path string) (plan.Value, bool) {
	sourceDecl, sourceOK := p.resolveStructType(source)
	targetDecl, targetOK := p.resolveStructType(target)
	if !sourceOK || !targetOK {
		return plan.Value{}, false
	}

	if sameType(source, target) {
		return plan.Value{}, false
	}

	// Nested structs use the globally configured defaults.
	defaultTypes := typesDefaultsWithDefaults(p.spec.Defaults.Packages.Types, defaultTypesDefaults)

	var enumSpec spec.Enum
	if defaultTypes.Enum != nil {
		// If this is nil, something is quite wrong...
		enumSpec = *enumWithDefaults(nil, defaultTypes.Enum)
	}

	nested := plan.Type{
		Source:     plan.TypeRefFromTypeDecl(sourceDecl),
		Target:     plan.TypeRefFromTypeDecl(targetDecl),
		SourceDecl: sourceDecl,
		TargetDecl: targetDecl,
		SourceType: source,
		TargetType: target,
		TypeParams: sourceDecl.Type.TypeParams,
		Signature:  defaultMapperSignature,
		EnumSpec:   enumSpec,
		// We can't set structSpec in this case, because it's just field mapping currently. To have
		// field mapping this type pair would have to be defined explicitly.
	}

	var err error
	nested.FunctionName, err = p.nestedFunctionName(sourceDecl, targetDecl)
	if err != nil {
		return unsupportedMapping(source, target, path, fmt.Sprintf("failed to generated nested function name: %v", err)), false
	}

	p.planType(&nested)

	return plan.Value{
		Operation:   plan.OperationStruct,
		Source:      source,
		Target:      target,
		Plan:        &nested,
		CanError:    nested.CanError,
		Diagnostics: nested.Diagnostics,
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

func (p *Planner) discoverFunctionCallable(
	sourceType, targetType types.Type,
	source plan.CallableSource,
) (plan.CallableRef, bool) {
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

	return bestCallableCandidate(candidates)
}

func (p *Planner) discoverMethodCallable(sourceType, targetType types.Type) (plan.CallableRef, bool) {
	methodTypeDecl, _, ok := p.methodTypeDecl(sourceType)
	if !ok {
		return plan.CallableRef{}, false
	}

	candidates := make(map[plan.CallableRef]callableCompatibility)
	for _, method := range methodTypeDecl.Methods {
		callable, ok := plan.CallableRefFromMethod(method, plan.CallableSourceDiscovered)
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
		compatibility := assessMethodCompatibility(sourceType, targetType, method)
		if !compatibility.Compatible() {
			continue
		}

		candidates[callable] = compatibility
	}

	return bestCallableCandidate(candidates)
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

// callableCompatibility is a type used to help prioritize compatible callables, allowing Morph to
// select, in theory, the most appropriate callable for a pair of types.
type callableCompatibility struct {
	Input        callableInputCompatibility
	Result       callableResultCompatibility
	ReturnsError bool

	// Only set when Result is callableResultGeneric.
	TypeBindings map[string]types.Type
}

// Compatible returns whether this callable is compatible at all.
func (c callableCompatibility) Compatible() bool {
	return c.Input.Compatible() && c.Result.Compatible()
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
	if sameType(targetType, resultType) {
		return callableResultExact, nil
	}

	if len(bindings) == 0 {
		return callableResultIncompatible, nil
	}

	resultType = substituteTypeParams(resultType, bindings)
	if sameType(targetType, resultType) {
		return callableResultGeneric, bindings
	}

	return callableResultIncompatible, nil
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
		elem := substituteTypeParams(*typ.Elem, bindings)
		typ.Elem = &elem
		typ.String = ""
	}

	if typ.Key != nil {
		key := substituteTypeParams(*typ.Key, bindings)
		typ.Key = &key
		typ.String = ""
	}

	if typ.Value != nil {
		value := substituteTypeParams(*typ.Value, bindings)
		typ.Value = &value
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

func bestCallableCandidate(candidates map[plan.CallableRef]callableCompatibility) (plan.CallableRef, bool) {
	var best plan.CallableRef
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
			bestRank = rank
			found = true
		}
	}

	return best, found
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

func (p *Planner) canConvertByBasicType(source types.Type, target types.Type) bool {
	sourceUnderlying, sourceOK := p.basicUnderlying(source, map[plan.TypeRef]bool{})
	targetUnderlying, targetOK := p.basicUnderlying(target, map[plan.TypeRef]bool{})
	if !sourceOK || !targetOK {
		return false
	}
	if sameType(sourceUnderlying, targetUnderlying) {
		return true
	}
	return canConvertNumericLosslessly(sourceUnderlying.Name, targetUnderlying.Name)
}

// basicUnderlying recursively unwraps a type and/or it's underlying type to find a basic type. If
// the underlying type is basic, we can (probably) try to convert it.
// TODO: This may be too permissive, maybe it should be just basic types and aliases of them?
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
	sourceInfo, sourceOK := maxSafeNumericConversionBits(source)
	targetInfo, targetOK := maxSafeNumericConversionBits(target)
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

func maxSafeNumericConversionBits(name string) (numericInfo, bool) {
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

func (p *Planner) nestedFunctionName(sourceDecl, targetDecl types.TypeDecl) (string, error) {
	input := NameInput{
		Source:     sourceDecl.Type,
		Target:     targetDecl.Type,
		TypeParams: sourceDecl.Type.TypeParams,
		Signature:  mapperSignatureWithDefaults(spec.MapperSignature{}, defaultMapperSignature),
		RunHash:    p.runHash,
	}

	return MapperName(input, defaultNestedMapperName)
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
