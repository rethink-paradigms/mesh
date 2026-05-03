# Dimension 7: AI Agent Skill Design for Code Generation

## Deep Research: SubstrateAdapter Generator Skill for AI Agents

**Research Date**: 2025  
**Scope**: Design a reusable AI agent skill that generates Go SubstrateAdapter implementations from OpenAPI specs + interface definitions  
**Method**: 25+ independent web searches across agent skills, code generation reliability, Go interface patterns, validation gates, and AI coding benchmarks.

---

## 1. Executive Summary

- **Agent Skills are now an open standard**: Anthropic's Agent Skills specification (agentskills.io) defines a portable `SKILL.md` format adopted by 30+ platforms including Claude Code, GitHub Copilot, Cursor, OpenCode, and Gemini CLI[^459^][^377^]. This is the correct foundational format for our skill.
- **Progressive disclosure is the key architectural pattern**: Skills load only metadata (~100 tokens) at startup, full instructions (<5,000 tokens) when triggered, and references/scripts on demand[^384^][^399^]. This keeps context windows manageable while enabling deep expertise.
- **Speakeasy has validated the "agent skills for SDK generation" pattern**: They publish 21 skills covering OpenAPI authoring, SDK generation (per-language), Terraform provider generation, and diagnostics[^137^][^135^]. Their design principle: "focused, use-case-specific skills outperform broad meta-skills"[^135^].
- **Test-Driven Generation is the most reliable pattern**: Multiple sources confirm that "write tests first → generate implementation to pass → refactor" produces more correct, reviewable code than generating both at once[^362^][^364^][^365^]. AI-TDD catches hallucinations like negation bugs that static analysis misses.
- **Go interface satisfaction must be enforced at compile time**: The `var _ Interface = (*Type)(nil)` pattern is the idiomatic Go compile-time assertion[^422^][^424^][^428^]. Generated adapters MUST include this line.
- **Boundary enforcement requires constitutional constraints**: Research shows that embedding non-negotiable architectural rules in the specification layer ("controllers must not access db directly") reduces defects by 73% compared to unconstrained generation[^510^][^511^].
- **Go SWE-bench resolve rates are ~30%**: On SWE-bench Multilingual, Go tasks have a 30.95% resolution rate across models[^491^]. On SWE-bench Pro, Go and Python show higher resolve rates than JS/TS[^435^]. Claude 4.5 Sonnet achieves 75.4% on SWE-bench Verified with tool creation[^436^].
- **Validation gates should be automated via hooks**: Claude Code hooks (PostToolUse, Stop) can auto-run `go build`, `go vet`, and interface checks after every edit[^509^][^512^][^521^].
- **Structured Chain-of-Thought (SCoT) prompting outperforms standard CoT by up to 13.79%** on code generation benchmarks[^366^][^369^]. For Go adapter generation, the skill should require the agent to think in program structures (sequence, branch, loop) before writing code.
- **Few-shot prompting with 2-3 working adapter examples is the sweet spot**: Research shows diminishing returns after 2-3 examples[^419^]. For our skill, bundling 2 reference adapters (e.g., Docker adapter, AWS adapter) as `assets/` files and instructing the agent to follow their pattern is the most reliable approach.

---

## 2. Detailed Findings

### 2.1 Input Format for the Agent

**Research Question**: What is the most reliable input format for an AI agent generating SubstrateAdapter code?

#### Finding 1.1: The OpenAPI Spec + Target Interface + Template Pattern is Validated

Claim: Speakeasy's production SDK generation system uses exactly this pattern: parse OpenAPI spec → apply language-specific templates → generate idiomatic code[^405^][^502^].  
Source: Speakeasy Documentation  
URL: https://www.speakeasy.com/docs/sdks/create-client-sdks  
Date: 2026-01-22  
Excerpt: "Speakeasy validates the specifications and generates the SDK after receiving all inputs. The process executes `speakeasy run` to validate, generate, compile, and set up the SDK."  
Context: Speakeasy generates SDKs for 7+ languages from OpenAPI specs using a CLI-driven workflow.  
Confidence: High

Claim: Stainless (another SDK generator) also uses OpenAPI spec → template → generated SDK, producing type-safe clients with nested namespaces for complex types[^502^].  
Source: Dev.to comparison article  
URL: https://dev.to/andy_tate_/generating-building-the-openai-sdk-with-stainless-speakeasy-32nb  
Date: 2025-03-13  
Excerpt: "It extends APIResource to inherit standard API functionality like authentication and request handling. The method name 'create' is derived from the OpenAPI operationId. The return type uses a generic APIPromise to handle asynchronous operations while maintaining type safety."  
Context: Comparison of Stainless vs Speakeasy for OpenAI SDK generation.  
Confidence: High

#### Finding 1.2: A Structured "Provider Manifest" is Superior to Raw Prompts

Claim: Spec-Driven Development (SDD) research shows that structured specifications with constitutional constraints produce 73% fewer security defects than unconstrained AI generation[^510^].  
Source: arXiv paper "Constitutional Spec-Driven Development"  
URL: https://arxiv.org/html/2602.02584v1  
Date: 2026-01-31  
Excerpt: "The Constitution sits at the apex of the development hierarchy, governing all downstream artifacts... Constitutional constraints reduce security defects by 73% compared to unconstrained AI generation while maintaining developer velocity."  
Context: Banking microservices case study with 10 CWE vulnerabilities.  
Confidence: High

Claim: GitHub's Spec Kit defines a "Constitution" as the first stage of spec-driven workflows, encoding stack versions, naming conventions, layering principles, and allowed/forbidden libraries[^518^].  
Source: GitHub spec-kit repository  
URL: https://github.com/github/spec-kit/blob/main/spec-driven.md  
Date: 2025-08-21  
Excerpt: "The Constitution stage encodes your project DNA by documenting stack versions, naming conventions, layering and architectural principles, allowed/forbidden libs, and auth/logging/accessibility."  
Context: The "Nine Articles of Development" formalize spec-driven AI workflows.  
Confidence: High

#### Finding 1.3: Provider SDK Godoc is a Valuable but Secondary Input

