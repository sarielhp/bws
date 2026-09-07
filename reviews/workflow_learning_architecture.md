I grounded this in the actual tree rather than the design doc — `internal/learn` already implements most of the proposal, which changes the answers. I have the full review written out below; I couldn't save it to the empty `reviews/workflow_learning_architecture.md` you'd staged because the write wasn't approved — say the word and I'll drop it there.

---

# Architectural review — workflow-driven sandbox learning in `bws`

## Executive summary

**The pivot is sound, and the design doc under-sells it by mis-locating the risk.** Two facts from the code reshape all three challenges:

**1. System toolchain coverage already comes from the read-only system root, not from capability profiles.** `internal/config/assets/default_config.jsonc:48-65` binds `/usr`, `/bin`, `/sbin`, `/lib*` read-only, and `addOptBind` (`internal/bwrap/helpers.go:159`) adds `/opt` unconditionally. Every distro-packaged binary, header, LSP, `/usr/share/zoneinfo`, MIME db and CA bundle is *already* in the sandbox. What capability profiles actually contribute is `$HOME` state/cache dirs (because `StageHome` gives an ephemeral home), user-local toolchains (`~/.cargo`, `~/.rustup`, nvm/pyenv/nix), env/PATH, and feature toggles. That's a fifth of the surface Challenge 1 assumes — and exactly the machine-specific part where hand-maintained profiles diverge and tracing is strongest.

**2. The negative mask layer contained the credential leak, not profile hygiene.** Your cited example is directionally right but mechanically wrong: no profile in `profiles/` requires `gh`. The leak in the tree today is `go-dev → git → binds_ro ~/.git-credentials` (`profiles/git.json:12-21`). It's neutralised only because `BlockGH` defaults true (`config.go:257`), `collectForgeMasks` covers `~/.git-credentials` (`helpers.go:255`), and `addMaskArgs` runs **last** in `BuildArgs` so masks override binds. That's the strongest argument for your split: the invariant deny-by-default layer is load-bearing; the additive layer is not trustworthy.

**The real danger isn't coverage — it's that `bws learn` as built reproduces the exact vulnerability class that motivates the pivot, invisibly**, because it traces on the host, filters secrets through a 9-entry denylist, and never verifies its output.

| Dimension | Rating | Assessment |
| :--- | :---: | :--- |
| Strategic direction | **9/10** | Correct. The two layers have opposite trust semantics; merging them was the original mistake. |
| Challenge 1 — coverage | **7/10** | Much smaller than stated. Remaining gap is cold-start + interactive discovery; answer is fast recovery, not a guarantee. |
| Challenge 2 — delta | **6/10** | Already implemented and structurally right. Four defects (§3). |
| Challenge 3 — perf | **4/10** | Menu misses the two things that matter: `--seccomp-bpf`, and tracing *failures inside the sandbox*. Drop `LD_PRELOAD`. |
| `bws learn` security (as built) | **2/10** | Read-side denylist misses `~/.ssh`, `~/.kube`, `~/.docker`, `~/.npmrc`, `~/.cargo/credentials.toml`, keyrings, browsers. |
| `bws learn` correctness (as built) | **3/10** | `execve` is never traced, so gate-script learning can't discover the tools the gate script runs. |
| Portability of result | **5/10** | Divergence relocates into committed `.bws/config.jsonc`. No story for "learned on Arch, checked out on Debian/CI". |

---

## 1. Viability and failure modes

### 1.1 Right direction, for an unstated reason: trust asymmetry

A hardening mask is *safe to be wrong* — `no-gh` masking a nonexistent `/usr/local/bin/gh` is a no-op. So it can be speculative, distro-agnostic, shipped in the binary. A capability bind is *unsafe to be wrong* — `git` binding `~/.git-credentials` on a machine where that file exists silently puts a credential store in a repo-isolated sandbox. It must be minimal, verified, machine-derived.

These two objects have no business sharing a file format, a registry, a resolver, and an `bws add <name>` verb. Split them **in the type system**, not just the catalog: add a required `kind: "hardening" | "capability"` to `Profile`, refuse embedded `capability` profiles, refuse user profiles declaring `mask`/`unshare_net`/`no_net` unless `kind: hardening`. Today `bws add no-gh` and `bws add my-rust-env` are syntactically identical with opposite security meaning — a footgun that survives the pivot unless closed deliberately.

