package spec

// Preset closely mirrors the types section of the packages defaults, as it provides a repeatable,
// easily-referenced set of defaults to apply.
type Preset struct {
	Name          string       `json:"name"`
	Enum          EnumDefaults `json:"enum"`
	Mappers       Mappers      `json:"mappers"`
	Bidirectional *bool        `json:"bidirectional"`
}
