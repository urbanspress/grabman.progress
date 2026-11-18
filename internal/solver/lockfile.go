package solver

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"grabman/internal/manifest"
	"grabman/internal/sitedna"
)

// LayerSpec describes an upstream/peer ecosystem layer in the 6-Layer Compatibility Matrix.
type LayerSpec struct {
	Key         string
	Repo        string
	Aliases     []string
	BaselineMin string
	DefaultExpr string
	SemVerTier  string
	Available   []string // ascending SemVer order
}

// EcosystemMatrix returns the frozen 6-layer compatibility matrix for Step 5 v1.0.0.
func EcosystemMatrix() []LayerSpec {
	return []LayerSpec{
		{
			Key:         "grab",
			Repo:        "urbanspress/grab",
			Aliases:     []string{"grab"},
			BaselineMin: "1.1.0",
			DefaultExpr: ">=1.1.0 <2.0.0",
			SemVerTier:  "v1.x",
			Available:   []string{"1.1.0"},
		},
		{
			Key:         "grex-js",
			Repo:        "urbanspress/grex-js",
			Aliases:     []string{"grex_js", "grex-js"},
			BaselineMin: "1.0.0",
			DefaultExpr: ">=1.0.0 <2.0.0",
			SemVerTier:  "v1.x",
			Available:   []string{"1.0.0"},
		},
		{
			Key:         "grablab-cms",
			Repo:        "urbanspress/grablab-cms",
			Aliases:     []string{"grablab_cms", "grablab-cms"},
			BaselineMin: "1.0.1",
			DefaultExpr: ">=1.0.1 <2.0.0",
			SemVerTier:  "v1.x",
			Available:   []string{"1.0.1", "1.1.0", "1.4.0", "1.4.1"},
		},
		{
			Key:         "grex-builder",
			Repo:        "urbanspress/grex-builder",
			Aliases:     []string{"grex_builder", "grex-builder"},
			BaselineMin: "1.0.0",
			DefaultExpr: ">=1.0.0 <2.0.0",
			SemVerTier:  "v1.x",
			Available:   []string{"1.0.0"},
		},
		{
			Key:         "groxy-ai",
			Repo:        "urbanspress/groxy-ai",
			Aliases:     []string{"groxy_ai", "groxy-ai"},
			BaselineMin: "1.0.0",
			DefaultExpr: ">=1.0.0 <2.0.0",
			SemVerTier:  "v1.x",
			Available:   []string{"1.0.0"},
		},
		{
			Key:         "step-5",
			Repo:        "urbanspress/grabman,urbanspress/grabtaculous,urbanspress/gashboard",
			Aliases:     []string{"grabman", "grabtaculous", "gashboard", "step_5", "step-5"},
			BaselineMin: "0.1.0",
			DefaultExpr: ">=0.1.0 <2.0.0",
			SemVerTier:  "1.x-frozen",
			Available:   []string{"0.1.0", "0.2.0", "0.3.0", "1.0.0", "1.1.0"},
		},
	}
}

// SolveOptions controls solver execution and deterministic timestamps.
type SolveOptions struct {
	FixedTimestamp   string
	Now              func() time.Time
	ExtraReleases    map[string][]string // optional override for testing multi-version resolution
	WarnIncompatible bool                // emit non-blocking warnings instead of failing when <2.0.0 pin falls below BaselineMin
}

// DependencyAudit reports compatibility status for a single ecosystem layer.
type DependencyAudit struct {
	Layer      string `json:"layer"`
	Repo       string `json:"repo"`
	Configured string `json:"configured"`
	Locked     string `json:"locked,omitempty"`
	Expected   string `json:"expected"`
	Installed  string `json:"installed,omitempty"`
	Compatible bool   `json:"compatible"`
	Status     string `json:"status"` // "compatible", "warning", "blocked"
	Message    string `json:"message"`
}

