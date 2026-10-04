# CLI command reference

Comprehensive reference for all commands and options in `bws`.

---

## Table of contents

* [Help and informational commands](#help-and-informational-commands)
* [Core execution commands](#core-execution-commands)
* [Current environment](#current-environment)
* [Environment stacks management](#environment-stacks-management)
* [Environment modifiers (mount, bin, copy, path)](#environment-modifiers-mount-bin-copy-path)
* [Configuration management & remote sync](#configuration-management--remote-sync)

---

## Help and informational commands

`bws` renders three help tiers from the same command tree:

* `bws -h` / `bws <command> -h` — concise help: usage, subcommands, and flags.
* `bws --help` / `bws <command> --help` — extended help: adds long descriptions,
  parameter explanations, and examples.
* `bws -H` — extended help as a single-letter alias.

Additional reference topics, and their aliases:

| Invocation | Purpose |
| :--- | :--- |
| `bws help` | Top-level command list |
| `bws help <command>` | Help for a command |
| `bws help flags` | Grouped list of every global flag |
| `bws help examples` / `bws -E` | Every example in one place |
| `bws help topics` | List the available help topics |
| `bws help man` | The complete reference manual |
| `bws manpage` | Print the roff manual page |
| `bws manpage --man-install` | Install it under the user's data directory for `man(1)` |
| `bws manpage --man-uninstall` | Remove a page installed this way |
| `bws docs` | Generate Markdown documentation (developer command, hidden) |

`bws config completion <shell>` (aliases `comp`) generates or installs shell
tab-completion scripts for `bash`, `zsh`, and `fish`.

---

## Core execution commands

### `bws [flags]`
Launch an interactive sandbox shell in the current directory.

| Flag | Short | Default | Description |
| :--- | :--- | :--- | :--- |
| `--verbose` | `-v` | `false` | Print detailed bwrap arguments, staging paths, and mount plans to stderr |
| `--force` | `-f` | `false` | Skip the safety prompt when the directory contains more than `max_file_count` files |
| `--no-net` | `-N`, `--offline` | `false` | Completely block network access (air-gapped network namespace) |
| `--no-color` | | `false` | Disable ANSI color in all output |

These are global flags: every subcommand accepts them. Run `bws help flags` for
the complete grouped list.

```bash
bws            # Interactive sandbox (read-write workspace)
bws -v         # Show full debug information before launching
bws -N         # Air-gapped interactive sandbox (no network access)
```

---

### `bws run <command> [args...]`
Run an arbitrary command inside the sandbox and exit with its status code (alias: `bws exec`).

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

Verbose logging is the global `-v`, not a `git-workflow` option.

```bash
bws gw                                      # Interactive shell in a disposable clone
bws gw agy                                  # Run Antigravity autonomously
bws gw -b fix-auth -- agy "Fix OAuth bug"   # Run agent on a named branch
bws gw --stash                              # Auto-stash dirty tree before starting
```

Upon sandbox exit, changes are fetched back to the host and presented with an interactive Merge/Squash/Keep/Discard menu.

#### `bws gw list [--merged | --unmerged]`
List `bws-agent-*` branches with commit info and merge status (alias: `ls`).

| Option | Description |
| :--- | :--- |
| `--merged` | List only merged agent branches |
| `--unmerged` | List only unmerged agent branches |

#### `bws gw prune [-a] [-n]`
Remove merged or abandoned `bws-agent` branches and clean up leftover `/tmp/bws/agent_*` directories (aliases: `clean`, `rm`).

| Option | Description |
| :--- | :--- |
| `-a`, `--all` | Remove all agent branches, including unmerged/abandoned |
| `-n`, `--dry-run` | Preview branches and temp directories without deleting |

```bash
bws gw list --unmerged
bws gw prune -n
bws gw prune -a
```

---

### `bws test <target>`
Run automated smoke tests for a profile inside an isolated sandbox.

```bash
bws test python
bws test rust
bws test node
```

---

### `bws learn [options] [--] <cmd> [args...]`
Learn required bind mounts, binary PATH additions, and sandbox features dynamically from an interactive shell session or command, with smart hierarchy-aware diffing and live config merging.

| Option | Description |
| :--- | :--- |
| `-n`, `--dry-run` | Preview discovered additions/deltas without saving |
| `-p`, `--profile <name>` | Save discovery as a reusable capability profile |

The global `-g`/`-l` flags select the target config, and `-v` enables verbose
logging; they are not `learn`-specific. There is no `--force` on `learn`.

#### Tracing an entire interactive session (recommended)
Drop into a traced interactive shell, run all commands, builds, and tools needed by your workflow, then exit. `bws learn` traces the entire process tree (`strace -f`) across all subcommands and synthesizes the required mounts and features upon exit:

```bash
# Start an interactive traced session and save as a profile
bws learn -p myproject bash
# Inside the session: run builds, tests, scripts, tools...
$ make build
$ pytest tests/
$ ./scripts/fetch_data.sh
$ exit
# Done! Profile saved to profiles/myproject.json

# Or trace an interactive session and merge directly into .bws/config.jsonc
bws learn bash

# Preview discovered additions from a session without modifying files
bws learn -n bash
```

#### Tracing single commands
```bash
bws learn python train.py              # Learn Python dependencies and merge into .bws/config.jsonc
bws learn -n -- pytest -k test_foo     # Preview discovered delta without modifying config
bws learn -p myapp ./bin/myapp         # Trace binary and generate profiles/myapp.json
bws learn -g cargo build               # Learn and merge additions into global configuration
```

---

## Current environment

### `bws init [options] [dir]`
Inspect workspace markers, suggest compound profiles, and generate a reference-based `.bws/config.jsonc` (aliases: `setup`, `init-dev`). Existing projects remain unchanged without `--force`. Noninteractive use requires an explicit selection or `--basic`.

| Option | Description |
| :--- | :--- |
| `-s`, `--stack <name>` | Select an environment stack by name (e.g. `go-agent`, `python-uv`) |
| `-n`, `--dry-run` | Print generated config to stdout without writing |
| `--preset <stack>` | Select a preset stack (`go`, `python`, `rust`, `node`, `latex`, `agent`, `all`) |
| `-p`, `--profile <name>`| Include tool profile(s) (repeatable); does not add detected profiles |
| `--opencode` | Force inclusion of OpenCode config directories |
| `--basic` | Select detected embedded tool profiles |
| `-y`, `--yes` | Confirm the selected initialization plan |

```bash
bws init                        # Suggest and select interactively
bws init --stack go-agent       # Initialize with Go agent persona stack
bws init -p go-dev -n            # Preview the selected configuration
bws init --preset python        # Explicitly select Python stack
bws init -p docker,pandoc       # Select these profiles explicitly
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

### `bws doctor`
Inspect and validate sandbox environment, configuration, mounts, and prerequisites. Runs seven diagnostic checks (kernel user namespaces, critical host tools, configuration and profile validity, dead bind mounts, workspace symlink boundary audit, mount masking conflicts, and SSH agent socket health).

```bash
bws doctor                      # Run environment diagnostics and health report
bws doctor -v                   # Run diagnostics with verbose output
```

---

### `bws add <name...> [-g | -l] [-c | --create]`
Add and enable one or more capability profiles in the current environment. Alias: `enable`.
```bash
bws add python                  # Enable python in the current workspace config
bws add python node rust        # Enable multiple profiles at once
bws add docker -g               # Enable docker globally
bws add -c fish                 # Synthesize the profile if missing, then enable it
```

Use `-g`/`-l` to target the global config or the local workspace config, and
`-c`/`--create` to synthesize a profile that does not yet exist.

---

### `bws rm <name...> [-g | -l]`
Remove and disable one or more capability profiles from the current environment. Aliases: `del`, `remove`, `disable`.
```bash
bws rm python                   # Remove python from the current workspace config
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
Update all installed global profiles from the remote repository (alias: `sync`).
```bash
bws profile update
```

### `bws profile test <name>`
Run all verification and smoke tests declared by a profile inside a sandbox.
```bash
bws profile test python
```

### `bws profile add <name...> [-g | -l] [-c | --create]`
Add and enable one or more capability profiles in the local or global config (alias: `enable`).
```bash
bws profile add python -l
bws profile add -c fish -g       # Synthesize if missing, then enable
```

### `bws profile rm <name...> [-g | -l]`
Remove and disable one or more capability profiles. Aliases: `del`, `remove`, `disable`.
```bash
bws profile rm python -l
```

### `bws profile save <name> [options]`
Save effective global, trusted local, and profile capabilities as a reusable compound profile. Aliases: `snap`, `export`. Saving does not activate it.

| Option | Description |
| :--- | :--- |
| `-d`, `--desc <text>` | Description |
| `-n`, `--dry-run` | Preview JSON and limitations without writing |
| `-y`, `--yes` | Confirm saving after reviewing the preview |
| `--flatten` | Materialize capabilities, dropping dependency references |
| `--allow-machine-paths` | Acknowledge machine-specific absolute paths |
| `--omit <field>` | Acknowledge an unsupported configuration field (repeatable) |
| `--match <file>` | Match project filenames (repeatable) |
| `--no-detect` | Do not derive project matching rules |

```bash
bws profile save my-env                       # Save a compound profile
bws profile save ml-env -d "ML stack setup"   # Set a custom description
bws profile save my-env --dry-run             # Preview JSON without writes
bws profile save my-env --flatten             # Materialize supported settings
```

There is no `-f`/`--force` and no `-g`/`-l` on `profile save`; confirmation is
controlled by `-y`, and the destination is determined by the reviewed preview.

---

### `bws profile compose <name> --profiles <names> [options]`
Combine explicit profiles into a compound profile without activating it.

| Option | Description |
| :--- | :--- |
| `-d`, `--desc <text>` | Description |
| `-n`, `--dry-run` | Preview JSON and limitations without writing |
| `-y`, `--yes` | Confirm saving after reviewing the preview |
| `--flatten` | Materialize capabilities, dropping dependency references |
| `--allow-machine-paths` | Acknowledge machine-specific absolute paths |
| `--omit <field>` | Acknowledge an unsupported configuration field (repeatable) |
| `--match <file>` | Match project filenames (alternatives; repeatable) |
| `--no-detect` | Do not derive project matching rules |
| `-p`, `--profiles <names>` | Dependencies (repeatable or comma-separated) |

```bash
bws profile compose go-agent --profiles go,opencode,git --no-ssh --match go.mod
```

### `bws profile suggest [directory] [options]`
Explain which profiles match a directory without changing configuration.

| Option | Description |
| :--- | :--- |
| `--json` | Print machine-readable suggestions |
| `--compound` | Suggest only compound profiles |

```bash
bws profile suggest --compound
bws profile suggest /path/to/project --json
```

### `bws profile review <name> [--accept]`
Review changed dependencies before approval.

| Option | Description |
| :--- | :--- |
| `--accept` | Approve displayed dependency changes |

```bash
bws profile review go-agent
bws profile review go-agent --accept
```

See [compound profiles](compound_profiles.md) for matching rules, save limitations,
portability acknowledgments, dependency review, and noninteractive behavior.

## Environment stacks management

Manage, inspect, save, and update persona environment stacks. See [Environment stacks](stacks.md) for concepts, genesis invariants, and architecture.

### `bws stack list [options]`
List all registered seed stacks and user-saved stacks (alias: `ls`).

```bash
bws stack list                      # List all stacks grouped by source
bws stack list -c runtime           # Filter stacks by category
bws stack list --json               # Print JSON output
```

### `bws stack show <name>`
Inspect full details, constituent profiles, features, environment variables, digest, and provenance for a stack (aliases: `info`, `view`).

```bash
bws stack show go-agent
bws stack show latex-review
```

### `bws stack save <name> [options]`
Save the current active workspace as a reusable user stack in `~/.config/bws/stacks/<name>.json` (alias: `snap`). Enforces the genesis invariant (must be run in an active workspace), strips machine-specific paths and secrets, runs profile smoke tests, and computes a cryptographic approval digest.

| Option | Description |
| :--- | :--- |
| `-t`, `--title <text>` | Human-readable title for the stack |
| `-d`, `--desc <text>` | Description of the stack persona |
| `--no-verify` | Bypass smoke tests for constituent profiles |

```bash
bws stack save my-persona -t "My Custom Persona" -d "Custom dev environment"
bws stack save my-persona --no-verify
```

### `bws stack update [options]`
Pull upstream stack definition changes into the current workspace (aliases: `upgrade`, `pull`). Diffs workspace against upstream stack definition, displays permission changes, prompts for review, and applies atomically with a `.bak` backup.

| Option | Description |
| :--- | :--- |
| `-n`, `--dry-run` | Preview upstream changes without applying |
| `-y`, `--yes` | Confirm and apply updates without interactive prompt |

```bash
bws stack update --dry-run          # Preview upstream changes
bws stack update                    # Review and apply upstream updates
```

## Environment modifiers (mount, bin, copy, path)

### `bws mount add <host-path> [dest] [-g | -l] [--rw] [--ro]`
Add a persistent bind mount to configuration (defaults to local workspace `-l`).

Bind mounts default to read-only (`binds_ro`). Pass `--rw` to explicitly configure a read-write mount (`binds_rw`). The `--ro` flag is supported as a backward-compatible no-op.

```bash
bws mount add /data/models /models        # Read-only mount (default)
bws mount add /opt/tools /tools --rw      # Read-write mount
bws mount add /usr/share/dict --ro        # Read-only mount (explicit)
bws mount add /opt/global-tools -g        # Global read-only mount
```

#### Symlink auto-resolution
When `<host-path>` points to a symbolic link on the host, `bws` detects it via `os.Lstat` and canonicalizes the destination using `filepath.EvalSymlinks`:
- **Dangling symlinks**: If the target does not exist, the command exits with code 1 and prints a descriptive error.
- **Internal workspace symlinks**: If the canonical target resides inside the current workspace directory (`currentDir`), `bws` reports that the target is already accessible within the workspace and exits cleanly without modifying configuration.
- **External symlinks**: If the canonical target is outside the workspace, paths located under `$HOME` are tokenized as `@@HOME@@`. The tool prints `Resolved symlink <hostPath> -> <target>` and persists the canonical target into `binds_ro` (or `binds_rw` if `--rw` is set).

#### Nested mount semantics
The runtime mount engine structures Bubblewrap arguments to support nested mounts (read-only directories within read-write mounts, and read-write subdirectories within read-only mounts):
- Base parent mounts (`sandbox home`, host temporary directory `/tmp`, and the workspace directory `currentDir`) are mounted prior to user-defined mounts. This ensures user mounts targeting subdirectories of the workspace are not shadowed by `--bind currentDir currentDir`.
- User bind mounts are topologically sorted so parent directories strictly precede child subdirectories (`depth parent < depth child`).
- When path depths are equal, read-only mounts precede read-write mounts, allowing read-write overrides over read-only roots.
- Security masking (`addMaskArgs`) executes strictly last, ensuring security barriers take precedence over all user mounts.

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

### `bws bin add <host-path> [-g | -l]`
Expose a single executable or script inside the sandbox as a read-only bind mount on `$PATH`. If the host executable is outside `~/bin`, it is automatically mapped to `~/bin/<basename>` inside the sandbox.
```bash
bws bin add ~/bin/agy-run-wild         # Expose script in local workspace
bws bin add /opt/tools/myprog -g       # Expose tool globally on sandbox PATH
```

### `bws bin rm <name-or-path> [-g | -l]`
Remove an exposed executable from configuration by binary name or host path (aliases: `del`, `delete`, `remove`).
```bash
bws bin rm agy-run-wild
```

### `bws bin list`
List all configured executable binaries exposed in the sandbox (alias: `ls`).
```bash
bws bin list
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

### `bws config trust`

Approve the current contents of local workspace configuration and `.bws/profiles/` after reviewing them. Required for existing configurations, manual edits, and profiles received from another source. Approval is recorded outside the project; sandboxed code cannot approve host configuration for a later launch.

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

### `bws undo [-g | -l]`
Restore a configuration file to its contents immediately before the most recent `bws` write (alias: `revert`). One backup slot is kept per config file, at `<config>.bak`; deeper history is not retained. After an undo the restored local file is no longer trusted, so run `bws config trust` in its workspace after reviewing it.
```bash
bws undo          # Restore the local workspace config
bws undo -g       # Restore the global config
```

### `bws config push <user@host:>`
Copy global configuration and themes to a remote host via SCP (aliases: `scp`, `sync`).
```bash
bws config push user@server:
```

### `bws config completion <shell>`
Generate or install shell tab-completion scripts (alias: `comp`).
```bash
bws config completion bash
bws config completion zsh
```
Run `bws config completion --help` for install options.
