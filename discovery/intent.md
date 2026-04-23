# Intent

## The Problem

Containers were designed stateless — kill and recreate, state in the DB. AI agents broke this. An agent is a persistent process that writes files as it works; the filesystem IS the state. Nothing in the existing stack is shaped for this.

## What We're Building

A portable agent-body runtime. Gives an AI agent a persistent, elastic compute identity (a "body") that can live on any substrate — always-on VM, shared-tenant fleet, ephemeral sandbox — and move between them without losing itself.

**The body is a filesystem.** An agent installs packages, writes files, modifies config. The body is the sum of all that state, portable as an OCI image + volume tarball. Where it physically runs (the "form") is a cost/latency knob, not an architectural commitment.

## Core Abstractions

- **Body**: permanent identity + filesystem state. Persists across substrate changes.
- **Form**: current physical instantiation on a specific substrate. Ephemeral by nature.
- **Substrate**: where a form runs. Three pools: Local (laptop/Pi), Fleet (user's BYO VMs via Nomad), Sandbox (Daytona, E2B, Fly, Modal, Cloudflare).

## User Profile

- Agent builders who need their agents to have persistent, portable compute.
- Small teams (1-5 people) or solo developers.
- Own their compute — BYO VMs, own API keys, own network.
- Don't want to manage infrastructure manually.
- Interface is natural language (MCP + skills), not CLI commands.

## Non-Goals (Explicit)

- Not competing with Hermes, Claude Code, or any specific agent. Mesh is the substrate agents run ON.
- Not building a hosted platform. Self-hosted, user-owned.
- Not building for hyperscale. If you have 200 VMs, you already solved this.
- Not requiring K8s. Ever.
- Not providing memory-state checkpointing in v0. Agents stop at task boundaries.
