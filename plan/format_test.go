package plan

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatDiagnostic(t *testing.T) {
	t.Run("formats diagnostics with a path", func(t *testing.T) {
		got := FormatDiagnostic(Diagnostic{
			Level:   DiagnosticLevelFatal,
			Path:    "source.Type->target.Type :: Source->Target",
			Message: "configure a callable",
		})

		assert.Equal(t, "fatal: source.Type->target.Type :: Source->Target\n  configure a callable", got)
	})

	t.Run("formats diagnostics without a path", func(t *testing.T) {
		got := FormatDiagnostic(Diagnostic{
			Level:   DiagnosticLevelWarning,
			Message: "something happened",
		})

		assert.Equal(t, "warning:\n  something happened", got)
	})

	t.Run("indents multiline messages", func(t *testing.T) {
		got := FormatDiagnostic(Diagnostic{
			Level:   DiagnosticLevelFatal,
			Path:    "source.Type->target.Type",
			Message: "first line\nsecond line",
		})

		assert.Equal(t, "fatal: source.Type->target.Type\n  first line\n  second line", got)
	})
}

func TestFormatDiagnostics(t *testing.T) {
	got := FormatDiagnostics([]Diagnostic{
		{
			Level:   DiagnosticLevelFatal,
			Path:    "source.Type->target.Type",
			Message: "fatal message",
		},
		{
			Level:   DiagnosticLevelWarning,
			Path:    "source.Type->target.Type :: target field Extra",
			Message: "warning message",
		},
	})

	assert.Equal(t, "fatal: source.Type->target.Type\n  fatal message\n\nwarning: source.Type->target.Type :: target field Extra\n  warning message", got)
	assert.Empty(t, FormatDiagnostics(nil))
}

func TestFormatDiagnosticsHeader(t *testing.T) {
	t.Run("formats fatal and warning counts", func(t *testing.T) {
		got := FormatDiagnosticsHeader([]Diagnostic{
			{Level: DiagnosticLevelFatal},
			{Level: DiagnosticLevelFatal},
			{Level: DiagnosticLevelWarning},
		})

		assert.Equal(t, "Morph found 2 fatal diagnostics and 1 warning\nCode generation will not continue\n", got)
	})

	t.Run("formats warning-only counts", func(t *testing.T) {
		got := FormatDiagnosticsHeader([]Diagnostic{
			{Level: DiagnosticLevelWarning},
			{Level: DiagnosticLevelWarning},
		})

		assert.Equal(t, "Morph found 2 warnings\n", got)
	})

	t.Run("formats unknown counts", func(t *testing.T) {
		got := FormatDiagnosticsHeader([]Diagnostic{
			{Level: DiagnosticLevelFatal},
			{Level: DiagnosticLevel(99)},
		})

		assert.Equal(t, "Morph found 1 fatal diagnostic and 1 other diagnostic\nCode generation will not continue\n", got)
	})

	t.Run("formats empty diagnostics as empty", func(t *testing.T) {
		assert.Empty(t, FormatDiagnosticsHeader(nil))
	})
}