Claim: Go's implicit interface satisfaction means the agent needs the target interface definition (method signatures + doc comments) as primary input, not just the OpenAPI spec[^497^].  
Source: OneUptime blog on Go interfaces  
URL: https://oneuptime.com/blog/post/2026-01-23-go-implement-interfaces/view  
Date: 2026-01-23  
Excerpt: "Go's interface system is one of its most elegant features. Unlike other languages, Go uses implicit implementation - there's no `implements` keyword. If your type has the right methods, it implements the interface."  
Context: Go interface implementation guide.  
Confidence: High

**Preliminary Recommendation**: The optimal input format is a composite manifest:

```yaml
# provider-manifest.yaml
openapi_spec: ./openapi.yaml          # Primary: API contract
target_interface: ./interface.go      # Primary: Substrate interface to satisfy
reference_adapters:                     # Secondary: few-shot examples
  - ./adapters/docker/adapter.go
  - ./adapters/aws/adapter.go
template: ./templates/adapter.go.tmpl   # Optional: structural template
constraints:                            # Tertiary: constitutional boundaries
  max_lines: 200
  forbidden_patterns:
    - "net/http.Client"                 # Agent must NOT generate
    - "json.Marshal"                    # Use SDK's serialization
```

This aligns with the Constitutional Spec-Driven Development model where the manifest acts as the "constitution" for generation[^510^][^518^].

---

### 2.2 Existing Agent Skill Patterns

#### Finding 2.1: Anthropic Agent Skills Standard (agentskills.io)

Claim: The Agent Skills open standard defines a `SKILL.md` file with YAML frontmatter + Markdown body, stored in a directory with optional `scripts/`, `references/`, and `assets/` subdirectories[^459^].  
Source: agentskills.io specification  
URL: https://agentskills.io/specification  
Date: 2026 (current)  
Excerpt: "A skill is a directory containing, at minimum, a `SKILL.md` file... The `SKILL.md` file must contain YAML frontmatter followed by Markdown content."  
Context: Official specification document.  
Confidence: High

Claim: Progressive disclosure is the core design principle: metadata (~100 tokens) loads at startup, instructions (<5,000 tokens) load when triggered, resources load on demand[^459^][^384^].  
Source: Anthropic engineering blog  
URL: https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills  
Date: 2025-10-16  
Excerpt: "At startup, the agent pre-loads the `name` and `description` of every installed skill into its system prompt... If Claude thinks the skill is relevant to the current task, it will load the skill by reading its full `SKILL.md` into context."  
Context: Documenting the PDF skill as a real example.  
Confidence: High

#### Finding 2.2: Speakeasy Skills for SDK Generation

Claim: Speakeasy publishes 21 focused skills rather than one broad skill, following the principle that "focused, use-case-specific skills outperform broad meta-skills"[^135^].  
Source: Speakeasy blog  
URL: https://www.speakeasy.com/blog/release-agent-skills  
Date: 2026-02-02  
Excerpt: "Rather than a single comprehensive skill for SDK generation, we have separate skills for each language, for starting a new project, for diagnosing failures, and for customizing hooks. Each skill has a specific trigger condition that makes activation clear."  
Context: Speakeasy's skill design philosophy.  
Confidence: High

Claim: Speakeasy skills use the standard `speakeasy:<skill-name>` namespacing and are installable via `npx skills add speakeasy-api/skills`[^137^].  
Source: GitHub speakeasy-api/skills  
URL: https://github.com/speakeasy-api/skills  
Date: 2026-01-23  
Excerpt: "A collection of Agent Skills for SDK generation and OpenAPI tooling with the Speakeasy CLI."  
Context: Repository README showing available skills table.  
Confidence: High

#### Finding 2.3: Claude Code Skills vs Slash Commands vs Hooks

Claim: Claude Code has three distinct mechanisms: Skills (auto-triggered based on context), Slash Commands (explicit user invocation), and Hooks (lifecycle event handlers)[^314^][^317^].  
Source: MindStudio blog  
URL: https://www.mindstudio.ai/blog/claude-code-skills-vs-slash-commands/  
Date: 2026-04-18  
Excerpt: "Claude Code Skills auto-invoke based on what you're doing. Slash commands require you to type something first... Skills are designed around process. Slash commands are explicit, user-initiated instructions."  
Context: Detailed comparison of the two mechanisms.  
Confidence: High

Claim: Claude Code hooks can auto-run `go vet`, `go build`, and custom validation scripts after every file edit[^509^][^521^].  
Source: Claude Code hooks documentation  
URL: https://code.claude.com/docs/en/hooks  
Date: 2025-09-01  
Excerpt: "Hooks let you run code at key points in Claude Code's lifecycle: format files after edits, block commands before they execute, send notifications when Claude finishes."  
Context: Official hooks reference documentation.  
Confidence: High

#### Finding 2.4: Cursor Rules, Copilot Instructions, OpenCode Skills

Claim: Cursor uses `.cursorrules` / `.mdc` files with glob-scoped rules, `alwaysApply` flags, and constraint-based language ("Functions must be under 30 lines")[^323^].  
Source: DataCamp tutorial  
URL: https://www.datacamp.com/tutorial/cursor-rules  
Date: 2026-03-11  
Excerpt: "Rules written with soft language ('try to keep functions small') give the AI permission to ignore them. Write commands instead: 'Functions must be under 30 lines. All endpoints must be async.'"  
Context: Cursor rules best practices.  
Confidence: High

Claim: GitHub Copilot supports custom instructions via `.github/copilot-instructions.md`, path-specific `.instructions.md` files, and agent skills in `.github/skills/<skill-name>/SKILL.md`[^315^][^319^].  
Source: GitHub Docs  
URL: https://docs.github.com/en/copilot/reference/customization-cheat-sheet  
Date: 2026-03-10  
Excerpt: "Agent skills: Folder of instructions, scripts, and resources that Copilot loads when relevant to a task. Location: `.github/skills/<skill-name>/SKILL.md`"  
Context: Copilot customization cheat sheet.  
Confidence: High

