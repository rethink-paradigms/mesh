# Issues — mesh-nav QA

## 2026-04-25: Fresh Bootstrap QA

### I1: CLI --db arg order is fragile
- **Impact**: `gov.py trace D3 --db .mesh/governance.db` fails; must use `gov.py --db .mesh/governance.db trace D3`
- **Cause**: argparse parent parser requires --db before subcommand
- **Fix options**: (a) Document clearly, (b) Use env var MESH_DB as fallback, (c) Default --db to `.mesh/governance.db`

### I2: CONTEXT.md Quick Commands omit --db
- **Impact**: Copy-pasting examples from CONTEXT.md fails without --db flag
- **Location**: CONTEXT.md lines 93-97
- **Fix**: Add `--db .mesh/governance.db` to all examples or add a default

### I3: D3 has no outgoing graph edges
- **Impact**: `trace D3` returns "No connections found" — may confuse users expecting graph density
- **Severity**: Low — D3 is a valid leaf node (receives edges from D1)
- **Context**: Graph has 27 edges total; D1→D3 exists; D3 just doesn't enable/block anything downstream
