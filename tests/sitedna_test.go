package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"grabman/internal/manifest"
	"grabman/internal/sitedna"
)

const validSiteDNAv040 = "{\n" + `
  "schema_version": "0.4.0",
  "bundle_name": "groxy-site-dna-v0.4.0",
  "exported_at": "2026-11-15T01:00:00Z",
  "read_only_snapshot": true,
  "compatible_consumers": ["gashboard", "grabman", "grabtaculous"],
  "site_name": "Urban Coffee Roasters",
  "site_url": "https://urbancoffee.example",
  "active_style_pack": {
    "id": "warm-artisan",
    "label": "Warm Artisan"
  },
  "design_system": {
    "tokens": {
      "primary": "#047857",
      "surface": "#F8FAFC",
      "ink": "#0F172A",
      "accent": "#10B981",
      "muted": "#475569"
    },
    "reusable_classes": ["hero", "pricing-card"],
    "raw_markdown": ` + `"# GRAB-DESIGN-SYSTEM.md\n\nNever emit wrapper ` + "`<div>`" + ` elements or ` + "`javascript:`" + ` links in generated Grex templates.\n"` + `
  },
  "site_context": {
    "counts": {"pages": 4, "posts": 12, "products": 2},
    "products": [
      {
        "title": "Ethiopian Single Origin",
        "path": "/products/ethiopian",
        "sku": "ETH-250G",
        "price": "24.95",
        "currency": "USD",
        "availability": "in_stock"
      },
      {
        "title": "Micro-Lot Reserve",
        "path": "/products/micro-lot",
        "sku": "MLR-001",
        "price": "12345678901234567890.00100",
        "currency": "USD",
        "availability": "in_stock"
      }
    ],
    "redirects": [
      {"old_path": "/beans", "target_path": "/products/ethiopian", "status_code": 301}
    ],
    "markdown": ` + `"# SITE-CONTEXT.md\n\nSemantic HTML uses ` + "`<main>`" + ` and ` + "`<section>`" + ` instead of ` + "`<div>`" + `.\n"` + `
  },
  "nightly_watch_digest": {
    "site_health_score": 96,
    "overall_grade": "A"
  },
  "decimal_integrity_verified": true
}`

func TestSiteDNAValidatesExactDecimalsAndBacktickedMarkdown(t *testing.T) {
	bundle, err := sitedna.ValidateBytes([]byte(validSiteDNAv040))
	if err != nil {
		t.Fatalf("expected valid Site DNA bundle, got error: %v", err)
	}
	if bundle.SiteName != "Urban Coffee Roasters" {
		t.Fatalf("unexpected site_name %q", bundle.SiteName)
	}
	if len(bundle.Products) != 2 || bundle.Products[1].Price != "12345678901234567890.00100" {
		t.Fatalf("expected exact ADR-025c price preservation, got %+v", bundle.Products)
	}
	if !bundle.DecimalIntegrityVerified {
		t.Fatalf("expected DecimalIntegrityVerified=true")
	}

	// Verify v1.0.0 schema_version accepted too
	v1 := strings.ReplaceAll(validSiteDNAv040, `"0.4.0"`, `"1.0.0"`)
	v1 = strings.ReplaceAll(v1, `"groxy-site-dna-v0.4.0"`, `"groxy-site-dna-v1.0.0"`)
	if _, err := sitedna.ValidateBytes([]byte(v1)); err != nil {
		t.Fatalf("expected v1.0.0 Site DNA bundle to validate cleanly: %v", err)
	}
}

