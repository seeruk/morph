package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHasFatalDiagnostics(t *testing.T) {
	t.Run("returns false without fatal diagnostics", func(t *testing.T) {
		assert.False(t, HasFatalDiagnostics([]Diagnostic{{
			Level:   DiagnosticLevelWarning,
			Message: "warning",
		}}))
	})

	t.Run("returns true with fatal diagnostics", func(t *testing.T) {
		assert.True(t, HasFatalDiagnostics([]Diagnostic{{
			Level:   DiagnosticLevelFatal,
			Message: "fatal",
		}}))
	})
}