Claim: OpenCode skills follow the same `SKILL.md` standard as Claude Code, with discovery from `.opencode/skills/`, `.claude/skills/`, and `.agents/skills/` directories[^316^].  
Source: OpenCode documentation  
URL: https://opencode.ai/docs/skills/  
Date: 2026-04-28  
Excerpt: "Agent skills let OpenCode discover reusable instructions from your repo or home directory. Skills are loaded on-demand via the native `skill` tool—agents see available skills and can load the full content when needed."  
Context: OpenCode skills documentation.  
Confidence: High

**Preliminary Recommendation**: Use the **Anthropic Agent Skills standard** as the base format for maximum portability. The skill should be installable in Claude Code (`~/.claude/skills/`), Copilot (`.github/skills/`), Cursor (`.cursor/skills/`), and OpenCode (`.opencode/skills/`).

---

### 2.3 Reliability Patterns

#### Finding 3.1: Few-Shot Examples (2-3 is the Sweet Spot)

Claim: Few-shot prompting with 2-5 examples provides the best balance between guidance and flexibility; diminishing returns appear after 2-3 examples[^419^][^417^].  
Source: PromptHub few-shot guide  
URL: https://www.prompthub.us/blog/the-few-shot-prompting-guide  
Date: 2025-10-23  
Excerpt: "Research shows diminishing returns after two to three examples. Including too many examples just burns more tokens without adding much value. Typically, two to five examples are good, and we recommend not going beyond eight."  
Context: Comprehensive few-shot prompting guide.  
Confidence: High

Claim: For code generation specifically, few-shot examples teach formatting, naming conventions, and logic patterns better than instructions alone[^417^].  
Source: QuillBot few-shot guide  
URL: https://quillbot.com/blog/ai-prompt-writing/few-shot-prompting/  
Date: 2025-12-19  
Excerpt: "Code generation: Produces functions, snippets, or patterns in a consistent style. Examples teach formatting, naming, and logic patterns."  
Context: Table of few-shot prompting applications.  
Confidence: High

#### Finding 3.2: Structured Chain-of-Thought (SCoT) for Code

Claim: Structured Chain-of-Thought prompting, which asks LLMs to use program structures (sequence, branch, loop) for reasoning, outperforms standard CoT by up to 13.79% on HumanEval, MBPP, and MBCPP benchmarks[^366^][^369^].  
Source: ACM TOSEM / arXiv  
URL: https://dl.acm.org/doi/10.1145/3690635  
Date: 2025-01-21  
Excerpt: "SCoT prompting outperforms CoT prompting by up to 13.79% in Pass@1. SCoT prompting is robust to examples and achieves substantial improvements. The human evaluation also shows human developers prefer programs from SCoT prompting."  
Context: Peer-reviewed paper on structured CoT for code generation.  
Confidence: High

Claim: For tasks requiring 5+ reasoning steps, reasoning models (o1-mini) outperform non-reasoning models by 16.67%; for simpler tasks (<3 steps), non-reasoning models are better[^363^].  
Source: PromptHub CoT guide  
URL: https://www.prompthub.us/blog/chain-of-thought-prompting-guide  
Date: 2025-10-23  
Excerpt: "For CoT tasks with five or more steps, o1-mini outperforms GPT-4o by 16.67%. For CoT tasks under five steps, o1-mini's advantage shrinks to 2.89%."  
Context: Analysis of when CoT helps vs hurts.  
Confidence: Medium

#### Finding 3.3: Test-Driven Generation (Red-Green-Refactor with AI)

Claim: Test-First Prompting forces upfront threat modeling, catches hallucinations, and empowers human-in-the-loop validation[^362^].  
Source: Endor Labs blog  
URL: https://www.endorlabs.com/learn/test-first-prompting-using-tdd-for-secure-ai-generated-code  
Date: 2026-02-04  
Excerpt: "TDD is essential for secure prompting because it: Forces Upfront Threat Modeling; Catches Hallucinations; Empowers the Human-in-the-Loop."  
Context: 5-part series on secure code prompt patterns.  
Confidence: High

Claim: The AI-TDD workflow is: (1) Generate tests from spec → (2) Run tests (fail) → (3) Generate implementation to pass tests → (4) Refactor while keeping tests green[^364^][^365^].  
Source: Dev.to TDD with Claude Code  
URL: https://dev.to/myougatheaxo/test-driven-development-with-claude-code-write-tests-first-then-make-them-pass-2a6m  
Date: 2026-03-11  
Excerpt: "Classic TDD: write a failing test → write the minimum code to pass → refactor. With Claude Code, each phase looks like this: Phase 1 — Write the test... Phase 2 — Write the implementation... Phase 3 — Refactor."  
Context: Practical TDD workflow with Claude Code.  
Confidence: High

Claim: Generating tests and code in one shot produces tests that match the implementation (not requirements), missing edge cases, and tests that pass because they test the wrong thing[^365^].  
Source: Dev.to TDD article  
URL: https://dev.to/myougatheaxo/test-driven-development-with-claude-code-write-tests-first-then-make-them-pass-2a6m  
Date: 2026-03-11  
Excerpt: "Asking Claude Code to 'write UserService with tests' in one shot often produces: Tests that are written to match the implementation (not requirements); Missing edge cases; Tests that pass because they test the wrong thing."  
Context: Warning about single-shot generation.  
Confidence: High

#### Finding 3.4: Reference Implementation Copy

Claim: Speakeasy's per-language SDK generation uses language-specific templates crafted with domain experts to produce idiomatic code[^405^].  
Source: Speakeasy docs  
URL: https://www.speakeasy.com/docs/sdks/create-client-sdks  
Date: 2026-01-22  
Excerpt: "For each language, Speakeasy has crafted generators with language experts to be highly idiomatic."  
Context: SDK generation documentation.  
Confidence: High

Claim: Matching existing codebase patterns is critical for maintainability. Cursor rules explicitly instruct: "Match the patterns already in the codebase. Do not refactor surrounding code unless asked."[^323^]  
Source: DataCamp  
URL: https://www.datacamp.com/tutorial/cursor-rules  
Date: 2026-03-11  
Excerpt: "When modifying existing code: Match the patterns already in the codebase. Do not refactor surrounding code unless asked. Do not add type annotations to unchanged code. Do not add docstrings to unchanged functions."  
Context: Cursor rules best practices.  
Confidence: High

