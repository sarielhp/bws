# Comprehensive Review: The `/rw` (Run Wild) Disposable Agent Sandbox Skill

We are evaluating the `/rw` skill used by parent AI coding agents (such as Google Antigravity/Gemini) to dispatch autonomous child agents (`agy-run-wild`) into isolated Bubblewrap Git sandboxes using `bws gw`.

Perform an exhaustive, critical review as a senior AI systems architect, Linux container engineer, and CLI tool designer.

---

## Part 1: The `/rw` Skill Specification

The current skill definition (`~/.gemini/config/skills/rw/SKILL.md`) is:

```markdown
---
name: rw
description: >-
  Dispatch autonomous coding agent tasks into an isolated, disposable Bubblewrap
  Git sandbox using `bws gw` (and `agy-run-wild`). Trigger whenever the user asks to run wild, use `/rw`, `/bws`, run in sandbox, dispatch an autonomous agent task,
  or perform isolated refactors/testing.
---

# `rw` (Run Wild) Disposable Agent Sandbox Skill

This skill allows AI coding assistants to delegate complex, autonomous coding tasks to a sandboxed child agent running in an ephemeral Git clone with air-gapped isolation, automatic commit on exit, automated post-run code review, and 1-key merge triage.

---

## When to Use This Skill

* The user types `/rw`, asks to "run wild", "run in sandbox", "use bws", or "dispatch an agent".
* The user provides a high-level task and wants it executed autonomously to completion without step-by-step confirmation.
* A task involves deep refactors, automated test/lint loops, or untrusted code where host `$HOME` protection and Git branch isolation are desirable.

---

## Core Delegation Rules for the Parent Agent

1. **Zero Host Editing Before Dispatch**: Never perform partial code changes or manual file edits on the host when delegating to the sandbox. Formulate the technical specification and dispatch immediately.
2. **Translate High-Level Intent to Concrete Specs**: The parent agent must ground the user's high-level goal by injecting:
   - Target files, functions, and data structures.
   - Project-specific constraints (e.g. `AGENTS.md` rules, file line limits, scripting conventions).
   - Exact verification and test commands (e.g. `go test -v ./...`, `./tools/verify_build.sh`, `cargo test`).
3. **Clean Worktree Guarantee**: Always ensure the host working tree is clean before invoking `bws gw`. Auto-commit a checkpoint using `--no-verify` to bypass restrictive pre-commit hooks:
   `[[ -z $(git status --porcelain) ]] || (git add -A && git commit --no-verify -m "wip: pre-sandbox checkpoint")`
4. **Descriptive Branch Slugs**: Choose a concise, kebab-case branch name describing the task (e.g. `feat-interactive-search`, `fix-audio-sync`, `refactor-db-pool`), avoiding spaces or generic names.

---

## Execution Procedure

### 1. Ensure Clean Host State & Launch Sandbox Agent
Run the task in the background. Execute the following sequential steps:

```bash
# 1. Commit all tracked & untracked working changes to ensure a clean base
[[ -z $(git status --porcelain) ]] || (git add -A && git commit --no-verify -m "wip: pre-sandbox checkpoint")

# 2. Dynamically add required project language profiles to global sandbox config (e.g., go, python, rust)
bws add ai <detected-langs> -g

# 3. Launch the sandboxed agent in the background.
# CRITICAL: Use single quotes for the description to prevent shell expansion errors. Escape internal single quotes as '\x27'
bws gw -b <descriptive-branch-slug> -- agy-run-wild --effort high -p '/goal <grounded technical task description>'
```

*Note*: After launching the command in the background, stop calling tools to wait for the reactive notification.

---

### 2. Verify State & Review Generated Code
When the background task notifies you it has stopped or paused:
1. **Check for Early Failures**: If the task exited without displaying the interactive triage menu (`[s] Squash-merge`, etc.), the child agent likely crashed or failed. Review the logs, report the error to the user, and abort.
2. **Review Code (Concurrent)**: If paused at the menu, use a **separate** command execution tool (do NOT send input to the paused task) to inspect the changes:
   `git diff HEAD..<descriptive-branch-slug>`
3. Verify correctness, edge cases, test coverage, and project-specific guidelines (e.g. file line limits, conventions, formatting).

---

### 3. Report & Recommend Triage Action
Present the summary and explicit recommendation to the user before displaying the menu:
* **Summary of Changes**: Bulleted breakdown of modifications.
* **Code Review Assessment**: Verification findings (tests passed, edge cases handled, line limits respected).
* **Clear Recommendation**: Explicitly state the recommended action (e.g. **Recommendation: `[s] Squash-merge`**).
* **Triage Menu Options**:
  - `[s] Squash-merge`: Merge as a single commit into current branch *(Recommended)*.
  - `[m] Merge`: Fast-forward or merge branch preserving individual commits.
  - `[k] Keep`: Preserve the branch for manual inspection without merging.
  - `[d] Discard`: Delete the branch and discard changes.
  - `[v] View`: Open full diff in pager.

---

### 4. Execute Chosen Triage
Send the user's chosen triage input to the paused background task using `manage_task(send_input)`:
* **Critical**: You must append a newline character (e.g., `s\n` or `m\n`) to simulate the Enter key.
* **Conflict Handling**: If the host branch advanced during the run and the merge fails, alert the user and leave the branch intact for manual conflict resolution.
```

