package to

type Container struct {
	First  Node
	Second Node
}

type GenericContainer struct {
	StringBox Box[string]
	IntBox    Box[int64]
}

type Node struct {
	Name string
	Next *Node
}

type Box[T any] struct {
	Value T
}
