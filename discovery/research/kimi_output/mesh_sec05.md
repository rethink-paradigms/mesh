## 5. Agent Skill Specification

### 5.1 Input Format

#### 5.1.1 Composite Provider Manifest

The SubstrateAdapter Generator accepts a single structured manifest rather than raw prompts. Speakeasy's production SDK generation validates this pattern: parse an OpenAPI specification, apply language-specific templates, and emit idiomatic code from a unified input set[^405^][^502^]. The Mesh manifest layers four inputs into one document:

```yaml
# provider-manifest.yaml
api_version: v1
openapi_spec: ./openapi.yaml
target_interface:
  package: substrate
  name: ContainerRuntime
  file: ./substrate/container.go
reference_adapters:
  - path: ./adapters/docker/adapter.go
    description: "Docker SDK mapping reference"
  - path: ./adapters/hetzner/adapter.go
    description: "Hetzner hcloud-go mapping reference"
constraints:
  max_lines: 250
  forbidden_imports: ["net/http", "encoding/json", "github.com/cenkalti/backoff"]
  required_patterns:
    - "var _ substrate.ContainerRuntime = (*Adapter)(nil)"
    - "ctx context.Context"
```

This format aligns with the Constitutional Spec-Driven Development model, where the manifest acts as the "constitution" governing downstream artifacts. A banking microservices case study found that embedding constitutional constraints in the specification layer reduces security defects by 73% compared with unconstrained generation[^510^]. GitHub's Spec Kit formalizes the same hierarchy: a Constitution stage encodes naming conventions, layering principles, and allowed or forbidden libraries before any code is generated[^518^].

Because Go uses implicit interface satisfaction (no `implements` keyword), the agent must receive both the interface definition and the OpenAPI spec to map method signatures to SDK calls correctly[^497^]. Without both, it hallucinates parameter mappings or invents nonexistent types.

#### 5.1.2 Constitutional Boundaries

The manifest's `constraints` section encodes hard limits the agent must not violate. Cursor's rules documentation confirms that constraint-based language outperforms soft guidance: "Functions must be under 30 lines" produces better compliance than "try to keep functions small"[^323^]. For Mesh, the constraints use RFC 2119 enforcement levels per the constitutional SDD model[^510^]:

| Constraint | Value | Level |
|---|---|---|
| Maximum file lines | 250 | MUST — mapping layer only |
| Forbidden imports | `net/http`, `encoding/json`, `backoff` | MUST — use SDK transport |
| Required patterns | Compile-time interface check, `context.Context` | MUST — every adapter |
| Maximum methods | 20 | SHOULD — split oversized interfaces |

---

### 5.2 Skill Structure

#### 5.2.1 Anthropic Agent Skills Standard

The skill follows the `agentskills.io` open standard, a portable `SKILL.md` format adopted by 30+ platforms including Claude Code, GitHub Copilot, Cursor, OpenCode, and Gemini CLI[^459^][^377^]. A skill is a directory containing a `SKILL.md` file with YAML frontmatter followed by Markdown content[^459^].

**Steal this**: Speakeasy publishes 21 focused skills — one per language, one for diagnostics — rather than a single broad meta-skill, following the principle that "focused, use-case-specific skills outperform broad meta-skills"[^135^][^137^]. Mesh ships one skill: `substrate-adapter-generator`, triggered when the user asks to "generate an adapter" or "implement interface for provider".

**Avoid this**: Platform-specific extensions like Claude Code's experimental `allowed-tools` field[^398^][^438^]. These break portability. The core skill should use only standardized frontmatter keys, with an optional `.claude/settings.json` hook file for users who want platform-specific automation.

#### 5.2.2 Progressive Disclosure

The standard's core design principle is progressive disclosure: metadata (~100 tokens) loads at startup, full instructions load when triggered, and references or scripts load on demand[^384^][^399^]. This keeps context windows manageable while enabling deep expertise when needed.

