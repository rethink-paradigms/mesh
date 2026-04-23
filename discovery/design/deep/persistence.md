# Deep-Dive: Persistence

> Module: Persistence (Snapshot + Storage)
> Agent: deep
> Date: 2026-04-23
> Status: draft

## Contract (Detailed)

### Inputs
- `SnapshotEngine.Capture(bodyId)` — receives a body identifier from Orchestration. Calls Orchestration to resolve to a container handle.
- `SnapshotEngine.Restore(snapshotRef, targetBody)` — receives a storage reference and a target body. Calls StorageBackend to retrieve snapshot data.
- `StorageBackend.Put(snapshot)` — receives a Snapshot struct (manifest JSON + tarball stream).
- `StorageBackend.Get(ref)` — receives a snapshot reference string.
- Lifecycle ops: `List()`, `Delete(ref)`, `GarbageCollect(policy)`.

### Outputs
- `Capture → SnapshotRef` — opaque reference usable for Restore, List, Delete.
- `Restore → Body` — returns image reference with metadata applied; caller (Orchestration) creates the running container.
- `List → []SnapshotMeta` — id, size, created_at, body_id, platform, tags.
- `Delete → error | nil`

### Guarantees (Invariants)
- **INV-1**: Capture either produces a complete, restorable snapshot (manifest + tarball stored in backend) or returns an error. No partial snapshots in storage.
- **INV-2**: Restore(body) produces a filesystem-identical body with correct runtime metadata (CMD, ENV, WORKDIR, USER, EXPOSE). If metadata is missing, restore fails with an explicit error, not silent degradation.
- **INV-3**: A stored snapshot is portable across all OCI-compatible substrates on the same platform (amd64/amd64, arm64/arm64). Cross-platform restore attempts fail with a clear error.
- **INV-4**: Snapshot pipeline memory usage is bounded (streaming, no full-tarball-in-memory). Respects C1 (2GB VM).
- **INV-5**: At most one capture operation per body at any time. Concurrent capture requests for the same body are serialized, not duplicated.

### Assumptions
- **ASM-1**: Orchestration provides a valid container handle (running or stopped). If the container doesn't exist, Orchestration returns an error — Persistence doesn't discover this independently.
- **ASM-2**: The container runtime supports `docker export` (or substrate-adapter equivalent). Substrate adapter exposes an `Export(bodyId) → io.ReadCloser` method.
- **ASM-3**: The body's filesystem is consistent at export time. For running containers, callers should stop the body first (D4 cold migration). Persistence does NOT stop bodies — that's Orchestration's job.
- **ASM-4**: StorageBackend plugins handle their own retry logic for transient failures (network blips, rate limits). Persistence retries at the pipeline level only, not inside the plugin.
- **ASM-5**: Volume data is outside Persistence's scope (per D2, F5). Callers handle volume backup/restore separately.

## State Machine

### States
- **None** — no snapshot exists for this body+ref.
- **Capturing** — pipeline in progress (pre-prune → export → compress → store).
- **Stored** — complete snapshot in storage backend. Restorable.
- **Deleting** — GC or explicit delete in progress.
- **Failed** — capture failed; no valid snapshot. Orphan cleanup may be needed.

### Transitions

| From | Trigger | To | Side Effects |
|------|---------|----|--------------|
| None | Capture(bodyId) | Capturing | Acquire body-level lock. Begin pipeline. |
| Capturing | Pipeline succeeds | Stored | Release lock. Return SnapshotRef. |
| Capturing | Pipeline fails | Failed | Release lock. Clean up temp artifacts. Return error. |
| Stored | Delete(ref) | Deleting | Begin storage backend delete. |
| Deleting | Delete succeeds | None | Snapshot removed from backend. |
| Stored | Capture(bodyId) (new snapshot) | Capturing | Creates NEW snapshot ref. Previous Stored snapshot unaffected. |
| Failed | Capture(bodyId) (retry) | Capturing | Body-level lock acquired. Previous failed artifacts cleaned first. |

### Illegal Transitions

| From | Rejected Trigger | Error Type | Recovery |
|------|-----------------|------------|----------|
| Capturing | Capture(same body) | CONFLICT — snapshot already in progress | Wait for existing capture, or cancel and retry. Caller should poll. |
| None | Restore(ref) | NOT_FOUND — no such snapshot | Caller must Capture first. |
| Deleting | Restore(same ref) | NOT_FOUND — snapshot being deleted | Use a different ref or wait. |
| Stored | Restore(ref, wrong_platform) | INVALID_ARGUMENT — platform mismatch | Must restore on matching platform. See INV-3. |

