package from

type Single struct {
	Foo string
	Bar Optional[string]
}

type Optional[T any] struct {
	Value T
	Valid bool
}
