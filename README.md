# bws — bubblewrap sandbox launcher

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](go.mod)
[![Platform: Linux](https://img.shields.io/badge/Platform-Linux-FCC624?logo=linux)](https://github.com/sarielhp/bws)

**`bws`** is a fast, declarative, unprivileged Linux sandbox launcher and orchestrator built on top of [**Bubblewrap (`bwrap`)**](https://github.com/containers/bubblewrap) — the gold-standard, unprivileged containerization engine developed by the Flatpak and Red Hat teams.

> **Note on Bubblewrap**: `bws` is a higher-level declarative frontend for [Bubblewrap](https://github.com/containers/bubblewrap). It wraps `bwrap`'s raw namespace primitives into a complete developer workflow with ephemeral home directories, declarative capability profiles, path masking, shell hooks, and automatic SSH integration.

---

## Key capabilities

* **Zero-privilege sandboxing**: Runs purely in Linux user namespaces (`bwrap` unprivileged mode). No root permissions, setuid binaries, or daemon sockets required.
* **Disposable ephemeral homes**: Stages an isolated `$HOME` directory (`/tmp/bws/stage_*`) populated with clean skeleton dotfiles (`.bashrc`, `.profile`, `.tmux.conf`) that disappear when the session ends.
* **Declarative capability profiles**: Compose dev stacks and toolchains with a single line (e.g. `profiles: ["go-dev"]` or `profiles: ["secure-agent"]`).
* **Path masking & security hardening**: Neutralize host privilege escalation tools (`no-sudo`), SSH credentials (`no-ssh`), browser cookies (`no-browser`), cloud keys (`no-secrets`), and command history (`no-history`).
* **Automated smoke testing**: Verify sandbox integrity and tool accessibility before running code (`bws test <profile>`).
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

The binary will be compiled and installed to `~/bin/bws`.

### Prerequisites

* **Linux kernel** 3.8+ with user namespaces enabled (standard on Debian, Ubuntu, Fedora, and Arch).
* **`bubblewrap`** (`bwrap`):
  ```bash
  # Debian / Ubuntu / Mint
  sudo apt install bubblewrap

  # Fedora / RHEL
  sudo dnf install bubblewrap

  # Arch Linux
  sudo pacman -S bubblewrap
  ```

---

## Quick start

### 1. Launch an ephemeral interactive sandbox

Running `bws` without arguments drops you into a fully isolated, interactive sandbox shell in your current working directory:

```bash
bws
```

**What happens behind the scenes:**
* **Ephemeral `$HOME` staging**: `bws` provisions a fresh, temporary home directory in `/tmp/bws/stage_*` populated with clean skeleton dotfiles (`.bashrc`, `.profile`, `.tmux.conf`) from your global skeleton (`~/.config/bws/skeleton/`).
* **In-place workspace mounting**: Your current directory is bind-mounted directly at its exact path, allowing compilers, language tools, and editors to work seamlessly on your project without copying large trees.
* **Zero-privilege namespaces**: Runs inside unprivileged Linux user, mount, IPC, PID, and UTS namespaces (`bwrap` unprivileged mode) with no root or setuid requirements.
* **Host privacy protection**: Your host `$HOME` dotfiles, SSH credentials, browser cookies, cloud API keys, and shell histories are shielded from running code.
* **Clean automatic teardown**: The moment you exit the sandbox (`exit` or `Ctrl+D`), the ephemeral stage directory and all temporary mounts are destroyed cleanly without leaving leftover state on your host.

### 2. Auto-detect and initialize a project workspace

The `bws init-dev` command inspects your workspace (detecting Go, Python/UV, Rust, Node, LaTeX, AI agents, etc.), creates the local `.bws/` subdirectory, and writes a tailored `.bws/config.jsonc` file:

```bash
# Auto-detect language stack and initialize .bws/config.jsonc in current directory
bws init-dev

# Dry-run preview: print generated JSONC to stdout without writing to disk
bws init-dev -n

# Explicitly select a preset stack (go, python, rust, node, latex, agent, all)
bws init-dev --preset python

# Include additional tool profiles
bws init-dev -p docker,quarto
```

Your workspace is mounted directly at its exact absolute path inside the sandbox (read-write by default, or read-only via `-r`), allowing compilation and testing to run seamlessly in place while isolating the rest of your system.

**Why is workspace initialization needed?**  
Because `bws` creates a clean, isolated `$HOME` by default, your personal dotfiles and development toolchain caches are shielded. Initializing your workspace ensures only the specific directories required for your project are mapped into the bubble:
* **Go**: Exports `~/.go` and `~/.cache/go-build` so `go build`, package modules, and GOPATH function seamlessly.
* **Python / UV**: Exports `~/.cache/uv` and `~/.cache/pip` so package managers and virtual environments share cached wheels without re-downloading.
* **Rust**: Exports `~/.cargo` and `~/.rustup` for compilers, cargo binaries, and crate registries.
* **Node**: Exports `~/.npm`, `~/.pnpm-store`, and `~/.cache/yarn`.

This gives you full isolation from sensitive personal data and host secrets, while your language toolchains and package caches remain fast and fully operational.

### 3. Run commands directly inside a sandbox

```bash
# Execute a single command inside a sandbox environment
bws exec -- go test ./...
bws exec -- uv run main.py
```

### 4. Search and inspect capability profiles

**What are capability profiles and why use them?**  
Profiles are modular, declarative sandboxing recipes. Instead of manually crafting dozens of complex `bwrap` arguments (specifying cache directories, PATH entries, read-only config mounts, environment variables, and security masks), profiles package all requirements for a tool or language into a reusable definition. You can activate full development stacks or security bundles simply by listing their names in your configuration (e.g. `profiles: ["python-dev", "docker", "no-secrets"]`).

**Thousands of profiles out of the box:**  
Beyond the embedded profile catalog, `bws` integrates with the [**Homebrew Formula API**](https://brew.sh) (for dependencies, binaries, and package metadata across 7,000+ open-source tools) and [**Firejail Profiles**](https://github.com/netblue30/firejail) (for battle-tested filesystem access and security rules). This allows `bws` to search and synthesize sandboxing intelligence on-the-fly for virtually any tool.

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
```

---

## Declarative capability profiles

`bws` features a modular **profile engine** with over 35 pre-configured development stacks and security profiles. See the full [**profiles catalog**](profiles/README.md) for detailed documentation on every profile.

### Intelligence from Homebrew & Firejail

When you need to sandbox a tool not yet in the embedded catalog, `bws` queries:
1. [**Homebrew API**](https://formulae.brew.sh): Inspects package metadata, runtime dependencies, binaries, and descriptions.
2. [**Firejail Catalog**](https://github.com/netblue30/firejail): Adapts proven security profiles, whitelist/blacklist paths, and isolation rules.

```bash
# Search profiles by keyword or description
bws profile search <query>

# Generate a new profile from Homebrew & Firejail intelligence
bws profile new <name>

# List all available profiles (embedded, global, local)
bws profile list

# Inspect resolved dependency chain, mounts, masked paths, and tests
bws profile show <name>

# Fetch community profiles from GitHub
bws profile fetch <name>
bws profile update
```

### Meta-profiles & bundles

Profiles can declare dependencies (`requires`) to build complete developer stacks:

```jsonc
// Embedded go-dev profile (profiles/go-dev.json)
{
  "name": "go-dev",
  "description": "Full Go developer environment",
  "requires": [
    "go",
    "gopls",
    "git",
    "gh",
    "editor"
  ]
}
```

---

## Security & path masking engine

`bws` provides **path masking** to overlay empty `tmpfs` mounts over directories or `/dev/null` over binaries, making host utilities invisible inside the sandbox.

### Built-in hardening profiles

* **`no-sudo`**: Blocks `/usr/bin/sudo`, `su`, `pkexec`, `doas`, `gpasswd`, `newgrp`, and masks `/etc/sudoers`.
* **`no-ssh`**: Blocks `~/.ssh` and `/etc/ssh/ssh_config`.
* **`no-browser`**: Blocks browser profiles (`~/.mozilla`, Chrome, Chromium, Brave, Edge) to protect session cookies.
* **`no-email`**: Blocks desktop email databases (`~/.thunderbird`, `~/.config/evolution`, `~/.mutt`, `~/Maildir`).
* **`no-secrets`**: Blocks cloud credentials (`~/.aws`, `~/.azure`, `~/.config/gcloud`, `~/.password-store`, `~/.gnupg`).
* **`no-history`**: Blocks shell history files (`.bash_history`, `.zsh_history`, `.python_history`, `.psql_history`).
* **`secure-agent`**: Zero-trust bundle combining all hardening profiles with the `ai` stack for autonomous coding agents.

---

## Configuration architecture

`bws` uses a layered, comment-supported **JSONC** configuration hierarchy:

### 1. Global user configuration (`~/.config/bws/`)

Located in your user home directory according to the XDG standard:
* **`~/.config/bws/config.jsonc`**: Global base settings (preferred editor, default profiles, standard environment pass-through).
* **`~/.config/bws/skeleton/`**: Default dotfiles (`.bashrc`, `.profile`, `.tmux.conf`) copied into every new sandbox.
* **`~/.config/bws/profiles/`**: Custom user-authored capability and security profiles.

### 2. Local project configuration (`.bws/` in workspace root)

Local configuration is **scoped directly to your current project/repository** and lives inside the `.bws/` directory at the project root (mirroring conventions like `.vscode/` or `.cargo/`):
* **`.bws/config.jsonc`**: Workspace-specific overrides (e.g. declaring `profiles: ["go-dev", "no-secrets"]`, adding project-specific bind mounts, custom environment variables).
* **`.bws/skeleton/`**: Project-specific dotfiles (e.g. a custom `.bws/skeleton/.bashrc` with project aliases or build shortcuts) that overlay on top of the global skeleton.

> **Why `.bws/` instead of `.config/bws/` in projects?**  
> `~/.config/` is strictly a user-home concept (XDG). In a project repository, having a top-level `.config/` folder is ambiguous and can conflict with project build tools. A dedicated `.bws/` folder in the project root is clean, self-contained, easily gitignored or committed, and follows standard tool patterns (`.vscode/`, `.github/`, `.devcontainer/`).

---

### Example global configuration (`~/.config/bws/config.jsonc`)

```jsonc
{
  // Active personal defaults across all projects
  "profiles": [
    "editor"
  ],

  // Pass-through safe operational environment variables from host
  "pass_env": [
    "USER", "LOGNAME", "SHELL", "TERM", "LANG", "LC_ALL"
  ],

  // Sandbox environment variables ($VAR expansion supported)
  "env": {
    "HOME": "@@HOME@@",
    "EDITOR": "emacs -nw",
    "VISUAL": "$EDITOR"
  },

  // Base read-only protection for host tools
  "binds_ro": [
    ["~/.local", "@@HOME@@/.local"]
  ]
}
```

### Example local project configuration (`.bws/config.jsonc`)

Generated automatically via `bws init-dev`:

```jsonc
{
  // Stack profiles required specifically for this project
  "profiles": [
    "go-dev",
    "docker",
    "no-sudo"
  ],

  // Project-specific environment variables
  "env": {
    "CGO_ENABLED": "0"
  },

  // Project-specific mounts
  "binds_rw": [
    ["/data/test_fixtures", "/data/test_fixtures"]
  ]
}
```

---

## Skeletons & ephemeral homes

Whenever `bws` launches a sandbox:
1. It creates an isolated temporary directory in `/tmp/bws/stage_*`.
2. It copies base dotfiles from the **global skeleton** (`~/.config/bws/skeleton/`).
3. It overlays workspace dotfiles from the **local skeleton** (`.bws/skeleton/` if present in the project).
4. It dynamically appends profile PATHs and shell hooks to the staged `.bashrc`.
5. When the session terminates, the ephemeral home directory is cleanly removed.

---

## Command reference

| Command | Description |
| :--- | :--- |
| **`bws`** | Launch an interactive sandbox shell |
| **`bws exec -- <cmd...>`** | Execute a command inside the sandbox |
| **`bws test <profile>`** | Run multi-command verification tests for a profile |
| **`bws init-dev`** | Auto-detect workspace and generate `.bws/config.jsonc` |
| **`bws profile search <query>`** | Search profiles by name, description, and tools |
| **`bws profile list`** | List all registered profiles |
| **`bws profile show <name>`** | Display detailed resolution plan for a profile |
| **`bws profile new <name>`** | Generate a profile via Homebrew API & Firejail |
| **`bws profile fetch <name>`**| Download a profile from GitHub |
| **`bws profile update`** | Synchronize all local profiles with GitHub |
| **`bws conf info`** | Preview merged bwrap argument plan (dry run) |
| **`bws cbind add <path>`** | Add a bind mount to config |
| **`bws ccopy add <path>`** | Add a copy path to config |
| **`bws scp <user@host:>`** | Copy configuration and themes to remote machine |

---

## Detailed technical documentation

* [**Architecture & internal design**](docs/architecture.md): Deep dive into Linux user namespaces, ephemeral home staging, path masking, SSH deploy-key injection, and lifecycle sequence.
* [**Capability profiles catalog**](profiles/README.md): Comprehensive reference of all 35+ embedded development stacks and security profiles.

---

## License

MIT License. Developed for robust, unprivileged Linux & WSL sandboxing.