## T1: Happy Path Traces

### Capture (A1: Hermes periodic snapshot)

1. Skill calls `mesh.body.snapshot("hermes-42")` via MCP.
2. Interface routes to `SnapshotEngine.Capture("hermes-42")`.
3. Persistence calls Orchestration: `GetHandle("hermes-42")` → container ID `abc123`.
4. Persistence acquires body-level lock on `hermes-42`.
5. Substrate adapter: `Exec(container="abc123", cmd="sh -c 'apt-get clean && rm -rf /tmp/* /var/tmp/* ~/.cache/pip ~/.npm'")` → success (best-effort; failure logged but not fatal).
6. Substrate adapter: `Inspect(container="abc123")` → metadata `{Cmd: ["python", "agent.py"], Env: {"AGENT_ID": "h42"}, Workdir: "/agent", User: "agent", ExposedPorts: ["8080/tcp"]}`.
7. Substrate adapter: `Export(container="abc123")` → `io.ReadCloser` (tar stream).
8. Stream: `export_stream | zstd -3 | storage_backend.Put(key, stream)`. No local temp file. Manifest JSON stored as separate object with same key prefix.
9. Manifest: `{body_id: "hermes-42", snapshot_ref: "snap-abc", platform: "linux/amd64", metadata: {cmd, env, workdir, user, ports}, tarball_size: 3200000000, created_at: "2026-04-23T10:30:00Z"}`.
10. Lock released. Return `SnapshotRef("snap-abc")` to caller.

### Restore (Migration to new substrate)

1. Skill calls `mesh.body.restore("snap-abc", "hermes-42-new")`.
2. Persistence calls `StorageBackend.Get("snap-abc")` → `(manifest_stream, tarball_stream)`.
3. Parse manifest. Validate `platform == "linux/amd64"` matches target substrate. Validate all required metadata fields present (INV-2).
4. Construct `--change` flags from manifest: `--change "CMD ['python', 'agent.py']" --change "ENV AGENT_ID=h42" --change "WORKDIR /agent" --change "USER agent" --change "EXPOSE 8080/tcp"`.
5. Stream: `tarball_stream | zstd -dc | substrate_adapter.Import(changes, platform="linux/amd64")` → image ref `hermes-42-restored:latest`.
6. Return image ref + metadata to caller. Orchestration creates container from this image.

## T2: Failure Analysis

| External Call | Failure Mode | State Left | Recovery Path | Cleanup By |
|---|---|---|---|---|
| Orchestration.GetHandle | Body not found / wrong state | None (before lock) | Return `NOT_FOUND` to caller. | N/A |
| Substrate.Exec (pre-prune) | Exec fails (container lacks sh, permission error) | None (before export) | Log warning, proceed without prune. Pre-prune is best-effort. | N/A |
| Substrate.Inspect (metadata) | Inspect fails (container destroyed between steps) | Capturing | Abort pipeline. Return `INSTANCE_NOT_FOUND`. | Persistence cleans lock. |
| Substrate.Export | Export fails mid-stream (disk I/O error, container destroyed) | Capturing, partial stream open | Abort. Close stream. Return `UNKNOWN`. | Persistence closes export stream. Storage backend's partial upload cleaned by backend plugin. |
| Substrate.Export | Export succeeds but container was running (ASM-3 violated) | Capturing | Complete pipeline. Snapshot reflects potentially inconsistent FS. Caller's fault for not stopping. Warn in logs. | N/A |
| zstd compress | Process killed (OOM on 2GB VM with `--long` flag) | Capturing, partial write to backend | Abort. Don't use `--long` flag. Use `-3` with `--memlimit=256M`. | Storage backend handles partial upload abort. |
| StorageBackend.Put (manifest) | Network timeout, auth error | Capturing, tarball stored but manifest missing | Retry with backoff (3 attempts). If persistent, fail pipeline. | Tarball is orphaned in backend. GC cleans later. |
| StorageBackend.Put (tarball) | Multipart upload fails mid-transfer | Capturing, partial blob in storage | Backend plugin retries multipart. If exhausted, abort pipeline. | Backend plugin aborts multipart. GC cleans orphaned parts. |
| StorageBackend.Get | Not found, network error | None (read-only) | Return `NOT_FOUND` or retry on transient. | N/A |
| Substrate.Import | Import fails (disk full on target) | Target body in broken state | Return error. Caller must clean up target body. | Orchestration destroys broken body. |
| Substrate.Import | Platform mismatch (amd64 tarball on arm64 host) | Failed import, corrupted image | Validate platform in manifest BEFORE import (INV-3). Fail fast. | N/A |

