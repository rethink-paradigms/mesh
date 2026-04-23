# Research: kubernetes-sigs/agent-sandbox

> Completed: 2026-04-22
> Source: GitHub repo, Google OSS Blog (Nov 2025), CRD source code, roadmap

## Problem Statement (Their Words)

> "Autonomous AI Agents capable of reasoning, planning, and executing actions by generating their own code introduce a fundamental security gap: how to safely allow agents to run untrusted, unverified generated code."

> "Agent behavior often involves quick, iterative tool calls ... each of these calls requires its own isolated sandbox. The challenge is that these sandboxes must be created from scratch, extremely quickly."

> "Up to tens of thousands of parallel sandboxes, processing thousands of queries per second."

**Their north star:** GKE-hyperscale ephemeral code execution for LLM tool calls.

**Our north star:** Portable long-lived agent bodies across heterogeneous substrates including edge.

Related problems, not the same.

## CRD Shape

Four resources:

| Resource | Purpose | Key Fields |
|----------|---------|------------|
| **Sandbox** | The instance. Singleton pod with stable identity. | `podTemplate`, `volumeClaimTemplates`, `replicas` (0 or 1), `lifecycle.shutdownTime` |
| **SandboxTemplate** | Platform-team-owned blueprint + security policy. | `podTemplate`, `networkPolicy` (Managed/Unmanaged), `envVarsInjectionPolicy` |
| **SandboxClaim** | User-facing "give me a sandbox from template X." | `templateRef`, `lifecycle`, `envVar overrides`, `warmPoolPolicy` |
| **SandboxWarmPool** | Pre-warmed pods for sub-second new-spawn. | `replicas` (HPA-controllable), `updateStrategy` |

`replicas=0` tears the pod down but keeps CR + PVC + headless Service. That's the hibernate primitive.

## Scale-to-Zero Mechanics

- Pod deleted, PVC retained, CR retained, headless Service retained
- Wake = normal pod scheduling cold start (seconds)
- Warm pools are for *new* sandboxes, not reviving scaled-down ones
- PVC-based resume explicitly listed as unfinished in roadmap

## Identity Model

- Stable hostname = `metadata.name`
- Headless Service with `.status.serviceFQDN`
- Pod name stabilized via annotation when adopted from warm pool
- Identity survives pod recreation (Service + PVC + CR persist)

## State Model

- PVC only. No VolumeSnapshot integration in spec.
- No image-commit flow. No "agent-body as portable tarball."
- No memory-state checkpointing. Roadmap mentions as desired, zero implementation.

## Isolation

- Runtime-agnostic via K8s RuntimeClass (gVisor, Kata, runc)
- NetworkPolicy default-deny when template uses Managed policy
- Multi-tenant safety = your runtime choice + NetworkPolicy enforcement

## Agent-Facing Interface

- Python SDK wraps K8s API for create/exec/read/write
- No MCP surface
- No sibling-spawn primitive without K8s RBAC
- Agent inside sandbox has no standardized back-channel to controller

## K8s Dependencies (Load-Bearing)

PodTemplate (corev1.PodSpec), PVC + CSI, headless Service + cluster DNS, NetworkPolicy (CNI), HPA, controller-runtime, RuntimeClass. None ports cleanly to Nomad without re-implementing a small K8s.

## Maturity

- Created Aug 2025, announced Nov 2025
- ~1,884 stars, 60 contributors, Google-dominated
- v1alpha1 API — pre-beta
- Active: pushed within last day
- Open issues: PyPI pending, Go client missing, PVC resume not implemented, metadata propagation broken

## What Mesh Can Borrow vs. Must Reject

### Borrow (API shape only, not implementation)
- Sandbox / Template / Claim / WarmPool four-resource decomposition
- `replicas: 0|1` as the scale-to-zero primitive
- `shutdownTime` + `shutdownPolicy` lifecycle fields
- Template/Claim separation for platform-team security boundary

### Reject (K8s-tied implementation)
- PodTemplate as the container spec format
- PVC/CSI for state persistence
- Headless Service + cluster DNS for identity
- NetworkPolicy via CNI
- HPA for warm pool scaling
- The entire reconciler pattern over K8s control plane

### What They Don't Solve (Mesh's Actual Differentiators)
- Portable agent-body as OCI-image-plus-volume-tarball (no commit/export primitive)
- Agent-to-controller MCP surface (no back-channel)
- Cross-substrate migration (single-cluster assumption)
- Edge/2GB VM deployment
- Running without K8s
- Filesystem bloat management over long-running life

## Verdict

Complementary, not competitive. agent-sandbox optimizes the "ephemeral isolated exec for LLM tool calls at hyperscale on GKE" lane. Mesh optimizes the "portable long-lived agent bodies across heterogeneous substrates including edge" lane. The API shape (Sandbox/Template/Claim/WarmPool) is worth adopting as a mental model. The K8s implementation is not.
