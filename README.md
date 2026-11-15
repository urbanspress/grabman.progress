# `grabman`

![Built with GRAB v1.1.0](docs/badges/built-with-grab.svg)
![GRAB Ecosystem Step 5 — grabman](docs/badges/ecosystem-step-5.svg)
![v0.1.0 Spec — In Review](docs/badges/spec-in-review.svg)

**`grabman`** is the Ecosystem Project Scaffolder, Compatibility Doctor, `Site DNA` Validator, and Deterministic `grabman.lock` Solver for **Step 5 of the Post-1.0 GRAB Ecosystem**.

- **Current Status:** `v0.1.0` — **Specification in Review** ([`specifications/GRABMAN-SPEC-001.md`](specifications/GRABMAN-SPEC-001.md) drafted; zero implementation code written pending owner approval).
- **Development Repo (`origin`):** `https://github.com/urbanspress/grabman.progress`
- **Production Release Repo (`stable`):** `https://github.com/urbanspress/grabman.stable`

---

## Core Capabilities (Planned `v0.1.0`)

1. **Deterministic Project Scaffolder (`grabman init`):**
   - Generates clean, `grab check`-ready workspaces (`app`, `cms-stack`, `ai-stack`, or hydrated from a Groxy AI `--from-site-dna` snapshot) with `grabman.json`, `grabman.lock`, `grab.platform.json`, and `grab.data.json`—zero hardcoded secrets and zero unverified remote install scripts.
2. **Plain-English Compatibility Doctor (`grabman doctor`):**
   - Verifies local `GRAB v1.1.0`, `Grex JS v1.0.0`, `GrabLab CMS v1.0.1`, `Grex Theme Builder v1.0.0`, and `Groxy AI v1.0.0` readiness, `--read-only` container flags, SHA-256 contract digests, capability boundaries, and `ADR-025c` decimal precision.
3. **`grabman.lock` 6-Layer Compatibility Solver (`grabman solve`):**
   - Solves and pins SemVer (`ADR-027c`) version bounds and SHA-256 contract hashes across all 6 ecosystem layers into a deterministic `grabman.lock` file (`--verify` detects drift before deployment).
4. **`Site DNA` Bridge (`grabman dna`):**
   - Validates and diffs portable `groxy-site-dna-v0.4.0` bundles exported from **Groxy AI** (`GET /api/groxy/site-dna/export`) for safe zero-downtime fleet rollouts through **Grabtaculous** and **Gashboard**.

---

## 6-Layer Ecosystem Compatibility Matrix

| Ecosystem Layer | Repository | Required Version Range | Role |
|---|---|---|---|
| **Core Language, Bytecode VM & Grisp Server** | `urbanspress/grab` | `>= v1.1.0 (< v2.0.0)` | Executes `.grab`/`.grab.html`, `grab check`, `grab migrate`, `grab mcp` |
| **Reactive Frontend Runtime & 20-Element Library** | `urbanspress/grex-js` | `>= v1.0.0 (< v2.0.0)` | `grex.elements.json` & reactive UI runtime contract |
| **Flagship Zero-Plugin CMS** | `urbanspress/grablab-cms` | `>= v1.0.1 (< v2.0.0)` | Zero-plugin CMS & exclusive theme host |
| **Exclusive `mode: "visual"` Theme & Studio** | `urbanspress/grex-builder` | `>= v1.0.0 (< v2.0.0)` | `GRAB-DESIGN-SYSTEM.md` tokens & visual theme |
| **Autonomous Design, Content & SEO Agent** | `urbanspress/groxy-ai` | `>= v1.0.0 (< v2.0.0)` | `Site DNA` export (`groxy-site-dna-v0.4.0`) & MCP design agent |
| **Scaffolder, Orchestrator & Discreet VPS Panel** | `urbanspress/grabman`, `urbanspress/grabtaculous`, `urbanspress/gashboard` | `>= v0.1.0` | Project scaffolding, lockfile solving, `--read-only` rollouts & desktop VPS panel |

---

## Governance & Specifications

- [Agent Rules (`AGENTS.md`)](AGENTS.md)
- [Engineering Invariants (`GRABMAN-EXPERT.md`)](GRABMAN-EXPERT.md)
- [`v0.1.0` Specification (`specifications/GRABMAN-SPEC-001.md`)](specifications/GRABMAN-SPEC-001.md)
- [Roadmap (`docs/ROADMAP.md`)](docs/ROADMAP.md)
- [Handoff Log (`HANDOFF.md`)](HANDOFF.md)
- [License (`LICENSE` — MIT)](LICENSE)
