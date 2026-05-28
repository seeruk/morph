package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

func TestEnum_ApplyDefaults(t *testing.T) {
	t.Run("should copy the default failure mode when unset", func(t *testing.T) {
		defaultFailureMode := EnumFailureModeError
		enum := &Enum{}

		enum.ApplyDefaults(EnumDefaults{FailureMode: &defaultFailureMode})

		require.NotNil(t, enum.FailureMode)
		assert.Equal(t, EnumFailureModeError, *enum.FailureMode)
		assert.NotSame(t, &defaultFailureMode, enum.FailureMode)
	})

	t.Run("should keep an explicit failure mode", func(t *testing.T) {
		enum := &Enum{FailureMode: new(EnumFailureModeZero)}

		enum.ApplyDefaults(EnumDefaults{FailureMode: new(EnumFailureModeError)})

		require.NotNil(t, enum.FailureMode)
		assert.Equal(t, EnumFailureModeZero, *enum.FailureMode)
	})

	t.Run("should tolerate nil enum receivers", func(t *testing.T) {
		defaultFailureMode := EnumFailureModeError
		var enum *Enum

		assert.NotPanics(t, func() {
			enum.ApplyDefaults(EnumDefaults{FailureMode: &defaultFailureMode})
		})
	})
}
