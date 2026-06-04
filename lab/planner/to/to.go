package to

type RecipeDifficulty uint

const (
	RecipeDifficultyEasy RecipeDifficulty = iota
	RecipeDifficultyMedium
	RecipeDifficultyHard
	RecipeDifficultyInsane
	recipeDifficultyMax
)

type Stringy string

func (s Stringy) String() string {
	return string(s)
}

type Single struct {
	Foo    string
	Bar    Optional[string]
	Baz    []Optional[string]
	Nested Nested
	Bla    Stringy
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
	Bar int
}
