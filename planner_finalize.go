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

type finalPlanValidation struct {
	ImportSites      []importRequirementSite
	VisibilitySites  []visibilityRequirementSite
	CallableFailures []callableErrabilityFailure
}

func (p *attemptPlanner) isCallableBanned(path string, source, target types.Type, callable plan.CallableRef) bool {
	_, banned := p.callableBans[callableBanFor(path, source, target, callable)]
	return banned
}

func (p *attemptPlanner) finalizePlanErrability(outputGroups []plan.OutputGroup) {
	for {
		var changed bool
		walkOutputGroups(outputGroups, walkCallbacks{
			ValuePost: func(ctx walkContext) {
				if finalizeValueErrability(ctx.Value) {
					changed = true
				}
			},
			TypePost: func(ctx walkContext) {
				if finalizeTypeErrability(ctx.Type) {
					changed = true
				}
			},
		})
		if !changed {
			return
		}
	}
}

func finalizeTypeErrability(typ *plan.Type) bool {
	if typ == nil {
		return false
	}

	var canError, changed bool

	switch {
	case typ.EnumPlan != nil:
		// We can trust enums, because there's no opportunity for callables.
		canError = typ.EnumPlan != nil && typ.CanError
	case typ.StructPlan != nil:
		// Structs can have callables, so we need to check each field.
		for i := range typ.StructPlan.Fields {
			field := &typ.StructPlan.Fields[i]
			canError = canError || field.Mapping.CanError
		}
	}

	if typ.CanError != canError {
		typ.CanError = canError
		changed = true
	}

	return changed
}

func finalizeValueErrability(value *plan.Value) bool {
	if value == nil {
		return false
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
		return true
	}

	return false
}

func valueChildrenCanError(value *plan.Value) bool {
	return value.Elem != nil && value.Elem.CanError ||
		value.Key != nil && value.Key.CanError ||
		value.Value != nil && value.Value.CanError
}

func (p *attemptPlanner) validateFinalPlan(outputGroups []plan.OutputGroup) *callableBan {
	validation := collectFinalPlanValidation(outputGroups)
	for _, failure := range validation.CallableFailures {
		if _, banned := p.callableBans[failure.Ban]; !banned {
			ban := failure.Ban
			return &ban
		}
	}

	p.validateImportRequirements(validation.ImportSites)
	p.validateVisibilityRequirements(validation.VisibilitySites)

	for _, failure := range validation.CallableFailures {
		failure.Value.Diagnostics = appendDiagnostic(failure.Value.Diagnostics, failure.Diagnostic)
		if failure.Type != nil {
			failure.Type.Diagnostics = appendDiagnostic(failure.Type.Diagnostics, failure.Diagnostic)
		}
		p.diagnostics = appendDiagnostic(p.diagnostics, failure.Diagnostic)
	}
	return nil
}

func collectFinalPlanValidation(outputGroups []plan.OutputGroup) finalPlanValidation {
	var validation finalPlanValidation
	walkOutputGroups(outputGroups, walkCallbacks{
		TypePre: func(ctx walkContext) {
			validation.ImportSites = append(validation.ImportSites, typeSignatureImportSites(ctx)...)
			validation.VisibilitySites = append(validation.VisibilitySites, typeSignatureVisibilitySites(ctx)...)
		},
		ValuePre: func(ctx walkContext) {
			validation.ImportSites = append(validation.ImportSites, valueImportSites(ctx)...)
			validation.VisibilitySites = append(validation.VisibilitySites, valueVisibilitySites(ctx)...)
			validation.CallableFailures = append(validation.CallableFailures, callableErrabilityFailures(ctx)...)
		},
	})
	validation.ImportSites = dedupeImportRequirementSites(validation.ImportSites)
	validation.VisibilitySites = dedupeVisibilityRequirementSites(validation.VisibilitySites)
	return validation
}

func callableErrabilityFailures(ctx walkContext) []callableErrabilityFailure {
	value := ctx.Value
	if value == nil || value.Callable == nil {
		return nil
	}

	var failures []callableErrabilityFailure
	for i := range value.CallableArgs {
		arg := &value.CallableArgs[i]
		if !arg.Mapping.CanError || arg.ReturnsError {
			continue
		}

		failures = append(failures, callableErrabilityFailure{
			Ban: callableBanFor(
				ctx.Path,
				value.Source,
				value.Target,
				*value.Callable,
			),
			Value:      value,
			Type:       ctx.Owner,
			Diagnostic: callableErrabilityDiagnostic(callableArgPath(ctx.Path, i), i, arg.Mapping),
		})
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
