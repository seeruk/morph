# TODO

## Future Enhancements

* Function discovery does not support generic functions, unlike methods where generic methods are
  supported. The same mechanism should be used for functions, with the same priority system.

## Known Limitations

* Morph assumes at least one package referenced in the spec is the main module. If this is not the 
  case, Morph will not be able to figure out the workspace and planning will fail.
* If using `single_package` output, package name detection includes files that have build 
  constraints, which could mean either the package name is incorrect, or that an error is returned
  when it shouldn't be.

### Generic Containers

A generic container type (e.g. `Optional[T]`) can be mapped in various ways successfully already,
but if you wanted to do `from.Optional[foopb.Bar]` to `to.Optional[foo.Bar]` then currently you'd
need a mapping function for that exact _result_ (probably specifically a method for currently too?). 
There may be a mapping function for `foopb.Bar` to `foo.Bar` which could be used with a generic 
`from.Optional[T]` to `to.Optional[T]`, 

You'd probably need generic methods, so something like:

```go
package from

import "github.com/seeruk/morph/lab/planner/to"

type Optional[T any] struct {
	Value T
	Valid bool
}

func (o *Optional[T]) AsToOptional[O any](valueFn func(T) O) to.Optional[O] {
	return to.Optional[O]{
		Value: valueFn(o.Value),
		Valid: o.Valid,
	}
}
```

This would probably need to be a handwritten function, but it's incredibly useful because it 
expands on what we can automatically map with minimal handwritten code. That generic-method approach
is one way to tackle this problem, but it could also be tackled with functions, as long as functions
supported generics, and detecting this other form of allowable function works too. Here's what a 
function could look like today, without generic methods:

```go
package from

import "github.com/seeruk/morph/lab/planner/to"

func MapOptionalToOptionalFunc[I, O any](valueFn func(I) O) to.Optional[O] {
    return to.Optional[O]{
        Value: valueFn(o.Value),
        Valid: o.Valid,
    }
}
```

This does get a decent amount more complex if types have more than one type argument... Really, 
Morph would probably be relying on trusting the developer to write a valid and compatible signature
for what they wanted. After all, the compiler will check things like type constraint compatibility
already.
