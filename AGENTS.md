# AI Agent Guidelines for `bw` (Bubblewrap Sandbox Launcher)

## Quick Commands

```bash
make                          # build + test + lint (via Makefile)
./tools/verify_build.sh       # go vet + go test + go build in one
go vet ./...                  # static analysis
go test ./...                 # run all tests
./tools/test_long             # run all long tests (opt-in)
./tools/audit_lines.sh        # check 300/500-line limits per file
./tools/bump_version.sh       # increment patch, commit, push
./tools/bump_version.sh 0.2.0 # set explicit version, commit, push
./tools/snapshot.sh            # commit all with message & push
./tools/install               # build and install to ~/bin/bw
```

## Project Structure

```
bw/
├── main.go                  # Entry point: clihelp App/Command tree, dispatch
├── go.mod / go.sum
├── Makefile                 # Build, test, lint targets
├── tools/                 # Developer automation scripts
│   ├── verify_build.sh      # vet + test + build
│   ├── test_long             # run all long tests individually
│   ├── audit_lines.sh        # enforce 300/500-line limits
│   ├── bump_version.sh       # bump version, commit, push
│   ├── outline_symbols.sh    # sorted index of exported symbols
│   ├── show_symbol.sh        # show a symbol's declaration
│   ├── snapshot.sh           # commit all + push
│   └── install               # build and install to ~/bin/bw
├── internal/
│   ├── config/              # JSONC loading, @@HOME@@ token, global+local merge
│   ├── sandbox/             # Sandbox preparation (copy files, shell profiles, tmux, oh-my-posh)
│   ├── bwrap/               # Building bwrap argument list from merged config
│   ├── ssh/                 # SSH agent lifecycle, auto deploy keys via gh
│   ├── cli/                 # Subcommand handlers (scp, copy), tool verification, info display
│   └── util/                # File counting, command detection, path helpers
└── old_script/              # Original Ruby implementation (reference only)
```

## Conventions

- **Use `internal/`** for all packages — this is a single-binary CLI, not a library.
- **One package per directory**, named after the directory.
- **`main.go` is thin** — define the clihelp App/Command tree, delegate to packages. No business logic.
- **CLI framework** — use `github.com/sarielhp/clihelp` for commands, options, help rendering, and validation.
- **JSONC support** — use the built-in JSONC loader in `internal/config/`. No additional dependencies.

## Configuration Merging

- **Global + local config**: load both, deep-merge hashes, replace arrays. Implemented in `internal/config/merge.go` with unit tests.
- **`@@HOME@@` token**: replace at load time.

## Error Handling

- **Return errors, don't panic**. Use `fmt.Errorf` with `%w` for wrapping.
- **Main exits with `os.Exit(1)`** on fatal errors, printing to `os.Stderr`.
- **No `log.Fatal`** in packages — only in `main.go`.
- **clihelp handlers** return errors; the framework prints them with `clihelp.PrintError`.

## Before Committing

1. Run `go vet ./...` — no warnings.
2. Run `./scripts/verify_build.sh` — all tests pass, binary compiles.
3. Run `./scripts/audit_lines.sh` — no file exceeds the 500-line hard limit.
4. Run `./scripts/bump_version.sh` — every code change bumps the version (and auto-commits/pushes to git).
5. Commit messages follow conventional style: `area: description` or `Type(scope): description`.
   Examples: `feat(safety): block root directory`, `fix(bwrap): correct X11 socket order`, `chore: bump version to 0.1.1`.

## Testing

- **`*_test.go` alongside every package**.
- **Table-driven tests** for config merging, bwrap arg building.
- **Integration tests** in `main_test.go` exercise the built binary for safety checks and --info output.
- **Run `go vet ./...` and `go test ./...`** before pushing.

## Hard Constraints

- **300-line soft limit**, **500-line hard limit** per `.go` file. Run `./scripts/audit_lines.sh` to verify.
- **No `log.Fatalf` in handlers** — only in `main.go` for startup-only errors.
- **No `ioutil`** (deprecated since Go 1.16) — use `os` and `io` directly.
- **No external dependencies for trivial things**.
- **Bump version** on every code change via `./scripts/bump_version.sh` (auto-commits and pushes).
- **Config path is fixed** — `~/.config/bwss/config.jsonc`. Local override: `.bws/config.jsonc` in the current directory.

## Build & Distribution

- **`Makefile` with targets**: `build`, `test`, `lint`, `clean`.
- **`go build -o bw .`** produces a single static binary.
- **Version** is in `main.go` as `var Version = "X.Y.Z"`. Updated via `scripts/bump_version.sh`.

## Code Style

- **`gofumpt`** for formatting (fallback to `go fmt`).
- **No naked returns** except in trivial getters.
- **Explicit error checks** — no ignoring errors.
- **Comments on exported symbols** only.
- **Avoid `init()`** — use explicit initialization in `main()`.

## Key Behaviors to Preserve

1. **Safety checks**: block running from `/`, `~/`, or `~/bin/`, file count limit.
2. **SSH agent**: auto-start, reuse socket, deploy keys via `gh`.
3. **X11**: mount socket after `/tmp` bind (order matters).
4. **WSL detection**: bind `/run/WSL`, propagate `WSL_INTEROP`.
5. **oh-my-posh**: inject into `.bashrc`/`.zshrc` idempotently.
6. **`--info`**: dry run — no side effects, just print the plan.
7. **`test <target>`**: run tool, verify exit code, report.

## What NOT to Do

- Don't shell out to `bwrap` via `os/exec` in packages — build the arg list and let `main.go` call `exec.Command`.
- Don't embed the default config as a Go string literal — use `//go:embed`.
- Don't duplicate configuration keys — merge logic lives in `internal/config/merge.go` with tests.

## Scripts Catalog

| Script | Purpose |
|---|---|
| `verify_build.sh` | `go fmt` → `go vet` → `go test` → `go build` |
| `audit_lines.sh` | flag `.go` files exceeding 300 lines, error on >500 |
| `bump_version.sh [v]` | increment patch (or set explicit version), commit, push |
| `outline_symbols.sh` | sorted index of all Go types, functions, constants, vars |
| `show_symbol.sh <sym>` | display declaration lines for a named symbol |
| `snapshot.sh [msg]` | `git add -A && git commit -m "<msg>" && git push` |