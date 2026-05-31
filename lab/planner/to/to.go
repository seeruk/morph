package to

type Single struct {
	Foo string
	Bar Optional[string]
	Baz bool
}

type Generic[T any] struct {
	Foo Optional[T]
	Bar T
}

type Optional[T any] struct {
	Value T
	Valid bool
}
