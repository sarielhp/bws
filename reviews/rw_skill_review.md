I couldn't write `reviews/rw_skill_review.md` (write permission wasn't granted), so the review is below — say the word and I'll write it to the file.

# Review: the `/rw` disposable agent sandbox skill

Reviewed against `bws` v0.3.37 (`a3e98e2`). Everything is anchored to source. The headline: the skill is written against an *idealized* description of `bws gw` rather than the implementation. One failure-detection rule is inverted, the branch-naming rule opts the skill out of `bws gw list`/`prune`, and the quoting advice is wrong for both bash and your fish shell. Separately `bws gw` has four defects a background dispatch will hit routinely, and Part 2's security description overstates the sandbox.

## 1. Corrections to the Part 2 capability claims

| Claim | Reality | Evidence |
|---|---|---|
| "PID namespaces (`CLONE_NEWPID`)" | `--unshare-pid` is **never emitted**. Only `ipc`, `net` (conditional), `uts` (conditional). | `internal/bwrap/bwrap.go:38,46,65` |
| "`--die-with-parent` guarantees no orphan processes" | It sets a parent-death signal on `bwrap` only. Without a PID ns, `bwrap` isn't pid 1 and doesn't reap — a child that `setsid()`s survives. | `internal/bwrap/bwrap.go:238` |
| "Hardening profiles: `no-sudo`, `no-gh`, `no-secrets`…" | They exist, but `gw` enables **none** of them. Only `--no-ssh` is hardcoded. | `internal/gitworkflow/gitworkflow.go:97` |
| "`--shared file://` → 0-copy, instantaneous" | **Likely false.** Git derives `is_local` from `get_repo_path()`, which doesn't resolve a `file://` URL, so local optimizations are skipped. | `gitworkflow.go:78` |

Repro for the last one: `git clone --shared file:///path /tmp/c1; cat /tmp/c1/.git/objects/info/alternates` — absent means `--shared` was ignored. (I was blocked from running this here.) Note the tradeoff if you switch to a plain path: real alternates make the clone fragile against a concurrent host `git gc --prune`.

## 2. Defects, ranked by likelihood of being hit

**F1 — the triage prompt spins forever on non-TTY stdin (critical).** `promptTriage` does `input, _ := reader.ReadString('\n')` and falls to `default: "Invalid choice"` in an unbounded loop. On EOF that's `("", io.EOF)` → error discarded → `default` → repeat. Unbounded output at 100% CPU. This is *exactly* the skill's dispatch mode ("run in the background"). If the harness gives a pipe rather than a pty, the run wedges instead of pausing. Secondary hazard even with a pty: `cmd.Stdin = os.Stdin` hands the same fd to the child, then `bufio.NewReader` wraps it fresh — bytes the child left buffered get eaten as triage input.

**F2 — the skill's crash-detection rule is inverted (critical, skill-side).** Step 2.1 says no-menu ⇒ crash. Backwards. The child's exit status is discarded (`if err := cmd.Run(); err != nil { // explicitly ignored }`), so a crashed child still auto-commits, fetches, and shows the menu. The menu is *suppressed* on the healthy "no changes" path and on fetch failure. So "no menu" means no-changes-or-fetch-failed, and "menu shown" says nothing about success.

**F3 — reusing a branch slug silently destroys the work (critical).** The fetch-back has no `+` force refspec:
```go
if err := runCmd(hostRepo, "git", "fetch", tempDir, fmt.Sprintf("%s:%s", branchName, branchName)); err != nil {
    fmt.Printf("Note: No new commits or branch changes detected from agent session.\n")
    return nil
}
```
Diverged same-name branch → git rejects non-fast-forward → misreported as "no new commits" → `defer os.RemoveAll(tempDir)` deletes the only copy. The skill makes this reachable *by design*: descriptive slugs + `[k] Keep` leaves the branch, and the second `/rw` on the same topic (the normal iteration loop) loses the run.

**F4 — descriptive slugs are invisible to `list`/`prune` (high).** Both hard-filter on the prefix: `list.go:80` uses `refs/heads/bws-agent-*`. Your rule 4 (`feat-interactive-search`) produces branches the tool's own lifecycle management cannot see. Fix in the skill: `-b bws-agent-<slug>`.

**F5 — `git add -A` can commit `.env` and `.bws/` into history (high).** `copyConfigFiles` copies `.bws/` and every `.env*` into the clone; post-session `bws` runs `git add -A && git commit` there, fetches it back, and offers it for merge. If the target repo doesn't gitignore those, `[s]` writes secrets into history. *This* repo is safe only because its `.gitignore` lists them. `--stash` doesn't save you — `git stash push` without `-u` leaves untracked files in place. Your own rule 3 has the same shape, and `--no-verify` is precisely what defeats the hook that would catch it.

