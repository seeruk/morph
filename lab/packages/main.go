package main

import (
	"context"
	"log"

	"github.com/seeruk/morph/types"
)

func main() {
	loader := types.NewLoader(".")

	if err := loader.Load(context.Background(), "./lab/packages/example"); err != nil {
		panic(err)
	}

	pkgs := loader.Packages()

	for _, pkg := range pkgs {
		log.Println(pkg.Name)
		log.Println(pkg.ImportPath)
		log.Println(pkg.Dir)
	}
}
