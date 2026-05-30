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

// We support generic mapping, if it's in this form:

// GenericOne has a generic type parameter on the overall type, therefore, we'd look for T -> T
type GenericOne[T any] struct {
	Value Optional[T]
}

// GenericTwo has no generic type parameter, and the field has specified string. In this case, we
// can look for:
//
// * Optional[T] -> Optional[T]
// * Optional[string] -> to.Optional[string] specifically.
//   - This can exist in two forms:
//   - - Field Optional[string]
//   - - type Foo Optional[string]
//   - - - This is probably not an issue, because it's a named type, and we'd only care about the
//     return type of the method matching the field we're mapping to exactly.
//
// * Optional[T] -> Optional[string]:
//   - Not sure that this one makes much sense, buy maybe it should be allowed. It could error, if
//     the user wanted it to.
//
// So, maybe this can be distilled into:
//   - If the return type does not contain a type parameter, and it's what we want, then we can
//     probably just use that, because a method is saying it'll give us what we want, anyway.
//   - If the return type does contain type parameters, then we have to have a fully matching set of
//     type parameters on the way in, in any order.
type GenericTwo struct {
	Value Optional[string]
}

// OptionalString interesting... didn't think about this before
type OptionalString Optional[string]

func (s OptionalString) AsOtherOptional() to.Optional[string] {
	return to.Optional[string]{
		Value: s.Value,
		Valid: s.Valid,
	}
}
