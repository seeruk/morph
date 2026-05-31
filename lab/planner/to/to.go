package to

type Single struct {
	Foo    string
	Bar    Optional[string]
	Baz    bool
	Nested Nested
}

type Generic[T any] struct {
	Foo Optional[T]
	Bar T
}

type Optional[T any] struct {
	Value T
	Valid bool
}

type Nested struct {
	Foo string
	Bar int64
}
