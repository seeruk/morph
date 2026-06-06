# Morph

A tool for automatically generating mapping code to map similar types.

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
