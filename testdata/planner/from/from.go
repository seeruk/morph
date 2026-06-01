package from

type Container struct {
	First  Node
	Second Node
}

type GenericContainer struct {
	StringBox Box[string]
	IntBox    Box[int]
}

type Node struct {
	Name string
	Next *Node
}

type Box[T any] struct {
	Value T
}
