# TODOS

## Evaluate CGo zstd for v1 performance

**What:** Evaluate `github.com/DataDog/zstd` (CGo bindings to libzstd) as an alternative to `github.com/klauspost/compress/zstd` (pure Go) for snapshot compression in v1.

**Why:** Pure Go zstd is ~20-30% slower than native C zstd for large files. For a 5GB agent snapshot, that's ~10-15s difference. The delta grows with agent size. v0 uses pure Go to keep the single-binary build simple. v1 can evaluate whether the performance gain justifies the CGo build complexity.

**Pros:** Faster snapshots for large agents. Better compression ratios possible with higher levels.

**Cons:** CGo breaks cross-compilation simplicity (`GOOS=linux GOARCH=arm64 go build` no longer just works). Requires libzstd dev headers on build machine. Complicates the `curl | bash` install story.

**Context:** v0 chose pure Go zstd (klauspost/compress) specifically to avoid external dependencies and keep the single-binary distribution model clean. The performance cost is acceptable for v0 (single user, weekend build). This TODO exists to re-evaluate after v0 ships with real-world usage data.

**Depends on:** v0 shipping first. Benchmark real-world agent snapshot sizes and compression times before deciding.

**Created:** 2026-04-23 by /plan-eng-review

## .meshignore file for snapshot exclusion

**What:** A `.meshignore` file (like `.gitignore`) that lets users exclude patterns from snapshots. Place in agent workdir. Patterns like `node_modules/`, `.cache/`, `__pycache__/` are excluded from the tarball.

**Why:** A 5GB agent directory often contains 2GB of caches and dependencies that are rebuildable. Excluding them shrinks snapshots by 30-50%, speeds up tar + transfer, and reduces cache usage.

**Pros:** Smaller snapshots. Faster transfers. Less disk usage in snapshot cache. First customization users will want.

**Cons:** Pattern matching edge cases (glob vs regex, nested dirs, negation patterns). Adds complexity to the tar walk in the snapshot package.

**Context:** Considered for v0 scope during CEO review. Deferred to prove the core snapshot primitive first. The Go standard library has `filepath.Match` and `filepath.Glob` which provide basic glob matching. More sophisticated pattern matching (like `.gitignore` semantics) would require a dedicated parser.

**Effort:** S (human: ~1hr / CC: ~10min)

**Priority:** P2

**Depends on:** v0 core snapshot working first.

**Created:** 2026-04-23 by /plan-ceo-review
