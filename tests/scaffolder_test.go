package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"grabman/internal/doctor"
	"grabman/internal/manifest"
	"grabman/internal/scaffolder"
)

func TestScaffolderProfilesAndLiveGrabCheck(t *testing.T) {
	grabBin := doctor.LocateGrabBinary("")
	if grabBin == "" {
		t.Fatalf("GRAB v1.1.0 binary not found on workstation; expected C:\\Program Files\\GRAB\\grab.exe or GRAB on PATH")
	}

	profiles := []struct {
		profile   string
		edgeProxy string
		wantTheme bool
	}{
		{profile: "app", edgeProxy: "caddy", wantTheme: false},
		{profile: "cms-stack", edgeProxy: "nginx", wantTheme: true},
		{profile: "ai-stack", edgeProxy: "cloudflare-tunnel", wantTheme: true},
	}

	for _, tc := range profiles {
		t.Run(tc.profile, func(t *testing.T) {
			ws := filepath.Join(t.TempDir(), "proj-"+tc.profile)
			res, err := scaffolder.Init(scaffolder.InitOptions{
				TargetDir:      ws,
				Profile:        tc.profile,
				ProjectName:    "Demo " + tc.profile,
				EdgeProxy:      tc.edgeProxy,
				FixedTimestamp: "2026-11-15T00:00:00Z",
			})
			if err != nil {
				t.Fatalf("scaffolder.Init(%s) failed: %v", tc.profile, err)
			}
			if res.LockDigestSHA256 == "" {
				t.Fatalf("expected non-empty LockDigestSHA256")
			}

			themePath := filepath.Join(ws, "theme", "theme.json")
			_, themeErr := os.Stat(themePath)
			if tc.wantTheme && themeErr != nil {
				t.Fatalf("expected theme/theme.json for %s: %v", tc.profile, themeErr)
			}
			if !tc.wantTheme && themeErr == nil {
				t.Fatalf("did not expect theme/theme.json for app profile")
			}

			// Verify zero-wrapper-div HTML
			htmlBytes, err := os.ReadFile(filepath.Join(ws, "views", "home.grab.html"))
			if err != nil {
				t.Fatalf("read home.grab.html: %v", err)
			}
			if strings.Contains(strings.ToLower(string(htmlBytes)), "<div") {
				t.Fatalf("generated home.grab.html contains forbidden <div> tag")
			}

			// Live `grab check` compilation check
			cmd := exec.Command(grabBin, "check", "--platform-config", "grab.platform.json", "--db-config", "grab.data.json", "src/app.grab")
			cmd.Dir = ws
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("live grab check failed for %s: %v\nOutput:\n%s", tc.profile, err, string(out))
			}

			// Full `doctor --strict` must pass healthy
			rep, err := doctor.Run(ws, doctor.Options{GrabBinary: grabBin, Strict: true})
			if err != nil {
				t.Fatalf("doctor.Run failed: %v", err)
			}
			if rep.ExitCode(true) != 0 {
				t.Fatalf("expected strict doctor pass (exit 0) for %s, got %s:\n%s", tc.profile, rep.Status, doctor.FormatPlainEnglish(rep))
			}
		})
	}
}

func TestScaffolderFromSiteDNAAndOverwriteGuard(t *testing.T) {
	grabBin := doctor.LocateGrabBinary("")
	if grabBin == "" {
		t.Fatalf("GRAB v1.1.0 binary not found on workstation")
	}

	tmp := t.TempDir()
	dnaPath := filepath.Join(tmp, "site-dna.json")
	if err := os.WriteFile(dnaPath, []byte(validSiteDNAv040), 0o644); err != nil {
		t.Fatal(err)
	}

	ws := filepath.Join(tmp, "hydrated-store")
	res, err := scaffolder.Init(scaffolder.InitOptions{
		TargetDir:      ws,
		FromSiteDNA:    dnaPath,
		FixedTimestamp: "2026-11-15T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("Init --from-site-dna failed: %v", err)
	}
	if !res.HydratedSiteDNA || res.Profile != "ai-stack" || res.ProjectName != "Urban Coffee Roasters" {
		t.Fatalf("unexpected InitResult: %+v", res)
	}

	htmlBytes, _ := os.ReadFile(filepath.Join(ws, "views", "home.grab.html"))
	if !strings.Contains(string(htmlBytes), "12345678901234567890.00100") {
		t.Fatalf("expected hydrated HTML to preserve exact ADR-025c base-10 wire price")
	}

	// Live `grab check` + `doctor --strict`
	cmd := exec.Command(grabBin, "check", "--platform-config", "grab.platform.json", "--db-config", "grab.data.json", "src/app.grab")
	cmd.Dir = ws
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("live grab check failed on Site DNA hydrated workspace: %v\n%s", err, string(out))
	}
	rep, err := doctor.Run(ws, doctor.Options{GrabBinary: grabBin, Strict: true})
	if err != nil || rep.ExitCode(true) != 0 {
		t.Fatalf("expected strict doctor pass after Site DNA hydration, err=%v\n%s", err, doctor.FormatPlainEnglish(rep))
	}

	// Without --force, second Init must fail non-destructively
	if _, err := scaffolder.Init(scaffolder.InitOptions{TargetDir: ws}); err == nil {
		t.Fatalf("expected error overwriting existing workspace without --force")
	}
	// With --force, second Init succeeds cleanly
	if _, err := scaffolder.Init(scaffolder.InitOptions{TargetDir: ws, Force: true}); err != nil {
		t.Fatalf("expected Init with --force to succeed: %v", err)
	}
}

func TestPathConfinementGRABE401Parity(t *testing.T) {
	ws := t.TempDir()

	blockedTargets := []string{
		"../escape",
		filepath.Join(ws, ".git"),
		filepath.Join(ws, ".env"),
		filepath.Join(ws, ".env.production"),
		filepath.Join(ws, ".ssh"),
		filepath.Join(ws, "secrets.key"),
		filepath.Join(ws, "cert.pem"),
		filepath.Join(ws, "id_rsa"),
		filepath.Join(ws, "id_ed25519"),
	}
	for _, target := range blockedTargets {
		_, err := scaffolder.Init(scaffolder.InitOptions{TargetDir: target})
		if err == nil {
			t.Fatalf("expected GRAB-E401 error for target %q", target)
		}
		if got := manifest.ExtractCode(err); got != manifest.ErrPathConfinement {
			t.Fatalf("expected GRABMAN-E201 for target %q, got %s (%v)", target, got, err)
		}
	}

	// Workspace file resolution must also reject traversal & secrets
	badRelPaths := []string{
		"../outside.txt",
		"sub/../../outside.txt",
		".git/config",
		".env",
		".env.local",
		"keys/server.key",
		"tls/root.pem",
		".ssh/authorized_keys",
	}
	for _, rel := range badRelPaths {
		if _, _, err := manifest.ResolveWorkspaceFile(ws, rel, manifest.ResolveOptions{AllowMissing: true}); manifest.ExtractCode(err) != manifest.ErrPathConfinement {
			t.Fatalf("expected GRABMAN-E201 for workspace file %q, got %s (%v)", rel, manifest.ExtractCode(err), err)
		}
		if _, err := manifest.ResolveExternalReadFile(filepath.Join(ws, rel)); manifest.ExtractCode(err) != manifest.ErrPathConfinement {
			t.Fatalf("expected GRABMAN-E201 for external read %q, got %s (%v)", rel, manifest.ExtractCode(err), err)
		}
	}
}
