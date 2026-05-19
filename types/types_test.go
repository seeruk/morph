package types_test

import (
	"testing"

	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
)

func TestUnwrapAlias(t *testing.T) {
	named := types.Type{
		Kind: types.TypeKindNamed,
		Name: "Target",
	}
	alias := types.Type{
		Kind: types.TypeKindAlias,
		Name: "Alias",
		Elem: &named,
	}
	nested := types.Type{
		Kind: types.TypeKindAlias,
		Name: "Nested",
		Elem: &alias,
	}

	tests := []struct {
		name string
		typ  types.Type
		want types.Type
	}{
		{
			name: "non alias",
			typ:  named,
			want: named,
		},
		{
			name: "alias",
			typ:  alias,
			want: named,
		},
		{
			name: "nested alias",
			typ:  nested,
			want: named,
		},
		{
			name: "alias without element",
			typ:  types.Type{Kind: types.TypeKindAlias, Name: "Broken"},
			want: types.Type{Kind: types.TypeKindAlias, Name: "Broken"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, types.UnwrapAlias(tt.typ))
		})
	}
}
