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

	if len(r.Dependencies) > 0 {
		b.WriteString("\nDependencies & Version Compatibility:\n")
		for _, dep := range r.Dependencies {
			tag := "[OK]"
			switch dep.Status {
			case "warning":
				tag = "[WARN]"
			case "blocked":
				tag = "[BLOCKED]"
			}
			installedNote := ""
			if dep.Installed != "" {
				installedNote = fmt.Sprintf(", installed: v%s", dep.Installed)
			}
			lockedNote := ""
			if dep.Locked != "" {
				lockedNote = fmt.Sprintf(", locked: %s", dep.Locked)
			}
			b.WriteString(fmt.Sprintf("  %s %-13s configured: %-18s expected: %-16s%s%s\n", tag, dep.Layer, dep.Configured, dep.Expected, lockedNote, installedNote))
			if dep.Status != "compatible" {
				b.WriteString(fmt.Sprintf("                 %s\n", dep.Message))
			}
		}
	}
	return b.String()
}

// FormatJSON renders deterministic canonical JSON output for `grabman doctor --json`.
func FormatJSON(r *Report) ([]byte, error) {
	return manifest.CanonicalJSON(r)
}
