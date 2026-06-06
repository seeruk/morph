package source

type LowerSourceFieldContainer struct {
	secret string
}

type LowerTargetFieldContainer struct {
	Secret string
}

type SamePackageSecretSource struct {
	secret string
}

type SamePackageSecretTarget struct {
	secret string
}

type EmbeddedFieldContainer struct {
	ID
}

type ID string

type unexportedSourceRoot struct {
	Name string
}

type unexportedBothRoot struct {
	Name string
}

type samePackageUnexportedRoot struct {
	Name string
}
