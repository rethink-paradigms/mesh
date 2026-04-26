# Learnings — mesh-nav QA

## 2026-04-25: Fresh Bootstrap QA

### L1: Argparse --db position
All mesh-nav scripts (`gov.py`, `session.py`, `generate.py`) use a parent parser with `--db`. The flag MUST come before the subcommand. Pattern: `script.py --db <path> <subcommand> [args]`.

### L2: Graph is well-populated
27 edges across 34 entities. D1 is a hub node (enables D2, D3, D4). D2 enables D4. Most decisions are leaves.

### L3: Session tracking works
Session 3 correctly records summary, focus, next_steps, decisions_made (D-GOV1-8), files_created (5), files_modified (4). Clean output format.

### L4: Generated markdown matches CONTEXT.md
`generate.py context-summary` and `generate.py decisions-md` produce output consistent with the hand-written/auto-generated CONTEXT.md. Cross-check passes.