### 1.2 Blocking flaw: `execve` is never traced

`internal/learn/trace.go:44` traces `open,openat,creat,unlink,…,stat,lstat,newfstatat,statx,access,…` — **no `execve`/`execveat`**, and `populateParsedSyscall` (`parser.go:120-168`) has no case for it. PATH discovery is limited to `ResolveBinaryDir(opts.Command[0], …)` — the one command the user typed.

This is fatal for the pivot as specified. `bws learn ./tools/gate.sh` resolves `./tools/gate.sh` and nothing else. Every tool the script invokes is invisible: `execve` is kernel-side, and the loader opens *libraries*, not the main executable. Worse, statically linked binaries — essentially the whole Go toolchain and anything Go-built — generate no library `openat` calls either, so a Go linter in `~/go/bin` leaves no trace evidence at all.

Fix: add `execve,execveat`, add a parser case extracting the path argument, feed each resolved executable's directory through `ResolveBinaryDir` normalisation into `delta.Path`. ~30 lines, and it's the difference between the pivot working and not.

### 1.3 Blocking flaw: read-side secret filter is a short denylist

`ShouldFilterAccess` (`filter.go:158`) consults `isSensitiveSecretStore`, whose list (`filter.go:47-57`) is `~/.aws ~/.azure ~/.config/gcloud ~/.password-store ~/.vault-token ~/.gnupg ~/.config/auth ~/.auth ~/.netrc`.

Not on it: `~/.ssh`, `~/.config/gh`, `~/.kube`, `~/.docker`, `~/.npmrc`, `~/.pypirc`, `~/.cargo/credentials.toml`, `~/.gradle/gradle.properties`, `~/.m2/settings.xml`, `~/.terraform.d`, `~/.config/op`, `~/.local/share/keyrings`, `~/.mozilla`, `~/.config/google-chrome`, `~/.thunderbird`.

Note the asymmetry: `IsSensitivePath` (`security.go:14-32`) **does** include `~/.ssh` — but it's only consulted by `FilterSensitiveWrites`, the *write* path. Reads are what leak credentials. Trace a gate script that does `git push` over SSH: `CollapsePath` collapses `~/.ssh/id_ed25519` → `~/.ssh` (dot-dir rule, `collapse.go:52-58`), `ComputeDelta` proposes `binds_ro: ["~/.ssh"]`, `ApplyDelta` writes it — and every subsequent `bws` run in that project mounts the user's private keys. `~/.config/gh` and `~/.git-credentials` land the same way and are saved only by the independent forge-mask layer; `~/.ssh`, `~/.kube/config`, `~/.docker/config.json`, `~/.npmrc` are saved by nothing.

Two things make this worse than the leak it replaces. It's **data-dependent** — appears only on machines where the gate script touches the secret, so it never shows in code review or CI. And it's **self-authorising** — the tool writes the mount and prints `✓ Updated local configuration`, so nobody inspects it. A denylist can't fix this; §4.3 gives the structural fix.

### 1.4 `bws learn` is an open loop

`RunTrace` shells `strace` on the host. Nothing re-runs the workflow *inside* the sandbox to confirm sufficiency. The output is a hypothesis presented as a result.

Meanwhile the loop-closing machinery already exists: `RunProfileTests` (`internal/profile/runner.go:27`) builds a config, calls `BuildArgs`, and executes `TestSpec` commands inside the sandbox. Your complexity item 3 ("…runner smoke tests adds hundreds of lines of non-core logic") would **delete the verifier along with the catalog**. The runner is the only component in the codebase that can tell you whether a sandbox config is correct. Keep `TestSpec` + `RunProfileTests`; delete the catalog.

The DAG must stay too: `profiles/ai.json` requires `no-sudo`, `profiles/secure-agent.json` composes eight hardening profiles. Composition is a property of the *hardening* layer. `ResolveProfile` is ~90 lines and earns them.

### 1.5 Remaining failure modes, ranked

**a. Warm-cache under-learning (high/high).** Tracing sees what a process *does*, and a warm cache means it does almost nothing. `go test ./...` against a populated `~/.cache/go-build` never opens `$GOROOT/src`; `cargo test` with a warm `target/` never touches `~/.cargo/registry/src`. The config then works on the learning machine and fails on a cold checkout. Mitigate with `bws learn --cold` (relocate `GOCACHE`/`CARGO_HOME`/`npm_config_cache` to throwaway dirs) and by having static queries supply toolchain roots unconditionally rather than relying on observation.

