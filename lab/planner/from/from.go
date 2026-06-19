//go:generate flatc --gen-onefile --go-namespace from --filename-suffix .fb --go ./schema.fbs
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

type Recipe struct {
	RecipeId string
	Name     string
	Servings int32
}

func (r Recipe) NotAProperty() bool {
	return true
}

type Either[L, R any] struct {
	Left  L
	Right R
}

type Single struct {
	Foo    string
	Bar    Optional[string]
	Baz    *[]Optional[string]
	Nested Nested
	Bla    string
}

type Generic[T any] struct {
	Foo Optional[T]
	Bar T
}

type Explicit struct {
	Foo Optional[string]
}

type Nested struct {
	Foo Optional[string]
	Bar int64
	Baz *Optional[string]
}

type Optional[T any] struct {
	Value T
	Valid bool
}

func (o Optional[T]) Unwrap() T {
	return o.Value
}

func OtoO[IT, OT any](in Optional[IT], tfn func(IT) OT) to.Optional[OT] {
	return to.Optional[OT]{
		Value: tfn(in.Value),
		Valid: in.Valid,
	}
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

func OptionalOfString2(s *string) Optional[string] {
	return OptionalOfString(s)
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

func BsToS(bs []byte) string {
	return string(bs)
}

func SToBs(s string) []byte {
	return []byte(s)
}
