package spec

// Defaults represents available options for configuring Morph defaults.
type Defaults struct {
	Packages PackagesDefaults `json:"packages"`
}

// PackagesDefaults represents available options for configuring Packages-level defaults.
type PackagesDefaults struct {
	Types  TypesDefaults `json:"types"`
	Output Output        `json:"output"`
}

// TypesDefaults represents available options for configuring type-pair-level defaults.
type TypesDefaults struct {
	Enum          EnumDefaults `json:"enum"`
	Mappers       Mappers      `json:"mappers"`
	Bidirectional *bool        `json:"bidirectional"`
}

// EnumDefaults represents available options for configuring enum mapping defaults.
type EnumDefaults struct {
	FailureMode *EnumFailureMode `json:"failureMode"`
}
