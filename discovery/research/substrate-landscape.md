# Research: Stateful Workload Host Landscape

> Completed: 2026-04-22
> Scope: Systems that host stateful, long-lived workloads (AI agents whose filesystem IS their state)

## Comparison Table

| System | State model | Lifecycle verbs | Cold→warm | Portable? | Billing | Clone/fork | Standout quirk |
|--------|-------------|-----------------|-----------|-----------|---------|------------|----------------|
| **Daytona** | Opaque FS+memory snapshot | create/start/stop/pause/resume/archive/snapshot | sub-sec resume | No | Per-second compute; storage metered separately | Snapshot → spawn N new sandboxes | Three-tier lifecycle (running/stopped/archived) with distinct costs |
| **E2B** | Firecracker mem+FS snapshot | create/pause/resume/kill/Snapshot.create | ~1s resume, ~4s/GB pause | No | $0.0504/vCPU-hr, per-second | Snapshots spawn many from one point; no live fork | Bug: repeated pause/resume cycles lost FS deltas after first resume |
| **CF Durable Objects** | SQLite DB per object | get/deleteAll/bookmarks | <50ms | No (no POSIX FS) | Per-request + duration + SQL storage | No fork; one DO per ID | No POSIX filesystem — state must fit SQLite + KV |
| **CF Containers** | Ephemeral FS + R2 FUSE mount | start/stop/sleep | Seconds | Via OCI image | Per-instance active time | No clone | Disk resets on sleep — persistence requires R2 mount |
| **Fly Machines** | Firecracker full VM snapshot | suspend/resume/stop/start | Hundreds of ms | No | Per-second running; stopped = storage only | No native fork | Snapshots discarded on deploy or host migration |
| **Modal** | CRIU on gVisor, process tree + FS | implicit restore on invoke | sub-sec | No | Per-second CPU/GPU | Snapshot is read-only template; fan-out | GPU snapshotting is the differentiator |
| **Firecracker (raw)** | Memory + state files | CreateSnapshot/LoadSnapshot | ~28ms | Partial (same CPU+kernel) | N/A (OSS) | Restore many times = fork | Diff snapshots not resume-able directly |
| **Docker checkpoint** | CRIU dump + FS layer | checkpoint create/start --checkpoint | Seconds | Same kernel/arch only | N/A | None | Still experimental; ecosystem largely abandoned |
| **CRIU** | Process tree dump (no FS) | dump/restore/pre-dump | sub-sec | Same kernel/arch | N/A | dump --leave-running then restore elsewhere | FS not captured — must pair with overlay/btrfs snapshot |
| **k8s-sigs/agent-sandbox** | Pod + PVC, scale-to-zero | K8s verbs + Sandbox CRD | seconds | Across K8s clusters | N/A | PVC clone via CSI VolumeSnapshot | Singleton stateful agent pods with stable identity |
| **microsandbox** | OCI-compatible image + microVM fork | embedded SDK | N/A | OCI images | N/A | microVM fork | Needs /dev/kvm — not nestable on cloud VMs |

## Key Findings

### F1: No portable live-snapshot format exists
Every memory+FS snapshot is bound to its host — often to the specific CPU vendor and kernel version. Firecracker admits Intel↔AMD and cross-kernel are blocked. This is a hardware-level constraint.

### F2: Every provider exposes roughly the same four verbs
Create, pause/suspend, resume, destroy — plus snapshot/restore as a separate primitive. The substrate adapter interface is short and stable.

### F3: The only universally portable artifacts are OCI images + POSIX filesystem tarballs
Neither captures running memory. This is the honest baseline.

### F4: `docker commit` is wrong for persistence
Layered images grow monotonically and don't compose with repeated mutation. Nobody in production uses it for long-running stateful snapshots.

### F5: `docker export` is right for persistence
Flat tarball of the live FS. Captures everything. No overlay drama. Trivially transportable. Matches the "volume tarball" thesis.

### F6: Filesystem bloat is manageable
OverlayFS upperdir: deleting files created in upperdir does free space. Whiteouts only apply when shadowing lower-layer files. A pre-snapshot prune hook (pip cache purge, rm -rf /tmp/*, etc.) keeps snapshots lean.

## Synthesis

The clear gap: **nobody offers a portable snapshot format that moves a live agent across substrates.** FS-only portability (OCI + volume tar) is solved and boring. Memory/process portability is not, and CPU/kernel coupling blocks it at the primitive layer. The honest answer for a portable-agent-body layer is cold migration via flat filesystem tarballs, with provider-native suspend/resume as optional acceleration within a single substrate.