**b. Committed configs are host-specific (high/medium).** `ApplyDelta` writes host-resolved absolute paths — `NormalizePath` only rewrites `$HOME` to `~`, so `/opt/quarto-1.4.5`, `/usr/lib/jvm/java-21-openjdk-amd64`, `/nix/store/…` go in verbatim, into a committed file every contributor shares. Divergence hasn't been eliminated, just moved from a file the maintainer curates to one nobody audits. Fix: split — `.bws/config.jsonc` (committed) declares *intent*; `.bws/local.jsonc` (gitignored, merged on top) holds learned machine-specific mounts.

**c. Silent degradation instead of failure (medium/medium).** Many tools degrade quietly rather than erroring: `git` without `~/.gitconfig` commits under the wrong identity; `ssh` without `~/.ssh/config` ignores host aliases. Neither host tracing nor failure tracing catches these — and no pre-baked profile did either. Genuinely unsolved; handle with a small curated list of *identity* files opted into once globally, not by any learning mechanism.

**d. Cold start (certain/low).** A repo with no `tools/gate.sh` has nothing to learn from, and `bws add go-dev` was discoverable in a way `bws learn ./tools/gate.sh` isn't. Fix: have `bws init-dev` *generate* a `tools/env.sh` stub seeded by the existing `DetectProfiles` heuristics (`internal/profile/detect.go:9`) — repurpose detection from "which profile" to "which toolchain commands should the harness exercise".

---

## 2. Solving coverage without bloated profiles

### 2.1 Abandon the guarantee; guarantee recovery

You can't guarantee coverage of an open-ended interactive session — pre-baked profiles didn't either, they just made failure rarer and much broader in privilege. Replace the goal explicitly:

> Any missing-tool roadblock is diagnosable and repairable by the developer or agent in a single command, in under ten seconds, without a `bws` release.

### 2.2 Three layers, cheapest first

**Layer A — the RO system root (already shipped, zero cost).** `gopls`, `rust-analyzer`, `dlv`, `clangd`, `ruff`, zoneinfo, MIME db, CA bundles: all covered by `binds_ro: /usr` plus the cert binds in `addStandardMounts` (`bwrap.go:258-266`). Document this loudly — it removes most of Challenge 1's stated risk. The gap is only *user-local* installs.

**Layer B — static toolchain queries. Make this primary, not a fast path** — it returns *roots* not observed leaves, so it's immune to warm-cache under-learning:

| Toolchain | Query | Yields |
| :--- | :--- | :--- |
| Go | `go env GOROOT GOPATH GOCACHE GOMODCACHE GOBIN` | RO GOROOT; RW GOCACHE/GOMODCACHE/GOPATH; PATH GOBIN |
| Rust | `rustc --print sysroot`, `$CARGO_HOME` | RO sysroot (covers rust-analyzer, rustfmt, clippy, std src); RW registry/git; PATH `~/.cargo/bin` |
| Python | `sys.prefix`, `sysconfig.get_paths()`, `site.getusersitepackages()`, `$VIRTUAL_ENV` | RO prefix + site dirs; RW pip/uv caches |
| Node | `node -p process.execPath`, `npm config get prefix cache` | RO prefix; RW cache; PATH `$prefix/bin` |
| Java | `java -XshowSettings:properties -version` → `java.home` | RO java.home; RW `~/.gradle`, `~/.m2/repository` (**not** `settings.xml`) |
| TeX | `kpsewhich -var-value=TEXMFDIST TEXMFVAR TEXMFHOME` | RO TEXMFDIST; RW TEXMFVAR |
| Nix | `nix-store -qR $(readlink -f $(command -v <tool>))` | RO closure roots |

This is the honest answer to "how do you avoid bloated profiles": you don't delete the toolchain knowledge, you move it from **data** (paths, which diverge per distro) to **code** (queries, which don't). ~200 lines total, deletes 30 JSON files.

**Layer C — tracing, as fallback** for bespoke/vendored tools and unusual data files.

### 2.3 Make the roadblock self-diagnosing

This matters more for AI agents than any coverage percentage. An agent that gets `bash: gopls: command not found` improvises — installs into the sandbox, works around it, or gives up. An agent that gets:

