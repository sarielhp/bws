# Compound profiles

A compound profile is an ordinary profile with `kind: "compound"` and
`requires` dependencies. It uses the existing permission resolver. It does not
install tools, capture a running shell, or introduce a workflow format.

## Save a working setup

```bash
bws init --profile go,opencode,git --no-ssh
bws profile save go-agent --dry-run
bws profile save go-agent
```

Save reads effective global, trusted local, and profile configuration. The
default destination is `~/.config/bws/profiles/go-agent.json`. Use `--local`
for `.bws/profiles/go-agent.json`. Saving does not activate the profile.

Dependencies remain references where their settings can be represented without
changing the resulting policy. Other supported settings become explicit profile
declarations. Save checks reapplication against the current global baseline.
If ordering cannot be represented, `--flatten` materializes the effective
capabilities instead of retaining dependencies. A flattened profile no longer
inherits future dependency updates or their smoke tests.

Global profiles cannot retain workspace-local dependencies: use `--flatten`,
save locally, or install the dependency globally first.

### Review and portability

The preview includes writable paths, network/SSH/desktop access, environment
names, proposed detection rules, source files, and limitations. Environment
values are hidden in the human summary. The JSON preview contains configuration
literals; review it before sharing. Known credential-like literal names are
rejected; arbitrary strings cannot be proven non-secret.

Save never reads host environment values into the definition and never copies
credential files, cache contents, processes, or installed toolchains.
`pass_env` and `@@PASS@@` declarations remain references, not captured values.

- Home-relative paths use `@@HOME@@`.
- Workspace-relative paths use `@@WORKSPACE@@`, expanded against the selected
  workspace root, including when launching from a subdirectory.
- Other machine-specific absolute paths require `--allow-machine-paths`.
- Unsupported configuration-only settings require explicit acknowledgment:
  `--omit sandbox_path`, `--omit max_file_count`, etc. The preview lists the
  applicable names. A partial export does not reproduce those settings.
- Runtime-generated `BWS_*`, SSH socket, and D-Bus address variables are not
  saved. Flags from a previous launch cannot be recovered. Explicit permission
  flags on the save invocation, such as `--no-ssh`, are captured.

Custom system settings, persistent sandbox directories, model configuration
paths, nondefault session names/file limits, and auto-init preferences are not
profile capabilities. Defaults remain part of the host configuration.

`--force` is required to replace or shadow a profile. It does not bypass trust,
dependency checks, or portability checks. Writes reject symlink destinations,
use atomic replacement, and check that an existing destination still matches
the preview. Cooperating writers use a nonblocking lock.

Interactive saves ask for confirmation; `--yes` skips that prompt.
Noninteractive saves with explicit names do not prompt, but still require
explicit portability and omission flags where applicable.

## Compose a profile directly

```bash
bws profile compose go-agent --profiles go,opencode,git --no-ssh --match go.mod
bws profile show go-agent
bws profile list --compound
```

`profile create` remains an alias for remote profile synthesis, not composition.
Choose a particular assistant instead of `ai` if access to every assistant in
that bundle is unnecessary.

Conflicting dependency environment values require an explicit compound
`env` override. Contradictory feature declarations require an explicit
`features` choice. Different source/access declarations at the same mount
destination are rejected; remove or change the conflicting declaration.

Global configuration remains a baseline and can add access. A compound profile
is not a permission ceiling. Explicit compound environment settings override
global defaults; explicit project environment settings override those. Existing
restrictive profile settings, such as disabled SSH or isolated networking,
cannot be undone by legacy generated local defaults.

## Guessing and initialization

```bash
bws profile suggest
bws profile suggest /path/to/project --compound --json
bws init
bws init --profile go-agent --dry-run
bws init --profile go-agent
```

Interactive initialization presents compound candidates and accepts a number,
comma-separated profile names, or `basic`. It then displays effective
permissions before writing. Empty input cancels.

Explicit `--profile` selections do not acquire extra detected tools.
Initialization stores profile references and reviewed dependency fingerprints;
it does not copy dependency mounts into the local configuration.

Existing projects remain unchanged unless explicitly reinitialized with
`--force`. Replacement keeps a `.bak` copy. Noninteractive initialization
requires `--profile`, a preset, or `--basic`; `--yes` does not select a
candidate. `--basic` explicitly requests detected embedded tool profiles.

Both bare `bws` and `bws run` honor the same launch-time auto-init policy:

- `prompt`: offer selection on a terminal; do not read redirected command input.
- `never` or `--no-init`: do not initialize.
- `always`: retain unattended basic tool detection, not automatic activation
  of arbitrary saved agent profiles. Global/local overrides are never used as
  unattended detection candidates.

Preset flags remain compatibility adapters to tool-profile selections.

### Detection rules

The scanner inspects names only, follows no symlinks, and runs no commands.
It descends at most three directory levels and stops with an error after
10,000 entries. Dependency/build directories and hidden directory contents
are skipped. Detection never silently widens the selected workspace.

Exact root markers outrank nested markers and extension globs. Directory-name
hints are weak. Adding more dependencies does not improve a candidate's rank.
Ties remain alternatives, ordered by name for stable display.

Existing `files`, `globs`, and `dir_contains` rules are alternatives. Every
`all_of` group must also match; groups cannot nest:

```json
{
  "name": "go-web",
  "kind": "compound",
  "requires": ["go", "node", "git"],
  "detect": {
    "all_of": [
      {"files": ["go.mod", "go.work"]},
      {"files": ["package.json"]}
    ]
  }
}
```

Filenames without a slash can match within the bounded subtree. Use a relative
path such as `web/package.json` to require that location.

Save proposes rules from strong root manifests. `--match <file>` supplies
explicit alternatives; `--no-detect` disables rule generation. Compose does not
infer matching rules from an unrelated current project.

## Review updates

Saved compounds record dependency source/content fingerprints, a permission
baseline without environment values, and an effective-policy digest. Changes,
local shadowing, and host-conditional permission changes require review:

```bash
bws profile review go-agent
bws profile review go-agent --accept
```

Review prints added/removed grants and current permissions. It does not write
without `--accept`. Environment literal changes are detected by fingerprints,
but their values are not displayed. Review changed nested compounds first.

Initialized projects also pin their selected roots and dependency closure.
After approving a changed compound, explicitly update a project selection:

```bash
bws init --force --profile go-agent
```

Untrusted or manually edited local policy first requires the existing
`bws config trust` review step. Suggesting, saving, or using `--force` never
implicitly approves imported local files.

These records pin reviewed policy, not executable versions. Another machine's
global configuration can add grants; inspect its effective initialization plan.
