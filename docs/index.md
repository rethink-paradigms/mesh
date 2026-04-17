# Mesh

**Deploy containers across any cloud. Zero-SSH deployment. Auto-HTTPS. Multi-cloud.**

Mesh turns any collection of VMs — across AWS, DigitalOcean, Google Cloud, and 13+ cloud providers — into a single unified computer. Pulumi provisions. Tailscale connects. Nomad schedules.

---

## Get Started

<div class="grid cards" markdown>

-   :rocket: **[15-Minute Quickstart](tutorials/quickstart.md)**

    Deploy your first app in under 15 minutes — local or cloud.

-   :book: **[Deployment Guide](guides/deploy.md)**

    Provider setup, instance sizing, and configuration reference.

-   :gear: **[Architecture](architecture/overview.md)**

    How Mesh works under the hood — components, data flow, tiers.

-   :terminal: **[API Reference](reference/api.md)**

    CLI commands, Python interfaces, and module contracts.

</div>

---

## Why Mesh

| Metric | Mesh | Kubernetes | Heroku |
|:---|:---|:---|:---|
| 3-node cluster | **$25/mo** | $72+/mo (control plane) | $250+/mo |
| Control plane RAM | **530MB** | 2GB+ | N/A (managed) |
| Setup time | **<15 min** | 2+ hours | <5 min |
| Multi-cloud | **Native** | Complex (federation) | No |
| Auto HTTPS | **Let's Encrypt** | Cert Manager + config | Built-in |
| SSH required | **Optional** | Often | Never |

See the [full comparison](comparisons.md) for more alternatives.

---

## Quick Install

```bash
pip install rethink-mesh
mesh init
```

That's it. The interactive wizard handles everything else.

For questions, see the [FAQ](faq.md) or [open an issue](https://github.com/rethink-paradigms/mesh/issues).
