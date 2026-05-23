package main

import (
	"fmt"

	"github.com/seeruk/morph/config"
)

func main() {
	fmt.Println("Hello, World!")

	spec, err := config.LoadFromFile("spec/structure.yaml")
	if err != nil {
		panic(err)
	}

	for _, pkg := range spec.Packages {
		fmt.Printf("%s -> %s\n", pkg.Source, pkg.Target)
	}
}
