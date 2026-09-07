---
name: bws
description: >-
  Dispatch autonomous coding agent tasks into an isolated, disposable Bubblewrap
  Git sandbox using `bws gw` (and `agy-run-wild`). Trigger whenever the user asks
  to run wild, use `/rw`, run in sandbox, dispatch an autonomous agent task,
  or perform isolated refactors/testing.
---

# `bws` disposable agent sandbox skill

This skill allows AI coding assistants (such as Google Antigravity and OpenCode) to delegate complex, autonomous coding tasks to a sandboxed child agent running in an ephemeral Git clone with air-gapped isolation (`--unshare-pid`, `--unshare-ipc`, `--new-session`, `--no-ssh`), automatic commit on exit, automated post-run code review, and 1-key merge triage.

---

## When to use this skill

* The user types `/rw`, asks to "run wild", "run in sandbox", "use bws", or "dispatch an agent".
* A task involves untrusted code, deep refactors, or automated test loops where host `$HOME` protection and Git branch isolation are desirable.
* The user wants to run long-running or autonomous tasks without interactive confirmation prompts.

---

## Execution procedure

### 1. Launch the sandboxed agent
Ensure local workspace profiles are configured (e.g. `bws profile add secure-agent -l`), encode the goal prompt as Base64, and run the task in the background using `run_command`:

```bash
bws gw -b bws-agent-<slug> --stash -- sh -c 'agy-run-wild --effort high -p "$(printf %s <PROMPT_B64> | base64 -d)"'
```

### 2. Review the generated code
When the sandboxed agent completes, `bws gw` auto-commits the changes, fetches the branch back to the host, and pauses at the triage prompt. Before prompting the user:
1. Inspect the diff against merge base (`git diff $(git merge-base HEAD bws-agent-<slug>)..bws-agent-<slug>`).
2. Verify correctness, edge cases, test coverage, and project guidelines.
3. Formulate a definitive assessment and triage recommendation.

### 3. Report and recommend triage action
Present the summary and explicit recommendation to the user before displaying the menu:
* **Summary of changes**: Bulleted breakdown of what was modified and tested.
* **Code review assessment**: Verification findings (tests passed, edge cases handled, line limits respected).
* **Clear recommendation**: Explicitly state the recommended action (e.g. **Recommendation: `[s] Squash-merge`** because all changes form a coherent single unit of work).
* **Triage menu options**:
  - `[s] Squash-merge`: Merge as a single commit into current branch *(Recommended)*.
  - `[m] Merge`: Fast-forward or merge branch preserving individual commits.
  - `[k] Keep`: Preserve the branch for manual inspection without merging.
  - `[d] Discard`: Delete the branch.
  - `[v] View`: Open full diff in pager.

### 4. Execute chosen triage
Send the user's chosen triage input (e.g. `s\n` or `m\n`) to the background task using `manage_task(send_input)`.
