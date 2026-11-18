package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"grabman/internal/doctor"
)

func buildGrabmanCLI(t *testing.T) string {
	t.Helper()
	binName := "grabman"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	binPath := filepath.Join(t.TempDir(), binName)
	cmd := exec.Command("go", "build", "-o", binPath, "../cmd/grabman")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build grabman CLI: %v\n%s", err, string(out))
	}
	return binPath
}

func runCLI(t *testing.T, binPath string, args ...string) (string, int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return string(out), 0
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		return string(out), exitErr.ExitCode()
	}
	t.Fatalf("failed running CLI %v: %v\n%s", args, err, string(out))
	return string(out), 1
}

func TestCLIEndToEndWorkflow(t *testing.T) {
	bin := buildGrabmanCLI(t)
	tmp := t.TempDir()

	// 1. version
	out, code := runCLI(t, bin, "version", "--json")
	if code != 0 || !strings.Contains(out, `"version": "1.1.0"`) || !strings.Contains(out, `"schema_version": "1.0.0"`) {
		t.Fatalf("version --json failed (code=%d): %s", code, out)
	}

	// 2. init
	ws := filepath.Join(tmp, "cli-app")
	out, code = runCLI(t, bin, "init", "--dir", ws, "--profile", "ai-stack", "--name", "CLIShop", "--fixed-timestamp", "2026-11-15T00:00:00Z", "--json")
	if code != 0 {
		t.Fatalf("init failed (code=%d): %s", code, out)
	}

	// 3. solve --verify
	out, code = runCLI(t, bin, "solve", "--dir", ws, "--verify", "--json")
	if code != 0 || !strings.Contains(out, `"verified": true`) {
		t.Fatalf("solve --verify failed (code=%d): %s", code, out)
	}

	// 4. doctor --strict --json
	out, code = runCLI(t, bin, "doctor", "--dir", ws, "--strict", "--json")
	if code != 0 || !strings.Contains(out, `"status": "healthy"`) {
		t.Fatalf("doctor --strict failed (code=%d): %s", code, out)
	}

	// 5. solve --verify fails closed on unapproved v2.0.0 jump
	manifestPath := filepath.Join(ws, "grabman.json")
	mBytes, _ := os.ReadFile(manifestPath)
	mutated := strings.Replace(string(mBytes), `">=1.1.0 <2.0.0"`, `">=2.0.0"`, 1)
	if err := os.WriteFile(manifestPath, []byte(mutated), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runCLI(t, bin, "solve", "--dir", ws, "--verify", "--json")
	if code == 0 || !strings.Contains(out, "GRABMAN-E102") {
		t.Fatalf("expected non-zero GRABMAN-E102 failure on v2.0.0 jump (code=%d): %s", code, out)
	}

	// 5b. solve --warn-incompatible + non-strict doctor emit non-blocking WARNING on <2.0.0 pin
	warnWS := filepath.Join(tmp, "warn-cli-ws")
	if out, code := runCLI(t, bin, "init", "--dir", warnWS, "--profile", "cms-stack", "--name", "WarnCLI", "--non-interactive"); code != 0 {
		t.Fatalf("init warn-cli-ws failed (code=%d): %s", code, out)
	}
	wmPath := filepath.Join(warnWS, "grabman.json")
	wmBytes, _ := os.ReadFile(wmPath)
	if err := os.WriteFile(wmPath, []byte(strings.Replace(string(wmBytes), `"^1.0.1"`, `"=1.0.0"`, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	out, code = runCLI(t, bin, "solve", "--dir", warnWS, "--warn-incompatible", "--json")
	if code != 0 || !strings.Contains(out, "WARNING") {
		t.Fatalf("solve --warn-incompatible failed (code=%d): %s", code, out)
	}
	out, code = runCLI(t, bin, "doctor", "--dir", warnWS)
	if code != 0 || !strings.Contains(out, "Dependencies & Version Compatibility:") || !strings.Contains(out, "[WARN]") {
		t.Fatalf("doctor non-blocking warning check failed (code=%d):\n%s", code, out)
	}

	// 6. dna validate & diff with trailing --json flag
	oldDNA := filepath.Join(tmp, "dna-old.json")
	newDNA := filepath.Join(tmp, "dna-new.json")
	if err := os.WriteFile(oldDNA, []byte(validSiteDNAv040), 0o644); err != nil {
		t.Fatal(err)
	}
	updatedDNA := strings.Replace(validSiteDNAv040, `"price": "24.95"`, `"price": "29.00"`, 1)
	if err := os.WriteFile(newDNA, []byte(updatedDNA), 0o644); err != nil {
		t.Fatal(err)
	}

	out, code = runCLI(t, bin, "dna", "validate", oldDNA, "--json")
	if code != 0 || !strings.Contains(out, `"decimal_integrity_verified": true`) {
		t.Fatalf("dna validate failed (code=%d): %s", code, out)
	}
	out, code = runCLI(t, bin, "dna", "diff", oldDNA, newDNA, "--json")
	if code != 0 || !strings.Contains(out, `"has_changes": true`) {
		t.Fatalf("dna diff failed (code=%d): %s", code, out)
	}
}

func TestLiveGroxyAISiteDNAExportCompatibility(t *testing.T) {
	groxySiteContext := filepath.Join(`C:\Development\groxy-ai`, "public", "groxy", "site-context.js")
	if _, err := os.Stat(groxySiteContext); err != nil {
		t.Fatalf("expected groxy-ai upstream contract at %s: %v", groxySiteContext, err)
	}
	if doctor.LocateGrabBinary("") == "" {
		t.Fatalf("GRAB v1.1.0 binary required for live Groxy AI -> grabman -> grab check verification")
	}

	bin := buildGrabmanCLI(t)
	tmp := t.TempDir()
	exportFile := filepath.Join(tmp, "live-groxy-site-dna.json")

	// Execute live Groxy AI buildPortableSiteDna directly from C:\Development\groxy-ai via Node ESM.
	nodeScript := `
import fs from "node:fs";
import { pathToFileURL } from "node:url";

const mod = await import(pathToFileURL(process.argv[1]).href);
const siteContext = {
  site_name: "Groxy Live Bakery",
  site_url: "https://bakery.urbanpress.example",
  active_theme: "grex-builder",
  categories: ["Artisan Bread", "Coffee"],
  tags: ["grab-decimal", "wcag-aa"],
  menus: [{ slug: "primary", location: "primary", items: [{ label: "Shop", url: "/products/sourdough" }] }],
  redirects: [{ old_path: "/loaf", target_path: "/products/sourdough", status_code: 301 }],
  entries: [
    { id: 1, title: "Home", slug: "home", full_path: "/", content_type: "page", status: "published", body_html: "<p>Welcome to our bakery.</p>" },
    { id: 2, title: "Country Sourdough", slug: "sourdough", full_path: "/products/sourdough", content_type: "product", status: "published", categories: ["Artisan Bread"], custom_fields: { price: "12.50", currency: "USD", sku: "LOAF-01" }, body_html: "<p>48h cold ferment.</p>" },
    { id: 3, title: "Reserve Collector Roast", slug: "collector-roast", full_path: "/products/collector-roast", content_type: "product", status: "published", categories: ["Coffee"], custom_fields: { price: "12345678901234567890.00100", currency: "USD", sku: "ROAST-99" }, body_html: "<p>Exact ADR-025c test product.</p>" }
  ]
};
const bt = "\u0060";
const designSystemMd = "# GRAB-DESIGN-SYSTEM.md\n\nAvoid wrapper " + bt + "<div>" + bt + " elements.\n\n## 1. Design Tokens\n| Token | Value |\n|---|---|\n| " + bt + "--grex-primary" + bt + " | " + bt + "#0f766e" + bt + " |\n";
const dna = mod.buildPortableSiteDna({
  site_context: siteContext,
  style_pack: "creator-academy",
  design_system_md: designSystemMd
});
fs.writeFileSync(process.argv[2], JSON.stringify(dna, null, 2), "utf8");
`
	nodeCmd := exec.Command("node", "--input-type=module", "-e", nodeScript, groxySiteContext, exportFile)
	if nodeOut, err := nodeCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed executing live Groxy AI site-context.js via Node: %v\n%s", err, string(nodeOut))
	}

	// 1. `grabman dna validate` on live Groxy AI bundle
	out, code := runCLI(t, bin, "dna", "validate", exportFile, "--json")
	if code != 0 {
		t.Fatalf("grabman dna validate failed on live Groxy AI bundle (code=%d): %s", code, out)
	}
	var valResp map[string]any
	if err := json.Unmarshal([]byte(out), &valResp); err != nil || valResp["ok"] != true {
		t.Fatalf("unexpected dna validate JSON output: %s", out)
	}

	// 2. `grabman init --from-site-dna` on live Groxy AI bundle
	ws := filepath.Join(tmp, "live-hydrated-ws")
	out, code = runCLI(t, bin, "init", "--from-site-dna", exportFile, "--dir", ws, "--json")
	if code != 0 {
		t.Fatalf("grabman init --from-site-dna failed (code=%d): %s", code, out)
	}

	// 3. Verify ADR-025c wire precision preserved into generated workspace HTML
	htmlBytes, err := os.ReadFile(filepath.Join(ws, "views", "home.grab.html"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(htmlBytes), "12345678901234567890.00100") {
		t.Fatalf("hydrated HTML lost exact ADR-025c wire string")
	}

	// 4. `grabman doctor --strict --json` (including live `grab check`) must pass 100%
	out, code = runCLI(t, bin, "doctor", "--dir", ws, "--strict", "--json")
	if code != 0 || !strings.Contains(out, `"status": "healthy"`) {
		t.Fatalf("grabman doctor --strict failed on live Groxy AI hydrated workspace (code=%d):\n%s", code, out)
	}
}
