# Dimension 3: Self-Hosted & Edge Compute Options

Research on self-hosted container/VM runtimes that could serve as Mesh substrates.

*Research conducted: July 2025*
*Sources: 25+ independent searches across official docs, GitHub repos, API references*

---

## 1. Executive Summary

- **Incus/LXD** is the strongest all-around candidate: native Go client (`github.com/lxc/incus/client`), comprehensive REST API, full CRUD + exec + filesystem export/import, supports both containers and VMs, moderate operational complexity[^1^][^2^].
- **Podman** is the simplest Docker-compatible option: native Go bindings (`github.com/containers/podman/v5/pkg/bindings`), REST API over Unix socket, full container lifecycle + exec + export/import, no K8s required, low complexity[^3^][^4^].
- **Firecracker** offers the best isolation and snapshot speed (~28ms restore) but requires significant DIY orchestration: custom rootfs/kernel management, networking setup, and VM lifecycle tracking[^5^][^6^].
- **Cloud Hypervisor** has OpenAPI 3.0 REST API and Go clients available (generated + hand-written), snapshot/restore support, but requires pause-before-snapshot and has version-unstable snapshot features[^7^][^8^].
- **gVisor** (`runsc`) is an OCI runtime, not a standalone API-driven substrate. Direct CLI commands exist (create, start, delete, exec, checkpoint, restore) but managing it programmatically requires either containerd integration or wrapping the CLI[^9^][^10^].
- **Kata Containers** is tightly coupled to containerd/CRI-O. It can run standalone via `ctr` without Kubernetes, but still requires containerd as an intermediary — no direct REST API[^11^][^12^].
- **sysbox** is a Docker/K8s runtime extension, not a standalone substrate. No direct API or Go client; managed through existing Docker/Podman/Kubernetes tooling[^13^][^14^].
- **wazero (pure Go Wasm runtime)** has extremely fast startup (~1-3ms) but is NOT a general-purpose compute substrate. It can only execute pre-compiled WebAssembly modules, not arbitrary binaries, shells, or native code[^15^][^16^].
- **Top Mesh substrate recommendations**: 1) Incus (best balance of features + Go-native), 2) Podman (simplest integration), 3) Firecracker (best isolation if accepting DIY complexity).

---

## 2. Detailed Findings by Technology

### 2.1 Incus / LXD

#### Overview
Incus is Canonical's fork of LXD, a system container and VM manager. It provides a daemon (`incusd`) with a REST API and a first-class Go client library.

#### Go API / Client
Claim: Incus has an official Go client library at `github.com/lxc/incus/client` with full instance lifecycle support[^1^].
Source: pkg.go.dev / Incus official docs
URL: https://pkg.go.dev/github.com/lxc/incus/client
Date: 2024-03-26 (last updated)
Excerpt: "Package incus implements a client for the Incus API... This package lets you connect to Incus daemons or SimpleStream image servers over a Unix socket or HTTPs. You can then interact with those remote servers, creating instances, images, moving them around..."
Context: Official Go package documentation with extensive API coverage.
Confidence: high

Claim: The Go client supports `CreateInstance`, `UpdateInstanceState`, `ExecInstance`, `DeleteInstance`, `GetInstance`, `CreateInstanceSnapshot`, `GetInstanceBackupFile`, `CreateInstanceFromBackup`, and SFTP file access[^1^].
Source: pkg.go.dev / Incus client API
URL: https://pkg.go.dev/github.com/lxc/incus/client
Date: 2024-03-26
Excerpt: "CreateInstance(instance api.InstancesPost) (op Operation, err error)... UpdateInstanceState(name string, state api.InstanceStatePut, ETag string) (op Operation, err error)... ExecInstance(instanceName string, exec api.InstanceExecPost, args *InstanceExecArgs) (op Operation, err error)... GetInstanceFileSFTP(instanceName string) (*sftp.Client, error)"
Context: Full InstanceServer interface listing all supported operations.
Confidence: high

#### REST API / OpenAPI Spec
Claim: Incus exposes a versioned REST API over Unix socket (local) and HTTPS (remote). API version is 1.0. The API is documented with Swagger/OpenAPI annotations[^2^].
Source: Incus REST API documentation
URL: https://linuxcontainers.org/incus/docs/main/rest-api/
Date: 2015-11-17 (ongoing)
Excerpt: "The Incus REST API is available over both a local unix+http and remote https API. Authentication for local users relies on group membership and access to the unix socket. For remote users, the default authentication method is TLS client certificates."
Context: Core API documentation page.
Confidence: high

#### Automation: Create/Start/Stop/Destroy/Exec
Claim: All lifecycle operations are available via REST API and Go client. Example code exists for creating an instance and starting it programmatically[^1^].
Source: pkg.go.dev example code
URL: https://pkg.go.dev/github.com/lxc/incus/client
Date: 2024-03-26
Excerpt:
```go
req := api.InstancesPost{
  Name: "my-container",
  Source: api.InstanceSource{
    Type:  "image",
    Alias: "my-image",
  },
  Type: "container"
}
op, err := c.CreateInstance(req)
// ...
reqState := api.InstanceStatePut{
  Action: "start",
  Timeout: -1,
}
op, err = c.UpdateInstanceState(name, reqState, "")
```
Context: Official package documentation includes runnable examples for instance creation, starting, and command execution.
Confidence: high

Claim: The `InstanceStatePut` struct supports actions: `start`, `stop`, `restart`, `freeze`, `thaw` (unfreeze)[^17^].
Source: Incus shared/api package
URL: https://pkg.go.dev/github.com/lxc/incus/shared/api
Date: 2024-03-26
Excerpt: "InstanceStatePut represents the modifiable fields of an instance's state... Action string `json:"action" yaml:"action"`"
Context: The `StatusCodeNames` map defines states including Starting, Stopping, Freezing, Frozen, Thawed.
Confidence: high

#### Filesystem Export/Import
Claim: Incus supports full instance export to compressed tar.gz backups and import from backup files, including snapshots[^18^].
Source: Incus backup documentation
URL: https://linuxcontainers.org/incus/docs/main/howto/instances_backup/
Date: Ongoing
Excerpt: "Use the following command to export an instance to a compressed file... `incus export <instance_name> [<file_path>]`... You can import the export file... `incus import <file_path> [<instance_name>]`"
Context: Official how-to guide for backing up instances.
Confidence: high

