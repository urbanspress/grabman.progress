# GRABMAN-SPEC-100: `grabman v1.0.0` Production Contract Freeze & Cross-Layer Security Audit

- **Status:** Approved & Complete (`v1.0.0` Step 5 Production Stability Freeze Implemented, Verified & Synchronized)
- **Target Release:** `v1.0.0` (`MAJOR.MINOR.PATCH` — Step 5 CLI Scaffolder, Doctor & Lockfile Production Freeze)
- **Foundational Baselines:**
  - **GRAB:** `v1.1.0` (`urbanspress/grab` — `>= v1.1.0 < v2.0.0`)
  - **Grex JS:** `v1.0.0` (`urbanspress/grex-js` — `>= v1.0.0 < v2.0.0`)
  - **GrabLab CMS:** `v1.0.1` (`urbanspress/grablab-cms` — `>= v1.0.1 < v2.0.0`)
  - **Grex Theme Builder:** `v1.0.0` (`urbanspress/grex-builder` — `>= v1.0.0 < v2.0.0`)
  - **Groxy AI:** `v1.0.0` (`urbanspress/groxy-ai` — `>= v1.0.0 < v2.0.0`)
  - **Grabtaculous & Gashboard:** `v1.0.0` Step 5 Freeze Companions (`specifications/GRABTACULOUS-SPEC-100.md`, `specifications/GASHBOARD-SPEC-100.md`)
- **Governance:** Read alongside [`GRABMAN-EXPERT.md`](../GRABMAN-EXPERT.md), [`AGENTS.md`](../AGENTS.md), [`GRABMAN-SPEC-001.md`](GRABMAN-SPEC-001.md), and [`docs/ROADMAP.md`](../docs/ROADMAP.md).

---

## 1. Executive Summary (Plain English)

Before promoting Step 5 (`grabman`, **Grabtaculous**, and **Gashboard**) to `v1.0.0` for **Step 6 (Groxy Desk)**, an independent pre-`v1.0.0` cross-layer audit (`Orchestrator (Argon)`) verified every line of **`grabman`**, its compatibility solver, its `--read-only` security doctor, its path-confinement guard, and its live integration with **GRAB `v1.1.0`**, **Groxy AI `v1.0.0`**, **Grabtaculous**, and **Gashboard**.

This specification records that audit evidence and defines the exact, backwards-compatible contract freeze for **`grabman v1.0.0`**:
- Every existing `grabman v0.1.0` manifest (`schema_version: "0.1.0"`), lockfile constraint (`>=0.1.0 <1.0.0` and `>=0.1.0 <2.0.0`), CLI command, JSON output shape, and exit code remains **100% supported**.
- Step 5 in the 6-layer compatibility matrix expands cleanly to include `v1.0.0` alongside `0.1.0`, `0.2.0`, and `0.3.0`, unblocking all **`>=1.0.0 <2.0.0`** Step 6 desktop and fleet consumers.
- Zero breaking changes are introduced (`v1.0.0` freezes the `v1.x` contract; `v2.0.0` remains locked behind `ADR-027c`).

---

## 2. Independent Pre-`v1.0.0` Security & Cross-Layer Audit Evidence