**Preliminary Recommendation**: The skill should enforce a **4-phase generation workflow**:
1. **Analyze** (structured CoT): Agent analyzes OpenAPI spec + target interface, identifies mapping points
2. **Plan** (test-first): Agent writes a test file asserting interface satisfaction + behavior
3. **Write** (few-shot guided): Agent generates the adapter following reference examples
4. **Verify** (validation gates): Agent runs `go build`, `go vet`, interface checks, tests

---

### 2.4 Boundary Enforcement

#### Finding 4.1: The "Negative Boundary" Pattern in Skill Descriptions

Claim: Good skill descriptions include negative triggers: "Do NOT use for PDFs, spreadsheets, Google Docs, or general coding tasks"[^398^].  
Source: Medium deep dive on SKILL.md  
URL: https://abvijaykumar.medium.com/deep-dive-skill-md-part-1-2-09fc9a536996  
Date: 2026-03-17  
Excerpt: "That negative boundary is super important for preventing the skill from firing when it shouldn't—we'll dig deeper into why later."  
Context: Analysis of Anthropic's docx skill frontmatter.  
Confidence: High

#### Finding 4.2: Constitutional Constraints for Architectural Boundaries

Claim: Constitutional constraints can encode architectural principles like "controllers must not access the database directly" and "depend on abstractions rather than concrete implementations"[^510^].  
Source: arXiv CSDD paper  
URL: https://arxiv.org/html/2602.02584v1  
Date: 2026-01-31  
Excerpt: "Architectural Principles. Organizations can encode architectural constraints such as layered separation (controllers must not access the database directly), dependency inversion (depend on abstractions rather than concrete implementations)."  
Context: Generalizability section of the paper.  
Confidence: High

Claim: The same paper proposes enforcement levels (MUST/SHOULD/MAY per RFC 2119) for constraints[^510^].  
Source: arXiv CSDD paper  
URL: https://arxiv.org/html/2602.02584v1  
Date: 2026-01-31  
Excerpt: "Enforcement Level: MUST, SHOULD, or MAY... Constraint: What the code must/must not do."  
Context: Constitutional principles structure.  
Confidence: High

#### Finding 4.3: `allowed-tools` Field for Tool Restriction

Claim: The `allowed-tools` frontmatter field lets skill authors declare exactly which tools the skill needs, preventing scope creep[^398^][^402^].  
Source: Anthropic skills guide / Medium  
URL: https://abvijaykumar.medium.com/deep-dive-skill-md-part-1-2-09fc9a536996  
Date: 2026-03-17  
Excerpt: "The `allowed-tools` field in the frontmatter (still experimental) lets skill authors declare exactly which tools the skill needs: for example, `Bash(git:*) Bash(jq:*) Read`. This scopes the skill's permissions so it can't reach for tools it shouldn't need."  
Context: Security and trust considerations.  
Confidence: Medium (marked experimental)

#### Finding 4.4: Bitloops Architectural Constraints Framework

Claim: Architectural constraints are machine-readable rules about where code can live, what it can depend on, and how components interact[^380^].  
Source: Bitloops blog  
URL: https://bitloops.com/resources/governance/architectural-constraints-for-ai-agents  
Date: Unknown  
Excerpt: "Layer boundary constraints: 'Code in the `controllers` package cannot directly instantiate or import classes from the `db` package.'... Dependency direction constraints: 'Modules can only depend on modules at the same level or on lower (more foundational) levels.'"  
Context: Framework for enforcing structural patterns in AI-generated code.  
Confidence: Medium

**Preliminary Recommendation**: Boundary enforcement for the SubstrateAdapter Generator should use a multi-layer approach:

| Layer | Mechanism | Example |
|-------|-----------|---------|
| 1. Skill description | Negative triggers | "Do NOT generate HTTP clients, serialization logic, retry logic, or authentication handlers" |
| 2. Constitutional constraints | MUST/SHALL rules in skill body | "MUST only generate the mapping layer between the Substrate interface and the provider SDK" |
| 3. Template scaffolding | Pre-defined file structure | `assets/adapter-scaffold.go` with `// TODO: implement methods` stubs |
| 4. Validation script | `scripts/validate-boundary.sh` | Checks for forbidden imports (net/http, encoding/json direct use) |
| 5. Post-generation hook | `PostToolUse` hook | Runs boundary check after every file write |

---

### 2.5 Validation Gates

#### Finding 5.1: go build + go vet as Minimum Gates

Claim: `go build` is the most fundamental validation: if it compiles, the code is syntactically valid and all dependencies resolve[^422^].  
Source: Go interface compliance article  
URL: https://dev.to/kittipat1413/checking-if-a-type-satisfies-an-interface-in-go-432n  
Date: 2024-10-30  
Excerpt: "If `TypeName` does not fully implement `InterfaceName`, the Go compiler will raise an error immediately."  
Context: Compile-time interface satisfaction checks.  
Confidence: High

Claim: Static analysis tools (golangci-lint, staticcheck) are critical for AI-generated code because they catch issues humans miss[^383^][^385^].  
Source: JetBrains Qodana guide  
URL: https://www.jetbrains.com/pages/static-code-analysis-guide/code-analysis-for-ai-generated-code/  
Date: Unknown  
Excerpt: "AI-generated code doesn't understand how to maintain your application the same way an experienced developer does. Static analysis helps catch problems like code complexity, duplicate code, performance problems."  
Context: Guide for validating AI-generated code.  
Confidence: High

#### Finding 5.2: Go Interface Satisfaction Check

Claim: The `var _ InterfaceName = (*TypeName)(nil)` pattern provides compile-time verification that a type implements an interface, with zero runtime cost[^422^][^424^][^428^][^431^].  
Source: Stack Overflow / Go FAQ  
URL: https://stackoverflow.com/questions/10498547/ensure-a-type-implements-an-interface-at-compile-time-in-go  
Date: 2022-09-07  
Excerpt: "You can ask the compiler to check that the type `T` implements the interface `I` by attempting an assignment using the zero value for `T` or pointer to `T`, as appropriate."  
Context: Go FAQ reference.  
Confidence: High

