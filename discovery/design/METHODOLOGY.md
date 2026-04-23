# Deep-Dive Methodology

> How to run thought experiments on each Mesh module before implementation.
> This is the playbook. Follow it strictly.

## Purpose

SYSTEM.md defines 6 modules with contracts and data flows. That's the skeleton.
Deep-dives add the flesh: failure recovery, edge cases, concurrency, state machines.
The goal is to find design flaws through thought experiments, not code.

Code is compiled output. The design is the true artifact. If the design has holes,
the code will have holes. Find them here, not in production.

## Who This Is For

- Sisyphus agents executing deep-dives on individual modules
- Prometheus planning the order and dependencies of deep-dives
- The human reviewing findings and making decisions

## The Six Tests

Every module MUST be tested against all six. No exceptions.

### T1: Happy Path
Walk through the primary operation with everything working.
Question: Does the design work when nothing fails?
Output: Step-by-step trace showing which module calls which, what data flows.

### T2: Failure at Each Step
For each external call the module makes, ask: "What happens when this fails?"
Questions:
- Where does state get stuck? (orphaned resources, dangling references)
- Who is responsible for cleanup? (the module itself? the caller? nobody?)
- Is the failure recoverable (retry) or fatal (rollback)?
- Does partial state leak to other modules?
Output: Failure table — one row per external call, columns: call, failure mode, recovery path, cleanup responsibility.

### T3: Concurrency
Two operations on the same resource running simultaneously.
Questions:
- Two snapshots of the same body at the same time — what happens?
- Create and destroy called for the same body ID simultaneously — race condition?
- Does the module need locking? At what granularity? (body-level? operation-level?)
Output: Concurrency analysis — which operations conflict, proposed locking strategy.

### T4: Scale
Run with 100 bodies, 1000 bodies, 100 simultaneous operations.
Questions:
- What bottlenecks appear? (storage I/O, network calls, API rate limits)
- Does the module's design assume single-instance? Can it run multiple?
- What resource usage grows linearly vs exponentially?
Output: Scale analysis — bottleneck list, proposed mitigations.

### T5: State Machine Edge Cases
The module has a state machine. Test every illegal transition.
Questions:
- What if migration is requested while body is in "Starting" state?
- What if snapshot is requested for a body that's already "Destroyed"?
- What if two conflicting lifecycle commands arrive in quick succession?
Output: State machine diagram with ALL transitions (legal and illegal), error handling for each illegal transition.

### T6: Contract Verification
Does the module's contract hold under all the above conditions?
Questions:
- Does the module always return what it promises? (or error, never garbage)
- Are there conditions where the module silently produces wrong output?
- Does the module's contract make assumptions about other modules' behavior?
Output: Contract invariants — statements that MUST always be true. Flag any that break under T2-T5.

## Deep-Dive Output Template

Every deep-dive file MUST follow this exact structure. No deviations.

```
# Deep-Dive: [Module Name]

> Module: [name]
> Agent: [who produced this]
> Date: [when]
> Status: draft | reviewed | approved

## Contract (Detailed)

### Inputs
[What this module receives, from where, in what format]

### Outputs
[What this module returns, to whom, in what format]

### Guarantees (Invariants)
[Statements that MUST always be true — violations are bugs]
- [INV-1]: [description]
- [INV-2]: [description]

### Assumptions
[What this module assumes about other modules — if violated, behavior is undefined]
- [ASM-1]: [description]

## State Machine

### States
[List all states the module's primary entity can be in]

### Transitions
[Table: from_state | trigger | to_state | side_effects]

### Illegal Transitions
[Table: from_state | rejected_trigger | error_type | recovery]

## T1: Happy Path Traces

### [Operation 1]
[Step-by-step trace with data at each step]

### [Operation 2]
[Step-by-step trace with data at each step]

## T2: Failure Analysis

| External Call | Failure Mode | State Left | Recovery Path | Cleanup By |
|---------------|-------------|------------|---------------|------------|
| [call] | [timeout/error/refused] | [what's stuck] | [retry/rollback/ignore] | [who cleans] |

## T3: Concurrency Analysis

### Conflicting Operations
[Which operations on the same resource conflict]

### Proposed Locking
[At what level, what granularity, deadlock prevention]

## T4: Scale Analysis

### Bottlenecks
| Resource | Limit | Threshold | Mitigation |
|----------|-------|-----------|------------|
| [resource] | [hard limit] | [when it matters] | [how to handle] |

## T5: Edge Cases

### [Edge Case 1]
[Scenario, expected behavior, actual behavior if different]

### [Edge Case 2]
[Scenario, expected behavior, actual behavior if different]

## T6: Contract Verification

### Invariant Checklist
| Invariant | T1 | T2 | T3 | T4 | T5 | Status |
|-----------|----|----|----|----|----|----|
| [INV-1] | ✅ | ❌ | ✅ | ✅ | ❌ | BROKEN |
| [INV-2] | ✅ | ✅ | ✅ | ✅ | ✅ | OK |

### Broken Invariants → Design Changes
[For each broken invariant: what design change fixes it]

## Updated Interface

[Any changes to the interface signatures discovered during analysis]

## Open Questions

[Questions that need human decision — not implementation questions, design questions]
- [Q1]: [description] — [why it matters] — [options]
```

## Execution Rules

1. **Each module gets its own file** in `design/deep/` — named after the module.
2. **Read SYSTEM.md first.** The deep-dive must respect the existing contracts. If it finds a contract is wrong, flag it — don't silently change it.
3. **Read relevant research.** Each module has backing research. Use it. Don't re-research what's already documented.
4. **Be honest about broken invariants.** If T2-T5 breaks something, say so. Don't hide it.
5. **Propose fixes, don't just flag problems.** "This breaks" is useless. "This breaks, here's how to fix it" is valuable.
6. **Target 150-250 lines per module.** Dense. No filler. Every sentence carries information.
7. **If you find a design gap that crosses module boundaries**, flag it explicitly. These are a hardest to find and most important to fix.

## Module Dependencies for Deep-Dives

Some deep-dives are easier if others are done first:

1. **Orchestration** — do first. It defines body lifecycle, which everything else depends on.
2. **Provisioning** — depends on Orchestration's state machine (when does provisioning get called).
3. **Persistence** — depends on Orchestration (when does snapshot happen) and Provisioning (what handle to export from).
4. **Networking** — depends on Orchestration (when does identity get assigned).
5. **Interface** — depends on all above (what tools to expose).
6. **Plugin Infrastructure** — independent. Can run in parallel with any.

Recommended order: Orchestration → (Provisioning + Networking in parallel) → Persistence → Interface. Plugin Infra whenever.

## Review Process

After all 6 deep-dives:
1. Read all deep-dive files.
2. Check for cross-module inconsistencies (Module A assumes X, Module B assumes not-X).
3. Update SYSTEM.md if contracts changed.
4. Present findings to human for decision on open questions.
5. Only THEN move to implementation planning.