Claim: The Go client supports `GetInstanceBackupFile`, `CreateInstanceBackup`, `CreateInstanceFromBackup` for programmatic backup/restore[^1^].
Source: pkg.go.dev
URL: https://pkg.go.dev/github.com/lxc/incus/client
Date: 2024-03-26
Excerpt: "GetInstanceBackupFile(instanceName string, name string, req *BackupFileRequest) (resp *BackupFileResponse, err error)... CreateInstanceFromBackup(args InstanceBackupArgs) (op Operation, err error)"
Context: Full InstanceServer interface.
Confidence: high

#### Operational Complexity
- **Daemon**: Requires `incusd` daemon running (systemd service).
- **Kernel**: Works on standard Linux kernels; no special requirements beyond typical container support.
- **Networking**: Built-in bridge networking (incusbr0); can also use OVN, macvlan, etc.
- **Storage**: Supports multiple storage backends (zfs, btrfs, lvm, dir, ceph).
- **VMs**: Full QEMU-based VM support with virtio devices. VMs require KVM.
- **Clustering**: Supports clustering across multiple nodes.
- **Complexity**: Medium. One daemon to manage, but very feature-complete and stable.

#### Fits Mesh Constraints?
- **2GB VMs**: Yes — instance size limits configurable.
- **No K8s control plane**: Yes — completely independent of Kubernetes.
- **Go-native**: Yes — first-class Go client.
- **Verdict**: **STRONG FIT** — Best all-around substrate for Mesh.

---

### 2.2 Firecracker

#### Overview
Firecracker is AWS's microVM VMM written in Rust. It runs workloads in lightweight VMs with minimal overhead (<5 MiB per microVM, ~125ms cold boot)[^5^].

#### Go API / Client
Claim: Firecracker has an official Go SDK at `github.com/firecracker-microvm/firecracker-go-sdk` that provides a `Machine` type with lifecycle methods[^19^].
Source: Go Packages
URL: https://pkg.go.dev/github.com/firecracker-microvm/firecracker-go-sdk
Date: 2022-08-31
Excerpt: "Machine is the main object for manipulating Firecracker microVMs... Start actually start a Firecracker microVM... StopVMM stops the current VMM... Shutdown requests a clean shutdown..."
Context: Official SDK documentation.
Confidence: high

Claim: The Go SDK supports `CreateMachine`, `StartMachine`, `StopMachine`, `DeleteMachine` patterns as demonstrated in community tutorials[^20^].
Source: dev.to tutorial
URL: https://dev.to/lucasjacques/managing-firecracker-microvms-in-go-2ja0
Date: 2023-09-28
Excerpt:
```go
type VMMManagerInterface interface {
    CreateMachine(ctx context.Context, machineId string, config firecracker.Config) (*firecracker.Machine, error)
    StartMachine(ctx context.Context, machineId string) error
    StopMachine(ctx context.Context, machineId string) error
    GetMachine(machineId string) (*firecracker.Machine, error)
    DeleteMachine(ctx context.Context, machineId string) error
}
```
Context: Community tutorial showing how to build a manager around the SDK.
Confidence: high

#### REST API / OpenAPI Spec
Claim: Firecracker exposes a REST API over a Unix domain socket for VM lifecycle management[^5^].
Source: Firecracker official site
URL: https://firecracker-microvm.github.io/
Date: 2019-04-19
Excerpt: "You can control the Firecracker process via a RESTful API that enables common actions such as configuring the number of vCPUs or starting the machine."
Context: Official project homepage.
Confidence: high

#### Automation: Create/Start/Stop/Destroy/Exec
Claim: The Firecracker REST API supports PUT `/boot-source`, PUT `/drives/rootfs`, PUT `/actions` with `InstanceStart` to boot a VM[^21^].
Source: Medium tutorial
URL: https://betterprogramming.pub/getting-started-with-firecracker-a88495d656d9
Date: 2023-01-21
Excerpt:
```bash
curl --unix-socket /tmp/firecracker.socket -i \
    -X PUT 'http://localhost/boot-source' ...
curl --unix-socket /tmp/firecracker.socket -i \
    -X PUT 'http://localhost/drives/rootfs' ...
curl --unix-socket /tmp/firecracker.socket -i \
    -X PUT 'http://localhost/actions' \
    -d '{"action_type": "InstanceStart"}'
```
Context: Tutorial for running first microVM.
Confidence: high

Claim: There is no built-in "exec" API in Firecracker. Communication with the guest requires setting up vsock or serial console access. Most production users build a guest agent that listens on vsock[^22^].
Source: Medium article on code execution engine
URL: https://medium.com/@abhishekdadwal/building-a-production-grade-code-execution-engine-with-firecracker-microvms-21309dadeec9
Date: 2026-03-01
Excerpt: "Instead of scraping console output, I built a guest agent — a Go program running inside each VM as PID 1 that: Listens on vsock port 52, Receives code execution requests via JSON, Executes code using language runtimes..."
Context: Production experience report.
Confidence: high

#### Filesystem Export/Import
Claim: Firecracker uses raw block devices or ext4 files as rootfs. There is no built-in filesystem export/import API. Users typically create rootfs images from Docker containers via `docker export` + `mkfs.ext4` + extraction[^23^].
Source: Actuated blog
URL: https://actuated.com/blog/firecracker-container-lab
Date: 2023-09-05
Excerpt: "Here, a loopback file allocated with 5GB, then formatted as ext4... The script mounts the drive and then extracts the contents of the rootfs.tar file into it before unmounting the file."
Context: Lab guide for building Firecracker rootfs from containers.
Confidence: high

#### Snapshot/Restore (Key Advantage)
Claim: Firecracker supports VM snapshot/restore via REST API. A snapshot taken after guest agent initialization can be restored in ~28ms[^6^].
Source: Dev.to article
URL: https://dev.to/adwitiya/how-i-built-sandboxes-that-boot-in-28ms-using-firecracker-snapshots-i0k
Date: 2026-03-16
Excerpt: "When you restore from a snapshot, Firecracker doesn't boot a kernel. It doesn't run init... It memory-maps the snapshot file, loads the CPU state, and resumes execution from exactly where it left off."
Context: Engineering blog on sub-100ms sandbox boot.
Confidence: high

Claim: The official snapshot REST API uses `/snapshot/create` and `/snapshot/load` endpoints[^24^].
Source: Rust Utilities / Firecracker docs
URL: https://rustutils.com/tools/firecracker/
Date: Ongoing
Excerpt:
```bash
curl -X PUT --unix-socket /tmp/firecracker.socket \
  http://localhost/snapshot/create \
  -d '{"snapshot_type": "Full", "snapshot_path": "/tmp/snapshot_file", "mem_file_path": "/tmp/mem_file"}'
curl -X PUT --unix-socket /tmp/firecracker.socket \
  http://localhost/snapshot/load \
  -d '{"snapshot_path": "/tmp/snapshot_file", "mem_backend": {"backend_path": "/tmp/mem_file", "backend_type": "File"}, "resume_vm": true}'
```
Context: Firecracker feature reference.
Confidence: high

