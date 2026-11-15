# Changelog — `grabman`

All notable changes to **`grabman`** follow strict Semantic Versioning (`MAJOR.MINOR.PATCH` per `ADR-027c`).

---

## `v0.1.0` — Project Scaffolder, Compatibility Doctor, `Site DNA` Validator & Deterministic `grabman.lock` Solver (2026-11-15)

- **SemVer Tier:** `PRE-1.0 v0.1.0` (Step 5 Layer 1 baseline deliverable)
- **Ecosystem Impact:** Zero breaking changes to frozen upstream `v1.x` contracts (`GRAB v1.1.0`, `Grex JS v1.0.0`, `GrabLab CMS v1.0.1`, `Grex Theme Builder v1.0.0`, `Groxy AI v1.0.0`).

### Added
- **Pure Go Stdlib CLI (`cmd/grabman`):** Zero external runtime dependencies (`go 1.26.0`, `CGO_ENABLED=0`) supporting `init`, `doctor`, `solve`, `dna validate`, `dna diff`, and `version`.
- **Deterministic Project Scaffolder (`internal/scaffolder`):**
  - Curated profiles: `app`, `cms-stack`, `ai-stack`, plus `--from-site-dna` hydration from Groxy AI bundles.
  - Emits `grab check`-passing `src/app.grab`, semantic zero-`<div>` `views/home.grab.html`, externalized `public/assets/site.css` tokens stylesheet, `grab.platform.json`, `grab.data.json`, `mode: "visual"` `theme/theme.json`, placeholder-only `deploy/.env.example`, and hardened `--read-only` `deploy/docker-compose.yml`.
  - Non-destructive `--force` guard and `GRAB-E401` path confinement (`GRABMAN-E201`) strictly blocking `.git`, `.env`, `.ssh`, `*.key`, `*.pem`, `id_rsa`, `id_ed25519`, symlinks escaping workspace root, and `..` traversal.
- **6-Layer SemVer Solver & SHA-256 Pinning (`internal/solver`):**
  - Enforces all 6 ecosystem layer constraints (`GRAB >=1.1.0`, `Grex JS ^1.0.0`, `GrabLab CMS ^1.0.1`, `Grex Builder ^1.0.0`, `Groxy AI ^1.0.0`, `Step 5 ^0.1.0`) and frozen upstream SHA-256 contract digests (`grex.elements.json`, `grex.js`, `seo-playbook.v1.json`) into byte-for-byte deterministic `grabman.lock`.
  - `grabman solve --verify` fails closed (`GRABMAN-E102` / `GRABMAN-E103`) on unapproved `v2.0.0` jumps, version downgrades, wildcards, or checksum tampering.
- **`Site DNA` Bridge (`internal/sitedna`):**
  - Validates `groxy-site-dna-v0.4.0` / `v1.0.0` exports against `ADR-025c` exact base-10 wire decimals and strict zero-`<div>` / XSS hygiene (rejecting float numbers, IEEE-754 drift, scientific notation, `<script>`, `on*`, `javascript:`, `data:`, and `vbscript:`).
  - `grabman dna diff` computes deterministic human-readable and JSON deltas across design tokens, active style packs, product prices, redirects, and Nightly Watch health scores.
- **Compatibility & Security Doctor (`internal/doctor`):**
  - Plain-English and `--json` workspace audit (`--strict` option) verifying live `grab check`, lockfile integrity, `theme.json`, container `--read-only` hardening, secret/private-IP redaction (`GRABMAN-E301`), and template DOM hygiene.
- **Verification & Release Artifacts:**
  - Full `go test -count=1 ./...` unit & CLI integration suite (including live `GRAB v1.1.0` `grab check` and live `Groxy AI` `Site DNA` export compatibility), `go mod verify`, `govulncheck`, static `CGO_ENABLED=0` binaries for `windows/amd64` and `linux/amd64`, `SHA256SUMS.txt`, and [`docs/releases/pdf/Grabman-v0.1.0-Guide.pdf`](docs/releases/pdf/Grabman-v0.1.0-Guide.pdf).
