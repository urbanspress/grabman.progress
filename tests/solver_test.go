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