**Critical finding**: The pipeline has a consistency gap between tarball upload and manifest upload. If tarball succeeds but manifest upload fails, the backend has an orphaned tarball with no metadata. Fix: **upload manifest first (small, fast), then tarball**. On Restore, if manifest exists but tarball doesn't, snapshot is incomplete — fail with an explicit error. On GC, any tarball without a matching manifest is garbage.

## T3: Concurrency Analysis

### Conflicting Operations
1. **Capture + Capture (same body)**: Both would call `docker export` on the same container simultaneously. `docker export` is a read-only operation on OverlayFS merged view — it CAN run concurrently. But: both execute pre-prune concurrently (race on cache files), both waste I/O and storage. **Must serialize** per body.
2. **Capture + Destroy (same body)**: Orchestration destroys the container while Persistence is mid-export. Export stream breaks mid-read. **Body-level lock prevents this** — Destroy must wait for Capture to complete, or Capture must be cancellable.
3. **Restore + Restore (same snapshot ref, different targets)**: Both call `StorageBackend.Get` for the same ref. Storage backends handle concurrent reads natively. **No conflict.**
4. **Delete + Restore (same ref)**: Restore starts streaming, Delete removes the blob. Result depends on backend. S3 with versioning: Restore still works (reads old version). S3 without versioning: Stream breaks. **Fix: Delete is soft (mark for deletion). Hard delete deferred until no active restores.**

### Proposed Locking
- **Body-level lock** for Capture operations. Keyed on `body_id`. Lock is held for the full pipeline duration (pre-prune through store). Timeout: configurable, default 30 minutes (10GB container export time). Lock is in-memory (single Mesh instance assumption for v0).
- **Snapshot ref refcount** for Delete/Restore. Before hard-delete, check active Restore streams. If any, defer deletion. Implemented as an atomic counter on the snapshot ref.
- **No cross-body locking.** Capture(bodyA) and Capture(bodyB) run independently. They only contend on shared I/O bandwidth (T4 concern, not locking).

## T4: Scale Analysis

### Bottlenecks

| Resource | Limit | Threshold | Mitigation |
|----------|-------|-----------|------------|
| Disk I/O (export) | ~500MB/s NVMe sequential read | 10 simultaneous exports of 10GB bodies = 100GB read, ~200s each with contention | Limit concurrent captures (configurable, default 3). Queue remaining. |
| Disk I/O (import) | ~500MB/s NVMe sequential write | 10 simultaneous restores = same problem | Same: limit concurrent restores. |
| CPU (zstd) | All cores with `-T0` | 3 concurrent captures = 3 zstd processes each using all cores = thrashing | zstd without `-T0` (single-threaded per capture), OR limit concurrent captures to core count / 2. |
| Network (storage upload) | ~1Gbps typical VM | 10GB compressed tarball at 1Gbps = ~80s. 3 concurrent = 240s contention | Storage backend plugins handle upload concurrency internally. Use multipart uploads. |
| Local disk space (temp) | Limited on 2GB VMs | Cannot buffer 10GB tarball locally | **Streaming pipeline: no local temp files.** docker export | zstd | storage.Put must be a single pipe, no intermediate file. |
| Snapshot storage growth | Unbounded | 100 bodies × 10 snapshots × 5GB = 5TB | GC policy. Default: keep last N snapshots per body (configurable). Manual delete for others. |
| S3 API rate limits | 5,500 GET/s, 3,500 PUT/s per prefix | Unlikely to hit with <1000 bodies | Sharded key prefix if needed. Not a v0 concern. |

**Critical finding**: The streaming pipeline (`docker export | zstd | storage.Put`) is NOT optional — it's required by C1. Any design that buffers the full tarball to local disk before uploading violates the 2GB VM constraint for bodies > 2GB. The StorageBackend plugin interface MUST accept an `io.Reader` stream, not a `[]byte` or file path. The `BodySnapshot` struct in registry-strategy.md uses `[]byte` for `VolumeTarball` — **this is wrong for large bodies** and must be changed to `io.Reader`.

## T5: Edge Cases

### EC1: Body in "Starting" state during Capture
Capture is requested before the body has finished booting. `docker export` works on any container state (even Created, not yet Started), but the FS may be incomplete (packages still installing, configs not written). **Expected behavior**: Orchestration should reject Capture if body is not in "Running" or "Stopped" state. Persistence trusts Orchestration's state check (ASM-1). If Orchestration doesn't enforce this, snapshot captures an inconsistent FS — caller's fault.

