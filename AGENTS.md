# AI agent guidelines for `bws` (Bubblewrap sandbox launcher)
## Quick commands
```bash
rtk proxy tools/build          # build bin/bws
rtk proxy tools/gate           # fast gate: gofmt, mod tidy, vet, staticcheck, go-audit, temp-file check, tests, docs drift, build
rtk proxy tools/gate --full    # adds race, govulncheck, gocritic, shadow, revive, dupl
rtk proxy tools/lint           # static checks only
./tools/test_long              # long tests (opt-in, build tag `long`)
./tools/check_docs_drift.rb --strict --binary bin/bws   # docs/commands.md vs the CLI
rtk proxy tools/install        # requested installation to ~/bin/bws (plus agent integrations)
```

## Project structure
```
bws/
├── main.go                  # Entry point: clihelp App/Command tree, dispatch
├── go.mod / go.sum
├── Makefile                 # Thin wrapper over tools/ (build, gate, ci, install, release, long)
├── VERSION                  # Release version, embedded with //go:embed
├── README.md                # Main documentation
├── profiles/                # Capability & Security Profiles catalog
│   ├── README.md            # Detailed profiles catalog documentation
│   └── *.json               # Profile definitions (embedded in binary)
├── tools/                   # Developer automation scripts
│   ├── settings.rb          # Project settings: artifacts, checks, version, install targets
│   ├── tooling.rb           # Shared gate/build/release implementation (standards template)
│   ├── tmp_space.rb         # <tmpdir>/<user>/bws/ temporary tree for Ruby tools
│   ├── build, gate, check, lint, snapshot, commit, bump, release, install
│   ├── integrations.rb      # Antigravity skill and OpenCode commands, copied by install
│   ├── check_docs_drift.rb  # verify docs/commands.md against `bws inventory`
│   ├── test_long            # run all long tests individually
│   ├── go_tools_setup_fix   # link an installed Go into ~/.local/bin, set GOPATH=~/.go
│   ├── outline_symbols.sh   # sorted index of exported symbols
│   ├── show_symbol.sh       # show a symbol's declaration
│   └── review_cycle.json    # review-cycle configuration
└── internal/
    ├── config/              # JSONC loading, @@HOME@@ token, global+local merge
    ├── profile/             # Profile engine, dependency resolution, test runner
    ├── sandbox/             # Sandbox staging, skeletons, dotfiles, dynamic config
    ├── bwrap/               # Building bwrap argument list, path masking, mounts
    ├── ssh/                 # SSH agent lifecycle, auto deploy keys via gh
    ├── cli/                 # Subcommand handlers (init-dev, profile, conf, cbind)
    ├── util/                # File counting, command detection, path helpers, temp tree
    └── testutil/            # Helpers shared by test suites
```

## Conventions
- **Use `internal/`** for all packages — this is a single-binary CLI, not a library.
- **One package per directory**, named after the directory.
- **`main.go` is thin** — define the clihelp App/Command tree, delegate to packages. No business logic.
- **CLI framework** — use `github.com/sarielhp/clihelp` for commands, options, help rendering, and validation. The project tracks the latest release (currently v0.3.53) and uses its full surface: two help tiers (`-h` concise, `--help`/`-H` extended), examples (`-E`), grouped flags (`clihelp.Group`), `LongDescription` and `Parameters`, dynamic completion (`Option.Complete`, `Param.Complete`), flag groups and validators (`MutuallyExclusive`, `ValidateOptions`), `ManPageCommand`, `GenManPage`, and the `doc`/`clihelptest` subpackages.
- **JSONC support** — use the built-in JSONC loader in `internal/config/`. No additional dependencies.

## Configuration merging
- **Global + local config**: load both, deep-merge hashes, union arrays (deduplicated, global entries first). A local config can add list entries but never remove global ones; this is what keeps global `mask` entries in force. Binds are merged per list and keyed by host path, so a local entry replaces a global entry with the same host in the same list. `config.RestrictRW` drops local and profile read-write binds on a globally read-only path or under a system directory; subpaths of a read-only directory stay allowed. Implemented in `internal/config/merge.go` with unit tests.
- **`@@HOME@@` token**: replace at load time.

## Error handling
- **Return errors, don't panic**. Use `fmt.Errorf` with `%w` for wrapping.
- **Main exits with `os.Exit(1)`** on fatal errors, printing to `os.Stderr`.
- **No `log.Fatal`** in packages — only in `main.go`.
- **clihelp handlers** return errors; the framework prints them with `clihelp.PrintError`.

