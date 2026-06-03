package morph

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"text/template"

	"github.com/danielgtaylor/casing"
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
)

func (p *Planner) planEnum(typ *plan.Type) {
	values, diagnostics := p.planEnumValues(typ)

	typ.EnumPlan = &plan.Enum{
		FailureMode: typ.EnumSpec.FailureMode,
		Values:      values,
	}

	typ.CanError = typ.EnumPlan.FailureMode == spec.EnumFailureModeError
	typ.Diagnostics = appendDiagnostic(typ.Diagnostics, diagnostics...)
}

func (p *Planner) planEnumValues(typ *plan.Type) ([]plan.EnumValue, []plan.Diagnostic) {
	var values []plan.EnumValue
	var diagnostics []plan.Diagnostic

	sourceConstants := collectExportedConstants(typ.SourceDecl.Constants)
	targetConstants := collectExportedConstants(typ.TargetDecl.Constants)

	sourcePattern := typ.EnumSpec.Patterns.Source
	targetPattern := typ.EnumSpec.Patterns.Target

	targetsByNormalizedName, ambiguousTargets, targetDiagnostics := constantsByNormalizedName(targetConstants, targetPattern)
	diagnostics = appendDiagnostic(diagnostics, targetDiagnostics...)

	sourceNames := slices.Collect(maps.Keys(sourceConstants))
	slices.Sort(sourceNames)

	seenSources := make(map[string]struct{})

	// Look for explicit mappings first:
	for sourceName, targetName := range typ.EnumSpec.Values {
		sourceConstant, ok := sourceConstants[sourceName]
		if !ok {
			diagnostics = appendDiagnostic(diagnostics, plan.Diagnostic{
				Level:   plan.DiagnosticLevelWarning,
				Path:    plan.TypesPath(typ.SourceType, typ.TargetType),
				Message: fmt.Sprintf("source enum value %q not found", sourceName),
			})
			continue
		}

		targetConstant, ok := targetConstants[targetName]
		if !ok {
			diagnostics = appendDiagnostic(diagnostics, plan.Diagnostic{
				Level:   plan.DiagnosticLevelWarning,
				Path:    plan.TypesPath(typ.SourceType, typ.TargetType),
				Message: fmt.Sprintf("target enum value %q not found for source value %q", targetName, sourceName),
			})
			continue
		}

		values = append(values, plan.EnumValue{
			Source: sourceConstant,
			Target: targetConstant,
		})

		seenSources[sourceName] = struct{}{}
	}

	for _, sourceConstant := range sourceConstants {
		if _, ok := seenSources[sourceConstant.Name]; ok {
			continue
		}

		normalizedSourceName, err := normalizeEnumConstant(sourceConstant, sourcePattern)
		if err != nil {
			diagnostics = appendDiagnostic(diagnostics, plan.Diagnostic{
				Level:   plan.DiagnosticLevelFatal,
				Path:    plan.TypesPath(typ.SourceType, typ.TargetType),
				Message: fmt.Sprintf("failed to normalize source enum constant name: %v", err),
			})
			continue
		}

		if ambiguous := ambiguousTargets[normalizedSourceName]; len(ambiguous) > 0 {
			diagnostics = appendDiagnostic(diagnostics, plan.Diagnostic{
				Level: plan.DiagnosticLevelFatal,
				Path:  plan.TypesPath(typ.SourceType, typ.TargetType),
				Message: fmt.Sprintf(
					"enum value %q matches ambiguous normalized target value %q on %q (%s); configure enum value mapping explicitly",
					sourceConstant.Name,
					normalizedSourceName,
					typ.TargetType.Name,
					strings.Join(ambiguous, ","),
				),
			})
			continue
		}

		targetConstant, ok := targetsByNormalizedName[normalizedSourceName]
		if !ok {
			diagnostics = appendDiagnostic(diagnostics, plan.Diagnostic{
				Level: plan.DiagnosticLevelFatal,
				Path:  plan.TypesPath(typ.SourceType, typ.TargetType),
				Message: fmt.Sprintf(
					"unexpected missing target constant for normalized value %q",
					normalizedSourceName,
				),
			})
			continue
		}

		values = append(values, plan.EnumValue{
			Source: sourceConstant,
			Target: targetConstant,
		})

		seenSources[normalizedSourceName] = struct{}{}
	}

	slices.SortFunc(values, func(a, b plan.EnumValue) int {
		return cmp.Compare(a.Source.Name, b.Source.Name)
	})

	return values, diagnostics
}

