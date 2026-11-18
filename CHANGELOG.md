# Changelog — `grabman`

All notable changes to **`grabman`** follow strict Semantic Versioning (`MAJOR.MINOR.PATCH` per `ADR-027c`).

---

## `v1.1.0` — Interactive Scaffolder, System Prerequisite Gate & Non-Blocking Dependency Health (2026-11-18)

- **SemVer Tier:** `MINOR 1.x.0 — Backwards-Compatible Feature Release` (`100%` backwards compatible with `v0.1.0` and `v1.0.0` manifests, lockfiles, and CLI flags)
- **Ecosystem Impact:** Zero breaking changes to frozen upstream `v1.x` contracts (`GRAB v1.1.0`, `Grex JS v1.0.0`, `Grex Theme Builder v1.0.0`, `Groxy AI v1.0.0+`). Recognizes `GrabLab CMS v1.1.0`, `v1.4.0`, and `v1.4.1`, plus `Step 5 v1.1.0`.

### Added & Changed
- **Next.js / Composer-Style Interactive `grabman init` (`internal/scaffolder/prereq.go`, `cmd/grabman/main.go`):**
  - Added interactive prompt flow (`--interactive` auto-detected on TTY or forced via flag; `--non-interactive` / `--profile` preserved for CI) automatically bundling **GRAB `>=1.1.0`** and **Grex JS `v1.0.0`** (`public/grex/grex.js`, `public/grex/tokens.css`, `public/grex/grex.elements.json`) and prompting only for optional **GrabLab CMS** (`Y/n`) and **Groxy AI** (`y/N`).
  - Added **System Prerequisite Gate** checking `grab >= 1.1.0` (customizable via `--grab-bin`) and PostgreSQL (`psql` / `pg_isready` / Docker Postgres service) whenever GrabLab CMS is selected, printing plain-English setup steps if missing.
  - Upgraded scaffold layout to canonical Grisp structure (`src/app.grab`, `views/index.grab.html`, `views/home.grab.html`, `public/grex/*`, `storage/`, `uploads/`, symlink-confined `0600` `.env`, `.env.example`, `deploy/.env.example`, and `--read-only` `deploy/docker-compose.yml` mounting `--static /app/public` and `app-uploads`).
- **Dependency Health Section & Non-Blocking Incompatible-Pin Warnings (`internal/doctor/`, `internal/solver/`):**
  - Added `Dependencies & Version Compatibility` plain-English section and `--json` `dependencies` array to `grabman doctor` auditing every layer against `EcosystemMatrix()`.
  - Added `--warn-incompatible` (`SolveOptions.WarnIncompatible`, `ComputeLockfileWithWarnings`, `InspectDependencies`) so intentional incompatible dependency overrides in `grabman.json` emit clear non-blocking `WARNING` notices instead of breaking local workflows, while contract/lock tampering continues to fail closed.
- **Cross-Platform Release Artifacts:**
  - Built static `CGO_ENABLED=0` binaries for `windows/amd64`, `linux/amd64`, and `linux/arm64`, alongside [`docs/releases/pdf/Grabman-v1.1.0-Guide.pdf`](docs/releases/pdf/Grabman-v1.1.0-Guide.pdf), `grabman-v1.1.0-bundle.zip`, and `SHA256SUMS.txt`.

---

## `v1.0.0` — Step 5 Production Stability Freeze (2026-11-15)

- **SemVer Tier:** `v1.0.0 — Production Stability Freeze` (`100%` backwards compatible with `v0.1.0` manifests, lockfiles, and CLI outputs)
- **Ecosystem Impact:** Zero breaking changes to frozen upstream `v1.x` contracts (`GRAB v1.1.0`, `Grex JS v1.0.0`, `GrabLab CMS v1.0.1`, `Grex Theme Builder v1.0.0`, `Groxy AI v1.0.0`). Unblocks Step 6 (`Groxy Desk`) consumers pinning `>=1.0.0 <2.0.0`.

### Changed & Verified
- **CLI, Schema & Toolchain Freeze:**
  - Promoted CLI `Version` and default `schema_version` (`cmd/grabman/main.go`, `internal/manifest/schema.go`, `internal/doctor/checks.go`, `internal/scaffolder`, `internal/sitedna`, `internal/solver`) to **`1.0.0`** while preserving dual validation and deterministic lockfile compatibility for existing `"0.1.0"` manifests and lockfiles.
  - Aligned `go.mod` on `toolchain go1.27.4` matching `grabtaculous v1.0.0` and `gashboard v1.0.0`, verified with `govulncheck ./...` (`0` vulnerabilities) and `go mod verify`.
  - Updated `EcosystemMatrix()` `step-5` release list to `["0.1.0", "0.2.0", "0.3.0", "1.0.0"]` with default constraint `">=0.1.0 <2.0.0"` and `SemVerTier: "1.x-frozen"`.
- **Cross-Platform Release Artifacts:**
  - Built static `CGO_ENABLED=0` binaries for `windows/amd64` (`grabman-windows-amd64.exe`), `linux/amd64` (`grabman-linux-amd64`), and `linux/arm64` (`grabman-linux-arm64`), alongside [`docs/releases/pdf/Grabman-v1.0.0-Guide.pdf`](docs/releases/pdf/Grabman-v1.0.0-Guide.pdf), `grabman-v1.0.0-bundle.zip`, and `SHA256SUMS.txt`.

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
