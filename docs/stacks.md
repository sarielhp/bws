# Environment stacks

Stacks provide curated, persona-driven environment baselines for `bws` workspaces.

---

## Concept and architecture

A **Stack** represents a complete developer persona baseline (such as `go-agent`, `latex-review`, `python-uv`, or `rust-dev`).

Where individual **Profiles** serve as granular, additive capability modules (e.g. `go`, `git`, `neovim`, `copilot`), a Stack establishes the cohesive foundation for an entire workflow.

```
+-------------------------------------------------------------+
|                  Local Workspace Overrides                  |
|          (bws add <profile>, local binds & env vars)        |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
|                     Base Persona Stack                      |
|         (e.g. go-agent: go-dev + secure-agent + env)        |
+-------------------------------------------------------------+
                              |
                              v
+-------------------------------------------------------------+
|               Global Baseline Configuration                 |
|                   (~/.config/bws/config.jsonc)              |
+-------------------------------------------------------------+
```

### Key properties

1. **Curated base layer**: When a stack is configured in `.bws/config.jsonc`, `bws` loads the stack definition, merges its constituent base profiles, and applies persona-specific features and environment variables before local overrides.
2. **Genesis invariant**: User stacks cannot be authored in vacuum or free-floating JSON. They can only originate from an active, verified `bws` workspace using `bws stack save <name>`.
3. **Automated sanitization & smoke tests**: Saving a stack verifies that constituent profiles pass automated sandbox verification tests, strips machine-specific absolute paths, redacts credentials, and computes a cryptographic SHA-256 approval digest.
4. **Pull/upgrade workflow**: Upstream stack changes do not silently alter existing workspaces. Workspaces pin the approved digest in `reviewed_stack`. Running `bws stack update` previews differences and applies upstream updates atomically with a `.bak` backup.

---

## Seed stacks catalog

`bws` includes four curated seed stacks embedded directly in the binary:

| Stack | Category | Constituent profiles | Description |
| :--- | :--- | :--- | :--- |
| `go-agent` | Agents & Autonomous | `go-dev`, `secure-agent` | Hardened Go AI agent environment with Go compiler, Gopls, Git, and secret isolation |
| `latex-review` | Document & Publishing | `latex-dev`, `no-history`, `no-secrets` | LaTeX authoring, inspection, and compilation sandbox for manuscript reviews |
| `python-uv` | Language & Runtime | `python-dev` | High-performance Python workspace with uv package manager, Python 3, Git, and editor |
| `rust-dev` | Language & Runtime | `rust-dev` | Safe Rust developer environment with cargo, rustc, git, and editor |

---

## Initializing workspaces with stacks

### Explicit selection

Initialize a directory directly with a named stack:

```bash
bws init --stack go-agent -y
```

This generates `.bws/config.jsonc` specifying the base stack and pinning its approved digest:

```jsonc
{
  "stack": "go-agent",
  "reviewed_stack": {
    "source": "embedded",
    "sha256": "12ae2d0cb72a053e77c6a34e79eb79c45deef35d15ce908651af2439d1f951c7"
  }
}
```

### Intelligent marker ranking

When `bws init` is invoked in an existing project without arguments, it scans workspace files and ranks matching stacks based on detected toolchain markers:

* `go.mod`, `go.work`, or `*.go` ranks `go-agent` first.
* `pyproject.toml`, `requirements.txt`, or `*.py` ranks `python-uv` first.
* `Cargo.toml` or `*.rs` ranks `rust-dev` first.
* `*.tex` or `latexmkrc` ranks `latex-review` first.

### Empty directory catalog

In empty or clean directories (such as newly cloned empty repositories), `bws init` displays an interactive, categorized catalog of available Curated Seed Stacks and User Saved Stacks.

---

## Saving custom user stacks

To export an active workspace configuration into a reusable persona stack:

```bash
bws stack save my-custom-agent -t "Custom Agent Persona" -d "Hardened agent with custom tooling"
```

The command enforces the genesis invariant:
1. Validates that current directory is an active workspace containing `.bws/config.jsonc`.
2. Resolves effective profiles and verifies that each profile passes its smoke tests (bypass with `--no-verify`).
3. Strips host-specific secrets, credential environment variables, and absolute paths.
4. Generates cryptographic SHA-256 digest and records provenance metadata (source workspace, timestamp).
5. Writes definition atomically to `~/.config/bws/stacks/<name>.json`.

---

## Updating workspace stacks

When an upstream stack definition changes:

```bash
# Preview changes and permission diffs without applying
bws stack update --dry-run

# Review and apply changes with atomic .bak backup
bws stack update
```

The update operation:
1. Compares the workspace's pinned `reviewed_stack` digest with the upstream stack.
2. Displays changed profiles, features, or environment variables.
3. Prompts for explicit review (or accepts `-y` in scripted pipelines).
4. Creates a `.bak` backup of `.bws/config.jsonc`.
5. Updates the workspace configuration atomically with the new approved digest.

---

## Command reference

### `bws stack list`
List registered seed stacks and user-saved stacks.

* `-c, --category <cat>`: Filter by category (e.g. `runtime`, `agents`).
* `--json`: Print output as JSON array.

### `bws stack show <name>`
Inspect full details, constituent profiles, features, environment variables, digest, and provenance for a stack.

### `bws stack save <name>`
Export the current active workspace as a reusable user stack.

* `-t, --title <text>`: Custom human-readable title.
* `-d, --desc <text>`: Persona description.
* `-f, --force`: Overwrite existing stack with the same name.
* `--no-verify`: Skip running smoke tests for constituent profiles.

### `bws stack update`
Pull upstream stack updates into the current workspace.

* `-n, --dry-run`: Preview updates without modifying workspace files.
* `-y, --yes`: Apply updates without interactive confirmation.
