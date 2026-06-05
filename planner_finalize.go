package morph

import (
	"fmt"

	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/types"
)

type callableBan struct {
	Path      string
	SourceKey string
	TargetKey string
	Callable  plan.CallableRef
}

func callableBanFor(path string, source, target types.Type, callable plan.CallableRef) callableBan {
	return callableBan{
		Path:      path,
		SourceKey: types.TypeKey(source),
		TargetKey: types.TypeKey(target),
		Callable:  callable,
	}
}

type callableErrabilityFailure struct {
	Ban        callableBan
	Value      *plan.Value
	Type       *plan.Type
	Diagnostic plan.Diagnostic
}

func (p *attemptPlanner) isCallableBanned(path string, source, target types.Type, callable plan.CallableRef) bool {
	_, banned := p.callableBans[callableBanFor(path, source, target, callable)]
	return banned
}

func (p *attemptPlanner) finalizePlanErrability(outputGroups []plan.OutputGroup) {
	for {
		var changed bool
		seen := make(map[*plan.Type]struct{})
		for _, outputGroup := range outputGroups {
			for _, root := range outputGroup.Roots {
				if finalizeTypeErrability(root, seen) {
					changed = true
				}
			}
		}
		if !changed {
			return
		}
	}
}

func finalizeTypeErrability(typ *plan.Type, seen map[*plan.Type]struct{}) bool {
	if typ == nil {
		return false
	}
	if _, ok := seen[typ]; ok {
		return false
	}
	seen[typ] = struct{}{}

	var canError, changed bool

	switch {
	case typ.EnumPlan != nil:
		// We can trust enums, because there's no opportunity for callables.
		canError = typ.EnumPlan != nil && typ.CanError
	case typ.StructPlan != nil:
		// Structs can have callables, so we need to check each field.
		for i := range typ.StructPlan.Fields {
			field := &typ.StructPlan.Fields[i]
			if finalizeValueErrability(&field.Mapping, seen) {
				changed = true
			}
			canError = canError || field.Mapping.CanError
		}
	}

	if typ.CanError != canError {
		typ.CanError = canError
		changed = true
	}

	return changed
}

func finalizeValueErrability(value *plan.Value, seen map[*plan.Type]struct{}) bool {
	if value == nil {
		return false
	}

	var changed bool
	if value.Plan != nil && finalizeTypeErrability(value.Plan, seen) {
		changed = true
	}
	if value.Elem != nil && finalizeValueErrability(value.Elem, seen) {
		changed = true
	}
	if value.Key != nil && finalizeValueErrability(value.Key, seen) {
		changed = true
	}
	if value.Value != nil && finalizeValueErrability(value.Value, seen) {
		changed = true
	}
	for i := range value.CallableArgs {
		if finalizeValueErrability(&value.CallableArgs[i].Mapping, seen) {
			changed = true
		}
	}

	canError := adaptationsCanError(value.SourceAdaptations, value.Optionality) ||
		adaptationsCanError(value.TargetAdaptations, value.Optionality)

	switch value.Operation {
	case plan.OperationFunction, plan.OperationMethod:
		canError = canError || value.Callable != nil && value.Callable.ReturnsError
	case plan.OperationStruct, plan.OperationEnum:
		canError = canError || value.Plan != nil && value.Plan.CanError
	case plan.OperationSlice, plan.OperationArray, plan.OperationMap, plan.OperationUnsupported:
		canError = canError || valueChildrenCanError(value)
	}

	if value.CanError != canError {
		value.CanError = canError
		changed = true
	}

	return changed
}

func valueChildrenCanError(value *plan.Value) bool {
	return value.Elem != nil && value.Elem.CanError ||
		value.Key != nil && value.Key.CanError ||
		value.Value != nil && value.Value.CanError
}

