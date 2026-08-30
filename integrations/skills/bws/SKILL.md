---
name: bws
description: >-
  Dispatch autonomous coding agent tasks into an isolated, disposable Bubblewrap
  Git sandbox using `bws gw` (and `agy-run-wild`). Use whenever the user asks to run an
  agent in a sandbox, dispatch an autonomous task without risking the host repo,
  or perform red-team/security audits.
---

# `bws` Disposable Agent Sandbox Skill

This skill allows AI coding assistants (such as Google Antigravity and OpenCode) to delegate complex, autonomous coding tasks to a sandboxed child agent running in an ephemeral Git clone with air-gapped SSH (`--no-ssh`), automatic commit on exit, and 1-key merge triage.

---

## When to Use This Skill

* The user asks to "run in a sandbox", "use bws", "run wild", or "dispatch an agent".
* A task involves untrusted code, deep refactors, or automated test loops where host `$HOME` protection and Git branch isolation are desirable.
* The user wants to run long-running or autonomous tasks without interactive confirmation prompts.

---

## Execution Procedure

### 1. Pre-flight Check
Verify that the host repository is clean:
```bash
git status --porcelain
```
If dirty, either commit first or pass `--stash` to `bws gw`.

### 2. Launch the Sandboxed Agent
Run the task in the background using `run_command` (with `bws gw` and `agy-run-wild`):

```bash
bws gw -b <branch-name> --stash -- agy-run-wild --effort high -p "/goal <clear actionable task description>"
```

### 3. Monitor & Report
* Check the background task log (`.system_generated/tasks/task-XXXX.log`).
* When the agent completes, `bws gw` will auto-commit the changes, fetch the branch back to the host, and display the diff summary with the triage prompt:
  - `[m] Merge`: Fast-forward/merge into current branch.
  - `[s] Squash`: Squash all agent commits into one.
  - `[k] Keep`: Preserve the branch for manual inspection.
  - `[d] Discard`: Delete the branch.
  - `[v] View`: View full diff.
* Present the results to the user and send their chosen triage action to the task using `manage_task(send_input)`.
