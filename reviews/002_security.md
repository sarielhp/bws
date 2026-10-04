# Systems Code Review Report #002 (Lens: security)

- **Date**: 2026-10-04
- **Auditor**: Gemini 3.8 Flash (Tier 0 Workhorse) via `tools/audit`
- **Focus Lens**: `security`
- **Model Tier**: `TIER0`
- **Backend**: `gemini`
- **Scope**: `.`
- **Status**: Action Required

---

Auditing via Gemini Flash (bws run) [Profile: security]...
[SEVERITY]: Critical
[LOCATION]: internal/bwrap/helpers.go:188-226
[ROOT CAUSE]: Path Traversal & Sandbox Filesystem Boundary Escape. `addQuartoBind` attempts to detect and bind Quarto bundle installations by navigating up two directory levels from the executable path: `binDir := filepath.Dir(realPath)` and `rootDir := filepath.Dir(binDir)`. When `quarto` is installed in standard system paths (e.g., `/bin/quarto`, `/usr/bin/quarto`, or `/usr/local/bin/quarto`), `rootDir` evaluates to `/` or `/usr`. Because `/` is not present in existing `*args`, the `alreadyBound` check evaluates to false, causing `addQuartoBind` to append `--ro-bind-try / /` to the Bubblewrap arguments. This mounts the entire host root filesystem read-only into `/`, breaking sandbox isolation and exposing all host files. Similarly, if `quarto` is installed in `~/.local/bin/quarto`, `rootDir` evaluates to `~/.local`, mounting `~/.local` read-only and leaking host credential stores such as `~/.local/share/keyrings` and `~/.local/share/gh`.
[FAILURE TRACE]:
1. The host environment has `quarto` installed in `/bin/quarto` (or symlinked to `/bin/quarto`).
2. A user launches any standard sandbox session via `bws`.
3. In `internal/bwrap/bwrap.go`, `BuildArgs` invokes `addFeatureMountArgs`, which calls `addQuartoBind(args)`.
4. `execLookPath("quarto")` finds `/bin/quarto`. `realPath` is `/bin/quarto`, `binDir` is `/bin`, and `rootDir` is `/`.
5. `alreadyBound` fails to match `/` in existing mount arguments.
6. `addQuartoBind` appends `"--ro-bind-try", "/", "/"` to the Bubblewrap command arguments.
7. The sandbox executes with the host root filesystem mounted at `/`, granting processes inside the sandbox read access across the entire host filesystem.
[REMEDIATION]:
Validate `rootDir` and `binDir` to ensure they never match `/`, system roots, or home directory roots before adding bind mounts:
```go
func isSystemOrHomeRoot(p string) bool {
	clean := filepath.Clean(p)
	if clean == "/" || clean == "/usr" || clean == "/bin" || clean == "/usr/local" {
		return true
	}
	home := filepath.Clean(util.HomeDir())
	return clean == home || clean == filepath.Join(home, ".local")
}

// In addQuartoBind:
if !isSystemOrHomeRoot(rootDir) {
	if fi, err := os.Stat(rootDir); err == nil && fi.IsDir() && !isAlreadyBound(*args, rootDir) {
		*args = append(*args, "--ro-bind-try", rootDir, rootDir)
	}
}
```

---

[SEVERITY]: Critical
[LOCATION]: internal/config/atomic.go:198-228 and internal/config/trust.go:69-80
[ROOT CAUSE]: Symlink Traversal and Arbitrary File Overwrite. While `AtomicPolicyWrite` explicitly checks `unix.O_NOFOLLOW` and refuses non-regular files to prevent symlink traversal attacks, `WriteTrustedFile` (used by `bws config set`, `bws bind add`, `bws path add`, `bws config create`, and `bws profile fetch`) delegates to `atomicWriteFile` and `backupConfig`. Both functions explicitly call `filepath.EvalSymlinks(path)` and overwrite the resolved target. When a user runs commands in a repository containing a symlink at `.bws/config.jsonc` (or `.bws.jsonc`) pointing to a file outside the workspace (such as `~/.bashrc`), writing policy modifications follows the symlink and overwrites the host file with JSON configuration, leading to host file corruption or arbitrary code execution upon next shell startup.
[FAILURE TRACE]:
1. A user clones an untrusted repository containing a symlink: `.bws/config.jsonc -> ~/.bashrc`.
2. The user runs a workspace configuration command, such as `bws config set enable_ssh true` or `bws bind add /data`.
3. The command calls `config.WriteTrustedFile(targetPath, out)`.
4. `backupConfig` evaluates the symlink, backing up `~/.bashrc` to `~/.bashrc.bak`.
5. `atomicWriteFile` executes `filepath.EvalSymlinks(path)`, resolving the path to `~/.bashrc`.
6. A temporary file containing JSON configuration is created in the target's directory and renamed over `~/.bashrc`.
7. `~/.bashrc` is corrupted with JSON text, and `WriteTrustedFile` marks the file trusted in the host trust store.
[REMEDIATION]:
Refuse to write through symlinks in `atomicWriteFile` and `backupConfig`, or ensure `WriteTrustedFile` delegates to `AtomicPolicyWrite` with `O_NOFOLLOW`:
```go
func atomicWriteFile(path string, data []byte) error {
	fi, err := os.Lstat(path)
	if err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("refusing to write through symlink: %s", path)
	}
	perm := os.FileMode(0644)
	if err == nil {
		perm = fi.Mode().Perm()
	}
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	// ... rest of write, sync, chmod, rename
}
```