```
bws: 'gopls' not found in sandbox but present on host at ~/go/bin/gopls
     fix: bws mount --ro ~/go/bin   (or: bws learn -- gopls version)
```

repairs itself in one turn. Add `bws doctor <cmd>`, and install a `command_not_found_handle` in the skeleton `bashrc` (`internal/sandbox/embedded_skeleton/bashrc`) that calls it. That single hook converts the pivot's dominant failure mode from a hard stop into a one-line fix.

### 2.4 Verify with the runner you already have

Add `bws verify [script]`: resolve config, run the gate script in-sandbox via the `RunProfileTests` path, report pass/fail. Then `bws learn` becomes a convergence loop rather than a one-shot guess, and CI can catch "works on the machine that learned it".

---

## 3. Incremental delta learning

`ComputeDelta` (`diff.go:21`) is already the right shape: canonicalise both sides, treat a discovered path as covered if it's a subpath of an existing mount, detect RO→RW upgrades, diff features separately, never rewrite wholesale. With append-only `ApplyDelta`, custom mounts are structurally safe. Challenge 2 is largely solved. Four defects:

### 3.1 The delta is blind to implicit mounts — highest-leverage fix

`ComputeDelta` diffs only against `targetConfig.BindsRW/BindsRO/Path/Features`. But `BuildArgs` adds mounts appearing in no config file: `/opt` + symlink target (`addOptBind`), a Quarto root inferred from PATH (`addQuartoBind`), `/run/systemd/resolve`, `/opt/google`, `/etc/ssl`, `/etc/ca-certificates`, `/usr/share/ca-certificates` (`bwrap.go:258-266`), all of `/etc` entry-by-entry under `enable_etc_auto_bind`, the staged `$HOME`, `/tmp`, `--proc`, `--dev`, `/sys`, and the SSH pinholes `~/.ssh/config`/`known_hosts` (`helpers.go:51-55`).

Anything under these is already reachable, yet the delta proposes it, `ApplyDelta` writes it, and it sits there forever widening the *audited* surface without widening the real one. Worse, `filter.go:12-27` maintains a **second hardcoded copy** of this knowledge (`defaultFilteredPrefixes`) that must be hand-synced with the default template — and it already diverges: `/opt` is auto-bound but absent from `defaultFilteredPrefixes`.

Fix: compute the **effective mount set** by calling `bwrap.BuildArgs(cfg, …, dryRun=true, …)` and parsing the `--bind`/`--ro-bind`/`--ro-bind-try`/`--bind-try` triples, then diff against that. `BuildArgs` is already pure and dry-run-safe — `runner.go:223` uses it exactly this way for `bws info`. One source of truth, and `defaultFilteredPrefixes` shrinks to genuinely ephemeral paths.

### 3.2 Add-only, so privilege monotonically increases

No prune path. Remove SQLite from your gate script and its mounts stay forever. Over a year a config accumulates every tool it ever used — the same least-privilege rot, relocated from the shipped catalog into per-project files nobody audits. Add `bws learn --prune`, which needs §3.3.

### 3.3 No provenance

`ApplyDelta` writes bare strings (`merge.go:40`). A learned mount is indistinguishable from a deliberate one, which makes pruning unsafe and makes "never wipe custom mounts" an accident of append-only editing rather than a guarantee. With the `.bws/local.jsonc` split from §1.5b this falls out free: everything in `local.jsonc` is learned and prunable, everything in `config.jsonc` is untouchable. Stronger and simpler than marker comments, and it fixes portability simultaneously.

### 3.4 Non-atomic merge with swallowed errors

`ApplyDelta` re-reads and rewrites the file once per entry; an interrupt leaves a partial merge. And every error is discarded:

```go
if err := config.AddBindArrayElement(targetPath, "binds_rw", entry); err == nil {
    res.AddedRW++
}
```

A write failure raises nothing and `handleLiveMerge` still prints `✓ Updated local configuration`. Silent partial success on a security config is the wrong default. Fix: apply all edits to a temp copy, `rename(2)` on success, propagate the first error.

### 3.5 Recommended algorithm