// VerifyResult captures the outcome of `grabman solve --verify`.
type VerifyResult struct {
	SchemaVersion    string   `json:"schema_version"`
	Verified         bool     `json:"verified"`
	Deterministic    bool     `json:"deterministic"`
	Workspace        string   `json:"workspace"`
	LockDigestSHA256 string   `json:"lock_digest_sha256"`
	Warnings         []string `json:"warnings,omitempty"`
	DriftDetails     []string `json:"drift_details,omitempty"`
	Code             string   `json:"code,omitempty"`
	Error            string   `json:"error,omitempty"`
	Remediation      string   `json:"remediation,omitempty"`
}

// SolveResult captures the outcome of `grabman solve`.
type SolveResult struct {
	SchemaVersion    string             `json:"schema_version"`
	Workspace        string             `json:"workspace"`
	LockfilePath     string             `json:"lockfile_path"`
	LockDigestSHA256 string             `json:"lock_digest_sha256"`
	Warnings         []string           `json:"warnings,omitempty"`
	Lockfile         *manifest.Lockfile `json:"lockfile"`
}

func resolveTimestamp(opts SolveOptions, existing *manifest.Lockfile, newDigest string) string {
	if strings.TrimSpace(opts.FixedTimestamp) != "" {
		return strings.TrimSpace(opts.FixedTimestamp)
	}
	if epochRaw := strings.TrimSpace(os.Getenv("SOURCE_DATE_EPOCH")); epochRaw != "" {
		if sec, err := strconv.ParseInt(epochRaw, 10, 64); err == nil {
			return time.Unix(sec, 0).UTC().Format(time.RFC3339)
		}
	}
	if existing != nil && existing.LockDigestSHA256 == newDigest && strings.TrimSpace(existing.SolvedAtUTC) != "" {
		return existing.SolvedAtUTC
	}
	if opts.Now != nil {
		return opts.Now().UTC().Format(time.RFC3339)
	}
	return time.Now().UTC().Format(time.RFC3339)
}

// ComputeLockfile resolves the 6-layer matrix and hashes workspace contracts purely read-only.
func ComputeLockfile(workspaceRoot string, opts SolveOptions) (*manifest.Lockfile, error) {
	lock, _, err := ComputeLockfileWithWarnings(workspaceRoot, opts)
	return lock, err
}