---

## Part 2: Underlying Capabilities of `bws`

`bws` is a fast, unprivileged container engine frontend for Bubblewrap (`bwrap`) with rich orchestration features:

1. **`bws gw` Lifecycle**:
   - Host provisions an ephemeral Git clone (`/tmp/bws/agent_*`) using `git clone --shared file://<hostRepo>` (0-copy object sharing, instantaneous).
   - Checks out the named branch (`-b <branch>`).
   - Copies local `.bws` workspace config and `.env` into the ephemeral clone.
   - Automatically masks `.bws/` and `.bws.jsonc` inside the container with empty `tmpfs` and `/dev/null` to prevent in-sandbox tampering.
   - Executes `bws run --no-ssh -- <command>`: air-gapped SSH, dedicated ephemeral `$HOME`.
   - On container exit: auto-commits remaining dirty changes on the agent branch on the host, fetches branch back to host repo (`git fetch /tmp/bws/agent_*`), displays `git diff --stat`, and prompts with an interactive triage menu (`Merge`, `Squash`, `Keep`, `Discard`, `View`).
   - Flags on `bws gw`:
     - `--stash`: Auto-stash dirty host working tree before starting and pop after triage.
     - `--allow-dirty`: Run even if working tree has uncommitted changes.
     - `-b, --branch`: Explicit branch name.
     - `-v, --verbose`: Debug logging.
     - Subcommands: `bws gw list [--merged | --unmerged]`, `bws gw prune [-a] [-n]`.
2. **Security & Boundary Hardening**:
   - Unprivileged Linux user namespaces (`CLONE_NEWUSER`), PID namespaces (`CLONE_NEWPID`), mount namespaces (`CLONE_NEWNS`), IPC unshared (`--unshare-ipc`).
   - `--die-with-parent` guarantees no orphan processes remain when session terminates.
   - Hardening profiles: `no-sudo`, `no-ssh`, `no-gh`, `no-secrets` (AWS/GPG/Azure/netrc/git-credentials), `no-history`, `offline` (unshares network namespace `CLONE_NEWNET`, isolating loopback from host `127.0.0.1` services).
   - In-sandbox forge binary blocking (`gh`, `glab`, `hub`, `tea` overlaid with `/dev/null`).
   - Environment scrubbing: `GH_TOKEN`, `GITHUB_TOKEN` unset or blocked from forwarding.
3. **Filesystem Mount Engine**:
   - Topological sorting: parents strictly precede children (`depth(parent) < depth(child)`).
   - Read-only by default for external mounts (`binds_ro`).
   - Symlink auto-resolution on `bws mount add`: resolves canonical target, guards against dangling links and redundant workspace links.
   - Ephemeral staged `$HOME` with dotfile skeleton and isolated history files.
4. **Network and Proxying**:
   - In-process HTTP/IPv4 traffic tunneling (`--proxy` / `--no-proxy`).
   - Filtered session D-Bus proxy (`--dbus` via `xdg-dbus-proxy`).

---

## Review Questions for Claude

1. **Capabilities Exploitation**: How can the `/rw` skill take fuller advantage of `bws`'s capabilities? (e.g. should it leverage `--stash` instead of manual host commits? Should it support offline/air-gapped execution `-N` for purely offline tasks? Should it support passing custom mounts or workspace profiles dynamically?)
2. **Parent-Child Prompt Engineering**: How can the instructions to the parent agent be improved to craft better `/goal` specifications? Should the prompt give the child agent awareness of the sandbox environment, or is transparent execution better?
3. **Failure Handling & Edge Cases**: What edge cases are unhandled? (e.g. child agent infinite loops/timeouts, Git merge conflicts on squash, non-zero child exits before triage, dirty stash restoration failures).
4. **Shell Quoting & Execution Robustness**: Is passing the `/goal` via bash single-quotes with `'\x27'` escaping fragile? What is the safest way to dispatch multi-line complex prompts into `agy-run-wild`?
5. **Concrete Recommendations**: Provide a prioritized list of concrete enhancements to update and elevate `SKILL.md`.
