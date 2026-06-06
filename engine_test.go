package morph

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngineGenerate_WithFatalDiagnostics(t *testing.T) {
	engine := New(".")
	specification := resolveTestConfig(t, configWithUnsupportedMapping())

	files, plan, err := engine.Generate(specification, "morph.yaml")
	require.NoError(t, err)

	assert.Nil(t, files)
	assert.True(t, plan.HasFatalDiagnostics())
}
