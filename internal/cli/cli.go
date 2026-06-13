package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/seeruk/morph"
	"github.com/seeruk/morph/config"
	"github.com/seeruk/morph/plan"
	urfavecli "github.com/urfave/cli/v3"
)

const (
	exitOK    = 0
	exitError = 1
	exitUsage = 2
)

// Run executes Morph's CLI and returns a process exit code.
func Run(args []string, stdout, stderr io.Writer) int {
	r := &runner{
		stdout:   stdout,
		stderr:   stderr,
		exitCode: exitOK,
	}

	cmd := r.command()
	if err := cmd.Run(context.Background(), append([]string{"morph"}, args...)); err != nil {
		return exitUsage
	}

	return r.exitCode
}

type runner struct {
	stdout   io.Writer
	stderr   io.Writer
	exitCode int
}

func (r *runner) command() *urfavecli.Command {
	return &urfavecli.Command{
		Name:           "morph",
		Usage:          "generate mapping code for similar Go types",
		DefaultCommand: "generate",
		Flags: []urfavecli.Flag{
			&urfavecli.StringFlag{
				Name:  "config",
				Value: "morph.yaml",
				Usage: "path to Morph config file",
			},
			&urfavecli.BoolFlag{
				Name:  "dry-run",
				Usage: "preview generated files without writing them",
			},
		},
		Commands: []*urfavecli.Command{
			{
				Name:   "generate",
				Usage:  "generate mapping code",
				Action: r.runGenerate,
			},
		},
		Writer:    r.stdout,
		ErrWriter: r.stderr,
		ExitErrHandler: func(context.Context, *urfavecli.Command, error) {
			// Run returns exit codes directly; keep urfave/cli from calling os.Exit for action errors.
		},
	}
}

func (r *runner) runGenerate(_ context.Context, cmd *urfavecli.Command) error {
	if r.generate(cmd.String("config"), cmd.Bool("dry-run")) {
		r.exitCode = exitOK
	} else {
		r.exitCode = exitError
	}

	return nil
}

func (r *runner) generate(configPath string, dryRun bool) bool {
	files, planned, err := generate(configPath)
	if err != nil {
		printError(r.stderr, err)
		return false
	}

	writeDiagnostics(r.stderr, planned.Diagnostics)
	if planned.HasFatalDiagnostics() {
		return false
	}

	if dryRun {
		if err := writeDryRun(r.stdout, files); err != nil {
			printError(r.stderr, err)
			return false
		}
		return true
	}

	for _, file := range files {
		if err := writeOutputFile(file); err != nil {
			printError(r.stderr, err)
			return false
		}
	}

	return true
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

type dryRunStatus string

const (
	dryRunCreate    dryRunStatus = "create"
	dryRunUpdate    dryRunStatus = "update"
	dryRunUnchanged dryRunStatus = "unchanged"
)

type dryRunFile struct {
	status dryRunStatus
	file   morph.OutputFile
}

func writeDryRun(w io.Writer, files []morph.OutputFile) error {
	preview := make([]dryRunFile, 0, len(files))
	counts := map[dryRunStatus]int{
		dryRunCreate:    0,
		dryRunUpdate:    0,
		dryRunUnchanged: 0,
	}

	for _, file := range files {
		status, err := classifyDryRunFile(file)
		if err != nil {
			return err
		}

		preview = append(preview, dryRunFile{status: status, file: file})
		counts[status]++
	}

	_, _ = fmt.Fprintf(
		w,
		"Morph would generate %s: %d to create, %d to update, %d unchanged\n\n",
		formatFileCount(len(files)),
		counts[dryRunCreate],
		counts[dryRunUpdate],
		counts[dryRunUnchanged],
	)
	for _, item := range preview {
		_, _ = fmt.Fprintf(w, "%-9s %s\n", item.status, displayPath(item.file.LogicalPath))
	}
	_, _ = fmt.Fprintln(w, "\nNo files were written")

	return nil
}

func classifyDryRunFile(file morph.OutputFile) (dryRunStatus, error) {
	existing, err := os.ReadFile(file.LogicalPath)
	if errors.Is(err, os.ErrNotExist) {
		return dryRunCreate, nil
	}
	if err != nil {
		return "", fmt.Errorf("read existing output file %s: %w", file.LogicalPath, err)
	}
	if bytes.Equal(existing, file.Source) {
		return dryRunUnchanged, nil
	}
	return dryRunUpdate, nil
}

func formatFileCount(count int) string {
	if count == 1 {
		return "1 file"
	}
	return fmt.Sprintf("%d files", count)
}

func displayPath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.ToSlash(path)
	}
	if realCwd, err := filepath.EvalSymlinks(cwd); err == nil {
		cwd = realCwd
	}

	rel, err := filepath.Rel(cwd, path)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(path)
	}

	return filepath.ToSlash(rel)
}
