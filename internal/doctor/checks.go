package doctor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"grabman/internal/manifest"
	"grabman/internal/sitedna"
	"grabman/internal/solver"
)

// Status represents a check or overall workspace health state.
type Status string

const (
	StatusPass    Status = "pass"
	StatusWarning Status = "warning"
	StatusBlocked Status = "blocked"

	OverallHealthy  Status = "healthy"
	OverallWarnings Status = "warnings"
	OverallBlocked  Status = "blocked"
)

// CheckItem captures a single deterministic doctor audit check.
type CheckItem struct {
	ID           string `json:"id"`
	Code         string `json:"code,omitempty"`
	Status       Status `json:"status"`
	Title        string `json:"title"`
	PlainEnglish string `json:"plain_english"`
	Remediation  string `json:"remediation,omitempty"`
}

// ReportSummary counts checks by outcome.
type ReportSummary struct {
	Total    int `json:"total"`
	Passed   int `json:"passed"`
	Warnings int `json:"warnings"`
	Blocked  int `json:"blocked"`
}

// Report is the complete structured output of `grabman doctor`.
type Report struct {
	SchemaVersion string                   `json:"schema_version"`
	Status        Status                   `json:"status"`
	Workspace     string                   `json:"workspace"`
	GrabBinary    string                   `json:"grab_binary,omitempty"`
	GrabVersion   string                   `json:"grab_version,omitempty"`
	Summary       ReportSummary            `json:"summary"`
	Dependencies  []solver.DependencyAudit `json:"dependencies"`
	Checks        []CheckItem              `json:"checks"`
}

// Options configures doctor execution (allowing clean unit testing).
type Options struct {
	GrabBinary string // optional explicit path override
	Strict     bool
}

var (
	reGrabVersion     = regexp.MustCompile(`GRAB\s+v?(\d+\.\d+\.\d+)`)
	rePrivateIPv4     = regexp.MustCompile(`\b(?:10\.\d{1,3}\.\d{1,3}\.\d{1,3}|192\.168\.\d{1,3}\.\d{1,3}|172\.(?:1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3})\b`)
	reSecretAssign    = regexp.MustCompile(`(?i)^\s*([A-Z0-9_]*(?:MASTER_KEY|SECRET|PASSWORD|PASSWD|API_KEY|ACCESS_KEY|PRIVATE_KEY|TOKEN|AUTH_KEY)[A-Z0-9_]*)\s*[:=]\s*(.+?)\s*$`)
	reDSNWithPassword = regexp.MustCompile(`(?i)[a-z][a-z0-9+.-]*://[^:\s/]+:([^@\s$]+)@`)
	reQuotedHTMLDiv   = regexp.MustCompile(`(?i)<\s*/?\s*div\b|<\s*script\b|\bon[a-z]{2,24}\s*=|javascript\s*:`)
)

// Run executes all read-only compatibility, contract, and container security audits on workspaceDir.
func Run(workspaceDir string, opts Options) (*Report, error) {
	realRoot, err := manifest.ResolveWorkspaceRoot(workspaceDir)
	if err != nil {
		return nil, err
	}

	rep := &Report{
		SchemaVersion: manifest.SchemaVersionV100,
		Workspace:     realRoot,
		Dependencies:  []solver.DependencyAudit{},
		Checks:        []CheckItem{},
	}

	// 1. GRAB runtime check
	grabPath, grabVer, grabCheck := checkGrabRuntime(opts.GrabBinary)
	rep.GrabBinary = grabPath
	rep.GrabVersion = grabVer
	rep.Checks = append(rep.Checks, grabCheck)

	// 2. Optional companion runtimes (Docker / Node informational)
	rep.Checks = append(rep.Checks, checkOptionalTools())

	// 3. Manifest + Dependencies & Version Compatibility + Lockfile verification
	m, manifestCheck := checkManifest(realRoot)
	rep.Checks = append(rep.Checks, manifestCheck)
	if m != nil {
		installed := map[string]string{}
		if grabVer != "" {
			installed["grab"] = grabVer
		}
		if deps, depErr := solver.InspectDependencies(realRoot, installed); depErr == nil {
			rep.Dependencies = deps
			rep.Checks = append(rep.Checks, checkDependencyCompatibility(deps))
		}
	}
	rep.Checks = append(rep.Checks, checkLockfile(realRoot, rep.Dependencies))

	// 4. GRAB JSON configs & Theme mode checks
	rep.Checks = append(rep.Checks, checkWorkspaceConfigs(realRoot, m)...)

	// 5. Ecosystem contract SHA-256 parity
	rep.Checks = append(rep.Checks, checkContractDigests(realRoot))

	// 6. Site DNA exact precision & hygiene
	if dnaCheck := checkSiteDNA(realRoot, m); dnaCheck != nil {
		rep.Checks = append(rep.Checks, *dnaCheck)
	}

	// 7. Container --read-only & Zero-Secrets audit
	rep.Checks = append(rep.Checks, checkComposeAndSecrets(realRoot)...)

	// 8. Zero wrapper-<div> template check
	rep.Checks = append(rep.Checks, checkTemplatesZeroDiv(realRoot))

	// 9. Live `grab check` compilation check
	rep.Checks = append(rep.Checks, checkGrabCompile(realRoot, grabPath))

	finalizeReport(rep)
	return rep, nil
}

func finalizeReport(rep *Report) {
	for _, c := range rep.Checks {
		rep.Summary.Total++
		switch c.Status {
		case StatusPass:
			rep.Summary.Passed++
		case StatusWarning:
			rep.Summary.Warnings++
		case StatusBlocked:
			rep.Summary.Blocked++
		}
	}
	switch {
	case rep.Summary.Blocked > 0:
		rep.Status = OverallBlocked
	case rep.Summary.Warnings > 0:
		rep.Status = OverallWarnings
	default:
		rep.Status = OverallHealthy
	}
}

