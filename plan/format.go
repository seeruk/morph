package plan

import (
	"fmt"
	"strings"
)

// FormatDiagnostic returns a human-readable, multi-line representation of a diagnostic.
func FormatDiagnostic(diagnostic Diagnostic) string {
	var sb strings.Builder
	if diagnostic.Path == "" {
		fmt.Fprintf(&sb, "%s:", diagnostic.Level)
	} else {
		fmt.Fprintf(&sb, "%s: %s", diagnostic.Level, diagnostic.Path)
	}
	sb.WriteByte('\n')
	formatDiagnosticMessage(&sb, diagnostic.Message)
	return sb.String()
}

// FormatDiagnostics returns human-readable, multi-line diagnostics separated by blank lines.
func FormatDiagnostics(diagnostics []Diagnostic) string {
	if len(diagnostics) == 0 {
		return ""
	}

	out := make([]string, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		out = append(out, FormatDiagnostic(diagnostic))
	}

	return strings.Join(out, "\n\n")
}

// FormatDiagnosticsHeader returns a human-readable summary header for a set of diagnostics.
func FormatDiagnosticsHeader(diagnostics []Diagnostic) string {
	if len(diagnostics) == 0 {
		return ""
	}

	fatalCount := 0
	warningCount := 0
	unknownCount := 0
	for _, diagnostic := range diagnostics {
		switch diagnostic.Level {
		case DiagnosticLevelFatal:
			fatalCount++
		case DiagnosticLevelWarning:
			warningCount++
		default:
			unknownCount++
		}
	}

	parts := make([]string, 0, 3)
	if fatalCount > 0 {
		parts = append(parts, formatDiagnosticCount(fatalCount, "fatal diagnostic"))
	}
	if warningCount > 0 {
		parts = append(parts, formatDiagnosticCount(warningCount, "warning"))
	}
	if unknownCount > 0 {
		parts = append(parts, formatDiagnosticCount(unknownCount, "other diagnostic"))
	}

	header := "Morph found " + createCompoundSentence(parts)
	if fatalCount > 0 {
		header += "\nCode generation will not continue"
	}

	header += "\n"

	return header
}

// formatDiagnosticMessage formats the given diagnostic message, indenting each new line.
func formatDiagnosticMessage(sb *strings.Builder, message string) {
	sb.WriteString("  ")
	sb.WriteString(strings.ReplaceAll(message, "\n", "\n  "))
}

func formatDiagnosticCount(count int, label string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, label)
	}
	return fmt.Sprintf("%d %ss", count, label)
}

func createCompoundSentence(parts []string) string {
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	case 2:
		return parts[0] + " and " + parts[1]
	default:
		return strings.Join(parts[:len(parts)-1], ", ") + ", and " + parts[len(parts)-1]
	}
}
