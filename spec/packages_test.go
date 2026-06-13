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
		{
			name: "invalid",
			mode: EnumFailureMode(99),
			want: "EnumFailureMode(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.mode.String())
		})
	}
}

func TestEnumFailureMode_MarshalText(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := EnumFailureModeError.MarshalText()

		require.NoError(t, err)
		assert.Equal(t, []byte("error"), got)
	})

	t.Run("invalid", func(t *testing.T) {
		got, err := EnumFailureMode(99).MarshalText()

		require.Error(t, err)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "unknown enum failure mode: EnumFailureMode(99)")
	})
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
		{
			name: "invalid",
			kind: ParameterKind(99),
			want: "ParameterKind(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.kind.String())
		})
	}
}

func TestParameterKind_MarshalText(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := ParameterKindValue.MarshalText()

		require.NoError(t, err)
		assert.Equal(t, []byte("value"), got)
	})

	t.Run("invalid", func(t *testing.T) {
		got, err := ParameterKind(99).MarshalText()

		require.Error(t, err)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "unknown parameter kind: ParameterKind(99)")
	})
}

func TestCallablePriority_String(t *testing.T) {
	tests := []struct {
		name     string
		priority CallablePriority
		want     string
	}{
		{
			name:     "type",
			priority: CallablePriorityType,
			want:     "type",
		},
		{
			name:     "invalid",
			priority: CallablePriority(99),
			want:     "CallablePriority(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.priority.String())
		})
	}
}

func TestPointerOptionality_String(t *testing.T) {
	assert.Equal(t, "zero", PointerOptionalityZero.String())
	assert.Equal(t, "PointerOptionality(99)", PointerOptionality(99).String())
}

func TestPointerOptionality_MarshalText(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := PointerOptionalityZero.MarshalText()

		require.NoError(t, err)
		assert.Equal(t, []byte("zero"), got)
	})

	t.Run("invalid", func(t *testing.T) {
		got, err := PointerOptionality(99).MarshalText()

		require.Error(t, err)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "unknown pointer optionality: PointerOptionality(99)")
	})
}

func TestValueOptionality_String(t *testing.T) {
	assert.Equal(t, "nil", ValueOptionalityNil.String())
	assert.Equal(t, "ValueOptionality(99)", ValueOptionality(99).String())
}

func TestValueOptionality_MarshalText(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := ValueOptionalityNil.MarshalText()

		require.NoError(t, err)
		assert.Equal(t, []byte("nil"), got)
	})

	t.Run("invalid", func(t *testing.T) {
		got, err := ValueOptionality(99).MarshalText()

		require.Error(t, err)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "unknown value optionality: ValueOptionality(99)")
	})
}
