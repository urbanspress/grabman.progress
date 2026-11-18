package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"grabman/internal/manifest"
	"grabman/internal/solver"
)

func writeTestManifest(t *testing.T, dir string, m manifest.Manifest) {
	t.Helper()
	b, err := manifest.CanonicalJSON(m)
	if err != nil {
		t.Fatalf("CanonicalJSON: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "grabman.json"), b, 0o644); err != nil {
		t.Fatalf("WriteFile grabman.json: %v", err)
	}
}

func validTestManifest(name string) manifest.Manifest {
	return manifest.Manifest{
		SchemaVersion: manifest.SchemaVersionV010,
		ProjectName:   name,
		Profile:       "ai-stack",
		TargetRuntime: map[string]string{
			"grab":         ">=1.1.0 <2.0.0",
			"grex-js":      "^1.0.0",
			"grablab-cms":  "^1.0.1",
			"grex-builder": "^1.0.0",
			"groxy-ai":     "^1.0.0",
			"step-5":       "^0.1.0",
		},
		DeploySecurity: manifest.DeploySecurity{
			ReadOnlyRoot:      true,
			NonRootUID:        "10001:10001",
			LoopbackOnlyPorts: true,
			EdgeProxy:         "caddy",
		},
	}
}

func TestSolverDeterministicAndVerify(t *testing.T) {
	ws1 := t.TempDir()
	ws2 := t.TempDir()
	writeTestManifest(t, ws1, validTestManifest("deterministic-shop"))
	writeTestManifest(t, ws2, validTestManifest("deterministic-shop"))

	fixedTS := "2026-11-15T00:00:00Z"
	res1, err := solver.Solve(ws1, solver.SolveOptions{FixedTimestamp: fixedTS})
	if err != nil {
		t.Fatalf("Solve ws1 failed: %v", err)
	}
	res2, err := solver.Solve(ws2, solver.SolveOptions{FixedTimestamp: fixedTS})
	if err != nil {
		t.Fatalf("Solve ws2 failed: %v", err)
	}

	if res1.LockDigestSHA256 != res2.LockDigestSHA256 {
		t.Fatalf("expected identical lock digest, got %s vs %s", res1.LockDigestSHA256, res2.LockDigestSHA256)
	}

	b1, _ := os.ReadFile(filepath.Join(ws1, "grabman.lock"))
	b2, _ := os.ReadFile(filepath.Join(ws2, "grabman.lock"))
	if string(b1) != string(b2) {
		t.Fatalf("expected byte-for-byte identical grabman.lock across repeated solves")
	}

	// Re-solving without FixedTimestamp must also preserve exact bytes when lock digest is unchanged.
	if _, err := solver.Solve(ws1, solver.SolveOptions{}); err != nil {
		t.Fatalf("re-solve ws1 failed: %v", err)
	}
	b1After, _ := os.ReadFile(filepath.Join(ws1, "grabman.lock"))
	if string(b1) != string(b1After) {
		t.Fatalf("expected re-solve to keep solved_at_utc and exact bytes stable")
	}

	vRes, err := solver.Verify(ws1, solver.SolveOptions{})
	if err != nil {
		t.Fatalf("Verify failed: %v", err)
	}
	if !vRes.Verified || !vRes.Deterministic {
		t.Fatalf("expected verified & deterministic true, got %+v", vRes)
	}
}

func TestSolverRejectsIncompatibleConstraints(t *testing.T) {
	cases := []struct {
		name     string
		override map[string]string
		wantCode manifest.ErrorCode
	}{
		{
			name:     "grab below 1.1.0 baseline",
			override: map[string]string{"grab": "<1.1.0"},
			wantCode: manifest.ErrSemVerViolation,
		},
		{
			name:     "grablab-cms pinned to 1.0.0 instead of >=1.0.1",
			override: map[string]string{"grablab-cms": "=1.0.0"},
			wantCode: manifest.ErrSemVerViolation,
		},
		{
			name:     "unapproved v2.0.0 major jump (ADR-027c)",
			override: map[string]string{"grab": ">=2.0.0"},
			wantCode: manifest.ErrSemVerViolation,
		},
		{
			name:     "wildcard constraint forbidden",
			override: map[string]string{"grex-js": "*"},
			wantCode: manifest.ErrSemVerViolation,
		},
		{
			name:     "latest keyword forbidden",
			override: map[string]string{"groxy-ai": "latest"},
			wantCode: manifest.ErrSemVerViolation,
		},
		{
			name:     "unknown layer key rejected",
			override: map[string]string{"unknown-plugin": "^1.0.0"},
			wantCode: manifest.ErrSemVerViolation,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := t.TempDir()
			m := validTestManifest("bad-constraints")
			for k, v := range tc.override {
				m.TargetRuntime[k] = v
			}
			writeTestManifest(t, ws, m)

			_, err := solver.Solve(ws, solver.SolveOptions{FixedTimestamp: "2026-11-15T00:00:00Z"})
			if err == nil {
				t.Fatalf("expected error for case %s, got nil", tc.name)
			}
			if got := manifest.ExtractCode(err); got != tc.wantCode {
				t.Fatalf("expected code %s, got %s (err=%v)", tc.wantCode, got, err)
			}
		})
	}
}

