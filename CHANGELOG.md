# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased] - 2026-08-14

### Added
- Added `uv` and `uvx` toolchain support to [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw): configured `~/.local/bin` in `PATH` and added read-write bindings for `~/.local/bin`, `~/.local/share/uv`, `~/.cache/uv`, and `~/.config/uv` to enable seamless Python tool and MCP server execution (e.g., fetch MCP for opencode) inside the sandbox.
- Added `-test uv` / `--test uv` CLI test option to [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to verify `uv` and `uvx` availability and execution inside the sandbox.
- Added `uv` to recommended host tool startup verifications (`verify_installed_tools`).

### Fixed
- Fixed CLI option parsing in [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) so flags passed to sandboxed commands (e.g. `bw pdflatex --version` or `bw uv --version`) are preserved instead of being intercepted by the launcher.

## [Unreleased] - 2026-07-30

### Added
- Added Quarto CLI support to [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw): automatically detects if Quarto is installed on the host, dynamically mounts `/opt` and Quarto installation root paths (resolving symlinks such as `/opt` -> `/extc/opt`), and includes `/opt` in default read-only bindings.
- Added `-test quarto` / `--test quarto` command-line option to [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to launch bubblewrap, verify Quarto execution inside the sandbox, print the detected Quarto version, and exit with status 0 ("Everything is fine.").
- Added Quarto to recommended host tool startup verifications (`verify_installed_tools`).

## [Unreleased] - 2026-07-25

### Added
- Added environment variable forwarding for `OPENROUTER_API_KEY` to [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw): passes `OPENROUTER_API_KEY` from the host environment or config into the bubblewrap container environment.
- Added `verify_bwrap_userns_support` to [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to detect AppArmor / unprivileged user namespace restrictions, present Option 2 solution details, and automatically create/reload `/etc/apparmor.d/bwrap` via `sudo` upon user interactive confirmation.
- Configured default editor environment variables (`EDITOR="emacs -nw"` and `VISUAL="emacs -nw"`) in [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) default configuration and injected exports into shell profile files (`.bashrc` and `.zshrc`).
- Added automated GitHub Deploy Key creation and registration (`get_auto_deploy_key`) to [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw): automatically detects the active repository (`owner/repo`), generates a dedicated SSH deploy key pair in `~/.sandbox/deploy_keys/`, registers it via `gh repo deploy-key add`, and populates the sandbox's `ssh-agent` with **only** that repository's deploy key. This restricts GitHub access inside the sandbox exclusively to the current project while keeping private key files hidden.
- Added host tool dependency validation (`verify_installed_tools`) to [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw): checks for required tools (`bwrap`, `tmux`) and recommended tools (`emacs`, `gh`, `git`, `ssh-agent`) on startup, presenting clear error/warning messages and package manager install suggestions (`apt`, `dnf`, `pacman`).

## [Unreleased] - 2026-06-14


### Changed
- Updated `bind_file` and `bind_dir` helper functions in [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to handle exactly `$HOME` (or `~`), resolving it to `/home/sbox` in the sandbox.
- Added default bindings in [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) for git credentials (`~/.git-credentials`), git user config directory (`~/.config/git`), SSH configurations (`~/.ssh/config`), and SSH known hosts (`~/.ssh/known_hosts`) to support seamless Git authentication and SSH operations inside the sandbox.
- Added a safety check in [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to block executing the sandbox from the host user's home directory to prevent accidentally bind-mounting the entire host home directory inside the sandbox.
- Fixed tmux line wrapping/overwrite glitches inside the sandbox by propagating the host's `LC_ALL` environment variable, binding host user terminfo directories (`~/.terminfo` and `~/.local/share/terminfo`), and starting the default tmux session with the `-u` flag to force UTF-8 mode.
- Updated [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to copy the host's `.tmux.conf` to the sandbox if the host file is newer than the sandbox copy, ensuring any configuration updates (such as enabling mouse mode) are dynamically synchronized.

## [Unreleased] - 2026-06-13

### Added
- Created [GEMINI.md](file:///home/sariel/prog/26/misc/bubblewrap_script/GEMINI.md) documenting code evaluation and improvements.
- Created [CHANGELOG.md](file:///home/sariel/prog/26/misc/bubblewrap_script/CHANGELOG.md) to track changes.
- Added `bind_file` and `bind_dir` helper functions in [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to dynamically bind files and directories to their relative locations inside the sandbox.

### Changed
- Refactored [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to clean up shebangs and command syntax.
- Switched from binding a static host `/tmp/pi_generic` directory to using a unique, instance-specific `/tmp/bw/sandbox_XXXXXX` directory, bind-mounted to `/tmp` and automatically cleaned up via an EXIT/INT/TERM shell trap.
- Fixed `.tmux.conf` bind mount path to point to `/home/sbox/.tmux.conf` so tmux inside the sandbox reads the config.
- Implemented `inject_cdtoday` in [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to dynamically inject the `cdtoday` bash function into the sandbox's `.bashrc` and `.zshrc`.
- Added existence checks for copied files (`models.json`, `resolv.conf`) to make the script robust.
- Configured [bw](file:///home/sariel/prog/26/misc/bubblewrap_script/bw) to support running custom commands passed as script arguments instead of always defaulting to `tmux`.
- Added automatic SSH agent socket forwarding and git configuration (`~/.gitconfig`) binding inside the sandbox.
- Added preservation of the `LANG` environment variable to support UTF-8 locales inside the sandbox.
- Mounted `/run/systemd/journal` read-only to resolve systemd/journald logging connection failures (such as from `tmux` or name-service lookups) which raised the warning: `Failed to create stream fd: No such file or directory`.
- Configured isolated UTS namespace with hostname `bubble` inside the sandbox.
- Added custom tmux configuration (`.tmux.conf`) inside the sandbox displaying a purple `[ BUBBLE ]` status bar indicator.
- Integrated `oh-my-posh` prompt theme engine inside the sandbox shell profiles (`.bashrc` and `.zshrc`) utilizing a custom dark-themed prompt config.
- Re-mapped the sandbox starting directory to bind the host's current working directory to its relative location under `/home/sbox/` (e.g. `/home/sbox/path/to/project`) instead of keeping the host's absolute path.

### Removed
- Removed host-side `/tmp/pi_generic` creation and deletion.
- Removed copying of nonexistent `~/bin/cdtoday`.