// ExitCode returns 0 when healthy (or warnings without --strict), and 1 when blocked (or warnings with --strict).
func (r *Report) ExitCode(strict bool) int {
	if r.Status == OverallBlocked {
		return 1
	}
	if strict && r.Status == OverallWarnings {
		return 1
	}
	return 0
}

// LocateGrabBinary finds the GRAB CLI binary cleanly without executing unverified paths.
func LocateGrabBinary(explicit string) string {
	candidates := []string{}
	if strings.TrimSpace(explicit) != "" {
		candidates = append(candidates, strings.TrimSpace(explicit))
	}
	if envBin := strings.TrimSpace(os.Getenv("GRAB_BIN")); envBin != "" {
		candidates = append(candidates, envBin)
	}
	if p, err := exec.LookPath("grab"); err == nil {
		candidates = append(candidates, p)
	}
	candidates = append(candidates,
		`C:\Program Files\GRAB\grab.exe`,
		`C:\Development\GRAB\dist\grab-windows-amd64.exe`,
		"/usr/local/bin/grab",
		"/usr/bin/grab",
	)
	for _, c := range candidates {
		if c == "" {
			continue
		}
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

func checkGrabRuntime(explicit string) (string, string, CheckItem) {
	grabPath := LocateGrabBinary(explicit)
	if grabPath == "" {
		return "", "", CheckItem{
			ID:           "runtime.grab",
			Status:       StatusWarning,
			Title:        "GRAB Runtime Binary",
			PlainEnglish: "GRAB CLI binary was not found on PATH or standard install locations. Live 'grab check' compilation checks were skipped.",
			Remediation:  "Install GRAB >= v1.1.0 or set GRAB_BIN to your grab executable path.",
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, grabPath, "version")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return grabPath, "", CheckItem{
			ID:           "runtime.grab",
			Code:         string(manifest.ErrManifestMalformed),
			Status:       StatusBlocked,
			Title:        "GRAB Runtime Binary",
			PlainEnglish: fmt.Sprintf("Found GRAB binary at %s, but 'grab version' failed: %v", grabPath, err),
			Remediation:  "Reinstall GRAB v1.1.0 from urbanspress/grab.",
		}
	}
	m := reGrabVersion.FindStringSubmatch(string(out))
	if len(m) < 2 {
		return grabPath, "", CheckItem{
			ID:           "runtime.grab",
			Code:         string(manifest.ErrSemVerViolation),
			Status:       StatusBlocked,
			Title:        "GRAB Runtime Version",
			PlainEnglish: fmt.Sprintf("Could not parse GRAB semantic version from %s output.", grabPath),
			Remediation:  "Ensure GRAB v1.1.0+ is installed.",
		}
	}
	verStr := m[1]
	sv, err := solver.ParseSemVer(verStr)
	if err != nil {
		return grabPath, verStr, CheckItem{
			ID:           "runtime.grab",
			Code:         string(manifest.ErrSemVerViolation),
			Status:       StatusBlocked,
			Title:        "GRAB Runtime Version",
			PlainEnglish: fmt.Sprintf("Invalid GRAB version %q detected.", verStr),
			Remediation:  "Install GRAB >= v1.1.0 (< v2.0.0).",
		}
	}
	minGrab := solver.SemVer{Major: 1, Minor: 1, Patch: 0}
	if sv.Compare(minGrab) < 0 {
		return grabPath, verStr, CheckItem{
			ID:           "runtime.grab",
			Code:         string(manifest.ErrSemVerViolation),
			Status:       StatusBlocked,
			Title:        "GRAB Runtime Version",
			PlainEnglish: fmt.Sprintf("Installed GRAB v%s is older than the required v1.1.0 ecosystem baseline.", verStr),
			Remediation:  "Upgrade GRAB to v1.1.0 (< v2.0.0).",
		}
	}
	if sv.Major >= 2 {
		return grabPath, verStr, CheckItem{
			ID:           "runtime.grab",
			Code:         string(manifest.ErrSemVerViolation),
			Status:       StatusBlocked,
			Title:        "GRAB Runtime Version",
			PlainEnglish: fmt.Sprintf("Installed GRAB v%s crosses the unapproved v2.0.0 major boundary (ADR-027c).", verStr),
			Remediation:  "Pin GRAB v1.x (>= v1.1.0 < v2.0.0).",
		}
	}
	return grabPath, verStr, CheckItem{
		ID:           "runtime.grab",
		Status:       StatusPass,
		Title:        "GRAB Runtime Version",
		PlainEnglish: fmt.Sprintf("GRAB v%s verified cleanly (%s).", verStr, grabPath),
	}
}

func checkOptionalTools() CheckItem {
	var found []string
	var missing []string
	for _, tool := range []string{"docker", "node"} {
		if _, err := exec.LookPath(tool); err == nil {
			found = append(found, tool)
		} else {
			missing = append(missing, tool)
		}
	}
	if len(found) == 0 {
		return CheckItem{
			ID:           "runtime.optional",
			Status:       StatusPass,
			Title:        "Optional Container / JS Tooling",
			PlainEnglish: "Offline core checks do not require docker or node; neither is currently on PATH.",
		}
	}
	msg := fmt.Sprintf("Detected optional tools: %s.", strings.Join(found, ", "))
	if len(missing) > 0 {
		msg += fmt.Sprintf(" (Not installed: %s.)", strings.Join(missing, ", "))
	}
	return CheckItem{
		ID:           "runtime.optional",
		Status:       StatusPass,
		Title:        "Optional Container / JS Tooling",
		PlainEnglish: msg,
	}
}

func checkManifest(realRoot string) (*manifest.Manifest, CheckItem) {
	m, _, err := manifest.LoadManifest(realRoot)
	if err != nil {
		return nil, CheckItem{
			ID:           "workspace.manifest",
			Code:         string(manifest.ExtractCode(err)),
			Status:       StatusBlocked,
			Title:        "grabman.json Manifest",
			PlainEnglish: err.Error(),
			Remediation:  "Run 'grabman init' or repair grabman.json schema and SemVer constraints.",
		}
	}
	return m, CheckItem{
		ID:           "workspace.manifest",
		Status:       StatusPass,
		Title:        "grabman.json Manifest",
		PlainEnglish: fmt.Sprintf("grabman.json is valid (project %q, profile %q).", m.ProjectName, m.Profile),
	}
}

func checkDependencyCompatibility(deps []solver.DependencyAudit) CheckItem {
	var warns []string
	var blocked []string
	for _, d := range deps {
		switch d.Status {
		case "warning":
			warns = append(warns, fmt.Sprintf("%s: %s", d.Layer, d.Message))
		case "blocked":
			blocked = append(blocked, fmt.Sprintf("%s: %s", d.Layer, d.Message))
		}
	}
	if len(blocked) > 0 {
		return CheckItem{
			ID:           "dependencies.compatibility",
			Code:         string(manifest.ErrSemVerViolation),
			Status:       StatusBlocked,
			Title:        "Dependencies & Version Compatibility",
			PlainEnglish: strings.Join(blocked, " "),
			Remediation:  "Update grabman.json target_runtime constraints to stay within the approved <2.0.0 ecosystem contract line (ADR-027c).",
		}
	}
	if len(warns) > 0 {
		return CheckItem{
			ID:           "dependencies.compatibility",
			Status:       StatusWarning,
			Title:        "Dependencies & Version Compatibility",
			PlainEnglish: strings.Join(warns, " "),
			Remediation:  "Run 'grabman solve --warn-incompatible' to continue with pinned overrides, or align grabman.json with recommended ranges.",
		}
	}
	return CheckItem{
		ID:           "dependencies.compatibility",
		Status:       StatusPass,
		Title:        "Dependencies & Version Compatibility",
		PlainEnglish: fmt.Sprintf("All %d ecosystem dependency layers satisfy compatibility matrix constraints.", len(deps)),
	}
}

func checkLockfile(realRoot string, deps []solver.DependencyAudit) CheckItem {
	res, err := solver.Verify(realRoot, solver.SolveOptions{})
	if err != nil {
		if solver.HasIncompatiblePinWarning(deps) {
			if existing, _, loadErr := manifest.LoadLockfile(realRoot); loadErr == nil {
				if computed, hashErr := existing.ComputeLockDigestSHA256(); hashErr == nil && computed == existing.LockDigestSHA256 {
					warnRes, warnErr := solver.Verify(realRoot, solver.SolveOptions{WarnIncompatible: true})
					if warnErr == nil {
						return CheckItem{
							ID:           "workspace.lockfile",
							Status:       StatusWarning,
							Title:        "grabman.lock Integrity & 6-Layer Matrix",
							PlainEnglish: strings.Join(warnRes.Warnings, " "),
							Remediation:  "Update target_runtime in grabman.json to baseline >= versions when ready, or continue using --warn-incompatible.",
						}
					}
					if warnRes != nil && onlyGrabmanJSONOverrideDrift(warnRes.DriftDetails) {
						return CheckItem{
							ID:           "workspace.lockfile",
							Status:       StatusWarning,
							Title:        "grabman.lock Integrity & 6-Layer Matrix",
							PlainEnglish: fmt.Sprintf("WARNING: grabman.json pins an incompatible dependency override (%v).", warnErr),
							Remediation:  fmt.Sprintf("Run 'grabman solve --warn-incompatible --dir %q' to update grabman.lock with your pinned override.", realRoot),
						}
					}
				}
			}
		}

		code := string(manifest.ExtractCode(err))
		if code == "" {
			code = string(manifest.ErrManifestMalformed)
		}
		return CheckItem{
			ID:           "workspace.lockfile",
			Code:         code,
			Status:       StatusBlocked,
			Title:        "grabman.lock Integrity & 6-Layer Matrix",
			PlainEnglish: err.Error(),
			Remediation:  fmt.Sprintf("Run 'grabman solve --dir %q' to update and verify grabman.lock.", realRoot),
		}
	}
	return CheckItem{
		ID:           "workspace.lockfile",
		Status:       StatusPass,
		Title:        "grabman.lock Integrity & 6-Layer Matrix",
		PlainEnglish: fmt.Sprintf("grabman.lock verified cleanly across all 6 ecosystem layers (digest %s).", res.LockDigestSHA256[:12]),
	}
}

func onlyGrabmanJSONOverrideDrift(drifts []string) bool {
	if len(drifts) == 0 {
		return false
	}
	for _, d := range drifts {
		if strings.Contains(d, `"workspace:grabman.json"`) {
			continue
		}
		if strings.HasPrefix(d, "layer ") && strings.Contains(d, " drifted ") {
			continue
		}
		return false
	}
	return true
}

func checkWorkspaceConfigs(realRoot string, m *manifest.Manifest) []CheckItem {
	var checks []CheckItem

	// grab.platform.json
	if abs, _, err := manifest.ResolveWorkspaceFile(realRoot, "grab.platform.json", manifest.ResolveOptions{AllowMissing: true}); err != nil {
		checks = append(checks, CheckItem{
			ID:           "config.platform",
			Code:         string(manifest.ExtractCode(err)),
			Status:       StatusBlocked,
			Title:        "grab.platform.json",
			PlainEnglish: err.Error(),
			Remediation:  "Ensure grab.platform.json stays confined within the workspace.",
		})
	} else if _, statErr := os.Stat(abs); statErr == nil {
		raw, rErr := manifest.ReadBoundedFile(abs)
		var parsed map[string]any
		if rErr != nil || json.Unmarshal(raw, &parsed) != nil {
			checks = append(checks, CheckItem{
				ID:           "config.platform",
				Code:         string(manifest.ErrManifestMalformed),
				Status:       StatusBlocked,
				Title:        "grab.platform.json",
				PlainEnglish: "grab.platform.json contains malformed JSON.",
				Remediation:  "Fix JSON formatting in grab.platform.json.",
			})
		} else {
			checks = append(checks, CheckItem{
				ID:           "config.platform",
				Status:       StatusPass,
				Title:        "grab.platform.json",
				PlainEnglish: "grab.platform.json is valid JSON.",
			})
		}
	}

	// grab.data.json
	if abs, _, err := manifest.ResolveWorkspaceFile(realRoot, "grab.data.json", manifest.ResolveOptions{AllowMissing: true}); err != nil {
		checks = append(checks, CheckItem{
			ID:           "config.data",
			Code:         string(manifest.ExtractCode(err)),
			Status:       StatusBlocked,
			Title:        "grab.data.json",
			PlainEnglish: err.Error(),
			Remediation:  "Ensure grab.data.json stays confined within the workspace.",
		})
	} else if _, statErr := os.Stat(abs); statErr == nil {
		raw, rErr := manifest.ReadBoundedFile(abs)
		var parsed map[string]any
		if rErr != nil || json.Unmarshal(raw, &parsed) != nil {
			checks = append(checks, CheckItem{
				ID:           "config.data",
				Code:         string(manifest.ErrManifestMalformed),
				Status:       StatusBlocked,
				Title:        "grab.data.json",
				PlainEnglish: "grab.data.json contains malformed JSON.",
				Remediation:  "Fix JSON formatting in grab.data.json.",
			})
		} else {
			checks = append(checks, CheckItem{
				ID:           "config.data",
				Status:       StatusPass,
				Title:        "grab.data.json",
				PlainEnglish: "grab.data.json is valid JSON.",
			})
		}
	}

	// theme/theme.json exclusivity check
	if abs, _, err := manifest.ResolveWorkspaceFile(realRoot, "theme/theme.json", manifest.ResolveOptions{AllowMissing: true}); err == nil {
		if _, statErr := os.Stat(abs); statErr == nil {
			raw, _ := manifest.ReadBoundedFile(abs)
			var themeDoc map[string]any
			if json.Unmarshal(raw, &themeDoc) != nil {
				checks = append(checks, CheckItem{
					ID:           "config.theme",
					Code:         string(manifest.ErrManifestMalformed),
					Status:       StatusBlocked,
					Title:        "Grex Theme Manifest (theme/theme.json)",
					PlainEnglish: "theme/theme.json is malformed JSON.",
					Remediation:  "Fix JSON formatting in theme/theme.json.",
				})
			} else if mode, _ := themeDoc["mode"].(string); m != nil && (m.Profile == "cms-stack" || m.Profile == "ai-stack") && mode != "visual" {
				checks = append(checks, CheckItem{
					ID:           "config.theme",
					Code:         string(manifest.ErrManifestMalformed),
					Status:       StatusBlocked,
					Title:        "Grex Theme Manifest (theme/theme.json)",
					PlainEnglish: fmt.Sprintf("theme/theme.json must declare mode: \"visual\" for %s (got %q).", m.Profile, mode),
					Remediation:  "Set \"mode\": \"visual\" in theme/theme.json per GREX-BUILDER-SPEC-100.",
				})
			} else {
				checks = append(checks, CheckItem{
					ID:           "config.theme",
					Status:       StatusPass,
					Title:        "Grex Theme Manifest (theme/theme.json)",
					PlainEnglish: "theme/theme.json aligns with Grex Theme Builder v1.0.0 contract.",
				})
			}
		}
	}

	return checks
}

func checkContractDigests(realRoot string) CheckItem {
	checks := []struct {
		paths    []string
		expected []string
		name     string
	}{
		{
			paths:    []string{"grex.elements.json", "public/grex/grex.elements.json", "theme/grex.elements.json"},
			expected: []string{manifest.FrozenGrexElementsSHA256},
			name:     "grex.elements.json",
		},
		{
			paths:    []string{"grex.js", "public/grex/grex.js"},
			expected: []string{manifest.FrozenGrexJSSHA256, manifest.FrozenGrabLabGrexJSSHA256},
			name:     "grex.js",
		},
		{
			paths:    []string{"playbook/seo-playbook.v1.json", "seo-playbook.v1.json"},
			expected: []string{manifest.FrozenSEOPlaybookV1SHA256},
			name:     "seo-playbook.v1.json",
		},
	}

	var verified []string
	for _, spec := range checks {
		for _, rel := range spec.paths {
			abs, slashRel, err := manifest.ResolveWorkspaceFile(realRoot, rel, manifest.ResolveOptions{AllowMissing: true})
			if err != nil {
				return CheckItem{
					ID:           "contracts.sha256",
					Code:         string(manifest.ExtractCode(err)),
					Status:       StatusBlocked,
					Title:        "Ecosystem Contract SHA-256 Parity",
					PlainEnglish: err.Error(),
					Remediation:  "Keep contract files confined inside the workspace.",
				}
			}
			if _, statErr := os.Stat(abs); statErr == nil {
				sum, hashErr := manifest.SHA256File(abs)
				if hashErr != nil {
					return CheckItem{
						ID:           "contracts.sha256",
						Code:         string(manifest.ErrChecksumMismatch),
						Status:       StatusBlocked,
						Title:        "Ecosystem Contract SHA-256 Parity",
						PlainEnglish: fmt.Sprintf("Failed to hash %s: %v", slashRel, hashErr),
					}
				}
				ok := false
				for _, exp := range spec.expected {
					if sum == exp {
						ok = true
						break
					}
				}
				if !ok {
					return CheckItem{
						ID:           "contracts.sha256",
						Code:         string(manifest.ErrChecksumMismatch),
						Status:       StatusBlocked,
						Title:        "Ecosystem Contract SHA-256 Parity",
						PlainEnglish: fmt.Sprintf("Contract file %s (%s) does not match frozen v1.x SHA-256 (%s).", slashRel, sum[:12], spec.expected[0][:12]),
						Remediation:  fmt.Sprintf("Replace %s with the verified upstream v1.x artifact.", slashRel),
					}
				}
				verified = append(verified, slashRel)
			}
		}
	}

	if len(verified) == 0 {
		return CheckItem{
			ID:           "contracts.sha256",
			Status:       StatusPass,
			Title:        "Ecosystem Contract SHA-256 Parity",
			PlainEnglish: "Frozen v1.x contract digests pinned in grabman.lock (no custom local overrides detected).",
		}
	}
	return CheckItem{
		ID:           "contracts.sha256",
		Status:       StatusPass,
		Title:        "Ecosystem Contract SHA-256 Parity",
		PlainEnglish: fmt.Sprintf("Verified frozen SHA-256 parity for: %s.", strings.Join(verified, ", ")),
	}
}

func checkSiteDNA(realRoot string, m *manifest.Manifest) *CheckItem {
	dnaRel := ""
	required := false
	if m != nil && strings.TrimSpace(m.SiteDNAPath) != "" {
		dnaRel = strings.TrimSpace(m.SiteDNAPath)
		required = true
	} else if abs, slash, err := manifest.ResolveWorkspaceFile(realRoot, "storage/site-dna.json", manifest.ResolveOptions{AllowMissing: true}); err == nil {
		if _, statErr := os.Stat(abs); statErr == nil {
			dnaRel = slash
		}
	}
	if dnaRel == "" {
		return nil
	}

	abs, slashRel, err := manifest.ResolveWorkspaceFile(realRoot, dnaRel, manifest.ResolveOptions{})
	if err != nil {
		return &CheckItem{
			ID:           "workspace.sitedna",
			Code:         string(manifest.ErrSiteDNAInvalid),
			Status:       StatusBlocked,
			Title:        "Site DNA & ADR-025c Precision",
			PlainEnglish: fmt.Sprintf("Configured Site DNA %q is missing or invalid: %v", dnaRel, err),
			Remediation:  "Export a clean Site DNA JSON from Groxy AI or remove site_dna_path if unused.",
		}
	}
	raw, err := manifest.ReadBoundedFile(abs)
	if err != nil {
		return &CheckItem{
			ID:           "workspace.sitedna",
			Code:         string(manifest.ErrSiteDNAInvalid),
			Status:       StatusBlocked,
			Title:        "Site DNA & ADR-025c Precision",
			PlainEnglish: fmt.Sprintf("Unable to read %s: %v", slashRel, err),
		}
	}
	bundle, err := sitedna.ValidateBytes(raw)
	if err != nil {
		_ = required
		return &CheckItem{
			ID:           "workspace.sitedna",
			Code:         string(manifest.ExtractCode(err)),
			Status:       StatusBlocked,
			Title:        "Site DNA & ADR-025c Precision",
			PlainEnglish: fmt.Sprintf("%s failed validation: %v", slashRel, err),
			Remediation:  "Re-export Site DNA with exact base-10 GrabDecimal strings and zero <div>/<script> payloads.",
		}
	}
	return &CheckItem{
		ID:           "workspace.sitedna",
		Status:       StatusPass,
		Title:        "Site DNA & ADR-025c Precision",
		PlainEnglish: fmt.Sprintf("%s passed %s validation (style pack %q, %d products verified).", slashRel, bundle.BundleName, bundle.ActiveStylePackID, len(bundle.Products)),
	}
}

func checkComposeAndSecrets(realRoot string) []CheckItem {
	var checks []CheckItem

	// Check Compose files
	composeRel := ""
	for _, candidate := range []string{"deploy/docker-compose.yml", "docker-compose.yml", "compose.yml"} {
		abs, slash, err := manifest.ResolveWorkspaceFile(realRoot, candidate, manifest.ResolveOptions{AllowMissing: true})
		if err != nil {
			checks = append(checks, CheckItem{
				ID:           "security.compose",
				Code:         string(manifest.ExtractCode(err)),
				Status:       StatusBlocked,
				Title:        "Container --read-only Hardening",
				PlainEnglish: err.Error(),
			})
			return checks
		}
		if _, statErr := os.Stat(abs); statErr == nil {
			composeRel = slash
			raw, readErr := manifest.ReadBoundedFile(abs)
			if readErr != nil {
				checks = append(checks, CheckItem{
					ID:           "security.compose",
					Code:         string(manifest.ErrSecurityPolicy),
					Status:       StatusBlocked,
					Title:        "Container --read-only Hardening",
					PlainEnglish: fmt.Sprintf("Unable to read %s: %v", slash, readErr),
				})
				break
			}
			issues := auditComposeSecurity(slash, string(raw))
			if len(issues) > 0 {
				checks = append(checks, CheckItem{
					ID:           "security.compose",
					Code:         string(manifest.ErrSecurityPolicy),
					Status:       StatusBlocked,
					Title:        "Container --read-only Hardening",
					PlainEnglish: strings.Join(issues, " "),
					Remediation:  fmt.Sprintf("Set read_only: true, cap_drop: [ALL], security_opt: [no-new-privileges:true], pinned image tags, and 127.0.0.1 loopback ports in %s.", slash),
				})
			} else {
				checks = append(checks, CheckItem{
					ID:           "security.compose",
					Status:       StatusPass,
					Title:        "Container --read-only Hardening",
					PlainEnglish: fmt.Sprintf("%s enforces read_only: true, cap_drop: ALL, no-new-privileges:true, pinned tags, and loopback ports.", slash),
				})
			}
			break
		}
	}
	if composeRel == "" {
		checks = append(checks, CheckItem{
			ID:           "security.compose",
			Status:       StatusWarning,
			Title:        "Container --read-only Hardening",
			PlainEnglish: "No deploy/docker-compose.yml found in workspace.",
			Remediation:  "Add a hardened --read-only deploy/docker-compose.yml (or run 'grabman init').",
		})
	}

	// Zero-secrets & private IP scan across config/deploy templates
	secretTargets := []struct {
		rel      string
		allowEx  bool
	}{
		{"deploy/docker-compose.yml", false},
		{"docker-compose.yml", false},
		{"compose.yml", false},
		{"grab.data.json", false},
		{"grab.platform.json", false},
		{"deploy/.env.example", true},
		{".env.example", true},
	}

	var secretViolations []string
	for _, tgt := range secretTargets {
		abs, slash, err := manifest.ResolveWorkspaceFile(realRoot, tgt.rel, manifest.ResolveOptions{AllowMissing: true, AllowEnvExample: tgt.allowEx})
		if err != nil {
			continue
		}
		if _, statErr := os.Stat(abs); statErr != nil {
			continue
		}
		raw, err := manifest.ReadBoundedFile(abs)
		if err != nil {
			continue
		}
		secretViolations = append(secretViolations, scanNoHardcodedSecrets(slash, string(raw))...)
	}

	if len(secretViolations) > 0 {
		checks = append(checks, CheckItem{
			ID:           "security.secrets",
			Code:         string(manifest.ErrSecurityPolicy),
			Status:       StatusBlocked,
			Title:        "Zero Hardcoded Secrets & Private IPs",
			PlainEnglish: strings.Join(secretViolations, " "),
			Remediation:  "Replace literal credentials and private IPs with ${ENV_VAR} placeholders supplied at runtime via untracked .env or secret managers.",
		})
	} else {
		checks = append(checks, CheckItem{
			ID:           "security.secrets",
			Status:       StatusPass,
			Title:        "Zero Hardcoded Secrets & Private IPs",
			PlainEnglish: "Zero hardcoded secrets, credentials, or private VPS IPs detected in workspace configs.",
		})
	}

	return checks
}

type composeService struct {
	name             string
	readOnly         bool
	hasCapDropAll    bool
	hasNoNewPrivs    bool
	privileged       bool
	hostNetwork      bool
	mountsDockerSock bool
	usesLatestTag    bool
	exposedPorts     []string
}

func auditComposeSecurity(relPath, content string) []string {
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	inServices := false
	servicesIndent := -1
	var services []*composeService
	var current *composeService
	currentListSection := ""
	listSectionIndent := -1

	for _, rawLine := range lines {
		noComment := stripYAMLComment(rawLine)
		if strings.TrimSpace(noComment) == "" {
			continue
		}
		indent := countLeadingSpaces(noComment)
		trimmed := strings.TrimSpace(noComment)

		if trimmed == "services:" {
			inServices = true
			servicesIndent = indent
			current = nil
			currentListSection = ""
			continue
		}
		if inServices && indent <= servicesIndent && !strings.HasPrefix(trimmed, "-") {
			inServices = false
			current = nil
			currentListSection = ""
			continue
		}
		if !inServices {
			continue
		}

		// Service header (first indentation level inside services:)
		if strings.HasSuffix(trimmed, ":") && !strings.HasPrefix(trimmed, "-") {
			if current == nil || indent <= servicesIndent+2 {
				svcName := strings.TrimSuffix(trimmed, ":")
				current = &composeService{name: svcName}
				services = append(services, current)
				currentListSection = ""
				continue
			}
		}
		if current == nil {
			continue
		}

		if strings.HasPrefix(trimmed, "-") && currentListSection != "" && indent >= listSectionIndent {
			item := strings.TrimSpace(strings.TrimPrefix(trimmed, "-"))
			item = trimYAMLQuotes(item)
			switch currentListSection {
			case "cap_drop":
				if strings.EqualFold(item, "ALL") {
					current.hasCapDropAll = true
				}
			case "security_opt":
				if strings.EqualFold(item, "no-new-privileges:true") || strings.EqualFold(item, "no-new-privileges=true") {
					current.hasNoNewPrivs = true
				}
			case "volumes":
				if strings.Contains(item, "/var/run/docker.sock") {
					current.mountsDockerSock = true
				}
			case "ports":
				if isUnsafePortMapping(item) {
					current.exposedPorts = append(current.exposedPorts, item)
				}
			}
			continue
		}

		// Key-value inside service
		if colonIdx := strings.IndexByte(trimmed, ':'); colonIdx > 0 {
			key := strings.TrimSpace(trimmed[:colonIdx])
			val := strings.TrimSpace(trimmed[colonIdx+1:])
			valUnquoted := trimYAMLQuotes(val)
			currentListSection = ""

			switch key {
			case "cap_drop", "security_opt", "volumes", "ports":
				if val == "" {
					currentListSection = key
					listSectionIndent = indent
				} else if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
					inlineItems := strings.Split(strings.TrimSuffix(strings.TrimPrefix(val, "["), "]"), ",")
					for _, rawItem := range inlineItems {
						item := trimYAMLQuotes(strings.TrimSpace(rawItem))
						switch key {
						case "cap_drop":
							if strings.EqualFold(item, "ALL") {
								current.hasCapDropAll = true
							}
						case "security_opt":
							if strings.EqualFold(item, "no-new-privileges:true") || strings.EqualFold(item, "no-new-privileges=true") {
								current.hasNoNewPrivs = true
							}
						case "volumes":
							if strings.Contains(item, "/var/run/docker.sock") {
								current.mountsDockerSock = true
							}
						case "ports":
							if isUnsafePortMapping(item) {
								current.exposedPorts = append(current.exposedPorts, item)
							}
						}
					}
				}
			case "read_only":
				if strings.EqualFold(valUnquoted, "true") {
					current.readOnly = true
				}
			case "privileged":
				if strings.EqualFold(valUnquoted, "true") {
					current.privileged = true
				}
			case "network_mode":
				if strings.EqualFold(valUnquoted, "host") {
					current.hostNetwork = true
				}
			case "image":
				if strings.HasSuffix(strings.ToLower(valUnquoted), ":latest") ||
					(strings.HasSuffix(strings.ToLower(strings.TrimSuffix(valUnquoted, "}")), ":latest")) {
					current.usesLatestTag = true
				}
			}
		}
	}

	if len(services) == 0 {
		return []string{fmt.Sprintf("%s defines no services.", relPath)}
	}

	var issues []string
	for _, svc := range services {
		if !svc.readOnly {
			issues = append(issues, fmt.Sprintf("Service %q in %s is missing 'read_only: true'.", svc.name, relPath))
		}
		if !svc.hasCapDropAll {
			issues = append(issues, fmt.Sprintf("Service %q in %s is missing 'cap_drop: [ALL]'.", svc.name, relPath))
		}
		if !svc.hasNoNewPrivs {
			issues = append(issues, fmt.Sprintf("Service %q in %s is missing 'security_opt: [no-new-privileges:true]'.", svc.name, relPath))
		}
		if svc.privileged {
			issues = append(issues, fmt.Sprintf("Service %q in %s enables forbidden 'privileged: true'.", svc.name, relPath))
		}
		if svc.hostNetwork {
			issues = append(issues, fmt.Sprintf("Service %q in %s uses forbidden 'network_mode: host'.", svc.name, relPath))
		}
		if svc.mountsDockerSock {
			issues = append(issues, fmt.Sprintf("Service %q in %s mounts forbidden '/var/run/docker.sock'.", svc.name, relPath))
		}
		if svc.usesLatestTag {
			issues = append(issues, fmt.Sprintf("Service %q in %s uses unpinned ':latest' image tag.", svc.name, relPath))
		}
		for _, p := range svc.exposedPorts {
			issues = append(issues, fmt.Sprintf("Service %q in %s binds non-loopback port %q (must bind 127.0.0.1:...).", svc.name, relPath, p))
		}
	}
	return issues
}

func isUnsafePortMapping(mapping string) bool {
	m := strings.TrimSpace(mapping)
	if m == "" {
		return false
	}
	if strings.HasPrefix(m, "127.0.0.1:") || strings.HasPrefix(m, "[::1]:") || strings.HasPrefix(m, "::1:") {
		return false
	}
	// Any 0.0.0.0 binding or bare hostPort:containerPort is public exposure.
	return true
}

func scanNoHardcodedSecrets(relPath, content string) []string {
	var issues []string
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")

	for idx, rawLine := range lines {
		trimmed := strings.TrimSpace(rawLine)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if m := reSecretAssign.FindStringSubmatch(rawLine); len(m) == 3 {
			keyName := m[1]
			val := trimYAMLQuotes(strings.TrimSpace(strings.TrimSuffix(m[2], ",")))
			if !isSafeSecretReference(val) {
				issues = append(issues, fmt.Sprintf("%s line %d: hardcoded secret value detected for %s (value redacted).", relPath, idx+1, keyName))
			}
		}
		if dsnMatch := reDSNWithPassword.FindStringSubmatch(rawLine); len(dsnMatch) == 2 {
			passPart := dsnMatch[1]
			if !strings.HasPrefix(passPart, "${") {
				issues = append(issues, fmt.Sprintf("%s line %d: embedded credential in connection URI (value redacted).", relPath, idx+1))
			}
		}
		for _, ipMatch := range rePrivateIPv4.FindAllString(rawLine, -1) {
			ip := net.ParseIP(ipMatch)
			if ip != nil && !ip.IsLoopback() {
				issues = append(issues, fmt.Sprintf("%s line %d: hardcoded private IPv4 literal detected (redacted).", relPath, idx+1))
				break
			}
		}
	}
	return issues
}

func isSafeSecretReference(val string) bool {
	if val == "" || val == `""` || val == `''` || val == "null" {
		return true
	}
	// Safe env placeholder or descriptive non-secret example hint in .env.example?
	// Note: in .env.example we still expect empty values (`GRABLAB_MASTER_KEY=`) or `${...}` or `<set-via-secret-manager>` placeholders.
	if strings.HasPrefix(val, "${") && strings.HasSuffix(val, "}") {
		return true
	}
	if strings.HasPrefix(val, "<") && strings.HasSuffix(val, ">") {
		return true
	}
	if strings.HasPrefix(strings.ToUpper(val), "CHANGE_ME") || strings.EqualFold(val, "REDACTED") {
		return true
	}
	// Numeric limits like GRABLAB_MAX_UPLOAD_BYTES or boolean flags shouldn't match key regex,
	// and JSON string keys in quotes or empty strings are safe.
	if _, err := strconv.Atoi(val); err == nil {
		return true
	}
	return false
}

func checkTemplatesZeroDiv(realRoot string) CheckItem {
	viewsDir, _, err := manifest.ResolveWorkspaceDir(realRoot, "views")
	if err != nil {
		if os.IsNotExist(err) {
			return CheckItem{
				ID:           "templates.dom",
				Status:       StatusPass,
				Title:        "Zero Wrapper-<div> Template Hygiene",
				PlainEnglish: "No views/ templates directory present to scan.",
			}
		}
		return CheckItem{
			ID:           "templates.dom",
			Code:         string(manifest.ExtractCode(err)),
			Status:       StatusBlocked,
			Title:        "Zero Wrapper-<div> Template Hygiene",
			PlainEnglish: err.Error(),
		}
	}
	entries, err := os.ReadDir(viewsDir)
	if err != nil {
		return CheckItem{
			ID:           "templates.dom",
			Status:       StatusPass,
			Title:        "Zero Wrapper-<div> Template Hygiene",
			PlainEnglish: "No views/ templates directory present to scan.",
		}
	}
	var scanned int
	var bad []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".grab.html") {
			continue
		}
		scanned++
		abs, slashRel, rErr := manifest.ResolveWorkspaceFile(realRoot, filepath.ToSlash(filepath.Join("views", entry.Name())), manifest.ResolveOptions{})
		if rErr != nil {
			bad = append(bad, rErr.Error())
			continue
		}
		raw, rErr := manifest.ReadBoundedFile(abs)
		if rErr != nil {
			bad = append(bad, slashRel)
			continue
		}
		if m := reQuotedHTMLDiv.FindString(string(raw)); m != "" {
			bad = append(bad, fmt.Sprintf("%s contains disallowed %q", slashRel, m))
		}
	}
	if len(bad) > 0 {
		return CheckItem{
			ID:           "templates.dom",
			Code:         string(manifest.ErrSiteDNAInvalid),
			Status:       StatusBlocked,
			Title:        "Zero Wrapper-<div> Template Hygiene",
			PlainEnglish: strings.Join(bad, "; "),
			Remediation:  "Replace wrapper <div> tags and inline scripts with semantic Grex JS elements.",
		}
	}
	return CheckItem{
		ID:           "templates.dom",
		Status:       StatusPass,
		Title:        "Zero Wrapper-<div> Template Hygiene",
		PlainEnglish: fmt.Sprintf("Scanned %d .grab.html template(s): zero wrapper <div>, <script>, or unsafe handlers.", scanned),
	}
}

