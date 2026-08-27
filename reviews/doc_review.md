# Documentation review — `bws` (Bubblewrap Sandbox)

*Reviewer: Claude Sonnet 4.6 (Thinking) · Date: 2026-08-27*

---

## Executive DevEx scorecard

| Dimension | Score | Justification |
| :--- | :---: | :--- |
| **Clarity & Readability** | 8/10 | Prose is terse and technical without being impenetrable. A few sections remain cluttered or use weak passive phrasing. |
| **First-Run Usability** | 7/10 | The `init-dev` → `bws` reordering was the right call. Still missing: no mention of what `bws` outputs on first run, and no recovery guidance when things go wrong. |
| **Technical Precision** | 8/10 | Namespace primitives, masking mechanics, and deploy key architecture are described accurately. Minor imprecisions remain (see file-by-file notes). |
| **Information Architecture** | 9/10 | The split into `docs/` is well-structured and navigable. `README.md` is appropriately thin. TOCs are present throughout. |

**Overall: 8/10.** Solid documentation for a 0.2.x project. Main remaining gaps: missing failure recovery guidance, a few stale phrases, and overly terse `docs/commands.md`.

---

## Top high-impact recommendations

### 1. Add a `## Troubleshooting` section to `README.md`

There is currently zero recovery guidance when things go wrong. The most common failure modes — `bwrap` not found, user namespaces disabled, `gh` not authenticated, SSH remote in HTTPS format — are undocumented.

### 2. `docs/commands.md` needs flag documentation

Every command entry shows only usage examples with no explanation of flags, semantics, or defaults. `-f`, `-v`, `-n`, `-g`, `-l`, `-r`, and `-p` all appear without any definition. This file has the shape of a reference but not the content.

### 3. `docs/faq.md` line 55 — stale `.bws.jsonc` reference remains

Line 55 still reads: `The local .bws/ configuration directory and .bws.jsonc file are overlaid`. The `.bws.jsonc` reference contradicts the canonical `.bws/config.jsonc` naming used everywhere else.

### 4. `docs/security.md` — network isolation is undocumented

The threat model lists four guarantees but omits a critical one for AI agent developers: **network isolation is NOT provided**. Sandboxed code can make arbitrary outbound HTTP/HTTPS connections. This must be stated explicitly.

### 5. `docs/architecture.md` line 154 — contradicts the SSH agent isolation fix

After the security audit that removed host `SSH_AUTH_SOCK` inheritance, line 154 still reads: *"Auto-detects `SSH_AUTH_SOCK` or launches a persistent agent."* This is now factually incorrect.

---

## File-by-file surgical line edits

### `README.md`

**Line 7** — "fast" and "orchestrator" are informal claims. Suggested fix:
```diff
-**`bws`** is a fast, declarative, unprivileged Linux sandbox launcher and orchestrator built on top of
+**`bws`** is a declarative, unprivileged Linux sandbox launcher built on top of
```

**Line 9** — The callout block repeats information already in line 7. Consider removing it and folding the relevant detail into the opening paragraph.

**Lines 25–32** — Key capabilities list uses inconsistent verb forms. Normalise all bullets to third-person verb form:
```diff
-* **Declarative capability profiles**: Compose dev stacks and toolchains with a single line
+* **Declarative capability profiles**: Composes dev stacks and toolchains with a single line
```

**Line 32** — "instantly" crept back in after the superlatives pass. Remove.

**Line 121** — "7,000+ open-source tools" is a Homebrew-sourced number that may drift. Soften to "thousands of open-source tools" or link directly to the Homebrew formula count.

**Lines 163–166** — Step 2 of the Deploy Key section mentions `~/.sandbox/deploy_keys/` as the key location but omits that this directory is itself masked inside the sandbox — an important security nuance.

**Line 173** — "Zero-trust security guarantee" was in the superlatives-to-remove list. The replacement used elsewhere is "Security boundary". Pick one and use it consistently.

---

### `docs/faq.md`

**Line 32** — "instantly" is still present:
```diff
-* **Zero daemon overhead**: `bws` runs instantly as a lightweight, unprivileged user process.
+* **Zero daemon overhead**: `bws` runs as an unprivileged user process with no background daemon.
```

