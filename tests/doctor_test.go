package tests

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"grabman/internal/doctor"
	"grabman/internal/manifest"
	"grabman/internal/scaffolder"
)

func TestDoctorHealthyWorkspaceAndJSONFormat(t *testing.T) {
	grabBin := doctor.LocateGrabBinary("")
	ws := filepath.Join(t.TempDir(), "healthy-cms")

	if _, err := scaffolder.Init(scaffolder.InitOptions{
		TargetDir:   ws,
		Profile:     "cms-stack",
		ProjectName: "Healthy Store",
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	rep, err := doctor.Run(ws, doctor.Options{GrabBinary: grabBin, Strict: true})
	if err != nil {
		t.Fatalf("doctor.Run failed: %v", err)
	}
	if rep.Status != doctor.OverallHealthy || rep.ExitCode(true) != 0 {
		t.Fatalf("expected OverallHealthy, got %s:\n%s", rep.Status, doctor.FormatPlainEnglish(rep))
	}

	rawJSON, err := doctor.FormatJSON(rep)
	if err != nil {
		t.Fatalf("FormatJSON failed: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(rawJSON, &decoded); err != nil {
		t.Fatalf("invalid JSON from FormatJSON: %v", err)
	}
	if decoded["status"] != "healthy" {
		t.Fatalf("expected status=healthy in JSON, got %v", decoded["status"])
	}
	deps, ok := decoded["dependencies"].([]any)
	if !ok || len(deps) != 6 {
		t.Fatalf("expected 6 dependency layers in JSON dependencies array, got %T (%v)", decoded["dependencies"], decoded["dependencies"])
	}
	plain := doctor.FormatPlainEnglish(rep)
	if !strings.Contains(plain, "Dependencies & Version Compatibility:") || !strings.Contains(plain, "grablab-cms") {
		t.Fatalf("expected Dependencies & Version Compatibility section in plain-English output:\n%s", plain)
	}
}

func TestDoctorNonBlockingWarningOnIncompatiblePin(t *testing.T) {
	grabBin := doctor.LocateGrabBinary("")
	ws := filepath.Join(t.TempDir(), "pin-override")
	if _, err := scaffolder.Init(scaffolder.InitOptions{
		TargetDir:   ws,
		Profile:     "cms-stack",
		ProjectName: "Pinned App",
	}); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	manifestPath := filepath.Join(ws, "grabman.json")
	raw, _ := os.ReadFile(manifestPath)
	mutated := strings.Replace(string(raw), `"^1.0.1"`, `"=1.0.0"`, 1)
	if err := os.WriteFile(manifestPath, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}

	// Non-strict doctor exits 0 with WARNING status instead of blocking local workflow
	rep, err := doctor.Run(ws, doctor.Options{GrabBinary: grabBin, Strict: false})
	if err != nil {
		t.Fatalf("doctor.Run failed: %v", err)
	}
	if rep.ExitCode(false) != 0 || rep.Status != doctor.OverallWarnings {
		t.Fatalf("expected non-blocking warnings (exit 0), got status=%s code=%d:\n%s", rep.Status, rep.ExitCode(false), doctor.FormatPlainEnglish(rep))
	}
	plain := doctor.FormatPlainEnglish(rep)
	if !strings.Contains(plain, "[WARN]") || !strings.Contains(plain, "1.0.0") || !strings.Contains(plain, ">=1.0.1 <2.0.0") {
		t.Fatalf("expected expected-vs-pinned warning in doctor report:\n%s", plain)
	}
}

func TestDoctorSecurityHardeningAndSecretRedaction(t *testing.T) {
	grabBin := doctor.LocateGrabBinary("")

	t.Run("compose read_only false and privileged blocked", func(t *testing.T) {
		ws := filepath.Join(t.TempDir(), "unsafe-compose")
		if _, err := scaffolder.Init(scaffolder.InitOptions{TargetDir: ws, Profile: "app"}); err != nil {
			t.Fatal(err)
		}
		badCompose := `services:
  grab-app:
    image: ghcr.io/urbanspress/grab:latest
    read_only: false
    privileged: true
    network_mode: host
    ports:
      - "0.0.0.0:8080:8080"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
`
		if err := os.WriteFile(filepath.Join(ws, "deploy", "docker-compose.yml"), []byte(badCompose), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := doctor.Run(ws, doctor.Options{GrabBinary: grabBin, Strict: true})
		if err != nil {
			t.Fatal(err)
		}
		if rep.ExitCode(true) == 0 {
			t.Fatalf("expected blocked doctor report on insecure compose")
		}
		txt := doctor.FormatPlainEnglish(rep)
		for _, want := range []string{"GRABMAN-E301", "read_only: true", "privileged", "network_mode: host", ":latest", "docker.sock"} {
			if !strings.Contains(txt, want) {
				t.Fatalf("expected diagnostic %q in plain-English doctor output:\n%s", want, txt)
			}
		}
	})

	t.Run("hardcoded secrets and private IP redacted", func(t *testing.T) {
		ws := filepath.Join(t.TempDir(), "secret-leak")
		if _, err := scaffolder.Init(scaffolder.InitOptions{TargetDir: ws, Profile: "ai-stack"}); err != nil {
			t.Fatal(err)
		}
		secretVal := "SuperTopSecretPassword999"
		envContent := "GRABLAB_MASTER_KEY=" + secretVal + "\nINTERNAL_HOST=192.168.42.10\n"
		if err := os.WriteFile(filepath.Join(ws, "deploy", ".env.example"), []byte(envContent), 0o644); err != nil {
			t.Fatal(err)
		}

		rep, err := doctor.Run(ws, doctor.Options{GrabBinary: grabBin, Strict: true})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Status != doctor.OverallBlocked {
			t.Fatalf("expected OverallBlocked on leaked secret, got %s", rep.Status)
		}
		txt := doctor.FormatPlainEnglish(rep)
		jsonBytes, _ := doctor.FormatJSON(rep)
		if strings.Contains(txt, secretVal) || strings.Contains(string(jsonBytes), secretVal) {
			t.Fatalf("doctor leaked raw secret value into output")
		}
		if !strings.Contains(txt, string(manifest.ErrSecurityPolicy)) {
			t.Fatalf("expected GRABMAN-E301 diagnostic in output:\n%s", txt)
		}
	})

	t.Run("wrapper div in view template blocked", func(t *testing.T) {
		ws := filepath.Join(t.TempDir(), "wrapper-div")
		if _, err := scaffolder.Init(scaffolder.InitOptions{TargetDir: ws, Profile: "cms-stack"}); err != nil {
			t.Fatal(err)
		}
		badHTML := `<!doctype html><html><body><div class="wrapper"><p>Bad</p></div></body></html>`
		if err := os.WriteFile(filepath.Join(ws, "views", "home.grab.html"), []byte(badHTML), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := doctor.Run(ws, doctor.Options{GrabBinary: grabBin, Strict: true})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Status != doctor.OverallBlocked {
			t.Fatalf("expected OverallBlocked for wrapper <div>")
		}
		if !strings.Contains(doctor.FormatPlainEnglish(rep), string(manifest.ErrSiteDNAInvalid)) {
			t.Fatalf("expected GRABMAN-E202 for wrapper <div>:\n%s", doctor.FormatPlainEnglish(rep))
		}
	})

	t.Run("theme mode non-visual blocked on cms-stack", func(t *testing.T) {
		ws := filepath.Join(t.TempDir(), "non-visual-theme")
		if _, err := scaffolder.Init(scaffolder.InitOptions{TargetDir: ws, Profile: "cms-stack"}); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(ws, "theme", "theme.json"), []byte(`{"mode":"code"}`), 0o644); err != nil {
			t.Fatal(err)
		}
		rep, err := doctor.Run(ws, doctor.Options{GrabBinary: grabBin, Strict: true})
		if err != nil {
			t.Fatal(err)
		}
		if rep.Status != doctor.OverallBlocked || !strings.Contains(doctor.FormatPlainEnglish(rep), `mode: "visual"`) {
			t.Fatalf("expected visual-only theme check failure:\n%s", doctor.FormatPlainEnglish(rep))
		}
	})
}
