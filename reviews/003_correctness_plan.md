# Correctness Audit Remediation Plan (Report #003)

- **Review Reference**: `reviews/003_correctness.md`
- **Focus Lens**: `correctness`
- **Date**: 2026-10-04
- **Auditor**: Claude / Codex (Tier 1 Standard) via `tools/audit`
- **Remediation Agent**: Gemini 3.8 Flash (Tier 0 Workhorse)
- **Status**: Plan Approved, In Progress

---

## Triage Assessment

| Finding | Severity | Component | Finding Type | Triage Verdict | Rationale |
|---|---|---|---|---|---|
| 1. Lossy Wholesale Re-serialization in Stack Update | Critical | `internal/cli/stack_cmds.go:358-372` | Data Loss & Config Corruption | **Genuine Defect** | Parsing `localCfg` into `config.Config` and re-serializing via `json.MarshalIndent` corrupts `binds_rw`/`binds_ro` entries (serializing as `{"Host":"...","Sandbox":""}` which fails future loads), replaces `@@HOME@@` with absolute paths, strips JSONC comments, and injects `null` values. |
| 2. Unsafe Profile Synthesis Overwrite in Update Loop | Critical | `internal/cli/profile_cmds_ext.go:212-238`, `internal/cli/profile_cmds.go:175-234` | Destructive Overwrite & State Corruption | **Genuine Defect** | `HandleProfileUpdate` calls `HandleProfileFetch`, which on any network error, 404, or non-catalog profile falls back to `HandleProfileNew(..., force=false)`. `HandleProfileNew` synthesizes a generic skeleton from Homebrew/Firejail/PATH and overwrites existing profiles without an existence check or `--force` gate, destroying custom user profiles. |
| 3. Backup Single-Slot Overwrite on No-Op Bind Removal | Moderate | `internal/cli/bind_cmds.go:183-193`, `internal/config/jsonc.go:179-207` | Backup Invalidation & Silent Undo Failure | **Genuine Defect** | When removing a bind mount, `runBindDel` checks `binds_rw` then `binds_ro`. Even when nothing is found in `binds_ro`, `RemoveBindElement` calls `EditJSONC`, which calls `WriteTrustedFile`, clobbering `config.jsonc.bak` with the post-removal state. Subsequent `bws undo` becomes a no-op. |
| 4. Bypassed Deferred Cleanups via `os.Exit` in Runner | Moderate | `runner.go:195-198`, `main.go:237-244` | Resource & Terminal State Leak | **Genuine Defect** | `buildAndRun` invokes `os.Exit(exitErr.ExitCode())` when a sandboxed command exits non-zero. Skipping stack unwinding bypasses deferred cleanup functions in callers—specifically `defer tmuxCleanup()` in `runDefault` and `runExec`—leaving host tmux window and pane titles permanently polluted with `[BTS] `. |

---

## Detailed Remediation Strategies & Architecture

### 1. In-Place HuJSON Patching for Stack Updates (`internal/cli/stack_cmds.go`)
- **Root Cause**:
  `HandleStackUpdate` deserializes workspace policy bytes into Go struct `config.Config` and serializes it back using `json.MarshalIndent`. Because `BindEntry` lacks a `MarshalJSON` method, entries turn into JSON objects rather than strings or `[host, sandbox]` tuples, corrupting config files. It also strips user comments and expands `@@HOME@@`.
- **Targeted Fix**:
  - Decompose `HandleStackUpdate` by introducing helper `patchReviewedStack(before []byte, approval *config.ProfileApproval) ([]byte, error)`:
    - Parse raw `before` bytes into `hujson.Value` AST.
    - Marshal only the `config.ProfileApproval` struct.
    - Apply RFC 6902 patch `[{"op":"add","path":"/reviewed_stack","value":...}]`.
    - Run `ast.Format()` and return `ast.Pack()`.
  - In `HandleStackUpdate`, call `patchReviewedStack(before, ...)` and write the result via `config.AtomicPolicyWrite(localPath, updatedJSON, before)`.
  - Preserves all comments, tokens (`@@HOME@@`), formatting, and bind representations.
- **Regression Testing**:
  - Unit test `TestPatchReviewedStackPreservesCommentsAndBinds` in `internal/cli/stack_cmds_test.go` verifying that JSONC comments, `binds_rw` string slices, and `@@HOME@@` tokens remain intact when updating `reviewed_stack`.

### 2. Strict Remote-Only Profile Updates and Overwrite Protection (`internal/cli/profile_cmds_ext.go`, `internal/cli/profile_cmds.go`)
- **Root Cause**:
  `HandleProfileUpdate` delegates directly to `HandleProfileFetch`, which has a fallback to `HandleProfileNew`. `HandleProfileNew` unconditionally calls `profile.SaveProfile` without verifying whether the target file already exists.
