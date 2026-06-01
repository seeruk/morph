package mapsx

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInvert(t *testing.T) {
	t.Run("inverts keys and values", func(t *testing.T) {
		got := Invert(map[string]int{
			"one": 1,
			"two": 2,
		})

		assert.Equal(t, map[int]string{
			1: "one",
			2: "two",
		}, got)
	})

	t.Run("returns nil for nil maps", func(t *testing.T) {
		assert.Nil(t, Invert(map[string]string(nil)))
	})

	t.Run("returns an empty map for empty maps", func(t *testing.T) {
		got := Invert(map[string]string{})

		assert.NotNil(t, got)
		assert.Empty(t, got)
	})
}
