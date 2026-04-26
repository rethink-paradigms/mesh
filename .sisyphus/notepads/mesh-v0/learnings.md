# Learnings — mesh-v0

## 2026-04-23 Session Start
- Greenfield Go project — zero .go files exist yet
- Module path: github.com/rethink-paradigms/mesh
- Go 1.25.x available on darwin/arm64 (dev), target linux/amd64 (prod)
- Old .github/workflows/ from dead "lightweight K8s" era must be replaced
- discovery/ folder is out of scope — must NOT modify
- v0 deliberately deviates from D2 (no OCI images) and D5 (CLI only, no MCP)

## Task 1: Go Module Init + Project Scaffold (2026-04-23)

- Module path: `github.com/rethink-paradigms/mesh`, Go 1.24.x
- Old workflows (publish.yml, reusable-docker-build.yml, reusable-nomad-deploy.yml, test.yml, docs.yml, README.md) were from dead K8s framing — all deleted
- `.gitignore` already had `dist/` from Python build section; added Go-specific `/mesh`, `/coverage.out`, `*.exe`, `/dist/` at the end
- No external deps imported — pure stdlib `main.go`
- CI uses `golangci/golangci-lint-action@v6` with `actions/setup-go@v5` and Go 1.24
- `go test ./...` reports `[no test files]` with exit 0 — expected for foundation task
- Evidence saved to `.sisyphus/evidence/task-1-build.txt` and `task-1-test.txt`

## Task 2: Hash Round-Trip Test + Snapshot Pipeline Core

### Key Findings

- **io.Pipe + io.MultiWriter for streaming hash**: To hash the *compressed* output (not the raw tar), use `io.MultiWriter(outFile, hasher)` as the zstd writer's output. Do NOT use `io.TeeReader` on the pipeReader side — that hashes the raw tar before compression.
  - Correct: `pipeReader → zstd.NewWriter(io.MultiWriter(outFile, hasher))`
  - Wrong: `io.TeeReader(pipeReader, hasher) → zstd.NewWriter(outFile)`
- **Deterministic tar requires manual sorted walk**: `filepath.WalkDir` does NOT guarantee sorted order. Must use `os.ReadDir` + `sort.Slice` for deterministic entry ordering.
- **Don't write root dir header**: When snapshotting a workdir, only write entries *inside* the root, not the root itself. This avoids `.` entries and keeps tar contents relative.
- **Tar header cleanup for reproducibility**: Zero out Uid, Gid, Uname, Gname, Devmajor, Devminor in every header for cross-machine determinism.
- **klauspost/compress/zstd API**: Pure Go, no CGo. `zstd.NewWriter(w)` / `zstd.NewReader(r)`. v1.18.5 as of this task.
- **`os.WriteFile` may not respect perms on create**: Use explicit `os.Chmod` after `os.WriteFile` when specific permissions matter (test helpers do `mustWriteFile` with chmod).
- **`go vet` clean** on all code. Race detector: 0 races on all 5 tests.

### Files Created/Modified
- `go.mod`: Added `github.com/klauspost/compress v1.18.5`
- `internal/snapshot/snapshot.go`: `CreateSnapshot` with streaming tar→zstd→sha256 pipeline
- `internal/snapshot/snapshot_test.go`: 5 tests (HashRoundTrip, DeterministicHash, EmptyDirectory, PermissionPreservation, SymlinkPreservation)

## Task 3: TOML Config Parsing + Validation

### Key Findings
- **BurntSushi/toml v1.6.0**: `toml.DecodeFile(path, &struct)` — clean API, TOML tags on struct fields, no init required
- **Defaults pattern**: Apply defaults in a separate `applyDefaults()` before validation so defaults get validated too. Check zero-value (`== ""` or `== 0`) — TOML decoder leaves unset fields at zero
- **SSH key permissions**: `os.Stat().Mode().Perm()` returns `os.FileMode` perms. Compare `> 0600` to catch insecure keys. Skip on Windows via `runtime.GOOS`
- **Test hermeticity**: Use `t.TempDir()` (auto-cleaned), `t.Setenv()` (auto-restored), embed temp paths in TOML string literals with backtick concatenation
- **`go vet` clean**, 13 tests, 0 races
- **Load() contract**: Parse → applyDefaults → Validate, all in one call. Caller gets fully validated config or descriptive error

### Files Created/Modified
- `internal/config/config.go`: Config/Machine/Agent structs, Load, Validate, DefaultPath, applyDefaults, validateSSHKey, ExpandPath
- `internal/config/config_test.go`: 13 tests covering valid config, empty names, duplicates, machine refs, SSH key existence/perms, env override, defaults
- `go.mod`: Added `github.com/BurntSushi/toml v1.6.0`

