# GRABMAN-SPEC-001: `grabman v0.1.0` Ecosystem Project Scaffolder, Compatibility Doctor, `Site DNA` Bridge & `grabman.lock` Solver

- **Status:** Specification in Review (Awaiting Project-Owner Approval — Pre-Implementation Design Only)
- **Milestone:** `v0.1.0` — Step 5 Foundation (Project Scaffolder, Cross-Layer Doctor, `Site DNA` Validator & Deterministic `grabman.lock` Solver)
- **Foundational Upstream Baselines (`v1.x` Contracts):**
  - **GRAB:** `v1.1.0` (`urbanspress/grab` — `grab version`, `grab check`, `grab format`, `grab migrate`, `grab serve`, `grab mcp`, `grab.platform.json`, `grab.data.json`, `grab.api.json`, `grab.connect.lock`, `ADR-025a..h`, `ADR-027a..d`, `ADR-028a..c`)
  - **Grex JS:** `v1.0.0` (`urbanspress/grex-js` — `grex.elements.json`, `grex.js` SHA-256 contract digests, zero wrapper-`<div>` schema)
  - **GrabLab CMS:** `v1.0.1` (`urbanspress/grablab-cms`, `GRABLAB-SPEC-100.md` — Zero-Plugin Core, exclusive active theme contract, `/_health`, `/_ready`, hardened `deploy/docker-compose.yml`)
  - **Grex Theme Builder:** `v1.0.0` (`urbanspress/grex-builder`, `GREX-BUILDER-SPEC-100.md` — exclusive `mode: "visual"` theme manifest, `GRAB-DESIGN-SYSTEM.md`)
  - **Groxy AI:** `v1.0.0` (`urbanspress/groxy-ai`, `GROXY-AI-SPEC-005.md` — portable `Site DNA` bundle `groxy-site-dna-v0.4.0` from `GET /api/groxy/site-dna/export`)
- **Governance:** Read alongside [`GRABMAN-EXPERT.md`](../GRABMAN-EXPERT.md), [`AGENTS.md`](../AGENTS.md), and [`docs/ROADMAP.md`](../docs/ROADMAP.md).

---

## 1. Purpose & `v0.1.0` Scope

Before **Grabtaculous** provisions a VPS stack or **Gashboard** deploys a site across staging and production servers, operators and AI agents need one fast, deterministic command-line tool that guarantees every ecosystem layer fits together cleanly. **`grabman v0.1.0`** fills that exact role as a lightweight, zero-dependency Go binary (`go 1.26.0`, `CGO_ENABLED=0`) providing four tightly focused commands.

### Included in `v0.1.0`
1. **Deterministic Project Scaffolder (`grabman init`):**
   - Scaffolds clean, `grab check`-ready project workspaces across three curated profiles (`app`, `cms-stack`, `ai-stack`) plus optional `--from-site-dna <path>` hydration from a **Groxy AI** `Site DNA` export.
   - Writes canonical `grabman.json`, `grabman.lock`, `grab.platform.json`, `grab.data.json`, `.env.example`, and `--read-only` Compose references with zero hardcoded passwords, tokens, or private IPs.
2. **Cross-Layer Compatibility & Security Doctor (`grabman doctor`):**
   - Inspects local runtime tooling (`grab`, optional `node`, optional `docker compose`), workspace manifests (`grabman.json`, `grabman.lock`, `grab.platform.json`, `grab.data.json`, `grab.connect.lock`, `theme.json`), `grex.elements.json` / `grex.js` SHA-256 parity, `--read-only` container posture (`read_only: true`, `cap_drop: ["ALL"]`, `no-new-privileges:true`, loopback port bindings), and `grab check` compilation health.
   - Emits both plain-English diagnostics and deterministic `--json` status (`healthy` | `warnings` | `blocked`) without leaking environment secret values.