Claim: Uber Go Style Guide considered adding `var _ Interface = (*Type)(nil)` as a recommendation[^430^].  
Source: Uber Go Guide GitHub issue  
URL: https://github.com/uber-go/guide/issues/25  
Date: 2019-10-12  
Excerpt: "+1, if a type is expected to satisfy an interface, and we don't already verify this, then the `var _ <interface> = <type instantiation>` makes sense."  
Context: Uber style guide maintainer discussion.  
Confidence: High

#### Finding 5.3: Integration Test with Sandbox

Claim: Hopx demonstrates a validation workflow: Generate → Execute in isolated sandbox → Test → Validate → Iterate → Accept[^379^].  
Source: Hopx AI  
URL: https://hopx.ai/use-cases/validate-ai-code/  
Date: Unknown  
Excerpt: "Execute AI code in isolated micro-VMs. No risk to your systems, even if the code is malicious or buggy. Automated Testing: Execute generated code with unit tests and static analysis. Automatically reject code that fails validation."  
Context: AI code validation platform.  
Confidence: Medium

Claim: The `Stop` hook with an agent-based handler can verify all tests pass before allowing Claude to finish[^523^].  
Source: Claude Code hooks docs  
URL: https://code.claude.com/docs/en/hooks  
Date: 2025-09-01  
Excerpt: "This `Stop` hook verifies that all unit tests pass before allowing Claude to finish: `{ 'type': 'agent', 'prompt': 'Verify that all unit tests pass. Run the test suite and check the results. $ARGUMENTS', 'timeout': 120 }`"  
Context: Agent-based hooks example.  
Confidence: High

#### Finding 5.4: Claude Code Hooks for Automated Validation

Claim: PostToolUse hooks can auto-format and lint after every edit; Stop hooks can run final quality checks[^509^][^512^].  
Source: Pixelmojo blog  
URL: https://www.pixelmojo.io/blogs/claude-code-hooks-production-quality-ci-cd-patterns  
Date: 2026-02-13  
Excerpt: "Command Hooks: `prettier --write`, `eslint --fix`, `tsc --noEmit`... Agent Hooks: Spawns agent with Read, Grep, Glob to analyze codebase."  
Context: Production CI/CD patterns with hooks.  
Confidence: High

**Preliminary Recommendation**: The validation pipeline for generated adapters should be:

```bash
# Gate 1: Syntax & Compilation
go build ./...

# Gate 2: Static Analysis
go vet ./...
staticcheck ./...

# Gate 3: Interface Satisfaction (compile-time)
# (Built into go build via var _ assertions)

# Gate 4: Unit Tests
go test ./... -race

# Gate 5: Boundary Check
grep -r "net/http.Client" pkg/adapters/ && exit 1  # Forbidden
grep -r "json.Marshal" pkg/adapters/ && exit 1       # Forbidden

# Gate 6: Integration Test (sandbox)
docker-compose -f test/docker-compose.yml up --abort-on-container-exit
```

---

### 2.6 AI Code Generation Research

#### Finding 6.1: SWE-bench Results for Go

Claim: On SWE-bench Multilingual (300 tasks across 9 languages), Go has a 30.95% resolution rate with 42 tasks total[^491^].  
Source: SWE-bench official results  
URL: https://www.swebench.com/multilingual.html  
Date: 2026-02-17  
Excerpt: "Go: 13 resolved, 29 unresolved, 42 total, 30.95% resolution rate."  
Context: Appendix B evaluation results by language.  
Confidence: High

Claim: Go and Python generally show higher resolve rates on SWE-bench Pro compared to JavaScript/TypeScript, with some models exceeding 30% in Go[^435^].  
Source: Scale Labs SWE-bench Pro  
URL: https://labs.scale.com/leaderboard/swe_bench_pro_public  
Date: 2026-04-29  
Excerpt: "Go and Python tasks generally have higher resolution rates, with some models exceeding 30%. In contrast, performance on JavaScript (JS) and TypeScript (TS) is more varied and often lower."  
Context: SWE-bench Pro analysis section.  
Confidence: High

Claim: Live-SWE-agent (Claude 4.5 Sonnet) achieves 46.0% on SWE-bench Multilingual by creating custom tools on the fly[^436^].  
Source: arXiv paper  
URL: https://arxiv.org/html/2511.13646v1  
Date: 2025-11-17  
Excerpt: "Live-SWE-agent achieves a better performance by obtaining a resolve rate of 46.0%, while mini-SWE-agent only has a resolve rate of 40.0%."  
Context: Tool creation ablation study.  
Confidence: High

#### Finding 6.2: Papers on AI Generating Adapter/Wrapper Code

Claim: There is no specific published research on "AI generating adapter/wrapper code" as a standalone topic. However, the Adapter Pattern in Go is well-documented as the standard approach for wrapping third-party SDKs[^418^][^427^].  
Source: Medium articles on Adapter Pattern  
URL: https://medium.com/design-bootcamp/understanding-the-adapter-design-pattern-in-go-a-practical-guide-a595b256a08b  
Date: 2025-10-13  
Excerpt: "The Adapter Pattern revolves around four main components: Client, Target Interface, Adapter, Adaptee."  
Context: Go adapter pattern tutorial.  
Confidence: Medium (no direct AI research found)

#### Finding 6.3: Anthropic Research on Tool Use and Code Generation

Claim: Anthropic engineers self-report 60% Claude usage and 50% productivity boost, with task complexity increasing from 3.2 to 3.8 and max consecutive tool calls increasing 116%[^421^].  
Source: Anthropic research blog  
URL: https://www.anthropic.com/research/how-ai-is-transforming-work-at-anthropic  
Date: 2025-12-02  
Excerpt: "Employees are tackling increasingly complex tasks with Claude Code. Task complexity increased from 3.2 to 3.8 on average. The maximum number of consecutive tool calls Claude Code makes per transcript increased by 116%."  
Context: Internal research on AI transformation at Anthropic.  
Confidence: High

