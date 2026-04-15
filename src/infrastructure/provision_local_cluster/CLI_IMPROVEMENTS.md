# Feature: Improve CLI Usability

**Description:**
Enhance the local Multipass CLI (`cli.py`) with progress indicators, better error messages, and additional convenience commands for improved developer experience.

## Current State

**Existing CLI:** `src/infrastructure/provision_local_cluster/cli.py`

**Current Issues:**
- ❌ No progress indicators during VM provisioning (long-running operations appear frozen)
- ❌ Error messages are cryptic (raw subprocess output)
- ❌ No `cli.py logs <service>` command for viewing Nomad/Consul logs
- ❌ No `cli.py ssh <node>` shortcut for quick node access
- ❌ No clear feedback on what operation is happening

**Current Commands:**
```bash
python3 cli.py up    # Launches VMs, boots Nomad/Consul/Tailscale
python3 cli.py down  # Destroys VMs
python3 cli.py status  # Shows cluster status
```

## 🧩 Interface

### Enhanced Commands

| Command | Description | New Features |
|----------|-------------|--------------|
| `up` | Launch local cluster | ✅ Progress bars, better error messages |
| `down` | Destroy cluster | ✅ Confirmation prompt, progress feedback |
| `status` | Show cluster status | ✅ Formatted output, health indicators |
| `logs <service>` | View service logs | ✅ **NEW** - Follow Nomad/Consul/Tailscale logs |
| `ssh <node>` | SSH into node | ✅ **NEW** - Quick access to leader/worker |
| `restart <service>` | Restart service | ✅ **NEW** - Restart Nomad/Consul without destroy |

### Proposed Usage

```bash
# Launch with progress indicators
python3 cli.py up
# Shows: [████████░░] 80% Creating VMs...
#       [████████████] 100% Booting Nomad...

# View logs
python3 cli.py logs nomad
python3 cli.py logs consul
python3 cli.py logs tailscale

# SSH into node
python3 cli.py ssh leader
python3 cli.py ssh worker-0

# Show formatted status
python3 cli.py status
# Shows:
# Cluster: local-leader (Running)
# ├─ Leader: 192.168.64.2 (Nomad: ✓ Consul: ✓ Tailscale: ✓)
# └─ Workers: 1
#    └─ local-worker-0: 192.168.64.3 (Nomad: ✓ Consul: ✓ Tailscale: ✓)

# Destroy with confirmation
python3 cli.py down
# Shows: "This will destroy 2 VMs. Continue? [y/N]"
```

## 📦 Dependencies

- `rich` - Python library for beautiful terminal output (progress bars, tables, colors)
- Existing dependencies (Multipass, subprocess, etc.)

## 🧪 Tests

- [ ] Test: Progress bar displays during VM creation
- [ ] Test: Progress bar displays during cluster boot
- [ ] Test: Error messages are user-friendly (not raw subprocess output)
- [ ] Test: `logs` command shows correct service logs
- [ ] Test: `ssh` command connects to correct node
- [ ] Test: `status` command shows formatted output
- [ ] Test: Confirmation prompt works for `down` command
- [ ] Test: All existing functionality still works

## 📝 Design

### Problem Statement

**Current CLI Experience:**

1. **No Feedback During Long Operations:**
   - VM creation takes 30-60 seconds with no output
   - Users think CLI is frozen
   - No indication of progress or remaining time

2. **Cryptic Error Messages:**
   - Raw subprocess errors are confusing
   - No suggestions for common issues
   - Stack traces scare non-technical users

3. **Missing Convenience Commands:**
   - Must manually SSH: `multipass shell local-leader`
   - Must manually view logs: `multipass exec local-leader -- journalctl -u nomad`
   - No quick way to restart services