| Audit Domain | Audited Implementation Files | Verified Finding |
|---|---|---|
| **`GRAB-E401` Workspace & Symlink Confinement** | [`internal/manifest/pathguard.go`](../internal/manifest/pathguard.go) | Blocks empty paths, `\x00` NUL bytes, UNC `\\`/`//` shares, `..` traversal, escaping symlinks (verified via `EvalExistingSymlinks`), `.git`, `.env`, `.env.*` (except `.env.example` when explicitly permitted), `.ssh`, `id_rsa`, `id_ed25519`, `id_ecdsa`, `id_dsa`, `*.pem`, `*.key`, `*.p12`, `*.pfx`, and reads above `10 MiB` (`MaxReadBytes`). Returns deterministic `GRABMAN-E201` (`GRAB-E401` parity). |
| **Zero Shell-String Interpolation** | [`internal/doctor/checks.go`](../internal/doctor/checks.go) | All invocations of `grab` (`LocateGrabBinary`, `exec.CommandContext(ctx, grabPath, ...)`) and optional tool detection (`exec.LookPath`) pass structured argument slices directly to OS process APIs. No `sh -c`, `cmd /c`, or interpolated shell commands exist anywhere in `grabman`. |
| **Container `--read-only` & Secret Hygiene Checks** | [`internal/doctor/checks.go`](../internal/doctor/checks.go) (`auditComposeSecurity`, `scanNoHardcodedSecrets`), [`internal/scaffolder/templates.go`](../internal/scaffolder/templates.go) | Fail-closed (`GRABMAN-E301`) enforcement of `read_only: true`, `cap_drop: [ALL]`, `security_opt: [no-new-privileges:true]`, no `privileged: true`, no `network_mode: host`, no `/var/run/docker.sock`, no `:latest` image tags, no non-loopback `0.0.0.0`/bare ports, and zero hardcoded secrets or private RFC-1918 VPS IPs. |
| **Deterministic 6-Layer `grabman.lock` Solver** | [`internal/solver/lockfile.go`](../internal/solver/lockfile.go), [`internal/solver/semver.go`](../internal/solver/semver.go), [`internal/manifest/schema.go`](../internal/manifest/schema.go) | Enforces `ADR-027c` SemVer constraints across all 6 ecosystem layers (`grab`, `grex-js`, `grablab-cms`, `grex-builder`, `groxy-ai`, `step-5`), blocks any `>= 2.0.0` major jump (`GRABMAN-E102`), pins canonical `lock_digest_sha256` excluding `solved_at_utc`, and verifies frozen SHA-256 upstream digests (`grex.elements.json`: `c54c4bcf15ec8fea2dec4ef41975ba85a4b555dd400604e59c78883a8bbdebc9`, `grex.js`: `c1e28d74a7ecf19182e225644fa244aa8600658dd1ff8895c133bfc64976b6d6` / GrabLab CMS bundle `eded264c31c3f5efa6e4b67ed06049eb5a5e517d34179efcbba921811059a377`, `seo-playbook.v1.json`: `71817a8568908630b30c0d44d3319b3b86883d938b1d61466e78aad6d9d72cbe`). |
| **`Site DNA` & `ADR-025c` Precision Guard** | [`internal/sitedna/validate.go`](../internal/sitedna/validate.go), [`internal/sitedna/diff.go`](../internal/sitedna/diff.go) | Validates `groxy-site-dna-v0.4.0` bundles (`decimal_integrity_verified`, `read_only_snapshot`), rejects float precision drift (`0.30000000000000004`, scientific notation) and `<div>`/`<script>`/`javascript:` XSS payloads with `GRABMAN-E202`, and computes deterministic diffs. |
| **Zero Wrapper-`<div>` Template Audit** | [`internal/doctor/checks.go`](../internal/doctor/checks.go) (`checkTemplatesZeroDiv`) | Scans workspace `views/*.grab.html` templates and blocks `<div`, `<script`, inline `on*=` attributes, and `javascript:` URIs. |

### Baseline Audit Verification Results Observed
- `go build ./...` (from `C:\Development\grabman`): **PASS** (`0` errors).
- `go test -count=1 ./...`: **PASS** (`ok grabman/tests 10.097s`, `0` skipped tests).
- `govulncheck ./...`: **PASS** (`Your code is affected by 0 vulnerabilities` across all 7 workspace packages; note stdlib advisory `GO-2026-6859` for HTTP/1 server `MaxBytesReader` on `stdlib@go1.27.3` is uncalled because `grabman` starts no HTTP server, and `v1.0.0` implementation will align `go.mod` on `toolchain go1.27.4` matching `grabtaculous` and `gashboard`).

---

## 3. Frozen `v1.0.0` Contract Specifications (For Step 6 & Long-Term `v1.x` Stability)

Upon project-owner approval of this specification, the `v1.0.0` release implementation will apply strictly backwards-compatible contract finalizations:

### 3.1 CLI Command Surface, Flags & Exit Codes (`cmd/grabman/main.go`)

| Command & Signature | Mutates Disk? | Frozen `v1.x` Contract |
|---|---|---|
| `grabman init [--dir <path>] [--profile app\|cms-stack\|ai-stack] [--name <name>] [--edge-proxy caddy\|nginx\|cloudflare-tunnel] [--from-site-dna <file.json>] [--force] [--fixed-timestamp <RFC3339>] [--json]` | Only inside `--dir` | Writes `grabman.json`, `grabman.lock`, `grab.platform.json`, `grab.data.json`, `.env.example`, `deploy/docker-compose.yml` (`--read-only`), and optional `storage/site-dna.json`. Never overwrites existing files without `--force`. |
| `grabman doctor [--dir <path>] [--strict] [--grab-bin <path>] [--json]` | No (Read-Only) | Returns deterministic JSON `{schema_version, status: "healthy"\|"warnings"\|"blocked", workspace, grab_binary, grab_version, summary, checks}`. Exit `0` when healthy/warnings (unless `--strict`), `1` when blocked (or warnings under `--strict`). |
| `grabman solve [--dir <path>] [--verify] [--fixed-timestamp <RFC3339>] [--json]` | `--verify`: Read-Only; otherwise writes `grabman.lock` | Solves 6-layer matrix and checks SHA-256 digests. `--verify` fails closed (`1`, `GRABMAN-E102`/`GRABMAN-E103`) on any drift. |
| `grabman dna validate <file.json> [--json]` | No (Read-Only) | Validates `Site DNA` schema (`groxy-site-dna-v0.4.0` and future backwards-compatible `v1.0.0` tags if approved), `ADR-025c` decimal strings, and XSS/DOM safety. |
| `grabman dna diff <old.json> <new.json> [--json]` | No (Read-Only) | Emits deterministic human-readable & JSON diff of tokens, style pack, redirects, product prices (`ADR-025c`), and Watch health score. |
| `grabman version [--json]` | No (Read-Only) | Prints `grabman v1.0.0` (or JSON `{name: "grabman", version: "1.0.0", schema_version: "1.0.0", lock_version: "1"}`). |
| Global Argument Parsing & Exit Codes | — | `reorderFlags` allows flags before or after positional arguments. Exit `0` = success/healthy, `1` = runtime/validation/verification failure, `2` = invalid CLI usage. |

