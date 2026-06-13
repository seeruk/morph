package cli_test

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/seeruk/morph/internal/cli"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunGeneratesWithRootCommand(t *testing.T) {
	dir := newWorkspace(t, false)
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := cli.Run(nil, &stdout, &stderr)

	assert.Equal(t, 0, code)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
	assert.FileExists(t, filepath.Join(dir, "mapping", "mapping.morph.go"))
}

func TestRunGeneratesWithGenerateCommand(t *testing.T) {
	dir := newWorkspace(t, false)
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"generate"}, &stdout, &stderr)

	assert.Equal(t, 0, code)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
	assert.FileExists(t, filepath.Join(dir, "mapping", "mapping.morph.go"))
}

func TestRunUsesRootConfigFlag(t *testing.T) {
	dir := newWorkspace(t, false)
	t.Chdir(dir)
	require.NoError(t, os.Rename(
		filepath.Join(dir, "morph.yaml"),
		filepath.Join(dir, "custom.morph.yaml"),
	))

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"-config", "custom.morph.yaml"}, &stdout, &stderr)

	assert.Equal(t, 0, code)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
	assert.FileExists(t, filepath.Join(dir, "mapping", "mapping.morph.go"))
}

func TestRunUsesGenerateConfigFlag(t *testing.T) {
	dir := newWorkspace(t, false)
	t.Chdir(dir)
	require.NoError(t, os.Rename(
		filepath.Join(dir, "morph.yaml"),
		filepath.Join(dir, "custom.morph.yaml"),
	))

	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"generate", "-config", "custom.morph.yaml"}, &stdout, &stderr)

	assert.Equal(t, 0, code)
	assert.Empty(t, stdout.String())
	assert.Empty(t, stderr.String())
	assert.FileExists(t, filepath.Join(dir, "mapping", "mapping.morph.go"))
}

func TestRunDryRun(t *testing.T) {
	t.Run("reports creates without writing files", func(t *testing.T) {
		dir := newWorkspace(t, false)
		t.Chdir(dir)

		var stdout, stderr bytes.Buffer
		code := cli.Run([]string{"--dry-run"}, &stdout, &stderr)

		assert.Equal(t, 0, code)
		assert.Empty(t, stderr.String())
		assert.Contains(t, stdout.String(), "1 to create")
		assert.Contains(t, stdout.String(), "create    mapping/mapping.morph.go")
		assert.Contains(t, stdout.String(), "No files were written")
		assert.NoFileExists(t, filepath.Join(dir, "mapping", "mapping.morph.go"))
	})

	t.Run("reports unchanged files", func(t *testing.T) {
		dir := newWorkspace(t, false)
		t.Chdir(dir)

		var generateStdout, generateStderr bytes.Buffer
		require.Equal(t, 0, cli.Run([]string{"generate"}, &generateStdout, &generateStderr))

		var stdout, stderr bytes.Buffer
		code := cli.Run([]string{"generate", "--dry-run"}, &stdout, &stderr)

		assert.Equal(t, 0, code)
		assert.Empty(t, stderr.String())
		assert.Contains(t, stdout.String(), "1 unchanged")
		assert.Contains(t, stdout.String(), "unchanged mapping/mapping.morph.go")
	})

	t.Run("reports updates without writing files", func(t *testing.T) {
		dir := newWorkspace(t, false)
		t.Chdir(dir)

		var generateStdout, generateStderr bytes.Buffer
		require.Equal(t, 0, cli.Run([]string{"generate"}, &generateStdout, &generateStderr))

		outputPath := filepath.Join(dir, "mapping", "mapping.morph.go")
		stale := "package mapping\n\n// stale\n"
		require.NoError(t, os.WriteFile(outputPath, []byte(stale), 0o644))

		var stdout, stderr bytes.Buffer
		code := cli.Run([]string{"generate", "--dry-run"}, &stdout, &stderr)

		got, err := os.ReadFile(outputPath)
		require.NoError(t, err)

		assert.Equal(t, 0, code)
		assert.Empty(t, stderr.String())
		assert.Contains(t, stdout.String(), "1 to update")
		assert.Contains(t, stdout.String(), "update    mapping/mapping.morph.go")
		assert.Equal(t, stale, string(got))
	})
}

func TestRunFatalDiagnosticsReturnError(t *testing.T) {
	dir := newWorkspace(t, true)
	t.Chdir(dir)

	var stdout, stderr bytes.Buffer
	code := cli.Run(nil, &stdout, &stderr)

	assert.Equal(t, 1, code)
	assert.Empty(t, stdout.String())
	assert.Contains(t, stderr.String(), "Morph found")
	assert.Contains(t, stderr.String(), "fatal")
	assert.NoFileExists(t, filepath.Join(dir, "mapping", "mapping.morph.go"))
}

func TestRunInvalidUsageReturnsUsageError(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.Run([]string{"--not-a-real-flag"}, &stdout, &stderr)

	assert.Equal(t, 2, code)
	assert.Contains(t, stdout.String(), "GLOBAL OPTIONS")
	assert.Contains(t, stderr.String(), "Incorrect Usage")
}

func newWorkspace(t *testing.T, fatal bool) string {
	t.Helper()

	dir := t.TempDir()
	writeFile(t, dir, "go.mod", `module example.com/morphcli

go 1.26.3
`)
	writeFile(t, dir, "from/from.go", `package from

type User struct {
	Name string
}
`)

	targetFieldType := "string"
	if fatal {
		targetFieldType = "int"
	}
	writeFile(t, dir, "to/to.go", `package to

type User struct {
	Name `+targetFieldType+`
}
`)
	writeFile(t, dir, "morph.yaml", `packages:
- source: example.com/morphcli/from
  target: example.com/morphcli/to
  types:
  - name: User
`)

	realDir, err := filepath.EvalSymlinks(dir)
	require.NoError(t, err)

	return realDir
}

func writeFile(t *testing.T, root, name, contents string) {
	t.Helper()

	path := filepath.Join(root, filepath.FromSlash(name))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(strings.TrimLeft(contents, "\n")), 0o644))
}
