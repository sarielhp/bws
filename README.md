# bws — bubblewrap sandbox launcher

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](go.mod)
[![Platform: Linux](https://img.shields.io/badge/Platform-Linux-FCC624?logo=linux)](https://github.com/sarielhp/bws)

**`bws`** is a declarative, unprivileged Linux sandbox launcher built on top of [**Bubblewrap (`bwrap`)**](https://github.com/containers/bubblewrap) — an unprivileged containerization engine developed by the Flatpak and Red Hat teams.

> **Note on Bubblewrap**: `bws` is a higher-level declarative frontend for [Bubblewrap](https://github.com/containers/bubblewrap). It wraps `bwrap`'s raw namespace primitives into a complete developer workflow with ephemeral home directories, declarative capability profiles, path masking, shell hooks, and automatic SSH integration.

---

## Table of contents

* [Key capabilities](#key-capabilities)
* [Installation](#installation)
* [Quick start](#quick-start)
* [Automatic GitHub deploy keys](#automatic-github-deploy-keys--git-isolation)
* [Troubleshooting](#troubleshooting)
* [Documentation guide](#documentation-guide)
* [License](#license)

---

## Key capabilities

* **Zero-privilege sandboxing**: Runs purely in Linux user namespaces (`bwrap` unprivileged mode). No root permissions, setuid binaries, or daemon sockets required.
* **Disposable ephemeral homes**: Stages an isolated `$HOME` directory (`/tmp/bws/stage_*`) populated with clean skeleton dotfiles (`.bashrc`, `.profile`, `.tmux.conf`) that disappear when the session ends.
* **Declarative capability profiles**: Composes dev stacks and toolchains with a single line (e.g. `profiles: ["go-dev"]` or `profiles: ["secure-agent"]`).
* **Path masking & security hardening**: Neutralizes host privilege escalation tools (`no-sudo`), SSH credentials (`no-ssh`), browser cookies (`no-browser`), cloud keys (`no-secrets`), and command history (`no-history`).
* **Air-gapped offline mode**: Completely isolate the sandbox network namespace via `-N` / `--offline` or the `offline` profile, severing both internet access and host `127.0.0.1` services.
* **Automated smoke testing**: Verifies sandbox integrity and tool accessibility before running code (`bws test <profile>`).
* **Automatic SSH & Git integration**: Transparent SSH agent forwarding with on-the-fly GitHub Deploy Key generation via `gh`.
* **Safe host pass-through**: Explicit environment variable forwarding (`pass_env`) preserving isolation without secret leakage.

---

## Installation

### From source

```bash
git clone https://github.com/sarielhp/bws.git
cd bws
make install
```

The binary will be compiled and installed to `~/bin/bws`. Ensure `~/bin` is in your `$PATH` (e.g. `export PATH="$HOME/bin:$PATH"` in your `.bashrc` or `.zshrc`).

### Prerequisites

* **Linux kernel** 3.8+ with user namespaces enabled (standard on Debian, Ubuntu, Fedora, Arch).
* **`bubblewrap`** (`bwrap`):
  ```bash
  # Debian / Ubuntu / Mint
  sudo apt install bubblewrap

  # Fedora / RHEL
  sudo dnf install bubblewrap

  # Arch Linux
  sudo pacman -S bubblewrap
  ```
* **`gh`** (GitHub CLI, optional): Required for automated repository Deploy Key management (`gh auth login`).

---

## Quick start

### 1. Initialize your project workspace

Before entering a sandbox, `bws` inspects your repository to determine which language toolchains and build caches to map so that builds run directly without exposing your host `$HOME`.

In your project directory, run:

```bash
# Auto-detect language stack (Go, Python/UV, Rust, Node, LaTeX) and create .bws/config.jsonc
bws init-dev

# Dry-run preview without writing files
bws init-dev -n

# Or explicitly select a preset stack
bws init-dev --preset python
```

**Why is workspace initialization needed?**  
`bws` creates a clean, isolated `$HOME` by default. Initializing your project generates `.bws/config.jsonc`, ensuring essential toolchain directories are mapped into the sandbox while keeping all personal host files private:
* **Go**: Maps `~/.go` and `~/.cache/go-build` so `go build`, modules, and GOPATH function normally.
* **Python / UV**: Maps `~/.cache/uv` and `~/.cache/pip` so package managers share cached wheels.
* **Rust**: Maps `~/.cargo` and `~/.rustup` for compilers and crate registries.
* **Node**: Maps `~/.npm`, `~/.pnpm-store`, and `~/.cache/yarn`.

### 2. Launch an ephemeral interactive sandbox

Now, drop into your isolated development environment:

```bash
bws
```

**What happens behind the scenes:**
* **Ephemeral `$HOME` staging**: `bws` provisions a temporary home directory in `/tmp/bws/stage_*` populated with clean dotfiles (`.bashrc`, `.profile`, `.tmux.conf`) from `~/.config/bws/skeleton/`.
* **In-place workspace mounting**: Your current directory is bind-mounted directly at its exact path, allowing compilers, language tools, and editors to edit and build code directly.
* **Zero-privilege namespaces**: Runs inside unprivileged Linux user, mount, IPC, PID, and UTS namespaces with zero root requirements.
* **Host privacy protection**: Personal dotfiles, host SSH keys, browser cookies, cloud API tokens, and command history remain completely hidden.
* **Clean automatic teardown**: When you exit (`exit` or `Ctrl+D`), the ephemeral stage directory and all temporary mounts are destroyed cleanly.

### 3. Run commands directly inside a sandbox

```bash
# Execute a single command inside a sandbox environment
bws exec -- go test ./...
bws exec -N -- go test ./...       # Run tests air-gapped without network
bws exec -- uv run main.py
```

### 4. Search and inspect capability profiles

**What are capability profiles and why use them?**  
Profiles are modular, declarative sandboxing recipes. Instead of manually crafting dozens of complex `bwrap` arguments (specifying cache directories, PATH entries, read-only config mounts, environment variables, and security masks), profiles package all requirements for a tool or language into a reusable definition. You can activate full development stacks or security bundles simply by listing their names in your configuration (e.g. `profiles: ["python-dev", "docker", "no-secrets"]`).

**Thousands of profiles out of the box:**  
Beyond the embedded profile catalog, `bws` integrates with the [**Homebrew Formula API**](https://brew.sh) (for dependencies, binaries, and package metadata across thousands of open-source tools) and [**Firejail Profiles**](https://github.com/netblue30/firejail) (for filesystem access and security rules). This allows `bws` to search and synthesize sandboxing intelligence on-the-fly for additional tools.

```bash
# Search profiles across local, embedded, and Homebrew catalog
bws profile search python
bws profile search ripgrep

# List all locally registered and embedded profiles
bws profile list

# Inspect resolved dependency chain, mounts, masked paths, and smoke tests
bws profile show go-dev

# Generate a tailored sandbox profile on-the-fly for any tool
bws profile new ripgrep
```

### 5. Verify stack integrity

`bws test <profile>` executes automated smoke tests directly inside a real, isolated sandbox session to verify that your specific tools, compilers, and security rules function properly inside the bubble before running code:

* **Tool & compiler execution**: Confirms that binaries are in PATH, package managers work, compilers can build code, and language servers start cleanly.
* **Security enforcement**: Verifies that sensitive paths (like `sudo`, SSH keys, browser profiles, and cloud secrets) are actively blocked.

```bash
# Verify that the complete Go toolchain (Go compiler, Gopls LSP, Git, GitHub CLI, Editor) works in sandbox
bws test go-dev

# Verify hardened AI agent environment (OpenCode, Antigravity, and active secret masking)
bws test secure-agent

# Verify that privilege escalation binaries and sudoers are actively blocked
bws test no-sudo

# Verify that outbound network access and host local ports are blocked
bws test offline
```

---

## Automatic GitHub deploy keys & Git isolation

A common problem with developer sandboxes is Git authentication: *how to enable `git push` and `git pull` without exposing your personal host SSH keys or account-wide GitHub tokens?*

`bws` solves this automatically:
1. **Repository detection**: When run in a Git workspace connected to GitHub, `bws` inspects the origin remote.
2. **Dedicated key generation**: It creates a unique, isolated SSH keypair in `~/.sandbox/deploy_keys/<owner>_<repo>`.
3. **Automatic registration**: Using your host `gh` CLI credentials, it registers the public key as a read/write **GitHub Deploy Key**.
4. **Scoped agent injection**: It loads **only this repository key** into the sandbox SSH agent (the `~/.sandbox/deploy_keys/` directory is itself masked inside the sandbox).

```bash
# Pull, commit, and push inside the sandbox directly over SSH
bws exec -- git push origin main  # Requires SSH remote (e.g. git@github.com:owner/repo)
```

**Security boundary**: If code or an autonomous agent inside the sandbox is compromised, its reach is cryptographically confined to that single repository — your master SSH keys (`~/.ssh`) and personal GitHub account tokens remain strictly on the host.

---

## Troubleshooting

**`bws: command not found` after installation**  
Ensure `~/bin` is in your `$PATH`. Add `export PATH="$HOME/bin:$PATH"` to your `.bashrc` or `.zshrc` and restart your shell.

**`bwrap: command not found`**  
`bws` requires `bubblewrap` (`bwrap`) installed on the host. See [Prerequisites](#prerequisites).

**User namespaces disabled on host**  
On some hardened systems, unprivileged user namespaces may be disabled:
```bash
# Check current status
sysctl kernel.unprivileged_userns_clone

# Enable unprivileged user namespaces
sudo sysctl -w kernel.unprivileged_userns_clone=1
```

**`go build` (or other compiler) fails inside sandbox**  
Run `bws init-dev` first to generate `.bws/config.jsonc` and map the required language caches into the sandbox. Then re-enter with `bws`.

**GitHub deploy keys not working**  
* Verify your remote uses SSH format: `git remote get-url origin` (should look like `git@github.com:owner/repo.git`).
* Ensure `gh` is authenticated on the host: `gh auth status`.
* Ensure `auto_repo_deploy_key` is not disabled in your config.

---

## Documentation guide

Detailed guides are modularized across the `docs/` directory:

| Document | Description |
| :--- | :--- |
| [**Frequently asked questions (FAQ)**](docs/faq.md) | Common questions, design rationale (why `.bws/` in projects), comparisons with Docker & Firejail |
| [**Configuration reference**](docs/configuration.md) | Full JSONC schema, global vs local merging rules, `$VAR` expansion, and skeletons |
| [**Security & path masking**](docs/security.md) | Zero-trust threat model, path masking (`tmpfs` / `/dev/null`), and hardening profiles |
| [**CLI command reference**](docs/commands.md) | Comprehensive usage guide for all subcommands, options, and flags |
| [**Architecture & internals**](docs/architecture.md) | Deep technical guide to user namespaces, invocation lifecycle, and staging mechanics |
| [**Capability profiles catalog**](profiles/README.md) | Complete reference and test specifications for all 35+ embedded profiles |

---

## License

MIT License. Developed for robust, unprivileged Linux sandboxing.
