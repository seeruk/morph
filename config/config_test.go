package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sebdah/goldie/v2"
	"github.com/seeruk/morph/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadFromFile(t *testing.T) {
	got, err := config.LoadFromFile("testdata/load_from_file.yaml")

	require.NoError(t, err)
	goldie.New(t, goldie.WithFixtureDir("testdata/golden")).AssertJson(t, "load_from_file", got)
}

func TestLoadFromFile_MissingFile(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "missing.yaml")

	_, err := config.LoadFromFile(filename)

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to open file "+filename)
}

func TestLoadFromFile_InvalidYAML(t *testing.T) {
	filename := writeConfigFile(t, "packages:\n- [")

	_, err := config.LoadFromFile(filename)

	require.Error(t, err)
	assert.ErrorContains(t, err, "failed to parse file "+filename)
}

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()

	filename := filepath.Join(t.TempDir(), "morph.yaml")
	require.NoError(t, os.WriteFile(filename, []byte(content), 0o600))

	return filename
}