// ComputeLockfileWithWarnings resolves the 6-layer matrix and returns optional non-blocking
// compatibility warnings when opts.WarnIncompatible is enabled.
func ComputeLockfileWithWarnings(workspaceRoot string, opts SolveOptions) (*manifest.Lockfile, []string, error) {
	realRoot, err := manifest.ResolveWorkspaceRoot(workspaceRoot)
	if err != nil {
		return nil, nil, err
	}
	m, _, err := manifest.LoadManifest(realRoot)
	if err != nil {
		return nil, nil, err
	}

	knownAliases := map[string]struct{}{}
	matrix := EcosystemMatrix()
	for _, spec := range matrix {
		for _, a := range spec.Aliases {
			knownAliases[a] = struct{}{}
		}
	}
	for k := range m.TargetRuntime {
		if _, ok := knownAliases[k]; !ok {
			return nil, nil, manifest.NewError(
				manifest.ErrSemVerViolation,
				fmt.Sprintf("unknown layer %q in grabman.json target_runtime", k),
				"Use supported layer keys: grab, grex_js, grablab_cms, grex_builder, groxy_ai, grabtaculous, gashboard, grabman.",
			)
		}
	}

	var warnings []string
	compatibleAll := true
	layers := make(map[string]manifest.LockedLayer, len(matrix))
	for _, spec := range matrix {
		boundList, exprSummary := collectLayerConstraints(m.TargetRuntime, spec)
		var parsedBounds []ConstraintSet
		for _, expr := range boundList {
			cs, parseErr := ParseConstraint(expr)
			if parseErr != nil {
				return nil, nil, parseErr
			}
			parsedBounds = append(parsedBounds, cs)
		}

		minVer, err := ParseSemVer(spec.BaselineMin)
		if err != nil {
			return nil, nil, err
		}

		candidates := spec.Available
		if opts.ExtraReleases != nil && len(opts.ExtraReleases[spec.Key]) > 0 {
			candidates = opts.ExtraReleases[spec.Key]
		}

		chosen, err := selectBestCandidate(spec.Key, candidates, &minVer, parsedBounds)
		if err != nil {
			return nil, nil, err
		}

		if chosen == nil && opts.WarnIncompatible {
			fallback := resolveFallbackVersion(candidates, parsedBounds)
			if fallback != nil {
				chosen = fallback
				compatibleAll = false
				warnings = append(warnings, fmt.Sprintf(
					"WARNING: %s is pinned/constrained to %s (%q), expected %s (below ecosystem baseline >=%s <2.0.0). Continuing in non-blocking mode.",
					spec.Key, chosen.String(), exprSummary, spec.DefaultExpr, spec.BaselineMin,
				))
			}
		}

		if chosen == nil {
			return nil, nil, manifest.NewError(
				manifest.ErrSemVerViolation,
				fmt.Sprintf("incompatible 6-layer constraint for %s (%q): requires baseline >=%s and <2.0.0 (ADR-027c)", spec.Key, exprSummary, spec.BaselineMin),
				fmt.Sprintf("Update target_runtime for %s in grabman.json to allow >=%s <2.0.0.", spec.Key, spec.BaselineMin),
			)
		}

		layers[spec.Key] = manifest.LockedLayer{
			Layer:       spec.Key,
			Repo:        spec.Repo,
			Version:     chosen.String(),
			Constraint:  exprSummary,
			BaselineMin: spec.BaselineMin,
			SemVerTier:  spec.SemVerTier,
		}
	}

	contracts, err := collectWorkspaceContracts(realRoot, m)
	if err != nil {
		return nil, nil, err
	}

	lockSchemaVersion := strings.TrimSpace(m.SchemaVersion)
	if lockSchemaVersion == "" {
		lockSchemaVersion = manifest.SchemaVersionV100
	}
	lock := &manifest.Lockfile{
		LockVersion:           manifest.LockVersionV1,
		SchemaVersion:         lockSchemaVersion,
		ProjectName:           m.ProjectName,
		Profile:               m.Profile,
		Layers:                layers,
		Contracts:             contracts,
		CompatibilityVerified: compatibleAll,
	}
	digest, err := lock.ComputeLockDigestSHA256()
	if err != nil {
		return nil, nil, manifest.WrapError(manifest.ErrManifestMalformed, "failed to compute deterministic lock digest", "Retry grabman solve.", err)
	}
	lock.LockDigestSHA256 = digest
	return lock, warnings, nil
}

func selectBestCandidate(layerKey string, candidates []string, minVer *SemVer, parsedBounds []ConstraintSet) (*SemVer, error) {
	var chosen *SemVer
	for _, candStr := range candidates {
		cand, parseErr := ParseSemVer(candStr)
		if parseErr != nil {
			return nil, parseErr
		}
		if cand.Major >= 2 {
			return nil, manifest.NewError(
				manifest.ErrSemVerViolation,
				fmt.Sprintf("ADR-027c violation: candidate %s@%s crosses v2.0.0 major boundary", layerKey, candStr),
				"Require explicit project-owner approval before targeting v2.0.0.",
			)
		}
		if minVer != nil && cand.Compare(*minVer) < 0 {
			continue
		}
		if allConstraintsMatch(parsedBounds, cand) {
			cCopy := cand
			if chosen == nil || cCopy.Compare(*chosen) > 0 {
				chosen = &cCopy
			}
		}
	}
	return chosen, nil
}

