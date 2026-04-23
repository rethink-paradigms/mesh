# Mesh System: Flow-Based Architecture

This diagram shows Mesh through its data flows and operations, not static module boxes.

## Three Core Operations

1. **Create Body** - Initialize a new agent body on a substrate
2. **Snapshot Body** - Persist body state to storage
3. **Migrate Body** - Move body between substrates (cold migration)

## Mermaid Sequence Diagram: Migrate Flow

The migration operation touches all six modules and demonstrates the full system flow.

```mermaid
sequenceDiagram
    autonumber
    participant Skill as Skill (via MCP)
    participant Interface as Interface<br/>(MCP Server)
    participant Orch as Orchestration<br/>(Lifecycle)
    participant Persist as Persistence<br/>(Snapshot + Storage)
    participant Prov as Provisioning<br/>(Provider Plugins)
    participant Net as Networking<br/>(Tailscale + Identity)

    Note over Skill,Net: Cold Migration: Move body from old substrate to new substrate

    Skill->>Interface: migrate(bodyId, targetSubstrate)
    Interface->>Orch: stop(bodyId)

    Note over Orch,Persist: Phase 1: Stop and Capture

    Orch->>Persist: capture(bodyId)
    Persist->>Persist: pre_prune()
    Persist->>Persist: docker_export()
    Persist->>Persist: zstd_compress()
    Persist->>Persist: store_to_backend()
    Persist-->>Orch: SnapshotRef

    Note over Persist,Prov: Phase 2: Provision New

    Orch->>Prov: provision(spec, targetSubstrate)
    Prov->>Prov: load_plugin(targetSubstrate)
    Prov->>Prov: create_instance()
    Prov-->>Orch: NewHandle

    Note over Persist,Net: Phase 3: Restore and Connect

    Orch->>Persist: restore(SnapshotRef, NewHandle)
    Persist->>Persist: fetch_from_backend()
    Persist->>Persist: zstd_decompress()
    Persist->>Persist: docker_import()
    Persist-->>Orch: Restored

    Orch->>Net: assign_identity(NewHandle)
    Net->>Net: tailscale_up()
    Net-->>Orch: NetworkIdentity

    Note over Orch,Prov: Phase 4: Start and Cleanup

    Orch->>Orch: start(NewHandle)
    Orch->>Prov: destroy(OldHandle)
    Orch-->>Interface: Migrated Body
    Interface-->>Skill: Migration complete
```

## ASCII Flow Matrix: All Three Operations

Modules as columns, operations as rows.

```
              Interface    Provisioning  Orchestration  Persistence   Networking   Plugin Infra
              ─────────    ────────────  ─────────────  ───────────   ──────────   ────────────

CREATE         receive ────► load ───────► create ──────►              ──────────   discover
                           plugin         body
              ───────────► provision ────►              ──────────   ──────────   ──────────
                             instance
              ───────────►              ────► start ──►              ──────────   ──────────
              ───────────►              ────►          ──────────►  assign ID ──► ──────────
                                                                      
SNAPSHOT       receive ────►              ────► handle ─► capture ───► ──────────   ──────────
              ───────────►              ────►          ───► prune ──► ──────────   ──────────
              ───────────►              ────►          ───► export ─► ──────────   ──────────
              ───────────►              ────►          ───► compress► ──────────   ──────────
              ───────────►              ────►          ───► store ──► ──────────   ──────────
              ◄── ref ────              ◄───          ◄─── ref ◄─── ──────────   ──────────

MIGRATE        receive ────►              ────► stop ──►              ──────────   ──────────
(COLD)         ───────────►              ────►          ───► capture► ──────────   ──────────
               ───────────► provision ──►              ──────────   ──────────   load plugin
                             (target)
               ───────────►              ◄─── handle ◄── restore ◄── ──────────   ──────────
               ───────────►              ────►          ──────────   assign ID ──► ──────────
               ───────────►              ────► start ─►              ──────────   ──────────
               ───────────► destroy ────►              ──────────   ──────────   ──────────
                             (old)
               ◄── done ──              ◄───          ◄───         ◄───        ◄───
```

## Key Insight

**Migration is the archetypal flow.** If you understand migration, you understand the system — it touches all 6 modules, exercises both directions of data flow, and combines create + snapshot patterns into one coordinated sequence.