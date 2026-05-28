package alpha

type Difficulty uint

const (
	DifficultyEasy Difficulty = iota
	DifficultyHard
	difficultyMax

	Message = "hello"
)

type ID = string

type Embedded struct {
	Name string
}

type Node struct {
	Next *Node
}

type Pair[T any] struct {
	*Embedded
	Source T `json:"source"`
	Target T `json:"target"`
}

func (p Pair[T]) Invert() Pair[T] {
	return Pair[T]{Source: p.Target, Target: p.Source}
}

func (p *Pair[T]) Set(source T, target T) {
	p.Source = source
	p.Target = target
}

type Reader interface {
	Read(p []byte) (n int, err error)
}

type FuncType func(ctx string, values ...int) error

type Complex struct {
	Ptr      *Difficulty
	Slice    []string
	Array    [2]int
	Map      map[string]Difficulty
	SendOnly chan<- string
	RecvOnly <-chan int
	Both     chan bool
	Reader   Reader
	Handler  FuncType
}

func Exported(a int, b string) (bool, error) {
	return true, nil
}

func generic[T ~uint](value T) T {
	return value
}

func Variadic(prefix string, values ...int) []int {
	return values
}