---

[SEVERITY]: Critical
[LOCATION]: internal/sandbox/skeleton.go:73-77, internal/gitworkflow/gitworkflow.go:111-115, and internal/dbus/proxy.go:78-82
[ROOT CAUSE]: Predictable Shared Temporary Directory Denial of Service. In `internal/dbus/proxy.go`, the shared directory `/tmp/bws` is created with mode `0700` (`os.MkdirAll("/tmp/bws", 0700)`). On a multi-user Linux system, once one user runs `bws`, `/tmp/bws` is owned exclusively by that user with restrictive permissions. When any other user on the host subsequently attempts to launch `bws` (`sandbox.StageHome`) or `bws gw` (`gitworkflow.prepareClone`), calls to `os.MkdirTemp("/tmp/bws", "stage_")` and `os.MkdirTemp("/tmp/bws", "agent_")` fail with `permission denied`. Neither function implements fallback logic (unlike `sandboxTmpArgs`), resulting in a fatal error that prevents all other users from launching sandboxes. Furthermore, any local unprivileged attacker can pre-create `/tmp/bws` with mode `0700` to permanently block all users from running `bws`.
[FAILURE TRACE]:
1. User A runs `bws --dbus` on a shared workstation or server.
2. `/tmp/bws` is created with owner User A and permissions `0700`.
3. User B runs `bws`.
4. `sandbox.StageHome` calls `os.MkdirTemp("/tmp/bws", "stage_")`.
5. The call fails with `EACCES` (`permission denied`).
6. `bws` terminates with error `creating session stage directory: permission denied`.
[REMEDIATION]:
Scope the shared runtime directories per user (e.g., `/tmp/bws-<UID>`), verify ownership before use, and provide a fallback to standard `os.TempDir()`:
```go
func UserTempDir(sub string) (string, error) {
	base := filepath.Join(os.TempDir(), fmt.Sprintf("bws-%d", os.Getuid()))
	if err := os.MkdirAll(base, 0700); err != nil {
		return os.MkdirTemp("", "bws_"+sub+"_")
	}
	return os.MkdirTemp(base, sub+"_")
}
```

---

[SEVERITY]: Major
[LOCATION]: internal/cli/profile_cmds_ext.go:155, internal/cli/profile_cmds_ext.go:198, and internal/profile/generate.go:149
[ROOT CAUSE]: Unbounded Network Stream Read / Resource Exhaustion. In `HandleProfileSearch`, `HandleProfileFetch`, and `fetchHomebrewFormula`, HTTP responses received from external endpoints (such as `formulae.brew.sh` and raw GitHub content) are read directly using `io.ReadAll(resp.Body)` without wrapping `resp.Body` in `io.LimitReader`. A malicious proxy, man-in-the-middle attacker, or compromised upstream server sending an infinite byte stream or an excessively large payload will cause unbounded slice growth in the Go runtime, resulting in memory exhaustion and fatal OOM termination of the CLI process.
[FAILURE TRACE]:
1. A user executes `bws profile fetch <name>` or `bws profile search <query>`.
2. The HTTP client issues a request to the remote endpoint.
3. An adversary in a MITM position or a rogue upstream server returns HTTP 200 OK with an endless or multi-gigabyte data stream.
4. `io.ReadAll(resp.Body)` repeatedly doubles the internal buffer.
5. Host RAM is exhausted, and the process terminates via `runtime: out of memory`.
[REMEDIATION]:
Enforce an upper bound on bytes read from HTTP response bodies using `io.LimitReader`:
```go
const maxHTTPResponseBytes = 5 * 1024 * 1024 // 5 MB

body, err := io.ReadAll(io.LimitReader(resp.Body, maxHTTPResponseBytes))
if err != nil {
	return false
}
```

---

[SEVERITY]: Moderate
[LOCATION]: internal/gitworkflow/helpers.go:127
[ROOT CAUSE]: Insecure File Permissions on Staged Credentials. When `copyConfigFiles` replicates repository configurations into the agent clone workspace (`/tmp/bws/agent_XXXX`), it matches any file starting with `.env` (`strings.HasPrefix(name, ".env")`). It writes these files using `dest.WriteFile(path, data, 0644)`. This creates sensitive files containing API keys, database passwords, and tokens with world-readable permissions (`0644`), disregarding the restrictive permissions (such as `0600`) typically applied to `.env` files.
[FAILURE TRACE]:
1. A repository contains `.env` or `.env.local` containing secrets with mode `0600`.
2. The user executes `bws gw` to launch an isolated agent session.
3. `copyConfigFiles` identifies `.env` and calls `copyPolicyFile`.
4. `dest.WriteFile(path, data, 0644)` writes the secret file to the agent workspace with mode `0644`.
5. The secret file becomes world-readable to other users on the host system.
[REMEDIATION]:
Preserve the source file's permissions or restrict permissions to `0600` for `.env` files:
```go
perm := info.Mode().Perm()
if strings.HasPrefix(filepath.Base(path), ".env") {
	perm = 0600
}
if err := dest.WriteFile(path, data, perm); err != nil {
	return err
}
```
