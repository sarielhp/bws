# bws — Bubblewrap Sandbox Launcher

[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/badge/Go-1.21+-00ADD8?logo=go)](go.mod)
[![Platform: Linux](https://img.shields.io/badge/Platform-Linux%20%7C%20WSL-FCC624?logo=linux)](https://github.com/sarielhp/bws)

**`bws`** is a fast, declarative, unprivileged Linux sandbox launcher and orchestrator built on top of [**Bubblewrap (`bwrap`)**](https://github.com/containers/bubblewrap) — the gold-standard, unprivileged containerization engine developed by the Flatpak and Red Hat teams.

> **Note on Bubblewrap**: `bws` is a higher-level declarative frontend for [Bubblewrap](https://github.com/containers/bubblewrap). It wraps `bwrap`'s raw namespace primitives into a complete developer workflow with ephemeral home directories, declarative capability profiles, path masking, shell hooks, and automatic SSH/WSL integration.

---

## Key Capabilities

* **Zero-Privilege Sandboxing**: Runs purely in Linux user namespaces (`bwrap` unprivileged mode). No root permissions, setuid binaries, or daemon sockets required.
* **Disposable Ephemeral Homes**: Stages an isolated `$HOME` directory (`/tmp/bws/stage_*`) populated with clean skeleton dotfiles (`.bashrc`, `.profile`, `.tmux.conf`) that disappear when the session ends.
* **Declarative Capability Profiles**: Compose dev stacks and toolchains with a single line (e.g. `profiles: ["go-dev"]` or `profiles: ["secure-agent"]`).
* **Path Masking & Security Hardening**: Neutralize host privilege escalation tools (`no-sudo`), SSH credentials (`no-ssh`), browser cookies (`no-browser`), cloud keys (`no-secrets`), and command history (`no-history`).
* **Automated Smoke Testing**: Verify sandbox integrity and tool accessibility before running code (`bws test <profile>`).
* **Automatic SSH & WSL Integration**: Transparent SSH agent forwarding with on-the-fly GitHub Deploy Key generation via `gh`, X11 display forwarding, and WSL clipboard integration.
* **Safe Host Pass-Through**: Explicit environment variable forwarding (`pass_env`) preserving isolation without secret leakage.

---

## Installation

### From Source

```bash
git clone https://github.com/sarielhp/bws.git
cd bws
make install
```

The binary will be compiled and installed to `~/bin/bws`.

### Prerequisites

* **Linux kernel** 3.8+ with user namespaces enabled (standard on Debian, Ubuntu, Fedora, Arch, and WSL2).
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

## Quick Start

### 1. Launch an Ephemeral Interactive Sandbox

```bash
# Launch an interactive shell in an isolated bubble
bws
```

### 2. Auto-Detect and Initialize a Project Workspace (in local directory)

Inspects the current workspace (detecting Go, Python/UV, Rust, Node, LaTeX, etc.), creates the local `.bws/` subdirectory, and writes a tailored `.bws/config.jsonc` configuration file:

```bash
# Auto-detect language stack and initialize .bws/config.jsonc in current directory
bws init-dev

# Dry-run preview without writing to disk
bws init-dev -n
```

### 3. Run Commands Directly Inside a Sandbox

```bash
# Execute a single command inside a sandbox environment
bws exec -- go test ./...
bws exec -- uv run main.py
```

### 4. Verify Stack Integrity

```bash
# Run multi-command verification tests in an isolated sandbox
bws test go-dev
bws test secure-agent
```

---

## Declarative Capability Profiles

`bws` features a modular **Profile Engine** with over 35 pre-configured development stacks and security profiles.

```bash
# List all available profiles (embedded, global, local)
bws profile list

# Inspect resolved dependency chain, mounts, masked paths, and tests
bws profile show go-dev
bws profile show secure-agent

# Generate a new profile from Homebrew and Firejail rules
bws profile new ripgrep

# Fetch community profiles from GitHub
bws profile fetch zig
bws profile update
```

### Meta-Profiles & Bundles

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

## Security & Path Masking Engine

`bws` provides **Path Masking** to overlay empty `tmpfs` mounts over directories or `/dev/null` over binaries, making host utilities invisible inside the sandbox.

### Built-in Hardening Profiles

* **`no-sudo`**: Blocks `/usr/bin/sudo`, `su`, `pkexec`, `doas`, `gpasswd`, `newgrp`, and masks `/etc/sudoers`.
* **`no-ssh`**: Blocks `~/.ssh` and `/etc/ssh/ssh_config`.
* **`no-browser`**: Blocks browser profiles (`~/.mozilla`, Chrome, Chromium, Brave, Edge) to protect session cookies.
* **`no-email`**: Blocks desktop email databases (`~/.thunderbird`, `~/.config/evolution`, `~/.mutt`, `~/Maildir`).
* **`no-secrets`**: Blocks cloud credentials (`~/.aws`, `~/.azure`, `~/.config/gcloud`, `~/.password-store`, `~/.gnupg`).
* **`no-history`**: Blocks shell history files (`.bash_history`, `.zsh_history`, `.python_history`, `.psql_history`).
* **`secure-agent`**: Zero-trust bundle combining all hardening profiles with the `ai` stack for autonomous coding agents.

---

## Configuration Architecture

`bws` uses a layered, comment-supported **JSONC** configuration hierarchy:

### 1. Global User Configuration (`~/.config/bws/`)
Located in your user home directory according to the XDG standard:
* **`~/.config/bws/config.jsonc`**: Global base settings (preferred editor, default profiles, standard environment pass-through).
* **`~/.config/bws/skeleton/`**: Default dotfiles (`.bashrc`, `.profile`, `.tmux.conf`) copied into every new sandbox.
* **`~/.config/bws/profiles/`**: Custom user-authored capability and security profiles.

### 2. Local Project Configuration (`.bws/` in Workspace Root)
Local configuration is **scoped directly to your current project/repository** and lives inside the `.bws/` directory at the project root (mirroring conventions like `.vscode/` or `.cargo/`):
* **`.bws/config.jsonc`**: Workspace-specific overrides (e.g. declaring `profiles: ["go-dev", "no-secrets"]`, adding project-specific bind mounts, custom environment variables).
* **`.bws/skeleton/`**: Project-specific dotfiles (e.g. a custom `.bws/skeleton/.bashrc` with project aliases or build shortcuts) that overlay on top of the global skeleton.

> **Why `.bws/` instead of `.config/bws/` in projects?**  
> `~/.config/` is strictly a user-home concept (XDG). In a project repository, having a top-level `.config/` folder is ambiguous and can conflict with project build tools. A dedicated `.bws/` folder in the project root is clean, self-contained, easily gitignored or committed, and follows standard tool patterns (`.vscode/`, `.github/`, `.devcontainer/`).

---

### Example Global Configuration (`~/.config/bws/config.jsonc`)

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

### Example Local Project Configuration (`.bws/config.jsonc`)

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

## Skeletons & Ephemeral Homes

Whenever `bws` launches a sandbox:
1. It creates an isolated temporary directory in `/tmp/bws/stage_*`.
2. It copies base dotfiles from the **Global Skeleton** (`~/.config/bws/skeleton/`).
3. It overlays workspace dotfiles from the **Local Skeleton** (`.bws/skeleton/` if present in the project).
4. It dynamically appends profile PATHs and shell hooks to the staged `.bashrc`.
5. When the session terminates, the ephemeral home directory is cleanly removed.

---

## Command Reference

| Command | Description |
| :--- | :--- |
| **`bws`** | Launch an interactive sandbox shell |
| **`bws exec -- <cmd...>`** | Execute a command inside the sandbox |
| **`bws test <profile>`** | Run multi-command verification tests for a profile |
| **`bws init-dev`** | Auto-detect workspace and generate `.bws/config.jsonc` |
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

## License

MIT License. Developed for robust, unprivileged Linux & WSL sandboxing.