| Layer | Content | Tokens (approx.) | Load Trigger |
|---|---|---|---|
| Metadata | Name, description, negative triggers | ~100 | Always in system prompt |
| Instructions | 4-phase workflow, boundary rules | <5,000 | Skill activation |
| References | Go idioms, error handling patterns | On demand | Agent `Read` tool |
| Assets | 2-3 reference adapter files | On demand | Write phase only |
| Scripts | `validate.sh`, `generate-tests.sh` | On demand | Verify phase only |

This structure resolves the tension between few-shot examples and concise instructions: examples live in `assets/` and are read only during the Write phase, not bloating the initial context[^419^].

---

### 5.3 Reliability Patterns

#### 5.3.1 Four-Phase Workflow

The skill enforces a structured workflow derived from three validated patterns: Structured Chain-of-Thought (SCoT), test-first generation, and few-shot prompting. SCoT — which asks the model to reason through program structures (sequence, branch, loop) before writing code — outperforms standard chain-of-thought by up to 13.79% on HumanEval, MBPP, and MBCPP[^366^][^369^]. Test-first prompting forces upfront threat modeling and catches hallucinations like negation bugs[^362^]. Generating tests and code in a single shot produces tests that match the implementation rather than the requirements, missing edge cases and testing the wrong behavior[^365^].

The 4-phase workflow synthesizes these findings:

| Phase | Activity | Artifact | Source Pattern |
|---|---|---|---|
| 1. Analyze | Map interface methods to OpenAPI operations, identify type conversions | Mapping document | SCoT reasoning[^366^] |
| 2. Plan | Write table-driven tests for happy path, errors, context cancellation | `*_test.go` (failing) | Test-first TDD[^362^] |
| 3. Write | Read 2-3 reference adapters from `assets/`, generate mapping layer | `adapter.go` (~200 lines) | Few-shot guidance[^419^] |
| 4. Verify | Run `go build`, `go vet`, tests, boundary check; fix and re-verify | Validation report | Automated gates[^509^] |

The Plan phase is critical: generating a failing test first (the RED phase in classic TDD) gives the agent an objective target. When the agent later runs tests in Verify, passing tests confirm that the code satisfies the interface contract, not merely that it compiles[^364^][^365^].

#### 5.3.2 Few-Shot Sweet Spot

Research shows diminishing returns after 2-3 few-shot examples[^419^][^417^]. The optimal reference set is 2 adapters of different complexity: one minimal (Hetzner, ~120 lines) and one moderate (Docker, ~200 lines). The skill instructs the agent to "Follow the pattern in `assets/docker-adapter.go` and `assets/hetzner-adapter.go`" during the Write phase. More than 3 examples burn tokens without improving reliability and risk confusing the agent when patterns diverge[^419^].

---

### 5.4 Boundary Enforcement

#### 5.4.1 Agent Must Not Generate

The adapter's scope is strictly the mapping layer between the Substrate interface and the provider SDK. The agent is forbidden from generating infrastructure the SDK already handles: HTTP client construction (`net/http.Client`), JSON serialization (`json.Marshal`), retry logic with backoff, and authentication handlers. These are architectural boundaries, not stylistic preferences. If the agent generates an HTTP client, it duplicates battle-tested SDK functionality and introduces bugs in TLS, connection pooling, and header injection.

#### 5.4.2 Agent Only Writes Mapping Layer

The generated file is expected to be 100-250 lines. This is enforceable because the agent delegates every operation to an existing SDK method. A typical adapter contains: a struct wrapping the SDK client (1-5 lines), a constructor (3-5 lines), the compile-time interface check (1 line), and 8-15 method implementations (each 5-15 lines of parameter mapping and error wrapping).

#### 5.4.3 Four-Layer Defense

Boundary enforcement cannot rely on a single mechanism. The skill uses four overlapping layers, each catching violations the others miss:

| Layer | Mechanism | Example |
|---|---|---|
| 1. Description negatives | Negative triggers in frontmatter | "Do NOT generate HTTP clients, serialization, retry logic"[^398^] |
| 2. Constitutional rules | MUST/SHALL rules in skill body | "MUST only generate mapping layer. MUST NOT exceed 250 lines."[^510^] |
| 3. Scaffold template | Pre-defined structure in `assets/` | `adapter-scaffold.go` with struct and TODO stubs |
| 4. Validation script | Post-generation check | `validate-boundary.sh` greps for forbidden imports |

