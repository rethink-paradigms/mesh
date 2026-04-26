# mesh-v1 Learnings

## Initial State
- v0 codebase: Go module `github.com/rethink-paradigms/mesh`, Go 1.25.5
- Existing packages: agent, clone, config (TOML), manifest, restore, snapshot, transport
- Packages to KEEP (DO NOT TOUCH): snapshot, restore, manifest
- Packages to DELETE: clone, agent, transport
- Packages to RENAME: config (TOML → YAML, new location)
- Dependencies: cobra, toml, klauspost/compress
- New deps needed: docker/client, testcontainers-go, modernc.org/sqlite, gopkg.in/yaml.v3, mcp-go SDK
- new config pkg replaces old; old config pkg kept around for backward compat until full migration, or we delete it