### EC2: Snapshot of destroyed body
Body is destroyed (container removed) between GetHandle and Export. Export fails with "no such container." **Recovery**: Return `INSTANCE_NOT_FOUND`. Clean up lock. No orphaned storage.

### EC3: Restore with missing metadata fields
Manifest exists but `cmd` is null. The imported image defaults to `/bin/sh` — agent won't start correctly. **INV-2 enforcement**: Restore fails with `INVALID_ARGUMENT: required metadata field 'cmd' is null in snapshot manifest`. Does NOT silently produce a broken body.

### EC4: Cross-platform restore attempt
Snapshot captured on amd64, restore attempted on arm64 substrate. `docker import` succeeds (tarball is architecture-agnostic) but contained binaries won't execute. **Fix**: Manifest includes `platform` field. Restore validates platform matches target before importing. Fail fast with `INVALID_ARGUMENT: snapshot platform linux/amd64 does not match target linux/arm64`.

### EC5: A4 Burst Clone — FS delta merge
Clone modifies filesystem, task completes, caller requests merge back to parent. **Options**:
1. **Full re-tarball**: Export clone FS → stop parent → import as new parent image → restart parent. Downtime = export + import time. Simple. Correct. Loses parent's running processes (acceptable per D1).
2. **Upperdir diff**: Only export clone's OverlayFS upperdir (changes). Apply to parent's upperdir. Complex: must handle whiteouts, file ordering, concurrent writes to parent. Risk of corruption.
3. **File-level sync**: `rsync` from clone FS to parent FS. Requires both mounted simultaneously. Substrate-specific. Not portable.

**Recommendation**: v0 uses full re-tarball (option 1). It's honest, portable, and matches D4 (cold migration). Upperdir diff (option 2) is a v1+ optimization that requires deep OverlayFS coupling and violates the simplicity principle. Document that merge = stop parent → snapshot clone → restore over parent → start parent. Downtime is proportional to clone's FS size.

### EC6: Very large body on 2GB VM (A1: 10+ GB)
Hermes running for weeks, 10GB filesystem. Capture on 2GB VM: `docker export` streams (no memory issue), `zstd` streams with `--memlimit=256M` (bounded), `storage.Put` streams to backend (no local temp). The pipeline works IF AND ONLY IF it's fully streaming. Any buffering step breaks. **Fix**: Pipeline design is a single Unix-style pipe. No intermediate files. StorageBackend.Put receives `io.Reader`, not a byte slice. If backend plugin doesn't support streaming (e.g., needs content-length upfront), use HTTP `Transfer-Encoding: chunked` or S3 multipart with unknown total size (upload parts as they arrive).

### EC7: Pre-prune race with running agent
Pre-prune runs `rm -rf ~/.cache/pip` while agent is actively installing packages. Agent's `pip install` may fail or produce corrupt cache state. **Fix**: Pre-prune is best-effort and advisory. It should NOT fail the snapshot if it errors. The snapshot captures whatever FS state exists. For consistent snapshots, stop the body first (D4).

## T6: Contract Verification

### Invariant Checklist

| Invariant | T1 | T2 | T3 | T4 | T5 | Status |
|-----------|----|----|----|----|----|----|
| INV-1: Complete or error | ✅ | ❌ tarball stored but manifest missing | ✅ | ✅ | ✅ | BROKEN (T2) |
| INV-2: Identical FS + metadata | ✅ | ✅ | ✅ | ✅ | ❌ EC3 (missing metadata produces wrong body if not validated) | BROKEN (T5) |
| INV-3: Portable across substrates | ✅ | ✅ | ✅ | ✅ | ❌ EC4 (cross-platform silent corruption) | BROKEN (T5) |
| INV-4: Bounded memory | ✅ | ✅ | ✅ | ❌ concurrent zstd -T0 | ✅ | NEEDS GUARD |
| INV-5: One capture per body | ✅ | ✅ | ❌ without lock, duplicates | ✅ | ✅ | NEEDS LOCK |

### Broken Invariants → Design Changes

**INV-1 broken by T2**: Partial tarball in storage when manifest upload fails. Fix: **Reverse upload order — manifest first, then tarball.** Manifest is tiny (<1KB), nearly always succeeds. If manifest upload fails, no tarball is uploaded (clean state). If tarball upload fails after manifest, manifest marks snapshot as `status: incomplete`. GC cleans incomplete snapshots. Restore checks `status == "complete"` before proceeding.

**INV-2 broken by T5 (EC3)**: Missing metadata produces wrong body. Fix: **Manifest schema validation at Restore time.** Required fields: `cmd` (or `entrypoint`), `platform`, `created_at`. If any required field is null/missing, Restore returns `INVALID_ARGUMENT`. Never silently degrade.