func resolveFallbackVersion(candidates []string, parsedBounds []ConstraintSet) *SemVer {
	var pool []SemVer
	for _, cStr := range candidates {
		if sv, err := ParseSemVer(cStr); err == nil && sv.Major < 2 {
			pool = append(pool, sv)
		}
	}
	for _, fallbackStr := range []string{"0.1.0", "0.2.0", "0.3.0", "1.0.0", "1.0.1", "1.1.0"} {
		if sv, err := ParseSemVer(fallbackStr); err == nil {
			pool = append(pool, sv)
		}
	}
	for _, cs := range parsedBounds {
		for _, comp := range cs.Comparators {
			if comp.ver.Major < 2 {
				pool = append(pool, comp.ver)
				if comp.op == "<" && comp.ver.Patch > 0 {
					pool = append(pool, SemVer{Major: comp.ver.Major, Minor: comp.ver.Minor, Patch: comp.ver.Patch - 1})
				} else if comp.op == "<" && comp.ver.Minor > 0 {
					pool = append(pool, SemVer{Major: comp.ver.Major, Minor: comp.ver.Minor - 1, Patch: 0})
				}
			}
		}
	}
	var chosen *SemVer
	for _, cand := range pool {
		if cand.Major >= 2 {
			continue
		}
		if allConstraintsMatch(parsedBounds, cand) {
			cCopy := cand
			if chosen == nil || cCopy.Compare(*chosen) > 0 {
				chosen = &cCopy
			}
		}
	}
	return chosen
}

func allConstraintsMatch(bounds []ConstraintSet, cand SemVer) bool {
	for _, cs := range bounds {
		if !cs.Matches(cand) {
			return false
		}
	}
	return true
}

func collectLayerConstraints(runtimeMap map[string]string, spec LayerSpec) ([]string, string) {
	var exprs []string
	var parts []string
	for _, alias := range spec.Aliases {
		if val, ok := runtimeMap[alias]; ok && strings.TrimSpace(val) != "" {
			exprs = append(exprs, strings.TrimSpace(val))
			if len(spec.Aliases) > 1 {
				parts = append(parts, fmt.Sprintf("%s:%s", alias, strings.TrimSpace(val)))
			} else {
				parts = append(parts, strings.TrimSpace(val))
			}
		}
	}
	if len(exprs) == 0 {
		exprs = append(exprs, spec.DefaultExpr)
		parts = append(parts, spec.DefaultExpr)
	}
	return exprs, strings.Join(parts, " & ")
}

