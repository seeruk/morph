package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/seeruk/morph"
	"github.com/seeruk/morph/config"
	"github.com/seeruk/morph/plan"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stderr))
}

func run(args []string, stderr io.Writer) int {
	flags := flag.NewFlagSet("morph", flag.ContinueOnError)
	flags.SetOutput(stderr)

	var configPath string

	flags.StringVar(&configPath, "config", "morph.yaml", "path to Morph config file")
	if err := flags.Parse(args); err != nil {
		return 2
	}

	files, planned, err := generate(configPath)
	if err != nil {
		printError(stderr, err)
		return 1
	}

	writeDiagnostics(stderr, planned.Diagnostics)
	if planned.HasFatalDiagnostics() {
		return 1
	}

	for _, file := range files {
		if err := writeOutputFile(file); err != nil {
			printError(stderr, err)
			return 1
		}
	}

	return 0
}

func generate(configPath string) ([]morph.OutputFile, morph.Plan, error) {
	configPath = filepath.Clean(configPath)
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		return nil, morph.Plan{}, err
	}

	spec, err := morph.ResolveConfig(cfg)
	if err != nil {
		return nil, morph.Plan{}, err
	}

	engine := morph.New(filepath.Dir(configPath))
	return engine.Generate(spec, filepath.Base(configPath))
}

func writeDiagnostics(w io.Writer, diagnostics []plan.Diagnostic) {
	if len(diagnostics) == 0 {
		return
	}
	printLine(w, plan.FormatDiagnosticsHeader(diagnostics))
	printLine(w, plan.FormatDiagnostics(diagnostics))
}

func printError(w io.Writer, err error) {
	_, _ = fmt.Fprintf(w, "morph: %v\n", err)
}

func printLine(w io.Writer, value string) {
	_, _ = fmt.Fprintln(w, value)
}

func writeOutputFile(file morph.OutputFile) error {
	if err := os.MkdirAll(filepath.Dir(file.LogicalPath), 0o755); err != nil {
		return fmt.Errorf("create output directory for %s: %w", file.LogicalPath, err)
	}
	if err := os.WriteFile(file.LogicalPath, file.Source, 0o644); err != nil {
		return fmt.Errorf("write output file %s: %w", file.LogicalPath, err)
	}
	return nil
}
