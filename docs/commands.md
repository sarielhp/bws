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

### `bws`
Launch an interactive sandbox shell in current directory.
```bash
bws
bws -v    # Verbose debug output
bws -f    # Bypass file count safety checks
```

### `bws exec -- <cmd...>`
Execute a single command inside the sandbox and exit.
```bash
bws exec -- go test ./...
bws exec -- python -m pytest
```

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
Inspect workspace repository markers and generate `.bws/config.jsonc`.
```bash
bws init-dev                        # Auto-detect current directory
bws init-dev -n                     # Dry run: preview generated JSONC
bws init-dev --preset python        # Explicitly select Python stack
bws init-dev -p docker,quarto       # Include extra tool profiles
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
Show merged bwrap argument plan (dry run).
```bash
bws conf info
```

### `bws conf edit [-g | -l]`
Open global or local config in `$EDITOR`.
```bash
bws conf edit -g
bws conf edit -l
```

### `bws conf show [-g | -l]`
Display raw JSONC content of config file.
```bash
bws conf show -g
bws conf show -l
```

---

## Custom bind & copy management

### `bws cbind add <host-path> [dest] [-g | -l]`
Add a bind mount to global or local config.
```bash
bws cbind add /opt/tools -g
```

### `bws ccopy add <host-path> [-g | -l]`
Add a file to the copy list.
```bash
bws ccopy add ~/bin/helper -g
```

---

## Remote synchronization

### `bws scp <user@host:>`
Copy global configuration and themes to a remote host.
```bash
bws scp user@server:
```