func collectWorkspaceContracts(realRoot string, m *manifest.Manifest) (map[string]manifest.ContractDigest, error) {
	contracts := map[string]manifest.ContractDigest{
		"ecosystem:grex.elements.json": {
			Artifact: "grex.elements.json",
			Source:   "urbanspress/grex-js@v1.0.0",
			SHA256:   manifest.FrozenGrexElementsSHA256,
		},
		"ecosystem:grex-js-v1.0.0.js": {
			Artifact: "grex-js-v1.0.0.js",
			Source:   "urbanspress/grex-js@v1.0.0",
			SHA256:   manifest.FrozenGrexJSSHA256,
		},
		"ecosystem:seo-playbook.v1.json": {
			Artifact: "seo-playbook.v1.json",
			Source:   "urbanspress/groxy-ai@v1.0.0",
			SHA256:   manifest.FrozenSEOPlaybookV1SHA256,
		},
	}

	optionalWorkspaceFiles := []string{
		"grabman.json",
		"grab.platform.json",
		"grab.data.json",
		"grab.api.json",
		"grab.connect.lock",
		"deploy/docker-compose.yml",
		"theme/theme.json",
	}

	for _, rel := range optionalWorkspaceFiles {
		abs, slashRel, err := manifest.ResolveWorkspaceFile(realRoot, rel, manifest.ResolveOptions{AllowMissing: true})
		if err != nil {
			return nil, err
		}
		if _, statErr := os.Stat(abs); statErr == nil {
			sum, hashErr := manifest.SHA256File(abs)
			if hashErr != nil {
				return nil, hashErr
			}
			contracts["workspace:"+slashRel] = manifest.ContractDigest{
				Artifact: slashRel,
				Source:   "workspace",
				SHA256:   sum,
			}
		}
	}

	// Verify any local copies of frozen upstream contract files match expected SHA-256 digests.
	frozenLocalTargets := []struct {
		paths    []string
		expected []string
		label    string
	}{
		{
			paths:    []string{"grex.elements.json", "public/grex/grex.elements.json", "theme/grex.elements.json"},
			expected: []string{manifest.FrozenGrexElementsSHA256},
			label:    "grex.elements.json",
		},
		{
			paths:    []string{"grex.js", "public/grex/grex.js"},
			expected: []string{manifest.FrozenGrexJSSHA256, manifest.FrozenGrabLabGrexJSSHA256},
			label:    "grex.js",
		},
		{
			paths:    []string{"playbook/seo-playbook.v1.json", "seo-playbook.v1.json"},
			expected: []string{manifest.FrozenSEOPlaybookV1SHA256},
			label:    "seo-playbook.v1.json",
		},
	}

	for _, target := range frozenLocalTargets {
		for _, candidate := range target.paths {
			abs, slashRel, err := manifest.ResolveWorkspaceFile(realRoot, candidate, manifest.ResolveOptions{AllowMissing: true})
			if err != nil {
				return nil, err
			}
			if _, statErr := os.Stat(abs); statErr == nil {
				sum, hashErr := manifest.SHA256File(abs)
				if hashErr != nil {
					return nil, hashErr
				}
				matched := false
				for _, exp := range target.expected {
					if sum == exp {
						matched = true
						break
					}
				}
				if !matched {
					return nil, manifest.NewError(
						manifest.ErrChecksumMismatch,
						fmt.Sprintf("checksum drift in %s (%s): got %s, expected frozen ecosystem digest %s", slashRel, target.label, sum, target.expected[0]),
						fmt.Sprintf("Restore %s from the verified v1.x ecosystem release before locking.", slashRel),
					)
				}
				contracts["workspace:"+slashRel] = manifest.ContractDigest{
					Artifact: slashRel,
					Source:   "workspace-verified",
					SHA256:   sum,
				}
			}
		}
	}

	if strings.TrimSpace(m.SiteDNAPath) != "" {
		abs, slashRel, err := manifest.ResolveWorkspaceFile(realRoot, m.SiteDNAPath, manifest.ResolveOptions{})
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return nil, manifest.WrapError(
					manifest.ErrSiteDNAInvalid,
					fmt.Sprintf("configured site_dna_path %q does not exist in workspace", m.SiteDNAPath),
					"Export a Site DNA bundle to that path or update site_dna_path in grabman.json.",
					err,
				)
			}
			return nil, err
		}
		rawDNA, err := manifest.ReadBoundedFile(abs)
		if err != nil {
			return nil, err
		}
		if _, err := sitedna.ValidateBytes(rawDNA); err != nil {
			return nil, err
		}
		contracts["site_dna"] = manifest.ContractDigest{
			Artifact: slashRel,
			Source:   "groxy-site-dna",
			SHA256:   manifest.SHA256Bytes(rawDNA),
		}
	}

	return contracts, nil
}

