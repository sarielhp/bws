# Systems Audit Remediation Plan (Report #001)

## Executive Summary
This remediation plan addresses all 7 findings identified in `reviews/001_systems.md` (Lens: systems). Following triage, all 7 findings are confirmed as genuine systems-level defects spanning signal handling, configuration memory aliasing, HTTP connection/goroutine leakage, atomic state persistence, terminal format parsing, and concurrency safety.

## Triage Assessment

| Finding | Severity | Component | Finding Type | Triage Verdict | Rationale |
|---|---|---|---|---|---|
| 1. Signal Swallowing in TUI Selector | Major | `internal/cli/tui_select.go` | Signal Handling & Terminal Lifecycle | **Genuine Defect** | `sigCh` is registered with `signal.Notify` but never monitored, causing external `SIGTERM` to hang while blocking on stdin, leaving terminal in raw mode upon `SIGKILL`. |
| 2. Config Mutation via Shallow Copy | Major | `internal/profile/runner.go` | Slice & Pointer Aliasing | **Genuine Defect** | `copyConfig` performs shallow struct copying; slice appends and `Features.NoNet` pointer mutations alter caller's configuration permanently. |
| 3. HTTP Transport Leak in Forward Proxy | Major | `internal/proxy/proxy.go` | Resource & Goroutine Leak | **Genuine Defect** | `handleHTTP` creates an ephemeral `http.Transport` per request with default keep-alives and without `CloseIdleConnections`, leaking idle sockets and goroutines. |
| 4. Non-Atomic Trust State File Write | Major | `internal/config/trust.go` | Crash Resilience | **Genuine Defect** | `trustContents` uses `os.WriteFile` without tempfile+rename and fsync, risking corrupted/empty trust records on process crash or interrupt. |
| 5. Parsing Fragility in Tmux Window Title | Moderate | `internal/util/tmux.go` | String Parsing / Collision | **Genuine Defect** | Colon delimiter `:` collides with window names containing colons (e.g. `vim:main.go`, `server:8080`), corrupting title restoration on exit. |
| 6. Insecure Temp File & Missing Sync in Stack Save | Moderate | `internal/stack/registry.go` | Filesystem Race & Crash Resilience | **Genuine Defect** | Deterministic PID-based tmp filename risks collision, and missing `f.Sync()` before rename risks 0-byte or corrupted stack files across crashes. |
| 7. Data Race on `dbusProxy` & Leaked Signal Routine | Moderate | `runner.go` | Concurrency & Goroutine Leak | **Genuine Defect** | Signal handler reads `dbusProxy` without synchronization while `buildAndRun` writes it. Signal handler is never stopped on normal exit, leaking goroutine. |

---

## Detailed Remediation Strategies & Architecture

### 1. Signal Lifecycle Management in TUI Selector (`internal/cli/tui_select.go`)
- **Root Cause**: `signal.Notify(sigCh, ...)` suppresses default termination, but no goroutine reads `sigCh` while `runSelectorLoop` is blocked on `os.Stdin.Read`.
- **Targeted Fix**:
  - Implement `startSelectorSignalMonitor(sigCh <-chan os.Signal, done <-chan struct{}, onSignal func())` to concurrently wait for either an incoming termination signal or selector completion via `done`.
  - On signal, restore terminal state (`term.Restore(stdinFd, oldState)`) and terminate cleanly via `os.Exit(130)`.
  - When selection loop finishes normally or via escape/cancel key, `close(done)` terminates the background monitor goroutine cleanly.
- **Regression Testing**:
  - Unit test in `internal/cli/tui_select_test.go` exercising `startSelectorSignalMonitor` with both signal trigger path and clean `done` cancellation path.

### 2. Deep Cloning for Profile Test Configuration (`internal/profile/runner.go`)
- **Root Cause**: `copyConfig` does not clone `PassEnv`, `Mask`, `Copy`, or `Features`. Slices are appended to in-place and pointers are mutated.
- **Targeted Fix**:
  - Replace `copyConfig` with `config.Clone(cfg)` which deep-copies all slices, maps, and nested pointers.
  - Delete `copyConfig`.
