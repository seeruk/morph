# Morph

A tool for automatically generating mapping code to map similar types.

Morph is a CLI tool, but can also be used as a library, to be integrated into other applications as
part of more complex code-generation pipelines.

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

If you want to point Morph at a specific configuration file:

```bash
$ morph -config path/to/config.yml
```

Morph plans and generates code based on where the config file is. The configuration file must be 
within a Go module.

## Configuration Overview

Morph requires a configuration file to function. It does not support taking parameters as flags. A
very basic configuration file to map between a few types in a couple of packages could look like 
this:

```yaml
# yaml-language-server: $schema=../../schemas/config.schema.json
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

### Defaults Hierarchy

Morph configuration is layered, allowing you to specify defaults, and subsequently override them at
more granular levels. Morph aims to be an unopinionated tool with sensible defaults. Top-level 
defaults are specified in the `defaults` section of the configuration.

The order of preference is:

1. Field-level config
2. Type-level config
3. Type preset
4. Package-level config
5. Package preset
6. Top-level default config
7. Morph built-in defaults

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
disable them for certain packages, types, or fields. This can be done at any of these levels like 
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
      fields:
        LegacyID:
          conversions:
            enabled: false # Disable again for this field.
```

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
    # Or at a the specific type level
    preset: protobuf
```

### Common recipes

* Specifically how to configure certain things
  * Field / enum value override
  * Bidirectional mapping
  * Custom callables
  * Explicitly allowed conversions
  * Output types

## Known Limitations

* Morph only generates top-level mappers for struct-to-struct or enum-to-enum mappings and does not
  support generating mapping functions for other types (e.g. basic types, slices, maps, so on).
* Morph does not load test packages, so cannot create mappings for types in test files.
* Morph only supports non-embedded fields. Unexported fields are supported only when the generated
  mapper is emitted in the field's declaring package.
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

* Package-local helpers could support cross-package mappings involving unexported fields.

## License

MIT

[1]: docs/decisions/01-high-order-explicit-roots.md