3. **`grabman.lock` 6-Layer Compatibility Solver (`grabman solve`):**
   - Resolves and pins exact SemVer bounds (`ADR-027c`) and SHA-256 artifact digests across all **6 ecosystem layers** (`grab`, `grex-js`, `grablab-cms`, `grex-builder`, `groxy-ai`, `grabtaculous`/`gashboard`) into a byte-for-byte reproducible `grabman.lock`.
   - Includes `grabman solve --verify` (fail-closed gate used by **Grabtaculous** and **Gashboard** prior to rollout) detecting version drift, unapproved `v2.0.0` major jumps, or tampered digests.
4. **Portable `Site DNA` Validator & Diff Engine (`grabman dna`):**
   - Validates `groxy-site-dna-v0.4.0` JSON bundles (`grabman dna validate <file>`), enforcing `ADR-025c` base-10 string fidelity for prices/metrics, zero `<script>` / `<div>` / unsafe URL injection, and style-pack / design-system token completeness.
   - Computes deterministic structured diffs (`grabman dna diff <old> <new>`) summarizing token, menu, redirect, product price, and Watch score deltas for human review before fleet deployment.

### Explicitly Deferred to `v0.2.0+`
- Online package registry or `.grabpkg` remote plugin installer (third-party code plugins remain permanently excluded across the ecosystem).
- Air-gapped tarball bundling (`grabman bundle`) and upgrade impact simulator (`grabman upgrade --plan`).
- Cloud sync of `Site DNA` (handled securely through **Gashboard** + **Grabtaculous** SSH channels).

---

## 2. Planned `v0.1.0` Repository Layout (`C:\Development\grabman`)

```text
grabman/
├── GRABMAN-EXPERT.md
├── AGENTS.md
├── HANDOFF.md
├── README.md
├── LICENSE
├── .gitignore
├── go.mod                              # module grabman (go 1.26.0, zero external unsafe deps)
├── cmd/
│   └── grabman/
│       └── main.go                     # CLI entrypoint: init, doctor, solve, dna, version
├── internal/
│   ├── manifest/
│   │   ├── schema.go                   # grabman.json & grabman.lock strict schemas
│   │   └── pathguard.go                # GRAB-E401 workspace confinement & sensitive-path blocklist
│   ├── solver/
│   │   ├── semver.go                   # Strict MAJOR.MINOR.PATCH (ADR-027c) parser & range evaluator
│   │   └── lockfile.go                 # 6-layer compatibility matrix solver & SHA-256 verifier
│   ├── doctor/
│   │   ├── checks.go                   # Runtime, manifest, container & capability audit checks
│   │   └── report.go                   # Plain-English + JSON diagnostic formatter
│   ├── scaffolder/
│   │   ├── templates.go                # Embedded deterministic templates (app, cms-stack, ai-stack)
│   │   └── init.go                     # Safe atomic file generation + optional Site DNA hydration
│   └── sitedna/
│       ├── validate.go                 # groxy-site-dna-v0.4.0 validator + ADR-025c exact decimal guard
│       └── diff.go                     # Deterministic Site DNA diff generator
├── docs/
│   ├── ROADMAP.md
│   └── badges/
│       ├── built-with-grab.svg
│       ├── ecosystem-step-5.svg
│       └── spec-in-review.svg
├── specifications/
│   └── GRABMAN-SPEC-001.md
└── tests/
    ├── solver_test.go                  # Table-driven SemVer, 6-layer compatibility & drift tests
    ├── doctor_test.go                  # Mocked & live workspace security/capability doctor tests
    ├── scaffolder_test.go              # Init profiles, GRAB-E401 path guard & grab check tests
    └── sitedna_test.go                 # Site DNA v0.4.0 validation, float rejection & diff tests
```

---

## 3. File Contracts: `grabman.json` & `grabman.lock`

