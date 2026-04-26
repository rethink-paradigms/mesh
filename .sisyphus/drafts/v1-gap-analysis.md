# Draft: Mesh v1 Full Product Gap Analysis

## V0 (What Exists)

**Codebase**: 15 Go files, single binary, no daemon
- `cmd/mesh/main.go` — Cobra CLI with 7 commands
- `internal/config/` — TOML config (agents + machines)
- `internal/snapshot/` — Deterministic tar + zstd + SHA-256 pipeline
- `internal/restore/` — Snapshot extraction + post-restore hooks
- `internal/clone/` — Remote clone via scp/ssh
- `internal/agent/` — PID-based status detection
- `internal/manifest/` — JSON sidecar read/write
- `internal/transport/` — SSH transport for clone

**Dependencies**: cobra, toml, compress (zstd) — minimal

**What V0 does well**: Filesystem snapshot primitive is proven. Deterministic tar pipeline, streaming (no memory buffering), SHA-256 integrity, manifest sidecar — all production-quality.

---

## V1 Target (SYSTEM.md + 6 Deep Design Docs)

The deep designs are exceptionally thorough — each ~250 lines with state machines, failure analysis, concurrency models, scale analysis, edge cases, and contract verification. This is rare and valuable.

### Module-by-Module Gap

### 1. Interface (MCP + Skills) — GREENFIELD
- **Nothing exists**. V0 is CLI-only.
- Need: MCP server (Streamable HTTP transport), tool definitions (tiered), request routing, migration coordination, operation tracking, progress notifications, per-body operation queuing.
- Deep design identifies: migration intent records (durability), tiered tool loading (5 core + 7 discoverable), error code mapping (gRPC → MCP).
- **Open design questions**: OQ1 (clean vs crash-consistent snapshot default), OQ2 (single binary vs daemon), OQ3 (migration intent storage: bbolt vs in-memory).

### 2. Provisioning (Provider Plugins) — GREENFIELD
- **Nothing exists**. V0 clone uses scp, not providers.
- Need: Substrate adapter gRPC contract, plugin-based provider system, Docker local provider (minimum for v1), possibly Nomad/E2B/Fly.
- Deep design identifies: liveness probe after Provision, pending-destroy queue, optional adapter methods (listInstances, WatchEvents).
- **Open design questions**: OQ1 (default provider selection = Q3), OQ2 (blocking vs async provision), OQ4 (endpoint URI format).

### 3. Orchestration (Body Lifecycle) — Mostly GREENFIELD
- V0 has `internal/agent/` with PID check only.
- Need: Full body state machine (8 states), body metadata store, substrate adapter calls, background health checker, garbage collector, migration record tracking, startup reconciliation.
- Deep design identifies: MigrationRecord ownership (recommends Orchestration owns it, not Interface), compensating actions for partial failures, write-ahead log for crash recovery.
- **Open design questions**: OQ1 (migration ownership: Interface vs Orchestration — deep design recommends Orchestration), OQ2 (metadata persistence: recommends SQLite with WAL), OQ3 (Error state recoverability).

### 4. Persistence (Snapshot + Storage) — PARTIALLY EXISTS (50% reusable)
- V0 has: tar+zstd pipeline, restore, manifest sidecar, pruning.
- V0 needs refactoring: Works on filesystem dirs, not containers. Need docker export/import, storage backend abstraction (S3/local), streaming to backend (io.Reader not []byte), metadata (CMD, ENV, WORKDIR from docker inspect).
- Deep design identifies: Manifest-first upload order (fix INV-1), platform validation at restore, body-level capture lock.
- **Open design questions**: Q-P1 (per-body prune config), Q-P2 (manifest format), Q-P5 (critical gap: Export/Import/Exec NOT in substrate adapter contract — needs resolution).

### 5. Networking (Tailscale) — GREENFIELD
- **Nothing exists**.
- Need: Tailscale integration, identity assignment/reassignment/revocation, DNS management, optional networking mode, TailnetProvider abstraction (Tailscale SaaS vs headscale).
- Deep design identifies: Per-VM Tailscale proxy for packed bodies (A2), userspace networking fallback, atomic identity swap during migration (new confirmed before old revoked).
- **Open design questions**: OQ-N1 (shared vs per-body Tailscale), OQ-N2 (DNS: MagicDNS vs Mesh overlay), OQ-N4 (cross-module migration gap).

### 6. Plugin Infrastructure — GREENFIELD
- **Nothing exists**.
- Need: go-plugin integration, gRPC protocol definitions (protobuf), plugin registry, plugin lifecycle (discover/load/configure/use/unload), auto-reload on crash, Pulumi generation integration.
- Deep design identifies: Crash-resilient wrapper (auto-reload with max retries), per-plugin mutex, lazy-load + idle-unload for memory on 2GB VMs.
- **Open design questions**: OQ1 (auto-reload vs manual), OQ2 (generation latency), OQ4 (plugin language: Go vs TypeScript).

---

## Cross-Module Architecture Decisions (MUST resolve before planning)

