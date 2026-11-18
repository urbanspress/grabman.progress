package tests

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

			// Verify canonical project layout & zero-wrapper-div HTML
			for _, mustExist := range []string{
				"src/app.grab",
				"views/index.grab.html",
				"views/home.grab.html",
				"public/grex/grex.js",
				"public/grex/tokens.css",
				"public/grex/grex.elements.json",
				"public/assets/site.css",
				"storage/.gitkeep",
				"uploads/.gitkeep",
				".env",
				".env.example",
				"deploy/.env.example",
				"grab.data.json",
				"grab.platform.json",
				"grabman.json",
				"grabman.lock",
				"deploy/docker-compose.yml",
			} {
				if _, err := os.Stat(filepath.Join(ws, filepath.FromSlash(mustExist))); err != nil {
					t.Fatalf("expected canonical scaffold file %s for %s: %v", mustExist, tc.profile, err)
				}
			}
			envInfo, err := os.Stat(filepath.Join(ws, ".env"))
			if err != nil {
				t.Fatalf("stat .env: %v", err)
			}
			if runtime.GOOS != "windows" && envInfo.Mode().Perm() != 0o600 {
				t.Fatalf("expected .env mode 0600, got %o", envInfo.Mode().Perm())
			}
			envBytes, _ := os.ReadFile(filepath.Join(ws, ".env"))
			if !strings.Contains(string(envBytes), "GRABLAB_MASTER_KEY=") || strings.Contains(string(envBytes), "<set-via-secret-manager>") {
				t.Fatalf("expected auto-generated GRABLAB_MASTER_KEY in .env")
			}
			composeBytes, _ := os.ReadFile(filepath.Join(ws, "deploy", "docker-compose.yml"))
			if !strings.Contains(string(composeBytes), "read_only: true") || !strings.Contains(string(composeBytes), "--static") || !strings.Contains(string(composeBytes), "/app/public") {
				t.Fatalf("expected hardened --read-only & --static /app/public in compose:\n%s", string(composeBytes))
			}

			htmlBytes, err := os.ReadFile(filepath.Join(ws, "views", "home.grab.html"))
			if err != nil {
				t.Fatalf("read home.grab.html: %v", err)
			}
			if strings.Contains(strings.ToLower(string(htmlBytes)), "<div") {
				t.Fatalf("generated home.grab.html contains forbidden <div> tag")
			}
			indexHTMLBytes, err := os.ReadFile(filepath.Join(ws, "views", "index.grab.html"))
			if err != nil || !strings.Contains(string(indexHTMLBytes), "/grex/tokens.css") || strings.Contains(strings.ToLower(string(indexHTMLBytes)), "<div") {
				t.Fatalf("invalid views/index.grab.html (err=%v)", err)
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

func TestInteractiveSurveyAndPrerequisiteGate(t *testing.T) {
	grabBin := doctor.LocateGrabBinary("")
	if grabBin == "" {
		t.Fatalf("GRAB v1.1.0 binary required")
	}

	// 1. Survey asks only project name + GrabLab CMS + Groxy AI (never Groxy Desk or Gashboard)
	var out bytes.Buffer
	opts := scaffolder.InitOptions{TargetDir: filepath.Join(t.TempDir(), "survey-shop")}
	prereqHooks := scaffolder.PrereqOptions{
		GrabBinary: grabBin,
		LookPathFn: func(name string) (string, error) {
			if name == "psql" {
				return "/usr/bin/psql", nil
			}
			return "", fmt.Errorf("not found")
		},
		RunCmdFn: func(name string, args ...string) ([]byte, error) {
			if strings.Contains(name, "psql") {
				return []byte("psql (PostgreSQL) 18.1\n"), nil
			}
			return exec.Command(name, args...).CombinedOutput()
		},
		DialTCPFn: func(string) error { return fmt.Errorf("closed") },
	}
	status, err := scaffolder.RunInteractiveSurvey(strings.NewReader("Urban Press\ny\ny\n"), &out, &opts, prereqHooks)
	if err != nil {
		t.Fatalf("RunInteractiveSurvey failed: %v\nOutput:\n%s", err, out.String())
	}
	promptTxt := out.String()
	if strings.Contains(strings.ToLower(promptTxt), "groxy desk") || strings.Contains(strings.ToLower(promptTxt), "gashboard") {
		t.Fatalf("interactive survey must never include Groxy Desk or Gashboard:\n%s", promptTxt)
	}
	if !strings.Contains(promptTxt, "Install GrabLab CMS? (Y/n)") || !strings.Contains(promptTxt, "Install Groxy AI companion? (y/N)") {
		t.Fatalf("missing expected survey prompts:\n%s", promptTxt)
	}
	if opts.Profile != "ai-stack" || opts.ProjectName != "Urban Press" || !status.PostgresDetected {
		t.Fatalf("unexpected survey state: opts=%+v status=%+v", opts, status)
	}

	// 2. Missing PostgreSQL emits clear plain-English instructions & honors cancel
	var missingOut bytes.Buffer
	missingOpts := scaffolder.InitOptions{TargetDir: filepath.Join(t.TempDir(), "missing-pg")}
	missingHooks := scaffolder.PrereqOptions{
		GrabBinary: grabBin,
		LookPathFn: func(string) (string, error) { return "", fmt.Errorf("not found") },
		DialTCPFn:  func(string) error { return fmt.Errorf("connection refused") },
	}
	stMiss, err := scaffolder.RunInteractiveSurvey(strings.NewReader("Shop\ny\nn\nn\n"), &missingOut, &missingOpts, missingHooks)
	if err == nil || stMiss.PostgresDetected || !strings.Contains(missingOut.String(), "PostgreSQL 18 was not detected locally") {
		t.Fatalf("expected plain-English PostgreSQL install guidance + cancel error, err=%v out:\n%s", err, missingOut.String())
	}

	// 3. Missing GRAB fails prerequisite check cleanly before optional packages
	_, err = scaffolder.CheckPrerequisites("app", scaffolder.PrereqOptions{
		GrabBinary: filepath.Join(t.TempDir(), "missing-grab"),
		LookPathFn: func(string) (string, error) { return "", fmt.Errorf("not found") },
	})
	if err == nil || !strings.Contains(err.Error(), "GRAB CLI") {
		t.Fatalf("expected missing GRAB prerequisite error, got: %v", err)
	}
}
