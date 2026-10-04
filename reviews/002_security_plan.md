# Security Audit Remediation Plan (Report #002)

## Executive Summary
This remediation plan addresses all 5 findings identified in `reviews/002_security.md` (Lens: security). Following comprehensive architectural and security triage, all 5 findings are confirmed as genuine vulnerabilities spanning sandbox boundary escape, symlink-directed arbitrary host file overwrite, predictable shared temporary directory denial-of-service, unbounded network read resource exhaustion, and world-readable credential staging permissions.

## Triage Assessment

| Finding | Severity | Component | Finding Type | Triage Verdict | Rationale |
|---|---|---|---|---|---|
| 1. Quarto Bind Traversal & Sandbox Escape | Critical | `internal/bwrap/helpers.go:188-226` | Filesystem Isolation Boundary Escape | **Genuine Defect** | For system installations (e.g. `/bin/quarto`), `rootDir` resolves to `/`, mounting the entire host root filesystem read-only via `--ro-bind-try / /`. For `~/.local/bin/quarto`, it mounts `~/.local`, leaking host credential stores. |
| 2. Symlink Traversal & Host File Overwrite | Critical | `internal/config/atomic.go:198-228`, `internal/config/trust.go:69-80` | Symlink Traversal & Arbitrary File Overwrite | **Genuine Defect** | While `AtomicPolicyWrite` checks `O_NOFOLLOW`, `WriteTrustedFile` delegates to `atomicWriteFile` and `backupConfig`, which explicitly call `filepath.EvalSymlinks` and follow symlinks, allowing malicious repositories to overwrite arbitrary host files (e.g. `~/.bashrc`). |
| 3. Predictable Shared Temp Dir DoS | Critical | `internal/sandbox/skeleton.go:73-77`, `internal/gitworkflow/gitworkflow.go:111-115`, `internal/dbus/proxy.go:78-82` | Denial of Service & Multi-User Race | **Genuine Defect** | Shared `/tmp/bws` created with mode `0700` locks out subsequent users on multi-user systems. Unprivileged users or attackers can pre-create `/tmp/bws` with `0700` to completely disable `bws` and `bws gw`. |
| 4. Unbounded Network Stream Read | Major | `internal/cli/profile_cmds_ext.go:155, 198`, `internal/profile/generate.go:149` | Resource Exhaustion (OOM) | **Genuine Defect** | `io.ReadAll(resp.Body)` without an upper bound allows infinite streams or multi-gigabyte responses from upstream endpoints or MITM proxies to trigger out-of-memory crashes. |
| 5. Insecure Permissions on Staged Secrets | Moderate | `internal/gitworkflow/helpers.go:127` | Credential Exposure | **Genuine Defect** | When staging `.env*` files from repository roots into agent workspaces, files are created with mode `0644` (world-readable), ignoring source permissions and exposing secrets to other local users. |

---

## Detailed Remediation Strategies & Architecture

### 1. Quarto Bind Boundary Validation (`internal/bwrap/helpers.go`)
- **Root Cause**:
  `addQuartoBind` computes `rootDir` and `binDir` from `quarto` executable path. When `quarto` is `/bin/quarto`, `rootDir` is `/`, which is appended as `--ro-bind-try / /`. When in `~/.local/bin/quarto`, `rootDir` is `~/.local`.
- **Targeted Fix**:
  - Implement `isSystemOrHomeRoot(p string) bool` to reject `/`, `.`, `/usr`, `/bin`, `/sbin`, `/usr/local`, `/usr/bin`, `/usr/sbin`, `/etc`, `/var`, `/opt`, `/home`, `$HOME`, `$HOME/.local`, `$HOME/.local/bin`, and `$HOME/bin`.
  - Implement `isAlreadyBound(args []string, target string) bool` to deduplicate bind check logic.
  - Guard both `rootDir` and `binDir` binding with `!isSystemOrHomeRoot(...)`.
  - Refactor `addOptBind` to use `isAlreadyBound` for code reuse and length constraints.
- **Regression Testing**:
  - Unit tests in `internal/bwrap/helpers_test.go` validating `isSystemOrHomeRoot` and verifying that `addQuartoBind` never appends system roots or `$HOME/.local` to Bubblewrap arguments.

### 2. Strict Symlink Refusal in Atomic Config & Trust Persistence (`internal/config/atomic.go`, `internal/config/backup.go`, `internal/config/trust.go`)
- **Root Cause**:
  `atomicWriteFile` and `backupConfig` call `filepath.EvalSymlinks`, writing through symlinks and backing up symlink targets.
