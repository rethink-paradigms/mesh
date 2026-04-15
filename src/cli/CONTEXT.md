# Domain: Mesh CLI

**Description:**
Command-line interface for the Distributed Mesh Platform. Provides interactive cluster provisioning, agent lifecycle management, and cluster status visualization using Rich-formatted terminal output.

## 🧩 Public Interface

| Command | Input | Output | Description |
|:---|:---|:---|:---|
| `mesh init` | --demo, --provider, --workers | Cluster provisioned | Interactive cluster provisioning wizard |
| `mesh status` | --demo, --compare, --roadmap | Cluster health display | View cluster nodes, agents, topology |
| `mesh destroy` | --cluster, --demo | Cluster torn down | Snapshot agents, stop, terminate nodes |
| `mesh compare` | (none) | Resource comparison table | Mesh vs Kubernetes resource comparison |
| `mesh roadmap` | (none) | Capability timeline | Platform feature roadmap |
| `mesh agent deploy` | name, --image, --cpu, --memory, --gpu | Agent deployed | Deploy a new AI agent container |
| `mesh agent list` | (none) | Agent table | List all running agents |
| `mesh agent stop` | name | Agent stopped | Stop agent with optional snapshot |
| `mesh agent snapshot` | name, --output | Snapshot created | Capture agent filesystem state |
| `mesh logs` | [job_name], --follow, --tail N, --alloc, --stderr | Job logs or job list | Stream/view Nomad job logs |
| `mesh ssh` | [node_name], --user | SSH session or node list | SSH into cluster nodes via Tailscale |

## 📦 Dependencies

- **Typer** - CLI framework with type hints
- **Questionary** - Interactive prompts for init wizard
- **Rich** - Terminal formatting (panels, tables, trees, progress)
- **src/infrastructure/provision_local_cluster** - Multipass provisioning for local clusters
- **src/infrastructure/provision_cloud_cluster/automation** - Pulumi Automation API for cloud clusters
- **src/infrastructure/boot_consul_nomad** - Boot script generation
- **src/infrastructure/provision_node/multipass** - Multipass VM adapter

## 🏗 Structure

```
src/cli/
├── CONTEXT.md          # This file
├── main.py             # Typer app entry point with all commands
├── commands/
│   ├── init_cmd.py     # Interactive cluster provisioning wizard
│   ├── agent.py        # Agent deploy/list/stop/snapshot commands
│   ├── status.py       # Cluster status display
│   ├── logs.py         # Stream/view Nomad job logs
│   ├── ssh.py          # SSH into cluster nodes
│   ├── helpers.py      # Shared CLI helpers (get_nomad_addr)
│   └── destroy.py      # Cluster teardown
└── ui/
    ├── panels.py       # Rich UI components (banners, panels, progress)
    └── themes.py       # Color constants and status icons
```

## 🧪 Test Coverage

- [ ] Unit tests for CLI command argument parsing
- [ ] Integration tests for init wizard flow
- [ ] Integration tests for agent lifecycle commands
- [ ] UI component rendering tests

## 📝 Design Decisions

- **Typer over Click** - Built-in help generation, type-safe arguments
- **Questionary over Inquirer** - Lightweight, Python-native interactive prompts
- **Rich over custom formatting** - Consistent terminal styling across all commands
- **Demo mode** - All commands support `--demo` flag for testing without real infrastructure
