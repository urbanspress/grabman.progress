#!/usr/bin/env python3
"""
Generate plain-English HTML and PDF walkthrough guides for grabman releases.
Outputs:
  - docs/releases/html/Grabman-v<X.Y.Z>-Guide.html
  - docs/releases/pdf/Grabman-v<X.Y.Z>-Guide.pdf
"""

import os
import shutil
import subprocess
import sys
from pathlib import Path

RELEASE_DIR = Path(__file__).resolve().parent
HTML_DIR = RELEASE_DIR / "html"
PDF_DIR = RELEASE_DIR / "pdf"


def find_headless_browser() -> str:
    env_bin = os.environ.get("PDF_BROWSER_BIN")
    if env_bin and Path(env_bin).exists():
        return env_bin

    for candidate in ["google-chrome", "chromium", "chromium-browser", "msedge", "chrome"]:
        resolved = shutil.which(candidate)
        if resolved:
            return resolved

    windows_candidates = [
        Path(r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"),
        Path(r"C:\Program Files\Microsoft\Edge\Application\msedge.exe"),
        Path(r"C:\Program Files\Google\Chrome\Application\chrome.exe"),
        Path(r"C:\Program Files (x86)\Google\Chrome\Application\chrome.exe"),
    ]
    for path in windows_candidates:
        if path.exists():
            return str(path)

    raise FileNotFoundError(
        "No Chromium/Chrome/Edge executable found for HTML-to-PDF rendering. Set PDF_BROWSER_BIN."
    )


def render_v010_html() -> str:
    return """<!doctype html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Grabman v0.1.0 — Plain-English Architecture &amp; Walkthrough Guide</title>
  <style>
    @page { size: letter; margin: 0.68in; }
    body {
      font-family: "Segoe UI", system-ui, -apple-system, sans-serif;
      color: #0f172a;
      background: #ffffff;
      line-height: 1.55;
      font-size: 10.2pt;
      margin: 0;
    }
    header {
      border-bottom: 3px solid #047857;
      padding-bottom: 0.75rem;
      margin-bottom: 1.15rem;
    }
    h1 { font-size: 19.5pt; margin: 0 0 0.25rem 0; color: #0f172a; }
    h2 {
      font-size: 13pt;
      margin: 1.2rem 0 0.45rem 0;
      color: #047857;
      border-bottom: 1px solid #d1fae5;
      padding-bottom: 0.2rem;
      page-break-after: avoid;
    }
    h3 { font-size: 10.8pt; margin: 0.8rem 0 0.25rem 0; color: #0f172a; }
    p { margin: 0.35rem 0 0.6rem 0; }
    ul, ol { margin: 0.3rem 0 0.7rem 1.25rem; padding: 0; }
    li { margin-bottom: 0.28rem; }
    code {
      font-family: Consolas, "Courier New", monospace;
      font-size: 9pt;
      background: #f1f5f9;
      padding: 0.08rem 0.28rem;
      border-radius: 3px;
    }
    pre {
      background: #0f172a;
      color: #f8fafc;
      padding: 0.65rem 0.8rem;
      border-radius: 6px;
      font-family: Consolas, "Courier New", monospace;
      font-size: 8.8pt;
      overflow-x: auto;
      margin: 0.45rem 0 0.75rem 0;
      page-break-inside: avoid;
    }
    .subtitle { color: #334155; font-size: 10.5pt; margin: 0; }
    .meta { color: #475569; font-size: 9.2pt; margin-top: 0.4rem; }
    .callout {
      background: #ecfdf5;
      border-left: 4px solid #10b981;
      padding: 0.65rem 0.85rem;
      margin: 0.75rem 0;
      border-radius: 0 6px 6px 0;
      page-break-inside: avoid;
    }
    table {
      width: 100%;
      border-collapse: collapse;
      margin: 0.65rem 0 0.9rem 0;
      font-size: 9pt;
      page-break-inside: avoid;
    }
    th, td {
      border: 1px solid #cbd5e1;
      padding: 0.38rem 0.5rem;
      text-align: left;
      vertical-align: top;
    }
    th {
      background: #0f172a;
      color: #ffffff;
      font-weight: 600;
    }
    tr:nth-child(even) td { background: #f8fafc; }
    footer {
      margin-top: 1.5rem;
      padding-top: 0.6rem;
      border-top: 1px solid #cbd5e1;
      color: #475569;
      font-size: 8.8pt;
    }
  </style>
</head>
<body>
  <header>
    <h1>Grabman v0.1.0 &mdash; Plain-English Walkthrough Guide</h1>
    <p class="subtitle">Project Scaffolder, Compatibility Doctor, Site DNA Validator &amp; Deterministic <code>grabman.lock</code> Solver (Step 5 &bull; Layer 1)</p>
    <p class="meta"><strong>Release Tier:</strong> <code>PRE-1.0 v0.1.0</code> &bull; <strong>Runtime:</strong> Pure Go Stdlib Static Binary (<code>CGO_ENABLED=0</code>) &bull; <strong>Ecosystem Baseline:</strong> GRAB <code>v1.1.0</code> + 6-Layer Contract Matrix</p>
  </header>

  <div class="callout">
    <strong>What Grabman Does in Plain English:</strong> Before you deploy a GRAB application, CMS, or AI site, <code>grabman</code> builds a clean project skeleton that already compiles with <code>grab check</code>, verifies every ecosystem layer version matches without conflicts, locks SHA-256 contract hashes so nothing drifts silently, checks Groxy AI Site DNA files for exact money math and HTML safety, and blocks insecure server defaults before they hit production.
  </div>

  <section>
    <h2>1. Core Commands &amp; Quick Start</h2>
    <pre>grabman init --dir my-shop --profile ai-stack --name "Urban Roasters"
grabman init --dir my-shop --from-site-dna ./site-dna.json
grabman solve --dir my-shop --verify
grabman doctor --dir my-shop --strict
grabman dna validate ./site-dna.json --json
grabman dna diff ./old-dna.json ./new-dna.json</pre>
    <ul>
      <li><code>grabman init</code> &mdash; Creates ready-to-compile workspaces (<code>app</code>, <code>cms-stack</code>, <code>ai-stack</code>, or hydrated directly via <code>--from-site-dna</code>). Never executes remote shell installers (zero <code>curl | sh</code>) and never overwrites existing files unless <code>--force</code> is supplied.</li>
      <li><code>grabman solve [--verify]</code> &mdash; Computes a deterministic 6-layer compatibility lockfile (<code>grabman.lock</code>) and pinned SHA-256 artifact checksums. Passing <code>--verify</code> is strictly read-only and exits non-zero immediately on any drift or unapproved major jump to <code>v2.0.0</code>.</li>
      <li><code>grabman doctor [--strict] [--json]</code> &mdash; Audits installed GRAB runtime, workspace configs, theme mode rules, Site DNA exact decimals, template hygiene, and Docker Compose security settings, returning plain-English remediation steps.</li>
      <li><code>grabman dna validate / diff</code> &mdash; Validates <code>groxy-site-dna-v0.4.0</code> and <code>v1.0.0</code> bundles, preserving exact base-10 <code>ADR-025c</code> prices and flagging style, token, price, or site-health deltas before deployment.</li>
    </ul>
  </section>

  <section>
    <h2>2. The 6-Layer Ecosystem Compatibility Matrix</h2>
    <p>Every lockfile written by <code>grabman solve</code> enforces frozen <code>v1.x</code> SemVer ranges across all six ecosystem layers per <code>ADR-027c</code>:</p>
    <table>
      <thead>
        <tr>
          <th>Layer</th>
          <th>Repository</th>
          <th>Locked Range</th>
          <th>Baseline Pin</th>
          <th>Enforcement</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><strong>1. Core Language, VM &amp; Grisp</strong></td>
          <td><code>urbanspress/grab</code></td>
          <td><code>&gt;=1.1.0 &lt;2.0.0</code></td>
          <td><code>1.1.0</code></td>
          <td>Rejects <code>&lt;1.1.0</code> or unapproved <code>v2.0.0</code> jumps</td>
        </tr>
        <tr>
          <td><strong>2. Reactive Runtime &amp; Elements</strong></td>
          <td><code>urbanspress/grex-js</code></td>
          <td><code>^1.0.0</code></td>
          <td><code>1.0.0</code></td>
          <td>Pinned SHA-256 parity on <code>grex.elements.json</code> &amp; <code>grex.js</code></td>
        </tr>
        <tr>
          <td><strong>3. Flagship Zero-Plugin CMS</strong></td>
          <td><code>urbanspress/grablab-cms</code></td>
          <td><code>^1.0.1</code></td>
          <td><code>1.0.1</code></td>
          <td>Enforces hardened <code>v1.0.1</code> baseline and visual-mode compatibility</td>
        </tr>
        <tr>
          <td><strong>4. Visual Theme &amp; Studio</strong></td>
          <td><code>urbanspress/grex-builder</code></td>
          <td><code>^1.0.0</code></td>
          <td><code>1.0.0</code></td>
          <td>Validates exclusive <code>mode: "visual"</code> in <code>theme/theme.json</code></td>
        </tr>
        <tr>
          <td><strong>5. Autonomous Design &amp; SEO AI</strong></td>
          <td><code>urbanspress/groxy-ai</code></td>
          <td><code>^1.0.0</code></td>
          <td><code>1.0.0</code></td>
          <td>Pins <code>seo-playbook.v1.json</code> SHA-256 &amp; Site DNA contract</td>
        </tr>
        <tr>
          <td><strong>6. Scaffolder, Orchestrator &amp; Panel</strong></td>
          <td><code>grabman</code>, <code>grabtaculous</code>, <code>gashboard</code></td>
          <td><code>^0.1.0</code></td>
          <td><code>0.1.0</code></td>
          <td>Pre-<code>1.0</code> Step 5 integration layer across local CLI &amp; VPS deploys</td>
        </tr>
      </tbody>
    </table>
  </section>

  <section>
    <h2>3. Security Invariants &amp; Diagnostic Codes</h2>
    <table>
      <thead>
        <tr>
          <th>Code</th>
          <th>Guardrail</th>
          <th>What It Blocks in Practice</th>
        </tr>
      </thead>
      <tbody>
        <tr>
          <td><code>GRABMAN-E101</code></td>
          <td>Manifest &amp; Config Hygiene</td>
          <td>Malformed <code>grabman.json</code>, invalid profiles, non-visual CMS themes, or GRAB static compile errors.</td>
        </tr>
        <tr>
          <td><code>GRABMAN-E102</code></td>
          <td><code>ADR-027c</code> SemVer Matrix</td>
          <td>Downgrades below ecosystem baselines, wildcard ranges (<code>*</code>/<code>latest</code>), or unapproved <code>2.0.0</code> upgrades.</td>
        </tr>
        <tr>
          <td><code>GRABMAN-E103</code></td>
          <td>SHA-256 Lock &amp; Contract Drift</td>
          <td>Tampered <code>grabman.lock</code> digests, modified <code>grex.elements.json</code>/<code>grex.js</code>/<code>seo-playbook.v1.json</code>, or modified Site DNA.</td>
        </tr>
        <tr>
          <td><code>GRABMAN-E201</code></td>
          <td><code>GRAB-E401</code> Path Confinement</td>
          <td>Parent traversal (<code>..</code>), symlink escapes, and reads/writes touching <code>.git</code>, <code>.env</code>, <code>.ssh</code>, <code>*.key</code>, <code>*.pem</code>, <code>id_rsa</code>, or <code>id_ed25519</code>.</td>
        </tr>
        <tr>
          <td><code>GRABMAN-E202</code></td>
          <td>Site DNA &amp; <code>ADR-025c</code> Safety</td>
          <td>JSON number prices (float rounding), IEEE-754 drift (<code>0.30000000000000004</code>), wrapper <code>&lt;div&gt;</code> tags, <code>&lt;script&gt;</code>, <code>on*</code> handlers, and <code>javascript:</code>/<code>data:</code>/<code>vbscript:</code> URLs.</td>
        </tr>
        <tr>
          <td><code>GRABMAN-E301</code></td>
          <td><code>--read-only</code> Container Policy</td>
          <td>Missing <code>read_only: true</code>, missing <code>cap_drop: [ALL]</code> or <code>no-new-privileges:true</code>, <code>privileged: true</code>, <code>host</code> network, Docker socket mounts, <code>:latest</code> tags, non-loopback ports, hardcoded secrets, or private VPS IPs.</td>
        </tr>
      </tbody>
    </table>
  </section>

  <section>
    <h2>4. Workspace Architecture &amp; Zero-Dependency Guarantee</h2>
    <ul>
      <li><strong>Zero External Runtime Dependencies:</strong> Built cleanly with Go <code>1.26.0</code> standard library only (verified 0 vulnerabilities via <code>govulncheck</code> and <code>go mod verify</code>).</li>
      <li><strong>Generated Workspace Layout:</strong> Produces <code>grabman.json</code>, <code>grabman.lock</code>, <code>grab.platform.json</code>, <code>grab.data.json</code>, <code>src/app.grab</code>, semantic zero-<code>&lt;div&gt;</code> <code>views/home.grab.html</code>, externalized tokens stylesheet <code>public/assets/site.css</code>, optional <code>theme/theme.json</code> (<code>"mode": "visual"</code>), hardened <code>deploy/docker-compose.yml</code>, placeholder-only <code>deploy/.env.example</code>, and safe <code>.gitignore</code>.</li>
      <li><strong>Verified Live Ecosystem Parity:</strong> End-to-end tests verify live compilation through <code>GRAB v1.1.0</code> (<code>grab check</code>) and live <code>Site DNA</code> exports directly produced by <code>groxy-ai</code> (<code>public/groxy/site-context.js</code>).</li>
    </ul>
  </section>

  <footer>
    <p>Grabman v0.1.0 &bull; Urban's Press / GRAB Ecosystem &bull; Step 5 Layer 1 Complete (Next: Grabtaculous v0.1.0 Orchestrator)</p>
  </footer>
</body>
</html>
"""


def render_v100_html() -> str:
    html = render_v010_html()
    replacements = [
        ("Grabman v0.1.0", "Grabman v1.0.0"),
        ("PRE-1.0 v0.1.0", "v1.0.0 Production Stability Freeze"),
        ("Go <code>1.26.0</code>", "Go <code>1.27.4</code>"),
        ("<td><code>^0.1.0</code></td>\n          <td><code>0.1.0</code></td>\n          <td>Pre-<code>1.0</code> Step 5 integration layer across local CLI &amp; VPS deploys</td>",
         "<td><code>&gt;=0.1.0 &lt;2.0.0</code></td>\n          <td><code>1.0.0</code></td>\n          <td>Production-frozen Step 5 contract (dual support for <code>0.1.0</code> &amp; <code>1.0.0</code> schemas)</td>"),
        ("Step 5 Layer 1 Complete (Next: Grabtaculous v0.1.0 Orchestrator)",
         "Step 5 Layer 1 Production Freeze (Synchronized with Grabtaculous v1.0.0 &amp; Gashboard v1.0.0)"),
    ]
    for old, new in replacements:
        html = html.replace(old, new)
    return html


def render_v110_html() -> str:
    html = render_v100_html()
    replacements = [
        ("Grabman v1.0.0", "Grabman v1.1.0"),
        ("v1.0.0 Production Stability Freeze", "MINOR 1.1.0 &mdash; Interactive Scaffolder &amp; Dependency Health"),
        ("<td><code>1.0.0</code></td>\n          <td>Production-frozen Step 5 contract (dual support for <code>0.1.0</code> &amp; <code>1.0.0</code> schemas)</td>",
         "<td><code>1.1.0</code></td>\n          <td>Interactive <code>init</code>, system prereq gate (GRAB + Postgres), mandatory <code>public/grex/</code>, non-blocking <code>doctor</code> dependency warnings</td>"),
        ("Generates ready-to-compile workspaces (<code>app</code>, <code>cms-stack</code>, <code>ai-stack</code>, or hydrated directly via <code>--from-site-dna</code>). Never executes remote shell installers (zero <code>curl | sh</code>) and never overwrites existing files unless <code>--force</code> is supplied.",
         "Runs an interactive Next.js/Composer-style project survey (while keeping <code>--non-interactive</code> &amp; <code>--profile</code> for CI), enforces the System Prerequisite Gate (<code>grab &gt;= 1.1.0</code> + PostgreSQL check when GrabLab CMS is picked), bundles mandatory <strong>GRAB + Grex JS v1.0.0</strong> (<code>public/grex/</code>), and prompts only for optional GrabLab CMS and Groxy AI."),
        ("<code>grabman doctor [--strict] [--json]</code> &mdash; Audits installed GRAB runtime, workspace configs, theme mode rules, Site DNA exact decimals, template hygiene, and Docker Compose security settings, returning plain-English remediation steps.",
         "<code>grabman doctor [--strict] [--json]</code> &mdash; Includes a dedicated <strong>Dependencies &amp; Version Compatibility</strong> audit table (and JSON <code>dependencies</code> array) plus non-blocking warnings for overridden dependency pins, alongside live <code>grab check</code>, Site DNA exact decimals, template hygiene, and Compose security."),
        ("<code>grabman solve [--verify]</code> &mdash; Computes a deterministic 6-layer compatibility lockfile (<code>grabman.lock</code>) and pinned SHA-256 artifact checksums. Passing <code>--verify</code> is strictly read-only and exits non-zero immediately on any drift or unapproved major jump to <code>v2.0.0</code>.",
         "<code>grabman solve [--verify] [--warn-incompatible]</code> &mdash; Computes a deterministic 6-layer compatibility lockfile (<code>grabman.lock</code>). Supports non-blocking <code>--warn-incompatible</code> notices for intentional pins, while <code>--verify</code> fails closed on drift or unapproved <code>v2.0.0</code> jumps."),
        ("<strong>Generated Workspace Layout:</strong> Produces <code>grabman.json</code>, <code>grabman.lock</code>, <code>grab.platform.json</code>, <code>grab.data.json</code>, <code>src/app.grab</code>, semantic zero-<code>&lt;div&gt;</code> <code>views/home.grab.html</code>, externalized tokens stylesheet <code>public/assets/site.css</code>, optional <code>theme/theme.json</code> (<code>\"mode\": \"visual\"</code>), hardened <code>deploy/docker-compose.yml</code>, placeholder-only <code>deploy/.env.example</code>, and safe <code>.gitignore</code>.",
         "<strong>Canonical Grisp Workspace Layout:</strong> Produces <code>src/app.grab</code>, zero-<code>&lt;div&gt;</code> <code>views/index.grab.html</code> &amp; <code>views/home.grab.html</code>, bundled Grex JS assets (<code>public/grex/grex.js</code>, <code>tokens.css</code>, <code>grex.elements.json</code>), <code>storage/</code>, <code>uploads/</code>, symlink-confined <code>0600</code> <code>.env</code>, <code>.env.example</code>, <code>deploy/.env.example</code>, <code>grabman.json</code>, <code>grabman.lock</code>, <code>grab.platform.json</code>, <code>grab.data.json</code>, and hardened <code>--read-only</code> <code>deploy/docker-compose.yml</code>."),
        ("Step 5 Layer 1 Production Freeze (Synchronized with Grabtaculous v1.0.0 &amp; Gashboard v1.0.0)",
         "Step 5 v1.1.0 Interactive Scaffolder &amp; Dependency Doctor (Paired with GRAB v1.1.0 grab init &amp; Grabtaculous v1.1.0)"),
    ]
    for old, new in replacements:
        html = html.replace(old, new)
    return html


def main():
    version = sys.argv[1].lstrip("v") if len(sys.argv) > 1 else "1.0.0"
    renderers = {
        "0.1.0": render_v010_html,
        "1.0.0": render_v100_html,
        "1.1.0": render_v110_html,
    }
    if version not in renderers:
        raise ValueError(f"Unsupported guide version '{version}'. Available: {', '.join(sorted(renderers))}")

    HTML_DIR.mkdir(parents=True, exist_ok=True)
    PDF_DIR.mkdir(parents=True, exist_ok=True)

    html_path = HTML_DIR / f"Grabman-v{version}-Guide.html"
    pdf_path = PDF_DIR / f"Grabman-v{version}-Guide.pdf"
    html_path.write_text(renderers[version](), encoding="utf-8")

    browser = find_headless_browser()
    file_url = html_path.resolve().as_uri()
    cmd = [
        browser,
        "--headless",
        "--disable-gpu",
        "--no-pdf-header-footer",
        f"--print-to-pdf={pdf_path.resolve()}",
        file_url,
    ]
    proc = subprocess.run(cmd, stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, timeout=30)
    if proc.returncode != 0 or not pdf_path.exists() or pdf_path.stat().st_size == 0:
        raise RuntimeError(
            f"PDF generation failed ({proc.returncode}):\nSTDOUT: {proc.stdout}\nSTDERR: {proc.stderr}"
        )
    print(f"Generated {html_path}")
    print(f"Generated {pdf_path} ({pdf_path.stat().st_size} bytes)")


if __name__ == "__main__":
    try:
        main()
    except Exception as exc:
        print(f"[ERROR] {exc}", file=sys.stderr)
        sys.exit(1)
