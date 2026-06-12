package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnumFailureMode_UnmarshalText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want EnumFailureMode
		errs bool
	}{
		{
			name: "error",
			in:   "error",
			want: EnumFailureModeError,
		},
		{
			name: "zero",
			in:   "zero",
			want: EnumFailureModeZero,
		},
		{
			name: "fallback",
			in:   "fallback",
			want: EnumFailureModeFallback,
		},
		{
			name: "case insensitive",
			in:   "FALLBACK",
			want: EnumFailureModeFallback,
		},
		{
			name: "invalid",
			in:   "panic",
			errs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mode EnumFailureMode
			err := mode.UnmarshalText([]byte(tt.in))
			if tt.errs {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, mode)
		})
	}
}

func TestEnumFailureMode_String(t *testing.T) {
	tests := []struct {
		name string
		mode EnumFailureMode
		want string
	}{
		{
			name: "error",
			mode: EnumFailureModeError,
			want: "error",
		},
		{
			name: "zero",
			mode: EnumFailureModeZero,
			want: "zero",
		},
		{
			name: "fallback",
			mode: EnumFailureModeFallback,
			want: "fallback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.mode.String())
		})
	}
}

func TestParameterKind_UnmarshalText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want ParameterKind
		errs bool
	}{
		{
			name: "value",
			in:   "value",
			want: ParameterKindValue,
		},
		{
			name: "pointer",
			in:   "pointer",
			want: ParameterKindPointer,
		},
		{
			name: "case insensitive",
			in:   "POINTER",
			want: ParameterKindPointer,
		},
		{
			name: "invalid",
			in:   "reference",
			errs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var kind ParameterKind
			err := kind.UnmarshalText([]byte(tt.in))
			if tt.errs {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, kind)
		})
	}
}

func TestParameterKind_String(t *testing.T) {
	tests := []struct {
		name string
		kind ParameterKind
		want string
	}{
		{
			name: "value",
			kind: ParameterKindValue,
			want: "value",
		},
		{
			name: "pointer",
			kind: ParameterKindPointer,
			want: "pointer",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}
