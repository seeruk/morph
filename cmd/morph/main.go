package main

import (
	"fmt"
	"path/filepath"

	"github.com/seeruk/morph"
	"github.com/seeruk/morph/config"
	"github.com/seeruk/morph/plan"
)

func main() {
	cfgFilePath := filepath.Clean("lab/planner/morph.yaml")
	cfgFileName := filepath.Base(cfgFilePath)

	cfg, err := config.LoadFromFile(cfgFilePath)
	if err != nil {
		panic(err)
	}

	spec, err := morph.ResolveConfig(cfg)
	if err != nil {
		panic(err)
	}

	engine := morph.New("lab/planner")

	out, err := engine.Plan(spec, cfgFileName)
	if err != nil {
		panic(err)
	}

	if len(out.Diagnostics) > 0 {
		fmt.Println(plan.FormatDiagnosticsHeader(out.Diagnostics))
		fmt.Println(plan.FormatDiagnostics(out.Diagnostics))
	}
}
