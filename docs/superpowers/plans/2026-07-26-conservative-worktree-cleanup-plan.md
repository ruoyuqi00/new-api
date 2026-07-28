# Conservative Worktree Cleanup Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Reclaim local disk space by removing only three clean, fully merged worktrees while preserving every dirty or unaudited branch.

**Architecture:** Treat cleanup as a proof-driven Git operation: verify status and ancestry immediately before each removal, use non-forced `git worktree remove`, delete only merged local branches with `git branch -d`, then re-list preserved worktrees. No runtime directory, Docker object, remote branch, production file, or dirty worktree is touched.

**Tech Stack:** Git worktrees, PowerShell, Windows filesystem

---

## Fixed Scope

Remove only:

```text
D:\yucore-protocol-billing  -> codex/protocol-billing-contracts-20260724
D:\yucore-provider-perf     -> codex/provider-account-latency-20260714
D:\yucore-ui-baseline       -> detached HEAD 0945c9cc3
```

Preserve:

```text
D:\newapi-710-yuapi
D:\yucore-api-export
D:\yucore-dual-ui
D:\yucore-local-production
D:\yucore-ui
D:\yucore-newapi-fusion
D:\yucore-parent-reliability
D:\yucore-local-production-runtime
```

### Task 1: Re-prove the cleanup boundary

**Files:**
- Git metadata only; no repository file edits.

- [ ] **Step 1: Confirm the target branch and preserve its dirty UI work**

Run from `D:\yucore-local-production`:

```powershell
git branch --show-current
git status --short
```

Expected branch: `codex/local-production-brand-performance-20260725`.

Expected status includes the intentional `yucore-home.tsx` modification and untracked `yucore-home-details.tsx` until the UI plan commits them. Cleanup must continue without staging, stashing, resetting, or deleting those files.

- [ ] **Step 2: Confirm all three candidates are clean and have zero unique commits**

Run in each candidate:

```powershell
git status --porcelain
git rev-list --left-right --count codex/local-production-brand-performance-20260725...HEAD
```

The audit before this plan was committed observed `20 0`, `86 0`, and `93 0` respectively. Later target-branch commits legitimately increase only the left-hand number. At execution time the required result is:

```text
D:\yucore-protocol-billing  status empty, right-hand count 0
D:\yucore-provider-perf     status empty, right-hand count 0
D:\yucore-ui-baseline       status empty, right-hand count 0
```

If any status is non-empty or the right-hand count is nonzero, stop and preserve that candidate; do not use `--force`.

- [ ] **Step 3: Confirm unaudited worktrees still contain unique commits**

Run:

```powershell
git -C D:\yucore-newapi-fusion rev-list --left-right --count codex/local-production-brand-performance-20260725...HEAD
git -C D:\yucore-parent-reliability rev-list --left-right --count codex/local-production-brand-performance-20260725...HEAD
```

The pre-plan audit observed fusion right-hand count `55` and parent-reliability right-hand count `6`. The required execution-time condition is that each right-hand count remains greater than zero; both worktrees remain regardless of their exact count.

### Task 2: Remove the merged protocol-billing worktree

**Files:**
- Remove worktree directory: `D:\yucore-protocol-billing`
- Delete local branch: `codex/protocol-billing-contracts-20260724`

- [ ] **Step 1: Ask Git to remove the clean worktree without force**

Run from `D:\yucore-local-production`:

```powershell
git worktree remove D:\yucore-protocol-billing
```

Expected: exit 0. If Git reports local changes or a lock, stop; do not retry with `--force` and do not use `Remove-Item`.

- [ ] **Step 2: Delete only the now-merged local branch**

Run: `git branch -d codex/protocol-billing-contracts-20260724`

Expected: Git reports the branch deleted. Do not delete `ruoyu/codex/protocol-billing-contracts-20260724` or any other remote-tracking reference.

- [ ] **Step 3: Verify path and registration are gone**

Run:

```powershell
Test-Path -LiteralPath D:\yucore-protocol-billing
git worktree list --porcelain
```

Expected: `False` and no worktree stanza for that path.

### Task 3: Remove the merged provider-performance worktree

**Files:**
- Remove worktree directory: `D:\yucore-provider-perf`
- Delete local branch: `codex/provider-account-latency-20260714`

- [ ] **Step 1: Remove the clean worktree without force**

Run: `git worktree remove D:\yucore-provider-perf`

Expected: exit 0; otherwise stop without filesystem deletion.

- [ ] **Step 2: Delete the merged local branch**

Run: `git branch -d codex/provider-account-latency-20260714`

Expected: local branch deleted; remote-tracking refs remain.

- [ ] **Step 3: Verify removal**

Run:

```powershell
Test-Path -LiteralPath D:\yucore-provider-perf
git worktree list --porcelain
```

Expected: `False` and no matching worktree stanza.

### Task 4: Remove the detached UI baseline worktree

**Files:**
- Remove worktree directory: `D:\yucore-ui-baseline`
- No branch deletion; it is detached at `0945c9cc3`.

- [ ] **Step 1: Confirm detached state one final time**

Run: `git -C D:\yucore-ui-baseline status --short --branch`

Expected: `## HEAD (no branch)` and no changed files.

- [ ] **Step 2: Remove through Git without force**

Run: `git worktree remove D:\yucore-ui-baseline`

Expected: exit 0. There is no corresponding local branch to delete.

- [ ] **Step 3: Verify removal**

Run: `Test-Path -LiteralPath D:\yucore-ui-baseline`

Expected: `False`.

### Task 5: Prune metadata and prove preserved state

**Files:**
- Git worktree metadata only.

- [ ] **Step 1: Prune stale administrative entries**

Run: `git worktree prune --verbose`

Expected: exit 0; it may print nothing when removal already cleaned metadata.

- [ ] **Step 2: List all remaining worktrees**

Run: `git worktree list --porcelain`

Expected paths include all seven preserved repository worktrees and exclude only the three approved candidates.

- [ ] **Step 3: Recheck every preserved dirty worktree without modifying it**

Run:

```powershell
git -C D:\newapi-710-yuapi status --short --branch
git -C D:\yucore-api-export status --short --branch
git -C D:\yucore-dual-ui status --short --branch
git -C D:\yucore-local-production status --short --branch
git -C D:\yucore-ui status --short --branch
```

Expected: each original status is intact; no stash, reset, checkout, clean, or deletion occurred.

- [ ] **Step 4: Confirm runtime and production were outside cleanup scope**

Run:

```powershell
Test-Path -LiteralPath D:\yucore-local-production-runtime\new-api-preview.exe
Test-Path -LiteralPath D:\yucore-local-production-runtime\one-api.db
```

Expected: both `True`. Do not delete `D:\yucore-newapi-fusion-runtime` in this plan, do not touch Docker, and do not connect to the production server.

- [ ] **Step 5: Record the cleanup result without creating a code commit**

Report the three removed paths and two deleted local branches. Cleanup changes Git administrative state and disk usage only; it does not require a repository commit, push, PR, or deployment.
