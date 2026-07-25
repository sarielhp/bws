# WSL Clipboard Integration Issue in Bubblewrap Sandbox

The user wants to select text using the mouse inside a `tmux` window running inside the bubblewrap sandbox (`bw`) and have it automatically copied to the system clipboard (the X11 cut buffer / Windows clipboard) so that it can be pasted easily using only the mouse.

## Root Causes of the Issue

### 1. Incompatibility of `xclip` under WSL
* We initially added X11 display socket forwarding and bound `xclip` inside `.tmux.conf` to handle mouse selection copying.
* However, in WSL (Windows Subsystem for Linux), X11 is often not running, or there is no active X server, meaning `xclip` fails with `Can't open display: :0`.
* When the `copy-pipe-and-cancel` command fails due to `xclip` failing, `tmux` aborts the copy operation entirely, preventing even standard `tmux` buffer copies.

### 2. Sandbox Isolation blocks Windows Binary execution (`clip.exe`)
The standard way to copy to the Windows system clipboard in WSL is piping text to `/mnt/c/Windows/System32/clip.exe`. However, inside the `bw` bubblewrap sandbox:
1. **Missing interop socket**: The WSL interop Unix socket (referenced by `$WSL_INTEROP` in `/run/WSL/`) is not mounted in the sandbox.
2. **Missing `binfmt_misc` interpreter**: WSL executes Windows binaries using a system-wide kernel `binfmt_misc` registration called `WSLInterop`. This registration specifies `/init` as the interpreter. Because the sandbox filesystem does not mount the host's `/init` binary, any attempt to run a Windows binary fails with `execvp: No such file or directory`.
3. **Missing binary paths**: `/mnt/c` is not mounted in the sandbox, meaning the `clip.exe` binary itself is missing.

---

## Proposed Solutions

### Solution A: Enable WSL Interop inside Bubblewrap (Recommended)
We can enable the execution of `clip.exe` inside the sandbox by performing the following mounts:
1. Bind the `/init` interpreter:
   ```bash
   --ro-bind /init /init
   ```
2. Bind the WSL interop sockets directory:
   ```bash
   --ro-bind /run/WSL /run/WSL
   ```
3. Propagate the `WSL_INTEROP` environment variable:
   ```bash
   --setenv WSL_INTEROP "$WSL_INTEROP"
   ```
4. Bind the Windows `clip.exe` binary to its exact host path:
   ```bash
   --ro-bind /mnt/c/Windows/System32/clip.exe /mnt/c/Windows/System32/clip.exe
   ```
5. Configure `.tmux.conf` inside the sandbox to use `clip.exe` when WSL is detected:
   ```tmux
   if-shell 'test -n "$WSL_INTEROP"' {
     bind-key -T copy-mode-vi MouseDragEnd1Pane send-keys -X copy-pipe-and-cancel "/mnt/c/Windows/System32/clip.exe"
     bind-key -T copy-mode    MouseDragEnd1Pane send-keys -X copy-pipe-and-cancel "/mnt/c/Windows/System32/clip.exe"
   }
   ```

### Solution B: Fix OSC 52 Passthrough
Alternatively, since Windows Terminal natively supports the **OSC 52** escape sequence for clipboard synchronization:
1. Ensure `set-clipboard` is enabled:
   ```tmux
   set -s set-clipboard on
   ```
2. Revert any `copy-pipe-and-cancel` bindings that override `MouseDragEnd1Pane` so that `tmux` uses its native `copy-selection-and-cancel` behavior, which generates the OSC 52 escape sequences directly.
3. Configure `allow-passthrough` to allow the sequences to reach the terminal:
   ```tmux
   set-option -g allow-passthrough on
   ```