- **Targeted Fix**:
  - Implement `fetchRemoteProfile(name, targetDir string) (bool, error)` in `internal/cli/profile_cmds_ext.go`:
    - Queries `https://raw.githubusercontent.com/sarielhp/bws/main/profiles/<name>.json`.
    - Returns `(false, nil)` on HTTP 404 (clean miss).
    - Returns error on network or HTTP error.
    - Validates JSON and saves via `config.WriteTrustedFile` only on 200 OK.
  - Update `HandleProfileUpdate`:
    - Use `fetchRemoteProfile` directly for each installed global profile.
    - Never call `HandleProfileFetch` or synthesis.
    - Report skipped profiles on error and increment `updated` count only on successful remote updates.
  - Update `HandleProfileFetch`:
    - Refuse overwrite if target profile already exists and `!force`.
    - Attempt `fetchRemoteProfile`; if miss, fall back to synthesis via `HandleProfileNew`.
  - Update `HandleProfileNew`:
    - Refuse overwrite if target profile already exists and `!force`:
      `if _, err := os.Stat(targetPath); err == nil && !force { return fmt.Errorf("profile %q already exists at %s (use -f/--force to overwrite)", name, targetPath) }`.
- **Regression Testing**:
  - Unit tests in `internal/cli/profile_cmds_test.go` ensuring:
    1. `HandleProfileNew` fails when destination exists unless `--force` is provided.
    2. `fetchRemoteProfile` skips non-existent remote profiles without synthesizing or overwriting local profiles.

### 3. Non-Destructive Bind Removal with `ErrNoChange` (`internal/config/jsonc.go`, `internal/cli/bind_cmds.go`)
- **Root Cause**:
  `RemoveBindElement` mutates through `EditJSONC`, which calls `WriteTrustedFile`, triggering `backupConfig`. Even if no element is removed, the file is re-written and `.bak` is overwritten with the post-change contents.
- **Targeted Fix**:
  - In `internal/config/jsonc.go`:
    - Define sentinel error `var ErrNoChange = errors.New("no change")`.
    - In `EditJSONC`, if callback returns `ErrNoChange`, return `nil` immediately without writing or invoking `WriteTrustedFile`.
    - In `RemoveBindElement`, if `!found`, return `ErrNoChange`. Wrap check with `errors.Is(err, ErrNoChange)` to return `(false, nil)`.
  - In `internal/cli/bind_cmds.go:runBindDel`:
    - When `config.RemoveBindElement` returns `f == true`, break out of the `key` loop and the candidate loop immediately.
- **Regression Testing**:
  - Unit test in `internal/config/jsonc_test.go` validating that `RemoveBindElement` on a non-existent entry does not touch the target file or its `.bak` backup.
  - Integration/CLI test verifying `bws undo` properly restores deleted binds.

### 4. Stack Unwinding and Deferred Cleanup in Runner (`runner.go`, `main.go`)
- **Root Cause**:
  `buildAndRun` terminates the process prematurely with `os.Exit(exitErr.ExitCode())`, preventing deferred functions (`tmuxCleanup()`, `proxyServer.Close()`, etc.) from running.
- **Targeted Fix**:
  - In `runner.go`:
    - In `buildAndRun`, replace `if exitErr, ok := runErr.(*exec.ExitError); ok { os.Exit(exitErr.ExitCode()) }` with `return runErr`.
    - Allows caller defer blocks (`defer tmuxCleanup()`, `defer signal.Reset()`, `defer proxyServer.Close()`) to execute during standard return.
  - In `main.go`:
    - Check `if errors.As(err, &exitErr) && exitErr.ExitCode() > 0 { os.Exit(exitErr.ExitCode()) }` before `app.PrintError(err)`.
    - Ensures the process exits with the child's exact exit code cleanly without noisy wrapper error output.
- **Regression Testing**:
  - Unit/CLI test verifying exit code propagation from sandboxed commands without error wrapping, and confirming cleanup functions run on non-zero exits.

---

## Quality Metrics & Verification Gate
1. **Method Length & Metrics**:
   - Every modified and new function MUST adhere to AGENTS.md:
     - Method length <= 80 lines.
     - Cognitive complexity <= 15.
     - Nesting depth <= 4.
2. **Anti-Oscillation Ledger**:
   - Round 001 (systems) & Round 002 (security) remediations preserved intact:
     - `isSystemOrHomeRoot` and `isAlreadyBound` in `internal/bwrap/helpers.go`.
     - Symlink refusal in `atomicWriteFile`, `backupConfig`, `RestoreBackup`, `WriteTrustedFile`.
     - `util.UserTempDir` and user-scoped temp directory lifecycle.
     - Bounded HTTP reads (`maxHTTPResponseBytes = 5MB`).
     - Staged `.env*` secure file permissions (`0600`).
3. **Verification**:
   - Run `./tools/gate` (`verify_build.sh` + `audit_lines.rb`).
   - Run `/home/sariel/prog/26/bws/tools/gate` to ensure full parity.
4. **Summary & Commit**:
   - Document post-work report in `reviews/003_summary.md`.
   - Commit all changes cleanly.
