package from

import "github.com/seeruk/morph/lab/planner/to"

type Single struct {
	Foo string
	Bar Optional[string]
}

type Optional[T any] struct {
	Value T
	Valid bool
}

func (o *Optional[T]) AsOtherOptional() to.Optional[T] {
	return to.Optional[T]{
		Value: o.Value,
		Valid: o.Valid,
	}
}
