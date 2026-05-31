package main

import (
	"fmt"

	"github.com/seeruk/morph"
	"github.com/seeruk/morph/config"
)

func main() {
	fmt.Println("Hello, World!")

	spec, err := config.LoadFromFile("lab/planner/morph.yaml")
	if err != nil {
		panic(err)
	}

	for _, pkg := range spec.Packages {
		fmt.Printf("%s -> %s\n", pkg.Source, pkg.Target)
	}

	engine := morph.New("lab/planner")

	plan, err := engine.Plan(spec, "morph.yaml")
	if err != nil {
		panic(err)
	}

	for _, diagnostic := range plan.Diagnostics {
		fmt.Println(diagnostic)
	}
}