This defense-in-depth is necessary because 26.1% of agent skills in the wild contain security vulnerabilities, and automated generation without multi-layer enforcement compounds that risk[^510^].

---

### 5.5 Validation Gates

#### 5.5.1 Five-Gate Pipeline

Every generated adapter must pass a sequential validation pipeline. The gates progress from objective to subjective, with earlier gates filtering cheap failures before expensive tests run:

| Gate | Command / Check | Pass Criteria |
|---|---|---|
| 1. Compilation | `go build ./...` | Zero errors; all imports resolve |
| 2. Static Analysis | `go vet ./...` | Zero warnings |
| 3. Interface Satisfaction | `var _ Interface = (*Adapter)(nil)` compiles | Compile-time assertion proves satisfaction[^422^][^428^] |
| 4. Unit Tests | `go test ./... -race -count=1` | 100% pass; race detector clean |
| 5. Boundary Check | `scripts/validate-boundary.sh` | No forbidden imports (`net/http`, `encoding/json`) |

The interface satisfaction gate warrants emphasis. The Go idiom `var _ Interface = (*Type)(nil)` forces the compiler to verify that the adapter implements every method with zero runtime cost[^422^][^424^][^428^]. The Stack Overflow Go FAQ explicitly recommends this pattern for compile-time checks[^428^], and the Uber Go Style Guide maintainer endorsed it for interface compliance[^430^]. Static analysis tools are particularly valuable for AI-generated code because they catch complexity, duplication, and performance issues that compilation misses[^383^][^385^].

#### 5.5.2 Automation via Hooks or CI

The pipeline can be automated through two mechanisms. For Claude Code users, `PostToolUse` hooks run `scripts/validate.sh` after every file edit, and `Stop` hooks verify all tests pass before the agent finishes a session[^509^][^521^][^523^]. For CI, the same script runs as a pull-request check. The skill should ship both: hook configuration for interactive development, and `scripts/validate.sh` for CI.

**Steal this**: The `Stop` hook with an agent-based handler that spawns a sub-agent to run the test suite and check results[^523^]. This creates a "meta-validation" layer where one agent verifies another's output.

---

### 5.6 AI Code Generation Research

#### 5.6.1 Benchmarks for Go Code Generation

Published benchmarks provide a baseline, but they measure general issue resolution rather than narrow adapter generation. On SWE-bench Multilingual, Go has a 30.95% resolution rate across 42 tasks[^491^]. On SWE-bench Pro, Go and Python show higher resolve rates than JavaScript/TypeScript[^435^]. Claude 4.5 Sonnet achieves 75.4% on SWE-bench Verified with tool creation[^436^], and Live-SWE-agent reaches 46.0% on SWE-bench Multilingual by creating custom tools on the fly[^436^].

These numbers are informative but not directly applicable. SWE-bench tasks involve reading large codebases and producing multi-file patches. The SubstrateAdapter task is narrower: read a manifest, a spec, and an interface, then write a single ~200-line file. The expected success rate should be materially higher than the 30.95% baseline — but no published benchmark measures Go interface implementation tasks specifically. This gap means Mesh should build a custom benchmark of 10-20 adapter generation tasks, scored on compilation success, interface satisfaction, test pass rate, and boundary compliance. That benchmark becomes the skill's regression suite. **Confidence: Medium**.

#### 5.6.2 Internal Adoption Trends

Anthropic's internal research on Claude Code reveals trends relevant to agent-based code generation. Employees self-report 60% Claude usage and a 50% productivity boost. Task complexity increased from 3.2 to 3.8, while maximum consecutive tool calls per transcript increased by 116%[^421^]. Human turns decreased by 33% (6.2 to 4.1 per transcript), suggesting agents require less intervention over time[^421^]. These trends support the hypothesis that a well-specified skill with clear validation gates can operate with minimal human oversight, provided the task scope is narrow and the gates are objective.