#### Operational Complexity
- **Kernel/Rootfs Management**: User must provide `vmlinux` kernel and `rootfs.ext4` image for every VM. No built-in image store.
- **Networking**: No built-in bridge/DHCP. Must configure TAP devices, IP assignment, and routing manually or via a wrapper.
- **Jailer**: Production security requires using the `jailer` companion tool for chroot/cgroup isolation.
- **Exec**: No built-in exec — requires guest agent development.
- **Complexity**: **HIGH**. Significant DIY orchestration required.

#### Fits Mesh Constraints?
- **2GB VMs**: Yes — microVMs can be configured with any memory size.
- **No K8s control plane**: Yes — completely standalone.
- **Go-native**: Yes — official Go SDK exists.
- **Verdict**: **CONDITIONAL FIT** — Best for high-isolation, fast-snapshot use cases if team can accept DIY complexity.

---

### 2.3 gVisor

#### Overview
gVisor is Google's userspace kernel sandbox. It intercepts syscalls in the "Sentry" rather than using hardware virtualization. It runs as an OCI runtime (`runsc`).

#### Go API / Client
Claim: gVisor does NOT expose a standalone REST API or Go client for direct VM/container lifecycle management. It is an OCI runtime that integrates with Docker/containerd[^9^].
Source: gVisor containerd quick start
URL: https://gvisor.dev/docs/user_guide/containerd/quick_start/
Date: Ongoing
Excerpt: "You can run containers in gVisor via ctr or crictl... `sudo ctr run --runtime io.containerd.runsc.v1 -t --rm docker.io/library/hello-world:latest hello-wrold`"
Context: Official containerd integration guide.
Confidence: high

Claim: The `runsc` binary supports direct OCI commands: `run`, `create`, `start`, `delete`, `state`, `kill`, `exec`, `list`, `checkpoint`, `restore`[^25^].
Source: gVisor checkpoint/restore docs
URL: https://gvisor.dev/docs/user_guide/checkpoint_restore/
Date: Ongoing
Excerpt: "runsc checkpoint --image-path=<path> <container id>... runsc create <container id>... runsc restore --image-path=<path> <container id>"
Context: Official checkpoint/restore documentation.
Confidence: high

Claim: The gVisor Go codebase exposes internal container management APIs under `gvisor.dev/gvisor/runsc/container` but these are internal packages, not a supported public API for external orchestrators[^26^].
Source: pkg.go.dev
URL: https://pkg.go.dev/gvisor.dev/gvisor/runsc/container
Date: Ongoing
Excerpt: "Container represents a containerized application... Start(conf *config.Config) error... Destroy() error... Execute(conf *config.Config, args *control.ExecArgs) (int32, error)... Checkpoint(...) error... Restore(...) error"
Context: Internal Go package documentation.
Confidence: medium

#### REST API / OpenAPI Spec
Claim: No REST API or OpenAPI spec exists. gVisor is strictly an OCI runtime that speaks to containerd/CRI-O via ttrpc/shim APIs[^9^].
Source: Multiple official docs
URL: https://gvisor.dev/docs/
Date: Ongoing
Excerpt: N/A — no REST API documented.
Context: gVisor architecture assumes an upstream orchestrator (Docker/containerd/K8s).
Confidence: high

#### Automation: Create/Start/Stop/Destroy/Exec
Claim: Lifecycle management is done via the `runsc` CLI following OCI conventions, or through Docker/containerd. Direct programmatic use requires shelling out to `runsc` or integrating with containerd's Go client[^9^][^25^].
Source: gVisor OCI quick start
URL: https://gvisor.dev/docs/user_guide/quick_start/oci/
Date: Ongoing
Excerpt: "sudo runsc run hello... sudo runsc spec -- /hello"
Context: OCI bundle workflow.
Confidence: high

#### Filesystem Export/Import
Claim: gVisor supports rootfs tar snapshots and filesystem snapshots via `runsc` flags[^27^][^28^].
Source: gVisor docs
URL: https://gvisor.dev/docs/user_guide/rootfs_snapshot/
Date: Ongoing
Excerpt: "Rootfs Tar Snapshots... Filesystem Snapshots"
Context: Two different snapshot mechanisms documented.
Confidence: medium

#### Operational Complexity
- **Integration**: Requires Docker or containerd to be useful in practice.
- **Syscall compatibility**: ~95% of Linux syscalls implemented; some applications fail[^29^].
- **Performance**: 10-30% overhead from syscall interception.
- **Platform**: Requires ptrace or KVM platform (KVM recommended for performance).
- **Complexity**: Medium when used via Docker; high if trying to use standalone.

#### Fits Mesh Constraints?
- **2GB VMs**: Not applicable — gVisor is not a VM runtime. Containers only.
- **No K8s control plane**: Possible via Docker/containerd, but still needs an orchestrator layer.
- **Go-native**: No public Go client; must use containerd Go client + shim.
- **Verdict**: **WEAK FIT** for Mesh — designed to be an OCI runtime under an orchestrator, not a standalone substrate.

---

### 2.4 Kata Containers

#### Overview
Kata Containers runs containers inside lightweight VMs using a VMM backend (Cloud Hypervisor, Firecracker, or QEMU). It is an OCI runtime shim.

#### Go API / Client
Claim: Kata Containers does NOT expose a standalone REST API or direct Go client. It implements the containerd Runtime V2 (Shim) API and is managed through containerd or Kubernetes[^11^].
Source: Kata Containers containerd docs
URL: https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/containerd-kata.md
Date: Ongoing
Excerpt: "The containerd provides not only the `ctr` command line tool, but also the CRI interface for Kubernetes and other CRI clients."
Context: Official containerd integration documentation.
Confidence: high

#### REST API / OpenAPI Spec
Claim: No REST API or OpenAPI spec. The runtime communicates via ttrpc with containerd's shim v2 API[^11^].
Source: Kata Containers docs
URL: https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/containerd-kata.md
Date: Ongoing
Excerpt: N/A
Context: Architecture assumes containerd as the control plane.
Confidence: high

