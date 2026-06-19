# Morph

A Go tool for automatically generating mapping code for similar types.

Morph is a CLI tool, but can also be used as a library, to be integrated into other applications as
part of more complex code-generation pipelines.

Morph generates functions like this:

```go
func MapFromRecipeToToRecipe(source *from.Recipe) to.Recipe {
  if source == nil {
    return to.Recipe{}
  }
  var target to.Recipe
  target.ID = to.RecipeID(source.RecipeId)
  target.Name = source.Name
  target.Servings = int(source.Servings)
  return target
}
```

Morph can map structs and enums, including fields containing basic types, pointers, slices, arrays,
maps, nested structs, and concrete generic containers, using direct assignment, safe or configured
type conversions, generated nested mappers, and user-provided callables.

## Why Morph?

Using certain libraries and tools can mean you to end up with what are essentially duplicated types.
For example, the ProtoBuf compiler doesn't generate idiomatic Go code, so you may want to represent
the same types with idiomatic Go code (e.g. using `time.Time`, with correct initialism in field
names, so on), or maybe you have a database library which uses code-gen.

Morph exists to attempt to alleviate the burden of writing boring, error-prone, time-consuming
manual mapping code for these types.

## Quick Start

Install Morph using the Go toolchain:

```bash
$ go install github.com/seeruk/morph/cmd/morph@latest
```

