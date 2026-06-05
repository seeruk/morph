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

type OptionalBadContainer struct {
	Maybe Optional[OptionalBadThing]
}

type FallibleOptionalContainer struct {
	Maybe FallibleOptional[FallibleThing]
}

type EitherContainer struct {
	Result Either[EitherLeft, EitherRight]
}

type ExplicitCallableContainer struct {
	Value int
}

type MethodCallableContainer struct {
	ID string
}

type ConversionContainer struct {
	ID     string
	Count  int64
	Secret string
	Alias  string
	Values []string
}

type ConversionsPolicyContainer struct {
	ID    string
	Other string
}

type StructConversionContainer struct {
	Code StructCode
}

type ScopedStringBoxA struct {
	Box Box[int]
}

type ScopedStringBoxB struct {
	Box Box[int]
}

type WarningSliceContainer struct {
	Values []WarningThing
}

type OptionalThing struct {
	Name string
}

type OptionalBadThing struct {
	Value bool
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

type WarningThing struct {
	Name  string
	Extra string
}

type Node struct {
	Name     string
	Children []Node
	Required string
	Next     *Node
}

type OptionalRecursiveNode struct {
	Maybe    Optional[[]OptionalRecursiveNode]
	Required string
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

type StructCode struct {
	Value string
}
