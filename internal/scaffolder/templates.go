package scaffolder

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"sort"
	"strings"
	"unicode"

	"grabman/internal/manifest"
	"grabman/internal/sitedna"
)

type scaffoldContext struct {
	ProjectName    string
	ProjectSlug    string
	Profile        string
	EdgeProxy      string
	StylePackID    string
	StylePackLabel string
	PrimaryColor   string
	SurfaceColor   string
	InkColor       string
	AccentColor    string
	MutedColor     string
	FontHeading    string
	FontBody       string
	ProductCount   int
	Products       []sitedna.Product
	SiteURL        string
	BundleName     string
	FromSiteDNA    bool
}

func buildScaffoldContext(opts InitOptions, dna *sitedna.Bundle) scaffoldContext {
	name := strings.TrimSpace(opts.ProjectName)
	if name == "" && dna != nil && strings.TrimSpace(dna.SiteName) != "" {
		name = strings.TrimSpace(dna.SiteName)
	}
	if name == "" {
		name = "grab-project"
	}

	ctx := scaffoldContext{
		ProjectName:    name,
		ProjectSlug:    sanitizeSlug(name),
		Profile:        opts.Profile,
		EdgeProxy:      opts.EdgeProxy,
		StylePackID:    "clean-editorial",
		StylePackLabel: "Clean Editorial",
		PrimaryColor:   "#047857",
		SurfaceColor:   "#F8FAFC",
		InkColor:       "#0F172A",
		AccentColor:    "#10B981",
		MutedColor:     "#475569",
		FontHeading:    "system-ui, -apple-system, sans-serif",
		FontBody:       "system-ui, -apple-system, sans-serif",
		ProductCount:   0,
		SiteURL:        "http://127.0.0.1:8080",
	}

	if dna == nil {
		return ctx
	}

	ctx.FromSiteDNA = true
	ctx.BundleName = dna.BundleName
	if strings.TrimSpace(dna.ActiveStylePackID) != "" {
		ctx.StylePackID = strings.TrimSpace(dna.ActiveStylePackID)
	}
	if strings.TrimSpace(dna.ActiveStylePackLabel) != "" {
		ctx.StylePackLabel = strings.TrimSpace(dna.ActiveStylePackLabel)
	}
	if strings.TrimSpace(dna.SiteURL) != "" {
		ctx.SiteURL = strings.TrimSpace(dna.SiteURL)
	}
	ctx.PrimaryColor = firstSafeCSS(dna.Tokens, []string{"primary", "brand", "color-primary", "accent"}, ctx.PrimaryColor, false)
	ctx.SurfaceColor = firstSafeCSS(dna.Tokens, []string{"surface", "background", "bg", "color-surface"}, ctx.SurfaceColor, false)
	ctx.InkColor = firstSafeCSS(dna.Tokens, []string{"ink", "text", "foreground", "color-ink"}, ctx.InkColor, false)
	ctx.AccentColor = firstSafeCSS(dna.Tokens, []string{"accent", "highlight", "secondary", "color-accent"}, ctx.AccentColor, false)
	ctx.MutedColor = firstSafeCSS(dna.Tokens, []string{"muted", "subtext", "color-muted"}, ctx.MutedColor, false)
	ctx.FontHeading = firstSafeCSS(dna.Tokens, []string{"font_heading", "font-heading", "heading-font"}, ctx.FontHeading, true)
	ctx.FontBody = firstSafeCSS(dna.Tokens, []string{"font_body", "font-body", "body-font"}, ctx.FontBody, true)
	ctx.Products = append([]sitedna.Product(nil), dna.Products...)
	ctx.ProductCount = len(ctx.Products)
	return ctx
}

func firstSafeCSS(tokens map[string]string, keys []string, fallback string, isFont bool) string {
	for _, k := range keys {
		for tk, tv := range tokens {
			if strings.EqualFold(strings.TrimSpace(tk), k) {
				if sanitized := sanitizeCSSValue(tv, isFont); sanitized != "" {
					return sanitized
				}
			}
		}
	}
	return fallback
}