1. Discover: static queries ∪ failure-driven trace (§4.3).
2. Canonicalise (`~`/`@@HOME@@`, `EvalSymlinks`, `Clean` — `CanonicalPath` does this).
3. Reject anything covered by the `BuildArgs` dry-run effective mount set (§3.1).
4. Reject anything matched by a hardening mask **and say so loudly** — "`~/.config/gh` is required by your workflow but masked by `no-gh`; drop `no-gh` or vendor the credential" is actionable; silently writing a bind that `addMaskArgs` overwrites with `/dev/null` is not.
5. Classify RW/RO, collapse to roots, prune redundant children (`CollapseAndClassify` does this).
6. Confirm per entry — default to *show and ask*, not the current auto-apply.
7. Write once, atomically, into `.bws/local.jsonc`.
8. Verify in-sandbox; loop to 1 up to a small bound.

---

## 4. Trace performance

### 4.1 Ranking your four options

**1st — probe-first.** Correct, and more important than credited: its value isn't primarily speed, it's that it makes the trace incremental and *safe by construction* (§4.3). Adopt as the architecture.

**2nd — static queries.** Promote to primary discovery (§2.2). ~10ms, no ptrace, no privileges, no secret exposure, immune to warm caches.

**3rd — filtered strace.** Necessary fallback, but the filter list isn't where the win is (§4.2). Already partly done at `trace.go:44`.

**4th — reject `LD_PRELOAD`.** Worst option for *this* project specifically:
- **Go and Rust defeat it.** The Go runtime issues syscalls directly, bypassing libc; a preloaded `openat` wrapper sees nothing from `go`, `gopls`, `dlv`, or any Go-built tool. Static/musl Rust likewise. `bws`'s primary users are Go developers — a mechanism blind to the Go toolchain isn't a mechanism.
- **Silently dropped**, not errored, under `AT_SECURE`/setuid — so `sudo` and some wrappers produce empty traces that look like clean runs.
- **Requires a per-arch native `.so`**, breaking the single-static-binary property `AGENTS.md` treats as invariant, and adding CGO plus a cross-compile matrix.
- Can't see kernel path resolution, so it misses exactly the `execve` targets §1.2 shows you need most.

The answer to "ptrace is slow" isn't "intercept in userspace" — it's "stop tracing the expensive things and stop tracing successes" (§4.2), then "stop tracing almost everything" (§4.3).

### 4.2 What actually makes strace fast

**a. `--seccomp-bpf` (strace ≥5.3) — biggest single win, one flag.** Without it every syscall traps to the tracer and back twice regardless of `-e trace=`; the filter only suppresses *printing*. With it the kernel installs a seccomp-BPF program so untraced syscalls never stop the tracee. For file+exec-only tracing of a build this typically moves 10–50× into the low single digits. Caveats: needs `-f` (fine), incompatible with `-b`, and behaviour changes if the tracee installs its own filter. Probe once and cache.

**b. `-e status=successful` (strace ≥5.2).** `AnalyzeTraceLines` already discards every `!parsed.Success` line (`trace.go:130`) — but only after the kernel formatted it, wrote it, and Go re-parsed it. On `cargo build` the failed-`stat` volume dominates. Filtering in the tracer is free. (§4.3 wants the opposite filter, `-e status=failed`, for the same reason.)

**c. Drop the `stat` family from the default set.** `stat,lstat,newfstatat,statx,access,faccessat,faccessat2` are the highest-volume syscalls in any build and contribute nearly nothing: a path stat'd *and* opened is already captured by `open*`; a path stat'd but never opened generally doesn't need mounting. Keeping them is why the tracer is slow even with a syscall filter. Move behind `--paranoid`.

**d. `-qq -e signal=none`**, and reconsider `-s 1024`: any path longer than the limit is **silently truncated**, yielding a plausible wrong mount rather than an error. Truncated strings lack a closing quote and `extractQuotedStrings` (`parser.go:189`) will happily return the fragment. Detect the truncation marker and discard.

**e. Degrade gracefully when ptrace is unavailable.** `kernel.yama.ptrace_scope=3` disables ptrace outright, and hardened/container environments commonly do. `RunTrace` only checks `CommandExists("strace")` and otherwise fails with a raw strace error. Probe ptrace and fall back to static queries with a clear message rather than presenting the pivot's core workflow as broken.

### 4.3 The recommendation that dissolves all three challenges

**Trace *failures*, *inside the sandbox*, instead of *successes* on the *host*.**

Today: run `strace` on the host with the user's full environment reachable, collect every successful access, subtract what the config covers. Invert it: run the gate script *inside* the sandbox under `strace -e status=failed`, collect paths returning `ENOENT`/`EACCES`/`EPERM` **that exist on the host**. Those, and only those, are the delta.

