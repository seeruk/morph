package mapsx

// Invert returns a new map with keys and values swapped.
func Invert[K, V comparable](in map[K]V) map[V]K {
	if in == nil {
		return nil
	}

	out := make(map[V]K, len(in))
	for k, v := range in {
		out[v] = k
	}
	return out
}
