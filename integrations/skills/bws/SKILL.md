---
name: bws
description: >-
  Dispatch autonomous coding agent tasks into an isolated, disposable Bubblewrap
  Git sandbox using `bws gw` (and `agy-run-wild`). Trigger whenever the user asks
  to run wild, use `/rw`, run in sandbox, dispatch an autonomous agent task,
  or perform isolated refactors/testing.
---

# `bws` Disposable Agent Sandbox Skill

This skill allows AI coding assistants (such as Google Antigravity and OpenCode) to delegate complex, autonomous coding tasks to a sandboxed child agent running in an ephemeral Git clone with air-gapped SSH (`--no-ssh`), automatic commit on exit, automated post-run code review, and 1-key merge triage.

---

## When to Use This Skill

* The user types `/rw`, asks to "run wild", "run in sandbox", "use bws", or "dispatch an agent".
* A task involves untrusted code, deep refactors, or automated test loops where host `$HOME` protection and Git branch isolation are desirable.
* The user wants to run long-running or autonomous tasks without interactive confirmation prompts.

---

## Execution Procedure

### 1. Launch the Sandboxed Agent
Run the task in the background using `run_command` (with `bws gw` and `agy-run-wild`):

```bash
bws gw -b <branch-name> --stash -- agy-run-wild --effort high -p "/goal <clear actionable task description>"
```

### 2. Review the Generated Code
When the sandboxed agent completes, `bws gw` auto-commits the changes, fetches the branch back to the host, and pauses at the triage prompt. Before prompting the user:
1. Inspect the diff and commit history on the fetched branch (`git diff <base>..<branch>`).
2. Verify correctness, edge cases, test coverage, and project guidelines.
3. Formulate a definitive assessment and triage recommendation.

### 3. Report & Recommend Triage Action
Present the summary and explicit recommendation to the user before displaying the menu:
* **Summary of Changes**: Bulleted breakdown of what was modified and tested.
* **Code Review Assessment**: Verification findings (tests passed, edge cases handled, line limits respected).
* **Clear Recommendation**: Explicitly state the recommended action (e.g. **Recommendation: `[s] Squash-merge`** because all changes form a coherent single unit of work).
* **Triage Menu Options**:
  - `[m] Merge`: Fast-forward or merge branch into current branch.
  - `[s] Squash-merge`: Merge as a single commit into current branch.
  - `[k] Keep`: Preserve the branch for manual inspection without merging.
  - `[d] Discard`: Delete the branch.
  - `[v] View`: Open full diff in pager.

### 4. Execute Chosen Triage
Send the user's chosen triage input (e.g. `s\n` or `m\n`) to the background task using `manage_task(send_input)`.
