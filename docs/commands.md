# CLI command reference

Comprehensive reference for all commands and options in `bws`.

---

## Table of contents

* [Core execution commands](#core-execution-commands)
* [Workspace initialization](#workspace-initialization)
* [Capability profile management](#capability-profile-management)
* [Configuration inspection & editing](#configuration-inspection--editing)
* [Custom bind & copy management](#custom-bind--copy-management)
* [Remote synchronization](#remote-synchronization)

---

## Core execution commands

### `bws [flags]`
Launch an interactive sandbox shell in the current directory.

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--verbose` | `-v` | `false` | Print detailed bwrap arguments, staging paths, and mount plans to stderr |
| `--force` | `-f` | `false` | Skip the safety prompt when the directory contains more than `max_file_count` files |
| `--readonly` | `-r` | `false` | Mount the current workspace read-only inside the sandbox |
| `--no-net` | `-N` | `false` | Completely block network access (air-gapped network namespace) |
| `--info` | | `false` | Dry run: display the resolved bwrap argument plan without launching |

```bash
bws            # Interactive sandbox (read-write workspace)
bws -r         # Interactive sandbox (read-only workspace)
bws -v         # Show full bwrap argument list before launching
bws -N         # Air-gapped interactive sandbox (no internet or host localhost access)
bws --info     # Dry run: print the bwrap plan without executing it
```

---

### `bws exec [flags] -- <cmd...>`
Execute a single command inside the sandbox and exit with the command's status code.

```bash
bws exec -- go test ./...
bws exec -- python -m pytest
bws exec -r -- uv run main.py
bws exec -N -- pytest               # Run tests completely offline
```

---

### `bws test <profile>`
Run automated smoke tests for a profile inside an isolated sandbox.

```bash
bws test go-dev
bws test secure-agent
bws test no-sudo
```

---

## Workspace initialization

### `bws init-dev [options] [dir]`
Inspect workspace repository markers and generate `.bws/config.jsonc`. If `[dir]` is omitted, defaults to current directory.

| Option | Description |
| :--- | :--- |
| `-n`, `--dry-run` | Preview generated JSONC on stdout without creating `.bws/` |
| `--preset <name>` | Force stack preset (`go`, `python`, `rust`, `node`, `latex`, `agent`, `all`) |
| `-p`, `--profiles <list>`| Comma-separated extra capability profile names to include (e.g. `docker,quarto`) |

```bash
bws init-dev                        # Auto-detect current directory
bws init-dev -n                     # Dry run: preview generated JSONC
bws init-dev --preset python        # Explicitly select Python stack
bws init-dev -p docker,quarto       # Include extra tool profiles
bws init-dev /path/to/project       # Initialize specific directory
```

---

## Capability profile management

### `bws profile search <query>`
Search profiles across embedded catalog, local files, and Homebrew registry.
```bash
bws profile search python
bws profile search ripgrep
```

### `bws profile list`
List all locally registered and embedded profiles.
```bash
bws profile list
```

### `bws profile show <name>`
Display resolved dependency chain, mounts, and smoke tests for a profile.
```bash
bws profile show go-dev
```

### `bws profile new <name>`
Synthesize a new profile using Homebrew Formula API and Firejail intelligence.
```bash
bws profile new ripgrep
```

### `bws profile fetch <name>`
Download a community profile from GitHub or synthesize from Homebrew.
```bash
bws profile fetch zig
```

### `bws profile update`
Synchronize all installed global profiles from GitHub repository.
```bash
bws profile update
```

---

## Configuration inspection & editing

### `bws conf info`
Show merged bwrap argument plan (dry run). Note: `<ephemeral staged home>` is shown as a placeholder for the runtime `/tmp/bws/stage_*` directory.
```bash
bws conf info
```

### `bws conf edit [-g | -l]`
Open global or local config in `$EDITOR`.
```bash
bws conf edit -g    # Edit ~/.config/bws/config.jsonc
bws conf edit -l    # Edit .bws/config.jsonc in current workspace
```

### `bws conf show [-g | -l]`
Display raw JSONC content of config file (defaults to global if no flag given).
```bash
bws conf show -g
bws conf show -l
```

---

## Custom bind & copy management

### `bws cbind add <host-path> [dest] [-g | -l]`
Add a persistent read-write bind mount to configuration.

The mount is appended to `binds_rw` in your global (`-g`) or local project (`-l`) config and will be mounted on every subsequent `bws` launch.

| Flag | Description |
| :--- | :--- |
| `-g` | Add to global config (`~/.config/bws/config.jsonc`) |
| `-l` | Add to local project config (`.bws/config.jsonc`) |

```bash
bws cbind add /opt/tools -g          # Bind /opt/tools globally
bws cbind add /data/fixtures -l      # Bind /data/fixtures for this project only
```

---

### `bws ccopy add <host-path> [-g | -l]`
Add a file to be copied into the staged ephemeral `$HOME` at sandbox launch.

Unlike a bind mount, a copied file appears as a regular independent file in the sandbox home rather than a symlink to the host.

```bash
bws ccopy add ~/bin/helper -g        # Copy ~/bin/helper into staged $HOME at launch
```

---

## Remote synchronization

### `bws scp <user@host:>`
Copy global configuration and themes to a remote host.
```bash
bws scp user@server:
```
