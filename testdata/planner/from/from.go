package from

import "github.com/seeruk/morph/testdata/planner/to"

type Status uint

const (
	OK Status = iota
	StatusOK
)

type Container struct {
	First  Node
	Second Node
}

type GenericContainer struct {
	StringBox Box[string]
	IntBox    Box[int]
}

type NodeBoxContainer struct {
	Box Box[Node]
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
	Value string
}

type MethodCallableContainer struct {
	ID MethodID
}

type ConversionContainer struct {
	ID     UserID
	Count  int
	Secret SecretID
	Alias  AliasUserID
	Values []UserID
}

type CollectionContainer struct {
	Values []UserID
	Codes  [2]UserID
	Lookup map[UserID]UserID
}

type ConversionsPolicyContainer struct {
	ID    UserID
	Other UserID
}

type StructConversionContainer struct {
	Code StructCode
}

type ScopedStringBoxA struct {
	Box Box[string]
}

type ScopedStringBoxB struct {
	Box Box[string]
}

type WarningSliceContainer struct {
	Values []WarningThing
}

type OmissionContainer struct {
	Shared     string
	SourceOnly string
}

type OptionalThing struct {
	Name string
}

type OptionalBadThing struct {
	Value string
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
	Name string
}

type Node struct {
	Name     string
	Children []Node
	Required *string
	Next     *Node
}

type OptionalRecursiveNode struct {
	Maybe    Optional[[]OptionalRecursiveNode]
	Required *string
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

type MethodID struct {
	Value string
}

type UserID string

type AliasUserID = UserID

type SecretID string

type StructCode struct {
	Value string
}

func ExplicitStringToInt(in string) int {
	return len(in)
}

func TypeAStringToInt(in string) int {
	return len(in)
}

func TypeBStringToInt(in string) int {
	return len(in)
}

func ZZZStringToInt(in string) int {
	return len(in)
}

func (m MethodID) String() string {
	return m.Value
}

func MapOptional[I, O any](in Optional[I], mapValue func(I) O) to.Optional[O] {
	if !in.OK {
		return to.Optional[O]{}
	}

	return to.Optional[O]{
		Value: mapValue(in.Value),
		OK:    true,
	}
}

func MapOptionalWithError[I, O any](in Optional[I], mapValue func(I) (O, error)) (to.Optional[O], error) {
	if !in.OK {
		return to.Optional[O]{}, nil
	}

	value, err := mapValue(in.Value)
	if err != nil {
		return to.Optional[O]{}, err
	}

	return to.Optional[O]{
		Value: value,
		OK:    true,
	}, nil
}

func MapFallibleThing(in FallibleThing) (to.FallibleThing, error) {
	return to.FallibleThing{
		Name: in.Name,
	}, nil
}

func MapFallibleOptional[I, O any](
	in FallibleOptional[I],
	mapValue func(I) (O, error),
) (to.FallibleOptional[O], error) {
	if !in.OK {
		return to.FallibleOptional[O]{}, nil
	}

	value, err := mapValue(in.Value)
	if err != nil {
		return to.FallibleOptional[O]{}, err
	}

	return to.FallibleOptional[O]{
		Value: value,
		OK:    true,
	}, nil
}

func MapEither[LI, RI, LO, RO any](
	in Either[LI, RI],
	mapLeft func(LI) LO,
	mapRight func(RI) RO,
) to.Either[LO, RO] {
	if in.IsRight {
		return to.Either[LO, RO]{
			Right:   mapRight(in.Right),
			IsRight: true,
		}
	}

	return to.Either[LO, RO]{
		Left: mapLeft(in.Left),
	}
}
