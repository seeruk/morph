# Morph

A tool for automatically generating mapping code to map similar types.

## Known Limitations

* Morph assumes at least one package referenced in the spec is the main module. If this is not the
  case, Morph will not be able to figure out the workspace and planning will fail.
* If using `single_package` output, package name detection includes files that have build
  constraints, which could mean either the package name is incorrect, or that an error is returned
  when it shouldn't be.
* Morph does not support creating mappers explicitly for generic types. See
  docs/decisions/01-high-order-explicit-roots.md for the rationale. 

## License

MIT
