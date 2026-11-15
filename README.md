# `grabman`

![Built with GRAB v1.1.0](docs/badges/built-with-grab.svg)
![GRAB Ecosystem Step 5 — grabman](docs/badges/ecosystem-step-5.svg)
![grabman v1.0.0 Complete](docs/badges/release-v1.0.0.svg)

**`grabman`** is the Ecosystem Project Scaffolder, Compatibility Doctor, `Site DNA` Validator, and Deterministic `grabman.lock` Solver for **Step 5 of the Post-1.0 GRAB Ecosystem**.

- **Current Status:** **`v1.0.0` — Complete (Production Freeze)** ([`specifications/GRABMAN-SPEC-100.md`](specifications/GRABMAN-SPEC-100.md) | [Walkthrough PDF](docs/releases/pdf/Grabman-v1.0.0-Guide.pdf) | [Changelog](CHANGELOG.md))
- **Development Repo (`origin`):** `https://github.com/urbanspress/grabman.progress`
- **Production Release Repo (`stable`):** `https://github.com/urbanspress/grabman.stable`

---

## Core Capabilities (`v1.0.0`)

1. **Deterministic Project Scaffolder (`grabman init`):**
   - Generates clean, `grab check`-ready workspaces (`app`, `cms-stack`, `ai-stack`, or hydrated directly from a **Groxy AI** `--from-site-dna` snapshot) with `grabman.json`, `grabman.lock`, `grab.platform.json`, `grab.data.json`, semantic zero-`<div>` `.grab.html` templates, and `--read-only` Docker Compose baselines.
   - Strictly confines paths inside the target workspace (`GRAB-E401` parity blocking `.git`, `.env`, `.ssh`, `*.key`, `*.pem`, `id_rsa`, `id_ed25519`, symlinks escaping root, and `..`), refuses partial overwrites without `--force`, and runs zero remote `curl | sh` scripts.
2. **Plain-English Compatibility & Security Doctor (`grabman doctor`):**
   - Audits local `GRAB v1.1.0` compilation (`grab check`), `theme/theme.json` `mode: "visual"` exclusivity, frozen contract SHA-256 parity (`grex.elements.json`, `grex.js`, `seo-playbook.v1.json`), container hardening (`read_only: true`, `cap_drop: ["ALL"]`, `no-new-privileges:true`, pinned image tags, `127.0.0.1` loopback bindings), zero hardcoded secrets/private IPs (redacted in all outputs), and `Site DNA` precision. Outputs both plain-English advice and machine-readable `--json` (`--strict`).
3. **`grabman.lock` 6-Layer Compatibility Solver (`grabman solve`):**
   - Resolves and pins SemVer (`ADR-027c`) bounds and SHA-256 contract hashes across all 6 ecosystem layers into deterministic `grabman.lock` files; `solve --verify` fails closed (`exit != 0`) on any lockfile/contract drift, version downgrade, or unapproved jump to `v2.0.0`.
4. **`Site DNA` Bridge (`grabman dna`):**
   - Validates and diffs portable `groxy-site-dna-v0.4.0` / `v1.0.0` bundles exported from **Groxy AI** (`GET /api/groxy/site-dna/export`), enforcing exact base-10 `ADR-025c` `GrabDecimal`/`GrabInteger` wire strings and rejecting IEEE float drift, `<div>`, `<script>`, `on*`, or unsafe URLs (`javascript:`, `data:`, `vbscript:`).

---

## Quick Start

```bash
# Build static binary from source (pure Go stdlib, CGO_ENABLED=0)
CGO_ENABLED=0 go build -trimpath -o grabman ./cmd/grabman

# Scaffold a hardened GRAB + GrabLab + Groxy AI workspace
grabman init --dir ./my-shop --profile ai-stack --name "Urban Roasters"

# Hydrate a storefront directly from a Groxy AI Site DNA export
grabman init --dir ./my-shop --from-site-dna ./site-dna.json

# Verify 6-layer compatibility & SHA-256 lockfile deterministically
grabman solve --dir ./my-shop --verify

# Run full security, config, template & live `grab check` audit
grabman doctor --dir ./my-shop --strict

# Validate & diff Groxy AI Site DNA bundles
grabman dna validate ./site-dna.json --json
grabman dna diff ./old-site-dna.json ./new-site-dna.json
```

---

## 6-Layer Ecosystem Compatibility Matrix

| Ecosystem Layer | Repository | Required Version Range | Role |
|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | Executes `.grab`/`.grab.html`, `grab check`, `grab migrate`, `grab mcp` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `grex.elements.json` & reactive UI runtime contract |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` | Zero-plugin CMS & exclusive theme host |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `GRAB-DESIGN-SYSTEM.md` tokens & visual theme |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` | `Site DNA` export (`groxy-site-dna-v0.4.0`) & MCP design agent |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v0.1.0 (< v2.0.0)` (`v1.0.0` Frozen) | Project scaffolding, lockfile solving, `--read-only` rollouts & desktop VPS panel |

---

## Governance & Documentation

- [Walkthrough PDF Guide (`docs/releases/pdf/Grabman-v1.0.0-Guide.pdf`)](docs/releases/pdf/Grabman-v1.0.0-Guide.pdf)
- [Changelog (`CHANGELOG.md`)](CHANGELOG.md)
- [`v1.0.0` Production Freeze Specification (`specifications/GRABMAN-SPEC-100.md`)](specifications/GRABMAN-SPEC-100.md)
- [`v0.1.0` Specification (`specifications/GRABMAN-SPEC-001.md`)](specifications/GRABMAN-SPEC-001.md)
- [Roadmap (`docs/ROADMAP.md`)](docs/ROADMAP.md)
- [Engineering Invariants (`GRABMAN-EXPERT.md`)](GRABMAN-EXPERT.md)
- [Agent Rules (`AGENTS.md`)](AGENTS.md)
- [Handoff Log (`HANDOFF.md`)](HANDOFF.md)
- [License (`LICENSE` — MIT)](LICENSE)