### 3.1 Author Intent Manifest (`grabman.json`)
```json
{
  "schema_version": "0.1.0",
  "project_name": "acme-storefront",
  "profile": "ai-stack",
  "target_runtime": {
    "grab": ">=1.1.0 <2.0.0",
    "grex_js": ">=1.0.0 <2.0.0",
    "grablab_cms": ">=1.0.1 <2.0.0",
    "grex_builder": ">=1.0.0 <2.0.0",
    "groxy_ai": ">=1.0.0 <2.0.0",
    "grabtaculous": ">=0.1.0 <1.0.0"
  },
  "site_dna_path": "storage/site-dna.json",
  "deploy_security": {
    "read_only_root": true,
    "non_root_uid": "65532:65532",
    "loopback_only_ports": true,
    "edge_proxy": "caddy"
  }
}
```

### 3.2 Deterministic Output Lockfile (`grabman.lock`)
Every `grabman solve` invocation resolves exact versions and digests into `grabman.lock`:
- `lock_version`: `"1"`
- `solved_at_utc` (excluded from `--check-determinism` hash comparison; core `lock_digest_sha256` hashes only canonical resolved layers + artifacts)
- `layers`: exact pinned `version` and `repo` for all 6 layers (`grab`, `grex-js`, `grablab-cms`, `grex-builder`, `groxy-ai`, `step-5`)
- `contracts`: SHA-256 hashes of shared runtime artifacts (`grex.elements.json`, `grex-js-v1.0.0.js`, `seo-playbook.v1.json`, `grab.platform.json`, `grab.data.json`, optional `site_dna`)
- `compatibility_verified`: `true` | `false`
- **Fail-Closed Error Codes:**
  - `GRABMAN-E101`: Missing or malformed `grabman.json` / `grabman.lock`
  - `GRABMAN-E102`: SemVer compatibility violation across the 6-layer matrix (e.g., `grab < 1.1.0` with `use crypto`, or unauthorized `>= 2.0.0`)
  - `GRABMAN-E103`: Artifact or `grabman.lock` SHA-256 checksum mismatch / tampering detected
  - `GRABMAN-E201`: Path confinement or sensitive-path rejection (`.git`, `.env`, `.ssh`, `*.key`, `*.pem`, escaping symlink — `GRAB-E401` parity)
  - `GRABMAN-E202`: `Site DNA` validation error (missing required field, `ADR-025c` float drift in price/count, unsafe `<script>`/`<div>`/URL injection)
  - `GRABMAN-E301`: Container hardening policy violation (missing `read_only: true`, `cap_drop: ALL`, `0.0.0.0` non-loopback DB port, or plaintext secret literal)

---

## 4. `Site DNA` (`groxy-site-dna-v0.4.0`) Verification & Diff Rules

`grabman dna` consumes the frozen **Groxy AI** export format (`GET /api/groxy/site-dna/export`):
1. **Required Structure:** Verifies `schema_version` (`"0.4.0"`), `bundle_name` (`"groxy-site-dna-v0.4.0"`), `"grabman"` in `compatible_consumers`, `read_only_snapshot === true`, `decimal_integrity_verified === true`, `active_style_pack`, `design_system`, `site_context`, and `nightly_watch_digest`.
2. **`ADR-025c` Exact Numeric Check:** Rejects any product price or numeric wire field containing IEEE-754 float artifacts (`0.30000000000000004`, scientific notation, or unquoted imprecise floats) with `GRABMAN-E202`.
3. **Token & XSS Hygiene:** Rejects `<script`, `<iframe`, `javascript:`, `vbscript:`, or `expression(` inside `design_system` tokens, `.grex-c-*` declarations, or `site_context` summaries.
4. **Deterministic Diff (`grabman dna diff`):** Summarizes added/removed/changed design tokens, active style pack switches, `301` redirect additions, product price edits, and `nightly_watch_digest.site_health_score` changes so **Gashboard** can show an understandable pre-deploy summary before **Grabtaculous** applies anything.