func TestSiteDNARejectsFloatDriftXSSAndWrapperDivs(t *testing.T) {
	cases := []struct {
		name    string
		mutate  func(string) string
		wantMsg string
	}{
		{
			name: "unquoted JSON float price",
			mutate: func(s string) string {
				return strings.Replace(s, `"price": "24.95"`, `"price": 24.95`, 1)
			},
			wantMsg: "ADR-025c",
		},
		{
			name: "IEEE-754 float drift string",
			mutate: func(s string) string {
				return strings.Replace(s, `"price": "24.95"`, `"price": "0.30000000000000004"`, 1)
			},
			wantMsg: "ADR-025c",
		},
		{
			name: "scientific notation string",
			mutate: func(s string) string {
				return strings.Replace(s, `"price": "24.95"`, `"price": "1e2"`, 1)
			},
			wantMsg: "ADR-025c",
		},
		{
			name: "wrapper div inside product title",
			mutate: func(s string) string {
				return strings.Replace(s, `"Ethiopian Single Origin"`, `"<div>Ethiopian</div>"`, 1)
			},
			wantMsg: "div",
		},
		{
			name: "script tag inside style label",
			mutate: func(s string) string {
				return strings.Replace(s, `"Warm Artisan"`, `"Warm <script>alert(1)</script>"`, 1)
			},
			wantMsg: "script",
		},
		{
			name: "inline onclick handler inside token",
			mutate: func(s string) string {
				return strings.Replace(s, `"#047857"`, `"#047857 onclick=alert(1)"`, 1)
			},
			wantMsg: "handler",
		},
		{
			name: "javascript URL in redirect target",
			mutate: func(s string) string {
				return strings.Replace(s, `"/products/ethiopian"`, `"javascript:alert(1)"`, 1)
			},
			wantMsg: "URL",
		},
		{
			name: "data URI in redirect target",
			mutate: func(s string) string {
				return strings.Replace(s, `"/products/ethiopian"`, `"data:text/html,<svg>"`, 1)
			},
			wantMsg: "URL",
		},
		{
			name: "vbscript URI in product path",
			mutate: func(s string) string {
				return strings.Replace(s, `"/products/micro-lot"`, `"vbscript:msgbox(1)"`, 1)
			},
			wantMsg: "URL",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mutated := tc.mutate(validSiteDNAv040)
			_, err := sitedna.ValidateBytes([]byte(mutated))
			if err == nil {
				t.Fatalf("expected Site DNA rejection for %s, got nil", tc.name)
			}
			if got := manifest.ExtractCode(err); got != manifest.ErrSiteDNAInvalid {
				t.Fatalf("expected code GRABMAN-E202, got %s (err=%v)", got, err)
			}
			_ = tc.wantMsg
		})
	}
}

func TestSiteDNADiffDetectsStylePriceAndHealthChanges(t *testing.T) {
	dir := t.TempDir()
	oldFile := filepath.Join(dir, "old.json")
	newFile := filepath.Join(dir, "new.json")

	updated := strings.Replace(validSiteDNAv040, `"warm-artisan"`, `"clean-editorial"`, 1)
	updated = strings.Replace(updated, `"Warm Artisan"`, `"Clean Editorial"`, 1)
	updated = strings.Replace(updated, `"price": "24.95"`, `"price": "26.50"`, 1)
	updated = strings.Replace(updated, `"#047857"`, `"#0F766E"`, 1)
	updated = strings.Replace(updated, `"site_health_score": 96`, `"site_health_score": 99`, 1)

	if err := os.WriteFile(oldFile, []byte(validSiteDNAv040), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(newFile, []byte(updated), 0o644); err != nil {
		t.Fatal(err)
	}

	diffRep, err := sitedna.DiffFiles(oldFile, newFile)
	if err != nil {
		t.Fatalf("DiffFiles failed: %v", err)
	}
	if !diffRep.HasChanges {
		t.Fatalf("expected HasChanges=true")
	}
	if diffRep.NightlyWatch.Delta != 3 {
		t.Fatalf("expected NightlyWatch.Delta=3, got %d", diffRep.NightlyWatch.Delta)
	}
	text := sitedna.FormatDiffText(diffRep)
	if !strings.Contains(text, `"24.95" USD -> "26.50" USD`) || !strings.Contains(text, "warm-artisan -> clean-editorial") {
		t.Fatalf("unexpected diff summary text:\n%s", text)
	}
}
