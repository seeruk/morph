package main

import (
	"os"

	"github.com/seeruk/morph/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