---

## 5. `v0.1.0` CLI Command Surface

| Command | Mode | Description |
|---|---|---|
| `grabman init [--profile app\|cms-stack\|ai-stack] [--from-site-dna <file>] [--dir <path>]` | Writes inside confined `--dir` | Scaffolds clean project files, default `grab.platform.json`/`grab.data.json`, and initial `grabman.lock`. Never overwrites existing files without explicit `--force` (and never touches `.git`/`.env`/`.ssh`). |
| `grabman doctor [--dir <path>] [--json] [--strict]` | Strictly Read-Only | Audits local binaries, manifests, `--read-only` Compose safety, SHA-256 contract parity, and `grab check` pass status. Exits non-zero on `blocked` (or `warnings` under `--strict`). |
| `grabman solve [--dir <path>] [--verify] [--json]` | Read-only under `--verify`; writes `grabman.lock` otherwise | Solves 6-layer SemVer bounds and pins artifact SHA-256 hashes into `grabman.lock`. `--verify` validates existing `grabman.lock` without mutating disk. |
| `grabman dna validate <file.json> [--json]` | Strictly Read-Only | Validates a Groxy AI `Site DNA` bundle against `groxy-site-dna-v0.4.0` + `ADR-025c` + XSS safety rules. |
| `grabman dna diff <old.json> <new.json> [--json]` | Strictly Read-Only | Computes deterministic human-readable & JSON diff between two `Site DNA` bundles. |
| `grabman version` (`-v`, `--version`) | Strictly Read-Only | Prints canonical GRAB Ecosystem CLI version, build timestamp, and supported lockfile schema version. |

---

## 6. `v0.1.0` Acceptance & Verification Gates

`grabman v0.1.0` is eligible for completion and synchronized `origin` + `stable` promotion **only** when **100%** of these gates pass:
1. **Go Build, Unit & Race Verification:**
   - `go build ./...` compiles cleanly with `CGO_ENABLED=0` across `windows/amd64` and `linux/amd64`.
   - `go test -count=1 ./...` passes `100%` (plus `go test -race -count=1 ./...` on Linux/CGO).
   - `govulncheck ./...` and `go mod verify` report zero vulnerabilities or integrity errors.
2. **Table-Driven Functional & Security Gates:**
   - `solver_test.go`: Verifies 6-layer compatibility acceptance, fails closed on `v2.0.0` major jump (`GRABMAN-E102`), fails closed on SHA-256 tampering (`GRABMAN-E103`), and proves deterministic byte-for-byte `grabman.lock` output.
   - `scaffolder_test.go`: Verifies all 3 profiles (`app`, `cms-stack`, `ai-stack`) + `--from-site-dna`, verifies `GRAB-E401` path-traversal/symlink/sensitive-file rejection (`GRABMAN-E201`), and runs live `grab check` against generated `.grab` templates when `grab` is available.
   - `doctor_test.go`: Verifies plain-English & JSON checks catch insecure Docker flags (`read_only: false`, `0.0.0.0` DB exposure, hardcoded secrets → `GRABMAN-E301`) while passing clean hardened stacks.
   - `sitedna_test.go`: Tests against real `groxy-ai` `Site DNA` export fixtures, rejecting IEEE float drift (`GRABMAN-E202`) and `<script>` injection, and producing deterministic diffs.
3. **Plain-English Release Guide & Dual-Repo Promotion (upon `v0.1.0` completion):**
   - Generates `docs/releases/pdf/grabman-v0.1.0-Guide.pdf`, `CGO_ENABLED=0` binaries + `SHA256SUMS.txt`, runs `sync-notebook-sources.ps1`, and publishes synchronized `v0.1.0` GitHub Releases with the mandatory **Release Type & Affected Layers** + **6-Layer Ecosystem Compatibility Matrix** block to `urbanspress/grabman.progress` and `urbanspress/grabman.stable`.