#### Automation: Create/Start/Stop/Destroy/Exec
Claim: Containers can be launched via `ctr` without Kubernetes: `ctr run --runtime io.containerd.kata.v2 --runtime-config-path ...`[^11^].
Source: Kata Containers containerd docs
URL: https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/containerd-kata.md
Date: Ongoing
Excerpt: `$ sudo ctr run --cni --runtime io.containerd.kata.v2 --runtime-config-path $CONFIG_PATH -t --rm docker.io/library/busybox:latest hello sh`
Context: Standalone containerd usage guide.
Confidence: high

Claim: Historical `kata-runtime` v1.x had direct OCI commands (create, delete, exec, etc.) but v2.0+ moved to shim-v2 architecture and the standalone CLI was removed[^30^].
Source: GitHub issue
URL: https://github.com/kata-containers/kata-containers/issues/1133
Date: 2020-11-20
Excerpt: "Expected result: kata-runtime [create, delete, exec, kill, list, pause, ps, resume, run, spec, start, state, update, events, version...]"
Context: Issue reporting that v2.0.0 removed these OCI commands.
Confidence: high

#### Filesystem Export/Import
Claim: Kata Containers uses standard container images (OCI format). Import/export is handled by the upstream container runtime (containerd/Docker), not by Kata itself[^11^].
Source: Kata Containers docs
URL: https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/containerd-kata.md
Date: Ongoing
Excerpt: N/A
Context: Standard container image workflow.
Confidence: high

#### Operational Complexity
- **Containerd required**: Must install and configure containerd with CNI plugins.
- **VMM backends**: Multiple VMM options (QEMU, Cloud Hypervisor, Firecracker) with different tradeoffs.
- **Nested virtualization**: Requires KVM; may not work on all cloud instance types.
- **Boot time**: ~150-300ms depending on VMM[^31^].
- **Complexity**: Medium-High due to containerd + VMM + CNI stack.

#### Fits Mesh Constraints?
- **2GB VMs**: Yes — VM size configurable.
- **No K8s control plane**: Can use containerd directly, but still requires containerd daemon.
- **Go-native**: No direct Go client; must use containerd Go client.
- **Verdict**: **WEAK FIT** for Mesh — too many layers (containerd + shim + VMM) for a substrate that wants to avoid K8s complexity.

---

### 2.5 Cloud Hypervisor

#### Overview
Cloud Hypervisor is an Intel/Linux Foundation VMM written in Rust (~50k lines). It targets modern cloud workloads with virtio devices and supports CPU/memory hotplugging[^7^].

#### Go API / Client
Claim: Cloud Hypervisor has OpenAPI 3.0 compliant REST API, and Go clients have been generated from the spec. One such client is `github.com/ironcore-dev/cloud-hypervisor-provider/cloud-hypervisor/client`[^32^].
Source: pkg.go.dev
URL: https://pkg.go.dev/github.com/ironcore-dev/cloud-hypervisor-provider/cloud-hypervisor/client
Date: 2025-09-02
Excerpt: "Package client provides primitives to interact with the openapi HTTP API. Code generated by github.com/oapi-codegen/oapi-codegen/v2 version v2.4.1... NewCreateVMRequest... NewDeleteVMRequest... NewPutVmSnapshotRequest..."
Context: Auto-generated Go client from OpenAPI spec.
Confidence: high

Claim: Another community Go client exists at `github.com/afritzler/cloud-hypervisor-go` with a simplified interface[^33^].
Source: GitHub
URL: https://github.com/afritzler/cloud-hypervisor-go
Date: 2024-04-16
Excerpt:
```go
c, err := client.NewClient("http://localhost:8080")
res, err := c.CreateVM(context.TODO(), client.CreateVMJSONRequestBody{
    Cpus: &client.CpusConfig{MaxVcpus: 1},
    Memory: &client.MemoryConfig{Size: 1024 * 1024 * 1024},
})
```
Context: Community Go client project.
Confidence: high

#### REST API / OpenAPI Spec
Claim: Cloud Hypervisor exposes a REST API over Unix socket by default. The API is OpenAPI 3.0 compliant[^7^].
Source: Cloud Hypervisor API docs
URL: https://intelkevinputnam.github.io/cloud-hypervisor-docs-HTML/docs/api.html
Date: Ongoing
Excerpt: "The Cloud Hypervisor REST API triggers VM and VMM specific actions... The API is OpenAPI 3.0 compliant."
Context: Official API documentation.
Confidence: high

#### Automation: Create/Start/Stop/Destroy/Exec
Claim: REST endpoints include `/vm.create`, `/vm.boot`, `/vm.shutdown`, `/vm.delete`, `/vm.pause`, `/vm.resume`, `/vm.snapshot`, `/vm.restore`[^7^].
Source: Cloud Hypervisor API docs
URL: https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md
Date: Ongoing
Excerpt:
|Action|Endpoint|Request Body|Response Body|Prerequisites|
|-|-|-|-|-|
|Create the VM|`/vm.create`|`/schemas/VmConfig`|N/A|The VM is not created yet|
|Delete the VM|`/vm.delete`|N/A|N/A|N/A|
|Boot the VM|`/vm.boot`|N/A|N/A|The VM is created but not booted|
|Shut the VM down|`/vm.shutdown`|N/A|N/A|The VM is booted|
|Pause the VM|`/vm.pause`|N/A|N/A|The VM is booted|
|Resume the VM|`/vm.resume`|N/A|N/A|The VM is paused|
|Take a snapshot|`/vm.snapshot`|`/schemas/VmSnapshotConfig`|N/A|The VM is paused|
|Restore from snapshot|`/vm.restore`|`/schemas/RestoreConfig`|N/A|The VM is created but not booted|
Context: Official endpoint table.
Confidence: high

Claim: There is no built-in "exec" API. Like Firecracker, guest communication requires vsock, serial console, or a guest agent[^7^].
Source: Cloud Hypervisor docs
URL: https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md
Date: Ongoing
Excerpt: N/A — no exec endpoint in the REST API table.
Context: VM-level control only; guest interaction is out of scope.
Confidence: high

#### Filesystem Export/Import
Claim: Cloud Hypervisor uses disk image files (raw, qcow2). No built-in filesystem export/import. Storage management is external to the VMM[^7^].
Source: Cloud Hypervisor docs
URL: https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md
Date: Ongoing
Excerpt: N/A
Context: Disk paths are provided in `VmConfig` at creation time.
Confidence: high