### 3.2 Schema Versioning & 6-Layer Matrix Upgrade (`v0.1.0` -> `v1.0.0`)

1. **`grabman.json` & `grabman.lock` `schema_version` Backwards Compatibility (`internal/manifest/schema.go`):**
   - Introduce `SchemaVersionV100 = "1.0.0"` alongside existing `SchemaVersionV010 = "0.1.0"`.
   - `ValidateManifest` and `LoadLockfile` MUST accept both `"0.1.0"` and `"1.0.0"` cleanly so existing `v0.1.0` workspaces continue passing `grabman solve --verify` and `grabman doctor` with zero forced migrations.
   - Newly scaffolded workspaces (`grabman init`) emit `"schema_version": "1.0.0"` and lockfiles retain `"lock_version": "1"`.
2. **6-Layer `EcosystemMatrix()` Step 5 Release List (`internal/solver/lockfile.go`):**
   - Expand `step-5` entry from pre-1.0-only to:
     - `Available: []string{"0.1.0", "0.2.0", "0.3.0", "1.0.0"}`
     - `DefaultExpr: ">=0.1.0 <2.0.0"`
     - `SemVerTier: "1.x-frozen"`
   - Existing manifests specifying `">=0.1.0 <1.0.0"` resolve cleanly to `0.3.0` (`grabtaculous`/`gashboard`) or `0.1.0` when `0.1.0` is pinned, while manifests specifying `">=1.0.0 <2.0.0"` or default constraints resolve cleanly to `1.0.0`.
3. **Toolchain Parity (`go.mod`):**
   - Set `toolchain go1.27.4` in `go.mod` to match `grabtaculous` and `gashboard`.
4. **Frozen Error Code Registry (`internal/manifest/errors.go`):**
   - `GRABMAN-E101` (Manifest/Lockfile malformed or missing)
   - `GRABMAN-E102` (SemVer compatibility violation or unapproved `>= 2.0.0`)
   - `GRABMAN-E103` (SHA-256 contract or lockfile checksum mismatch)
   - `GRABMAN-E201` (`GRAB-E401` path confinement or sensitive-path rejection)
   - `GRABMAN-E202` (`Site DNA` / `ADR-025c` / XSS validation error)
   - `GRABMAN-E301` (`--read-only` Compose or secret hygiene violation)

---

## 4. `v1.0.0` Approval & Release Acceptance Gates (Completed)

Approved by the project owner and verified for Step 5 `v1.0.0` production release:
1. [x] Apply the `v1.0.0` version, `schema_version` dual-acceptance (`"0.1.0"` and `"1.0.0"`), `EcosystemMatrix()` `step-5` update (`0.1.0`..`1.0.0`), and `go.mod` `toolchain go1.27.4` alignment in `C:\Development\grabman`, plus unit/integration tests covering both `"0.1.0"` and `"1.0.0"` manifests and Step 5 SemVer resolution.
2. [x] Verify `go build ./...`, `go vet ./...`, `go test -count=1 ./...` (`100%` pass), `govulncheck ./...` (`0` vulnerabilities), `go mod verify`, and live integration with `grab v1.1.0`, `grabtaculous v1.0.0`, and `gashboard v1.0.0`.
3. [x] Build static `CGO_ENABLED=0` binaries (`windows/amd64`, `linux/amd64`, `linux/arm64`), generate `docs/releases/pdf/Grabman-v1.0.0-Guide.pdf`, `grabman-v1.0.0-bundle.zip`, and `SHA256SUMS.txt`, run `sync-notebook-sources.ps1`, and publish synchronized `v1.0.0` releases on `urbanspress/grabman.progress` and `urbanspress/grabman.stable`.