// Solve resolves the 6-layer matrix and writes deterministic `grabman.lock` inside workspaceRoot.
func Solve(workspaceRoot string, opts SolveOptions) (*SolveResult, error) {
	realRoot, err := manifest.ResolveWorkspaceRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}
	lock, warnings, err := ComputeLockfileWithWarnings(realRoot, opts)
	if err != nil {
		return nil, err
	}

	var existing *manifest.Lockfile
	if ex, _, loadErr := manifest.LoadLockfile(realRoot); loadErr == nil {
		existing = ex
	}
	lock.SolvedAtUTC = resolveTimestamp(opts, existing, lock.LockDigestSHA256)

	outBytes, err := manifest.CanonicalJSON(lock)
	if err != nil {
		return nil, manifest.WrapError(manifest.ErrManifestMalformed, "failed to serialize grabman.lock", "Retry grabman solve.", err)
	}
	if err := manifest.WriteFileAtomic(realRoot, "grabman.lock", outBytes, manifest.ResolveOptions{}); err != nil {
		return nil, err
	}
	lockPath, _, _ := manifest.ResolveWorkspaceFile(realRoot, "grabman.lock", manifest.ResolveOptions{})
	return &SolveResult{
		SchemaVersion:    manifest.SchemaVersionV100,
		Workspace:        realRoot,
		LockfilePath:     lockPath,
		LockDigestSHA256: lock.LockDigestSHA256,
		Warnings:         warnings,
		Lockfile:         lock,
	}, nil
}

// Verify runs strictly read-only verification of `grabman.lock` against `grabman.json`,
// the 6-layer compatibility matrix, and SHA-256 contract hashes. Fails closed on any drift.
func Verify(workspaceRoot string, opts SolveOptions) (*VerifyResult, error) {
	realRoot, err := manifest.ResolveWorkspaceRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}
	existing, _, err := manifest.LoadLockfile(realRoot)
	if err != nil {
		return nil, err
	}
	var verifyWarnings []string
	if !existing.CompatibilityVerified {
		if !opts.WarnIncompatible {
			return nil, manifest.NewError(
				manifest.ErrChecksumMismatch,
				"grabman.lock has compatibility_verified=false",
				"Run 'grabman solve' to resolve a verified lockfile.",
			)
		}
		verifyWarnings = append(verifyWarnings, "WARNING: grabman.lock records non-standard pinned dependency overrides (compatibility_verified=false).")
	}
	computedExistingDigest, err := existing.ComputeLockDigestSHA256()
	if err != nil {
		return nil, manifest.WrapError(manifest.ErrManifestMalformed, "cannot hash grabman.lock core", "Regenerate grabman.lock via grabman solve.", err)
	}
	if computedExistingDigest != existing.LockDigestSHA256 {
		return nil, manifest.NewError(
			manifest.ErrChecksumMismatch,
			fmt.Sprintf("grabman.lock tampering/drift detected: stored lock_digest_sha256 (%s) != computed (%s)", existing.LockDigestSHA256, computedExistingDigest),
			"Re-run 'grabman solve' after inspecting workspace diffs.",
		)
	}

	fresh, freshWarnings, err := ComputeLockfileWithWarnings(realRoot, opts)
	if err != nil {
		return nil, err
	}
	verifyWarnings = append(verifyWarnings, freshWarnings...)

	var drifts []string
	if existing.ProjectName != fresh.ProjectName {
		drifts = append(drifts, fmt.Sprintf("project_name drifted (%q -> %q)", existing.ProjectName, fresh.ProjectName))
	}
	if existing.Profile != fresh.Profile {
		drifts = append(drifts, fmt.Sprintf("profile drifted (%q -> %q)", existing.Profile, fresh.Profile))
	}
	for _, k := range sortedLayerKeys(fresh.Layers) {
		fl := fresh.Layers[k]
		el, ok := existing.Layers[k]
		if !ok {
			drifts = append(drifts, fmt.Sprintf("missing locked layer %q", k))
			continue
		}
		if el.Version != fl.Version || el.Constraint != fl.Constraint {
			drifts = append(drifts, fmt.Sprintf("layer %q drifted (locked %s [%s] vs expected %s [%s])", k, el.Version, el.Constraint, fl.Version, fl.Constraint))
		}
	}
	for k := range existing.Layers {
		if _, ok := fresh.Layers[k]; !ok {
			drifts = append(drifts, fmt.Sprintf("unexpected extra locked layer %q", k))
		}
	}

	for _, k := range sortedContractKeys(fresh.Contracts) {
		fc := fresh.Contracts[k]
		ec, ok := existing.Contracts[k]
		if !ok {
			drifts = append(drifts, fmt.Sprintf("missing pinned contract %q", k))
			continue
		}
		if ec.SHA256 != fc.SHA256 {
			drifts = append(drifts, fmt.Sprintf("SHA-256 drift on %q (locked %s vs current %s)", k, ec.SHA256, fc.SHA256))
		}
	}
	for k := range existing.Contracts {
		if _, ok := fresh.Contracts[k]; !ok {
			drifts = append(drifts, fmt.Sprintf("removed or unpinned contract %q in lockfile", k))
		}
	}

	if len(drifts) > 0 || existing.LockDigestSHA256 != fresh.LockDigestSHA256 {
		sort.Strings(drifts)
		return &VerifyResult{
			SchemaVersion:    manifest.SchemaVersionV100,
			Verified:         false,
			Deterministic:    false,
			Workspace:        realRoot,
			LockDigestSHA256: existing.LockDigestSHA256,
			Warnings:         verifyWarnings,
			DriftDetails:     drifts,
			Code:             string(manifest.ErrChecksumMismatch),
			Error:            fmt.Sprintf("%s: grabman.lock verification failed (%s)", manifest.ErrChecksumMismatch, strings.Join(drifts, "; ")),
			Remediation:      "Run 'grabman solve' to refresh grabman.lock if these changes are intentional.",
		}, manifest.NewError(
			manifest.ErrChecksumMismatch,
			fmt.Sprintf("grabman.lock verification failed: %s", strings.Join(drifts, "; ")),
			"Review unexpected changes or run 'grabman solve' to update grabman.lock.",
		)
	}

	return &VerifyResult{
		SchemaVersion:    manifest.SchemaVersionV100,
		Verified:         true,
		Deterministic:    true,
		Workspace:        realRoot,
		LockDigestSHA256: existing.LockDigestSHA256,
		Warnings:         verifyWarnings,
	}, nil
}

