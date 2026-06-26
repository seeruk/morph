package to

type Status uint

const (
	OK Status = iota
)

type Container struct {
	First  Node
	Second Node
}

type GenericContainer struct {
	StringBox Box[string]
	IntBox    Box[int64]
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
	Value int
}

type MethodCallableContainer struct {
	ID string
}

type ContextualCallableContainer struct {
	Value Contextual[int]
}

type ContextualHigherOrderContainer struct {
	Maybe Contextual[OptionalThing]
}

type Contextual[T any] struct {
	Value   T
	Present bool
}

type ConversionContainer struct {
	ID     string
	Count  int64
	Secret string
	Alias  string
	Values []string
}

type CollectionContainer struct {
	Values []string
	Codes  [2]string
	Lookup map[string]string
}

type ConversionsPolicyContainer struct {
	ID    string
	Other string
}

type StructConversionContainer struct {
	Code StructCode
}

type StructCode struct {
	Value string
}

type FieldToSetterContainer struct {
	name string
}

func (f *FieldToSetterContainer) SetName(name string) {
	f.name = name
}

type GetterToFieldContainer struct {
	Name string
	Kept string
}

type GetterToSetterContainer struct {
	name string
	Kept string
}

func (g *GetterToSetterContainer) SetName(name string) {
	g.name = name
}

type ReadMethodOnlyContainer struct {
	Kept string
}

func (r ReadMethodOnlyContainer) Name() string {
	return "name"
}

type ExplicitAccessorContainer struct {
	email string
	Kept  string
}

func (e *ExplicitAccessorContainer) StoreEmail(email string) {
	e.email = email
}

type BidirectionalAccessorContainer struct {
	email string
}

func (b BidirectionalAccessorContainer) GetEmail() string {
	return b.email
}

func (b *BidirectionalAccessorContainer) SetEmail(email string) {
	b.email = email
}

type ErrorAccessorContainer struct {
	name string
}

func (e *ErrorAccessorContainer) SetName(name string) error {
	e.name = name
	return nil
}

type ProtolikeMessage struct {
	ID string
}

type CaseInsensitivePropertyContainer struct {
	RecipeId string
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

type OmissionContainer struct {
	Shared     string
	TargetOnly string
	Mapped     string
}

type AmbiguousOmissionContainer struct {
	ApiID string
	APIId string
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
