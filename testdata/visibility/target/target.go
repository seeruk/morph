package target

type LowerSourceFieldContainer struct {
	Secret string
}

type LowerTargetFieldContainer struct {
	secret string
}

type EmbeddedFieldContainer struct {
	ID string
}

type UnexportedSourceRoot struct {
	Name string
}

type unexportedBothRoot struct {
	Name string
}