## Task 4: Snapshot Command Full Workflow

### Key Findings
- **Testability via optional cacheDir parameter**: `Run(ctx, cfg, agentName, cacheDir)` accepts an empty `cacheDir` to use the default `~/.mesh/snapshots/{agent}/`, or a test-provided temp dir. Avoids environment variable hacks.
- **Timestamp format `20060102-150405` has 1-second granularity**: Two snapshots in the same second get the same filename and overwrite. Test with `time.Sleep(1100ms)` or pre-seed fake snapshot files.
- **Pruning uses sort-by-name**: Since filenames encode timestamps as `agent-YYYYMMDD-HHMMSS.tar.zst`, lexicographic sort = chronological sort. No need to parse timestamps from filenames.
- **Pruning deletes both `.tar.zst` and `.sha256` sidecars**: Must clean up sidecars when pruning, not just the tarball.
- **Prune errors are non-fatal**: Snapshot creation succeeds even if pruning fails — better to keep the snapshot than lose it due to cleanup issues.
- **`os.ReadDir` checks readability**: After `os.Stat` confirms directory exists, `os.ReadDir` catches permission-denied on the workdir.
- **`config.ExpandPath` handles `~/` expansion**: Used before checking workdir existence.
- **12 tests total (5 old + 7 new), 0 races**, `go vet` clean.

### Files Modified
- `internal/snapshot/snapshot.go`: Added `ResolveAgent`, `SnapshotCacheDir`, `Run`, `pruneSnapshots`
- `internal/snapshot/snapshot_test.go`: Added `TestResolveAgentFound`, `TestResolveAgentNotFound`, `TestRunCreatesSnapshot`, `TestRunAgentNotFound`, `TestRunNonExistentWorkdir`, `TestRunUnreadableWorkdir`, `TestRunMaxSnapshots`, `TestTimestampedFilename`

## Task 7: Manifest Package

### Key Findings
- **Pure stdlib, no new deps**: `encoding/json`, `os`, `path/filepath`, `strings`, `time` — nothing beyond Go stdlib needed
- **json.MarshalIndent with 2-space indent**: Human-readable sidecar files. `json.MarshalIndent(m, "", "  ")`
- **time.Time serializes as RFC3339 by default**: No custom marshaling needed — `time.Time.MarshalJSON()` uses RFC3339Nano
- **ManifestPath uses strings.TrimSuffix**: For `.tar.zst` → `.json` derivation. Must use `HasSuffix` check before trim since the extension has a dot prefix
- **Write creates parent dirs**: `os.MkdirAll(filepath.Dir(path), 0755)` before `os.WriteFile` ensures nested paths work
- **8 tests, 0 races**, all hermetic via `t.TempDir()`
- **Added bonus test `TestWriteCreatesParentDirs`**: Validates nested directory creation behavior

### Files Created
- `internal/manifest/manifest.go`: Manifest struct, Write, Read, ManifestPath
- `internal/manifest/manifest_test.go`: 8 tests (RoundTrip, AllFields, MalformedJSON, ManifestPath, ManifestPathNoExt, TimestampFormat, EmptyManifest, WriteCreatesParentDirs)

## Task 5: Restore Command