Claim: Claude Code usage has shifted toward more autonomous coding tasks, with human turns decreasing 33% (6.2 → 4.1 per transcript)[^421^].  
Source: Anthropic research  
URL: https://www.anthropic.com/research/how-ai-is-transforming-work-at-anthropic  
Date: 2025-12-02  
Excerpt: "The number of human turns decreased by 33%. The average number of human turns decreased from 6.2 to 4.1 per transcript, suggesting that less human input is necessary to accomplish a given task now compared to six months ago."  
Context: Claude Code usage trend analysis.  
Confidence: High

---

## 3. Contradictions and Conflict Zones

### Conflict 1: Test-First vs. Reference-First
- **Test-First camp** (Endor Labs, TDD advocates): Write tests before implementation to catch hallucinations and enforce requirements[^362^][^364^].
- **Reference-First camp** (Speakeasy, SDK generators): Use language-specific templates and expert-crafted generators for consistent output[^405^].
- **Resolution**: For adapter generation (narrow scope, ~200 lines), test-first is better because the target interface IS the spec. For full SDK generation (broad scope), template-based generation is better.

### Conflict 2: Skill Portability vs. Platform-Specific Features
- **Portability camp** (agentskills.io standard): Skills should work across all platforms without modification[^459^].
- **Feature camp** (Claude Code): Platform-specific features like `context: fork`, `allowed-tools`, `agent` hooks provide better control[^438^].
- **Resolution**: Design the core skill to the open standard, with an optional `.claude/settings.json` hook configuration for Claude Code users who want automated validation.

### Conflict 3: Few-Shot Examples vs. Concise Instructions
- **Few-shot camp**: 2-3 examples provide the most reliable code generation[^419^].
- **Concise camp**: The Agent Skills spec recommends keeping `SKILL.md` under 5,000 tokens; too many examples bloat the skill[^459^].
- **Resolution**: Store few-shot examples in `assets/` directory and reference them in instructions: "Follow the pattern in `assets/docker-adapter.go`". This leverages progressive disclosure.

### Conflict 4: Automated Validation vs. Human Review
- **Automation camp**: Hooks can auto-validate without human intervention[^509^].
- **Human camp**: 26.1% of agent skills contain security vulnerabilities; human review remains essential[^510^].
- **Resolution**: Use automated gates for objective checks (compiles, passes tests, no forbidden imports) but require human approval for subjective quality (idiomatic Go, error handling patterns).

---

## 4. Gaps in Available Information

1. **No published benchmark for "AI generating Go adapter code" specifically**: SWE-bench tests general issue resolution, not adapter generation. We need to create our own benchmark with 10-20 adapter generation tasks.

2. **No standard "provider manifest" format**: The concept exists in Constitutional SDD and GitHub Spec Kit, but no standardized schema for "API spec + interface + constraints" exists. We would need to define this.

3. **Limited data on Go static analysis for AI-generated code**: While `go vet` and `staticcheck` are known tools, there are no published studies on their effectiveness specifically for AI-generated Go adapter code.

4. **Hook support varies across platforms**: The `allowed-tools` field is marked experimental in the Agent Skills spec[^398^]. Claude Code hooks are Claude-specific[^521^]. A portable validation strategy cannot rely solely on hooks.

5. **No research on "interface-driven code generation" accuracy**: How well do LLMs generate code that satisfies a specific Go interface when given an OpenAPI spec? This is a novel research question.

---

## 5. Preliminary Recommendations

### Recommendation 1: Skill Format (High Confidence)
Use the **Anthropic Agent Skills open standard** with the following structure:

```
substrate-adapter-generator/
├── SKILL.md                    # Core instructions
├── scripts/
│   ├── validate.sh             # Run go build + go vet + boundary check
│   └── generate-tests.sh       # Generate test scaffold from interface
├── references/
│   ├── generation-workflow.md  # Detailed 4-phase workflow
│   └── go-idioms.md            # Go-specific patterns for adapters
└── assets/
    ├── docker-adapter.go       # Few-shot example 1
    ├── aws-adapter.go          # Few-shot example 2
    └── adapter-scaffold.go     # Template with TODO stubs
```

### Recommendation 2: Input Format (High Confidence)
Use a **composite provider manifest** as the primary input:

```yaml
# substrate-provider-manifest.yaml
api_version: v1
openapi_spec: ./openapi.yaml
target_interface:
  package: substrate
  name: ContainerRuntime
  file: ./substrate/container.go
reference_adapters:
  - path: ./adapters/docker/adapter.go
    description: "Reference implementation for Docker SDK"
  - path: ./adapters/aws/adapter.go
    description: "Reference implementation for AWS SDK"
constraints:
  max_lines: 250
  max_methods_per_adapter: 20
  forbidden_imports:
    - "net/http"
    - "encoding/json"
    - "github.com/cenkalti/backoff"  # Use SDK's retry
  required_patterns:
    - "var _ substrate.ContainerRuntime = (*Adapter)(nil)"
    - "ctx context.Context"  # All methods must accept context
```

### Recommendation 3: Generation Workflow (High Confidence)
Enforce a **4-phase structured workflow** in the skill instructions:

```markdown
## Phase 1: Analyze (Structured CoT)
1. Read the OpenAPI spec and identify all operations
2. Read the target interface and list all required methods
3. Map each interface method to OpenAPI operations
4. Identify data type transformations needed
5. Note which methods may return errors

## Phase 2: Plan (Test-First)
1. Generate a test file with table-driven tests
2. Include tests for: happy path, error cases, context cancellation
3. Run tests to confirm they fail (RED phase)

## Phase 3: Write (Few-Shot Guided)
1. Read reference adapters from `assets/`
2. Generate the adapter following the exact pattern of references
3. Include compile-time interface check: `var _ Interface = (*Adapter)(nil)`
4. Keep file under 250 lines

## Phase 4: Verify (Validation Gates)
1. Run `go build` - must pass
2. Run `go vet` - must pass
3. Run `go test` - all tests must pass
4. Run boundary check script - must pass
5. If any gate fails, fix and re-verify
```

### Recommendation 4: Boundary Enforcement (High Confidence)
Use a **4-layer defense**:

