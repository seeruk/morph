package spec

import (
	"testing"

	"github.com/seeruk/morph/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallableRef_UnmarshalText(t *testing.T) {
	tt := []struct {
		name string
		in   string
		out  CallableRef
		errs bool
	}{
		{
			name: "empty",
			in:   "",
			out:  CallableRef{},
		},
		{
			name: "function",
			in:   "module.test/example.Function",
			out: CallableRef{
				ImportPath: "module.test/example",
				Name:       "Function",
			},
		},
		{
			name: "method",
			in:   "module.test/example.Type.Method",
			out: CallableRef{
				ImportPath: "module.test/example",
				TypeName:   "Type",
				Name:       "Method",
			},
		},
		{
			name: "should parse stdlib function refs",
			in:   "strconv.Atoi",
			out: CallableRef{
				ImportPath: "strconv",
				Name:       "Atoi",
			},
		},
		{
			name: "should parse stdlib method refs",
			in:   "time.Time.Format",
			out: CallableRef{
				ImportPath: "time",
				TypeName:   "Time",
				Name:       "Format",
			},
		},
		{
			name: "invalid - trailing dots",
			in:   "module.test/example.Type.Invalid.",
			errs: true,
		},
		{
			name: "invalid - only package",
			in:   "module.test/example",
			errs: true,
		},
		{
			name: "invalid - empty type",
			in:   "module.test/example..Method",
			errs: true,
		},
	}

	for _, tc := range tt {
		t.Run(tc.name, func(t *testing.T) {
			var ref CallableRef
			err := ref.UnmarshalText([]byte(tc.in))
			if tc.errs {
				require.Error(t, err, "expected error for %q", tc.in)
			} else {
				require.NoError(t, err, "expected no error for %q", tc.in)
			}
			assert.Equal(t, tc.out, ref, "expected ref %q to equal %q", tc.out, ref)
		})
	}
}

func TestCallableRefFromMethod(t *testing.T) {
	owner := types.Type{
		Kind:    types.TypeKindNamed,
		Name:    "Thing",
		Package: types.PackageRef{ImportPath: "module.test/example"},
	}
	target := types.Type{
		Kind:   types.TypeKindBasic,
		Name:   "string",
		String: "string",
	}

	t.Run("should return false without method owner or receiver", func(t *testing.T) {
		ref, ok := CallableRefFromMethod(types.Method{Name: "String"})

		assert.False(t, ok)
		assert.Equal(t, CallableRef{}, ref)
	})

	t.Run("should use method owner for pointer receiver refs", func(t *testing.T) {
		receiver := types.Type{
			Kind: types.TypeKindPointer,
			Elem: &owner,
		}

		ref, ok := CallableRefFromMethod(types.Method{
			Owner:    owner,
			Receiver: &types.Parameter{Type: receiver},
			Name:     "String",
			Results:  []types.Parameter{{Type: target}},
		})

		require.True(t, ok)
		assert.Equal(t, CallableRef{
			ImportPath: "module.test/example",
			TypeName:   "Thing",
			Name:       "String",
		}, ref)
	})
}