## Before committing
1. Run `rtk proxy tools/gate` (use `--full` before a release); it must pass without baseline changes.
2. Commit only when asked, through `rtk proxy tools/commit <message>`; release through `rtk proxy tools/release <message>`, which bumps `VERSION`, pushes and installs.
3. Commit messages follow conventional style: `area: description` or `Type(scope): description`.
   Examples: `feat(safety): block root directory`, `fix(bwrap): correct X11 socket order`.

## Testing
- **`*_test.go` alongside every package**.
- **Table-driven tests** for config merging, bwrap arg building.
- **Integration tests** in `main_test.go` exercise the built binary for safety checks and --info output.
- **Unix sockets in tests** go in `testutil.ShortSocketDir(t)`: `t.TempDir()` under the gate's TMPDIR exceeds the 107-byte socket path limit.

## Hard constraints
- **Go standards**: follow `~/prog/standards/go/GUIDELINES.md`. Nesting depth ≤ 4, decision branches ≤ 15 per function.
- **Function limits by tier** (enforced by `go-audit --strict`): standard logic 110 lines, declarative builders 160, dispatchers 200, table-driven tests 250. Decompose long functions in place.
- **File sizing**: hard limit **1100 lines** (1600 for `*_test.go`). Never split a file through a function body.
- **Temporary files** live under `<tmpdir>/<user>/bws/<purpose>/` via `util.TempPath`, `util.UserTempDir` or `util.UserTempFile`. Never call `os.TempDir()` elsewhere or write a literal `/tmp` path without `tmp-exempt: <reason>`.
- **No `log.Fatalf` in handlers** — only in `main.go` for startup-only errors.
- **No `ioutil`** (deprecated since Go 1.16) — use `os` and `io` directly.
- **No external dependencies for trivial things**.
- **Config path is fixed** — `~/.config/bws/config.jsonc`. Local override: `.bws/config.jsonc` in the current directory.

## Build & distribution
- **`rtk proxy tools/build`** produces `bin/bws`; `tools/install` copies it to `~/bin/bws`.
- **Version** lives in `VERSION`, embedded into `main.Version`, and is bumped by `tools/bump` or `tools/release`.
- **Toolchain** is pinned by the `toolchain` line in `go.mod`.

## Code style
- **`gofmt -s`** formatting (checked by the gate).
- **No naked returns** except in trivial getters.
- **Explicit error checks** — no ignoring errors.
- **Doc comments** on every package and exported symbol (revive checks them).
- **Avoid `init()`** — use explicit initialization in `main()`.

## Documentation & writing standards
- **Sentence-case headings**: Always use clean sentence case for markdown headings (e.g. `## Key capabilities`, `## Quick start`). No title case or shouting.
- **Zero superlatives or fluff**: Never use hyperbolic marketing language (*"gold-standard"*, *"battle-tested"*, *"seamlessly"*, *"best of both worlds"*, *"instantly"*, *"zero-trust"*). Describe mechanisms, boundaries, and trade-offs factually.
- **Modular documentation**: Keep `README.md` lean (<200 lines) with a concise overview, prerequisites, 5-step Quick Start, and a Table of Contents delegating deep dives to `docs/` (`faq.md`, `configuration.md`, `security.md`, `commands.md`, `architecture.md`).
- **Audits & review artifacts**: Store reusable evaluation prompts in `prompts/` and generated review reports in `reviews/`.

## Key behaviors to preserve
1. **Safety checks**: block running from `/`, `~/`, or `~/bin/`, file count limit.
2. **SSH agent**: auto-start, reuse socket, deploy keys via `gh`.
3. **X11**: mount socket after `/tmp` bind (order matters).
4. **WSL detection**: bind `/run/WSL`, propagate `WSL_INTEROP`.
5. **oh-my-posh**: inject into `.bashrc`/`.zshrc` idempotently.
6. **`--info`**: dry run — no side effects, just print the plan.
7. **`test <target>`**: run tool, verify exit code, report.

## What not to do
- Don't shell out to `bwrap` via `os/exec` in packages — build the arg list and let `main.go` call `exec.Command`.
- Don't embed the default config as a Go string literal — use `//go:embed`.
- Don't duplicate configuration keys — merge logic lives in `internal/config/merge.go` with tests.

## Documentation drift check

`docs/commands.md` is hand-written and drifts whenever the CLI changes. The build
prevents that: the hidden `bws inventory` command emits a JSON tree of the whole
command surface (names, aliases, per-command flags, globals), and
`tools/check_docs_drift.rb` diffs it against the `` `bws ...` `` headings and
option tables in `docs/commands.md`. It fails on a documented command or flag
that does not exist, and (in `--strict`, used by the gate) on a real
command with no heading. Hidden commands are excluded from the inventory.

To add a command: give it a `` ### `bws <name>` `` heading with an option table,
then run `rtk proxy tools/gate`, which runs `./tools/check_docs_drift.rb --strict --binary bin/bws`.

## Go tooling and authorization

