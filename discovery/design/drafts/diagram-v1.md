# Mesh Architecture Diagram

## Mermaid Flowchart

```mermaid
flowchart TB
    %% External Boundary
    subgraph External["External Layer"]
        Skills_Agents[Skills/Agents]
    end

    %% Core Boundary
    subgraph Core["Core System"]
        direction TB

        Interface["Interface<br/>(MCP + Skills)<br/>External Entry Point"]
        Orchestration["Orchestration<br/>(Body Lifecycle)<br/>State Machine Manager"]
        Networking["Networking<br/>(Tailscale + Identity)<br/>Network Identity"]
        PluginInfra["Plugin Infrastructure<br/>(Discovery + Loading)<br/>Meta-layer"]
    end

    %% Plugin Boundary
    subgraph Plugins["Plugin Layer"]
        Provisioning["Provisioning<br/>(Provider Plugins)<br/>Provision Compute Substrates"]
        StorageBackends["Storage Backends<br/>Store/Retrieve Snapshots"]
    end

    %% External connections
    Skills_Agents -->|"lifecycle commands,<br/>snapshot commands,<br/>plugin management"| Interface

    %% Core internal connections
    Interface -->|"lifecycle commands"| Orchestration
    Interface -->|"snapshot commands"| Provisioning
    Interface -->|"plugin management"| PluginInfra

    %% Orchestration connections
    Orchestration -->|"provision compute"| Provisioning
    Orchestration -->|"assign identity"| Networking

    %% Plugin Infrastructure connections
    Provisioning -->|"load provider plugins"| PluginInfra
    StorageBackends -->|"load storage plugins"| PluginInfra

    %% Persistence connections
    Interface -->|"store/retrieve"| StorageBackends

    %% Styling
    classDef external fill:#f9f,stroke:#333,stroke-width:2px
    classDef core fill:#bbf,stroke:#333,stroke-width:2px
    classDef plugin fill:#bfb,stroke:#333,stroke-width:2px

    class Skills_Agents external
    class Interface,Orchestration,Networking,PluginInfra core
    class Provisioning,StorageBackends plugin
```

## ASCII Version

```
┌─────────────────────────────────────────────────────────────────────┐
│                        EXTERNAL LAYER                              │
│                                                                   │
│                        Skills/Agents                                │
└───────────────────────────────┬───────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         CORE SYSTEM                                │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    Interface (MCP + Skills)                 │   │
│  │                   External Entry Point                      │   │
│  └───────────────────────┬───────────────────────────────────┘   │
│                          │                                         │
│          ┌───────────────┼───────────────┐                      │
│          ▼               ▼               ▼                      │
│  ┌──────────────┐ ┌──────────────┐ ┌──────────────┐          │
│  │Orchestration │ │ Provisioning │ │Plugin Infra. │          │
│  │(Body Life-   │ │(Provider     │ │(Discovery +  │          │
│  │ cycle)       │ │ Plugins)     │ │ Loading)     │          │
│  └──────┬───────┘ └──────┬───────┘ └──────┬───────┘          │
│         │                 │                  │                     │
│         ▼                 └────────┬─────────┘                     │
│  ┌──────────────┐                  │                             │
│  │ Networking   │                  ▼                             │
│  │(Tailscale + │         ┌────────────────┐                     │
│  │ Identity)    │         │ Storage        │                     │
│  └──────────────┘         │ Backends       │                     │
│                          └────────────────┘                     │
└─────────────────────────────────────────────────────────────────────┘

                          PLUGIN LAYER
                    (Provisioning + Storage Backends)
```

## Legend

| Color/Section | Description |
|---------------|-------------|
| **External** | Skills and Agents that interact with the system |
| **Core** | Core modules: Interface, Orchestration, Networking, Plugin Infrastructure |
| **Plugin** | Extensible plugins: Provisioning providers, Storage backends |

## Module Descriptions

- **Interface (MCP + Skills)**: External entry point for the system. Handles lifecycle commands, snapshot operations, and plugin management.
- **Orchestration (Body Lifecycle)**: Manages the body state machine. Coordinates with Provisioning for compute resources and Networking for network identity.
- **Networking (Tailscale + Identity)**: Assigns network identity to bodies using Tailscale.
- **Plugin Infrastructure (Discovery + Loading)**: Meta-layer that enables dynamic loading of Provisioning and Storage plugins.
- **Provisioning (Provider Plugins)**: Plugin layer for provisioning compute substrates from various providers.
- **Storage Backends**: Plugin layer for storing and retrieving body snapshots.
