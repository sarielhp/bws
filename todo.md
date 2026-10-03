# Prioritized backlog

Reviewed against v0.3.68 on 2026-10-03. Items are ordered by usefulness and
importance within each tier, not by implementation size. This is a backlog,
not a commitment to implement every proposal. Investigation items are not
claims of confirmed vulnerabilities; any confirmed boundary escape becomes
an immediate release blocker.

This is the single roadmap. It subsumes the older `to_do.md`, whose unique
concrete items are preserved under "Concrete follow-ups carried from the
original roadmap". Compound profile save, compose, suggestions, interactive
initialization, dependency review, and the September 15 boundary fixes are
already implemented and are not new tasks.

## Priority 1: Safety and dependable daily use

- [ ] **Make learning safe and explicit about execution.** `bws learn` currently
  executes the target on the host, including with `--dry-run`. Warn prominently
  before execution and require explicit host-execution acknowledgment in scripts.
  Investigate sandbox-first tracing with host-side review of proposed grants;
  never treat a failed access as authorization to expose that path. Separate
  “do not save” from “do not execute,” and test both. Do not adopt the older
  learning blueprint's automatic permission expansion or extra config layer
  without a separate design decision.

- [ ] **Make effective access understandable before approval.** Extend the
  existing plan and save previews with consistent source attribution, including
  the global baseline, dependency chain, local overrides, and invocation flags.
  Highlight writable host paths, shared caches, forwarded sockets, network,
  SSH, X11, D-Bus, and WSL access. Explain that read-only access can disclose
  secrets and that a compound profile is not a permission ceiling. Redact
  literal secrets in human-readable output.

- [ ] **Review profile changes without replacing unrelated project settings.**
  Add a focused project-selection review/update operation. Show added and removed
  effective permissions and update approval fingerprints only after acceptance;
  preserve mounts, environment, comments, and other local customizations.
  Currently the documented reinitialization route uses `init --force`.
  Retain trust checks, stale-preview detection, and atomic writes.

- [ ] **Unify launch preparation and cleanup.** Share configuration-to-launch
  preparation across normal runs, command tests, and profile tests. Keep policy
  calculation separate from staging and service startup; previews must perform
  neither. Test equivalent permissions across entry points, startup failures,
  child exit codes, signals, and exactly-once cleanup. Keep actual Bubblewrap
  execution at the executable boundary, using an injected executor as needed.

- [ ] **Require security integration coverage in automated checks.** Add a
  supported Linux CI environment that can actually run Bubblewrap, alongside
  unit tests, vet, race checks, and line audits. Include malicious Git metadata,
  trust changes, symlink/mount boundaries, dry-run side effects, dependency
  drift, and concurrent/stale writes. Report skips explicitly; a skipped
  security suite must not count as a verified sandbox boundary.

- [ ] **Audit credential and host-service authority.** Verify SSH-agent ownership
  and reuse, deploy-key provisioning, socket mounts, desktop integrations, and
  WSL interop. Add automated SSH lifecycle coverage. Clearly distinguish
  forwarding an existing identity from creating a repository-scoped key;
  make remote key creation and write access explicit and document revocation.
  Check that requested restrictions cannot silently fall back to broader access.

- [ ] **Correct security and architecture documentation.** Reconcile claims
  about home visibility, SSH behavior, masks, cleanup, and workspace discovery
  with implementation and tests. Remove absolute isolation claims and clarify
  that hiding a binary is not an authorization boundary. Resolve the conflict
  between AGENTS.md's array-replacement wording and the existing merge behavior;
  document current behavior without silently migrating user configuration.

## Priority 2: Usability and reusable environments

- [ ] **Rewrite the README around autonomous AI development.** Highest-priority
  usability/documentation improvement: the current introduction describes the
  mechanisms but undersells why someone would use bws. Lead with giving an AI
  agent room to edit, build, test, and experiment inside a deliberate access
  boundary instead of exposing the ordinary host environment. Show a concrete
  end-to-end session: choose an agent/tool compound, inspect permissions, run
  in a disposable Git clone, review the changes, then keep or discard the work.
  Explain how saved compounds make that setup reusable across projects. Convey
  the appeal of “letting AI run wild” inside the selected environment without
  promising unrestricted execution is harmless: writable files can be damaged,
  granted credentials and network remain usable, and agent approval settings
  are separate from bws isolation. Use a short example or terminal recording,
  plain language, and factual benefits rather than a feature inventory or
  security superlatives. Keep README under 200 lines with the five-step quick
  start, and link technical details and limitations to the deeper guides.

- [ ] **Improve actionable failure messages.** Distinguish missing host tools,
  inaccessible mounts, untrusted configuration, changed dependencies, profile
  conflicts, and unsupported user namespaces. Name the relevant file/profile
  and give a safe next command. Do not suggest broad writable mounts or bypassing
  trust as the default remedy.

