# v1.1 Roadmap Learnings

- DE1-DE16 refine D1-D10 for concrete v1.1 implementation
- Key shifts: Pulumi -> OpenAPI codegen (DE4), Docker built-in -> Docker plugin (DE8), auto-scheduler -> static config (DE2)
- All DE decisions dated 2026-04-29, suggesting a concentrated design review session
- 15 accepted, 0 discarded (D9 was the only discarded decision from the D-series)
- Fly Machines has no filesystem export API -- adapter must use self-export pattern (DE13)
- Extension interfaces (DE14/DE16) follow database/sql driver pattern from Go stdlib
