package morph_test

import (
	"testing"

	"github.com/seeruk/morph"
	"github.com/seeruk/morph/spec"
	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapperName(t *testing.T) {
	input := morph.NameInput{
		Source: types.Type{
			Name:    "User",
			Package: types.PackageRef{Name: "source"},
		},
		Target: types.Type{
			Name:    "Person",
			Package: types.PackageRef{Name: "target"},
		},
		TypeParams: []types.TypeParam{
			{
				Name:       "T",
				Constraint: types.Type{Name: "Entity", Package: types.PackageRef{Name: "domain"}},
			},
		},
		Signature: spec.MapperSignature{
			Accepts: new(spec.ParameterKindPointer),
			Returns: new(spec.ParameterKindValue),
		},
		CanError: true,
	}

	tests := []struct {
		name  string
		templ string
		want  string
	}{
		{
			name:  "renders type names",
			templ: "Map{{ .Source.Type }}To{{ .Target.Type }}",
			want:  "MapUserToPerson",
		},
		{
			name:  "renders package and type names",
			templ: "Map{{ .Source.Package }}{{ .Source.Type }}To{{ .Target.Package }}{{ .Target.Type }}",
			want:  "MapSourceUserToTargetPerson",
		},
		{
			name:  "renders signature names as function name parts",
			templ: "Map{{ .Source.Type }}{{ .Signature.Accepts }}To{{ .Target.Type }}{{ .Signature.Returns }}",
			want:  "MapUserPointerToPersonValue",
		},
		{
			name:  "renders type parameter names and constraints",
			templ: "Map{{ .Source.Type }}{{ range .TypeParams }}{{ .Name }}{{ .Constraint.Package }}{{ .Constraint.Type }}{{ end }}To{{ .Target.Type }}",
			want:  "MapUserTDomainEntityToPerson",
		},
		{
			name:  "renders error suffix when mapper can error",
			templ: "Map{{ .Source.Type }}To{{ .Target.Type }}{{ if .CanError }}OrError{{ end }}",
			want:  "MapUserToPersonOrError",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := morph.MapperName(input, tt.templ)

			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMapperName_Error(t *testing.T) {
	input := morph.NameInput{
		Source: types.Type{Name: "User"},
		Target: types.Type{Name: "Person"},
		Signature: spec.MapperSignature{
			Accepts: new(spec.ParameterKindPointer),
			Returns: new(spec.ParameterKindValue),
		},
	}

	tests := []struct {
		name       string
		templ      string
		wantErrMsg string
	}{
		{
			name:       "returns parse errors",
			templ:      "Map{{ if }}",
			wantErrMsg: "failed to parse template",
		},
		{
			name:       "returns execute errors",
			templ:      "Map{{ .Source.Name }}To{{ .Target.Name }}",
			wantErrMsg: "failed to execute template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := morph.MapperName(input, tt.templ)

			require.Error(t, err)
			assert.Empty(t, got)
			assert.ErrorContains(t, err, tt.wantErrMsg)
		})
	}
}