### Solution Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  Enhanced CLI with Rich Library                             │
│                                                              │
│  1. Progress Bars                                           │
│     - VM creation progress (multipass launch)               │
│     - Cluster boot progress (service startup)               │
│     - Overall operation progress                            │
│                                                              │
│  2. Formatted Output                                        │
│     - Status tables with colors                             │
│     - Health indicators (✓ ✗)                              │
│     - IP addresses and service status                       │
│                                                              │
│  3. Better Error Messages                                   │
│     - Parse common errors                                   │
│     - Provide actionable suggestions                        │
│     - Remove stack traces from user output                 │
│                                                              │
│  4. New Commands                                            │
│     - logs: View service logs with tail -f                 │
│     - ssh: Quick node access                                │
│     - restart: Restart services                             │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

### Implementation Strategy

#### Phase 1: Add Rich Dependency and Progress Bars

**Install Rich:**
```bash
pip install rich
```

**Update CLI with Progress Bars:**
```python
from rich.console import Console
from rich.progress import Progress, SpinnerColumn, TextColumn, BarColumn, TaskProgressColumn
from rich.table import Table
from rich.panel import Panel

console = Console()

def launch_vms(count, name_prefix):
    """Launch VMs with progress bar"""
    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        BarColumn(),
        TaskProgressColumn(),
        console=console
    ) as progress:
        task = progress.add_task("Creating VMs...", total=count)

        for i in range(count):
            vm_name = f"{name_prefix}-{i}"
            progress.update(task, description=f"Creating {vm_name}...")
            create_vm(vm_name)
            progress.advance(task)
```

#### Phase 2: Better Error Messages

**Error Parsing:**
```python
import subprocess
from rich.text import Text

def run_command(cmd):
    """Run command with user-friendly error messages"""
    try:
        result = subprocess.run(cmd, capture_output=True, text=True, check=True)
        return result.stdout
    except subprocess.CalledProcessError as e:
        # Parse error and provide helpful message
        error_msg = parse_common_error(e.stderr)
        console.print(Panel.fit(
            Text(error_msg, style="bold red"),
            title="Error",
            border_style="red"
        ))
        raise SystemExit(1)

def parse_common_error(stderr):
    """Parse common errors and provide suggestions"""
    if "Multipass is not installed" in stderr:
        return "Multipass not installed. Install with: brew install --cask multipass"
    elif "VM already exists" in stderr:
        return "VM already exists. Run 'python3 cli.py down' first to clean up."
    elif "Permission denied" in stderr:
        return "Permission denied. Try running with sudo or check file permissions."
    else:
        # Fallback to showing raw error
        return f"Command failed: {stderr}"
```

#### Phase 3: New Commands

**logs Command:**
```python
def cmd_logs(args):
    """View service logs"""
    service = args.service
    node = args.node or "local-leader"

    console.print(f"Following {service} logs on {node}...")
    console.print("Press Ctrl+C to stop")

    cmd = ["multipass", "exec", node, "--", "journalctl", "-u", service, "-f"]
    subprocess.run(cmd)
```

**ssh Command:**
```python
def cmd_ssh(args):
    """SSH into node"""
    node = args.node

    if node == "leader":
        node = "local-leader"
    elif node == "worker":
        node = "local-worker-0"

    console.print(f"Connecting to {node}...")
    subprocess.run(["multipass", "shell", node])
```

**restart Command:**
```python
def cmd_restart(args):
    """Restart service"""
    service = args.service
    node = args.node or "local-leader"

    with console.status(f"Restarting {service}..."):
        subprocess.run([
            "multipass", "exec", node, "--",
            "sudo", "systemctl", "restart", service
        ])

    console.print(f"✓ {service} restarted successfully")
```

#### Phase 4: Formatted Status Output

**Enhanced Status Command:**
```python
def cmd_status(args):
    """Show cluster status with rich table"""
    vms = get_multipass_vms()

    table = Table(title="Cluster Status", show_header=True, header_style="bold magenta")
    table.add_column("Node", style="cyan")
    table.add_column("IP", style="green")
    table.add_column("Nomad")
    table.add_column("Consul")
    table.add_column("Tailscale")

    for vm in vms:
        if vm['name'].startswith('local-leader'):
            node_type = "Leader"
        else:
            node_type = "Worker"

        # Check service status
        nomad_status = check_service(vm['name'], 'nomad')
        consul_status = check_service(vm['name'], 'consul')
        tailscale_status = check_service(vm['name'], 'tailscaled')

        table.add_row(
            node_type,
            vm['ipv4'][0],
            "✓" if nomad_status else "✗",
            "✓" if consul_status else "✗",
            "✓" if tailscale_status else "✗"
        )

    console.print(table)
```

