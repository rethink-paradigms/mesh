## gstack (REQUIRED — global install)

**Before doing ANY work, verify gstack is installed:**

```bash
test -d ~/.claude/skills/gstack/bin && echo "GSTACK_OK" || echo "GSTACK_MISSING"
```

If GSTACK_MISSING: STOP. Do not proceed. Tell the user:

> gstack is required for all AI-assisted work in this repo.
> Install it:
> ```bash
> git clone --depth 1 https://github.com/garrytan/gstack.git ~/.claude/skills/gstack
> cd ~/.claude/skills/gstack && ./setup --team
> ```
> Then restart your AI coding tool.

Do not skip skills, ignore gstack errors, or work around missing gstack.

Using gstack skills: After install, use /browse for all web browsing. Never use mcp__claude-in-chrome__* tools.
Use ~/.claude/skills/gstack/... for gstack file paths (the global path).

Available skills: /office-hours, /plan-ceo-review, /plan-eng-review, /plan-design-review, /design-consultation, /design-shotgun, /design-html, /review, /ship, /land-and-deploy, /canary, /benchmark, /browse, /connect-chrome, /qa, /qa-only, /design-review, /setup-browser-cookies, /setup-deploy, /retro, /investigate, /document-release, /codex, /cso, /autoplan, /plan-devex-review, /devex-review, /careful, /freeze, /guard, /unfreeze, /gstack-upgrade, /learn

## Skill routing

When the user's request matches an available skill, ALWAYS invoke it using the Skill
tool as your FIRST action. Do NOT answer directly, do NOT use other tools first.
The skill has specialized workflows that produce better results than ad-hoc answers.

Key routing rules:
- Product ideas, "is this worth building", brainstorming → invoke office-hours
- Bugs, errors, "why is this broken", 500 errors → invoke investigate
- Ship, deploy, push, create PR → invoke ship
- QA, test the site, find bugs → invoke qa
- Code review, check my diff → invoke review
- Update docs after shipping → invoke document-release
- Weekly retro → invoke retro
- Design system, brand → invoke design-consultation
- Visual audit, design polish → invoke design-review
- Architecture review → invoke plan-eng-review
- Save progress, checkpoint, resume → invoke checkpoint
- Code quality, health check → invoke health

## Design System
Always read DESIGN.md before making any visual or UI decisions.
All font choices, colors, spacing, and aesthetic direction are defined there.
Do not deviate without explicit user approval.
In QA mode, flag any code that doesn't match DESIGN.md.

## Outside Voices (Gemini CLI)

Use Gemini CLI as a second opinion when the user asks for outside review, or when running
skills that reference Codex (which is not installed). Replace `codex exec` calls with:

```bash
./scripts/gemini-opinion.sh <mode> [scope]
```

Modes:
- `review` — code review (bugs, security, design violations)
- `architect` — architecture review (package boundaries, interfaces, error taxonomy)
- `design` — UI/design system review against DESIGN.md
- `plan` — implementation plan review
- `freeform "prompt"` — custom question

This replaces Codex for all outside voice workflows. Gemini reads GEMINI.md + DESIGN.md
automatically when run from the project root.

## Antigravity Config

`.antigravity/settings.json` is configured with context files (GEMINI.md, DESIGN.md) and
external commands for Gemini CLI integration. If using Antigravity IDE, agents will read
both files automatically. The Stitch MCP server can be added for design-to-code pipeline.