#### Snapshot/Restore
Claim: Cloud Hypervisor supports snapshot/restore. The VM must be paused before snapshotting. Snapshots produce `config.json`, `memory-ranges`, and `state.json` files[^34^].
Source: Cloud Hypervisor snapshot docs
URL: https://intelkevinputnam.github.io/cloud-hypervisor-docs-HTML/docs/snapshot_restore.html
Date: Ongoing
Excerpt: "The goal for the snapshot/restore feature is to provide the user with the ability to take a snapshot of a previously paused virtual machine... The new virtual machine is restored in a paused state."
Context: Official snapshot/restore documentation.
Confidence: high

Claim: Snapshot stability is NOT guaranteed across versions[^35^].
Source: Northflank blog
URL: https://northflank.com/blog/guide-to-cloud-hypervisor
Date: 2026-01-30
Excerpt: "Snapshot/restore and live migration features exist but aren't guaranteed stable across versions. Production deployments should test these features thoroughly before relying on them."
Context: Production deployment guidance.
Confidence: high

#### Operational Complexity
- **Kernel/Rootfs**: User must provide kernel and disk image, similar to Firecracker.
- **Networking**: Requires manual TAP/bridge setup or integration with a network manager.
- **Hotplugging**: Supports CPU/memory hotplug — more flexible than Firecracker.
- **Complexity**: Medium-High. Easier than QEMU but still requires VM image management.

#### Fits Mesh Constraints?
- **2GB VMs**: Yes.
- **No K8s control plane**: Yes — standalone VMM.
- **Go-native**: Yes — generated Go client available.
- **Verdict**: **CONDITIONAL FIT** — Good if snapshot stability and guest agent requirements are acceptable. Firecracker has better snapshot maturity and speed.

---

### 2.6 Wasmtime + WASI P2 / wazero

#### Overview
WebAssembly (Wasm) with WASI is an emerging sandboxing technology. wazero is a pure Go WebAssembly runtime with zero CGO dependencies.

#### Go API / Client
Claim: wazero provides a native Go API at `github.com/tetratelabs/wazero` with `Runtime.Instantiate`, `ModuleConfig.WithFS`, and host function support[^15^].
Source: pkg.go.dev
URL: https://pkg.go.dev/github.com/tetratelabs/wazero
Date: 2025-12-18
Excerpt:
```go
type Runtime interface {
    Instantiate(ctx context.Context, source []byte) (api.Module, error)
    InstantiateWithConfig(ctx context.Context, source []byte, config ModuleConfig) (api.Module, error)
    NewHostModuleBuilder(moduleName string) HostModuleBuilder
    CompileModule(ctx context.Context, binary []byte) (CompiledModule, error)
    InstantiateModule(ctx context.Context, compiled CompiledModule, config ModuleConfig) (api.Module, error)
}
```
Context: Official wazero API documentation.
Confidence: high

#### REST API / OpenAPI Spec
Claim: No REST API. wazero is an in-process library, not a daemon or service[^15^].
Source: wazero docs
URL: https://pkg.go.dev/github.com/tetratelabs/wazero
Date: 2025-12-18
Excerpt: N/A
Context: Library-only runtime.
Confidence: high

#### Automation: Create/Start/Stop/Destroy/Exec
Claim: wazero can instantiate Wasm modules, but "execution" means calling exported functions, not running arbitrary programs. There is no concept of "exec" into a running Wasm sandbox[^15^].
Source: wazero docs
URL: https://pkg.go.dev/github.com/tetratelabs/wazero
Date: 2025-12-18
Excerpt: "Runtime allows embedding of WebAssembly modules... mod, _ := r.Instantiate(ctx, wasm)"
Context: Basic initialization example.
Confidence: high

#### Filesystem Export/Import
Claim: wazero supports filesystem mapping via `ModuleConfig.WithDirMount` and `WithFSMount`, but there is no "export" of a running sandbox's state[^15^].
Source: pkg.go.dev
URL: https://pkg.go.dev/github.com/tetratelabs/wazero
Date: 2025-12-18
Excerpt: "WithDirMount(dir, guestPath string) FSConfig... WithFSMount(fs fs.FS, guestPath string) FSConfig"
Context: FSConfig interface for mounting host directories into the guest.
Confidence: high

#### Critical Limitation: Not General-Purpose Compute
Claim: wazero (and WebAssembly in general) CANNOT run arbitrary native binaries, shell commands, or fork/exec subprocesses. It only executes WebAssembly modules compiled from supported languages (Rust, C, TinyGo, etc.)[^36^][^37^].
Source: xeiaso.net blog / Hacker News discussion
URL: https://xeiaso.net/blog/serde-precompiled-stupid/
Date: 2023-08-26
Excerpt: "There is a POSIX-like layer for WebAssembly programs called WASI that does bridge a lot of the gap, but it misses a lot of other things that would be needed for full compatibility including network socket and subprocess execution support."
Context: Discussion of WASI limitations for general-purpose computing.
Confidence: high

Claim: wazero does NOT support WASI Preview 2 / Component Model as of 2025[^37^].
Source: Hacker News / wazero GitHub
URL: https://news.ycombinator.com/item?id=43046922
Date: 2025-02-14
Excerpt: "I would like to interact with WASM components from a go binary, but it seems like wazero doesn't and won't support the component model anytime soon."
Context: Community report on wazero Component Model status.
Confidence: high

Claim: Wasmtime (Rust-based) supports WASI Preview 2 and Component Model, but requires CGO to embed in Go, adding build complexity[^38^].
Source: wasmRuntime.com comparison
URL: https://wasmruntime.com/en/compare/wasmtime-vs-wazero
Date: 2026-01-01
Excerpt: "WASI Preview 2: Wasmtime=✅, Wazero=🔜... Component Model: Wasmtime=✅, Wazero=⚪"
Context: Feature comparison table.
Confidence: medium

#### Operational Complexity
- **Compilation target**: All workloads must be compiled to WebAssembly.
- **Language support**: Rust, C/C++, TinyGo, AssemblyScript work well. Standard Go (gc) cannot compile to stable WASI yet.
- **No shell, no exec**: Cannot run `bash`, `python`, or arbitrary binaries.
- **Complexity**: Low for embedding, but **very high conceptual barrier** if trying to use as a general compute substrate.

#### Fits Mesh Constraints?
- **2GB VMs**: Memory limits configurable, but not VM-based.
- **No K8s control plane**: Yes — pure library.
- **Go-native**: Yes — pure Go, zero CGO.
- **General compute**: **NO** — Cannot run arbitrary code.
- **Verdict**: **NOT A FIT** for Mesh as a general-purpose substrate, unless Mesh only needs to run pre-compiled Wasm plugins.

---

### 2.7 sysbox (Nestybox)