### Key Findings
- **Snapshot format consumed**: `{name}.tar.zst` + `{name}.tar.zst.sha256` (hex digest + newline). The hash is of the compressed .tar.zst, not the raw tar.
- **Atomic restore via temp dir + rename**: `os.MkdirTemp(parentDir, ...)` creates the temp in the same filesystem as target's parent. `os.Rename` is atomic on same filesystem. If target exists, `os.RemoveAll` it first, then rename.
- **EXDEV fallback**: `os.Rename` fails with cross-device error when temp and target are on different filesystems. Fall back to `recursiveCopy + os.RemoveAll`. Detect via error string matching ("invalid cross-device link") since `syscall.EXDEV` requires importing syscall.
- **Transactional cleanup**: Use a `cleanup` bool flag + deferred function. Set `cleanup = false` only after successful rename. This is cleaner than named return + deferred `os.RemoveAll`.
- **Tar extraction handles 3 types**: `tar.TypeReg` (files), `tar.TypeDir` (dirs), `tar.TypeSymlink` (symlinks). Must `os.Chmod` after writing since `os.OpenFile` may not respect requested perms (umask).
- **Path traversal prevention**: After `filepath.Join(dstDir, header.Name)`, verify the result starts with `filepath.Clean(dstDir) + os.PathSeparator`. `strings.HasPrefix` (not `filepath.HasPrefix` which doesn't exist).
- **Writability probe**: `os.CreateTemp(dir, ".mesh-write-test-*")` + cleanup is more reliable than checking permission bits (ACLs, macOS sandboxing).
- **8 tests, 0 races**, `go vet` clean. Full project `go test -race ./...` passes.

### Files Created
- `internal/restore/restore.go`: `VerifyHash`, `Restore`, `extractTar`, `checkWritableDir`, `isEXDEV`, `recursiveCopy`, `copyFile`
- `internal/restore/restore_test.go`: 8 tests (RestoreRoundTrip, HashMismatch, AtomicRename, CleanupOnFailure, RestoreNonExistentSnapshot, RestoreToNonWritableDir, VerifyHashCorrect, VerifyHashMismatch)

## Task 6: Agent Process Management

### Key Findings
- **pgrep -f matches command line, not working directory**: `cmd.Dir` sets child's working directory but does NOT appear in `/proc/PID/cmdline`. Must embed the workdir path in the command string itself (e.g., `cd /path && sleep 30`) for `pgrep -f <path>` to find it.
- **Zombie processes and Signal(0)**: After SIGTERM, child process becomes zombie if parent hasn't called Wait(). `os.FindProcess(pid).Signal(syscall.Signal(0))` returns nil for zombies (process entry still exists in kernel table). Fix in tests: `go cmd.Wait()` immediately after `cmd.Start()` to reap zombies promptly.
- **Stop polling interval**: 100ms poll with Signal(0) is reliable for detecting process exit. SIGKILL fallback after timeout handles stuck/unresponsive processes.
- **Setpgid for process groups**: `syscall.SysProcAttr{Setpgid: true}` creates new process group, useful for clean process tree management.
- **Test hermeticity**: Use `os.MkdirTemp("", "mesh-agent-test-"+t.Name())` for unique patterns. Embed unique temp dir path in command string (`cd <dir> && sleep N`). Cleanup with `pkill -f <dir>` via `t.Cleanup()`.
- **strconv.Quote for shell escaping**: Use `strconv.Quote(dir)` when embedding paths in shell commands to handle spaces/special chars.
- **8 tests, 0 races**, `go vet` clean. Full project `go test -race ./...` passes.

### Files Created
- `internal/agent/agent.go`: FindPID, Stop, Start, StartBackground, IsRunning, ReadPIDFile
- `internal/agent/agent_test.go`: 8 tests (FindPID, FindPIDNoMatch, FindPIDMultipleMatches, Stop, StopTimeout, Start, IsRunning, ReadPIDFile with subtests)

## Task 8: Clone Command

### Key Findings
- **Clone is the first integration point** between multiple packages: snapshot + restore + transport + config
- **Local clone target = workdir + "-clone" suffix**: When target machine is empty or "local", restore to `{workdir}-clone`
- **findLatestSnapshot uses prefix+suffix filtering**: Must filter by `agentName + "-"` prefix AND `.tar.zst` suffix to avoid matching other agents' snapshots or `.sha256` sidecars
- **Lexicographic sort = chronological**: Since filenames encode timestamps as `{name}-{YYYYMMDD-HHMMSS}.tar.zst`, `sort.Strings` gives chronological order — last element is newest
- **Transport package is thin os/exec wrappers**: `SCP()` and `ExecSSH()` are pure `exec.Command` wrappers with `-o StrictHostKeyChecking=no` for v0
- **SCP port flag is -P (uppercase)**, SSH port flag is -p (lowercase) — easy to mix up
- **Remote restore uses zstd+tar on command line**: Remote machine may not have mesh installed, so we decompress via `zstd -d | tar xf -` over SSH
- **4 hermetic tests (no network, no SSH)**: LocalCloneRoundTrip, NonExistentTarget, NonExistentAgent, LatestSnapshotDetection
- **0 races, go vet clean**, all packages pass

### Files Created
- `internal/transport/ssh.go`: SCP, ExecSSH wrappers
- `internal/clone/clone.go`: Run, findLatestSnapshot, isLocalTarget, resolveMachine, localClone, remoteClone
- `internal/clone/clone_test.go`: 4 tests

## Task 9: CLI Wire-Up — Cobra Subcommands

### Key Findings
- **spf13/cobra v1.10.2**: Clean API for building CLI subcommands. `cobra.Command` with `Use`, `Short`, `Long`, `RunE`, `Args` fields.
- **SilenceUsage + SilenceErrors pattern**: Set both to true on root, handle error printing + exit codes in main(). This prevents double-printing of errors.
- **Exit code differentiation**: Check error message for "required"/"accepts"/"arg(s)" to distinguish usage errors (exit 2) from runtime errors (exit 1). Not ideal but Cobra doesn't expose error types.
- **MarkFlagRequired**: `cobra.CheckErr(cmd.MarkFlagRequired("target"))` makes cobra enforce the flag and print a usage error if missing.
- **debug.ReadBuildInfo for version**: `runtime/debug.ReadBuildInfo()` gives VCS revision + dirty state. Falls back to `v0.0.0-dev`.
- **loadConfig helper**: Centralizes --config flag reading + DefaultPath() fallback. All commands call it.
- **findLatestSnapshot helper**: Reused by restore command. Same logic as clone package's internal version but accessible from CLI layer.
- **parseHookTimeout duplicated**: Same logic exists in snapshot package as unexported function. CLI has its own copy since snapshot's is unexported. Acceptable for now.
- **Status/list/inspect/prune are stubs**: Basic working implementations that will be enhanced in Task 10. List iterates snapshot dirs, status checks IsRunning, inspect reads manifest JSON, prune sorts and deletes oldest.
- **go vet clean**, all existing tests pass with -race, no regressions.

### Files Modified
- `cmd/mesh/main.go`: Complete rewrite from 11-line stub to full Cobra CLI with 7 subcommands
- `go.mod`: Added `github.com/spf13/cobra v1.10.2`, `github.com/spf13/pflag v1.0.9`, `github.com/inconshreveable/mousetrap v1.1.0`

## Task 10: Operational Commands — status, list, inspect, prune

### Key Findings
- **Task 9 stubs were already functional but minimal**: status showed running+count, list showed paths, inspect dumped JSON, prune deleted .tar.zst+.sha256. Task 10 enhanced all four.
- **t.Setenv("HOME", tmpDir) for hermetic CLI tests**: Redirects `os.UserHomeDir()` which is used by `snapshot.SnapshotCacheDir()` and the list command's `~/.mesh/snapshots/` resolution. Auto-restored by testing framework.
- **Cobra command testing pattern**: Create command via `newXxxCmd()`, set stdout to `bytes.Buffer`, use `cmd.SetArgs()` to pass args+flags, call `cmd.Execute()`. No need for full `newRootCmd()`.
- **inspect accepts both paths and agent names**: When stat fails on the arg, it tries to resolve it as an agent name by looking up `~/.mesh/snapshots/{arg}/` and finding the latest snapshot. This dual resolution was in the spec.
- **Prune must delete 3 sidecars**: .tar.zst + .sha256 + .json. The old stub only deleted the first two. Added `manifest.ManifestPath(tarPath)` to derive the .json path.
- **parseTimestampFromFilename parses {agent}-{YYYYMMDD-HHMMSS}.tar.zst**: Uses `strings.SplitN(trimmed, "-", 2)` since the timestamp itself contains a dash. Validates 8-digit date + 6-digit time before parsing with `time.Parse`.
- **humanSize uses float division for KB/MB/GB**: `fmt.Sprintf("%.1f KB", float64(bytes)/1024.0)` — avoids integer truncation. Shows bytes directly for < 1KB.
- **17 new tests, 0 races**, `go vet` clean, all packages pass.

### Files Modified/Created
- `cmd/mesh/main.go`: Enhanced status (timestamp + cache size), list (timestamp + size + machine), inspect (plain text), prune (.json cleanup). Added parseTimestampFromFilename, humanSize, findLatestInDir helpers.
- `cmd/mesh/main_test.go`: 17 tests covering all four commands + helper functions

## Task 13: README + Install Instructions

### Key Findings
- **No existing README.md**: Project had none, clean write
- **LICENSE file is MIT**: Copyright 2026 Mesh Platform Contributors
- **1,439 words** in final README
- **Go version is 1.25.5** (from go.mod `go 1.25.5`)
- **7 commands verified against source**: snapshot, restore, clone, status, list, inspect, prune — all usage strings and flags match `cmd/mesh/main.go`
- **Config schema verified against `internal/config/config.go`**: Machine (6 fields) and Agent (10 fields) with TOML tags, defaults, and validation rules all accurate
- **Manifest struct has 8 fields**: agent_name, timestamp, source_machine, source_workdir, start_cmd, stop_timeout, checksum, size — verified against `internal/manifest/manifest.go`
- **Snapshot produces 3 files**: .tar.zst + .sha256 + .json sidecar — confirmed in snapshot.go and manifest.go
- **Transport uses os/exec ssh/scp**: Confirmed in clone package, no Go SSH libraries
- **No badges, no marketing language**: Technical, concise tone as specified

### File Created
- `README.md`: 1,439 words, 11 top-level sections, 12 sub-sections for commands + config
