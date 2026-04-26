# Decisions — mesh-v0

## 2026-04-23 Session Start
- External deps: klauspost/compress/zstd, BurntSushi/toml, spf13/cobra
- Streaming pipeline: io.Pipe for tar → zstd → sha256 in one pass
- No CGo, no Docker, no SSH libraries — pure Go + os/exec
- Snapshot storage: ~/.mesh/snapshots/{agent_name}/
- Agent identification: by name from config