- [ ] **Improve suggestion selection and explanations.** Show why candidates
  match, their source, included profiles, and permission differences. Support
  straightforward selection of multiple profiles and explain no-match cases.
  Keep ties explicit, scanning bounded, and project inspection non-executing.
  Never infer permission to activate an agent or credentials from filenames.

- [ ] **Make saving and composition easier to revise.** Show a before/after diff
  when replacing a saved profile, explain reference versus flattening trade-offs,
  and make omitted settings and machine-specific paths prominent. Provide a
  documented edit/rename/remove lifecycle with dependency checks and explicit
  overwrite confirmation. Do not capture credentials or previous process state.

- [ ] **Add profile validation and portability checks.** Validate supported keys,
  dependency closure, conflicts, detection rules, and path tokens without
  launching tools. Warn about unknown configuration keys before introducing
  stricter rejection. Explain host prerequisites and global-baseline differences
  when moving a compound to another machine; do not imply toolchain pinning.

- [ ] **Review imported and generated profiles before installation.** Extend
  fetch/update/generation with source information, permission diffs, destination
  checks, and explicit replacement approval. Reuse trust and atomic-write
  mechanisms. Audit response size limits and failure handling; remote profile
  metadata must never execute commands during discovery or preview.

- [ ] **Make machine-readable output consistent.** Extend existing suggestion
  JSON to plan, validation, diagnostics, and review where useful. Define stable
  fields and exit statuses, send diagnostics to stderr, and keep stdout parseable.
  Test redirected stdin and noninteractive use; `--yes` must never resolve an
  ambiguous selection or bypass trust and portability requirements.

- [ ] **Improve doctor as a troubleshooting entry point.** Add effective-policy
  context, compound dependency status, missing mount/tool explanations, and
  readable versus writable access checks. Distinguish checks that only inspect
  state from active probes. Offer suggested repairs, not unrequested mutations.

- [ ] **Make configuration changes recoverable.** Review mutating commands for
  consistent preview, atomic writes, stale-content checks, and trust handling.
  Preserve a recoverable previous version where appropriate, including repeated
  forced initialization, and document a restore procedure. Test partial failures.

- [ ] **Polish help, completion, and first-run examples.** Keep canonical commands
  and aliases consistent, complete installed compound names, and cover the
  inspect → initialize → run → save → reuse path. Explain the default tmux
  session and the direct-command alternative. Test examples against the CLI.

- [ ] **Audit built-in profile grants and smoke tests.** Check tool/config/cache
  distinctions, writable paths, credential exposure, and missing-tool reporting.
  Aider, LLM, Shell-GPT, and Copilot definitions already exist: validate them
  before adding catalog entries. Do not automatically widen `ai` or
  `secure-agent` when a new assistant is added; prefer explicit combinations.

## Priority 3: Targeted maintenance and agent workflows

- [ ] **Separate setup planning from terminal interaction.** Extract an
  `internal/setup` package for init/save/compose orchestration: plan, approve,
  then apply. Keep prompts and rendering in `internal/cli`, and resolution in
  `internal/policy`. Test plans independently of terminals and commit writes
  against the exact reviewed state. Do this before substantial setup expansion.

- [ ] **Replace process exits and boolean argument chains.** Return errors and
  exit-status results from internal handlers; exit only at the executable
  boundary. Introduce typed launch/setup options and injectable input/output.
  Add command cancellation and timeouts where subprocesses can hang, preserving
  intentionally long-running interactive sessions.

- [ ] **Extract diagnostics and finish package-boundary cleanup.** Move doctor
  checks into `internal/doctor` and profile-test execution out of the profile
  resolver. Keep command registration thin. Remove obsolete initialization
  helpers only after checking compatibility. Do not create one package per
  command or a separate workflow model for compound profiles.

- [ ] **Reduce legacy complexity incrementally.** Decompose existing oversized
  functions as they are touched; consolidate duplicate cloning, path, and
  formatting helpers only when their semantics really match. Preserve the
  current line limits without inventing generic abstraction packages.

- [ ] **Expand configuration and resolver property tests.** Fuzz JSONC, profile
  dependency graphs, path tokens, and malformed inputs. Assert deterministic
  resolution, input immutability, export/reapply equivalence, and fail-closed
  handling of invalid or changed policy. Add concurrency tests for trust/write
  boundaries and benchmarks before optimizing repeated discovery.

- [ ] **Improve disposable-clone recovery and triage.** Build on existing `gw`
  list/prune support with clear recovery instructions after interrupted export,
  resumable retained work where practical, and conflict guidance. Make generated
  artifact inclusion visible before accepting changes. Preserve agent work on
  failure and keep host Git away from untrusted clone configuration.