### Error Message Examples

**Before (Cryptic):**
```
Traceback (most recent call last):
  File "cli.py", line 45, in launch_vms
    subprocess.run(cmd, check=True)
subprocess.CalledProcessError: Command '['multipass', 'launch', '-c', '2', '-n', 'local-leader', '-m', '2G']' returned non-zero exit status 1.
```

**After (User-Friendly):**
```
╭───────────────────────────────────────╮
│ Error                                  │
├───────────────────────────────────────┤
│ VM already exists.                     │
│ Run 'python3 cli.py down' first to    │
│ clean up.                              │
╰───────────────────────────────────────╯
```

### Progress Bar Examples

**VM Creation:**
```
Creating VMs...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 100%  ETA: 0:00:05

[████████████████████] 100% Creating local-leader...
[████████████░░░░░░░░░░]  50% Creating local-worker-0...
```

**Cluster Boot:**
```
Booting cluster...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 100% ETA: 0:02:30

[████████████░░░░░░░░]  60% Starting Nomad...
[████████████████░░░]  80% Starting Consul...
[████████████████████] 100% Starting Tailscale...
```

### CLI Argument Parser

**Updated Arguments:**
```python
parser = argparse.ArgumentParser(description="Local Cluster CLI")
subparsers = parser.add_subparsers(dest='command', help='Available commands')

# up command
up_parser = subparsers.add_parser('up', help='Launch cluster')
up_parser.add_argument('--count', type=int, default=1, help='Number of worker nodes')

# down command
down_parser = subparsers.add_parser('down', help='Destroy cluster')
down_parser.add_argument('--force', action='store_true', help='Skip confirmation')

# status command
status_parser = subparsers.add_parser('status', help='Show cluster status')

# logs command (NEW)
logs_parser = subparsers.add_parser('logs', help='View service logs')
logs_parser.add_argument('service', choices=['nomad', 'consul', 'tailscale'], help='Service to view logs for')
logs_parser.add_argument('--node', help='Node to view logs from (default: leader)')

# ssh command (NEW)
ssh_parser = subparsers.add_parser('ssh', help='SSH into node')
ssh_parser.add_argument('node', help='Node to connect to (leader, worker-0, etc.)')

# restart command (NEW)
restart_parser = subparsers.add_parser('restart', help='Restart service')
restart_parser.add_argument('service', choices=['nomad', 'consul', 'tailscale'], help='Service to restart')
restart_parser.add_argument('--node', help='Node to restart on (default: leader)')
```

## ⚠️ Limitations & Considerations

### Rich Library Dependency

**Consideration:** Adds external dependency

**Mitigation:**
- Rich is a popular, well-maintained library
- Small footprint (~100KB)
- Can be made optional (fallback to basic output)
- Add to requirements.txt

### Multipass Output Parsing

**Consideration:** Multipass output format may change

**Mitigation:**
- Use JSON output when available (`multipass ls --format json`)
- Handle parsing errors gracefully
- Fallback to showing raw output on parse failure

### Cross-Platform Compatibility

**Consideration:** Rich works best on modern terminals

**Mitigation:**
- Rich detects terminal capabilities automatically
- Falls back to basic output on dumb terminals
- No color output when redirected to file

## 🔒 Best Practices

### DO ✅

1. **Provide clear feedback:**
   ```python
   console.print("✓ VM created successfully")
   console.print("✓ Nomad started", style="bold green")
   ```

2. **Use colors sparingly:**
   ```python
   # Good: Highlight important information
   console.print("Cluster is [bold green]healthy[/bold green]")

   # Bad: Rainbow text everywhere
   console.print("[red]R[/red][yellow]A[/yellow][green]I[/green][cyan]N[/cyan][blue]B[/blue][magenta]O[/magenta]W")
   ```