- **Challenge 3 dissolves:** the event set shrinks from "every file the build touches" (hundreds of thousands) to "paths that were denied" (tens). With `--seccomp-bpf` + `-e status=failed` the tracer has almost nothing to write. And in the common case — config already sufficient — the probe runs untraced at exactly zero overhead. Probe-first taken to its conclusion.
- **Challenge 2 dissolves:** a failure-driven trace can only surface paths the sandbox doesn't already provide, so "discovered minus existing" is computed by the kernel rather than by `ComputeDelta`'s subpath arithmetic. §3.1's implicit-mount blindness goes with it — an implicitly bound path doesn't fail, so it never becomes a candidate.
- **§1.3 dissolves — the important one.** A process inside the sandbox **cannot read a secret that isn't mounted**. The discovery set is bounded by the sandbox boundary, not by the completeness of a hand-maintained denylist. `~/.ssh/id_ed25519` can't appear as a successful read; it can only appear as an `ENOENT`, at which point `bws` asks "your workflow tried to read `~/.ssh/id_ed25519` — mount it?" and the human decides. The denylist stops being a security control and becomes a UX hint about which prompts to pre-answer "no".

Two honest limits. **It iterates** — each run reveals the next missing path, so a cold start may need several passes; bound the loop (~8) and seed iteration 1 from the static queries so common toolchains are satisfied before it begins (converges in one or two passes in practice). **It can't see silent degradation** (§1.5c); host-side full tracing does, so keep it as opt-in `bws learn --from-host` behind a clear warning, with the denylist retained for that path only.

---

## 5. Prioritised recommendations

**P0 — before shipping the pivot**
1. **Trace `execve`/`execveat`** (§1.2). ~30 lines. Nothing else works until this is done.
2. **Move learning inside the sandbox, drive it from failures** (§4.3). Architecture, not optimisation. Fixes the leak class structurally.
3. **Stop auto-applying by default.** Invert `handleDeltaDispatch` (`learn_cmd.go:66`): preview by default, `--apply`/`--yes` to write.
4. **Make `ApplyDelta` atomic and error-propagating** (§3.4).
5. **Extend the read-side denylist as a stopgap** for `--from-host` — and better, have `isSensitiveSecretStore` and `IsSensitivePath` share one list, since the split is what let the read path diverge.

**P1 — makes the pivot pleasant**
6. **`bws probe` / static toolchain queries** as primary discovery (§2.2). This is what replaces the 30 JSON files.
7. **`bws verify [script]`** on the existing runner; keep `TestSpec` and `RunProfileTests` (§1.4).
8. **Split config**: committed `config.jsonc` + gitignored `local.jsonc` (§1.5b, §3.3). Fixes portability and provenance in one change.
9. **`command_not_found_handle` → `bws doctor`** in the skeleton (§2.3). Highest value per line for agent workflows.
10. **`--seccomp-bpf`, `-e status=`, drop the `stat` family, handle truncated paths, degrade without ptrace** (§4.2).

**P2 — don't repeat the old mistake**
11. **`kind: hardening | capability`** on `Profile`, enforced (§1.1).
12. **Refuse to learn a mount a hardening mask covers**, loudly (§3.5 step 4). Add a regression test asserting `addMaskArgs` runs after `buildBinds` in `BuildArgs` — that ordering is the last line of defence and nothing currently pins it.
13. **`bws learn --prune`** so learned privilege can decrease (§3.2).
14. **Fix `profiles/git.json`**: drop the `~/.git-credentials` bind. It's neutralised only because `BlockGH` defaults true, and it's the concrete instance of the leak class motivating this pivot. That line shouldn't exist either way.
15. **Repurpose `DetectProfiles`** into gate-script scaffolding in `bws init-dev` (§1.5d), preserving cold-start discoverability.

**Do not do**
- Don't build the `LD_PRELOAD` interceptor (§4.1) — blind to Go and static binaries, silently disabled under `AT_SECURE`, breaks single-binary distribution.
- Don't delete `ResolveProfile`, `TestSpec`, or `RunProfileTests`. The DAG is required by `secure-agent` and `ai`; the runner is your only verifier.
- Don't keep `defaultFilteredPrefixes` as a parallel model of what's already mounted. Derive it from `BuildArgs` (§3.1).