// constantsByNormalizedName returns a map from normalized name (e.g. ExampleFoo would be keyed on
// "foo"), to that normalized name's corresponding constant declaration. Additionally, this function
// returns a map from normalized name to input names, highlighting constant names which have yielded
// the same normalized name.
func constantsByNormalizedName(
	constants map[string]types.ConstantDecl,
	pattern string,
) (normalized map[string]types.ConstantDecl, ambiguous map[string][]string, diagnostics []plan.Diagnostic) {
	normalized = make(map[string]types.ConstantDecl)
	ambiguous = make(map[string][]string)

	for constantName, constantDecl := range constants {
		normName, err := normalizeEnumConstant(constantDecl, pattern)
		if err != nil {
			diagnostics = appendDiagnostic(diagnostics, plan.Diagnostic{
				Level:   plan.DiagnosticLevelWarning,
				Path:    types.TypeKey(constantDecl.Type) + " :: " + constantName,
				Message: fmt.Sprintf("failed to normalize enum constant name: %v", err),
			})
			continue
		}
		if normName == "" {
			diagnostics = appendDiagnostic(diagnostics, plan.Diagnostic{
				Level:   plan.DiagnosticLevelWarning,
				Path:    types.TypeKey(constantDecl.Type) + " :: " + constantName,
				Message: "enum value name normalization resulted in empty name, skipping",
			})
			continue
		}

		if existing, exists := normalized[normName]; exists {
			ambiguous[normName] = append(ambiguous[normName], existing.Name, constantName)
			delete(normalized, normName)
			continue
		}

		if names, exists := ambiguous[normName]; exists {
			ambiguous[normName] = append(names, constantName)
			continue
		}

		normalized[normName] = constantDecl
	}

	for normName, names := range ambiguous {
		slices.Sort(names)
		ambiguous[normName] = names
	}

	return normalized, ambiguous, diagnostics
}

func normalizeEnumConstant(constantDecl types.ConstantDecl, pattern string) (string, error) {
	if pattern != "" {
		return normalizeEnumConstantPattern(constantDecl, pattern)
	}

	return normalizeEnumConstantAuto(constantDecl), nil
}

func normalizeEnumConstantAuto(constant types.ConstantDecl) string {
	nameParts := casing.MergeNumbers(casing.Split(constant.Name))
	typeParts := casing.MergeNumbers(casing.Split(constant.Type.Name))

	if len(nameParts) == 0 {
		return ""
	}

	maxStrips := 1
	if len(typeParts) > 1 {
		maxStrips = len(nameParts) / len(typeParts)
	}

	var strips int
	for len(typeParts) > 0 && len(nameParts) >= len(typeParts) {
		if !partsEqualFold(nameParts[:len(typeParts)], typeParts) {
			break
		}

		nameParts = nameParts[len(typeParts):]
		strips++
		if strips >= maxStrips {
			break
		}
	}

	return casing.Join(nameParts, "_", strings.ToUpper)
}

func partsEqualFold(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := range a {
		if !strings.EqualFold(a[i], b[i]) {
			return false
		}
	}

	return true
}

type enumTemplateData struct {
	Type  enumTemplateName
	Value enumTemplateName
}

type enumTemplateName struct {
	Raw       string
	Pascal    string
	Camel     string
	Kebab     string
	Snake     string
	Screaming string
}

func enumTemplateDataForType(typ types.Type) enumTemplateData {
	return enumTemplateData{
		Type: enumTemplateName{
			Raw:       typ.Name,
			Camel:     casing.LowerCamel(typ.Name),
			Kebab:     casing.Kebab(typ.Name),
			Pascal:    casing.Camel(typ.Name),
			Screaming: casingScreamingSnake(typ.Name),
			Snake:     casing.Snake(typ.Name),
		},
		Value: enumTemplateName{
			Raw:       enumValueMarker("Raw"),
			Camel:     enumValueMarker("Camel"),
			Kebab:     enumValueMarker("Kebab"),
			Pascal:    enumValueMarker("Pascal"),
			Screaming: enumValueMarker("Screaming"),
			Snake:     enumValueMarker("Snake"),
		},
	}
}

var enumValueMarkerReplacements = map[string]string{
	"Raw":       `(.+?)`,
	"Camel":     `([a-z][A-Za-z0-9]*)`,
	"Kebab":     `([a-z][a-z0-9]*(?:-[a-z0-9]+)*)`,
	"Pascal":    `([A-Z][A-Za-z0-9]*)`,
	"Screaming": `([A-Z][A-Z0-9]*(?:_[A-Z0-9]+)*)`,
	"Snake":     `([a-z][a-z0-9]*(?:_[a-z0-9]+)*)`,
}

func normalizeEnumConstantPattern(constant types.ConstantDecl, pattern string) (string, error) {
	templateData := enumTemplateDataForType(constant.Type)
	validation, err := renderEnumPatternTemplate(pattern, templateData)
	if err != nil {
		return "", fmt.Errorf("failed to render enum template: %w (validation)", err)
	}

	tokens := tokenizeRenderedEnumPattern(validation)
	if err := validateEnumValueBoundaries(tokens); err != nil {
		return "", fmt.Errorf("enum pattern template validation failed: %w", err)
	}

	temp, err := renderEnumPatternTemplate(pattern, templateData)
	if err != nil {
		return "", fmt.Errorf("failed to render enum template: %w", err)
	}

	rep := regexp.QuoteMeta(temp)
	for style, replacement := range enumValueMarkerReplacements {
		rep = strings.ReplaceAll(rep, regexp.QuoteMeta(enumValueMarker(style)), replacement)
	}

	re, err := regexp.Compile("^" + rep + "$")
	if err != nil {
		return "", fmt.Errorf("failed to compile enum constant pattern: %w", err)
	}

	matches := re.FindStringSubmatch(constant.Name)
	if matches == nil {
		return "", errors.New("failed to match enum constant pattern")
	}

	return casingScreamingSnake(matches[1]), nil
}

