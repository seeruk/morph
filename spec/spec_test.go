package spec

import (
	"testing"

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