**F6 — shared PID namespace defeats the env scrubbing (high).** `--proc /proc` (`bwrap.go:235`) with a shared PID ns enumerates **all host processes**. Same-uid `/proc/<pid>/environ` is gated by `PTRACE_MODE_READ`, which Yama does *not* restrict (Yama only gates `ATTACH`), so `ptrace_scope=1` doesn't block it. A sandboxed agent can read `ANTHROPIC_API_KEY` / `GEMINI_API_KEY` / `GH_TOKEN` straight out of the parent agent's process. Signals to host processes also work. Unverified (userns interaction needs a real check) — repro: `bws run -- sh -c 'cat /proc/<host-pid>/environ | tr "\0" "\n" | head'`. Fix is one flag: `--unshare-pid`.

**F7 — no `--new-session` (high).** Absent from the codebase. bwrap's manual is explicit that omitting it permits TIOCSTI keystroke injection into the controlling terminal and calls it a sandbox escape. It matters more here than generically, because `gw` reads a *security decision* from that same terminal right after the child exits — a child can queue `s\n` before exiting and the parent never sees the menu. Mitigated only where `dev.tty.legacy_tiocsti=0`.

**F8 — the `ai` profile grants RW access to the parent agent's own config (high).** `bws add ai -g` pulls in `agy`, `claude`, `opencode`, … and those bind agent state **read-write**: `agy.json` → `~/.gemini`, `~/.config/gcloud`; `claude.json` → `~/.claude`, `~/.claude.json`. `~/.gemini/config/skills/` is where `/rw` itself lives, and `~/.claude/settings.json` declares hooks that run arbitrary host commands. That's delayed host code execution, reached by the exact command the skill prescribes. Masks do win (`addMaskArgs` is last, `bwrap.go:305`), but `ai` pulls in only `no-sudo` — not `no-secrets` — and nothing masks `~/.gemini`/`~/.claude` at all. **`profiles/secure-agent.json` already exists** and composes `ai + no-sudo + no-ssh + no-browser + no-email + no-secrets + no-history + no-gh`. The skill should request that.

**F9 — `bws gw` silently discards every global flag (high).** `gitworkflow.Options` carries only `Branch, AllowDirty, Stash, Command, Verbose`, and the inner call is hardcoded `[]string{"run", "--no-ssh"}`. But `-N/--offline`, `--proxy`, `--dbus`, `--no-init` are *persistent* options (`main.go:31-42`), so `bws gw -N` parses fine, sets `f.noNet`, and is ignored. Contrast `bws run`, which does forward them (`commands_mod.go:234`). **This is the direct answer to your offline question: `/rw` can't support `-N` today, not because the skill omits it but because `gw` drops it.**

**F10 — failed `git stash pop` strands work silently (medium).** The deferred pop runs after triage, so a merge touching the same files conflicts; the error is swallowed and nothing is printed. The user's work sits in the stash under `bws-git-workflow-auto-stash` with no indication.

**F11 — base is a branch name, not a SHA (medium).** If the host branch advances during the run, `base..branch` shows the agent's additions *plus the inverse of your own new commits* — you review a diff that isn't what a merge produces. In detached HEAD, `getCurrentBranch` returns literal `"HEAD"`; `git merge HEAD` is a successful no-op, then `git branch -D` deletes the work.

**F12 — no timeout anywhere.** `cmd.Run()` blocks indefinitely; `--die-with-parent` only fires if the *parent* dies.

**F13 — `bws gw prune` races a live session (low).** `prune.go:87-96` deletes `/tmp/bws/agent_*`, guarding only cwd/repo-root/exec-path. Pruning from the host repo during a live session removes the clone dir; the fetch-back then fails — and per F3 that's reported as "no changes".

**F14 — `gitworkflow.Run` is entirely untested.** Tests cover only the helpers plus `list`/`prune`. F3 is a five-line table test.

## 3. Your five questions

**Q1 — should it use `--stash`? No**, and not for the obvious reason. `git clone` reads committed refs, so uncommitted host work reaches the agent under *neither* strategy. The difference is what the agent sees: `--stash` removes the WIP so the agent is blind to your in-flight changes; the checkpoint commit makes them visible. For "go finish what I started," the checkpoint is semantically correct and `--stash` is wrong — your instinct is right, the execution isn't. Fix it two ways: use `git add -u` plus enumerated new files (never `-A`), and offer `git reset --soft HEAD~1` after triage so no `wip:` commit survives.

**Q1 — offline / mounts / dynamic profiles.** All three are blocked at the tool layer. Offline: F9. Custom mounts: only via `bws mount add`, which writes config; no per-invocation flag. Dynamic profiles: **`bws run` has no `-p/--profile`** — only `bws init` does (`commands.go:29`). So `bws add ai -g` is the only mechanism, and it *permanently mutates the global config on every dispatch*; profiles accumulate across unrelated projects and are never removed, steadily widening the sandbox for every future `bws` run. Until `bws run -p` exists, use `-l` (workspace scope, copied into the clone by `copyConfigFiles`).

