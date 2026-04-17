# Mesh v0.3.0 First Public Release — Execution Plan

## Project Root
`/Users/samanvayayagsen/project/rethink-paradigms/infa/mesh-workspace/oss/`

## Goal
Ensure the happy path (install → init → deploy → status → logs → ssh → destroy) actually works end-to-end before announcing the project publicly.

## Project State
- Published on PyPI as `rethink-mesh` v0.3.0
- GitHub repo is public
- 375 tests, 98.7% non-E2E
- Claims 50+ cloud providers via Libcloud
- Only manually tested once with DigitalOcean, long ago

---

## Phase 0 — Static Audit ✅ COMPLETE

Pure code analysis. No infra, no changes. Found 23 defects.

**Deliverables:**
- [x] Complete codebase analysis (4 vertical slice domains, 83 Python files, 11 CLI commands)
- [x] Full happy path traces for: init, deploy, status, logs, ssh, destroy, doctor
- [x] Provider-specific hardcoding inventory (23 defects: 7 critical, 8 high, 8 medium)
- [x] Defect tracking file: `.sisyphus/phase0-defects.md`

**Key Findings:**
- `mesh destroy` does NOTHING for cloud clusters (C1)
- `mesh ssh` hardcodes user="ubuntu", breaks on DO/Linode/Vultr (C2)
- AWS defaults hardcoded in provisioning (C3)
- INGRESS/PRODUCTION deployment returns False (C4)
- Hetzner in wizard but not in PROVIDER_ENUMS (C5)
- Missing GPU/spot boot scripts (C6)
- AWS spot handler hardcoded for all providers (C7)
- Only 4 providers in init wizard, claims 50+ (H2)
- Tailscale env var mismatch between multipass/cloud paths (H1)

**Relevant Files:**
- `.sisyphus/phase0-defects.md` — full defect list with acceptance criteria

---

## Phase 1 — Demo Mode Polish ⏳ NEXT

Run every CLI command in `--demo` mode. Fix crashes and weird output. No cloud infra, no cost.

**Scope:**
- `mesh init --demo`
- `mesh deploy --demo`
- `mesh status --demo`
- `mesh logs --demo`
- `mesh ssh --demo`
- `mesh destroy --demo`
- `mesh doctor --demo`
- `mesh version`
- `mesh compare`
- `mesh roadmap`

**Deliverables:**
- [ ] Every command runs without crash in demo mode
- [ ] Output is clean, professional, no stack traces
- [ ] New defects found are added to `.sisyphus/phase0-defects.md`
- [ ] Demo mode feels like a real product demo

**Notes:**
- This is TESTING ONLY. Fixes may be done inline for trivial issues.
- Non-trivial fixes go into the defect tracking file.
- May discover additional defects not found in static audit.

---

## Defect Fix Sprint — C1-C7 Critical Fixes ⬜ BLOCKED BY PHASE 1

Fix critical defects before spending real money on cloud testing.

**Must Fix (blocks Phase 2/3):**
- [ ] C1: Implement cloud destroy (will leave orphaned VMs otherwise)
- [ ] C2: Fix SSH user per provider (Phase 3 will fail on DO without this)
- [ ] C3: Remove AWS defaults (breaks non-AWS providers)
- [ ] C5: Fix Hetzner in PROVIDER_ENUMS (or remove from wizard)
- [ ] C6: Remove or stub missing boot scripts (boot will crash otherwise)

**Should Fix:**
- [ ] C4: Graceful message for INGRESS/PRODUCTION tiers
- [ ] C7: Gate spot handler behind AWS provider check
- [ ] H1: Standardize Tailscale env var
- [ ] H2: Add GCP, Azure, Linode, Vultr to init wizard

**Defect File:** `.sisyphus/phase0-defects.md`

**Strategy:** Fire parallel agents — one per independent defect. Each agent reads the defect file, picks up their item, marks in-progress, fixes, marks completed.

---

## Phase 2 — Local Multipass Validation ⬜ BLOCKED BY DEFECT FIX SPRINT

Full end-to-end on laptop with real services. Exercises boot scripts, Nomad, Consul, Caddy.

**Prerequisites:**
- Multipass installed
- Tailscale account + auth key
- Defects C3, C5, C6 fixed

**Scope:**
- `mesh init --provider multipass`
- `mesh deploy` (deploy a test app)
- `mesh status`
- `mesh logs`
- `mesh ssh`
- `mesh destroy`

**Deliverables:**
- [ ] Full happy path works on Multipass
- [ ] Caddy ingress serves HTTPS
- [ ] Boot scripts execute without error
- [ ] All 4 Nomad templates valid
- [ ] New defects logged to tracking file

---

## Phase 3 — DigitalOcean End-to-End ⬜ BLOCKED BY PHASE 2

Real money. Single cloud provider proven working.

**Prerequisites:**
- DigitalOcean account + API token
- Tailscale account + auth key
- Domain name (optional, for HTTPS testing)
- Defects C1, C2 fixed

**Scope:**
- `mesh init --provider digitalocean --region nyc3`
- `mesh deploy` (deploy a test app)
- `mesh status`
- `mesh logs`
- `mesh ssh` (as root on DO)
- `mesh destroy` (MUST work — no orphaned resources)

**Deliverables:**
- [ ] Full happy path works on DigitalOcean
- [ ] HTTPS with Let's Encrypt works
- [ ] SSH as correct user (root for DO)
- [ ] Destroy cleans up ALL resources
- [ ] Cost documented (expected spend: ~$1-5 for testing)
- [ ] Results documented for release notes

---

## Phase 4 — Harden Claims ⬜ BLOCKED BY PHASE 3

Update documentation to honestly reflect tested vs theoretical support.

**Scope:**
- Update README: tested providers vs claimed providers
- Update docs: what's verified vs what's theoretical
- Add provider compatibility matrix
- Update CHANGELOG if needed
- Review quickstart guide accuracy

**Deliverables:**
- [ ] README reflects reality
- [ ] Provider support clearly documented
- [ ] Quickstart guide tested and accurate
- [ ] No unverified claims in marketing copy

---

## Constraints

From AGENTS.md:
- Vertical slice architecture (cli, infrastructure, workloads, verification)
- Read CONTEXT.md before modifying any feature
- Co-located tests (test_<thing>.py beside <thing>.py)
- Memory budget ~530MB RAM
- Conventional commits: `feat(scope):`, `fix(scope):`, etc.
- Python 3.11+, Typer for CLI, Pulumi for IaC, Libcloud for multi-cloud

## Key Files

| File | Purpose |
|---|---|
| `.sisyphus/phase0-defects.md` | Defect tracking (23 defects) |
| `src/mesh/cli/commands/` | All CLI command implementations |
| `src/mesh/infrastructure/providers/__init__.py` | Provider registry & credentials |
| `src/mesh/infrastructure/provision_cloud_cluster/` | Cloud cluster orchestration |
| `src/mesh/infrastructure/boot_consul_nomad/` | Boot scripts & templates |
| `src/mesh/workloads/deploy_app/deploy.py` | Tier-aware deployment dispatcher |
| `AGENTS.md` | Project conventions |
