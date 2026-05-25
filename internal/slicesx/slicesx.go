package slicesx

// Filter takes a slice and a predicate function, returning a new slice containing only elements
// which passed the given predicate function.
func Filter[T any](s []T, fn func(T) bool) []T {
	var out []T
	for _, v := range s {
		if fn(v) {
			out = append(out, v)
		}
	}
	return out
}
