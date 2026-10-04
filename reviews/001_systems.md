# Systems Code Review Report #001 (Lens: systems)

- **Date**: 2026-10-04
- **Auditor**: Gemini 3.8 Flash (Tier 0 Workhorse) via `tools/audit`
- **Focus Lens**: `systems`
- **Model Tier**: `TIER0`
- **Backend**: `gemini`
- **Scope**: `.`
- **Status**: Action Required

---

Auditing via Gemini Flash (bws run) [Profile: systems]...
[SEVERITY]: Major
[LOCATION]: `internal/cli/tui_select.go:58-62`
[ROOT CAUSE]: Signal swallowing and unhandled signal lifecycle. In [`SelectStackInteractive`](file:///home/sariel/prog/26/bws/internal/cli/tui_select.go#L58-L62), `signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)` is called, which intercepts `SIGINT` and `SIGTERM` into `sigCh`. However, `sigCh` is never read anywhere in the package, and no goroutine or select loop monitors it. Calling `signal.Notify` suppresses the Go runtime's default process termination behavior for these signals. Because [`runSelectorLoop`](file:///home/sariel/prog/26/bws/internal/cli/tui_select.go#L7319) enters a synchronous blocking read on `os.Stdin`, any external `SIGTERM` (sent by a process supervisor, job runner, or `kill <pid>`) is buffered in `sigCh` and ignored. The process hangs indefinitely. When subsequently terminated via uncatchable `SIGKILL`, the deferred `term.Restore(stdinFd, oldState)` cannot execute, leaving the user's host terminal stuck in raw mode.
[FAILURE TRACE]:
1. User or script invokes interactive stack selection (`bws stack select`).
2. `term.MakeRaw(stdinFd)` puts the terminal into raw mode.
3. `sigCh := make(chan os.Signal, 1)` and `signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)` are executed.
4. `runSelectorLoop` blocks on `readSelectorKey(os.Stdin)`, waiting for user keypresses.
5. An external process or supervisor sends `SIGTERM` (`kill -15 <pid>`).
6. The Go runtime delivers `SIGTERM` to `sigCh` instead of executing default termination.
7. Because nothing reads `sigCh`, the signal is ignored and the process remains blocked in `os.Stdin.Read`.
8. The operator is forced to send `SIGKILL` (`kill -9 <pid>`), which bypasses deferred functions and leaves the terminal driver in raw mode (echo disabled, line discipline broken).
[REMEDIATION]:
Monitor `sigCh` concurrently to restore the terminal state and exit on signal receipt:
```go
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)

	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-sigCh:
			_ = term.Restore(stdinFd, oldState)
			os.Exit(130)
		case <-done:
		}
	}()
```

---

[SEVERITY]: Major
[LOCATION]: `internal/profile/runner.go:86-112, 172-187`
[ROOT CAUSE]: Unexpected slice and pointer aliasing causing caller configuration corruption. [`profileTestConfig`](file:///home/sariel/prog/26/bws/internal/profile/runner.go#L86-L112) uses a custom helper [`copyConfig(cfg)`](file:///home/sariel/prog/26/bws/internal/profile/runner.go#L172-L187) which performs a shallow copy `cp := *c` and omits `PassEnv`, `Mask`, `Copy`, and `Features`. In `profileTestConfig`, `testCfg.PassEnv` and `testCfg.Mask` are appended to directly via `append(...)`, which mutates the underlying capacity buffers of `cfg.PassEnv` and `cfg.Mask` in the caller when `cap > len`. Furthermore, `config.MergeFeatures(testCfg.Features, resolved.Features)` returns `cfg.Features` directly when `resolved.Features` is `nil`. When `resolved.UnshareNet` is true, `testCfg.Features.NoNet = &enabled` mutates `cfg.Features.NoNet` in the caller's shared config, permanently setting `NoNet: true` across subsequent operations.
[FAILURE TRACE]:
1. Caller passes a `cfg *config.Config` into `RunProfileTests` where `cfg.Features = &config.FeaturesConfig{}` and `cfg.PassEnv = make([]string, 1, 4)`.
2. Profile has `UnshareNet: true` and `Features: nil`.
3. `profileTestConfig` calls `copyConfig(cfg)` which shallow-copies the struct.
4. `config.MergeFeatures(testCfg.Features, nil)` returns `cfg.Features`.
5. `testCfg.Features.NoNet = &enabled` mutates the caller's `cfg.Features.NoNet` pointer.
6. `testCfg.PassEnv = append(testCfg.PassEnv, resolved.PassEnv...)` mutates the shared underlying backing array of `cfg.PassEnv`.
7. After `RunProfileTests` finishes, the caller's `cfg` permanently retains network unsharing (`NoNet = true`) and mutated environment settings for future sandbox runs.
[REMEDIATION]:
Replace the shallow copy with [`config.Clone`](file:///home/sariel/prog/26/bws/internal/config/clone.go#L8287), which is specifically implemented to deep-copy all slices, maps, and pointer fields:
```go
func profileTestConfig(cfg *config.Config, resolved *ResolvedProfile) *config.Config {
	testCfg := config.Clone(cfg)
	if testCfg == nil {
		testCfg = &config.Config{}
	}
	testCfg.Features = config.MergeFeatures(testCfg.Features, resolved.Features)
...
```

---

[SEVERITY]: Major
[LOCATION]: `internal/proxy/proxy.go:134-150`
[ROOT CAUSE]: Goroutine and socket leak in the forward proxy connection pool. In [`handleHTTP`](file:///home/sariel/prog/26/bws/internal/proxy/proxy.go#L134-L150), a new `&http.Transport{}` is allocated on every incoming request. When `resp.Body.Close()` is called, the transport places the connection into its internal idle connection pool and keeps background `readLoop` and `writeLoop` goroutines alive waiting for keep-alive timeouts. Because `transport.CloseIdleConnections()` is never called, and the transport is neither reused across requests nor configured with `DisableKeepAlives: true`, each forwarded HTTP request leaks active goroutines and open TCP sockets until connection limits or file descriptors are exhausted.
[FAILURE TRACE]:
1. Ephemeral proxy is enabled with `bws run --proxy`.
2. A tool inside the sandbox (e.g. `curl`, `pip`, or `npm`) executes a batch of HTTP requests.
3. Every request triggers `handleHTTP`, allocating a separate `http.Transport` instance.
4. When `resp.Body.Close()` runs, each ephemeral transport retains an active socket in its idle pool with two background goroutines.
5. In scenarios with numerous dependency fetches, file descriptor exhaustion (`too many open files`) or goroutine memory exhaustion occurs in the proxy process.
[REMEDIATION]:
Explicitly disable keep-alives and close idle connections on the ephemeral transport:
```go
	transport := &http.Transport{
		DisableKeepAlives: true,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			dialer := &net.Dialer{Timeout: 15 * time.Second}
			conn, err := dialer.DialContext(ctx, "tcp4", addr)
			if err != nil {
				return dialer.DialContext(ctx, "tcp", addr)
			}
			return conn, nil
		},
	}
	defer transport.CloseIdleConnections()
```

---

[SEVERITY]: Major
[LOCATION]: `internal/config/trust.go:52-61`
[ROOT CAUSE]: Non-atomic file overwrite without tempfile+rename and fsync, risking corrupted trust state across crashes. [`trustContents`](file:///home/sariel/prog/26/bws/internal/config/trust.go#L52-L61) persists the SHA256 checksum of an approved workspace configuration directly via `os.WriteFile(record, ...)` with mode `0600`. `os.WriteFile` truncates and writes the destination file in place without fsync or atomic rename. If `bws` crashes, is killed (SIGINT/SIGTERM), or loses power while writing the record, the trust record is left empty or partially written. On subsequent runs, `ReadTrustedFile` reads the truncated digest, detects a hash mismatch, and locks out the workspace with "untrusted or changed local configuration", breaking configuration trust persistence across crashes.
[FAILURE TRACE]:
1. User modifies configuration using `bws config set ...` or runs `WriteTrustedFile`.
2. `atomicWriteFile(path, data)` writes the configuration file atomically.
3. `trustContents` starts writing `~/.config/bws/trusted/<hash>` via `os.WriteFile`.
4. A SIGINT, system crash, or power loss interrupts the process mid-write.
5. `~/.config/bws/trusted/<hash>` contains 0 bytes or a truncated checksum.
6. The user runs `bws run`. `ReadTrustedFile` checks `string(approved) != digest`, causing a fatal error and refusing to load the legitimate configuration.
[REMEDIATION]:
Use `atomicWriteFile` (already available in `internal/config/atomic.go`) to ensure synced tempfile creation and atomic rename:
```go
func trustContents(path string, data []byte) error {
	record, err := trustRecord(path)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(TrustDir(), 0700); err != nil {
		return err
	}
	return atomicWriteFile(record, []byte(fmt.Sprintf("%x", sha256.Sum256(data))))
}
```

---

[SEVERITY]: Moderate
[LOCATION]: `internal/util/tmux.go:31-47`
[ROOT CAUSE]: Parsing fragility and data corruption in tmux format string parsing with user-controlled window titles. [`SetHostTmuxTitle`](file:///home/sariel/prog/26/bws/internal/util/tmux.go#L31-L47) executes `tmux display-message -p "#{pane_id}:#W:#{automatic-rename}:#{pane_title}"` and splits the output using `strings.SplitN(..., ":", 4)`. In tmux, window names (`#W`) frequently contain colons (e.g. `vim:main.go`, `server:8080`, `git:branch`). When a colon is present in `#W`, `SplitN` misaligns all subsequent fields: `WindowName` is truncated, `automatic-rename` receives the remaining part of the window name (failing the `== "1"` check), and `PaneTitle` is prepended with the actual rename flag. Upon sandbox termination, `cleanup()` renames the window to the corrupted, truncated name.
[FAILURE TRACE]:
1. User is inside a tmux session with window name `dev:server.go`.
2. User runs `bws` with a direct shell.
3. `SetHostTmuxTitle` executes `tmux display-message -p "#{pane_id}:#W:#{automatic-rename}:#{pane_title}"`.
4. Output is `%1:dev:server.go:1:bash`.
5. `strings.SplitN(out, ":", 4)` yields:
   - `parts[0] = "%1"`
   - `parts[1] = "dev"`
   - `parts[2] = "server.go"`
   - `parts[3] = "1:bash"`
6. `state.WindowName` is assigned `"dev"` and `state.AutoRename` becomes `false` (`"server.go" == "1"`).
7. On exit, `cleanup()` executes `tmux rename-window -t %1 dev`, destroying the user's original window name `dev:server.go`.
[REMEDIATION]:
Use a non-colliding delimiter such as `\t` (tab) in the tmux format string:
```go
	out, err := exec.Command("tmux", "display-message", "-p", "#{pane_id}\t#W\t#{automatic-rename}\t#{pane_title}").Output()
	if err != nil {
		return func() {}, err
	}

	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 4)
```

---

[SEVERITY]: Moderate
[LOCATION]: `internal/stack/registry.go:178-200`
[ROOT CAUSE]: Missing fsync before rename and predictable temporary file collisions in stack persistence. [`SaveUserStack`](file:///home/sariel/prog/26/bws/internal/stack/registry.go#L178-L200) claims to persist user stacks atomically, but writes data via `os.WriteFile(tmp, data, 0644)` followed by `os.Rename(tmp, target)`. `os.WriteFile` does not call `f.Sync()`. Renaming unsynced files can lead to zero-length or corrupted target files if a crash occurs before dirty filesystem pages are flushed to physical storage. Additionally, `tmp := target + fmt.Sprintf(".tmp-%d", os.Getpid())` uses a deterministic path based solely on PID rather than secure temporary file generation (`os.CreateTemp`).
[FAILURE TRACE]:
1. User saves a stack using `bws stack save ...`.
2. `SaveUserStack` writes `data` to `tmp` using `os.WriteFile`.
3. `os.Rename(tmp, target)` updates the directory inode pointer.
4. A sudden power loss or kernel panic occurs before the page cache flushes dirty blocks.
5. On reboot, the filesystem metadata points to the new inode, but the blocks are unwritten, resulting in an empty or corrupted stack JSON file.
[REMEDIATION]:
Write and fsync via `os.CreateTemp` prior to rename:
```go
	f, err := os.CreateTemp(dir, "."+s.Name+".json.tmp-*")
	if err != nil {
		return "", fmt.Errorf("creating temporary stack file: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, target); err != nil {
		return "", fmt.Errorf("persisting stack file: %w", err)
	}
```

---

[SEVERITY]: Moderate
[LOCATION]: `runner.go:77-87, 97-124`
[ROOT CAUSE]: Unsynchronized concurrent access on `dbusProxy` pointer and leaked background signal goroutine. [`setupSandboxHome`](file:///home/sariel/prog/26/bws/runner.go#L77-L87) registers a signal handler with `signal.Notify` and spawns a background goroutine that reads `dbusProxy` via closure `getDBus()`. Concurrently, [`buildAndRun`](file:///home/sariel/prog/26/bws/runner.go#L97-L124) writes `dbusProxy = setupDBusService(...)` without mutex or atomic synchronization. Furthermore, when `buildAndRun` completes normally, `signal.Stop(sigChan)` is never called, leaving the goroutine permanently blocked on `<-sigChan` and retaining references to the staging directory cleanup closure.
[FAILURE TRACE]:
1. `buildAndRun` invokes `setupSandboxHome`, which registers `sigChan` and spawns `go func() { <-sigChan ... getDBus() ... }()`.
2. The main thread continues execution and invokes `dbusProxy = setupDBusService(...)`.
3. If an interrupt or termination signal arrives during or around initialization, `getDBus()` reads `dbusProxy` while `dbusProxy` is being written to by `buildAndRun`, triggering a data race.
4. If execution finishes normally, `sigChan` remains registered in the Go signal table, leaking the background goroutine.
[REMEDIATION]:
Manage `sigChan` lifecycle explicitly, stop notifications on return, and synchronize access or initialize `dbusProxy` before launching the signal handler:
```go
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer signal.Stop(sigChan)
	done := make(chan struct{})
	defer close(done)
	go func() {
		select {
		case <-sigChan:
			if p := getDBus(); p != nil {
				_ = p.Close()
			}
			cleanup()
			os.Exit(130)
		case <-done:
		}
	}()
```