These affect the entire codebase structure. Can't start implementation without them:

1. **Process Architecture**: Single binary (`mesh serve` starts MCP + core + plugins) vs separate daemon (Mesh daemon + thin MCP wrapper)?
   - Implication: Affects deployment, crash isolation, module communication.

2. **Inter-Module Communication**: Direct Go function calls (same process) or gRPC between modules?
   - Implication: gRPC between all modules is overkill for a single binary. But plugin boundary IS gRPC.

3. **Metadata Storage**: What stores body state, migration records, plugin registry on 2GB VM?
   - Options: SQLite (recommended by deep designs), bbolt, flat JSON files.
   - Implication: Crash recovery, durability, query capability.

4. **Migration Ownership**: Who orchestrates the 7-step cold migration sequence?
   - SYSTEM.md says Interface. Deep orchestration design recommends Orchestration.
   - Implication: Crash recovery during migration depends on this.

5. **Substrate Adapter Protocol**: Extend the 6 verbs (create, start, stop, destroy, getStatus, exec) to include Export, Import, Inspect?
   - Persistence deep design flags this as a cross-module gap. Currently Export/Import not in adapter contract.
   - Implication: Persistence can't capture/restore without these.

6. **Daemon vs CLI-only for v1**: V0 is CLI-only. V1 needs MCP server (long-running process). Does v1 introduce `mesh serve` as a daemon?
   - Implication: systemd user service, PID management, graceful shutdown.

## Unresolved Discovery Questions (from open-questions.md)

- Q1: Where does a body live when idle? (affects scheduler)
- Q2: Registry strategy (where snapshots live) (affects Persistence storage)
- Q3: Scheduler — core or plugin? (affects Provisioning/Orchestration)
- Q4: Bootstrap — how does first install happen? (affects Interface/CLI)

## Code Reuse Assessment

| V0 Code | Reuse Potential | Notes |
|---------|----------------|-------|
| `internal/snapshot/` tar+zstd pipeline | HIGH | Core streaming pipeline reusable. Need to switch from filesystem walk → docker export stream. |
| `internal/restore/` extraction | HIGH | zstd decompression + tar extraction reusable. Need to add docker import + metadata application. |
| `internal/manifest/` JSON sidecar | MEDIUM | Manifest schema needs expansion (add CMD, ENV, WORKDIR, platform, body_id). |
| `internal/config/` TOML parsing | LOW | Need YAML, per-module config sections, different schema entirely. |
| `internal/clone/` scp/ssh | LOW | Replaced by migration flow (snapshot → provision → restore). SSH transport may survive as utility. |
| `internal/agent/` PID check | LOW | Replaced by body state machine + substrate adapter status. |
| `internal/transport/` SSH | LOW | May be useful for specific providers but not core. |
| `cmd/mesh/` Cobra CLI | MEDIUM | Keep as thin debugging surface (mesh init, mesh serve, mesh status). Not primary interface. |

---

## Previous Implementation (Python v0.4.0) — Key Learnings

**Path**: `/Users/samanvayayagsen/project/rethink-paradigms/infa/mesh-workspace/oss`

A CLI-first, multi-cloud infrastructure orchestration platform ("lightweight K8s alternative") built in Python. Used Pulumi + Nomad + Consul + Tailscale + Caddy.

### What Worked (Reusable Patterns for v1)
1. **UniversalCloudNode** — Pulumi Dynamic Resource wrapping Libcloud for 13+ providers. Single abstraction, zero provider-specific code in orchestration layer. Maps to substrate adapter concept.
2. **Progressive activation tiers** — Auto-detect topology, activate only needed services. LITE (1 node, ~200MB) vs STANDARD (2+ nodes, ~350MB). The TierConfig pattern (feature flags from topology) is clean.
3. **Boot script pipeline** — Jinja2 templates with StrictUndefined, modular phases (01-install-deps, 02-install-tailscale, etc.), cloud-init YAML output. Template validation post-render catches unreplaced vars.
4. **Nomad HCL templates** — Parameterized .nomad.hcl for container deployment. Directly reusable for agent body deployment.
5. **Atomic snapshot storage** — temp-file-then-rename pattern for metadata JSON.
6. **Plugin system via entry_points** — Zero-code plugin discovery.

### What Was Abandoned (Informs v1 Direction)
1. **Traefik/INGRESS/PRODUCTION tiers** (D9 discarded) — Web hosting framing is dead.
2. **CLI as primary interface** — Replaced by MCP (D5).
3. **Application-level snapshots** — Wrong abstraction level. Needed body-level docker export.
4. **13+ providers in core** — D6 moved all to plugins.
5. **Pulumi as provisioning engine in core** — Pulumi is now a plugin option, not core.
6. **Consul dependency** — Not needed for agent-body management.
7. **Web service deployment** — Mesh deploys agent bodies, not web apps.

### Scope Confirmed
**v1 = "Ship core loop first"**: Orchestration + Persistence + MCP Interface + Docker provider. No networking, no plugin generation. Core body lifecycle works end-to-end.
