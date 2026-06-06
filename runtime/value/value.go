package value

import "reflect"

// IsComparableZero returns true if the given comparable value is zero.
func IsComparableZero[T comparable](value T) bool {
	var zero T
	return value == zero
}

// IsIncomparableZero returns true if the given incomparable value is zero, using reflection.
// Morph generated code will only use this if it finds a type that is not comparable.
func IsIncomparableZero[T any](value T) bool {
	return reflect.ValueOf(value).IsZero()
}
