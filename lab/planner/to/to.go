package to

type Monster struct {
	Name   string
	Health int32
}

type RecipeID string

type Recipe struct {
	ID       RecipeID
	Name     string
	Servings int
	Glorp    string
}

func (r *Recipe) SetID(id RecipeID) {
	r.ID = id
}

type Tuple[A, B any] struct {
	A A
	B B
}

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
	Bar    Optional[Stringy]
	Baz    []Optional[*Stringy]
	Nested Nested
	Bla    Stringy
}

type Generic[T any] struct {
	Foo Optional[T]
	Bar T
}

type Explicit struct {
	Foo string
}

type Optional[T any] struct {
	Value T
	Valid bool
}

type Nested struct {
	Foo string
	Bar int
	Baz Optional[Stringy]
}
