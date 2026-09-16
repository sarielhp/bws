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
go build -o bws .
./tools/install
```

The installer places the binary in `~/bin/bws`. Ensure `~/bin` is in your
shell's PATH. See [commands](docs/commands.md) for optional integrations.

## Quick start

### 1. Inspect a project

```bash
cd /path/to/project
bws profile suggest
```

Suggestions inspect filenames without executing project code. A Go repository
can match several environments; its contents do not determine whether you want
an editor, an agent, or an offline run.

### 2. Select profiles

```bash
bws init                         # Interactive selection and permission review
# Or select explicitly:
bws init --profile go,git --no-ssh
```

Use profiles appropriate to your project. Explicit selections do not silently
add detected tools. Noninteractive initialization requires `--profile`, a
preset, or `--basic` for detected embedded tool profiles.

### 3. Review access

```bash
bws plan
bws profile show go
```

Global configuration remains a baseline and can add permissions. Existing or
manually changed local configuration requires review followed by
`bws config trust`; do not approve repository-supplied policy without inspecting it.

### 4. Run a command

```bash
bws run -- go test ./...
# Or enter the default tmux/Bash session:
bws
```

To run an agent in a disposable Git clone, see `bws gw --help` and the
[Git workflow documentation](docs/commands.md). Offline mode is
`--offline`; SSH forwarding can be disabled with `--no-ssh`.

### 5. Save and reuse the setup

```bash
bws profile save my-go --dry-run
bws profile save my-go

cd /path/to/another-go-project
bws profile suggest --compound
bws init --profile my-go
```

Saving captures reusable capabilities, not credentials, processes, or tool
versions. The preview reports unsupported settings and machine-specific paths.
Use `bws profile compose` to create a combination directly. Read the
[compound profile guide](docs/compound_profiles.md) for matching rules,
dependency updates, and portability.

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
./tools/verify_build.sh
./tools/audit_lines.rb
```

The verification script formats, vets, tests, and builds. Optional long tests
are available through `./tools/test_long`. See [AGENTS.md](AGENTS.md) for
repository conventions and [the prioritized backlog](todo.md) for additional work.

## License

[MIT](LICENSE)