func (p *attemptPlanner) validateCallableErrability(outputGroups []plan.OutputGroup) *callableBan {
	failures := collectCallableErrabilityFailures(outputGroups)
	for _, failure := range failures {
		if _, banned := p.callableBans[failure.Ban]; !banned {
			return new(failure.Ban)
		}
	}

	for _, failure := range failures {
		failure.Value.Diagnostics = appendDiagnostic(failure.Value.Diagnostics, failure.Diagnostic)
		if failure.Type != nil {
			failure.Type.Diagnostics = appendDiagnostic(failure.Type.Diagnostics, failure.Diagnostic)
		}
		p.diagnostics = appendDiagnostic(p.diagnostics, failure.Diagnostic)
	}
	return nil
}

func collectCallableErrabilityFailures(outputGroups []plan.OutputGroup) []callableErrabilityFailure {
	var failures []callableErrabilityFailure
	seen := make(map[*plan.Type]struct{})
	for _, outputGroup := range outputGroups {
		for _, root := range outputGroup.Roots {
			failures = append(failures, collectTypeCallableErrabilityFailures(root, seen)...)
		}
	}
	return failures
}

func collectTypeCallableErrabilityFailures(typ *plan.Type, seen map[*plan.Type]struct{}) []callableErrabilityFailure {
	if typ == nil {
		return nil
	}
	if _, ok := seen[typ]; ok {
		return nil
	}

	seen[typ] = struct{}{}

	var failures []callableErrabilityFailure
	if typ.StructPlan == nil {
		return failures
	}

	for i := range typ.StructPlan.Fields {
		field := &typ.StructPlan.Fields[i]
		path := plan.FieldPath(typ.SourceType, typ.TargetType, field.SourceField, field.TargetField)
		failures = append(failures, collectValueCallableErrabilityFailures(
			&field.Mapping,
			typ,
			path,
			seen,
		)...)
	}

	return failures
}

func collectValueCallableErrabilityFailures(
	value *plan.Value,
	typ *plan.Type,
	path string,
	seen map[*plan.Type]struct{},
) []callableErrabilityFailure {
	if value == nil {
		return nil
	}

	var failures []callableErrabilityFailure
	if value.Callable != nil {
		for i := range value.CallableArgs {
			arg := &value.CallableArgs[i]
			argPath := callableArgPath(path, i)
			if arg.Mapping.CanError && !arg.ReturnsError {
				failures = append(failures, callableErrabilityFailure{
					Ban: callableBanFor(
						path,
						value.Source,
						value.Target,
						*value.Callable,
					),
					Value:      value,
					Type:       typ,
					Diagnostic: callableErrabilityDiagnostic(argPath, i, arg.Mapping),
				})
			}
			failures = append(failures, collectValueCallableErrabilityFailures(
				&arg.Mapping,
				typ,
				argPath,
				seen,
			)...)
		}
	}

	if value.Elem != nil {
		failures = append(failures, collectValueCallableErrabilityFailures(
			value.Elem,
			typ,
			path+"[]",
			seen,
		)...)
	}

	if value.Key != nil {
		failures = append(failures, collectValueCallableErrabilityFailures(
			value.Key,
			typ,
			path+"[key]",
			seen,
		)...)
	}

	if value.Value != nil {
		failures = append(failures, collectValueCallableErrabilityFailures(
			value.Value,
			typ,
			path+"[value]",
			seen,
		)...)
	}

	if value.Plan != nil {
		failures = append(failures, collectTypeCallableErrabilityFailures(value.Plan, seen)...)
	}

	return failures
}

func callableArgPath(path string, index int) string {
	return fmt.Sprintf("%s :: callable argument %d", path, index+1)
}

func callableErrabilityDiagnostic(path string, index int, mapping plan.Value) plan.Diagnostic {
	return plan.Diagnostic{
		Level: plan.DiagnosticLevelFatal,
		Path:  path,
		Message: fmt.Sprintf(
			"callable argument %d maps %s to %s and can return an error, but the higher-order callable argument does not return error",
			index+1,
			types.TypeKey(mapping.Source),
			types.TypeKey(mapping.Target),
		),
	}
}
