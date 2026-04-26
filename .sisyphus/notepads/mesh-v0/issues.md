# Issues — mesh-v0

(No issues yet)

## F4 Scope Fidelity Check (2026-04-24)

### Issues Found
1. **Task 5 (Restore) — Missing disk space pre-flight**: Plan specifies `syscall.Statfs` check for available disk space before restore. Current implementation only checks writability via temp file creation. Restore will still fail on ENOSPC but without a helpful pre-flight message. Low severity.

2. **Task 8 (Clone) — Missing remote agent start**: Plan specifies "Start agent on target via SSH: execute start_cmd". `remoteClone()` in `internal/clone/clone.go` extracts the tarball but never executes the agent's `start_cmd` on the remote machine. Moderate severity — cloned agent won't auto-start.

### Verified Clean
- All 14 "Must NOT Have" constraints pass
- discovery/ directory untouched
- No forbidden imports (docker, gRPC, protobuf, Go SSH libs)
- No cross-task contamination
- 11/13 tasks fully compliant, 2 with missing features