Follow the Go [GUIDELINES](~/prog/standards/go/GUIDELINES.md) and
[RATIONALE](~/prog/standards/go/RATIONALE.md). Read `tools/settings.rb` for this
project's artifact layout, checks, version mode, and declared installation targets.

Use the exact command forms when applicable:

| Command | Use |
|---|---|
| `rtk proxy tools/build` | Repository-local build |
| `rtk proxy tools/gate` or `rtk proxy tools/check` | Fast gate; add `--full` for release verification |
| `rtk proxy tools/lint` | Static feedback |
| `rtk proxy tools/snapshot [message]` | Local checkpoint without gate/push |
| `rtk proxy tools/commit [--full] <message>` | Requested gated local commit |
| `rtk proxy tools/bump` | Requested version bump and verified push |
| `rtk proxy tools/bump --resume` | Resume an inspected failed push without bumping again |
| `rtk proxy tools/install` | Requested installation to declared targets |
| `rtk proxy tools/release [--no-install] <message>` | Requested release; installs to the declared targets via tools/install unless `--no-install` |

Run routine build, lint, test, and gate checks for authorized work. Existing
command approval allows execution; it does not authorize unrelated commits,
releases, installations, or changes outside the task. Reuse authorization already
given in the session. Keep sandboxing and manual approval review enabled; these
scripts do not change approval settings or writable roots. Bootstrap analysis
tools separately when authorized. Never enlarge a baseline to hide a new finding.

Create every temporary file under `<tmpdir>/<user>/<project>/` (standards README,
"Temporary and cached files"): through `ProjectTmp` (`tools/tmp_space.rb`) in Ruby, or the
language's call with an explicit base under that tree. Never use the system
temporary directory directly or name a literal `/tmp` path; the gate rejects it
unless the line says `tmp-exempt: <reason>`. Files reused across runs are caches,
not temporary files: put them under `$XDG_CACHE_HOME/<project>/`.

## Autonomous review cycle (`review-cycle`)

The engine and review-audit live in ~/prog/26/ruby/review_cycle and run from PATH. The repository contract is ~/prog/standards/workflow/README.md; project choices belong in tools/review_cycle.json. Read project/language guidelines, agent instructions, and tooling settings before remediation.

| Command | Effect |
|---|---|
| `rtk proxy review-cycle --doctor` | Check host and configured sandbox prerequisites. |
| `rtk proxy review-cycle --setup` | Requested onboarding: configure local bws profiles/tools and project review files. |
| `rtk proxy review-cycle --no-push` | Requested audit/remediation with local commits, gate/build, and differential verification. |
| `rtk proxy review-cycle --loop --no-push` | Bounded loop across lenses; default maximum seven cycles. |
| `rtk proxy review-cycle --push` | Authorized upstream publication with remote SHA verification. |
| `rtk proxy review-cycle --status` | Inspect rotation and history. |
| `rtk proxy review-cycle --dry-run --no-push` | Audit/report without remediation or commits; writes reports/logs and runs pre-flight checks. |

Declare executable tools/gate and tools/build. Adopt all language helpers, entry points, and settings. The canonical gate owns analyzers, tests, and one build; tools/check delegates to it. Build validates repository artifacts and never installs or changes Git. Compiler/dependency caches may be external. Custom gates need a checked separate build unless gate_includes_build:true declares they include it.

The pipeline audits a lens, dispatches sandbox remediation, inspects its diff, runs the post-merge gate/build and differential audit, then writes summary/archive/state artifacts. Invalid configuration, missing commands, failed verification, and failed Git operations stop the run. Remediation or verification failures roll back to the cycle base with pre-existing untracked-file preservation. Ignored output/cache changes and external effects remain outside rollback.

Every differential finding blocks completion. --skip-diff-audit records local needs_review, exits nonzero, and prevents publication. Engine evidence overrides agent-authored verification claims in summaries. Preserve regression coverage and add appropriate defect tests. Missing test edits remain advisory; --strict-tests rejects detected test weakening and --strict-diff rejects oversized diffs. Neither proves complete coverage.

Commits remain local by default; inspect existing explicit push:true settings and use --no-push when needed. Authorized publication requires a remote branch upstream and one push URL, pushes the exact intended commit, and verifies its destination SHA. Publication failures exit nonzero and retain verified local commits; inspect the remote before retrying. Batched pushes follow the same policy.

Review commits do not contain lifecycle structured verification or create release tags. Run the full project gate before authorized publication or use tools/bump/tools/release. Install only through the explicitly authorized project installation workflow.

Command approval permits execution, not unrelated work. Reuse session authorization. Keep sandboxing and manual approval review enabled; never enlarge writable roots, modify approval rules, install dependencies, or publish merely because these instructions are present.
