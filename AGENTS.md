# AI agent guidelines for `bws` (Bubblewrap sandbox launcher)
## Quick commands
```bash
make                          # build + test + lint (via Makefile)
./tools/verify_build.sh       # go vet + go test + go build in one
go vet ./...                  # static analysis
go test ./...                 # run all tests
./tools/test_long             # run all long tests (opt-in)
./tools/audit_lines.rb        # audit function (80 max) & file limits (800 warn / 1100 max)
./tools/bump_version.sh       # increment patch, commit, push
./tools/bump_version.sh 0.2.0 # set explicit version, commit, push
./tools/snapshot.sh           # commit all with message & push
./tools/install               # build and install to ~/bin/bws
```

## Project structure
```
bws/
├── main.go                  # Entry point: clihelp App/Command tree, dispatch
├── go.mod / go.sum
├── Makefile                 # Build, test, lint targets
├── README.md                # Main documentation
├── profiles/                # Capability & Security Profiles catalog
│   ├── README.md            # Detailed profiles catalog documentation
│   └── *.json               # Profile definitions (embedded in binary)
├── tools/                   # Developer automation scripts
│   ├── verify_build.sh      # vet + test + build
│   ├── test_long            # run all long tests individually
│   ├── audit_lines.rb       # enforce 80-line func & 800/1100-line file limits
│   ├── bump_version.sh      # bump version, commit, push
│   ├── outline_symbols.sh   # sorted index of exported symbols
│   ├── show_symbol.sh       # show a symbol's declaration
│   ├── snapshot.sh          # commit all + push
│   └── install              # build and install to ~/bin/bws
└── internal/
    ├── config/              # JSONC loading, @@HOME@@ token, global+local merge
    ├── profile/             # Profile engine, dependency resolution, test runner
    ├── sandbox/             # Sandbox staging, skeletons, dotfiles, dynamic config
    ├── bwrap/               # Building bwrap argument list, path masking, mounts
    ├── ssh/                 # SSH agent lifecycle, auto deploy keys via gh
    ├── cli/                 # Subcommand handlers (init-dev, profile, conf, cbind)
    └── util/                # File counting, command detection, path helpers
```

## Conventions
- **Use `internal/`** for all packages — this is a single-binary CLI, not a library.
- **One package per directory**, named after the directory.
- **`main.go` is thin** — define the clihelp App/Command tree, delegate to packages. No business logic.
- **CLI framework** — use `github.com/sarielhp/clihelp` for commands, options, help rendering, and validation.
- **JSONC support** — use the built-in JSONC loader in `internal/config/`. No additional dependencies.

## Configuration merging
- **Global + local config**: load both, deep-merge hashes, replace arrays. Implemented in `internal/config/merge.go` with unit tests.
- **`@@HOME@@` token**: replace at load time.

## Error handling
- **Return errors, don't panic**. Use `fmt.Errorf` with `%w` for wrapping.
- **Main exits with `os.Exit(1)`** on fatal errors, printing to `os.Stderr`.
- **No `log.Fatal`** in packages — only in `main.go`.
- **clihelp handlers** return errors; the framework prints them with `clihelp.PrintError`.

## Before committing
1. Run `go vet ./...` — no warnings.
2. Run `./tools/verify_build.sh` — all tests pass, binary compiles.
3. Run `./tools/audit_lines.rb` — no function exceeds 80 lines and files remain within 300–700 lines (warn > 800, max 1100).
4. Run `./tools/bump_version.sh` — every code change bumps the version (and auto-commits/pushes to git).
5. Commit messages follow conventional style: `area: description` or `Type(scope): description`.
   Examples: `feat(safety): block root directory`, `fix(bwrap): correct X11 socket order`, `chore: bump version to 0.1.1`.

## Testing
- **`*_test.go` alongside every package**.
- **Table-driven tests** for config merging, bwrap arg building.
- **Integration tests** in `main_test.go` exercise the built binary for safety checks and --info output.
- **Run `go vet ./...` and `go test ./...`** before pushing.

## Hard constraints
- **Function limit**: Hard limit of **80 lines** per function. Decompose long functions in place.
- **File sizing**: Recommended **300–700 lines**, warning over **800 lines**, hard limit **1100 lines**. Never split a file through a function body.
- **No `log.Fatalf` in handlers** — only in `main.go` for startup-only errors.
- **No `ioutil`** (deprecated since Go 1.16) — use `os` and `io` directly.
- **No external dependencies for trivial things**.
- **Bump version** on every code change via `./tools/bump_version.sh` (auto-commits and pushes).
- **Config path is fixed** — `~/.config/bws/config.jsonc`. Local override: `.bws/config.jsonc` in the current directory.

## Build & distribution
- **`Makefile` with targets**: `build`, `test`, `lint`, `clean`.
- **`go build -o bws .`** produces a single static binary.
- **Version** is in `main.go` as `var Version = "X.Y.Z"`. Updated via `tools/bump_version.sh`.

## Code style
- **`gofumpt`** for formatting (fallback to `go fmt`).
- **No naked returns** except in trivial getters.
- **Explicit error checks** — no ignoring errors.
- **Comments on exported symbols** only.
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

## Scripts catalog
| Script | Purpose |
|---|---|
| `verify_build.sh` | `go fmt` → `go vet` → `go test` → `go build` |
| `audit_lines.rb` | audit function lengths (80 max) and file lengths (800 warn / 1100 max) |
| `bump_version.sh [v]` | increment patch (or set explicit version), commit, push |
| `outline_symbols.sh` | sorted index of all Go types, functions, constants, vars |
| `show_symbol.sh <sym>` | display declaration lines for a named symbol |
| `snapshot.sh [msg]` | `git add -A && git commit -m "<msg>" && git push` |