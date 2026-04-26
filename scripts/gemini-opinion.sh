#!/usr/bin/env bash
# gemini-opinion.sh — Get a second opinion from Gemini CLI
# Usage: ./scripts/gemini-opinion.sh <mode> [file-or-scope]
# Modes: review, architect, design, plan, freeform
#
# Called from Claude Code as a Codex replacement for outside opinions.

set -euo pipefail

MODE="${1:-review}"
SCOPE="${2:-.}"
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
OUTFILE="/tmp/gemini-opinion-${TIMESTAMP}.md"

cd "$REPO_ROOT"

case "$MODE" in
  review)
    PROMPT="You are reviewing this codebase as a second opinion alongside another AI reviewer. Focus on:
1. Bugs and logic errors (name the file, function, and line)
2. Security issues (injection, auth, secrets)
3. Design system violations (read DESIGN.md first if doing UI work)
4. Test coverage gaps
5. Performance concerns

Be direct. No hedging. Number each finding. If nothing is wrong, say 'clean' and explain why."
    ;;
  architect)
    PROMPT="You are a senior architect giving a second opinion on this Go codebase. Evaluate:
1. Package boundaries — are they clean? Any circular deps?
2. Interface design — does the Provider interface properly abstract substrates?
3. Error taxonomy — are structured error codes used consistently?
4. Concurrency — any race conditions or deadlock risks?
5. Backwards compatibility — will v1 changes break v0 users?

Reference discovery/design/SYSTEM.md for the target architecture. Be specific."
    ;;
  design)
    PROMPT="You are a senior product designer reviewing this UI code against the design system.
Read DESIGN.md first. Then check:
1. Typography — Instrument Serif for display, Geist Sans for body, Berkeley Mono for code
2. Colors — #0A0A0B bg, #C8956C copper accent, warm grays, no blue/purple
3. Spacing — 4px base unit, compact density
4. Border-radius — 2/4/8/12/9999px scale
5. Motion — intentional only, spring entrances, preview render pattern
6. Layout — chat-first with Rail/Canvas/Prompt zones

Flag every violation with file:line. No 'looks good overall' hedging."
    ;;
  plan)
    PROMPT="You are a senior engineer reviewing an implementation plan. Read the plan context below and evaluate:
1. Is the build order correct? Any dependency ordering issues?
2. Are the interfaces well-defined before implementation starts?
3. Are error cases and edge cases accounted for?
4. Is the scope right — too ambitious or too timid?
5. What would you change about the approach?

Be opinionated. This is YOUR engineering judgment."
    ;;
  freeform)
    PROMPT="${3:-What do you think about this codebase? Any concerns or improvements?}"
    ;;
  *)
    echo "Unknown mode: $MODE"
    echo "Usage: $0 <review|architect|design|plan|freeform> [scope] [freeform-prompt]"
    exit 1
    ;;
esac

echo "Running Gemini CLI ($MODE mode) on $SCOPE..."
echo "Output: $OUTFILE"
echo "---"

gemini -p "$PROMPT" 2>/dev/null | tee "$OUTFILE"

echo ""
echo "---"
echo "Second opinion saved to: $OUTFILE"