**Line 55** — Stale `.bws.jsonc` reference:
```diff
-* **Auto-masked `.bws/`**: The local `.bws/` configuration directory and `.bws.jsonc` file are overlaid with an empty `tmpfs` / `/dev/null`
+* **Auto-masked `.bws/`**: The local `.bws/` configuration directory is overlaid with an empty `tmpfs`,
```

**Line 64** — "ephemeral SSH key pair" is inaccurate. These keys persist across sessions:
```diff
-it automatically generates an isolated, ephemeral SSH key pair.
+it automatically generates an isolated, per-repository SSH keypair stored in `~/.sandbox/deploy_keys/`.
```

**Missing** — A FAQ entry: *"Does `bws` restrict outbound network access?"* (see rewrite proposals below).

---

### `docs/configuration.md`

**Line 27 and Lines 106–110** — `@@HOME@@` token docs appear in two places with some redundancy. Consolidate into one section.

**Line 109** — "dynamically" is vague. Specify *when*:
```diff
-Values in the `env` map support dynamic `$VAR` expansion:
+Values in the `env` map support shell-style `$VAR` expansion, resolved at sandbox launch time (not at config parse time):
```

**Line 42** — The `features` schema row lists sub-keys in parentheses but does not document their types or defaults. Add a follow-up note or link.

---

### `docs/security.md`

**Line 3** — "zero-trust" is overloaded marketing language:
```diff
-`bws` is designed for unprivileged, hermetic developer environments and zero-trust autonomous AI coding agent execution.
+`bws` is designed for unprivileged, hermetic developer environments and isolated autonomous AI coding agent execution.
```

**Line 51** — Stale `.bws.jsonc` reference remains:
```diff
-When `bws` launches inside a workspace, the local `.bws/` directory and `.bws.jsonc` file are **automatically masked by default**
+When `bws` launches inside a workspace, the local `.bws/` configuration directory is **automatically masked by default**
```

**Line 29** — The `--tmpfs`/`--ro-bind-try` distinction is important but the docs don't warn that using the wrong primitive for the target type will silently fail. Add a note.

**Missing** — No mention of network isolation (or its absence). Critical gap for AI agent use cases.

---

### `docs/commands.md`

This is the weakest file in the suite. Every command shows examples but no flag documentation.

**Lines 22–26** — Expand bare `bws` entry with flag table:
```diff
-bws
-bws -v    # Verbose debug output
-bws -f    # Bypass file count safety checks
+| Flag | Short | Default | Description |
+| :--- | :--- | :--- | :--- |
+| `--verbose` | `-v` | off | Print bwrap arguments and staging steps to stderr |
+| `--force` | `-f` | off | Skip the file count safety prompt |
+| `--readonly` | `-r` | off | Mount the workspace read-only inside the sandbox |
+| `--info` | | off | Dry run: print the bwrap argument plan without launching |
```

**Line 47** — `bws init-dev [options] [dir]` — the `[dir]` argument is never explained.

**Lines 125–135** — `cbind` and `ccopy` have zero explanatory text. See rewrite proposal 3 below.

**Line 102** — `bws conf info` needs a note that the dry-run output shows `<ephemeral staged home>` as a placeholder, not the actual runtime path.

---

### `docs/architecture.md`

**Line 3** — Broaden the target audience:
```diff
-This document provides a deep technical reference for developers, security engineers, and contributors
+This document is a technical reference for developers, security engineers, contributors, and anyone evaluating `bws`'s isolation guarantees.
```

**Line 147** — "seamless" is a superlative that survived the sanitisation pass:
```diff
-2. **Seamless builds**: Absolute paths in compiler outputs, stack traces, and debuggers match host paths exactly.
+2. **Exact-path builds**: Absolute paths in compiler outputs, stack traces, and debuggers match host paths exactly.
```

**Line 154** — Factually wrong after the SSH agent isolation fix:
```diff
-* **SSH agent & deploy keys**: Auto-detects `SSH_AUTH_SOCK` or launches a persistent agent.
+* **SSH agent & deploy keys**: Always launches a dedicated isolated `ssh-agent` (never inheriting the host `SSH_AUTH_SOCK`). When operating in a GitHub repository, provisions a per-repository deploy key via `gh`.
```

