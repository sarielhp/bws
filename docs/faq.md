# Frequently asked questions (FAQ)

## Table of contents

* [Why `.bws/` in workspace roots instead of `.config/bws/`?](#why-bws-in-workspace-roots-instead-of-configbws)
* [How does `bws` differ from Docker or Podman?](#how-does-bws-differ-from-docker-or-podman)
* [How does `bws` differ from raw `bwrap` or Firejail?](#how-does-bws-differ-from-raw-bwrap-or-firejail)
* [Does `bws` require root or daemon processes?](#does-bws-require-root-or-daemon-processes)
* [Can code or agents inside the sandbox escape or modify host configs?](#can-code-or-agents-inside-the-sandbox-escape-or-modify-host-configs)
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

* **Zero daemon overhead**: `bws` runs instantly as a lightweight, unprivileged user process. There are no background daemons, socket permissions, or storage overlay drivers.
* **Direct host toolchain access**: Rather than bundling heavy multi-gigabyte container images, `bws` leverages existing host compilers, language servers, and tools inside isolated user namespaces.
* **Instant in-place startup**: Launches in milliseconds without container image build steps.

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

## How does automatic SSH deploy-key generation work?

When operating in a Git workspace connected to GitHub:
1. `bws` detects the remote repository.
2. If enabled, it automatically generates an isolated, ephemeral SSH key pair.
3. It registers the key as a repository deploy key using the `gh` CLI.
4. The key is injected into the sandbox SSH agent without exposing your host personal SSH keys (`~/.ssh`).

---

## What happens to files created inside the sandbox home?

* **Files in your workspace ($PWD)**: Written directly to disk and preserved.
* **Files in language caches (`~/.go`, `~/.cache/uv`, `~/.cargo`)**: Stored in their respective host cache directories.
* **Files in ephemeral `$HOME`**: Deleted automatically when the sandbox session terminates.
