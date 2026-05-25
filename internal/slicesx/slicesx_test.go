package slicesx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilter(t *testing.T) {
	tests := []struct {
		name string
		in   []int
		fn   func(int) bool
		want []int
	}{
		{
			name: "keeps matching values",
			in:   []int{1, 2, 3, 4, 5, 6},
			fn: func(v int) bool {
				return v%2 == 0
			},
			want: []int{2, 4, 6},
		},
		{
			name: "keeps matching values in source order",
			in:   []int{5, 1, 4, 2, 3},
			fn: func(v int) bool {
				return v < 4
			},
			want: []int{1, 2, 3},
		},
		{
			name: "returns empty sequence when nothing matches",
			in:   []int{1, 3, 5},
			fn: func(v int) bool {
				return v%2 == 0
			},
			want: nil,
		},
		{
			name: "returns empty sequence for empty source",
			in:   nil,
			fn: func(v int) bool {
				return true
			},
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Filter(tt.in, tt.fn)
			assert.Equal(t, tt.want, got)
		})
	}
}