3. **Handle Ctrl+C gracefully:**
   ```python
   try:
       run_long_operation()
   except KeyboardInterrupt:
       console.print("\n[yellow]Operation cancelled by user[/yellow]")
       sys.exit(0)
   ```

### DON'T ❌

1. **Don't overwhelm with animations:**
   ```python
   # Bad: Too many animations at once
   with Progress() as p1:
       with Progress() as p2:
           # Nested progress bars confuse users
   ```

2. **Don't hide important errors:**
   ```python
   # Bad: Silent failures
   try:
       create_vm()
   except:
       pass  # Error lost!

   # Good: Show user-friendly error
   try:
       create_vm()
   except Exception as e:
       console.print(f"[red]Error:[/red] {e}")
       raise
   ```

## 🎯 Success Criteria

- [ ] Progress bars show during VM creation and cluster boot
- [ ] Error messages are user-friendly and actionable
- [ ] `logs` command shows real-time service logs
- [ ] `ssh` command connects to specified node
- [ ] `status` command shows formatted table output
- [ ] All existing functionality still works
- [ ] Rich library added to dependencies
- [ ] Unit tests for new commands
- [ ] Documentation updated

## 📚 References

- [Rich Library Documentation](https://rich.readthedocs.io/)
- [Multipass Documentation](https://multipass.run/docs/)
- [Python argparse Documentation](https://docs.python.org/3/library/argparse.html)

## 📝 Implementation Checklist

- [x] Analyze current CLI implementation
- [x] Identify usability issues
- [ ] Create CONTEXT.md (this file)
- [ ] Add Rich library to dependencies
- [ ] Implement progress bars for VM creation
- [ ] Implement progress bars for cluster boot
- [ ] Implement error message parsing
- [ ] Implement `logs` command
- [ ] Implement `ssh` command
- [ ] Implement `restart` command
- [ ] Enhance `status` command with rich table
- [ ] Add confirmation prompt for `down` command
- [ ] Create unit tests for new commands
- [ ] Test on real Multipass cluster
- [ ] Update documentation with new commands
- [ ] Update tech-debt.md to mark TD-008 as complete

## 💡 Usage Examples

### Example 1: Launch Cluster with Progress

```bash
$ python3 cli.py up

Launching local cluster...
Creating VMs...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 100%  ETA: 0:00:30

[████████████████████] 100% Creating local-leader...
[████████████████████] 100% Creating local-worker-0...

Booting services...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 100%  ETA: 0:02:00

[████████████████████] 100% Starting Nomad...
[████████████████████] 100% Starting Consul...
[████████████████████] 100% Starting Tailscale...

✓ Cluster ready!
  Leader: 192.168.64.2
  Workers: 1
  ├─ local-worker-0: 192.168.64.3
```

### Example 2: View Logs

```bash
$ python3 cli.py logs nomad

Following nomad logs on local-leader...
Press Ctrl+C to stop

Jan 03 12:34:56 localhost nomad[1234]: Starting Nomad agent...
Jan 03 12:35:01 localhost nomad[1234]: Cluster leadership acquired
Jan 03 12:35:05 localhost nomad[1234]: Node registered successfully
```

### Example 3: SSH into Node

```bash
$ python3 cli.py ssh leader

Connecting to local-leader...
Welcome to Ubuntu 22.04 LTS (GNU/Linux 5.15.0 x86_64)
ubuntu@local-leader:~$
```

### Example 4: Formatted Status

```bash
$ python3 cli.py status

┏━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ Cluster Status                        ┃
┡━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┩
│ Node         IP            Nom Consul Tail │
├─────────────────────────────────────────┤
│ Leader       192.168.64.2  ✓   ✓      ✓    │
│ local-work... 192.168.64.3  ✓   ✓      ✓    │
└─────────────────────────────────────────┘
```

### Example 5: Destroy with Confirmation

```bash
$ python3 cli.py down

This will destroy 2 VMs. Continue? [y/N]: y

Destroying cluster...
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━ 100%  ETA: 0:00:10

[████████████████████] 100% Deleting local-leader...
[████████████████████] 100% Deleting local-worker-0...

✓ Cluster destroyed successfully
```