func TestSolverVerifyDetectsDriftAndChecksumTamper(t *testing.T) {
	ws := t.TempDir()
	writeTestManifest(t, ws, validTestManifest("tamper-check"))

	if _, err := solver.Solve(ws, solver.SolveOptions{FixedTimestamp: "2026-11-15T00:00:00Z"}); err != nil {
		t.Fatalf("initial Solve failed: %v", err)
	}

	// 1. Tamper lockfile digest
	lockPath := filepath.Join(ws, "grabman.lock")
	origLock, _ := os.ReadFile(lockPath)
	tampered := strings.Replace(string(origLock), `"version": "1.1.0"`, `"version": "2.0.0"`, 1)
	if err := os.WriteFile(lockPath, []byte(tampered), 0o644); err != nil {
		t.Fatalf("write tampered lock: %v", err)
	}
	if _, err := solver.Verify(ws, solver.SolveOptions{}); manifest.ExtractCode(err) != manifest.ErrChecksumMismatch {
		t.Fatalf("expected GRABMAN-E103 on tampered lockfile, got %v (code=%s)", err, manifest.ExtractCode(err))
	}

	// Restore valid lockfile and check tampered frozen artifact
	if err := os.WriteFile(lockPath, origLock, 0o644); err != nil {
		t.Fatalf("restore lock: %v", err)
	}
	if err := os.WriteFile(filepath.Join(ws, "grex.elements.json"), []byte(`{"tampered":true}`), 0o644); err != nil {
		t.Fatalf("write tampered grex.elements.json: %v", err)
	}
	if _, err := solver.Verify(ws, solver.SolveOptions{}); manifest.ExtractCode(err) != manifest.ErrChecksumMismatch {
		t.Fatalf("expected GRABMAN-E103 on tampered grex.elements.json, got %v (code=%s)", err, manifest.ExtractCode(err))
	}
}

