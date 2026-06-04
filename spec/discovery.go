package spec

// Discovery represents available options for broad package auto-discovery of mapping functions.
type Discovery struct {
	// Packages is a list of package import paths to auto-discover functions from within.
	Packages []string
	// Exclusions allows functions to be excluded from broad package discovery. This can be useful if
	// you want to include a package, but not all functions within it.
	Exclusions []CallableRef
}
