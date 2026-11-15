package doctor

import (
	"fmt"
	"strings"

	"grabman/internal/manifest"
)

// FormatPlainEnglish renders a non-jargon, actionable plain-English report for operators.
func FormatPlainEnglish(r *Report) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("grabman doctor — Workspace Health: %s\n", strings.ToUpper(string(r.Status))))
	b.WriteString(fmt.Sprintf("Workspace: %s\n", r.Workspace))
	if r.GrabVersion != "" {
		b.WriteString(fmt.Sprintf("GRAB CLI: v%s (%s)\n", r.GrabVersion, r.GrabBinary))
	}
	b.WriteString(fmt.Sprintf("Summary: %d passed, %d warning(s), %d blocked\n\n", r.Summary.Passed, r.Summary.Warnings, r.Summary.Blocked))

	for _, c := range r.Checks {
		tag := "[PASS]"
		switch c.Status {
		case StatusWarning:
			tag = "[WARN]"
		case StatusBlocked:
			tag = "[BLOCKED]"
		}
		if c.Code != "" {
			b.WriteString(fmt.Sprintf("%s %s (%s): %s\n", tag, c.Title, c.Code, c.PlainEnglish))
		} else {
			b.WriteString(fmt.Sprintf("%s %s: %s\n", tag, c.Title, c.PlainEnglish))
		}
		if c.Remediation != "" && c.Status != StatusPass {
			b.WriteString(fmt.Sprintf("        Fix: %s\n", c.Remediation))
		}
	}
	return b.String()
}

// FormatJSON renders deterministic canonical JSON output for `grabman doctor --json`.
func FormatJSON(r *Report) ([]byte, error) {
	return manifest.CanonicalJSON(r)
}
