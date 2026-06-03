package from

import (
	"fmt"

	"github.com/seeruk/morph/lab/planner/to"
)

type Difficulty uint

const (
	DifficultyEasy Difficulty = iota
	DifficultyMedium
	DifficultyHard
	DifficultyUltra
	difficultyMax
)

type Single struct {
	Foo    string
	Bar    Optional[string]
	Baz    []Optional[string]
	Nested Nested
	Bla    string
}

type Generic[T any] struct {
	Foo Optional[T]
	Bar T
}

type Nested struct {
	Foo string
	Bar int
}

type Optional[T any] struct {
	Value T
	Valid bool
}

func (o Optional[T]) Unwrap() T {
	return o.Value
}

func OptionalOfString(s *string) Optional[string] {
	if s == nil {
		return Optional[string]{Valid: false}
	}

	return Optional[string]{
		Value: *s,
		Valid: true,
	}
}

func (o Optional[T]) AsOtherOptional() to.Optional[T] {
	return to.Optional[T]{
		Value: o.Value,
		Valid: o.Valid,
	}
}

func (o Optional[T]) AsOptionalString() to.Optional[string] {
	return to.Optional[string]{
		Value: fmt.Sprintf("%v", o.Value),
		Valid: o.Valid,
	}
}
