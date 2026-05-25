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

type mapTestCase[I, O any] struct {
	name string
	in   []I
	fn   func(I) O
	want []O
}

func runMapTests[I, O any](t *testing.T, tests []mapTestCase[I, O]) {
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Map(tt.in, tt.fn)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMap(t *testing.T) {
	runMapTests(t, []mapTestCase[int, int]{
		{
			name: "maps values",
			in:   []int{1, 2, 3},
			fn: func(v int) int {
				return v * 2
			},
			want: []int{2, 4, 6},
		},
		{
			name: "maps values in source order",
			in:   []int{5, 1, 4, 2, 3},
			fn: func(v int) int {
				return v + 10
			},
			want: []int{15, 11, 14, 12, 13},
		},
		{
			name: "returns empty sequence for empty source",
			in:   nil,
			fn: func(v int) int {
				return v * 2
			},
			want: []int{},
		},
	})

	runMapTests(t, []mapTestCase[int, string]{
		{
			name: "maps to a different output type",
			in:   []int{1, 2, 3},
			fn: func(v int) string {
				if v%2 == 0 {
					return "even"
				}

				return "odd"
			},
			want: []string{"odd", "even", "odd"},
		},
	})
}
