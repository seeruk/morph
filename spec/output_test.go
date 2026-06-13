package spec

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOutputStrategy_UnmarshalText(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want OutputStrategy
		errs bool
	}{
		{
			name: "single package",
			in:   "single_package",
			want: OutputStrategySinglePackage,
		},
		{
			name: "source package",
			in:   "source_package",
			want: OutputStrategySourcePackage,
		},
		{
			name: "target package",
			in:   "target_package",
			want: OutputStrategyTargetPackage,
		},
		{
			name: "case insensitive",
			in:   "SOURCE_PACKAGE",
			want: OutputStrategySourcePackage,
		},
		{
			name: "invalid",
			in:   "together_package",
			errs: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var strategy OutputStrategy
			err := strategy.UnmarshalText([]byte(tt.in))
			if tt.errs {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tt.want, strategy)
		})
	}
}

func TestOutputStrategy_String(t *testing.T) {
	tests := []struct {
		name     string
		strategy OutputStrategy
		want     string
	}{
		{
			name:     "single package",
			strategy: OutputStrategySinglePackage,
			want:     "single_package",
		},
		{
			name:     "source package",
			strategy: OutputStrategySourcePackage,
			want:     "source_package",
		},
		{
			name:     "target package",
			strategy: OutputStrategyTargetPackage,
			want:     "target_package",
		},
		{
			name:     "invalid",
			strategy: OutputStrategy(99),
			want:     "OutputStrategy(99)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, tt.strategy.String())
		})
	}
}

func TestOutputStrategy_MarshalText(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		got, err := OutputStrategySinglePackage.MarshalText()

		require.NoError(t, err)
		assert.Equal(t, []byte("single_package"), got)
	})

	t.Run("invalid", func(t *testing.T) {
		got, err := OutputStrategy(99).MarshalText()

		require.Error(t, err)
		assert.Nil(t, got)
		assert.ErrorContains(t, err, "unknown output strategy: OutputStrategy(99)")
	})
}