#### Overview
Sysbox is a container runtime that enhances standard containers to run system-level workloads (systemd, Docker, Kubernetes) with stronger isolation via user namespaces.

#### Go API / Client
Claim: No direct Go client or REST API. Sysbox is an OCI runtime (`sysbox-runc`) that plugs into Docker/containerd/CRI-O[^13^].
Source: Nestybox GitHub
URL: https://github.com/nestybox/sysbox
Date: 2020-08-02
Excerpt: "Once installed, Sysbox works under the covers: you use Docker, Kubernetes, etc. to deploy containers with it... `docker run --runtime=sysbox-runc -it any_image`"
Context: Project README.
Confidence: high

#### REST API / OpenAPI Spec
Claim: None. Managed entirely through upstream orchestrator APIs (Docker, K8s)[^13^].
Source: Nestybox docs
URL: https://github.com/nestybox/sysbox
Date: 2020-08-02
Excerpt: N/A
Context: Runtime is invisible to users; Docker/K8s are the control planes.
Confidence: high

#### Automation: Create/Start/Stop/Destroy/Exec
Claim: All operations go through Docker or Kubernetes. Example: `docker run --runtime=sysbox-runc ...`, `kubectl apply -f` with `runtimeClassName: sysbox-runc`[^13^][^14^].
Source: Nestybox docs / blog
URL: https://github.com/nestybox/sysbox / https://blog.nestybox.com/2019/11/11/docker-sandbox.html
Date: 2020-08-02 / 2019-11-11
Excerpt: "$ docker run --runtime=sysbox-runc -it --hostname=syscont nestybox/alpine-docker:latest"
Context: Docker quick start examples.
Confidence: high

#### Filesystem Export/Import
Claim: Same as Docker/containerd — uses standard image and volume mechanisms. No special sysbox-specific export/import[^13^].
Source: Nestybox docs
URL: https://github.com/nestybox/sysbox
Date: 2020-08-02
Excerpt: N/A
Context: Standard container workflow.
Confidence: high

#### Operational Complexity
- **Kernel requirements**: Minimum Linux 5.15; shiftfs required on older kernels[^39^].
- **Host requirements**: 4 CPUs + 4GB RAM recommended[^40^].
- **Distro support**: Ubuntu, Debian, Fedora, Rocky, Amazon Linux, etc. (see compatibility matrix)[^39^].
- **User namespace**: Always enabled; maps root inside container to unprivileged UID on host.
- **Performance**: Near-native for CPU; disk I/O same as containers; network 17% hit for inner containers[^41^].
- **Complexity**: Low-Medium if Docker is already present; adds runtime installation and kernel compatibility checks.

#### Fits Mesh Constraints?
- **2GB VMs**: Supports container memory limits; not true VMs but "VM-like" containers.
- **No K8s control plane**: Can use Docker, but then requires Docker daemon.
- **Go-native**: No direct Go API.
- **Verdict**: **WEAK FIT** — Not a standalone substrate; requires an existing container orchestrator (Docker/K8s).

---

### 2.8 Podman

#### Overview
Podman is a Docker-compatible container engine that runs containers without a daemon (fork-exec model). It exposes a REST API via `podman system service`.

#### Go API / Client
Claim: Podman has official Go bindings at `github.com/containers/podman/v5/pkg/bindings/containers` with full lifecycle support[^3^].
Source: pkg.go.dev
URL: https://pkg.go.dev/github.com/containers/podman/v5/pkg/bindings/containers
Date: Ongoing
Excerpt: "ExecCreate creates a new exec session in an existing container... Start starts a non-running container... Stop stops a running container... Remove removes a container from local storage... Export creates a tarball of the given name or ID of a container."
Context: Official Podman Go bindings documentation.
Confidence: high

Claim: The Go bindings support `CreateWithSpec`, `Start`, `Stop`, `Remove`, `ExecCreate`, `ExecStart`, `Export`, `Checkpoint`, `Restore`, and more[^3^].
Source: Podman Go bindings tutorial
URL: https://podman.io/blogs/2020/08/10/podman-go-bindings.html
Date: 2020-08-10
Excerpt:
```go
s := specgen.NewSpecGenerator(rawImage, false)
r, err := containers.CreateWithSpec(connText, s)
err = containers.Start(connText, r.ID, nil)
```
Context: Official tutorial for Podman Go bindings.
Confidence: high

#### REST API / OpenAPI Spec
Claim: Podman exposes a REST API (libpod API) over Unix socket when `podman system service` is running. It is OpenAPI-documented and compatible with Docker API v1.40+ with extensions[^4^].
Source: OneUptime blog / Podman docs
URL: https://oneuptime.com/blog/post/2026-03-18-use-podman-rest-api-execute-commands-containers/view
Date: 2026-03-18
Excerpt:
```bash
curl -s --unix-socket /run/podman/podman.sock \
  -X POST \
  -H "Content-Type: application/json" \
  -d '{"AttachStdout": true, "AttachStderr": true, "Cmd": ["ls", "-la", "/"]}' \
  "http://localhost/v4.0.0/libpod/containers/test-container/exec"
```
Context: Practical guide to Podman REST API exec functionality.
Confidence: high

#### Automation: Create/Start/Stop/Destroy/Exec
Claim: Full container lifecycle is automatable via REST API and Go bindings. Exec requires two-step create-then-start pattern[^4^].
Source: OneUptime blog
URL: https://oneuptime.com/blog/post/2026-03-18-use-podman-rest-api-execute-commands-containers/view
Date: 2026-03-18
Excerpt: "Unlike simpler API operations, executing a command in a container requires two API calls: 1. Create an exec instance with the command and configuration. 2. Start the exec instance to run the command and receive output."
Context: REST API walkthrough.
Confidence: high

Claim: Go bindings include `Start`, `Stop`, `Restart`, `Pause`, `Unpause`, `Remove`, `Kill`, `Wait`, `Checkpoint`, `Restore`, `Export`[^3^].
Source: pkg.go.dev
URL: https://pkg.go.dev/github.com/containers/podman/v5/pkg/bindings/containers
Date: Ongoing
Excerpt: "Start starts a non-running container... Stop stops a running container... Remove removes a container... Export creates a tarball of the given name or ID of a container."
Context: Official package documentation.
Confidence: high

#### Filesystem Export/Import
Claim: Podman supports `export` (container filesystem as flat tar) and `import` (tar to image), as well as `save`/`load` for full image layers[^42^].
Source: Podman export docs
URL: https://docs.podman.io/en/latest/markdown/podman-export.1.html
Date: Ongoing
Excerpt: "podman export exports the filesystem of a container and saves it as a tarball on the local machine... podman export writes to STDOUT by default..."
Context: Official man page.
Confidence: high

