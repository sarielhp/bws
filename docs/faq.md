# Frequently asked questions (FAQ)

## Table of contents

* [Why `.bws/` in workspace roots instead of `.config/bws/`?](#why-bws-in-workspace-roots-instead-of-configbws)
* [How does `bws` differ from Docker or Podman?](#how-does-bws-differ-from-docker-or-podman)
* [How does `bws` differ from raw `bwrap` or Firejail?](#how-does-bws-differ-from-raw-bwrap-or-firejail)
* [Does `bws` require root or daemon processes?](#does-bws-require-root-or-daemon-processes)
* [Can code or agents inside the sandbox escape or modify host configs?](#can-code-or-agents-inside-the-sandbox-escape-or-modify-host-configs)
* [Why is the GitHub CLI (`gh`) blocked inside the sandbox?](#why-is-the-github-cli-gh-blocked-inside-the-sandbox)
* [How do I push Git branches or create pull requests?](#how-do-i-push-git-branches-or-create-pull-requests)
* [Does blocking `gh` affect `bws gw` (git-workflow)?](#does-blocking-gh-affect-bws-gw-git-workflow)
* [Does `bws` restrict outbound network access?](#does-bws-restrict-outbound-network-access)
* [How does automatic SSH deploy-key generation work?](#how-does-automatic-ssh-deploy-key-generation-work)
* [What happens to files created inside the sandbox home?](#what-happens-to-files-created-inside-the-sandbox-home)

---

## Why `.bws/` in workspace roots instead of `.config/bws/`?

* **`~/.config/` is strictly a user-home concept (XDG Base Directory)**: It exists to store global defaults for the user account (`~/.config/bws/config.jsonc`).
* **Avoids repository clutter and tool confusion**: Having a `.config/` folder at the root of a code repository looks like an accidentally checked-in user home folder, and can conflict with tools that happen to generate a `.config` directory.
* **Consistent with industry tool conventions**: A dedicated `.bws/` directory mirrors standard repository-scoped tools:
  * VS Code: `.vscode/settings.json`
  * Git: `.git/config`
  * GitHub: `.github/workflows/`
  * Cargo: `.cargo/config.toml`
  * Devcontainers: `.devcontainer/devcontainer.json`
  * OpenCode: `.opencode/`
* **Isolated & masked**: Inside the sandbox, `.bws/` is automatically masked by default via an empty `tmpfs`, ensuring running processes cannot tamper with host launcher rules.

---

## How does `bws` differ from Docker or Podman?

* **Zero daemon overhead**: `bws` runs as an unprivileged user process with no background daemon. There are no background daemons, socket permissions, or storage overlay drivers.
* **Direct host toolchain access**: Rather than bundling heavy multi-gigabyte container images, `bws` leverages existing host compilers, language servers, and tools inside isolated user namespaces.
* **In-place startup**: Launches in milliseconds without container image build steps.

---

## How does `bws` differ from raw `bwrap` or Firejail?

* **Declarative profiles & DAG resolution**: Raw `bwrap` requires writing sprawling, error-prone shell scripts with dozens of `--bind`, `--ro-bind`, and `--tmpfs` flags. `bws` lets you compose stacks declaratively (`profiles: ["python-dev", "no-secrets"]`).
* **Ephemeral home staging**: `bws` automatically synthesizes disposable `$HOME` environments with clean skeletons and dotfiles.
* **Autonomous AI agent hardening**: Includes out-of-the-box profiles to isolate sensitive host secrets, browser sessions, email databases, and shell histories.

---

## Does `bws` require root or daemon processes?

**No.** `bws` uses unprivileged Linux user namespaces (`CLONE_NEWUSER`). It never requires `sudo`, root privileges, setuid binaries, or daemon access.

---

## Can code or agents inside the sandbox escape or modify host configs?

* **Unmapped host `$HOME`**: Your real host home directory is not mounted; only explicitly declared toolchain caches and the staged ephemeral home exist.
* **Auto-masked `.bws/`**: The local `.bws/` configuration directory and `.bws.jsonc` file are overlaid with an empty `tmpfs` / `/dev/null`, preventing in-sandbox code from modifying host launcher rules.
* **Privilege escalation blocked**: Profiles like `no-sudo` overlay `/dev/null` on `sudo`, `su`, `pkexec`, and mask `/etc/sudoers`.

---

## Why is the GitHub CLI (`gh`) blocked inside the sandbox?

`bws` hardens developer environments against credential leakage and unauthorized API actions:

* **Account-wide token containment**: Untrusted scripts, compromised dependencies, or autonomous AI agents running inside the sandbox should not have ambient access to account-level GitHub tokens, personal access tokens (PATs), or forge administrative capabilities.
* **Blast-radius minimization**: An unrestricted `gh` CLI inside the sandbox could list private repositories across your GitHub account or organizations, push unauthorized branches, create public gists containing sensitive code, or tamper with pull requests and releases.
* **Default masking**: `gh` and sibling forge binaries (`glab`, `hub`, `tea`), along with credential caches (`~/.config/gh`, `~/.git-credentials`, `~/.netrc`), are masked by default via `/dev/null` overlays and empty `tmpfs` mounts.

---

## How do I push Git branches or create pull requests?

`bws` uses a host-triage model for Git forge interactions:

1. **Local repository workflow in sandbox**: Inside the sandbox, agents and developers work freely on local Git branches—creating commits, rebasing, running test suites, and staging changes.
2. **Push and PR triage on the host**: Because the workspace directory is mounted directly into the sandbox, changes made inside the sandbox are immediately visible on your host filesystem. Pushing branches (`git push origin <branch>`) and opening pull requests (`gh pr create`) can be performed directly from your authenticated host terminal.
3. **Automated deploy keys for scoped push**: If outbound Git push is required from within the sandbox, enable SSH agent integration (`enable_ssh: true`). `bws` registers a dedicated per-repository deploy key on GitHub via the host `gh` CLI before launching the container, allowing repository-scoped pushes over SSH without exposing account-wide tokens.
4. **Explicit opt-out**: If an interactive session genuinely requires `gh` inside the sandbox, disable blocking by setting `"block_gh": false` under `"features"` in `.bws/config.jsonc`.

---

## Does blocking `gh` affect `bws gw` (git-workflow)?

**No.** `bws gw` (or `bws git-workflow`) remains 100% operational.

* **Host orchestration**: `bws gw` runs on the host to create an isolated, disposable Git clone or worktree in `/tmp/bws/agent_*`.
* **Isolated execution**: `bws` executes the agent command inside the sandbox on that isolated clone. The agent commits its work to local Git branches inside the worktree without needing `gh`.
* **Host triage and integration**: When the agent finishes, `bws gw` on the host inspects the worktree, shows diffs, and manages branch merging or cleanup. Any remote Git push or PR creation is initiated from the host environment where your host credentials reside.

---

## How does automatic SSH deploy-key generation work?

When operating in a Git workspace connected to GitHub via SSH (`git@github.com:...`) and authenticated with the `gh` CLI (`gh auth login`):
1. `bws` detects the remote repository.
2. If enabled, it automatically generates an isolated, per-repository SSH keypair stored in `~/.sandbox/deploy_keys/`.
3. It registers the key as a repository deploy key using the `gh` CLI.
4. The key is injected into the sandbox SSH agent without exposing your host personal SSH keys (`~/.ssh`).

---

## Does `bws` restrict outbound network access?

**By default, no** — but it provides native **air-gapped network isolation** via `-N` / `--offline` or the `offline` profile.

### Default behavior
By default, outbound TCP/UDP traffic is permitted so package managers (`go`, `npm`, `cargo`, `uv`, `pip`) and language servers can fetch packages and dependencies.

### Air-gapped isolation (`-N` / `offline` profile)
When network isolation is enabled (via `bws -N`, `bws exec -N`, or adding `"offline"` to your `profiles`):
1. **Internet blocked**: All outbound DNS, HTTP, and socket connections fail immediately.
2. **Host `127.0.0.1` protected**: The sandbox gets a private loopback interface (`lo`). Services running on the host'''s `127.0.0.1` (e.g. Postgres, Redis, Ollama, Docker TCP, local dev servers) are **completely unreachable**.
3. **Internal IPC only**: Processes started *inside that same sandbox* can communicate with each other on the sandbox'''s private `127.0.0.1`.

---

## What happens to files created inside the sandbox home?

* **Files in your workspace ($PWD)**: Written directly to disk and preserved.
* **Files in language caches (`~/.go`, `~/.cache/uv`, `~/.cargo`)**: Stored in their respective host cache directories.
* **Files in ephemeral `$HOME`**: Deleted automatically when the sandbox session terminates.
