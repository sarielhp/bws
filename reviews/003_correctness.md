# Systems Code Review Report #003 (Lens: correctness)

- **Date**: 2026-10-04
- **Auditor**: Claude / Codex (Tier 1 Standard) via `tools/audit`
- **Focus Lens**: `correctness`
- **Model Tier**: `STANDARD`
- **Backend**: `auto`
- **Scope**: `.`
- **Status**: Action Required

---

Auditing via Claude CLI (sonnet, tier: standard) [Profile: correctness]...
[SEVERITY]: Critical
[LOCATION]: `internal/cli/stack_cmds.go`, `HandleStackUpdate` (`json.MarshalIndent(localCfg, "", "  ")` then `AtomicPolicyWrite`)
[ROOT CAUSE]: The local config is parsed into `config.Config` and re-serialized wholesale. This round trip is lossy in four ways:
- `BindEntry` has `UnmarshalJSON` but no `MarshalJSON`, so it is written as `{"Host":"…","Sandbox":""}`. `UnmarshalJSON` accepts only a string or a `[host, sandbox]` pair, so the written file cannot be loaded again.
- `Parse` already replaced `@@HOME@@` with the absolute home path, so portability tokens are lost.
- All JSONC comments are dropped.
- Unset fields are written as `"system": null`, `"binds_rw": null`, and so on.

[FAILURE TRACE]:
1. A workspace has `.bws/config.jsonc` with `"stack": "go-agent"` and `"binds_rw": ["~/.cache/go-build"]`, for example added via `bws mount add`.
2. The embedded stack digest changes after a bws upgrade.
3. The user runs `bws stack update -y`.
4. The file is rewritten with `"binds_rw":[{"Host":"/home/u/.cache/go-build","Sandbox":""}]`.
5. Every later command that loads the config fails with "bind entry must be a string or [host, sandbox] pair".
6. The user's comments and `@@HOME@@` tokens are lost. Only the `.bak` copy retains the original.

[REMEDIATION]: Patch only the `reviewed_stack` member of the original bytes and keep everything else intact:
```go
ast, err := hujson.Parse(before)
if err != nil { return err }
val, _ := json.Marshal(config.ProfileApproval{Source: upstream.Source, SHA256: upstreamDigest})
patch := fmt.Sprintf(`[{"op":"add","path":"/reviewed_stack","value":%s}]`, val)
if err := ast.Patch([]byte(patch)); err != nil { return err }
updatedJSON := ast.Pack()
```

---

[SEVERITY]: Critical
[LOCATION]: `internal/cli/profile_cmds_ext.go`, `HandleProfileUpdate` → `HandleProfileFetch` fallback → `HandleProfileNew` → `profile.SaveProfile`
[ROOT CAUSE]: Updating an installed profile is not idempotent or safe on failure. `HandleProfileFetch` falls back to synthesis on any fetch failure: network error, 404, or invalid JSON. `HandleProfileNew` then calls `SaveProfile` with no existence check, no `--force` gate, and no backup. Profile files are not covered by the config single-slot backup. The update loop also counts such overwrites as "updated".

[FAILURE TRACE]:
1. `~/.config/bws/profiles/` holds a hand-edited or `bws profile save`-created profile whose name is not in the remote catalog, for example `fish`, or any name that matches a Homebrew formula or Firejail profile.
2. The user runs `bws profile update`, either offline or for a profile with no upstream copy.
3. The remote fetch fails and the code falls back to `HandleProfileNew("fish", …, force=false)`.
4. Synthesis succeeds, because Homebrew, Firejail, or a binary on `$PATH` is found.
5. `SaveProfile` replaces the user's customized `fish.json` with the synthesized skeleton.
6. All custom mounts, masks, and reviewed fingerprints are lost, and `updated++` reports success.

[REMEDIATION]: Never synthesize or overwrite from the update path. Add an `allowSynthesis` parameter or a separate fetch-only function.
```go
func fetchProfile(name string, local bool) (bool, error) { /* remote only; returns false on miss */ }

// HandleProfileUpdate
if ok, err := fetchProfile(pName, false); err != nil {
    fmt.Fprintf(os.Stderr, "skip %s: %v\n", pName, err)
} else if ok {
    updated++
}
```
Also refuse in `HandleProfileNew` when the target file exists and `!force`.

---

[SEVERITY]: Moderate
[LOCATION]: `internal/cli/bind_cmds.go`, `runBindDel` (inner `for _, key := range []string{"binds_rw","binds_ro"}` loop)
[ROOT CAUSE]: The loop does not stop after a successful removal. Each `RemoveBindElement` call goes through `EditJSONC`, which always rewrites the file via `WriteTrustedFile`, and that calls `backupConfig`. A later no-op call overwrites the single-slot `.bak` with the already-modified state.

[FAILURE TRACE]:
1. Local config has `/data` in `binds_rw`, and `binds_ro` exists.
2. The user runs `bws mount rm /data`.
3. The `binds_rw` call removes the entry, and the backup holds the original.
4. The `binds_ro` call finds nothing but still rewrites the file, so the backup now holds the post-removal contents.
5. `bws undo` restores the config without `/data`, so the undo is a silent no-op.

[REMEDIATION]: Make `RemoveBindElement` skip the write when nothing was found, and stop searching once a match is found:
```go
// in RemoveBindElement's callback
if !found { return errNoChange }
// and in EditJSONC: treat errNoChange as success without writing
```
In `runBindDel`, `break` out of the key loop after `f == true`, or only rewrite when `found`.

---

[SEVERITY]: Moderate
[LOCATION]: `runner.go`, `buildAndRun` (`os.Exit(exitErr.ExitCode())`) combined with `defer tmuxCleanup()` in `runDefault` and `runExec`
[ROOT CAUSE]: `os.Exit` skips deferred functions. When the sandboxed command exits non-zero, `tmuxCleanup`, `proxyServer.Close`, and `signal.Reset` never run. As a result, the host tmux window and pane titles keep the `[BTS] ` prefix, and automatic-rename is never restored.

[FAILURE TRACE]:
1. Inside host tmux, the user runs `bws run bash` or `bws` as a direct shell.
2. The shell exits with status 1, for example after a failed last command.
3. `buildAndRun` calls `os.Exit(1)`.
4. The tmux window name stays "[BTS] …" permanently and the original title is not restored.

[REMEDIATION]: Return the exit code instead of exiting in the middle of the call stack, and exit only in `main`:
```go
if exitErr, ok := runErr.(*exec.ExitError); ok {
    return exitErr // main already maps *exec.ExitError to os.Exit(code)
}
```
`main` already uses `errors.As(err, &exitErr)`, so deferred cleanups will run first.