- **Regression Testing**:
  - Unit test `TestProfileTestConfigIsolation` in `internal/profile/profile_test.go` verifying that running `profileTestConfig` with extra env, mask, and `UnshareNet: true` leaves caller's `Config` completely unchanged.

### 3. Ephemeral Transport Cleanup in Forward Proxy (`internal/proxy/proxy.go`)
- **Root Cause**: Each HTTP request instantiates a new `http.Transport` without closing idle connections or disabling keep-alives.
- **Targeted Fix**:
  - Configure `DisableKeepAlives: true` on the ephemeral transport in `handleHTTP`.
  - Add `defer transport.CloseIdleConnections()` to ensure idle sockets and background dialer goroutines are reclaimed immediately upon request completion.
- **Regression Testing**:
  - Unit test in `internal/proxy/proxy_test.go` verifying proxy handling multiple HTTP requests without leaking goroutines or idle connections.

### 4. Atomic Trust Record Persistence (`internal/config/trust.go`)
- **Root Cause**: Direct `os.WriteFile` truncates and writes in place without fsync or atomic rename.
- **Targeted Fix**:
  - Use `atomicWriteFile(record, ...)` in `trustContents`, which creates a synced tempfile, performs `f.Sync()`, and atomically renames over destination.
- **Regression Testing**:
  - Unit test `TestTrustContentsAtomic` in `internal/config/trust_test.go` verifying atomic trust writes, file permissions, and subsequent validation via `ReadTrustedFile`.

### 5. Robust Delimited Parsing for Host Tmux State (`internal/util/tmux.go`)
- **Root Cause**: Splitting on `:` fails when user window titles contain colons.
- **Targeted Fix**:
  - Switch `tmux display-message` format to use `\t` (tab) delimiter: `#{pane_id}\t#W\t#{automatic-rename}\t#{pane_title}`.
  - Extract parsing logic into `parseTmuxOutput(out string) *HostTmuxState`.
  - Split on `\t` to guarantee that colons, commas, or spaces in window titles are preserved without field misalignment.
- **Regression Testing**:
  - Unit tests in `internal/util/tmux_test.go` testing window titles with colons (e.g. `vim:main.go`, `server:8080:test`), spaces, and edge cases.

### 6. Atomic & Synced User Stack Persistence (`internal/stack/registry.go`)
- **Root Cause**: Non-random temp filename `target + .tmp-<pid>` and missing `f.Sync()` before `os.Rename`.
- **Targeted Fix**:
  - Replace deterministic PID file with `os.CreateTemp(dir, "."+s.Name+".json.tmp-*")`.
  - Explicitly call `f.Write`, `f.Sync()`, `f.Close()`, followed by `os.Rename(tmp, target)`.
  - Clean up temp file on failure with deferred `os.Remove(tmp)`.
- **Regression Testing**:
  - Unit test `TestSaveUserStackAtomic` in `internal/stack/registry_test.go` verifying stack saving, sync/rename integrity, and lack of leftover temp files.

### 7. Concurrency Safety for `dbusProxy` & Signal Goroutine Lifecycle (`runner.go`)
- **Root Cause**: Data race on `dbusProxy` between initialization on main thread and read in signal handler. Background goroutine is never stopped on normal exit.
- **Targeted Fix**:
  - Use `sync/atomic.Pointer[dbus.Proxy]` for `dbusProxy`.
  - Manage signal lifecycle with `done` channel and `signal.Stop(sigChan)`.
  - Wrap cleanup so exiting `buildAndRun` terminates the signal goroutine.
  - Keep `buildAndRun` and `setupSandboxHome` strictly under 80 lines and cognitive complexity <= 15.
- **Regression Testing**:
  - Unit test in `runner_test.go` verifying that `setupSandboxHome` starts signal monitoring and that calling its cleanup cleanly stops the goroutine without leakage.

---

## Quality Metrics & Verification Gate
1. Every modified or added function MUST remain <= 80 lines.
2. Cognitive complexity <= 15, nesting depth <= 4.
3. Run `tools/gate` (`./tools/verify_build.sh` + `./tools/audit_lines.rb`).
4. Run `go test -race ./...` to guarantee concurrency safety.
5. Save final summary to `reviews/001_summary.md`.