**Q2 — sandbox-aware or transparent? Aware.** The environment changes what *correct behavior is*: `git push`/`gh` will fail (forge binaries overlaid with `/dev/null`, SSH off) and an unaware agent burns turns diagnosing it, possibly "fixing" it by rewriting remotes; there's no human, so any question hangs the run (F1); files outside the repo are discarded; and everything left dirty is auto-committed as the deliverable, so the diff should be kept clean. Rule 2 is right but under-specified — say *where* to find constraints: read `AGENTS.md`/`CLAUDE.md` and quote the hard limits verbatim (here: 80-line functions, 1100-line files, `./tools/verify_build.sh`, version bump). Add an explicit definition-of-done and a non-goals list; vague goals are what make returned diffs unreviewable.

**Q3 — unhandled edge cases.** Beyond F1–F14: host repo with no `.bws/` → the clone has none → inner `bws run` reaches auto-init, which is TTY-guarded (`internal/cli/auto_init.go:82`) so it won't hang but *silently skips*, leaving the agent with no profiles and `agy` not on `PATH` — a failure that looks nothing like its cause. Also: submodules (no `--recurse-submodules`), Git LFS (pointers only), signed commits (auto-commit inherits `commit.gpgsign`, can block on a passphrase with no tty), and repo-local pre-commit hooks (the auto-commit does *not* pass `--no-verify`, so a hook can veto and lose the work).

**Q4 — quoting. The current advice is wrong twice.** `'\x27'` is not an escape in any POSIX shell — inside single quotes `\` is literal, so you get the six characters `\x27`. The POSIX idiom is `'\''`. And your shell is fish, which *does* honor `\'` inside single quotes — so an escape correct for fish breaks under bash and vice versa. Shell-specific advice doesn't belong in a skill a harness may run through either.

Stop putting prose on a command line. Ranked:
1. **Add `bws gw --goal-file <path>`** — `gw` already copies files into the clone; one more landing at a known path plus a `.git/info/exclude` entry is ~15 lines and kills the problem class.
2. **Base64 today.** Its alphabet (`A–Za–z0–9+/=`) needs no quoting in bash *or* fish:
   ```bash
   bws gw -b bws-agent-<slug> -- \
     sh -c 'agy-run-wild --effort high -p "/goal $(printf %s <B64> | base64 -d)"'
   ```
   The only literal quotes are in the fixed wrapper the parent never varies.
3. `AGY_GOAL` env var — `agy.json` already allowlists `AGY_*` in `pass_env`, so it crosses the boundary with no `bws` change, but setting it still needs quoting. Worse than (2) alone.

Not a heredoc: the parent's tool layer may not run a full shell, and heredocs interact badly with backgrounding.

**Q5 — prioritized recommendations.**

*Blocking `bws` fixes (the skill can't work around these):*
1. F1 — break `promptTriage` on `io.EOF` and after N invalid reads.
2. F3 — force the refspec or rename on collision; never report fetch failure as "no changes", never `RemoveAll` on that path.
3. F9 — thread persistent flags into the inner `bws run`; at minimum reject `bws gw -N` loudly.
4. F6/F7 — add `--unshare-pid` and `--new-session`. Two flags.
5. F2 — propagate and print the child's exit status.

*High-value `bws` fixes:* skip `.env*`/`.bws/` in the post-session commit and pass `--no-verify` to it (F5); report stash-pop failure (F10); capture base as a SHA and refuse detached HEAD (F11); add `--goal-file` and `--timeout`; add `bws run -p`; add `bws gw --triage <squash|merge|keep|discard|none>` for non-interactive automation; table-test the clone→fetch→triage path.

*Skill-only fixes (no tool changes):* prefix `bws-agent-<slug>` (F4); delete the inverted crash rule and replace with a real signal table (F2); `secure-agent -l` instead of `ai -g` (F8); `git add -u` + enumerated files instead of `git add -A --no-verify` (F5); base64 dispatch (F4/Q4); `git diff $(git merge-base HEAD <branch>)..<branch>` (F11); the sandbox-awareness preamble (Q2); preflight checks for `.bws/config.jsonc`, detached HEAD, and branch-name collision.

## The three highest-leverage changes

Skill side: prefix branches `bws-agent-`, fix the inverted crash heuristic, switch `ai -g` → `secure-agent -l`.
Tool side: bound the triage loop on EOF, force the fetch refspec, add `--unshare-pid --new-session`.

---

I have a full rewritten `SKILL.md` drafted (preflight block, signal table, base64 dispatch, sandbox preamble, cleanup section) — want me to write that plus this review to `reviews/rw_skill_review.md`?
