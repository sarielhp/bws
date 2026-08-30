# CLI command reference

Comprehensive reference for all commands and options in `bws`.

---

## Table of contents

* [Core execution commands](#core-execution-commands)
* [Workspace initialization](#workspace-initialization)
* [Environment status & plan](#environment-status--plan)
* [Capability profile management](#capability-profile-management)
* [Environment modifiers (mount, copy, path)](#environment-modifiers-mount-copy-path)
* [Configuration management & remote sync](#configuration-management--remote-sync)

---

## Core execution commands

### `bws [flags]`
Launch an interactive sandbox shell in the current directory.

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--verbose` | `-v` | `false` | Print detailed bwrap arguments, staging paths, and mount plans to stderr |
| `--force` | `-f` | `false` | Skip the safety prompt when the directory contains more than `max_file_count` files |
| `--no-net` | `-N`, `--offline` | `false` | Completely block network access (air-gapped network namespace) |

```bash
bws            # Interactive sandbox (read-write workspace)
bws -v         # Show full debug information before launching
bws -N         # Air-gapped interactive sandbox (no network access)
```

---

### `bws run [flags] <cmd> [args...]`
Execute a single command inside the sandbox and exit with the command's status code (alias: `bws exec`).

```bash
bws run go test ./...
bws run python -m pytest
bws run -N pytest               # Run tests completely offline
```

---

### `bws git-workflow [options] [-- command [args...]]`
Run an isolated, disposable agent session in a temporary Git clone (aliases: `gw`, `worktree`).

| Option | Description |
| :--- | :--- |
| `-b`, `--branch <name>` | Custom target branch name for the agent session |
| `--stash` | Automatically stash uncommitted changes before starting |
| `--allow-dirty` | Allow starting even if the working tree has uncommitted changes |
| `-v`, `--verbose` | Enable verbose diagnostic logging |

```bash
bws gw                                      # Interactive shell in a disposable clone
bws gw agy                                  # Run Antigravity autonomously
bws gw -b fix-auth -- agy "Fix OAuth bug"   # Run agent on a named branch
bws gw --stash                              # Auto-stash dirty tree before starting
```

Upon sandbox exit, changes are fetched back to the host and presented with an interactive Merge/Squash/Keep/Discard menu.

---

### `bws test <target>`
Run automated smoke tests for a profile inside an isolated sandbox.

```bash
bws test python
bws test rust
bws test node
```

---

## Current environment

### `bws init [options] [dir]`
Inspect workspace repository markers and generate `.bws/config.jsonc` (aliases: `setup`, `init-dev`). If `[dir]` is omitted, defaults to current directory.

| Option | Description |
| :--- | :--- |
| `-n`, `--dry-run` | Preview generated JSONC on stdout without creating `.bws/` |
| `--preset <name>` | Force stack preset (`go`, `python`, `rust`, `node`, `latex`, `agent`, `all`) |
| `-p`, `--profile <name>`| Comma-separated extra capability profile names to include |

```bash
bws init                        # Auto-detect current directory
bws init -n                     # Dry run: preview generated JSONC
bws init --preset python        # Explicitly select Python stack
bws init -p docker,pandoc       # Include extra tool profiles
bws init /path/to/project       # Initialize specific directory
```

---

### `bws status [all]`
Display active environment status and installed capability profiles (aliases: `info`, `current`). Pass `all` to see the full execution plan.

```bash
bws status                      # Show installed profiles in resolved order
bws status all                  # Show full bwrap execution plan and mounts
```

---

### `bws plan`
Display the complete resolved sandbox execution plan, mounts, variables, and flags (Terraform-style dry-run inspector).

```bash
bws plan
```

---

### `bws add <profile...> [-g | -l]`
Add and enable one or more capability profiles in the current environment (defaults to local workspace `-l`; pass `-g` for global). Alias: `enable`.
```bash
bws add python                  # Enable python in local workspace
bws add python node rust        # Enable multiple profiles at once
bws add docker -g               # Enable docker globally
```

---

### `bws rm <profile...> [-g | -l]`
Remove and disable one or more capability profiles from the current environment. Aliases: `del`, `remove`, `disable`.
```bash
bws rm python                   # Remove python from local workspace
bws rm node rust                # Remove multiple profiles at once
```

### `bws profile list`
List all locally registered and embedded profiles (alias: `ls`).
```bash
bws profile list
```

### `bws profile search <query>`
Search profiles across embedded catalog, local files, and Homebrew registry (alias: `find`).
```bash
bws profile search python
```

### `bws profile show <name>`
Display resolved dependency chain, mounts, environment, and smoke tests for a profile (aliases: `view`, `cat`, `info`).
```bash
bws profile show python
```

### `bws profile generate <name> [-g | -l]`
Synthesize a new profile definition from Homebrew Formula API and Firejail intelligence (aliases: `create`, `new`, `gen`, `synthesize`).
```bash
bws profile generate ripgrep
```

### `bws profile fetch <name> [-g | -l]`
Download a community profile definition from GitHub repository (aliases: `pull`, `get`, `install`).
```bash
bws profile fetch zig
```

### `bws profile update`
Synchronize all installed global profiles from GitHub repository (alias: `sync`).
```bash
bws profile update
```

---

## Environment modifiers (mount, copy, path)

### `bws mount add <host-path> [dest] [-g | -l] [--ro]`
Add a persistent bind mount to configuration (defaults to local workspace `-l`).
```bash
bws mount add /opt/tools /tools           # Read-write mount
bws mount add /data/models /models --ro   # Read-only mount
bws mount add /opt/global-tools -g        # Global mount
```

### `bws mount rm <host-path> [-g | -l]`
Remove a bind mount by its host path (aliases: `del`, `delete`, `remove`).
```bash
bws mount rm /opt/tools
```

### `bws mount list`
List configured bind mounts (alias: `ls`).
```bash
bws mount list
```

---

### `bws copy add <host-path> [-g | -l]`
Add a host file to be copied into the staged ephemeral `$HOME` before launch.
```bash
bws copy add ~/bin/custom_helper.sh
```

### `bws copy rm <host-path> [-g | -l]`
Remove a path from the copy list (aliases: `del`, `delete`, `remove`).
```bash
bws copy rm ~/bin/custom_helper.sh
```

### `bws copy list`
List configured copy paths (alias: `ls`).
```bash
bws copy list
```

---

### `bws path add <directory> [-g | -l]`
Add a directory to the sandbox `PATH`.
```bash
bws path add /extc/opt/custom/bin
```

### `bws path rm <directory> [-g | -l]`
Remove a directory from the sandbox `PATH` (aliases: `del`, `delete`, `remove`).
```bash
bws path rm /extc/opt/custom/bin
```

### `bws path list`
List configured extra `PATH` directories (alias: `ls`).
```bash
bws path list
```

---

## Configuration management & remote sync

### `bws config show [-g | -l]`
Display raw JSONC content of config file (aliases: `cat`, `view`).
```bash
bws config show -g
bws config show -l
```

### `bws config set <key> <value> [-g | -l]`
Set a configuration key value in local or global configuration without opening an editor.
```bash
bws config set enable_proxy true       # Enable proxy in local workspace
bws config set enable_ssh true -g      # Enable SSH forwarding globally
bws config set max_file_count 25000    # Set file count safety limit
```

### `bws config get <key> [-g | -l]`
Read a configuration key value from local or global configuration.
```bash
bws config get enable_proxy
bws config get max_file_count -g
```

### `bws config unset <key> [-g | -l]`
Remove a configuration key from local or global configuration.
```bash
bws config unset enable_proxy
```

### `bws config edit [-g | -l]`
Open configuration file in `$EDITOR`.
```bash
bws config edit -l    # Edit local .bws/config.jsonc
bws config edit -g    # Edit global ~/.config/bws/config.jsonc
```

### `bws config where`
Print filepaths of active global and local configuration files (alias: `paths`).
```bash
bws config where
```

### `bws config reset [-g | -l]`
Reset configuration file to clean defaults (backs up existing to `.bak`; alias: `init`).
```bash
bws config reset -g
bws config reset -l
```

### `bws config push <user@host:>`
Copy global configuration and themes to a remote host via SCP (aliases: `scp`, `sync`).
```bash
bws config push user@server:
```