Morph requires a configuration file to get started. You can find more about that in the
[Configuration Overview](#configuration-overview) section below.

Once you have a valid configuration file, you can run Morph.

If you have a `morph.yaml` in the same folder:

```bash
$ morph
```

This is equivalent to the explicit generate command:

```bash
$ morph generate
```

If you want to point Morph at a specific configuration file:

```bash
$ morph -config path/to/config.yml
```

You can preview what Morph would write without changing files:

```bash
$ morph --dry-run
$ morph generate --dry-run
```

Morph plans and generates code based on where the config file is. The configuration file must be
within a Go module.

## Configuration Overview

Morph requires a configuration file to function. It does not support taking parameters as flags. A
very basic configuration file to map between a few types in a couple of packages could look like
this:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/seeruk/morph/main/schemas/config.schema.json
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - name: Recipe
  - name: Ingredient
  - source: Difficulty
    target: RecipeDifficulty
```

Configuration allows you to control quite a lot about how mapping works, what is generated, where it
gets generated, and what other resources Morph can draw on.

The following sections cover other config sections, and following that are some other common
"recipes" for things you might want to be able to do with Morph.

<details>
<summary>Defaults Hierarchy</summary>

### Defaults Hierarchy

Morph configuration is layered, allowing you to specify defaults, and subsequently override them at
more granular levels. Morph aims to be an unopinionated tool with sensible defaults. Top-level
defaults are specified in the `defaults` section of the configuration.

The order of preference is:

1. Property-level config
2. Type-level config
3. Type preset
4. Package-level config
5. Package preset
6. Top-level default config
7. Morph built-in defaults

It's worth noting, configuration on a package, type, or property level does not trickle down to nested
mapping functions that Morph generates automatically. If you need Morph to make a customized mapper,
it must be specified in the config file, or use top-level defaults.

</details>

<details>
<summary>Conversions</summary>

### Conversions

Morph supports generating type conversions between basic types when it's safe to do so. This
behaviour can be extended through configuration, allowing unsafe basic conversions, and allowing
custom types to be converted if their underlying type supports it.

Conversions are configured at the top-level in configuration:

```yaml
conversions:
- source: int
  targets:
  - int64
  - uint64
  bidirectional: true
- source: StringBasedID
  targets:
  - string
```

As conversions are global configuration, you might find there are scenarios where you want to
disable them for certain packages, types, or properties. This can be done at any of these levels like
so:

```yaml
packages:
- source: example.com/source
  target: example.com/target
  conversions:
    enabled: false # Disable for this package pair.
  types:
  - name: Example
    conversions:
      enabled: true # Re-enable for this type pair.
    struct:
      properties:
      - name: LegacyID
        conversions:
          enabled: false # Disable again for this property.
```

</details>

<details>
<summary>Discovery</summary>

### Discovery

Morph supports automatically finding and using potentially compatible mapping functions. This
functionality is separate from explicitly asking Morph to use callables for mapping, and allows
Morph to automatically use functions from explicitly listed packages, like so:

```yaml
discovery:
  packages:
  - github.com/example/mappers/datetime
  - github.com/example/mappers/numeric
  exclusions:
  - github.com/example/mappers/numeric.IntToInt64
```

Exclusions can be provided to prevent Morph from using specific functions discovered in these
packages, which can be useful if there are many potential functions, and not all of them are
actually intended for use as mapping functions.

</details>

<details>
<summary>Callables</summary>

### Callables

#### What are Callables?

Callables are functions or methods that can be explicitly referenced in the config file for Morph to
potentially use for mapping, instead of Morph generated the mapping itself. There are 2 main kinds
of callables:

##### Plain Callables

Plain callables are simple functions which take a source type and return a target type. These
callables can error, and if they do, that errability will propagate up to the parent mapper it's
used in, and so on.

```go
func FooToBar(foo Foo) Bar
func FooToBarE(foo Foo) (Bar, error)
```

Morph does also support generic callables, as long as they're used on matching concrete types. For
example. You might have an `Optional[T any]` and a `Nullable[T any]`, and they might be used on a
source field like `Foo Optional[string]` to `Foo Nullable[string]` - this is fine, and works pretty
much the same as above:

```go
func OptionalToNullable[T any](o Optional[T]) Nullable[T]
func OptionalToNullable[T any](o Optional[T]) (Nullable[T], error)
```

There are potential generic cases where Morph cannot use these functions though, for example, if the
type arguments differ on the source and target type (`Foo Optional[Bar]` to `Foo Nullable[Qux]`). In
this case, Morph wouldn't be able to map the inner type argument, it has no way to control it. For
these kinds of cases, you can use a combinator callable.

##### Combinator Callables

Combinator callables allow you to provide callables to Morph which can be used to handle many
generic types. They look like this:

```go
func OptionalToNullable[I, O any](o Optional[I], mapFn func(I) O) Nullable[O]
func OptionalToNullableE[I, O any](o Optional[I], mapFn func(I) (O, error)) (Nullable[O], error)
```

Morph can pass mapping functions it uses, or generates, or can generate inline mapping functions to
pass to these callables. If there are multiple type parameters, Morph expects a mapping function
argument on the callable for each type parameter on the source/target type; for example, for an
`Either[L, R any]` to `Tuple[A, B]` conversion, you could have:

```go
func EitherToTuple[L, R, A, B any](
    e Either[L, R],
    mapLeft func(L) A,
    mapRight mapRight func(R) B,
) Tuple[A, B]
```

The mapping functions should look like plain callables, and each mapping function argument may
return an error.

#### Configuring Callables

The aforementioned discovery is only for auto-discovery of entire packages worth of functions, for
other callables to be used by Morph, you must specify them explicitly. Discovery is a nice way to
include packages designed specifically for mapping, but you could end up pulling in way more than
you want. Also, discovery is not scoped.

Explicitly configuring callables is the solution to both of those issues. Similar to other
configuration options, you can configure callables in defaults, presets, on packages, on types, and
on specific properties. Configuration looks something like this:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - name: Recipe
    callables:
    - google.golang.org/protobuf/types/known/timestamppb.Timestamp.AsTime
    - google.golang.org/protobuf/types/known/timestamppb.New
```

In the above example, since this is specified at the type level, these functions can be used by
Morph for any property's value mapping. It will not trickle down to nested mappings.

Scoped callables are prioritized by where they are configured: type callables are tried before type
preset callables, then package callables, package preset callables, and finally defaults. Within the
same priority, Morph uses callable compatibility rank to choose the best candidate.

Specifying callables in the `defaults` section will make the callables available to any mapper at
the lowest priority.

Property callables can also receive ordered context source arguments. Context arguments must be exact
source fields or zero-argument methods; Morph does not infer or strip accessor prefixes for them.

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - name: Recipe
    struct:
      properties:
      - name: Title
        callable:
          forward:
            ref: example.com/foodplanner/food.OmittableFromPresence
            args:
            - source: MorphHasTitle
```

</details>

<details>
<summary>Presets</summary>

### Presets

Morph allows you to write named collections of default configuration which can be applied at the
package or type level. If you have a common pattern you want to use for certain packages, then it
means you can drastically cut down on duplicate config. Presets can be defined as so:

```yaml
presets:
  protobuf:
    bidirectional: true
    callables:
    - google.golang.org/protobuf/types/known/timestamppb.Timestamp.AsTime
    - google.golang.org/protobuf/types/known/timestamppb.New

    enum:
      failureMode: error
      patterns:
        source: "{{ .Type.Pascal }}_{{ .Type.Screaming }}_{{ .Value.Screaming }}"
        target: "{{ .Type.Pascal }}{{ .Value.Pascal }}"

    mappers:
      forward:
        name: Map{{ .Target.Type }}FromProto
        signature:
          accepts: pointer
          returns: value
      inverse:
        name: Map{{ .Target.Type }}ToProto
        signature:
          accepts: value
          returns: pointer

    optionality:
      onNilSourcePointer: zero
      onZeroSourceValue: nil

# And then applied:
packages:
- source: example.com/foopb
  target: example.com/foo
  preset: protobuf
  types:
  - name: Bar
    # Or at the specific type level
    preset: protobuf
```

</details>

### Other Common Scenarios

<details>
<summary>Configuring Output</summary>

#### Configuring Output

By default, Morph generates code into a single `mapping` package, in a `mapping` directory next to
the configuration file. You can configure this globally:

```yaml
defaults:
  packages:
    output:
      strategy: single_package
      path: internal/mapping
      package: mapping
      filename: mapping.morph.go
```

Or, for a specific package mapping:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  output:
    strategy: target_package
    filename: mapping.morph.go
  types:
  - name: Recipe
```

There are 3 output strategies:

* `single_package` writes generated code to the configured `path` and `package`.
* `source_package` writes generated code into the source package.
* `target_package` writes generated code into the target package.

For `source_package` and `target_package`, only `filename` is used. The package name and path come
from the existing package Morph is writing into.

</details>

<details>
<summary>Customizing Mapper Function Names</summary>

#### Customizing Mapper Function Names

Morph generates mapper function names from templates. You can configure mapper names in defaults,
presets, on packages, or on individual types. If you're generating ProtoBuf mappings, for example,
you might want names which make the direction clearer:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  mappers:
    forward:
      name: Map{{ .Target.Type }}FromProto
    inverse:
      name: Map{{ .Target.Type }}ToProto
  bidirectional: true
  types:
  - name: Recipe
```

With the above config, Morph would generate names like `MapRecipeFromProto` and
`MapRecipeToProto`.

Patterns use Go's `text/template` library. Input to the template is `nameTemplateData` found in
[naming.go](naming.go#L23). The package values are the Go package names with the first letter
uppercased, and the signature values are rendered as `Value` or `Pointer`.

</details>

<details>
<summary>Customizing Mapper Kind</summary>

#### Customizing Mapper Kind

By default, Morph generates plain functions. You can ask Morph to generate source-type methods
instead:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  output:
    strategy: source_package
  mappers:
    forward:
      kind: prefer_method
  types:
  - name: Recipe
```

* `kind: function` preserves the default function output.
* `kind: prefer_method` emits a method when the generated file is in the source type's package, and 
  otherwise falls back to a function.
* `kind: method` will emit a fatal diagnostic if Morph can't generate a method.

</details>

<details>
<summary>Customizing Mapper Signatures</summary>

#### Customizing Mapper Signatures

Similar to configuring mapper names, you can customize the signature of a mapper, controlling
whether the function accepts/returns pointers/values:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - name: Recipe
    mappers:
      forward:
        name: Map{{ .Source.Type }}PointerTo{{ .Target.Type }}
        signature:
          accepts: pointer
          returns: value
```

</details>

<details>
<summary>Overriding Property / Enum Value Mapping</summary>

#### Overriding Property / Enum Value Mapping

Morph will try to match logical struct properties by name, including case-insensitive matches. A
property is usually backed by a Go field, but can also be backed by getter and setter methods. If
property names don't match clearly, you can map them explicitly:

When `inferMethods` is enabled, Morph can use getter and setter-shaped methods as mapping
candidates, but unused inferred methods do not produce unmapped-property warnings. Coverage
warnings are reserved for field-backed properties.

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - name: Recipe
    struct:
      properties:
      - source: RecipeId
        target: ID
```

You can also configure exact accessors when Morph should read or write through specific methods:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - name: Recipe
    struct:
      properties:
      - source: EmailAddress
        target: Email
        accessors:
          forward:
            read: GetEmailAddress
            write: SetEmail
```

Enums work similarly. Morph will try to infer enum mappings by normalizing names, but you can
provide explicit value mappings where names don't line up:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - source: Difficulty
    target: RecipeDifficulty
    enum:
      failureMode: error
      values:
        Difficulty_DIFFICULTY_UNSPECIFIED: RecipeDifficultyUnknown
```

By default, enum mappers return an error when the source value falls through the generated switch.
Set `failureMode: zero` to return the target enum's zero value instead, or `failureMode: fallback`
to return a configured target enum constant. Fallback mode also allows inferred source constants
with no target match; those constants are omitted from the switch and use the fallback at runtime.
Explicit `values` entries are still validated.

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - source: Difficulty
    target: RecipeDifficulty
    enum:
      failureMode: fallback
      fallback:
        forward: RecipeDifficultyUnknown
        inverse: Difficulty_UNSPECIFIED
```

For enums with regular generated naming patterns, you can also configure patterns instead of
listing every value:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - source: Difficulty
    target: RecipeDifficulty
    enum:
      patterns:
        source: "{{ .Type.Pascal }}_{{ .Type.Screaming }}_{{ .Value.Screaming }}"
        target: "{{ .Type.Pascal }}{{ .Value.Pascal }}"
```

Patterns use Go's `text/template` library. Input to the template is `enumTemplateData` found in
[planner_enum.go](planner_enum.go#L259).

</details>

<details>
<summary>Omitting Properties</summary>

#### Omitting Properties

Morph reports coverage warnings when a target property cannot be populated from a source property,
or vice versa. If a property is intentionally outside the mapping, you can omit it like so:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  bidirectional: true
  types:
  - name: Recipe
    struct:
      omit:
        both:
        - Name
        source:
        - InternalState
        target:
        - CreatedAt
        - UpdatedAt
```

Use `both` when the same logical property exists on both sides but should not be mapped. Use `source`
for source-only properties that should be ignored, and `target` for target-only properties that should
be left unset.

For bidirectional mappings, source and target omissions are inverted automatically. Properties listed
under `source` are treated as target omissions on the inverse mapper, and properties listed under
`target` are treated as source omissions on the inverse mapper. Properties listed under `both` remain
matched omissions in both directions.

</details>

<details>
<summary>Bidirectional Mapping</summary>

#### Bidirectional Mapping

Many mappings are useful in both directions. You can enable bidirectional mapping in defaults, in a
preset, on a package, or on a specific type:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  bidirectional: true
  types:
  - name: Recipe
  - source: Difficulty
    target: RecipeDifficulty
```

This will generate both `foodpb -> food` and `food -> foodpb` mappings. Struct property mappings and
enum value mappings are inverted automatically for the inverse mapper. Enum fallback values are
configured directionally because each generated mapper returns a different target enum type.

If only one type should be bidirectional, configure it at the type level:

```yaml
packages:
- source: example.com/foodplanner/foodpb
  target: example.com/foodplanner/food
  types:
  - name: Recipe
    bidirectional: true
```

</details>

## Known Limitations

* Morph only generates top-level mappers for struct-to-struct or enum-to-enum mappings and does not
  support generating mapping functions for other types (e.g. basic types, slices, maps, so on).
* Morph does not load test packages, so cannot create mappings for types in test files.
* Morph does not support embedded fields.
* Morph only recognizes the standard, built-in `error` type for discovery, not custom aliases or
  wrappers.
* Morph does not support creating mappers explicitly for generic types. See
  [docs/decisions/01-high-order-explicit-roots.md][1] for the rationale.
* Morph assumes at least one package referenced in the spec is the main module. If this is not the
  case, Morph will not be able to figure out the workspace and planning will fail.
* If using `single_package` output, package name detection includes files that have build
  constraints, which could mean either the package name is incorrect, or that an error is returned
  when it shouldn't be.

## Future Enhancements

* Assignment of literal / constant values for unmapped fields (i.e. while mapping set field x to y)
* CLI improvements:
  * `morph plan` / `morph explain`, some sort of human-readable and/or machine-readable plan view
  * `morph init`, maybe point it at packages or something? Or maybe a different command which
    creates or updates config to include pairs of types found? `morph scan` or something?
* Package-local helpers could support cross-package mappings involving unexported fields.
* Built-in helpers which can be used for discovery en masse

## License

MIT

[1]: docs/decisions/01-high-order-explicit-roots.md