func sanitizeCSSValue(raw string, isFont bool) string {
	val := strings.TrimSpace(raw)
	if val == "" || len(val) > 120 {
		return ""
	}
	lower := strings.ToLower(val)
	for _, bad := range []string{";", "{", "}", "<", ">", "`", "\"", "'", "url(", "expression(", "javascript:", "data:", "vbscript:"} {
		if strings.Contains(lower, bad) && !(isFont && (bad == "'" || bad == "\"")) {
			return ""
		}
	}
	for _, r := range val {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return val
}

func sanitizeSlug(name string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(strings.TrimSpace(name)) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if !lastDash && b.Len() > 0 {
			b.WriteByte('-')
			lastDash = true
		}
	}
	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "grab-project"
	}
	return slug
}

func escapeGrabString(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '\\' || r == '"' {
			b.WriteByte('\\')
			b.WriteRune(r)
		} else if unicode.IsControl(r) {
			b.WriteByte(' ')
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func renderManifest(ctx scaffoldContext) ([]byte, error) {
	siteDNAPath := ""
	if ctx.FromSiteDNA {
		siteDNAPath = "storage/site-dna.json"
	}
	m := manifest.Manifest{
		SchemaVersion: manifest.SchemaVersionV100,
		ProjectName:   ctx.ProjectName,
		Profile:       ctx.Profile,
		TargetRuntime: map[string]string{
			"grab":         ">=1.1.0 <2.0.0",
			"grex-js":      "^1.0.0",
			"grablab-cms":  "^1.0.1",
			"grex-builder": "^1.0.0",
			"groxy-ai":     "^1.0.0",
			"step-5":       ">=0.1.0 <2.0.0",
		},
		SiteDNAPath: siteDNAPath,
		DeploySecurity: manifest.DeploySecurity{
			ReadOnlyRoot:      true,
			NonRootUID:        "10001:10001",
			LoopbackOnlyPorts: true,
			EdgeProxy:         ctx.EdgeProxy,
		},
	}
	return manifest.CanonicalJSON(m)
}

func renderPlatformConfig(ctx scaffoldContext) ([]byte, error) {
	cfg := map[string]any{
		"version": 1,
		"caches": map[string]any{
			"app_cache": map[string]any{
				"driver":    "memory",
				"namespace": ctx.ProjectSlug,
				"max_keys":  5000,
				"max_bytes": 16777216,
			},
		},
	}
	return manifest.CanonicalJSON(cfg)
}

func renderDataConfig(ctx scaffoldContext) ([]byte, error) {
	cfg := map[string]any{
		"connections": map[string]any{
			"main": map[string]any{
				"driver": "sqlite",
				"path":   fmt.Sprintf("storage/%s.db", ctx.ProjectSlug),
			},
		},
	}
	return manifest.CanonicalJSON(cfg)
}

func renderThemeConfig(ctx scaffoldContext) ([]byte, error) {
	cfg := map[string]any{
		"schema_version":     "1.0.0",
		"mode":               "visual",
		"style_pack_id":      ctx.StylePackID,
		"style_pack_label":   ctx.StylePackLabel,
		"builder_contract":   "GREX-BUILDER-SPEC-100",
		"zero_wrapper_divs":  true,
		"tokens": map[string]string{
			"primary":      ctx.PrimaryColor,
			"surface":      ctx.SurfaceColor,
			"ink":          ctx.InkColor,
			"accent":       ctx.AccentColor,
			"muted":        ctx.MutedColor,
			"font_heading": ctx.FontHeading,
			"font_body":    ctx.FontBody,
		},
	}
	return manifest.CanonicalJSON(cfg)
}

func renderGrabEntry(ctx scaffoldContext) []byte {
	var b strings.Builder
	b.WriteString("use cache\n")
	b.WriteString("use data\n")
	b.WriteString("use server\n\n")
	b.WriteString("set app_cache to cache.open using \"app_cache\"\n")
	b.WriteString("set db to data.open using \"main\"\n\n")
	b.WriteString("route \"GET\" \"/\" using request\n")
	fmt.Fprintf(&b, "  give back server.html using \"../views/index.grab.html\", record {project_name: %q, profile: %q, style_pack: %q, primary_color: %q, product_count: \"%d\"}\n",
		escapeGrabString(ctx.ProjectName),
		escapeGrabString(ctx.Profile),
		escapeGrabString(ctx.StylePackLabel),
		escapeGrabString(ctx.PrimaryColor),
		ctx.ProductCount,
	)
	b.WriteString("end\n\n")
	b.WriteString("route \"GET\" \"/home\" using request\n")
	fmt.Fprintf(&b, "  give back server.html using \"../views/home.grab.html\", record {project_name: %q, profile: %q, style_pack: %q, primary_color: %q, product_count: \"%d\"}\n",
		escapeGrabString(ctx.ProjectName),
		escapeGrabString(ctx.Profile),
		escapeGrabString(ctx.StylePackLabel),
		escapeGrabString(ctx.PrimaryColor),
		ctx.ProductCount,
	)
	b.WriteString("end\n\n")
	b.WriteString("route \"GET\" \"/health\" using request\n")
	fmt.Fprintf(&b, "  give back server.json using 200, record {status: \"ok\", project: %q, profile: %q, style_pack: %q, decimal_mode: \"ADR-025c\"}\n",
		escapeGrabString(ctx.ProjectName),
		escapeGrabString(ctx.Profile),
		escapeGrabString(ctx.StylePackID),
	)
	b.WriteString("end\n\n")
	b.WriteString("route \"GET\" \"/_ready\" using request\n")
	fmt.Fprintf(&b, "  give back server.json using 200, record {status: \"ready\", project: %q, profile: %q}\n",
		escapeGrabString(ctx.ProjectName),
		escapeGrabString(ctx.Profile),
	)
	b.WriteString("end\n")
	return []byte(b.String())
}

func renderHomeTemplate(ctx scaffoldContext) []byte {
	var productsBlock strings.Builder
	if len(ctx.Products) > 0 {
		productsBlock.WriteString("    <section aria-labelledby=\"catalog-heading\">\n")
		productsBlock.WriteString("      <h2 id=\"catalog-heading\">Verified ADR-025c Catalog</h2>\n")
		productsBlock.WriteString("      <ul>\n")
		for _, p := range ctx.Products {
			title := html.EscapeString(strings.TrimSpace(p.Title))
			if title == "" {
				title = html.EscapeString(strings.TrimSpace(p.SKU))
			}
			price := html.EscapeString(strings.TrimSpace(p.Price))
			currency := html.EscapeString(strings.TrimSpace(p.Currency))
			if currency == "" {
				currency = "USD"
			}
			fmt.Fprintf(&productsBlock, "        <li><strong>%s</strong> — <data value=\"%s\">%s %s</data></li>\n", title, price, currency, price)
		}
		productsBlock.WriteString("      </ul>\n")
		productsBlock.WriteString("    </section>\n")
	}

	return []byte(fmt.Sprintf(`<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>{project_name} — GRAB Workspace</title>
  <link rel="stylesheet" href="/grex/tokens.css">
  <link rel="stylesheet" href="/assets/site.css">
</head>
<body>
  <main>
    <header>
      <h1>{project_name}</h1>
      <p>Profile: <strong>{profile}</strong> | Style Pack: <strong>{style_pack}</strong></p>
    </header>
    <section aria-labelledby="runtime-heading">
      <h2 id="runtime-heading">Ecosystem Runtime Contract</h2>
      <p>Scaffolded deterministically by grabman with Grex JS v1.0.0, pinned SHA-256 contracts, semantic HTML5 layout (zero wrapper div tags), and ADR-025c base-10 wire precision.</p>
      <ul>
        <li>Primary Accent Token: <code>{primary_color}</code></li>
        <li>Hydrated Catalog Items: <code>{product_count}</code></li>
        <li>Grex Runtime: <code>/grex/grex.js</code> + <code>/grex/tokens.css</code></li>
        <li>Health Endpoint: <a href="/health">/health</a></li>
      </ul>
    </section>
%s    <footer>
      <p>Built on the GRAB Ecosystem (v1.1.0 + Grex JS v1.0.0).</p>
    </footer>
  </main>
</body>
</html>
`,
		productsBlock.String(),
	))
}

func renderStylesheet(ctx scaffoldContext) []byte {
	return []byte(fmt.Sprintf(`:root {
  --grab-primary: %s;
  --grab-surface: %s;
  --grab-ink: %s;
  --grab-accent: %s;
  --grab-muted: %s;
  --grab-font-heading: %s;
  --grab-font-body: %s;
}

body {
  margin: 0;
  padding: 2rem;
  font-family: var(--grab-font-body);
  color: var(--grab-ink);
  background: var(--grab-surface);
  line-height: 1.6;
}

main {
  max-width: 56rem;
  margin: 0 auto;
}

header {
  border-bottom: 2px solid var(--grab-accent);
  padding-bottom: 1rem;
  margin-bottom: 1.5rem;
}

h1, h2 {
  font-family: var(--grab-font-heading);
  color: var(--grab-primary);
}

article, section {
  background: #ffffff;
  border: 1px solid #e2e8f0;
  border-radius: 0.75rem;
  padding: 1.25rem;
  margin-bottom: 1.25rem;
}

footer {
  color: var(--grab-muted);
  font-size: 0.9rem;
  margin-top: 2rem;
}
`,
		ctx.PrimaryColor,
		ctx.SurfaceColor,
		ctx.InkColor,
		ctx.AccentColor,
		ctx.MutedColor,
		ctx.FontHeading,
		ctx.FontBody,
	))
}

func renderDockerCompose(ctx scaffoldContext) []byte {
	var b strings.Builder
	b.WriteString("# Hardened read-only production Compose baseline generated by grabman v1.1.0\n")
	b.WriteString("services:\n")

	writeHardenedService(&b, "grab-app", "ghcr.io/urbanspress/grab:v1.1.0",
		[]string{"127.0.0.1:8080:8080"},
		[]string{"../:/app:ro", "app-storage:/app/storage", "app-uploads:/app/uploads"},
		[]string{
			fmt.Sprintf("GRAB_PROJECT_NAME=%s", ctx.ProjectSlug),
			"GRAB_PLATFORM_CONFIG=/app/grab.platform.json",
			"GRAB_DB_CONFIG=/app/grab.data.json",
			"GRAB_APP_SECRET=${GRAB_APP_SECRET}",
		},
		[]string{
			"grab", "serve",
			"--addr", "0.0.0.0:8080",
			"--allow-public",
			"--allow-scripts",
			"--static", "/app/public",
			"--allow-read", "/app",
			"--platform-config", "/app/grab.platform.json",
			"--db-config", "/app/grab.data.json",
			"/app/src/app.grab",
		},
	)

	if ctx.Profile == "cms-stack" || ctx.Profile == "ai-stack" {
		writeHardenedService(&b, "grablab-cms", "ghcr.io/urbanspress/grablab-cms:v1.0.1",
			[]string{"127.0.0.1:8081:8081"},
			[]string{"cms-storage:/app/storage"},
			[]string{
				"GRABLAB_MASTER_KEY=${GRABLAB_MASTER_KEY}",
				"GRABLAB_VISUAL_MODE=true",
			},
			nil,
		)
	}

	if ctx.Profile == "ai-stack" {
		writeHardenedService(&b, "groxy-ai", "ghcr.io/urbanspress/groxy-ai:v1.0.0",
			[]string{"127.0.0.1:8090:8090"},
			[]string{"groxy-storage:/app/storage"},
			[]string{
				"GROXY_PROVIDER_API_KEY=${GROXY_PROVIDER_API_KEY}",
				"GROXY_SITE_DNA_EXPORT=readonly",
			},
			nil,
		)
	}

	switch ctx.EdgeProxy {
	case "nginx":
		writeHardenedService(&b, "edge-proxy", "nginx:1.27-alpine",
			[]string{"127.0.0.1:8443:8443"},
			[]string{"nginx-cache:/var/cache/nginx"},
			nil,
			nil,
		)
	case "cloudflare-tunnel":
		writeHardenedService(&b, "edge-proxy", "cloudflare/cloudflared:2025.2.1",
			nil,
			nil,
			[]string{"TUNNEL_TOKEN=${CLOUDFLARE_TUNNEL_TOKEN}"},
			nil,
		)
	default:
		writeHardenedService(&b, "edge-proxy", "caddy:2.8-alpine",
			[]string{"127.0.0.1:8443:8443"},
			[]string{"caddy-data:/data", "caddy-config:/config"},
			nil,
			nil,
		)
	}

	b.WriteString("\nvolumes:\n")
	b.WriteString("  app-storage:\n")
	b.WriteString("  app-uploads:\n")
	if ctx.Profile == "cms-stack" || ctx.Profile == "ai-stack" {
		b.WriteString("  cms-storage:\n")
	}
	if ctx.Profile == "ai-stack" {
		b.WriteString("  groxy-storage:\n")
	}
	switch ctx.EdgeProxy {
	case "nginx":
		b.WriteString("  nginx-cache:\n")
	case "caddy":
		b.WriteString("  caddy-data:\n")
		b.WriteString("  caddy-config:\n")
	}

	return []byte(b.String())
}

func writeHardenedService(b *strings.Builder, name, image string, ports, volumes, env, cmd []string) {
	fmt.Fprintf(b, "  %s:\n", name)
	fmt.Fprintf(b, "    image: %s\n", image)
	b.WriteString("    read_only: true\n")
	b.WriteString("    user: \"10001:10001\"\n")
	b.WriteString("    cap_drop:\n")
	b.WriteString("      - ALL\n")
	b.WriteString("    security_opt:\n")
	b.WriteString("      - no-new-privileges:true\n")
	b.WriteString("    tmpfs:\n")
	b.WriteString("      - /tmp:rw,noexec,nosuid,size=64m\n")
	if len(ports) > 0 {
		b.WriteString("    ports:\n")
		for _, p := range ports {
			fmt.Fprintf(b, "      - %q\n", p)
		}
	}
	if len(volumes) > 0 {
		b.WriteString("    volumes:\n")
		for _, v := range volumes {
			fmt.Fprintf(b, "      - %s\n", v)
		}
	}
	if len(env) > 0 {
		sort.Strings(env)
		b.WriteString("    environment:\n")
		for _, e := range env {
			fmt.Fprintf(b, "      - %s\n", e)
		}
	}
	if len(cmd) > 0 {
		b.WriteString("    command:\n")
		for _, c := range cmd {
			fmt.Fprintf(b, "      - %q\n", c)
		}
	}
	b.WriteString("    restart: unless-stopped\n")
}

func renderEnvExample(ctx scaffoldContext) []byte {
	var b strings.Builder
	b.WriteString("# Environment template — populate via secret manager on target host; never commit real .env files.\n")
	fmt.Fprintf(&b, "GRAB_PROJECT_NAME=%s\n", ctx.ProjectSlug)
	b.WriteString("GRAB_APP_SECRET=<set-via-secret-manager>\n")
	if ctx.Profile == "cms-stack" || ctx.Profile == "ai-stack" {
		b.WriteString("GRABLAB_MASTER_KEY=<set-via-secret-manager>\n")
	}
	if ctx.Profile == "ai-stack" {
		b.WriteString("GROXY_PROVIDER_API_KEY=<set-via-secret-manager>\n")
	}
	if ctx.EdgeProxy == "cloudflare-tunnel" {
		b.WriteString("CLOUDFLARE_TUNNEL_TOKEN=<set-via-secret-manager>\n")
	}
	return []byte(b.String())
}

func randomHexSecret(nBytes int) (string, error) {
	buf := make([]byte, nBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf), nil
}

func renderDotEnv(ctx scaffoldContext) ([]byte, error) {
	appSecret, err := randomHexSecret(32)
	if err != nil {
		return nil, err
	}
	masterKey, err := randomHexSecret(32)
	if err != nil {
		return nil, err
	}
	var b strings.Builder
	b.WriteString("# Auto-generated local runtime secrets (0600, gitignored — do not commit)\n")
	fmt.Fprintf(&b, "GRAB_PROJECT_NAME=%s\n", ctx.ProjectSlug)
	b.WriteString("GRAB_ADDR=127.0.0.1:8080\n")
	fmt.Fprintf(&b, "GRAB_APP_SECRET=%s\n", appSecret)
	fmt.Fprintf(&b, "GRABLAB_MASTER_KEY=%s\n", masterKey)
	b.WriteString("GRABLAB_UPLOAD_DIR=./uploads\n")
	if ctx.Profile == "ai-stack" {
		b.WriteString("GROXY_PROVIDER_API_KEY=\n")
	}
	if ctx.EdgeProxy == "cloudflare-tunnel" {
		b.WriteString("CLOUDFLARE_TUNNEL_TOKEN=\n")
	}
	return []byte(b.String()), nil
}

func renderGitignore() []byte {
	return []byte(`# Sensitive runtime state and local credentials (never commit)
.env
.env.*
!.env.example
!deploy/.env.example
*.key
*.pem
.ssh/
id_rsa
id_ed25519
storage/*.db
storage/*.db-wal
storage/*.db-shm
dist/
`)
}
