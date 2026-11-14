# bws — Bubblewrap sandbox launcher

`bws` runs development commands and coding agents in Linux Bubblewrap sandboxes.
It stages a separate home directory and builds mounts, environment settings, and
optional services from JSONC configuration and composable tool profiles.

The workspace is writable by default. Other writable mounts, network access,
SSH forwarding, and desktop integration depend on the effective configuration.
Profiles grant access to installed tools; they do not install or pin toolchains.

## Contents

- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Quick start](#quick-start)
- [Reusable compound profiles](docs/compound_profiles.md)
- [Environment stacks](docs/stacks.md)
- [Configuration reference](docs/configuration.md)
- [Command reference](docs/commands.md)
- [Security boundaries](docs/security.md)
- [Architecture](docs/architecture.md)
- [Frequently asked questions](docs/faq.md)
- [Profile catalog](profiles/README.md)

## Prerequisites

- Linux with usable unprivileged user namespaces.
- Bubblewrap (`bwrap`) installed on the host.
- The tools you want to run, such as Go, Git, or an assistant CLI.
- Go matching [go.mod](go.mod) to build from source.
- tmux and Bash for the default interactive `bws` session.
- Optional: `gh` for deploy-key management, `strace` for learning,
  and `xdg-dbus-proxy` for filtered D-Bus access.

Use `bws doctor` to inspect host prerequisites and configuration.

## Installation

```bash
git clone https://github.com/sarielhp/bws.git
cd bws
tools/install
```

The installer places the binary in `~/bin/bws`. Ensure `~/bin` is in your
shell's PATH. See [commands](docs/commands.md) for optional integrations.

## Quick start

### 1. Choose what the workspace needs

```bash
cd /path/to/project
bws setup
```

One screen lists languages, editors, AI agents, tools, restrictions, and
access (network, git push, GUI). Detected languages are pre-checked; the rest
starts from your previous choices. The details pane shows what each choice
mounts and passes in. Enter saves `.bws/config.jsonc` and starts a tmux/Bash
session. Running `bws` in an unconfigured workspace opens the same screen.

Without a terminal, select directly:

```bash
bws setup -w latex,emacs,claude,codex --no-start
```

### 2. Review access

```bash
bws plan
```

Global configuration remains a baseline and can add permissions. A local
configuration changed outside `bws` (by hand, by an agent, or from a
repository) is refused until you review it and run `bws config trust`.

### 3. Work in the sandbox

```bash
bws                      # tmux/Bash session; start claude, codex, agy, ... inside
bws run -- latexmk paper.tex
```

To run an agent in a disposable Git clone, see `bws gw --help` and the
[Git workflow documentation](docs/commands.md). Offline mode is
`--offline`; SSH forwarding can be disabled with `--no-ssh`.

### 4. Change it later

Run `bws setup` again. It opens with the current choices and keeps settings it
does not manage, such as extra mounts (`bws mount`) and environment entries.

### 5. Going further

Explicit profile selection (`bws init -p ...`), saved compound profiles, and
environment stacks are described in the [command reference](docs/commands.md),
the [compound profile guide](docs/compound_profiles.md), and
[stacks](docs/stacks.md).

## Operational notes

- `--no-init` disables launch-time initialization. Existing projects retain
  their configuration until explicitly changed.
- `plan`, `profile suggest`, and configuration dry runs do not stage homes,
  start agents, write trust records, or initialize projects.
- `bws learn` currently executes its target on the host under tracing.
  Its `--dry-run` prevents saving configuration; it does not prevent execution.
- Writable shared caches and forwarded sockets are part of the access boundary.
  Inspect them along with ordinary file mounts.
- A sandbox does not protect a writable project from destructive commands.
  Keep backups and review agent changes.

## Development

```bash
tools/build          # build bin/bws
tools/gate           # formatting, static analysis, tests, docs drift, build
tools/gate --full    # adds race, govulncheck, gocritic, shadow, revive, dupl
```

The gate follows the Go standards in `~/prog/standards/go` and checks that
`docs/commands.md` matches the CLI surface. Optional long tests are available
through `./tools/test_long`. See [AGENTS.md](AGENTS.md) for repository
conventions and [the prioritized backlog](todo.md) for additional work.

## License

[MIT](LICENSE)