func TestSchemaVersionDualCompatibilityAndStep5MatrixV100(t *testing.T) {
	// Verify v0.1.0 manifest continues solving & verifying alongside v1.0.0 manifests.
	ws010 := t.TempDir()
	m010 := validTestManifest("legacy-v010")
	m010.SchemaVersion = manifest.SchemaVersionV010
	m010.TargetRuntime["step-5"] = ">=0.1.0 <1.0.0"
	writeTestManifest(t, ws010, m010)

	res010, err := solver.Solve(ws010, solver.SolveOptions{FixedTimestamp: "2026-11-15T00:00:00Z"})
	if err != nil {
		t.Fatalf("Solve v0.1.0 failed: %v", err)
	}
	if res010.Lockfile.SchemaVersion != manifest.SchemaVersionV010 {
		t.Fatalf("expected v0.1.0 lockfile schema_version to preserve 0.1.0, got %s", res010.Lockfile.SchemaVersion)
	}
	if got := res010.Lockfile.Layers["step-5"].Version; got != "0.3.0" {
		t.Fatalf("expected >=0.1.0 <1.0.0 to resolve to step-5 0.3.0, got %s", got)
	}
	if _, err := solver.Verify(ws010, solver.SolveOptions{}); err != nil {
		t.Fatalf("Verify v0.1.0 failed: %v", err)
	}

	// Verify v1.0.0 manifest resolves step-5 to 1.0.0.
	ws100 := t.TempDir()
	m100 := validTestManifest("frozen-v100")
	m100.SchemaVersion = manifest.SchemaVersionV100
	m100.TargetRuntime["step-5"] = ">=0.1.0 <2.0.0"
	writeTestManifest(t, ws100, m100)

	res100, err := solver.Solve(ws100, solver.SolveOptions{FixedTimestamp: "2026-11-15T00:00:00Z"})
	if err != nil {
		t.Fatalf("Solve v1.0.0 failed: %v", err)
	}
	if res100.Lockfile.SchemaVersion != manifest.SchemaVersionV100 {
		t.Fatalf("expected v1.0.0 lockfile schema_version 1.0.0, got %s", res100.Lockfile.SchemaVersion)
	}
	if got := res100.Lockfile.Layers["step-5"].Version; got != "1.1.0" {
		t.Fatalf("expected >=0.1.0 <2.0.0 to resolve to step-5 1.1.0, got %s", got)
	}
	if _, err := solver.Verify(ws100, solver.SolveOptions{}); err != nil {
		t.Fatalf("Verify v1.0.0 failed: %v", err)
	}
}

func TestSolverWarnIncompatibleAndInspectDependencies(t *testing.T) {
	ws := t.TempDir()
	m := validTestManifest("warn-override")
	m.TargetRuntime["grablab-cms"] = "=1.0.0"
	writeTestManifest(t, ws, m)

	// Strict solve (default) still rejects incompatible <1.0.1 pin
	if _, err := solver.Solve(ws, solver.SolveOptions{FixedTimestamp: "2026-11-15T00:00:00Z"}); manifest.ExtractCode(err) != manifest.ErrSemVerViolation {
		t.Fatalf("expected strict Solve to fail with GRABMAN-E102, got %v", err)
	}

	// Non-blocking --warn-incompatible succeeds with clear warning
	res, err := solver.Solve(ws, solver.SolveOptions{
		FixedTimestamp:   "2026-11-15T00:00:00Z",
		WarnIncompatible: true,
	})
	if err != nil {
		t.Fatalf("Solve with WarnIncompatible failed: %v", err)
	}
	if res.Lockfile.CompatibilityVerified {
		t.Fatalf("expected CompatibilityVerified=false when incompatible pin override is active")
	}
	if got := res.Lockfile.Layers["grablab-cms"].Version; got != "1.0.0" {
		t.Fatalf("expected pinned override 1.0.0, got %s", got)
	}
	if len(res.Warnings) == 0 || !strings.Contains(res.Warnings[0], "WARNING") || !strings.Contains(res.Warnings[0], "1.0.0") {
		t.Fatalf("expected clear non-blocking WARNING, got: %v", res.Warnings)
	}

	vRes, err := solver.Verify(ws, solver.SolveOptions{WarnIncompatible: true})
	if err != nil || !vRes.Verified || len(vRes.Warnings) == 0 {
		t.Fatalf("Verify with WarnIncompatible failed: res=%+v err=%v", vRes, err)
	}

	deps, err := solver.InspectDependencies(ws, map[string]string{"grab": "1.1.0"})
	if err != nil {
		t.Fatalf("InspectDependencies failed: %v", err)
	}
	foundCMSWarn := false
	for _, d := range deps {
		if d.Layer == "grablab-cms" && d.Status == "warning" && !d.Compatible {
			foundCMSWarn = true
		}
	}
	if !foundCMSWarn {
		t.Fatalf("expected grablab-cms dependency audit warning, got: %+v", deps)
	}
}
