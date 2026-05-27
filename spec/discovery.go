package spec

// Discovery represents available options for configuring auto-discovery of mapping functions.
type Discovery struct {
	// Packages is a list of package import paths to auto-discover functions from within.
	Packages []string `json:"packages"`
	// Functions is a list of function-callables to explicitly include in discovery. It's not
	// necessary to add the package the function is within to the Packages list, as the package will
	// be loaded automatically by including a function from it.
	Functions []CallableRef `json:"inclusions"`
	// Exclusions allows functions AND methods to be excluded from discovery. This can be useful if
	// a package contains multiple candidate conversion functions, and you want to use a specific
	// one, or if you want to include a package, but not all functions or methods within it.
	Exclusions []CallableRef `json:"exclusions"`
}
