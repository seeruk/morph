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

* High level concepts
* Link to fully documented example

### Common recipes

* Specifically how to configure certain things
  * Field / enum value override
  * Bidirectional mapping
  * Custom callables
  * Discovery
  * Explicitly allowed conversions
  * Output types
  * Presets

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
