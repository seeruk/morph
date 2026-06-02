package to

type Container struct {
	First  Node
	Second Node
}

type GenericContainer struct {
	StringBox Box[string]
	IntBox    Box[int64]
}

type NumberContainer[U ~int | ~int64] struct {
	Box Box[U]
}

type StringContainer[U ~string] struct {
	Box Box[U]
}

type OptionalContainer struct {
	Maybe Optional[OptionalThing]
}

type FallibleOptionalContainer struct {
	Maybe FallibleOptional[FallibleThing]
}

type EitherContainer struct {
	Result Either[EitherLeft, EitherRight]
}

type OptionalThing struct {
	Name string
}

type FallibleThing struct {
	Name string
}

type EitherLeft struct {
	Code string
}

type EitherRight struct {
	Name string
}

type Node struct {
	Name string
	Next *Node
}

type Box[T any] struct {
	Value T
}

type Optional[T any] struct {
	Value T
	OK    bool
}

type FallibleOptional[T any] struct {
	Value T
	OK    bool
}

type Either[L, R any] struct {
	Left    L
	Right   R
	IsRight bool
}