**INV-3 broken by T5 (EC4)**: Cross-platform restore produces unusable body. Fix: **Platform field in manifest + validation at Restore time.** Compare manifest.platform against target substrate's reported platform. Mismatch = error. Document that cross-platform migration requires rebuilding the body from source, not snapshot transfer.

**INV-4 at risk from T4**: Concurrent zstd processes with `-T0` can exhaust memory. Fix: **Don't use `-T0` by default.** Use single-threaded zstd per capture. Concurrency limit (default 3) caps total CPU usage. For single-capture scenarios on beefy machines, allow `-T0` via config opt-in.

**INV-5 needs lock**: Body-level mutex for Capture. In-memory for v0. Configurable timeout. If mesh is distributed in future, move to distributed lock (e.g., Nomad lock, or storage-backend CAS).

## Updated Interface

### Revised from registry-strategy.md analysis

```
SnapshotEngine:
  Capture(bodyId: string, opts?: CaptureOptions) → SnapshotRef
  Restore(ref: SnapshotRef, target: RestoreTarget) → RestoredImage
  List(bodyId?: string) → []SnapshotMeta
  Delete(ref: SnapshotRef) → error
  GarbageCollect(policy: GCPolicy) → []DeletedRef

CaptureOptions:
  prune: bool (default: true)
  compression_level: int (default: 3)
  labels: map<string, string>

RestoreTarget:
  substrate_type: string
  platform: string  // validated against manifest

RestoredImage:
  image_ref: string
  metadata: BodyMetadata  // → restored metadata

StorageBackend (plugin interface — stream-based):
  Put(ctx, ref: string, manifest: []byte, tarball: io.Reader) → PutResult
  Get(ctx, ref: string) → (manifest: []byte, tarball: io.ReadCloser, err)
  Delete(ctx, ref: string) → error
  List(ctx, prefix?: string) → []string
  HealthCheck(ctx) → bool

GCPolicy:
  max_snapshots_per_body: int (default: 10)
  max_age: duration (default: 0 = forever)
  cleanup_incomplete: bool (default: true)
```

**Key change**: `BodySnapshot.VolumeTarball` is `io.Reader` not `[]byte`. The registry-strategy.md proposal used `[]byte` which buffers the entire tarball in memory — fatal for 10GB bodies on 2GB VMs.

**Key change**: `StorageBackend.Put` accepts manifest and tarball separately. Manifest is uploaded first. This fixes INV-1.

**Key change**: `RestoreTarget` includes `platform` for INV-3 validation.

## Open Questions

- **Q-P1**: Should the pre-prune hook be configurable per body? (A1 Hermes may want to keep HF model cache; A2 Tool Agent wants aggressive prune.) — Matters for snapshot size. — Options: per-body prune config in body spec, or global default with body-level override.
- **Q-P2**: Snapshot manifest format — standalone JSON in storage backend, or OCI image config blob in a registry? — Determines how metadata travels with tarball. — Options: (a) JSON file alongside tarball in blob storage (simpler, S3-native), (b) OCI manifest wrapping tarball as a layer (registry-native, but layer semantics are awkward for flat tarballs). Recommend (a) for v0.
- **Q-P3**: Garbage collection trigger — who runs it? — Options: (a) Manual via MCP tool `mesh.snapshot.gc()`, (b) Background goroutine with configurable interval, (c) After every Capture (check if over limit). — Recommend (c) for v0 (simplest, bounded storage growth).
- **Q-P4**: The A4 merge workflow (clone → parent FS merge) — is full re-tarball acceptable for v0, or does it need optimization? — Full re-tarball means parent downtime = clone's FS size / I/O bandwidth. For a 5GB clone on NVMe, ~30s. Acceptable? — Options: (a) Full re-tarball only (v0), (b) Add upperdir-only export as optimization (v1). Recommend (a).
- **Q-P5**: Cross-module gap — Orchestration's `GetHandle` must return enough info for Persistence to call `Export`, `Inspect`, `Exec` on the container. This means that substrate adapter must expose these methods. Currently, substrate adapter in SYSTEM.md defines 6 verbs (Create, Start, Stop, Destroy, Status, List). Export/Import/Exec/Inspect are NOT in the adapter contract. **This is a cross-module design gap.** — Options: (a) Add Export/Import/Exec/Inspect to substrate adapter as optional capabilities, (b) Persistence bypasses adapter and talks directly to Docker/container runtime. Recommend (a) — keeps the plugin boundary clean.
