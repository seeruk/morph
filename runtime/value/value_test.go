package value

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsComparableZero(t *testing.T) {
	t.Run("returns true for zero values", func(t *testing.T) {
		assert.True(t, IsComparableZero(""))
		assert.True(t, IsComparableZero(0))
		assert.True(t, IsComparableZero(comparableValue{}))
	})

	t.Run("returns false for non-zero values", func(t *testing.T) {
		assert.False(t, IsComparableZero("hello"))
		assert.False(t, IsComparableZero(1))
		assert.False(t, IsComparableZero(comparableValue{Name: "morph"}))
	})
}

func TestIsIncomparableZero(t *testing.T) {
	t.Run("returns true for nil slices and maps", func(t *testing.T) {
		assert.True(t, IsIncomparableZero([]string(nil)))
		assert.True(t, IsIncomparableZero(map[string]int(nil)))
		assert.True(t, IsIncomparableZero(incomparableValue{}))
	})

	t.Run("returns false for non-zero values", func(t *testing.T) {
		assert.False(t, IsIncomparableZero([]string{"morph"}))
		assert.False(t, IsIncomparableZero(map[string]int{"morph": 1}))
		assert.False(t, IsIncomparableZero(incomparableValue{Names: []string{"morph"}}))
	})

	t.Run("returns false for initialized empty slices and maps", func(t *testing.T) {
		assert.False(t, IsIncomparableZero([]string{}))
		assert.False(t, IsIncomparableZero(map[string]int{}))
		assert.False(t, IsIncomparableZero(incomparableValue{Names: []string{}}))
	})
}

type comparableValue struct {
	Name string
	Age  int
}

type incomparableValue struct {
	Names []string
}
