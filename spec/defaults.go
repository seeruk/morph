package spec

// Defaults represents resolved top-level defaults used by Morph while planning implicit mappings.
type Defaults struct {
	Types TypeDefaults
}

// TypeDefaults represents resolved defaults used when Morph has to create implicit type mappings
// that were not present as explicit package type mappings.
type TypeDefaults struct {
	Enum        Enum
	Callables   []CallableRef
	Mappers     Mappers
	Optionality Optionality
	Conversions ConversionsPolicy
}