- [ ] **Support deliberate dependency reuse in disposable clones.** Evaluate
  read-only mounts or private reflink/copy snapshots for selected ignored build
  dependencies. Exclude secrets by default. Writable shared caches must be an
  explicit risk acknowledgment; avoid symlinks that silently expose host state.
  Demonstrate a useful offline build and isolation regression tests first.

- [ ] **Broaden compatibility testing and release documentation.** Test supported
  Linux distributions, user-namespace restrictions, alternate home/tool paths,
  and WSL where available. Run opt-in long toolchain tests periodically. Publish
  migration notes and known limitations; choose release numbers from delivered
  scope rather than the original roadmap's proposed v0.4.0 announcement.

## Priority 4: Investigate when a concrete need justifies the cost

- [ ] **Enforced outbound-network policy.** Investigate allowing required internet
  destinations while denying host/private services. Proxy environment variables
  alone are not enforcement. Account for direct sockets, DNS, IPv6, redirects,
  and proxy bypass before offering an isolation guarantee.

- [ ] **A permission ceiling independent of additive profiles.** Consider an
  explicit host-owned maximum policy that global defaults and profile updates
  cannot exceed. Specify precedence, denial diagnostics, and compatibility before
  adding it; keep it distinct from the existing compound-profile mechanism.

- [ ] **Optional syscall filtering.** Assess seccomp against the documented threat
  model and representative developer tools. Establish architecture support,
  debugging paths, and compatibility tests before enabling a default filter.

- [ ] **Resource limits for untrusted commands.** Investigate bounded memory,
  process count, execution time, and temporary storage with clear unprivileged
  host requirements. Explain which denial-of-service risks remain uncontained.

- [ ] **Read-only project sessions.** Verify or add a consistent explicit mode
  across launch entry points, with clear behavior for build outputs and caches.
  Do not equate a read-only workspace with a wholly read-only host boundary.

- [ ] **Optional toolchain-location diagnostics.** Start with read-only path
  inspection. If querying a tool is necessary, require explicit execution and
  controlled lookup; a project-supplied executable must not run on the host as
  part of automatic guessing. Avoid becoming a package manager.

- [ ] **Validated profile sharing.** Once import review and portability checks
  are established, consider exportable bundles with dependency/source metadata.
  A signature identifies an author; it does not approve permissions. Defer a
  registry service until local-file sharing proves insufficient.

- [ ] **Public release write-up.** After the safety/documentation work, prepare
  concrete examples and measured limitations for an announcement. Promotion
  and more assistant integrations rank below reliable existing workflows.

## Concrete follow-ups carried from the original roadmap

These items were on the pre-backlog roadmap and are not yet covered above in
concrete form. They fold into the priorities above; listed here so specific
tools, flags, and channels are not lost.

- [ ] **Author the remaining assistant profiles.** Add capability profiles with
  bind mounts, cache persistence, environment pass-through, and smoke tests for
  Aider (`~/.aider/`, `~/.aider.conf.yml`, tag caches; `aider --version`),
  LLM CLI (`~/.config/io.datasette.llm/`; `llm --version`), Shell-GPT
  (`~/.config/shell_gpt/`; `sgpt --version`), and GitHub Copilot CLI
  (`~/.config/github-copilot/`, `~/.config/gh/`). Validate before adding catalog
  entries; do not widen `ai` or `secure-agent` automatically (see Priority 2).

- [ ] **`--share-cache` for disposable clones.** Optional flag to bind-mount or
  snapshot selected ignored build dependencies (`node_modules`, `.venv`,
  `target/`) so clean workspaces need not re-download them. Weigh against the
  isolation trade-offs in Priority 3 "Support deliberate dependency reuse".

- [ ] **Interactive conflict-resolution helper for `bws gw`.** Guided 3-way merge
  when merge/squash-merge of an agent branch conflicts (see Priority 3
  "Improve disposable-clone recovery and triage").

- [ ] **Release announcement.** Once the safety and documentation work lands,
  prepare the announcement artifacts: a concise technical write-up on
  unprivileged Bubblewrap sandboxing for autonomous agents without Docker, and
  targeted posts (`r/golang`, `r/commandline`, `r/linux`, `r/LocalLLaMA`).
  Choose release numbers from delivered scope rather than a fixed v0.4.0.

## Scope guardrails

- Keep workflows as compound profiles, not a second configuration language.
- Do not silently activate guessed permissions, import trust, or capture secrets.
- Preserve side-effect-free plan/suggest/configuration dry runs; explicitly
  distinguish the current learning command's execution behavior.
- Do not broaden privileges to make a test pass or a tool installation easier.
- Prefer focused changes with regression tests over a whole-repository rewrite.
- Review older reports as historical proposals, not current implementation
  guarantees or automatic authorization for additional config files and features.
