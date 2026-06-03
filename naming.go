package morph

import (
	"fmt"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"

	"github.com/seeruk/morph/internal/slicesx"
	"github.com/seeruk/morph/plan"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
)

// NameInput is a type used to collect information used for templating a mapper function name.
type NameInput struct {
	Source     types.Type
	Target     types.Type
	TypeParams []types.TypeParam
	Signature  spec.MapperSignature
	RunHash    string
}

type nameTemplateData struct {
	Source     nameType
	Target     nameType
	TypeParams []nameTypeParam
	Signature  nameSignature
	RunHash    string
}

func nameTemplateDataFromInput(input NameInput) nameTemplateData {
	return nameTemplateData{
		Source:     nameTypeFromType(input.Source),
		Target:     nameTypeFromType(input.Target),
		TypeParams: slicesx.Map(input.TypeParams, nameTypeParamFromTypeParam),
		Signature:  nameSignatureFromType(input.Signature),
		RunHash:    input.RunHash,
	}
}

// nameType is a basic representation of a type for use in name templates.
type nameType struct {
	Type    string
	Package string
}

// nameTypeFromType returns a nameType from the given types.Type.
func nameTypeFromType(typ types.Type) nameType {
	return nameType{
		Type:    typ.Name,
		Package: uppercaseFirst(typ.Package.Name),
	}
}

// nameTypeParam is a basic representation of a type parameter for use in name templates.
type nameTypeParam struct {
	Name       string
	Constraint nameType
}

func nameTypeParamFromTypeParam(tp types.TypeParam) nameTypeParam {
	return nameTypeParam{
		Name:       tp.Name,
		Constraint: nameTypeFromType(tp.Constraint),
	}
}

// nameSignature is a basic representation of a signature for use in name templates.
type nameSignature struct {
	Accepts string
	Returns string
}

func nameSignatureFromType(sig spec.MapperSignature) nameSignature {
	return nameSignature{
		Accepts: uppercaseFirst(sig.Accepts.String()),
		Returns: uppercaseFirst(sig.Returns.String()),
	}
}

// MapperName attempts to return a name for the given NameInput using Go's text/template library,
// with NameInput as the template data.
func MapperName(input NameInput, templ string) (string, error) {
	temp := template.New(plan.TypeMapperKey(
		plan.TypeRefFromType(input.Source),
		plan.TypeRefFromType(input.Target),
		input.Signature,
	))

	temp, err := temp.Parse(templ)
	if err != nil {
		return "", fmt.Errorf("failed to parse template: %w", err)
	}

	var sb strings.Builder
	if err := temp.Execute(&sb, nameTemplateDataFromInput(input)); err != nil {
		return "", fmt.Errorf("failed to execute template: %w", err)
	}

	return sb.String(), nil
}

// uppercaseFirst uppercases the first Unicode character in the string, leaving the rest of the
// string untouched.
func uppercaseFirst(s string) string {
	if s == "" {
		return s
	}

	r, size := utf8.DecodeRuneInString(s)
	if r == utf8.RuneError && size == 0 {
		return s
	}

	return string(unicode.ToUpper(r)) + s[size:]
}