- **Targeted Fix**:
  - In `atomicWriteFile`, inspect destination using `os.Lstat(path)`. If it exists and is a symlink (`fi.Mode()&os.ModeSymlink != 0`), abort with `fmt.Errorf("refusing to write through symlink: %s", path)`.
  - In `backupConfig`, inspect `path` with `os.Lstat(path)`. If it is a symlink, return an error refusing to backup through symlinks. Remove symlink evaluation from `resolvedConfigPath`.
  - In `RestoreBackup`, verify that `path` is not a symlink before restoring.
  - In `WriteTrustedFile`, enforce that symlinks are rejected upfront.
  - Update `TestAtomicWriteFileKeepsSymlinkAndMode` to assert that `atomicWriteFile` rejects symlinks.
- **Regression Testing**:
  - Unit tests `TestWriteTrustedFileRejectsSymlink` and `TestBackupConfigRejectsSymlink` in `internal/config/security_test.go` verifying that attempts to write config through a symlink to a sensitive target fail immediately without modifying the target.

### 3. User-Scoped Runtime Directory & Resilient Fallback (`internal/util/util.go`, `internal/sandbox/skeleton.go`, `internal/gitworkflow/gitworkflow.go`, `internal/dbus/proxy.go`, `internal/bwrap/bwrap.go`)
- **Root Cause**:
  Shared `/tmp/bws` directory created with `0700` causes multi-user denial of service and lacks fallback in `sandbox.StageHome` and `gitworkflow.prepareClone`.
- **Targeted Fix**:
  - Implement `util.UserTempDir(sub string) (string, error)`:
    - Target per-user base: `/tmp/bws-<UID>` with mode `0700`.
    - Verify directory ownership and permissions (`fi.Mode().Perm()&0077 == 0`).
    - On any permission failure or inability to create per-user base, fall back to `os.MkdirTemp("", "bws_"+sub+"_")`.
  - Update `sandbox.StageHome` to use `util.UserTempDir("stage")`.
  - Update `gitworkflow.prepareClone` to use `util.UserTempDir("agent")`.
  - Update `dbus.Start` to use `util.UserTempDir("dbus_proxy")`.
  - Update `sandboxTmpArgs` to use `util.UserTempDir("sandbox")`.
  - Update `gitworkflow.PruneInDir` to scan both per-user temp base and legacy `/tmp/bws`.
- **Regression Testing**:
  - Unit test `TestUserTempDir` in `internal/util/util_test.go` verifying directory creation under `/tmp/bws-<UID>`, restrictive permissions (0700), and fallback behavior when base is blocked.

### 4. Bounded HTTP Response Stream Reading (`internal/cli/profile_cmds_ext.go`, `internal/profile/generate.go`)
- **Root Cause**:
  `io.ReadAll(resp.Body)` without stream limits allows unbounded memory growth and process crash on malicious or corrupted HTTP responses.
- **Targeted Fix**:
  - Define `const maxHTTPResponseBytes = 5 * 1024 * 1024` (5 MB).
  - In `HandleProfileSearch`: read via `io.LimitReader(resp.Body, maxHTTPResponseBytes)`.
  - In `HandleProfileFetch`: read via `io.LimitReader(resp.Body, maxHTTPResponseBytes)`.
  - In `fetchHomebrewFormula`: read via `io.LimitReader(resp.Body, maxHTTPResponseBytes)`.
  - In `fetchFirejail`: scan via `io.LimitReader(resp.Body, maxHTTPResponseBytes)`.
- **Regression Testing**:
  - Unit tests with `httptest.Server` serving streams exceeding 5MB, verifying that `HandleProfileFetch` and `fetchHomebrewFormula` safely cap input and handle oversized payloads without OOM.

### 5. Restrictive Permissions for Staged Secret Files (`internal/gitworkflow/helpers.go`)
- **Root Cause**:
  `copyPolicyFile` copies `.env*` files with `0644`, exposing API keys and tokens to all host users.
- **Targeted Fix**:
  - In `copyPolicyFile`:
    ```go
    perm := info.Mode().Perm()
    if strings.HasPrefix(filepath.Base(path), ".env") {
        perm = 0600
    }
    if err := dest.WriteFile(path, data, perm); err != nil {
        return err
    }
    ```
  - Preserves source file permissions for regular config files, and enforces `0600` for all `.env*` files.
- **Regression Testing**:
  - Unit test `TestCopyConfigFilesPreservesSecurePerms` in `internal/gitworkflow/helpers_test.go` ensuring staged `.env` files are created with mode `0600`.

---

## Quality Metrics & Verification Gate
1. All new or modified functions MUST satisfy AGENTS.md:
   - Line length <= 80 lines.
   - Cognitive complexity <= 15.
   - Nesting depth <= 4.
2. Anti-oscillation invariants preserved: no regressions to Round 001 fixes.
3. Verification with `tools/gate` (`verify_build.sh` + `audit_lines.rb`).
4. Save post-work summary to `reviews/002_summary.md` and commit changes.
