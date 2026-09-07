# Architectural Review: Transitioning `bws` to Workflow-Driven Sandbox Learning

We are evaluating an architectural pivot for `bws` (Bubblewrap Sandbox Launcher).
Review this design critically as a senior Linux systems engineer, security architect, and CLI tool designer.

---

## The Core Problem

Currently, `bws` ships with over 30 pre-baked capability profiles (`go-dev`, `python-dev`, `rust-dev`, `latex-dev`, `copilot`, `quarto`, `ai`, etc.) embedded as JSON files. This creates three severe issues:
1. **Maintenance & Distro Divergence**: File paths differ across Debian, Arch, Fedora, NixOS, and custom toolchains. Speculative paths bloat the profiles.
2. **Security Leakage**: Pre-baked profiles violate the principle of least privilege. For example, `go-dev` inadvertently depended on `gh`, which bound `~/.config/gh` and leaked host GitHub account tokens into what was supposed to be a repository-isolated sandbox.
3. **Code Complexity**: Maintaining profile DAG resolution, dependency cycles, precedence sorting, and runner smoke tests adds hundreds of lines of non-core logic.

---

## The Proposed Pivot: Workflow-Driven Learning

Instead of shipping pre-baked language profiles:
1. **Keep only Hardening Profiles (Negative Masks)**: Invariant security boundaries (`no-sudo`, `no-ssh`, `no-gh`, `no-secrets`, `no-history`, `offline`).
2. **Eliminate Pre-baked Capability Profiles**: No built-in `go-dev`, `python-dev`, `rust-dev`, etc.
3. **Workflow / Gate Script Learning**: The developer maintains a project workflow script (e.g. `tools/gate.sh` or `tools/env.sh` containing `go test ./...`, `cargo test`, linters, etc.). Running `bws learn ./tools/gate.sh` traces the script and synthesizes the exact minimal mounts into `.bws/config.jsonc`.
4. **User-Owned Profiles**: Discovered environments can optionally be saved to `~/.config/bws/profiles/<name>.json` via `bws profile save <name>`.

---

## Three Critical Challenges Under Review

### Challenge 1: Coverage Guarantees vs. Gate Script Narrowness
* Pre-baked profiles offered a broad blanket guarantee: all compiler tools, headers, and LSPs were present.
* A narrow gate script (`go test ./...`) might miss tools needed during interactive sessions or edge cases:
  - Interactive tools: Language servers (`gopls`, `rust-analyzer`), formatters, debuggers (`dlv`).
  - Runtime edge-case assets: Timezone files (`/usr/share/zoneinfo`), MIME databases, SSL certs.
  - Dynamically added dependencies (`go get`, `cargo add`).
* *Proposed mitigation*: Use an explicit "Environment Harness" (`tools/env.sh`) that exercises toolchains, combined with semantic path collapsing to toolchain roots rather than single files.

### Challenge 2: Incremental Delta Learning (Learning ONLY What Is New)
* A project already has a working `.bws/config.jsonc` with 15 mounts.
* When updating the workflow script (e.g. adding SQLite or Quarto), `bws learn` must NOT re-trace everything from scratch or wipe custom mounts.
* *Proposed mitigation*: Mount-tree filtering where accesses satisfied by existing mounts are discarded, and only `Delta = Discovered - ExistingMounts` is presented to the user for merging.

### Challenge 3: Tracing Performance & Overhead (`strace` is SLOW)
* Naive `strace -f` on heavy builds (`cargo build`, `go build`, `npm install`) introduces severe ptrace context-switch overhead (10x-50x slowdown).
* *Proposed mitigations*:
  1. **Static toolchain queries (Instant, ~10ms)**: Fast-path using tools' own query commands (`go env GOROOT GOPATH GOCACHE`, `rustc --print sysroot`, `python3 -c "import sys; print(sys.path)"`).
  2. **Filtered syscall tracing**: Restrict `strace` to `-e trace=%file,execve` and filter out high-volume I/O syscalls (`read`, `write`, `mmap`, `close`).
  3. **Probe-first execution**: Run the workflow inside the sandbox at 100% native speed. If it succeeds, 0 overhead. If it fails, trace only what was missing.
  4. **LD_PRELOAD interceptor**: A lightweight shared library tracing `openat`/`execve` at near-native speed without ptrace.

---

## Review Questions

1. **Viability & Flaws**: Is moving completely away from pre-baked capability profiles to workflow-driven learning sound? What are the biggest failure modes in real-world developer workflows?
2. **Solving Coverage**: How can `bws` guarantee an AI agent or developer won't hit missing toolchain roadblocks without falling back to bloated pre-baked profiles?
3. **Solving Incremental Learning**: What is the most robust way to calculate and merge the delta between an existing sandbox config and an updated workflow script?
4. **Solving Trace Performance**: Which performance approach (static queries, filtered strace, probe-first, LD_PRELOAD) offers the best balance of speed, simplicity, and unprivileged safety?
5. **Concrete Recommendations**: What architectural steps would you prioritize?