---

### `profiles/README.md`

**Line 100** — The Go compilation verification test is an unreadable one-liner. Reformat:
```diff
-`bash -c echo "package main; import (\"fmt\"); func main(){ fmt.Println(\"Go OK\") }" > /tmp/bw_hello.go && go run /tmp/bw_hello.go && rm -f /tmp/bw_hello.go`
+`bash -c 'printf "package main\nimport \"fmt\"\nfunc main() { fmt.Println(\"Go OK\") }\n" > /tmp/bw_hello.go && go run /tmp/bw_hello.go && rm -f /tmp/bw_hello.go'`
```

**General** — Security/hardening profiles (`no-sudo`, `no-ssh`, etc.) are not included in the overview table. A 2-row summary in the Overview section would help readers who skim for the security story.

---

## Concrete rewrite proposals

### Proposal 1: `README.md` — `## Troubleshooting` section

Insert between `## Automatic GitHub deploy keys` and `## Documentation guide`:

```markdown
## Troubleshooting

**`bws: command not found` after installation**  
Ensure `~/bin` is in your `$PATH`. Add `export PATH="$HOME/bin:$PATH"` to your `.bashrc` or `.zshrc` and restart your shell.

**`bwrap: command not found`**  
Install `bubblewrap` via your package manager. See [Prerequisites](#prerequisites).

**User namespaces disabled**  
Some systems disable unprivileged user namespaces by default:
```bash
# Check the current setting
sysctl kernel.unprivileged_userns_clone

# Enable (requires root)
sudo sysctl -w kernel.unprivileged_userns_clone=1
```

**`go build` (or other compiler) fails inside the sandbox**  
Run `bws init-dev` first to generate `.bws/config.jsonc` and map the required language caches. Then re-enter with `bws`.

**GitHub deploy keys not working**  
- Verify your remote uses SSH: `git remote get-url origin` should start with `git@github.com:`
- Fix if needed: `git remote set-url origin git@github.com:owner/repo.git`
- Verify `gh` is authenticated: `gh auth status`
```

---

### Proposal 2: `docs/faq.md` — Add network isolation entry

Add as a new FAQ entry after "Can code or agents inside the sandbox escape?":

```markdown
## Does `bws` restrict outbound network access?

**No.** `bws` does not provide network namespace isolation by default. Sandboxed processes can make arbitrary outbound TCP/UDP connections.

If you need network isolation, options include:
1. Wrapping `bws` inside a network namespace (`unshare --net bws`).
2. Applying host-level firewall rules (`nftables`, `iptables`) scoped to the sandbox process group.

This is an intentional trade-off: developer tools — package managers, LSP servers, API clients — require internet access to function. Network isolation would break them.
```

---

### Proposal 3: `docs/commands.md` — Explain `cbind` and `ccopy`

Replace the two sparse entries with:

```markdown
### `bws cbind add <host-path> [dest] [-g | -l]`
Add a persistent read-write bind mount to your configuration.

The mount is added to `binds_rw` in your global (`-g`) or local project (`-l`) config and will be active on every subsequent `bws` invocation in that scope.

| Flag | Description |
| :--- | :--- |
| `-g` | Add to global config (`~/.config/bws/config.jsonc`) |
| `-l` | Add to local project config (`.bws/config.jsonc`) |

```bash
bws cbind add /opt/tools -g          # Bind /opt/tools globally
bws cbind add /data/fixtures -l      # Bind /data/fixtures for this project only
```

---

### `bws ccopy add <host-path> [-g | -l]`
Add a file to be copied into the staged ephemeral `$HOME` at sandbox launch.

Unlike a bind mount, a copied file appears as a regular file in the sandbox home rather than a host symlink. Use this for files (e.g. scripts, configs) that should not reveal their host location to sandbox processes.

```bash
bws ccopy add ~/bin/helper -g        # Copy ~/bin/helper into sandbox $HOME/bin/helper
```
```

---

*End of review — 2026-08-27*
