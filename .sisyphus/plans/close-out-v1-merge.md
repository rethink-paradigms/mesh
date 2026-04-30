# Close Out v1.0 Merge

## TL;DR

> **Quick Summary**: Commit Q1-Q3 governance fixes + F1-F3 evidence, push mesh-v1-implementation branch and main + tag to GitHub.
>
> **Deliverables**:
> - mesh-impl: clean commit with evidence + DB fixes
> - mesh: main + v1.0.0 tag pushed to GitHub
>
> **Estimated Effort**: Quick (2 sequential tasks)

---

## Context

All v1.0 merge tasks complete. Q1-Q3 resolutions fixed (phantom DE citations removed). F1-F3 evidence files produced but untracked. Just need to commit and push.

---

## TODOs

- [ ] 1. Commit mesh-impl changes

  **What to do**:
  - Stage: `.mesh/governance.db`, `.sisyphus/plans/v1-merge-to-main.md`, all untracked evidence files (`.sisyphus/evidence/task-10*.txt`, `task-11*.txt`, `task-12*.txt`, `task-13*.txt`, `f1-*.md`, `f2-*.txt`, `f3-*.txt`)
  - Commit: `chore: fix Q1-Q3 resolutions (remove phantom DE citations), add F1-F3 verification evidence`
  - Verify: `git status` clean

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Sequential | Blocks: Task 2

  **QA Scenarios**:
  ```
  Scenario: Mesh-impl working tree clean after commit
    Tool: Bash (workdir: mesh-impl)
    Steps:
      1. git add .mesh/governance.db .sisyphus/plans/v1-merge-to-main.md .sisyphus/evidence/f*.md .sisyphus/evidence/f*.txt .sisyphus/evidence/task-1*.txt
      2. git commit -m "chore: fix Q1-Q3 resolutions (remove phantom DE citations), add F1-F3 verification evidence"
      3. git status → clean
    Evidence: .sisyphus/evidence/close-out-commit.txt
  ```

  **Commit**: YES — `chore: fix Q1-Q3 resolutions (remove phantom DE citations), add F1-F3 verification evidence`

- [ ] 2. Push to GitHub

  **What to do**:
  - In mesh-impl worktree: `git push origin mesh-v1-implementation`
  - In mesh worktree: `git push origin main --tags`
  - Verify: `git ls-remote origin main` shows same SHA, `v1.0.0` tag visible

  **Must NOT do**: Do NOT force push.

  **Recommended Agent Profile**: `quick`
  **Parallelization**: Sequential | Blocked By: Task 1

  **QA Scenarios**:
  ```
  Scenario: Both branches and tag pushed to GitHub
    Tool: Bash
    Steps:
      1. cd mesh-impl && git push origin mesh-v1-implementation
      2. cd mesh && git push origin main --tags
      3. git ls-remote origin refs/heads/main | awk '{print $1}'
      4. git ls-remote origin refs/tags/v1.0.0
    Evidence: .sisyphus/evidence/close-out-push.txt
  ```

  **Commit**: NO

---

## Success Criteria
- [ ] mesh-impl working tree clean
- [ ] mesh-v1-implementation pushed to GitHub
- [ ] main pushed to GitHub
- [ ] v1.0.0 tag on GitHub
