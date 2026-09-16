# Compound profile implementation

## Delivered interface

- Effective-policy `profile save`, with dry-run JSON, dependency references,
  optional flattening, workspace/home tokens, omission acknowledgments,
  machine-path review, and atomic writes.
- `profile compose`, `profile suggest [directory] [--json] [--compound]`,
  `profile list --compound`, and `profile review [--accept]`.
- Interactive profile selection during `init`; explicit selections do not
  silently acquire detected profiles. Existing projects remain unchanged
  without explicit replacement.
- Reference-based project configuration with reviewed selection fingerprints.
- Consistent launch-time initialization for bare `bws` and `bws run`.
- A bounded, read-only filename scanner shared by profile and legacy stack
  detection; deterministic ranking with evidence and grouped matching rules.

## Implementation

`internal/policy` now owns the launch/export resolution logic, input cloning,
export equivalence checks, source explanations, and explicit flag application.
`internal/detect` owns traversal limits and exclusions. Profiles retain the
existing resolver and add compound metadata, conflict diagnostics, reviewed
dependency fingerprints, and effective host-rule permission digests.

New initialization uses profile references rather than copying profile mounts.
Preset flags are adapters to explicit profile selections. Legacy generation
helpers remain available but are no longer the new initialization path.

Approval metadata is not a toolchain lockfile. It detects changed/shadowed
profile definitions and host-conditional policy changes. Effective global
configuration can still add access; profiles are not permission ceilings.

## Compatibility and boundaries

- Existing profile formats and command aliases remain accepted.
- Noninteractive `init` requires `--profile`, `--preset`, or `--basic`.
- Existing `auto_init: always` continues basic embedded-tool detection; it does
  not silently activate saved agent combinations or local/global overrides.
- Existing array-combination semantics are preserved. The discrepancy between
  AGENTS.md's replacement wording and the implementation/configuration docs
  was not turned into an unrelated merge migration.
- Unsupported configuration-only settings are reported and require explicit
  `--omit` acknowledgment for partial exports.
- Global compounds cannot retain workspace-local dependencies; flatten them
  or save locally.
- No captured processes, installed binaries, credential contents, cache data,
  or previous command-line flags.
- Known credential-like environment literals are rejected, but arbitrary
  strings cannot be proven non-secret. Human summaries hide environment values.
- Dependency review does not approve untrusted local source files. Existing
  host trust checks remain mandatory.
- Filesystem writers reject symlink destinations, use atomic replacement and
  expected-content checks, and coordinate through nonblocking locks.

## Verification

The standard format/vet/test/build verification passed. The full default
regression suite and race-detector suite passed. The line-count audit passed
with existing unrelated legacy-function notices; no new oversized functions.
Whitespace validation passed.

New coverage includes:

- Save → suggest → initialize across projects, without duplicated mounts.
- Effective global/local/profile export equivalence and input immutability.
- Changed and shadowed dependency approval; nested and host-rule checks.
- Built-in compound profile compatibility.
- Preserved SSH/offline restrictions despite legacy initialization defaults.
- Deterministic ranking, mixed-language requirements, and scan boundaries.
- Symlink destinations, create-only writes, and stale-preview refusal.
- Dry runs leaving configuration and trust state untouched.
- Credential redaction, unsupported-field acknowledgments, and path tokens.
- Confirmation input boundaries, noninteractive ambiguity, and launch auto-init.

Opt-in long tests were not run. The executable was verified on the current
Linux host; this does not establish compatibility with every distribution or WSL.

See [the user guide](../docs/compound_profiles.md) for examples and migration.