Claim: The REST API supports exporting running or stopped containers and importing from tarballs, URLs, or stdin[^42^].
Source: OneUptime export/import guide
URL: https://oneuptime.com/blog/post/2026-03-18-export-import-podman-containers/view
Date: 2026-03-18
Excerpt: "podman export my-container -o /tmp/my-container.tar... podman import /tmp/my-container.tar my-app-snapshot:latest"
Context: Practical migration guide.
Confidence: high

#### Operational Complexity
- **Daemon-less**: Core engine runs without daemon; API service (`podman system service`) is optional.
- **Rootless**: Supports rootless containers out of the box.
- **Docker-compatible**: Mostly drop-in replacement for Docker CLI and API.
- **Complexity**: **LOW**. Easiest option if Docker compatibility is desired.

#### Fits Mesh Constraints?
- **2GB VMs**: Container memory limits supported; not VM-based.
- **No K8s control plane**: Yes — completely standalone.
- **Go-native**: Yes — official Go bindings.
- **Verdict**: **STRONG FIT** — Simplest Docker-compatible substrate. Best if Mesh doesn't need VM-level isolation.

---

## 3. Contradictions and Conflict Zones

### 3.1 gVisor: "Container runtime" vs "Sandbox substrate"
- **Conflict**: gVisor is marketed as a sandbox but is architecturally an OCI runtime shim. It cannot function as a standalone substrate without Docker/containerd/K8s above it.
- **Implication**: Mesh would need to wrap containerd + runsc, adding complexity rather than reducing it.
- **Resolution**: Use gVisor only if Mesh already plans to use containerd as its core orchestrator.

### 3.2 Kata Containers: "Standalone" vs "Shim"
- **Conflict**: Some sources suggest Kata can run via `ctr` without K8s, implying standalone capability. However, `ctr` is containerd's CLI — containerd is still required.
- **Implication**: Kata cannot be used as a direct substrate; it always needs containerd + CNI + VMM.
- **Resolution**: Not suitable for Mesh's "no K8s control plane" constraint unless containerd itself is acceptable.

### 3.3 Cloud Hypervisor vs Firecracker: "Production snapshot stability"
- **Conflict**: Firecracker snapshots are proven at AWS Lambda scale[^6^]. Cloud Hypervisor snapshots exist but are "not guaranteed stable across versions"[^35^].
- **Implication**: For Mesh snapshot/restore requirements, Firecracker is lower risk despite higher DIY overhead.
- **Resolution**: Prefer Firecracker if snapshots are critical; prefer Cloud Hypervisor if hotplug and broader device support are needed.

### 3.4 wazero: "Pure Go" vs "Cannot run arbitrary code"
- **Conflict**: wazero is the most Go-native option (zero CGO, ~1ms startup) but is fundamentally incapable of running general-purpose compute (no shell, no native binaries, no fork/exec).
- **Implication**: If Mesh needs to run user-provided Python, Bash, or compiled binaries, wazero is not an option.
- **Resolution**: wazero is viable ONLY if Mesh plugins are compiled to WebAssembly by the plugin authors.

### 3.5 sysbox: "VM-like containers" vs "No standalone API"
- **Conflict**: sysbox provides VM-like isolation with user namespaces but has no API of its own — it is purely a runtime under Docker/K8s.
- **Implication**: Mesh would need to manage Docker or K8s to use sysbox, defeating the "no K8s" goal.
- **Resolution**: sysbox is not a standalone substrate candidate.

---

## 4. Gaps in Available Information

1. **gVisor direct Go API**: The `runsc/container` package has programmatic APIs but they are internal and not documented for external use. No clear public Go client exists for direct lifecycle management without containerd.

2. **Kata Containers without containerd**: No evidence of a direct API or Go client that bypasses containerd. All documentation assumes containerd or CRI-O as the shim host.

3. **Firecracker guest agent patterns**: While many blog posts describe building guest agents, there is no standard, reusable guest agent library. Each team builds their own.

4. **Cloud Hypervisor Go client maturity**: The generated Go clients are relatively new. Limited documentation on error handling, reconnection, and production patterns.

5. **wazero WASI Preview 2 timeline**: No committed roadmap for Preview 2 / Component Model support in wazero. Community reports suggest it won't happen "soon"[^37^].

6. **Incus VM performance overhead**: Limited benchmarks comparing Incus VMs to raw QEMU or Firecracker for the 2GB VM use case.

7. **Podman rootless + sysbox integration**: Whether Podman can use sysbox-runc as a runtime is theoretically possible but not well documented.

---

## 5. Preliminary Recommendations

| Rank | Technology | Go Client | REST API | Create/Start/Stop/Destroy/Exec | FS Export/Import | Complexity | Mesh Fit | Confidence |
|------|-----------|-----------|----------|-------------------------------|------------------|------------|----------|------------|
| 1 | **Incus** | ✅ Native | ✅ Full | ✅ All + exec | ✅ Backup/restore | Medium | **STRONG** | High |
| 2 | **Podman** | ✅ Native | ✅ Full | ✅ All + exec | ✅ Export/import | Low | **STRONG** | High |
| 3 | **Firecracker** | ✅ Official SDK | ✅ Unix socket | ✅ (no exec) | ❌ DIY rootfs | High | **Conditional** | High |
| 4 | **Cloud Hypervisor** | ✅ Generated | ✅ OpenAPI 3.0 | ✅ (no exec) | ❌ External disks | Medium-High | **Conditional** | Medium |
| 5 | **gVisor** | ❌ Internal only | ❌ None | Via runsc CLI | ✅ Snapshots | Medium | **Weak** | High |
| 6 | **Kata Containers** | ❌ None direct | ❌ None | Via containerd | Via containerd | High | **Weak** | High |
| 7 | **sysbox** | ❌ None | ❌ None | Via Docker/K8s | Via Docker/K8s | Low-Medium | **Weak** | High |
| 8 | **wazero** | ✅ Native (library) | ❌ None | ❌ Only Wasm funcs | ❌ Only FS mounts | Low | **Not a fit** | High |

### 5.1 Recommendation Details

#### Primary: Incus (Recommendation: ADOPT)
- **Rationale**: Native Go client, comprehensive REST API, supports both containers and VMs, full lifecycle + exec + backup/restore, no K8s dependency, moderate complexity.
- **Caveat**: Requires running `incusd` daemon. VM support uses QEMU (heavier than Firecracker).
- **Best for**: General-purpose Mesh substrate requiring both containers and VMs with a single control plane.

