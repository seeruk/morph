package to

type Container struct {
	First  Node
	Second Node
}

type Node struct {
	Name string
	Next *Node
}
