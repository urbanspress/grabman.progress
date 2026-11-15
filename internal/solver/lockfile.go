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

// EcosystemMatrix returns the frozen 6-layer compatibility matrix for Step 5 v0.1.0.
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
			Available:   []string{"1.0.1"},
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
			DefaultExpr: ">=0.1.0 <1.0.0",
			SemVerTier:  "pre-1.0",
			Available:   []string{"0.1.0"},
		},
	}
}

// SolveOptions controls solver execution and deterministic timestamps.
type SolveOptions struct {
	FixedTimestamp string
	Now            func() time.Time
	ExtraReleases  map[string][]string // optional override for testing multi-version resolution
}

// VerifyResult captures the outcome of `grabman solve --verify`.
type VerifyResult struct {
	SchemaVersion    string   `json:"schema_version"`
	Verified         bool     `json:"verified"`
	Deterministic    bool     `json:"deterministic"`
	Workspace        string   `json:"workspace"`
	LockDigestSHA256 string   `json:"lock_digest_sha256"`
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
	realRoot, err := manifest.ResolveWorkspaceRoot(workspaceRoot)
	if err != nil {
		return nil, err
	}
	m, _, err := manifest.LoadManifest(realRoot)
	if err != nil {
		return nil, err
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
			return nil, manifest.NewError(
				manifest.ErrSemVerViolation,
				fmt.Sprintf("unknown layer %q in grabman.json target_runtime", k),
				"Use supported layer keys: grab, grex_js, grablab_cms, grex_builder, groxy_ai, grabtaculous, gashboard, grabman.",
			)
		}
	}

	layers := make(map[string]manifest.LockedLayer, len(matrix))
	for _, spec := range matrix {
		boundList, exprSummary := collectLayerConstraints(m.TargetRuntime, spec)
		var parsedBounds []ConstraintSet
		for _, expr := range boundList {
			cs, parseErr := ParseConstraint(expr)
			if parseErr != nil {
				return nil, parseErr
			}
			parsedBounds = append(parsedBounds, cs)
		}

		minVer, err := ParseSemVer(spec.BaselineMin)
		if err != nil {
			return nil, err
		}

		candidates := spec.Available
		if opts.ExtraReleases != nil && len(opts.ExtraReleases[spec.Key]) > 0 {
			candidates = opts.ExtraReleases[spec.Key]
		}

		var chosen *SemVer
		for _, candStr := range candidates {
			cand, parseErr := ParseSemVer(candStr)
			if parseErr != nil {
				return nil, parseErr
			}
			if cand.Major >= 2 {
				return nil, manifest.NewError(
					manifest.ErrSemVerViolation,
					fmt.Sprintf("ADR-027c violation: candidate %s@%s crosses v2.0.0 major boundary", spec.Key, candStr),
					"Require explicit project-owner approval before targeting v2.0.0.",
				)
			}
			if cand.Compare(minVer) < 0 {
				continue
			}
			allMatch := true
			for _, cs := range parsedBounds {
				if !cs.Matches(cand) {
					allMatch = false
					break
				}
			}
			if allMatch {
				cCopy := cand
				if chosen == nil || cCopy.Compare(*chosen) > 0 {
					chosen = &cCopy
				}
			}
		}

		if chosen == nil {
			return nil, manifest.NewError(
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
		return nil, err
	}

	lock := &manifest.Lockfile{
		LockVersion:           manifest.LockVersionV1,
		SchemaVersion:         manifest.SchemaVersionV010,
		ProjectName:           m.ProjectName,
		Profile:               m.Profile,
		Layers:                layers,
		Contracts:             contracts,
		CompatibilityVerified: true,
	}
	digest, err := lock.ComputeLockDigestSHA256()
	if err != nil {
		return nil, manifest.WrapError(manifest.ErrManifestMalformed, "failed to compute deterministic lock digest", "Retry grabman solve.", err)
	}
	lock.LockDigestSHA256 = digest
	return lock, nil
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
	lock, err := ComputeLockfile(realRoot, opts)
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
		SchemaVersion:    manifest.SchemaVersionV010,
		Workspace:        realRoot,
		LockfilePath:     lockPath,
		LockDigestSHA256: lock.LockDigestSHA256,
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
	if !existing.CompatibilityVerified {
		return nil, manifest.NewError(
			manifest.ErrChecksumMismatch,
			"grabman.lock has compatibility_verified=false",
			"Run 'grabman solve' to resolve a verified lockfile.",
		)
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

	fresh, err := ComputeLockfile(realRoot, opts)
	if err != nil {
		return nil, err
	}

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
			SchemaVersion:    manifest.SchemaVersionV010,
			Verified:         false,
			Deterministic:    false,
			Workspace:        realRoot,
			LockDigestSHA256: existing.LockDigestSHA256,
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
		SchemaVersion:    manifest.SchemaVersionV010,
		Verified:         true,
		Deterministic:    true,
		Workspace:        realRoot,
		LockDigestSHA256: existing.LockDigestSHA256,
	}, nil
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