| Layer | Implementation | Confidence |
|-------|---------------|------------|
| Description negatives | "Do NOT generate HTTP clients, serialization, retry logic" | High |
| Constitutional rules in body | "MUST only write mapping layer. MUST NOT exceed 250 lines." | High |
| Scaffold template | `assets/adapter-scaffold.go` with pre-defined structure | High |
| Validation script | `scripts/validate-boundary.sh` checks for forbidden imports | High |

### Recommendation 5: Validation Pipeline (High Confidence)
Implement this automated pipeline:

```bash
#!/bin/bash
# scripts/validate.sh

set -euo pipefail

echo "=== Gate 1: Compilation ==="
go build ./...

echo "=== Gate 2: Static Analysis ==="
go vet ./...
# staticcheck ./...  # if available

echo "=== Gate 3: Interface Satisfaction ==="
# Built into go build via var _ assertions
# Additional runtime check:
go test -run TestInterfaceSatisfaction ./...

echo "=== Gate 4: Unit Tests ==="
go test ./... -race -count=1

echo "=== Gate 5: Boundary Check ==="
if grep -r "net/http.Client" pkg/adapters/ 2>/dev/null; then
    echo "FAIL: Forbidden import net/http.Client found"
    exit 1
fi
if grep -r "json.Marshal" pkg/adapters/ 2>/dev/null; then
    echo "FAIL: Forbidden json.Marshal found"
    exit 1
fi

echo "=== All gates passed ==="
```

### Recommendation 6: Platform-Specific Enhancements (Medium Confidence)
For Claude Code users, add a `.claude/settings.json` hooks configuration:

```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "Edit|Write",
        "handler": {
          "type": "command",
          "command": "bash ${CLAUDE_SKILL_DIR}/scripts/validate.sh"
        }
      }
    ]
  }
}
```

---

## 6. Proposed Skill Spec: SubstrateAdapter Generator

### 6.1 Skill Directory Structure

```
substrate-adapter-generator/
├── SKILL.md
├── scripts/
│   ├── validate.sh
│   └── generate-tests.sh
├── references/
│   ├── generation-workflow.md
│   ├── go-idioms.md
│   └── error-handling-patterns.md
└── assets/
    ├── docker-adapter.go
    ├── aws-adapter.go
    └── adapter-scaffold.go
```

### 6.2 SKILL.md Content

```markdown
---
name: substrate-adapter-generator
description: >-
  Generate a Go SubstrateAdapter that maps a provider SDK to a Substrate interface.
  Use when asked to "generate an adapter", "create a substrate adapter",
  "wrap SDK for substrate", or "implement interface for provider".
  Do NOT use for: generating full SDKs, HTTP clients, database adapters,
  or code in languages other than Go.
license: Apache-2.0
compatibility: Requires Go 1.21+, go vet, and access to provider SDK.
metadata:
  author: mesh-research-team
  version: "1.0.0"
  domain: infrastructure-sdk-generation
---

# SubstrateAdapter Generator

## Purpose
Generate a Go adapter that implements a Substrate interface by delegating to a provider SDK.

## Scope Boundaries (CRITICAL - DO NOT VIOLATE)
- ONLY generate the mapping layer (typically 100-250 lines)
- NEVER generate: HTTP clients, JSON serialization, retry logic, auth handlers
- ALWAYS use the provider SDK for all API calls
- ALWAYS include compile-time interface check

## Phase 1: Analyze
1. Read the provider manifest (provider-manifest.yaml)
2. Read the OpenAPI spec to understand API operations
3. Read the target interface file
4. Map each interface method to SDK operations
5. Note required type conversions

## Phase 2: Plan (Test-First)
1. Generate test file with table-driven tests
2. Include: happy path, errors, context cancellation
3. Run tests to confirm they fail (RED)

## Phase 3: Write
1. Read reference adapters from assets/
2. Follow the exact pattern of the reference
3. Generate adapter with:
   - `var _ substrate.Interface = (*Adapter)(nil)`
   - Constructor: `NewAdapter(client *sdk.Client) *Adapter`
   - Context.Context on all methods
   - Error wrapping with fmt.Errorf
4. Keep under 250 lines

## Phase 4: Verify
1. Run `scripts/validate.sh`
2. If any gate fails, fix and re-run
3. Do not declare completion until all gates pass

## References
- See `references/generation-workflow.md` for detailed phase instructions
- See `references/go-idioms.md` for Go-specific patterns
- See `assets/docker-adapter.go` for a working reference
```

### 6.3 Inputs

| Input | Format | Required | Description |
|-------|--------|----------|-------------|
| Provider Manifest | YAML | Yes | Composite manifest with openapi_spec, target_interface, reference_adapters, constraints |
| OpenAPI Spec | JSON/YAML | Yes | Provider API specification |
| Target Interface | Go source | Yes | The Substrate interface to satisfy |
| Reference Adapters | Go source | No | 2-3 few-shot examples in assets/ |

### 6.4 Outputs

| Output | Format | Description |
|--------|--------|-------------|
| Adapter implementation | Go source | The generated adapter file |
| Test file | Go source | Table-driven tests for the adapter |
| Validation report | Text | Results of all 5 validation gates |

### 6.5 Validation Gates

| Gate | Command | Pass Criteria |
|------|---------|---------------|
| 1. Compilation | `go build ./...` | Zero errors |
| 2. Static Analysis | `go vet ./...` | Zero warnings |
| 3. Interface Satisfaction | `var _ Interface = (*Adapter)(nil)` | Compile-time assertion present |
| 4. Unit Tests | `go test ./... -race` | 100% pass |
| 5. Boundary Check | `scripts/validate-boundary.sh` | No forbidden imports |

---

## 7. Citation Index

