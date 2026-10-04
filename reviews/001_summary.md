# Systems Audit Remediation Summary (Report #001)

- **Review Reference**: `reviews/001_systems.md`
- **Plan Reference**: `reviews/001_systems_plan.md`
- **Lens**: `systems`
- **Date**: 2026-10-04
- **Status**: Completed & Verified Cleanly
- **Footprint**: 17 files changed, 600 insertions(+), 62 deletions(-)
- **Differential Audit**: Clean (0 defects in diff)

---

## Triage & Remediation Overview

All 7 findings identified in `reviews/001_systems.md` were evaluated. Following triage, all 7 were confirmed as genuine defects. Targeted, minimal, idiomatic fixes were implemented with dedicated regression unit tests for every issue.

| Finding # | Location | Severity | Triage Status | Mitigation Summary | Verification |
|---|---|---|---|---|---|
| **1** | `internal/cli/tui_select.go` | Major | Genuine Defect | Added `startSelectorSignalMonitor` to concurrently monitor `sigCh` during interactive selector. On SIGINT/SIGTERM, terminal is restored via `term.Restore` before `os.Exit(130)`. Goroutine cleanly cancels on `done`. | `internal/cli/tui_select_test.go:TestStartSelectorSignalMonitor` validates signal trigger callback and clean `done` cancellation. |
| **2** | `internal/profile/runner.go` | Major | Genuine Defect | Replaced shallow `copyConfig` with `config.Clone(cfg)` in `profileTestConfig`, eliminating caller slice buffer pollution and `Features.NoNet` mutation. Deleted dead `copyConfig`. | `internal/profile/profile_test.go:TestProfileTestConfigIsolation` verifies caller's `PassEnv`, `Mask`, and `Features` remain unmutated. |
| **3** | `internal/proxy/proxy.go` | Major | Genuine Defect | Configured `DisableKeepAlives: true` and deferred `transport.CloseIdleConnections()` on ephemeral `http.Transport` in `handleHTTP`. | `internal/proxy/proxy_test.go:TestProxyHTTPSequentialRequestsNoLeak` executes 15 sequential requests verifying clean connection teardown and payload integrity. |
| **4** | `internal/config/trust.go` | Major | Genuine Defect | Refactored `trustContents` to use `atomicWriteFile(record, ...)` with temporary file creation, fsync, and atomic rename, followed by mode 0600 chmod. | `internal/config/security_test.go:TestTrustContentsAtomic` validates atomic write, 0600 permissions, and round-trip verification with `ReadTrustedFile`. |
| **5** | `internal/util/tmux.go` | Moderate | Genuine Defect | Changed format string delimiter to tab (`\t`) in `tmux display-message` and extracted `parseTmuxOutput` and `formatPrefixedTitle`. Window titles containing colons are now parsed without field misalignment. | `internal/util/tmux_test.go:TestParseTmuxOutputWithColons` tests colon-heavy titles (`dev:server.go:8080`), complex pane titles, and edge cases. |
| **6** | `internal/stack/registry.go` | Moderate | Genuine Defect | Refactored `SaveUserStack` to use `os.CreateTemp`, followed by explicit `f.Write`, `f.Sync()`, `f.Close()`, and atomic `os.Rename`. | `internal/stack/stack_test.go:TestSaveUserStackAtomicClean` verifies file creation, fsync integrity, and zero leftover temporary files. |
| **7** | `runner.go` | Moderate | Genuine Defect | Replaced naked pointer with `sync/atomic.Pointer[dbus.Proxy]` for race-free access across main and signal goroutines. Extracted `startSandboxSignalMonitor` and `appendDBusArgs`, managing signal registration and goroutine lifecycle with `sync.Once` and `done` channel. | `runner_lifecycle_test.go:TestSandboxSignalLifecycleCleanExit` and `TestSandboxDBusAtomicConcurrency` run under `go test -race`. |

---

## Architectural & Invariant Compliance

1. **Function & File Length Limits**:
   - Every modified and new function satisfies the hard limit of <= 80 lines (`setupSandboxHome`: 27 lines, `startSandboxSignalMonitor`: 39 lines, `appendDBusArgs`: 11 lines, `buildAndRun`: 63 lines, `SaveUserStack`: 35 lines, `SetHostTmuxTitle`: 33 lines, `parseTmuxOutput`: 13 lines, `profileTestConfig`: 30 lines).
   - Confirmed with `./tools/audit_lines.rb`: *All audited files and modified functions within limits.*

2. **Cognitive Complexity & Nesting**:
   - Decomposed in-place helpers: `startSelectorSignalMonitor`, `parseTmuxOutput`, `formatPrefixedTitle`, `startSandboxSignalMonitor`, and `appendDBusArgs`.
   - Maximum nesting depth across all modified functions is <= 3, cognitive complexity <= 6.

3. **Concurrency & Race Detection**:
   - `go test -race ./...` executed across all 15 packages in the repository with zero data races detected.

4. **Quality Gate Verification**:
   - Executed `tools/gate` (`./tools/verify_build.sh` + `./tools/audit_lines.rb`).
   - `go fmt`: clean.
   - `go vet ./...`: passed without warnings.
   - `go build -o bws .`: compiles cleanly.
   - `go test ./... -count=1`: all unit and integration tests pass.
   - `./tools/check_docs_drift.rb --strict`: documentation perfectly matches CLI surface.