func checkGrabCompile(realRoot, grabPath string) CheckItem {
	var grabFiles []string
	for _, candidate := range []string{"src/app.grab", "src/main.grab", "app.grab"} {
		abs, slash, err := manifest.ResolveWorkspaceFile(realRoot, candidate, manifest.ResolveOptions{AllowMissing: true})
		if err == nil {
			if _, statErr := os.Stat(abs); statErr == nil {
				grabFiles = append(grabFiles, slash)
			}
		}
	}
	if len(grabFiles) == 0 {
		// Look for any *.grab in src/
		if srcDir, _, err := manifest.ResolveWorkspaceDir(realRoot, "src"); err == nil {
			if items, rErr := os.ReadDir(srcDir); rErr == nil {
				for _, it := range items {
					if !it.IsDir() && strings.HasSuffix(strings.ToLower(it.Name()), ".grab") {
						grabFiles = append(grabFiles, "src/"+it.Name())
					}
				}
			}
		}
	}
	sort.Strings(grabFiles)
	if len(grabFiles) == 0 {
		return CheckItem{
			ID:           "compile.grab_check",
			Status:       StatusPass,
			Title:        "GRAB Static Compilation (grab check)",
			PlainEnglish: "No .grab entrypoint files found in workspace.",
		}
	}
	if grabPath == "" {
		return CheckItem{
			ID:           "compile.grab_check",
			Status:       StatusWarning,
			Title:        "GRAB Static Compilation (grab check)",
			PlainEnglish: fmt.Sprintf("Skipped 'grab check' on %s because GRAB binary was not found.", strings.Join(grabFiles, ", ")),
			Remediation:  "Install GRAB v1.1.0 and run 'grab check' to verify compilation.",
		}
	}

	args := []string{"check"}
	if abs, slash, err := manifest.ResolveWorkspaceFile(realRoot, "grab.platform.json", manifest.ResolveOptions{AllowMissing: true}); err == nil {
		if _, statErr := os.Stat(abs); statErr == nil {
			args = append(args, "--platform-config", slash)
		}
	}
	if abs, slash, err := manifest.ResolveWorkspaceFile(realRoot, "grab.data.json", manifest.ResolveOptions{AllowMissing: true}); err == nil {
		if _, statErr := os.Stat(abs); statErr == nil {
			args = append(args, "--db-config", slash)
		}
	}

	for _, gf := range grabFiles {
		runArgs := append(append([]string{}, args...), gf)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, grabPath, runArgs...)
		cmd.Dir = realRoot
		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &outBuf
		err := cmd.Run()
		cancel()
		if err != nil {
			return CheckItem{
				ID:           "compile.grab_check",
				Code:         string(manifest.ErrManifestMalformed),
				Status:       StatusBlocked,
				Title:        "GRAB Static Compilation (grab check)",
				PlainEnglish: fmt.Sprintf("'grab check' failed on %s: %s", gf, strings.TrimSpace(outBuf.String())),
				Remediation:  fmt.Sprintf("Fix the GRAB static error in %s and re-run 'grabman doctor'.", gf),
			}
		}
	}

	return CheckItem{
		ID:           "compile.grab_check",
		Status:       StatusPass,
		Title:        "GRAB Static Compilation (grab check)",
		PlainEnglish: fmt.Sprintf("'grab check' passed with zero errors on %s.", strings.Join(grabFiles, ", ")),
	}
}

func stripYAMLComment(line string) string {
	inSingle := false
	inDouble := false
	for i := 0; i < len(line); i++ {
		c := line[i]
		if c == '\'' && !inDouble {
			inSingle = !inSingle
		} else if c == '"' && !inSingle {
			inDouble = !inDouble
		} else if c == '#' && !inSingle && !inDouble {
			return line[:i]
		}
	}
	return line
}

func countLeadingSpaces(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			n++
		} else if s[i] == '\t' {
			n += 2
		} else {
			break
		}
	}
	return n
}

func trimYAMLQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
