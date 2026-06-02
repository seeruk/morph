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

// Map takes a slice and a mapping function, returning a new slice containing mapped elements.
func Map[I, O any](s []I, fn func(I) O) []O {
	out := make([]O, 0, len(s))
	for _, v := range s {
		out = append(out, fn(v))
	}
	return out
}

// Reduce takes a slice and a reducer function, returning the accumulated result.
func Reduce[I, O any](s []I, acc O, fn func(O, I) O) O {
	for _, v := range s {
		acc = fn(acc, v)
	}
	return acc
}