// InspectDependencies evaluates every ecosystem layer in grabman.json / grabman.lock
// against EcosystemMatrix() and optional installed version strings without mutating files.
func InspectDependencies(workspaceRoot string, installedVersions map[string]string) ([]DependencyAudit, error) {
	realRoot, err := manifest.ResolveWorkspaceRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}
	m, _, err := manifest.LoadManifest(realRoot)
	if err != nil {
		return nil, err
	}
	existingLock, _, _ := manifest.LoadLockfile(realRoot)

	matrix := EcosystemMatrix()
	out := make([]DependencyAudit, 0, len(matrix))
	for _, spec := range matrix {
		boundList, exprSummary := collectLayerConstraints(m.TargetRuntime, spec)
		lockedVer := ""
		if existingLock != nil {
			if l, ok := existingLock.Layers[spec.Key]; ok {
				lockedVer = l.Version
			}
		}
		installedVer := ""
		if installedVersions != nil {
			if v := strings.TrimSpace(installedVersions[spec.Key]); v != "" {
				installedVer = v
			} else {
				for _, alias := range spec.Aliases {
					if v := strings.TrimSpace(installedVersions[alias]); v != "" {
						installedVer = v
						break
					}
				}
			}
		}

		item := DependencyAudit{
			Layer:      spec.Key,
			Repo:       spec.Repo,
			Configured: exprSummary,
			Locked:     lockedVer,
			Expected:   spec.DefaultExpr,
			Installed:  installedVer,
		}

		var parsedBounds []ConstraintSet
		var parseErr error
		for _, expr := range boundList {
			cs, pErr := ParseConstraint(expr)
			if pErr != nil {
				parseErr = pErr
				break
			}
			parsedBounds = append(parsedBounds, cs)
		}
		if parseErr != nil {
			item.Status = "blocked"
			item.Compatible = false
			item.Message = fmt.Sprintf("Constraint %q violates ADR-027c SemVer policy (expected %s): %v", exprSummary, spec.DefaultExpr, parseErr)
			out = append(out, item)
			continue
		}

		minVer, _ := ParseSemVer(spec.BaselineMin)
		chosen, _ := selectBestCandidate(spec.Key, spec.Available, &minVer, parsedBounds)
		if chosen == nil {
			fallback := resolveFallbackVersion(spec.Available, parsedBounds)
			item.Status = "warning"
			item.Compatible = false
			if fallback != nil {
				item.Message = fmt.Sprintf("WARNING: Configured %q resolves to %s, outside expected %s (below baseline >=%s).", exprSummary, fallback.String(), spec.DefaultExpr, spec.BaselineMin)
			} else {
				item.Message = fmt.Sprintf("WARNING: Configured %q does not match any supported baseline release (%s).", exprSummary, spec.DefaultExpr)
			}
			out = append(out, item)
			continue
		}

		if installedVer != "" {
			instSV, instErr := ParseSemVer(installedVer)
			if instErr != nil {
				item.Status = "warning"
				item.Compatible = false
				item.Message = fmt.Sprintf("WARNING: Installed version %q is not strict SemVer (expected %s).", installedVer, spec.DefaultExpr)
				out = append(out, item)
				continue
			}
			if instSV.Major >= 2 {
				item.Status = "blocked"
				item.Compatible = false
				item.Message = fmt.Sprintf("Installed version v%s crosses unapproved v2.0.0 boundary (expected %s).", instSV.String(), spec.DefaultExpr)
				out = append(out, item)
				continue
			}
			if instSV.Compare(minVer) < 0 || !allConstraintsMatch(parsedBounds, instSV) {
				item.Status = "warning"
				item.Compatible = false
				item.Message = fmt.Sprintf("WARNING: Installed v%s does not satisfy %q / expected %s.", instSV.String(), exprSummary, spec.DefaultExpr)
				out = append(out, item)
				continue
			}
		}

		if lockedVer != "" {
			lSV, lErr := ParseSemVer(lockedVer)
			if lErr != nil || lSV.Compare(minVer) < 0 || !allConstraintsMatch(parsedBounds, lSV) {
				item.Status = "warning"
				item.Compatible = false
				item.Message = fmt.Sprintf("WARNING: Locked version %s differs from configured %q (expected %s). Run 'grabman solve --warn-incompatible' or update grabman.json.", lockedVer, exprSummary, spec.DefaultExpr)
				out = append(out, item)
				continue
			}
		}

		item.Status = "compatible"
		item.Compatible = true
		if installedVer != "" {
			item.Message = fmt.Sprintf("Compatible: installed v%s satisfies %s.", installedVer, spec.DefaultExpr)
		} else if lockedVer != "" {
			item.Message = fmt.Sprintf("Compatible: locked v%s satisfies %s.", lockedVer, spec.DefaultExpr)
		} else {
			item.Message = fmt.Sprintf("Compatible: %q satisfies %s.", exprSummary, spec.DefaultExpr)
		}
		out = append(out, item)
	}
	return out, nil
}

// HasIncompatiblePinWarning returns true iff deps contain non-blocking compatibility warnings
// and zero hard-blocked dependency violations.
func HasIncompatiblePinWarning(deps []DependencyAudit) bool {
	hasWarn := false
	for _, d := range deps {
		if d.Status == "blocked" {
			return false
		}
		if d.Status == "warning" {
			hasWarn = true
		}
	}
	return hasWarn
}

func sortedLayerKeys(m map[string]manifest.LockedLayer) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func sortedContractKeys(m map[string]manifest.ContractDigest) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
