# GRABMAN-EXPERT.md — Non-Negotiable Engineering Invariants

This checklist defines the **never-break invariants** for **`grabman`** (`urbanspress/grabman`), the project scaffolder, compatibility doctor, `Site DNA` validator, and `grabman.lock` solver in **Step 5 of the Post-1.0 GRAB Ecosystem**. Every specification, CLI subcommand, lockfile schema, and PR must satisfy all four pillars.

---

## 1. Deterministic `grabman.lock` & SemVer Compatibility Solving
- **Reproducible Offline Solver:** Given identical inputs (`grabman.json`, ecosystem constraint matrix, artifact SHA-256 digests, and optional `Site DNA`), `grabman solve` MUST produce byte-for-byte deterministic `grabman.lock` JSON with sorted keys and canonical formatting.
- **Strict SemVer (`ADR-027c`) Boundaries:** Never silently cross a `v2.0.0` major boundary or allow incompatible cross-layer versions (`grab < v1.1.0`, `grex-js < v1.0.0`, `grablab-cms < v1.0.1`, `grex-builder < v1.0.0`, `groxy-ai < v1.0.0`). Always fail closed with clear plain-English upgrade diagnostics (`GRABMAN-E101`–`GRABMAN-E104`).
- **SHA-256 Integrity Verification:** Pin shared contract digests (`grex.elements.json`, `grex-js-v1.0.0.js`, `seo-playbook.v1.json`, `site_dna_sha256`, and compose templates) in `grabman.lock` and fail closed on checksum drift.

---

## 2. Zero Arbitrary Code Execution & Path Confinement (`GRAB-E401` Parity)
- **Zero Plugin / Script Execution:** `grabman init` and `grabman doctor` MUST NEVER download or execute `curl | sh` post-install scripts, arbitrary shell templates, or remote code plugins. All scaffolds use verified, data-only embedded manifests.
- **Strict Workspace Path Confinement:** Reject empty paths, null bytes, `..` traversal escaping the workspace root, escaping symlinks (`filepath.EvalSymlinks`), and writes/reads touching sensitive paths (`.git`, `.env`, `.ssh`, `*.key`, `*.pem`, `id_rsa`, `id_ed25519`).
- **Zero Hardcoded Secrets:** Scaffolded `grab.data.json`, `grab.platform.json`, and `deploy/.env.example` templates MUST use host environment variable references only—never embedding real passwords, API keys, signing secrets, or private VPS IPs.

---

## 3. Portable `Site DNA` & Exact Numeric Fidelity (`ADR-025c`)
- **`groxy-site-dna-v0.4.0` Contract Enforcement:** `grabman dna` MUST validate incoming **Site DNA** bundles (`GET /api/groxy/site-dna/export`) against `schema_version`, `compatible_consumers`, `design_system`, `site_context`, and `decimal_integrity_verified: true`.
- **No Silent Float Coercion:** Base-10 `GrabDecimal` and `GrabInteger` wire strings inside `Site DNA` (`site_context.products`, pricing tokens, resource quotas) MUST remain exact strings—reject raw IEEE-754 float drift (`"0.30000000000000004"`, unquoted float drift) deterministically (`GRABMAN-E202`).
- **Zero Wrapper-`<div>` / XSS Hygiene:** Reject unsafe `<script>`, `<iframe>`, `javascript:` URLs, or wrapper `<div>` injections when validating `Site DNA` or `Grex` design tokens before deployment.

---

## 4. Plain-English Diagnostics & Dual-Repo SemVer Discipline
- **Actionable Plain-English Reporting:** Every `grabman doctor` check MUST return both structured JSON (`--json`) and plain-English explanations telling non-coders what passed, what failed, and the exact fix command—without exposing host environment secret values.
- **Committed Emerald `#10B981` Vector Badges:** Keep `docs/badges/built-with-grab.svg`, `docs/badges/ecosystem-step-5.svg`, and `docs/badges/spec-in-review.svg` (or release badge) committed and linked in `README.md`.
- **Dual-Repo Protection (`origin` vs. `stable`):** Never push unapproved specs or partial builds to `urbanspress/grabman.stable`. Every release on both repos MUST carry the **Release Type & Affected Layers** summary block and **6-Layer Ecosystem Compatibility Matrix** alongside its plain-English walkthrough PDF.
