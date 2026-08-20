# Gemini Documentation - Bubblewrap Sandbox Script

> [!CAUTION]
> **Freeze Directive: Do Not Update `bw`**
> Do **NOT** make any further modifications or updates to the `bw` script due to WSL issues until further notice.

This directory contains `bw`, a script that launches a bubblewrap sandbox designed to isolate the directory it is run from while providing a persistent home environment in `~/.sandbox/pi_generic/`.

## Code Evaluation of Original `bw` Script

During evaluation, several issues and areas of improvement were identified:

1. **Double Shebang**: 
   The script started with two shebangs:
   ```bash
   #! /bin/bash
   #!/bin/bash
   ```
   This is redundant and has been cleaned up to a single standard shebang.

2. **Incorrect `.tmux.conf` Binding Target**:
   The script ran:
   ```bash
   --ro-bind-try "$HOME/.tmux.conf" "$HOME/.tmux.conf"
   ```
   Inside the sandbox, `HOME` is set to `/home/sbox`. Therefore, tmux looks for its configuration at `/home/sbox/.tmux.conf`. The original script bound the host's `.tmux.conf` to `/home/sariel/.tmux.conf` (which is outside the sandbox's home directory and not even visible unless `/home/sariel` is mounted). This is a bug and is resolved by binding it to `/home/sbox/.tmux.conf`.

3. **Insecure and Redundant `/tmp` Directory Creation on Host**:
   The script removed and recreated `/tmp/pi_generic` on the host on every run and bound it to `/tmp`. If multiple sandbox instances run concurrently, they interfere with each other, leading to race conditions.
   * **Fix**: Use a unique, randomly-named directory on the host under `/tmp/bw/sandbox_XXXXXX` (via `mktemp -d`) and bind it to `/tmp` in the sandbox. We also configure a Bash `trap` on EXIT, INT, and TERM in `bw` to remove this directory upon exit, and execute `bwrap` directly (without `exec`) so the cleanup executes reliably. This keeps `/tmp` backed by host disk space (per user preference) while providing perfect isolation and concurrency safety.

4. **Handling of `cdtoday` as a Bash Function**:
   The original script attempted to copy `~/bin/cdtoday` (as an external executable) into the sandbox's `/bin/` path. However, `cdtoday` is a Bash function in the user's `.bashrc`.
   * **Fix**: External scripts cannot modify the working directory of the calling shell. To support this, we remove the `rsync` of `~/bin/cdtoday` and instead dynamically write/ensure the `cdtoday()` function is defined in the sandbox's `.bashrc` and `.zshrc` files.

5. **Missing File and Directory Existence Checks**:
   The script ran `rsync` on `~/info/llm/models.json` and `/etc/resolv.conf` without checking if they exist on the host, which could cause warnings/errors.
   * **Fix**: Added checks to ensure copy operations only execute when the source files exist.

6. **Helper Functions for Custom Bindings**:
   To make custom path mapping clean and declarative, we added:
   - `bind_file "~/path/whatever.txt"`
   - `bind_dir "~/path"`
   * **Behavior**: These expand `~` to the host's `HOME`, convert it to an absolute path, verify existence, and automatically translate paths under `$HOME/` to `/home/sbox/` inside the sandbox (preserving their relative locations). Non-home paths are bound to the same absolute path.

---

## Further Enhancements Implemented during Code Review

1. **Custom Command Execution**:
   - Instead of hardcoding `tmux new-session` at the end, the script now checks if any arguments are passed (`$# -eq 0`).
   - If no arguments are passed, it defaults to the interactive `tmux` session.
   - If arguments are passed (e.g., `./bw python3 test.py`), it executes those arguments directly inside the sandbox.

2. **SSH Agent Socket Forwarding**:
   - Added automatic detection and bind-mounting of `SSH_AUTH_SOCK` (and setting of the environment variable inside the sandbox). This allows SSH-based Git operations to work seamlessly within the insulated environment.

3. **Git Configuration Mapping**:
   - Bound the host's `~/.gitconfig` to `/home/sbox/.gitconfig` by default (using our `bind_file` helper) so the sandbox inherits Git settings (identity, credentials helper, etc.) out-of-the-box.

4. **Locale/Encoding Preservation**:
   - Preserves `LANG` (defaulting to `C.UTF-8` if empty) to ensure UTF-8 encoding support is maintained for Python, Node, Git, etc.

5. **Systemd Journal Socket Mounting**:
   - Bound the host's `/run/systemd/journal` directory read-only to resolve systemd/journald socket connection failures (such as from `tmux` or name-service lookups) which raised the warning: `Failed to create stream fd: No such file or directory`.
   - The `--ro-bind-try` argument mounts this directory as read-only, which allows outgoing logging connections to the socket but prevents writing or creating new files on the host filesystem.

6. **UTS Namespace and Hostname Isolation**:
   - Configured `--unshare-uts` and `--hostname bubble` to isolate the host name inside the sandbox. The hostname inside the sandbox resolves to `bubble`.

7. **Custom Tmux Config & Bubble Badge**:
   - Created a custom `.tmux.conf` inside the sandbox that inherits the host's keybindings but appends a prominent, purple `[ BUBBLE ]` badge to the status bar, visually signaling that the session is sandboxed.

8. **Oh My Posh Dark Theme Integration**:
   - Dynamically installs `oh-my-posh` in the sandbox (copied from `/home/sariel/bin/oh-my-posh`) and writes a custom dark-themed configuration (`.mytheme.omp.json`) inside the sandbox.
   - Automatically injects the initialization script in the sandbox's `.bashrc` and `.zshrc` to load the premium dark theme with custom `BUBBLE` indicator, username@hostname, current path, and git status segments.

9. **Relative Working Directory Mapping**:
   - Re-mapped the sandbox starting directory to bind the host's current directory to its relative location under `/home/sbox/` (for example, `/home/sariel/a/b/c` on the host mounts to `/home/sbox/a/b/c` inside the sandbox). This hides the host's username from absolute paths, keeps root paths clean, and keeps the project workspace aligned with the home directory scope.

---

## Improvement Plan

1. Update `bw` to handle the `cdtoday` function configuration dynamically in the sandbox user's shell profile.
2. Clean up shebangs and command syntax.
3. Replace `/tmp/pi_generic` host directory dependency with a unique, disk-backed `/tmp/bw/sandbox_XXXXXX` directory cleaned up on exit.
4. Fix the target path of `--ro-bind-try "$HOME/.tmux.conf"`.
5. Implement `bind_file` and `bind_dir` helper functions.
6. Maintain a `CHANGELOG.md` to track edits.

