# `grabman` Roadmap

**Repositories:** `urbanspress/grabman.progress` (active development) / `urbanspress/grabman.stable` (verified releases)  
**Foundational Baselines:** **GRAB `v1.1.0`**, **Grex JS `v1.0.0`**, **GrabLab CMS `v1.0.1`**, **Grex Theme Builder `v1.0.0`**, and **Groxy AI `v1.0.0`**

**Status Labels:**
- **Specification in Review** — Specification drafted and awaiting explicit project-owner approval before any Go implementation code is written.
- **Approved** — Authorized by the project owner for implementation.
- **Complete** — Implemented, `go test -count=1 ./...` + `govulncheck ./...` + E2E checks passing `100%`, walkthrough PDF generated, and synchronized to `origin` + `stable`.
- **Planned** — Future milestone awaiting prior milestone completion and spec drafting.

---

## `v0.1.0` — Ecosystem Project Scaffolder (`grabman init`), Compatibility Doctor (`grabman doctor`), `Site DNA` Bridge (`grabman dna`) & `grabman.lock` Solver (`grabman solve`)

**Status:** Complete ([`specifications/GRABMAN-SPEC-001.md`](../specifications/GRABMAN-SPEC-001.md))

Approved Scope:
- **`grabman init`:** Deterministic, offline-safe project scaffolder for `app`, `cms-stack`, `ai-stack`, and `--from-site-dna` profiles generating `grabman.json`, `grabman.lock`, `grab.platform.json`, and `grab.data.json` with zero hardcoded secrets and `GRAB-E401` path confinement.
- **`grabman doctor`:** Cross-layer compatibility and `--read-only` readiness doctor checking installed `grab v1.1.0` / runtime binaries, `grab check` parity, capability flags, `grex.elements.json` / `grex.js` SHA-256 digests, and `ADR-025c` decimal precision with plain-English + `--json` diagnostics.
- **`grabman solve` (`grabman.lock`):** Deterministic 6-layer SemVer constraint solver (`ADR-027c`) pinning exact release versions and SHA-256 contract hashes into `grabman.lock` (`--verify` fail-closed drift detection).
- **`grabman dna`:** Validates, inspects, and diffs Groxy AI `groxy-site-dna-v0.4.0` portable **Site DNA** bundles (`ADR-025c` decimal integrity, zero wrapper-`<div>` / XSS hygiene) for **Grabtaculous** and **Gashboard** fleet deployments.

---

## `v0.2.0` — Lockfile Upgrade Planner, Pre-Migration Compatibility Diff & Air-Gapped Bundle Exporter

**Status:** Planned

Planned Scope:
- `grabman upgrade --plan` showing non-breaking vs. neighbor-upgrade impact across the 6-layer compatibility matrix before changing `grabman.lock`.
- Air-gapped `.tar.gz` / `.zip` offline deployment bundle export (`grabman bundle`) pairing `grabman.lock`, `Site DNA`, and SHA-256-verified Compose manifests for **Grabtaculous**.

---

## `v1.0.0` — Production Freeze, Signed Release Binaries & Full 6-Layer Ecosystem E2E Verification

**Status:** Complete ([`specifications/GRABMAN-SPEC-100.md`](../specifications/GRABMAN-SPEC-100.md))

Completed Scope:
- Completed Step 5 multi-repository cross-layer security, SemVer lockfile, `GRAB-E401` path-confinement, and `go test -count=1 ./...` / `govulncheck ./...` verification (`0` vulnerabilities, [`specifications/GRABMAN-SPEC-100.md`](../specifications/GRABMAN-SPEC-100.md)).
- Froze `grabman` CLI, doctor, solver, scaffolder, and `Site DNA` contracts at `v1.0.0` with dual backwards compatibility for `"0.1.0"` and `"1.0.0"` manifests, aligned `go.mod` toolchain on `go1.27.4`, expanded `EcosystemMatrix()` `step-5` to `0.1.0`..`1.0.0`, built cross-platform static `CGO_ENABLED=0` binaries (`windows/amd64`, `linux/amd64`, `linux/arm64`), generated `docs/releases/pdf/Grabman-v1.0.0-Guide.pdf`, and synchronized `v1.0.0` releases across `origin` (`urbanspress/grabman.progress`) and `stable` (`urbanspress/grabman.stable`).
