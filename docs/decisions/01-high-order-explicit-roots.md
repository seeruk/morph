# Higher-Order Explicit Roots

## Status

Will not implement.

## Context

Morph can already plan concrete nested generic instantiations. For example, a field mapping from
`from.Optional[foopb.Bar]` to `to.Optional[foo.Bar]` can be planned as a concrete nested struct
mapper, and the `foopb.Bar` to `foo.Bar` value can even reuse an explicit root mapper.

Morph can also already use discovered higher-order functions. A user can provide a function such as 
`MapOptional[I, O any](in Optional[I], mapValue func(I) O) to.Optional[O]`, include it in 
discovery, and Morph can plan the mapper argument separately.

## Problem

Generic explicit roots are ambiguous. A mapping that looks like `from.Optional[T]` to
`to.Optional[T]` does not necessarily mean the source and target type arguments are the same type.
The source package and target package may both happen to name their type parameter `T`. Furthermore,
types with multiple type parameters are not only ambiguous in this way, but also in which type 
parameters should map to one another; for example, with `Either[L, R]` to `Tuple[A, B]` is `L` meant
to map to `A` or `B`?

A higher-order combinator would express that mapping with no ambiguity:

```go
func MapOptional[I, O any](
	in from.Optional[I],
	mapValue func(I) O,
) to.Optional[O]
```

Once you see this signature, you may realize there's another issue. Morph would need to generate 
many different versions of this higher-order mapper to support accepting mapper parameters which 
could error. Types with multiple type parameters would need config to specify which possible 
combinations to generate.

## Explored Design

A potential approach was explored, with the following requirements being determined:

* A new `kind` field could be added to mapper specification, only being usable on generic type 
  pairs where this functionality could be useful.
* Generic explicit roots could default to `kind: combinator`, with `kind: plain` preserving the
  current direct generic mapper shape.
* Mapper arguments would be configured explicitly, for example `mapLeft: L -> A` and
  `mapRight: R -> B`.
* The non-erroring combinator would always be generated from the mapper's base `name`.
* Additional error-capable variants would be selected explicitly, rather than generating every
  possible variant by default.
* Variant names could append a plain `suffix` to the base name, with an optional full `name`
  template override for uncommon cases.
* Repeated type parameters would be keyed by unique source/target pairs. For example, `T -> A` and
  `T -> B` would be separate mapper arguments.
* Crossed type parameters would be derived from explicit field mappings where possible. Morph should
  not guess from declaration order when field names differ, as such a fallback to explicitly specify
  this would also be required.
* Source and target type parameter name collisions would require deterministic generated names, such
  as renaming a colliding pair to `IT` and `OT`, then falling back to progressively more verbose
  names if those also collide like `InT` and `OutT`, so on.
* Mapper argument signatures might need their own value/pointer configuration, similar to existing
  mapper signatures. This also multiples the possible functions to generate.

## Decision Rationale

The design is possible, but while planning the functionality, it became apparent that the volume of 
configuration required was growing quickly. The user may need to specify mapper kind, type-parameter
mappings, selected error variants, variant naming, field mappings, type-parameter naming behaviour, 
and potentially per-argument value/pointer signatures.

At that point, the configuration can approach the complexity of writing the higher-order mapper
function directly, potentially even surpassing it. A user-written mapper is also clearer Go API 
design: it lets the user choose the exact generic signature, error behaviour, pointer/value shape, 
and naming without Morph inventing a large schema around those choices.

This would also significantly complicate both the planner and likely the code generator, as a 
greater number of paths to a valid plan would be possible.

## Current Solutions

For one-off or reusable generic container mappings, users should write the higher-order mapper
function directly and let Morph discover it, or explicitly include it in discovery.

This keeps Morph's core planning model simpler while still allowing advanced generic composition
where it is useful.

These mapping functions shouldn't need to be written frequently, or change often. Morph's aim is to
remove the manual work in generating large volumes of mapping code for similar types where it 
becomes error-prone, and typically this is issue becomes much more apparent when dealing with normal
domain types.