func renderEnumPatternTemplate(pattern string, data enumTemplateData) (string, error) {
	temp := template.New(pattern)
	temp, err := temp.Parse(pattern)
	if err != nil {
		return "", fmt.Errorf("failed to parse enum constant pattern: %w", err)
	}

	var sb strings.Builder
	if err := temp.Execute(&sb, data); err != nil {
		return "", fmt.Errorf("failed to execute enum constant pattern: %w", err)
	}

	return sb.String(), nil
}

func validateEnumValueBoundaries(tokens []enumPatternValueToken) error {
	for i := 0; i < len(tokens)-2; i++ {
		left := tokens[i]
		mid := tokens[i+1]
		right := tokens[i+2]

		if left.kind != enumPatternValueTokenKindValue ||
			mid.kind != enumPatternValueTokenKindLiteral ||
			right.kind != enumPatternValueTokenKindValue {
			continue
		}

		if ambiguousEnumValueBoundary(left.style, mid.text, right.style) {
			return fmt.Errorf(
				"ambiguous enum value boundary between %s and %s using %q",
				left.style,
				right.style,
				mid.text,
			)
		}
	}

	return nil
}

func ambiguousEnumValueBoundary(left, literal, right string) bool {
	if literal == "" {
		return true
	}

	if left == "Raw" && right == "Raw" {
		return true
	}

	switch left {
	case "Screaming":
		return literal == "_" && casingStartsUpper(right)
	case "Snake":
		return literal == "_" && casingStartsLower(right)
	case "Kebab":
		return literal == "-" && casingStartsLower(right)
	}

	return false
}

func casingStartsUpper(s string) bool {
	return s == "Screaming" || s == "Pascal"
}

func casingStartsLower(s string) bool {
	return s == "Camel" || s == "Kebab" || s == "Snake"
}

// enumPatternValueToken contains information about how values are intended to be rendered within an
// enum pattern string.
type enumPatternValueToken struct {
	kind  enumPatternValueTokenKind
	text  string // The literal text
	style string // Screaming, Pascal, Camel, etc.
}

type enumPatternValueTokenKind uint

const (
	enumPatternValueTokenKindLiteral enumPatternValueTokenKind = iota
	enumPatternValueTokenKindValue
)

// Impossible to accidentally write sentinel values for rendering into templates.
const (
	enumValueMarkerPrefix = "\x00MORPH_VALUE:"
	enumValueMarkerSuffix = "\x00"
)

// enumValueMarker returns a value marker string for the given style.
func enumValueMarker(style string) string {
	return enumValueMarkerPrefix + style + enumValueMarkerSuffix
}

var enumValueMarkerPattern = regexp.MustCompile(`\x00MORPH_VALUE:([A-Za-z]+)\x00`)

func tokenizeRenderedEnumPattern(rendered string) []enumPatternValueToken {
	var pos int
	var tokens []enumPatternValueToken

	for _, loc := range enumValueMarkerPattern.FindAllStringSubmatchIndex(rendered, -1) {
		if loc[0] > pos {
			tokens = append(tokens, enumPatternValueToken{
				kind: enumPatternValueTokenKindLiteral,
				text: rendered[pos:loc[0]],
			})
		}

		tokens = append(tokens, enumPatternValueToken{
			kind:  enumPatternValueTokenKindValue,
			style: rendered[loc[2]:loc[3]],
		})

		pos = loc[1]
	}

	if pos < len(rendered) {
		tokens = append(tokens, enumPatternValueToken{
			kind: enumPatternValueTokenKindLiteral,
			text: rendered[pos:],
		})
	}

	return tokens
}

// isEnumType returns true if the given type declaration looks like an enum type, i.e. it's a basic
// type that's either a string or int-ish, and has at least one constant associated with it.
func isEnumType(typ types.TypeDecl) bool {
	if len(collectExportedConstants(typ.Constants)) == 0 {
		return false
	}

	underlying := types.UnwrapAlias(typ.Underlying)
	if underlying.Kind != types.TypeKindBasic {
		return false
	}

	switch underlying.Name {
	case "string",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64",
		"uintptr":
		return true
	default:
		return false
	}
}

func collectExportedConstants(constants map[string]types.ConstantDecl) map[string]types.ConstantDecl {
	out := make(map[string]types.ConstantDecl, len(constants))
	for _, constant := range constants {
		if constant.IsExported {
			out[constant.Name] = constant
		}
	}
	return out
}

// casingScreamingSnake returns a SCREAMING_SNAKE_CASE version of the input.
func casingScreamingSnake(value string, transforms ...casing.TransformFunc) string {
	transforms = append(transforms, strings.ToUpper)
	return casing.Join(casing.MergeNumbers(casing.Split(value)), "_", transforms...)
}