[^135^]: Speakeasy blog - Agent skills for OpenAPI and SDK generation. https://www.speakeasy.com/blog/release-agent-skills  
[^137^]: GitHub speakeasy-api/skills - Agent Skills repository. https://github.com/speakeasy-api/skills  
[^314^]: MindStudio blog - Claude Code Skills vs Slash Commands. https://www.mindstudio.ai/blog/claude-code-skills-vs-slash-commands/  
[^315^]: GitHub Docs - Copilot customization cheat sheet. https://docs.github.com/en/copilot/reference/customization-cheat-sheet  
[^316^]: OpenCode docs - Agent Skills. https://opencode.ai/docs/skills/  
[^317^]: Batsov blog - Essential Claude Code Skills and Commands. https://batsov.com/articles/2026/03/11/essential-claude-code-skills-and-commands/  
[^319^]: GitHub Docs - Adding custom instructions for Copilot. https://docs.github.com/copilot/customizing-copilot/adding-custom-instructions-for-github-copilot  
[^320^]: dotcursorrules.com - Cursor rules examples. https://dotcursorrules.com/  
[^323^]: DataCamp - Cursor Rules tutorial. https://www.datacamp.com/tutorial/cursor-rules  
[^362^]: Endor Labs - Test-First Prompting. https://www.endorlabs.com/learn/test-first-prompting-using-tdd-for-secure-ai-generated-code  
[^363^]: PromptHub - Chain of Thought Prompting Guide. https://www.prompthub.us/blog/chain-of-thought-prompting-guide  
[^364^]: Momentic blog - How AI Will Bring TDD Back. https://momentic.ai/blog/test-driven-development  
[^365^]: Dev.to - TDD with Claude Code. https://dev.to/myougatheaxo/test-driven-development-with-claude-code-write-tests-first-then-make-them-pass-2a6m  
[^366^]: ACM TOSEM - Structured Chain-of-Thought Prompting for Code Generation. https://dl.acm.org/doi/10.1145/3690635  
[^369^]: arXiv - Structured Chain-of-Thought Prompting for Code Generation. https://arxiv.org/abs/2305.06599  
[^377^]: Firecrawl blog - Agent Skills Explained. https://www.firecrawl.dev/blog/agent-skills  
[^379^]: Hopx AI - Validate AI Code. https://hopx.ai/use-cases/validate-ai-code/  
[^380^]: Bitloops - Architectural Constraints for AI Agents. https://bitloops.com/resources/governance/architectural-constraints-for-ai-agents  
[^383^]: JetBrains Qodana - Code Analysis for AI-generated Code. https://www.jetbrains.com/pages/static-code-analysis-guide/code-analysis-for-ai-generated-code/  
[^384^]: Anthropic blog - Equipping agents for the real world with Agent Skills. https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills  
[^385^]: AppSecEngineer - Why Static Analysis Fails on AI-Generated Code. https://www.appsecengineer.com/blog/why-static-analysis-fails-on-ai-generated-code  
[^398^]: Medium - Deep Dive SKILL.md. https://abvijaykumar.medium.com/deep-dive-skill-md-part-1-2-09fc9a536996  
[^399^]: Ylang Labs - A Portable Format for Teaching AI Agents. https://ylanglabs.com/blogs/agent-skills  
[^405^]: Speakeasy docs - Generate SDKs from OpenAPI. https://www.speakeasy.com/docs/sdks/create-client-sdks  
[^417^]: QuillBot - Few Shot Prompting. https://quillbot.com/blog/ai-prompt-writing/few-shot-prompting/  
[^418^]: Medium - Adapter Design Pattern in Go. https://medium.com/design-bootcamp/understanding-the-adapter-design-pattern-in-go-a-practical-guide-a595b256a08b  
[^419^]: PromptHub - The Few Shot Prompting Guide. https://www.prompthub.us/blog/the-few-shot-prompting-guide  
[^421^]: Anthropic research - How AI Is Transforming Work at Anthropic. https://www.anthropic.com/research/how-ai-is-transforming-work-at-anthropic  
[^422^]: Dev.to - Checking if a Type Satisfies an Interface in Go. https://dev.to/kittipat1413/checking-if-a-type-satisfies-an-interface-in-go-432n  
[^424^]: Eric Apisani blog - Avoiding Go runtime errors with interface compliance. https://www.ericapisani.dev/avoiding-go-runtime-errors-with-interface-compliance-and-type-assertion-checks/  
[^427^]: Medium - Adapter Pattern in Go: Real-World Examples. https://medium.com/@itz.aman.av/adapter-pattern-in-go-real-world-idiomatic-examples-6bfd289d0394  
[^428^]: Stack Overflow - Ensure a type implements an interface at compile time in Go. https://stackoverflow.com/questions/10498547/ensure-a-type-implements-an-interface-at-compile-time-in-go  
[^430^]: GitHub uber-go/guide - Interface compliance recommendation. https://github.com/uber-go/guide/issues/25  
[^431^]: Medium - Ensuring Go interface satisfaction at compile-time. https://medium.com/stupid-gopher-tricks/ensuring-go-interface-satisfaction-at-compile-time-1ed158e8fa17  
[^435^]: Scale Labs - SWE-Bench Pro. https://labs.scale.com/leaderboard/swe_bench_pro_public  
[^436^]: arXiv - Can Software Engineering Agents Self-Evolve on the Fly? https://arxiv.org/html/2511.13646v1  
[^438^]: Claude Code Docs - Extend Claude with skills. https://code.claude.com/docs/en/skills  
[^459^]: agentskills.io - Agent Skills Specification. https://agentskills.io/specification  
[^491^]: SWE-bench Multilingual results. https://www.swebench.com/multilingual.html  
[^502^]: Dev.to - Generating OpenAI SDK with Stainless & Speakeasy. https://dev.to/andy_tate_/generating-building-the-openai-sdk-with-stainless-speakeasy-32nb  
[^509^]: Pixelmojo - Claude Code Hooks: All 12 Events. https://www.pixelmojo.io/blogs/claude-code-hooks-production-quality-ci-cd-patterns  
[^510^]: arXiv - Constitutional Spec-Driven Development. https://arxiv.org/html/2602.02584v1  
[^511^]: arXiv abstract - Constitutional Spec-Driven Development. https://arxiv.org/abs/2602.02584  
[^512^]: Claudefa.st - Claude Code Hooks Complete Guide. https://claudefa.st/blog/tools/hooks/hooks-guide  
[^518^]: GitHub spec-kit - Spec-Driven Development. https://github.com/github/spec-kit/blob/main/spec-driven.md  
[^521^]: Claude Code Docs - Automate workflows with hooks. https://code.claude.com/docs/en/hooks-guide  
[^523^]: Claude Code Docs - Hooks reference. https://code.claude.com/docs/en/hooks  
