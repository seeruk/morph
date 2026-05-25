package spec

// Discovery represents available options for configuring auto-discovery of mapping functions.
type Discovery struct {
	Packages   []DiscoveryPackage `json:"packages"`
	Exclusions []CallableRef      `json:"exclusions"`
}

// DiscoveryPackage represents available options for configuration auto-discovery of a specific
// referenced package, also allowing callables in that package to be excluded from discovery.
type DiscoveryPackage struct {
	ImportPath string `json:"import"` // This is intentionally different, it reads better in config
}
