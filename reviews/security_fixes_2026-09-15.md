# Sandbox boundary fixes

This change addresses the seven findings from the September 15 review.

| Finding | Change | Regression coverage |
| --- | --- | --- |
| Host Git executes agent-controlled hooks/configuration | Commit and bundle export run in a separate offline sandbox; host fetch reads a streamed bundle, never the agent clone | Live malicious hook, fsmonitor and clean-filter fixture; full CLI workflow with approved config; failed commit preserves source files |
| Skeleton links disclose host files | Directory-confined reads reject escaping links and special files, including linked skeleton roots | File, skeleton-directory and parent-directory escape fixtures; internal file link remains usable |
| Discovered root bypasses workspace safety | Discovery stops at protected directories; file counts cover the mounted workspace root | Home/bin ancestor fixtures, force checks, small child of oversized workspace |
| `no-ssh` still forwards an agent | Profile disables SSH/deploy-key setup; disabled SSH also clears the socket environment | Both security profiles resolved; live socket absent from mount arguments |
| New local configuration bypasses masks | File-path/content approval required for local configuration and profiles; approval store masked | Newly planted and modified configs rejected; profile shadowing rejected; explicit CLI approval tested |
| Profile failures ignored | Missing, cyclic, malformed and invalid-bind profiles return errors through launch | Dependency, parser, bind-shape and launch rejection tests |
| Preview initializes configuration | Embedded defaults parsed in memory when global configuration is absent | Preview leaves a fresh home empty |

## Compatibility

Review existing local configuration and profiles, then run `bws config trust`. Host configuration-writing commands record the contents they generate. Manual changes require renewed approval. Trust is not transferable merely by copying a file to a new workspace path; `gw` transfers approval only for already approved source configuration.

Profile dependencies must name installed profiles. Skeleton links must resolve to regular files within the skeleton tree. Use explicit read-only mounts for external resources.

Testing uses temporary homes so configuration-reset tests do not modify the developer's settings. This revealed a mount-order issue for homes beneath `/tmp`; the private `/tmp` is now mounted before the staged home, followed by the workspace mounts.

This change does not redefine access intentionally granted by trusted configuration, such as writable host mounts, X11, or network access. It is a targeted repair of the reviewed boundaries, not a complete security certification.

## Verification

`tools/verify_build.sh` passed formatting, `go vet ./...`, the complete default test suite, and compilation. Live Bubblewrap tests exercised malicious Git metadata, bundle import, and the full CLI workflow. `tools/audit_lines.rb` and `git diff --check` passed; the audit still reports pre-existing legacy function-length exceptions. Opt-in long toolchain tests were not run.
