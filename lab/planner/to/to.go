package to

type Single struct {
	Foo string
	Bar Optional[string]
	Baz bool
}

type Optional[T any] struct {
	Value T
	Valid bool
}
