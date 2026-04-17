# Comparisons

How Mesh compares to other infrastructure and platform-as-a-service tools.

---

## At a Glance

| | **Mesh** | **Kubernetes** | **Heroku** | **Coolify** | **Dokku** | **CapRover** |
|:---|:---|:---|:---|:---|:---|:---|
| **Type** | Orchestration | Container orchestration | Managed PaaS | Self-hosted PaaS | Mini-Heroku | Self-hosted PaaS |
| **Setup time** | <15 min | 2+ hours | <5 min | ~30 min | ~10 min | ~15 min |
| **3-node cost** | ~$25/mo | $72+/mo | $250+/mo | $20+/mo | $10+/mo | $20+/mo |
| **Control plane RAM** | ~530MB | 2GB+ | N/A | ~500MB | ~200MB | ~500MB |
| **Multi-cloud** | Native | Complex | No | No | No | No |
| **Auto HTTPS** | Yes | Requires setup | Yes | Yes | Plugin | Yes |
| **SSH required** | Optional | Often | Never | Yes | Yes | Yes |
| **Container runtime** | Docker | containerd/CRI-O | Buildpacks/Docker | Docker | Docker | Docker |
| **Scheduler** | Nomad | kube-scheduler | proprietary | Docker | Docker | Docker |
| **Service mesh** | Tailscale (WireGuard) | Istio/Linkerd (optional) | — | — | — | — |
| **Language** | Python | Go | — | PHP/JS | Shell/Go | JS/TS |
| **License** | MIT | Apache 2.0 | Proprietary | MIT | MIT | Apache 2.0 |

---

## Mesh vs Kubernetes

### When to choose Mesh

- You want **multi-cloud** without federation complexity
- Your team doesn't have Kubernetes expertise
- You're deploying **simple container workloads** (web apps, APIs, workers)
- You want **zero SSH** infrastructure management
- RAM budget is tight (Mesh uses 530MB vs K8s 2GB+ for control plane)

### When to choose Kubernetes

- You need the **Kubernetes ecosystem** (operators, Helm charts, CRDs)
- You're running **complex stateful workloads** (databases, message queues)
- You have a **dedicated platform team** to manage the cluster
- You need **pod-level networking policies** and advanced security contexts
- Your org already has K8s infrastructure and expertise

### Key difference

Kubernetes is a **general-purpose container orchestration platform** with a massive ecosystem. Mesh is an **opinionated deployment platform** that makes the 80% case (deploy containers, get HTTPS, scale out) trivially simple.

---

## Mesh vs Heroku

### When to choose Mesh

- **Cost matters** — Mesh is 10x cheaper at scale
- You need **multi-cloud** or specific provider support
- You want to **own your infrastructure** (no vendor lock-in)
- You need **custom networking** (Tailscale mesh, WireGuard)
- You're running workloads Heroku doesn't support well (long-running workers, GPU)

### When to choose Heroku

- You want **zero infrastructure management** (fully managed)
- Your team is small and **dev velocity matters more than cost**
- You use **Heroku Add-ons** (Postgres, Redis, etc.)
- You need **buildpacks** and don't want to write Dockerfiles
- You're prototyping and need to ship fast

### Key difference

Heroku is a **fully managed platform** — you never touch infrastructure. Mesh gives you **self-managed infrastructure** that's almost as easy, at 10x lower cost, with multi-cloud capability.

---

## Mesh vs Coolify

### When to choose Mesh

- You need **multi-cloud** or multi-provider support
- You want **programmatic infrastructure** (IaC with Pulumi)
- You prefer **CLI-driven** workflows over web UIs
- You need **WireGuard mesh networking** between nodes
- You want **zero SSH** management

### When to choose Coolify

- You prefer a **web-based UI** for all management
- You're running on a **single server** or single provider
- You need **one-click app installs** (WordPress, databases, etc.)
- You want **built-in database** management (Postgres, MySQL, Redis)
- You're a solo developer or small team who values visual management

### Key difference

Coolify is a **web UI for server management**. Mesh is a **CLI-driven orchestration platform** with multi-cloud networking. Coolify excels at single-server management; Mesh excels at multi-node, multi-cloud deployments.

---

## Mesh vs Dokku

### When to choose Mesh

- You need **multi-node** or **multi-cloud** deployments
- You want **automatic service discovery** and load balancing
- You need **encrypted mesh networking** between nodes
- You want **resource-aware scheduling** across multiple machines
- You're building production infrastructure, not a single-server hobby project

### When to choose Dokku

- You want the **simplest possible** deployment on a **single server**
- You're a **solo developer** with one VPS
- You love the **`git push` deploy** workflow
- You don't need multi-node scaling
- You want **Dockerfile-based** deployments without any orchestration

### Key difference

Dokku is **single-server Docker management** with a Heroku-like git push workflow. Mesh is **multi-cloud orchestration** with service discovery, scheduling, and mesh networking. If you need >1 server, Mesh is the better choice.

---

## Mesh vs CapRover

### When to choose Mesh

- You need **multi-cloud** deployments
- You want **CLI-first** infrastructure management
- You need **encrypted mesh networking** (WireGuard)
- You prefer **declarative** IaC over imperative management
- You need **automatic tier detection** and progressive activation

### When to choose CapRover

- You want a **web dashboard** for management
- You're deploying to a **single cluster** on one provider
- You need **one-click marketplace apps**
- You want **cluster token-based** multi-node (simpler than Tailscale)
- You're already using Docker Swarm

### Key difference

CapRover uses **Docker Swarm** for clustering and provides a **web UI**. Mesh uses **Nomad** for scheduling and provides a **CLI**. Mesh's multi-cloud mesh networking is unique among self-hosted platforms.

---

## Decision Matrix

| Your need | Best choice |
|:---|:---|
| "I just want to deploy a web app fast" | Dokku or Heroku |
| "I need multi-cloud under $50/mo" | **Mesh** |
| "I have a platform team and complex workloads" | Kubernetes |
| "I want a web UI for my server" | Coolify or CapRover |
| "I need the cheapest possible production setup" | **Mesh** (Lite tier, $8/mo) |
| "I'm a solo dev with one VPS" | Dokku or Coolify |
| "I need encrypted networking between cloud providers" | **Mesh** |
| "Zero infrastructure management, cost no object" | Heroku |
