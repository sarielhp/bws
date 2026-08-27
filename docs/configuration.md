# Configuration reference

`bws` uses a layered, comment-supported **JSONC** configuration hierarchy.

---

## Table of contents

* [Configuration hierarchy](#configuration-hierarchy)
* [Configuration schema & keys](#configuration-schema--keys)
* [Global configuration example](#global-configuration-example)
* [Local workspace configuration example](#local-workspace-configuration-example)
* [Dynamic environment variable expansion](#dynamic-environment-variable-expansion)
* [Skeletons & dotfile overlays](#skeletons--dotfile-overlays)

---

## Configuration hierarchy

1. **Global user configuration** (`~/.config/bws/config.jsonc`): Base defaults applied to all sandboxes across your system.
2. **Local workspace configuration** (`.bws/config.jsonc`): Workspace-specific overrides scoped to the current project repository.

### Merging semantics

* **Maps & hashes** (`env`): Deep merged. Local keys override global keys.
* **Lists & arrays** (`profiles`, `mask`, `pass_env`, `path`, `binds_rw`, `binds_ro`): Merged and deduplicated while preserving declaration order.
* **Tokens**: `@@HOME@@` is dynamically expanded to the target sandbox user home directory at launch time.

---

## Configuration schema & keys

| Key | Type | Description |
| :--- | :--- | :--- |
| **`profiles`** | `[]string` | Active capability and security profile names |
| **`pass_env`** | `[]string` | Host environment variable names to safely pass through |
| **`env`** | `map[string]string`| Environment variables to set inside sandbox ($VAR expansion supported) |
| **`path`** | `[]string` | Extra directories to prepend to in-sandbox `$PATH` |
| **`binds_rw`** | `[][2]string` | Read-write bind mounts `[host_path, sandbox_path]` |
| **`binds_ro`** | `[][2]string` | Read-only bind mounts `[host_path, sandbox_path]` |
| **`mask`** | `[]string` | Paths to mask (`tmpfs` for directories, `/dev/null` for files) |
| **`features`** | `object` | Feature toggles (`enable_ssh`, `enable_x11`, `enable_wsl`) |
| **`max_file_count`** | `int` | Maximum files allowed in directory before safety prompt (default: 1000) |

---

## Global configuration example

`~/.config/bws/config.jsonc`:

```jsonc
{
  // Default capability profiles active across all projects
  "profiles": [
    "editor"
  ],

  // Pass-through safe operational environment variables from host
  "pass_env": [
    "USER", "LOGNAME", "SHELL", "TERM", "LANG", "LC_ALL"
  ],

  // Sandbox environment variables
  "env": {
    "HOME": "@@HOME@@",
    "EDITOR": "emacs -nw",
    "VISUAL": "$EDITOR"
  },

  // Base read-only protection for host binaries
  "binds_ro": [
    ["~/.local", "@@HOME@@/.local"]
  ]
}
```

---

## Local workspace configuration example

`.bws/config.jsonc`:

```jsonc
{
  // Stacks required specifically for this project
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
    ["/data/fixtures", "/data/fixtures"]
  ]
}
```

---

## Dynamic environment variable expansion

Values in the `env` map support dynamic `$VAR` expansion:
* `"VISUAL": "$EDITOR"` resolves `$EDITOR` dynamically at runtime.
* `"GOPATH": "@@HOME@@/.go"` resolves `@@HOME@@` to the ephemeral home path.

---

## Skeletons & dotfile overlays

`bws` provisions ephemeral dotfiles during startup:
1. Base dotfiles from `~/.config/bws/skeleton/` (`.bashrc`, `.profile`, `.tmux.conf`).
2. Project-specific dotfiles from `.bws/skeleton/` (if present in the workspace root).
3. Dynamic PATH additions and prompt settings are appended automatically to `.bashrc`.