#### Secondary: Podman (Recommendation: ADOPT if Docker-compat needed)
- **Rationale**: Lowest complexity, daemon-less core, official Go bindings, full REST API, excellent export/import support.
- **Caveat**: Container-only (no VMs). Process-level isolation (shared kernel).
- **Best for**: Mesh workloads that accept container isolation and want simplest deployment.

#### Tertiary: Firecracker (Recommendation: ADOPT if isolation + speed critical)
- **Rationale**: Best-in-class snapshot restore (~28ms), hardware-level isolation, proven at AWS scale.
- **Caveat**: High DIY complexity — must manage rootfs, kernel, networking, and build guest agent for exec.
- **Best for**: High-security untrusted code execution with fast warm-start requirements.

#### Reject: wazero for general compute (Recommendation: AVOID as substrate)
- **Rationale**: Cannot run arbitrary binaries or shells. Only pre-compiled Wasm modules.
- **Exception**: If Mesh plugins are authored in Rust/C/TinyGo and compiled to Wasm, wazero becomes viable as an ultra-fast plugin runtime.

#### Reject: Kata Containers, sysbox, gVisor as standalone substrates (Recommendation: AVOID)
- **Rationale**: All require upstream orchestrators (containerd, Docker, K8s). They are runtimes, not control planes. Using them would force Mesh to manage Docker/containerd, which contradicts the "no K8s control plane" spirit.

---

## 6. Citation Index

[^1^]: https://pkg.go.dev/github.com/lxc/incus/client — Incus official Go client package
[^2^]: https://linuxcontainers.org/incus/docs/main/rest-api/ — Incus REST API documentation
[^3^]: https://pkg.go.dev/github.com/containers/podman/v5/pkg/bindings/containers — Podman Go bindings
[^4^]: https://oneuptime.com/blog/post/2026-03-18-use-podman-rest-api-execute-commands-containers/view — Podman REST API exec guide
[^5^]: https://firecracker-microvm.github.io/ — Firecracker official site
[^6^]: https://dev.to/adwitiya/how-i-built-sandboxes-that-boot-in-28ms-using-firecracker-snapshots-i0k — Firecracker snapshot performance
[^7^]: https://intelkevinputnam.github.io/cloud-hypervisor-docs-HTML/docs/api.html — Cloud Hypervisor REST API docs
[^8^]: https://github.com/cloud-hypervisor/cloud-hypervisor/blob/main/docs/api.md — Cloud Hypervisor API endpoint table
[^9^]: https://gvisor.dev/docs/user_guide/containerd/quick_start/ — gVisor containerd quick start
[^10^]: https://gvisor.dev/docs/user_guide/quick_start/oci/ — gVisor OCI quick start
[^11^]: https://github.com/kata-containers/kata-containers/blob/main/docs/how-to/containerd-kata.md — Kata Containers + containerd docs
[^12^]: https://northflank.com/blog/what-are-kata-containers — Kata Containers architecture overview
[^13^]: https://github.com/nestybox/sysbox — sysbox GitHub repository
[^14^]: https://blog.nestybox.com/2019/11/11/docker-sandbox.html — sysbox Docker sandbox blog
[^15^]: https://pkg.go.dev/github.com/tetratelabs/wazero — wazero Go package
[^16^]: https://wazero.io/specs/ — wazero specifications page
[^17^]: https://pkg.go.dev/github.com/lxc/incus/shared/api — Incus shared API types
[^18^]: https://linuxcontainers.org/incus/docs/main/howto/instances_backup/ — Incus backup how-to
[^19^]: https://pkg.go.dev/github.com/firecracker-microvm/firecracker-go-sdk — Firecracker Go SDK
[^20^]: https://dev.to/lucasjacques/managing-firecracker-microvms-in-go-2ja0 — Firecracker Go tutorial
[^21^]: https://betterprogramming.pub/getting-started-with-firecracker-a88495d656d9 — Firecracker getting started
[^22^]: https://medium.com/@abhishekdadwal/building-a-production-grade-code-execution-engine-with-firecracker-microvms-21309dadeec9 — Firecracker guest agent pattern
[^23^]: https://actuated.com/blog/firecracker-container-lab — Firecracker rootfs from container
[^24^]: https://rustutils.com/tools/firecracker/ — Firecracker snapshot API reference
[^25^]: https://gvisor.dev/docs/user_guide/checkpoint_restore/ — gVisor checkpoint/restore docs
[^26^]: https://pkg.go.dev/gvisor.dev/gvisor/runsc/container — gVisor internal container package
[^27^]: https://gvisor.dev/docs/user_guide/rootfs_snapshot/ — gVisor rootfs snapshots
[^28^]: https://gvisor.dev/docs/user_guide/fs_snapshot/ — gVisor filesystem snapshots
[^29^]: https://northflank.com/blog/your-containers-arent-isolated-heres-why-thats-a-problem-micro-vms-vmms-and-container-isolation — gVisor syscall compatibility note
[^30^]: https://github.com/kata-containers/kata-containers/issues/1133 — Kata runtime v2 missing OCI commands
[^31^]: https://northflank.com/blog/what-are-kata-containers — Kata boot time benchmarks
[^32^]: https://pkg.go.dev/github.com/ironcore-dev/cloud-hypervisor-provider/cloud-hypervisor/client — Cloud Hypervisor generated Go client
[^33^]: https://github.com/afritzler/cloud-hypervisor-go — Cloud Hypervisor community Go client
[^34^]: https://intelkevinputnam.github.io/cloud-hypervisor-docs-HTML/docs/snapshot_restore.html — Cloud Hypervisor snapshot/restore docs
[^35^]: https://northflank.com/blog/guide-to-cloud-hypervisor — Cloud Hypervisor production notes
[^36^]: https://xeiaso.net/blog/serde-precompiled-stupid/ — WASI subprocess limitation discussion
[^37^]: https://news.ycombinator.com/item?id=43046922 — wazero Component Model status
[^38^]: https://wasmruntime.com/en/compare/wasmtime-vs-wazero — Wasmtime vs wazero comparison
[^39^]: https://github.com/nestybox/sysbox/blob/master/docs/distro-compat.md — sysbox distro compatibility
[^40^]: https://learn.arm.com/install-guides/sysbox/ — sysbox install requirements
[^41^]: https://blog.nestybox.com/2020/09/23/perf-comparison.html — sysbox performance analysis
[^42^]: https://docs.podman.io/en/latest/markdown/podman-export.1.html — Podman export man page
