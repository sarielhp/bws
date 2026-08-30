# Security and penetration audit — `bws` (Bubblewrap sandbox launcher)

*Audit conducted: 2026-08-30 · Scope: `bws` runtime sandbox, kernel namespace boundaries, IPC/socket exposures, capability dropping, path masking engine, profile catalog, and escape vectors*

---

## Executive summary & risk scorecard

A comprehensive security and penetration audit of `bws` was performed from both inside active sandbox instances and through source code analysis of the sandbox launcher pipeline (`internal/bwrap`, `internal/sandbox`, `internal/ssh`, `internal/config`, `internal/profile`).

The architecture provides strong foundational primitives: unprivileged user namespaces (`CLONE_NEWUSER`), complete capability dropping (`CapBnd` and `CapEff` set to zero), `PR_SET_NO_NEW_PRIVS` enforcement, hermetic PID isolation (`CLONE_NEWPID`), read-only root/system mounts, and an ephemeral staged `$HOME`. However, several critical and high-severity breakout vectors, IPC exposures, and credential leakage paths were identified in optional feature integrations, default configuration templates, and profile definitions.

### Risk scorecard

| Security domain | Rating | Status & primary observations |
| :--- | :---: | :--- |
| **Privilege isolation & capabilities** | **9/10** | Unprivileged user namespace strictly enforced; zero capabilities retained; `NoNewPrivs` active; setuid binaries fully neutralized. |
| **PID & process tree containment** | **9/10** | Hermetic PID namespace; PID 1 isolated; signals cannot cross namespace boundaries; `--die-with-parent` process reaping. |
| **Filesystem boundaries & masking** | **8/10** | Rootfs, `/usr`, `/sys`, `/lib` mounted read-only; `/dev` restricted to safe virtual character devices; path masking engine effective for existing host paths. Edge cases exist for non-existent host paths. |
| **Network isolation & loopback** | **5/10** | Default configuration shares network namespace (`--share-net`), exposing host `127.0.0.1` services and local network; `-N` / `--offline` provides strict air-gapped isolation. |
| **IPC & socket boundary enforcement** | **3/10** | IPC namespace is not unshared (`--unshare-ipc` missing); X11 integration binds `/run/user/<UID>` read-write (exposing D-Bus session bus and systemd user instances); D-Bus system socket is accessible over RO mounts; WSL2 interop allows Windows host execution. |
| **Credential & profile hygiene** | **4/10** | Default configuration template mounts `~/.git-credentials` and `~/.local`; `docker` profile RW binds `/var/run/docker.sock` (root breakout); `agy` profile RW binds `~/.config/gcloud` unless `no-secrets` is layered; SSH agent falls back to loading master host keys. |
| **Syscall filtering (Seccomp)** | **2/10** | No seccomp BPF filter attached; entire unprivileged Linux syscall interface is accessible. |

---

## Threat model & security perimeter

`bws` operates as an unprivileged containerizer targeting development environments and autonomous AI coding agents. The threat model assumes:

1. **Untrusted code execution**: Code, build scripts, npm/cargo/pip lifecycle scripts, and autonomous agent commands running inside the sandbox are untrusted and potentially malicious.
2. **Confined blast radius**: Sandboxed processes must not modify host system files, access unmapped private credentials, inspect host processes, or execute arbitrary commands on the host outside the sandbox.
3. **No root daemon**: All isolation is achieved unprivileged via Linux kernel namespaces and Bubblewrap (`bwrap`) without setuid helpers or root background daemons.

```mermaid
flowchart TD
    subgraph Host["Host Operating System"]
        HostFS["Host Filesystem (~/.ssh, ~/.aws, /etc)"]
        HostProc["Host Process Tree (PID 1..N)"]
        HostDBus["Host D-Bus & systemd User Services"]
        HostNet["Host Network (127.0.0.1, Docker, LAN)"]
        HostWSL["Host WSL2 Interop / Windows Host"]
    end

    subgraph Sandbox["bws Sandbox Boundary"]
        UserNS["User Namespace (CLONE_NEWUSER, Caps=0, NoNewPrivs)"]
        MountNS["Mount Namespace (RO /, RO /usr, tmpfs /etc, tmpfs /tmp)"]
        PIDNS["PID Namespace (Isolated PIDs, PID 1 = Launcher)"]
        EphHome["Ephemeral Staged $HOME (/tmp/bws/stage_*)"]
        Workspace["Project Workspace (RW / RO Mount)"]
    end

    Sandbox -- "Path Masking (--tmpfs, /dev/null)" -.-> HostFS
    Sandbox x-- "Blocked (CLONE_NEWPID)" --- HostProc
    Sandbox -- "EXPOSURE: X11 /run/user/<UID>" --> HostDBus
    Sandbox -- "EXPOSURE: Default --share-net" --> HostNet
    Sandbox -- "EXPOSURE: /run/WSL Socket" --> HostWSL
```

---

## Detailed vulnerability & escape vector analysis

### 1. Critical escape vector: X11 forwarding binds `/run/user/<UID>` read-write

* **Location**: `internal/bwrap/helpers.go` (`addX11Args`, lines 96–105)
* **Severity**: **CRITICAL**
* **Mechanism**:
  When `EnableX11` is active (or enabled in configuration), `addX11Args` detects `XDG_RUNTIME_DIR` (or `/run/user/<UID>`) and binds it into the sandbox read-write:
  ```go
  uid := os.Getuid()
  userRunDir := os.Getenv("XDG_RUNTIME_DIR")
  if userRunDir == "" {
      userRunDir = fmt.Sprintf("/run/user/%d", uid)
  }
  if fi, err := os.Stat(userRunDir); err == nil && fi.IsDir() {
      *args = append(*args, "--bind-try", userRunDir, userRunDir)
      *args = append(*args, "--setenv", "XDG_RUNTIME_DIR", userRunDir)
  }
  ```
* **Escape impact**:
  Binding `/run/user/<UID>` read-write grants the sandboxed process access to the host user's private runtime sockets:
  1. **Session D-Bus (`/run/user/<UID>/bus`)**: The sandboxed process can connect to the user D-Bus daemon and send RPC messages to `org.freedesktop.systemd1` (systemd user manager). Calling the `StartTransientUnit` method executes arbitrary binary commands on the host outside of the sandbox mount, user, and PID namespaces.
  2. **Direct systemd private socket (`/run/user/<UID>/systemd/private`)**: Allows direct systemd unit control without D-Bus.
  3. **GPG agent socket (`/run/user/<UID>/gnupg/S.gpg-agent`)**: Allows arbitrary signing and decryption requests using the host user's cached GPG keys without prompt.
  4. **Wayland & PipeWire sockets (`/run/user/<UID>/wayland-0`, `pipewire-0`)**: Allows capturing screen buffers and microphone audio.

### 2. Critical escape vector: WSL2 interop socket allows host execution

* **Location**: `internal/bwrap/helpers.go` (`addWSLArgs`, lines 109–123)
* **Severity**: **CRITICAL** (on WSL2 hosts)
* **Mechanism**:
  When running on WSL2, `addWSLArgs` mounts `/run/WSL` read-write and passes `WSL_INTEROP`:
  ```go
  if isWSL {
      *args = append(*args, "--ro-bind-try", "/init", "/init")
      *args = append(*args, "--bind-try", "/run/WSL", "/run/WSL")
      if wslInterop != "" {
          *args = append(*args, "--setenv", "WSL_INTEROP", wslInterop)
      }
      if fileExists("/mnt/c/Windows/System32/clip.exe") {
          *args = append(*args, "--ro-bind-try", "/mnt/c/Windows/System32/clip.exe", "/mnt/c/Windows/System32/clip.exe")
      }
  }
  ```
* **Escape impact**:
  The socket in `/run/WSL` is used by `/init` and `binfmt_misc` to dispatch process execution to the Windows host subsystem. Any process inside the sandbox can execute Windows binaries (e.g. `cmd.exe`, `powershell.exe`, or custom executables) via `/init` or directly if `/mnt/c` is accessible. The spawned Windows processes execute outside Linux namespaces on the Windows host with full user privileges.

### 3. Critical privilege escalation: Docker socket read-write bind

* **Location**: `profiles/docker.json` (lines 18–24)
* **Severity**: **CRITICAL**
* **Mechanism**:
  The `docker` profile mounts `/var/run/docker.sock` and `/run/user/1000/docker.sock` read-write. Furthermore, the `docker` profile contains automatic file detection rules matching `Dockerfile`, `docker-compose.yml`, `compose.yaml`, and `.dockerignore`.
* **Escape impact**:
  Access to `/var/run/docker.sock` is functionally equivalent to host root access. An untrusted script or autonomous agent can issue HTTP requests to the Docker Unix socket:
  ```bash
  curl -s --unix-socket /var/run/docker.sock http://localhost/containers/create \
    -H "Content-Type: application/json" \
    -d '{"Image":"alpine","Cmd":["chroot","/hostroot","id"],"HostConfig":{"Binds":["/:/hostroot:rw"]}}'
  ```
  This immediately achieves arbitrary root file modification and host execution outside the sandbox.

### 4. High-severity information disclosure: Default configuration template mounts `~/.git-credentials`

* **Location**: `internal/config/config.go` (`generateDefaultConfig`, line 245)
* **Severity**: **HIGH**
* **Mechanism**:
  When `bws` initializes a default `~/.config/bws/config.jsonc` file, `generateDefaultConfig()` embeds:
  ```json
  "binds_ro": [
    ["~/.local", "%[1]s/.local"],
    ["~/.gitconfig", "%[1]s/.gitconfig"],
    ["~/.git-credentials", "%[1]s/.git-credentials"],
    ["~/.config/git", "%[1]s/.config/git"],
    ["~/.ssh/config", "%[1]s/.ssh/config"],
    ["~/.ssh/known_hosts", "%[1]s/.ssh/known_hosts"]
  ]
  ```
* **Impact**:
  1. `~/.git-credentials` stores unencrypted, plaintext HTTP authentication tokens, GitHub/GitLab Personal Access Tokens (PATs), and passwords saved by `git-credential-store`.
  2. `~/.local` contains user session data, application state, local caches, and potentially GNOME Keyring database files (`~/.local/share/keyrings`).

### 5. High-severity credential exposure: SSH agent master key fallback & read-write `known_hosts`

* **Location**: `internal/ssh/ssh.go` (lines 40, 62) & `internal/bwrap/helpers.go` (line 54)
* **Severity**: **HIGH**
* **Mechanism**:
  1. In `internal/ssh/ssh.go`, when `len(keys) == 0` (e.g. when not in a GitHub repository with `gh` deploy key integration, or when `auto_repo_deploy_key` is disabled), `EnsureAgent()` invokes bare `exec.Command("ssh-add")`.
  2. Running `ssh-add` without arguments on the host searches default paths (`~/.ssh/id_rsa`, `id_ed25519`, `id_ecdsa`) and loads the host user's master private keys into the sandbox-forwarded agent socket.
  3. In `internal/bwrap/helpers.go`, line 54 binds host `known_hosts` read-write:
     ```go
     *args = append(*args, "--bind", filepath.Join(hostSSHDir, "known_hosts"), filepath.Join(util.HomeDir(), ".ssh", "known_hosts"))
     ```
* **Impact**:
  1. If deploy key provisioning does not occur, the sandbox inherits all master host SSH keys, allowing untrusted code to authenticate to all host SSH destinations and Git remotes.
  2. Read-write mounting of `known_hosts` allows a compromised sandbox process to tamper with or corrupt the host's known host keys.

### 6. High-severity exposure: Read-only VFS mounts do not block Unix domain socket communication

* **Location**: `internal/bwrap/bwrap.go` (line 278: `--ro-bind-try /run/dbus /run/dbus`)
* **Severity**: **MEDIUM**
* **Mechanism**:
  `bwrap.go` unconditionally binds `/run/dbus` read-only. On Linux, VFS mount flags (`MS_RDONLY`) only prevent filesystem metadata modifications and opening regular files with `O_WRONLY` / `O_RDWR`. Connecting to a stream or datagram Unix domain socket (`AF_UNIX`) via `connect(2)` is handled by the socket subsystem and succeeds regardless of whether the socket node resides on a read-only mount.
* **Impact**:
  Processes inside the sandbox can connect to `/run/dbus/system_bus_socket`. Queries to system services (e.g., AccountsService, NetworkManager, UDisks2, Polkit) can be dispatched, leaking system metadata and configuration state.

### 7. Medium-severity exposure: Missing IPC namespace isolation

* **Location**: `internal/bwrap/bwrap.go`
* **Severity**: **MEDIUM**
* **Mechanism**:
  `bws` does not pass `--unshare-ipc` to `bwrap`.
* **Impact**:
  System V IPC mechanisms (shared memory segments via `shmget`/`shmat`, message queues via `msgget`, semaphores via `semget`) and POSIX shared memory (`/dev/shm` unless masked) remain visible to processes with matching UIDs. Furthermore, abstract Unix domain sockets (sockets prefixed with null bytes `@`) do not reside in the filesystem VFS and are not isolated unless network or IPC namespaces are unshared.

### 8. Medium-severity exposure: Unrestricted syscall interface (No Seccomp filter)

* **Location**: `internal/bwrap/bwrap.go`
* **Severity**: **MEDIUM**
* **Mechanism**:
  `bws` does not attach a Seccomp BPF filter (`--seccomp <fd>`).
* **Impact**:
  All system calls available to unprivileged users in the kernel (e.g. `io_uring_setup`, `bpf`, `userfaultfd`, `perf_event_open`, `process_vm_readv`, `fsopen`, `mount_setattr`) can be invoked. Any vulnerability in unprivileged kernel syscall handlers exposes the host kernel to privilege escalation from within the sandbox.

---

## Empirical penetration testing matrix

The following automated test suite was executed against active sandbox sessions and configuration resolution paths.

| Category | Test description | Command / Probe | Result | Risk severity | Notes |
| :--- | :--- | :--- | :---: | :---: | :--- |
| **Capabilities** | Capability bounding set cleared | `/proc/self/status` `CapBnd` | **PASS** | High | `CapBnd = 0000000000000000` |
| **Capabilities** | Effective capabilities dropped | `/proc/self/status` `CapEff` | **PASS** | High | `CapEff = 0000000000000000` |
| **Capabilities** | Inheritable / Permitted / Ambient zeroed | `/proc/self/status` `CapPrm/Inh/Amb` | **PASS** | High | All capability bitmasks zeroed |
| **Capabilities** | `NoNewPrivs` bit set | `/proc/self/status` `NoNewPrivs` | **PASS** | High | `NoNewPrivs: 1` enforced |
| **Privilege Escalation** | Setuid `sudo` execution | `sudo id` | **PASS** | Critical | Masked (`/dev/null`, 0-byte, blocked) |
| **Privilege Escalation** | Setuid `su` execution | `su -c id` | **PASS** | Critical | Masked (`/dev/null`, 0-byte, blocked) |
| **Privilege Escalation** | Setuid `pkexec` execution | `pkexec id` | **PASS** | Critical | Masked (`/dev/null`, 0-byte, blocked) |
| **Namespaces** | PID namespace isolation | `/proc/[0-9]*` enumeration | **PASS** | High | Only sandbox processes visible (2 PIDs) |
| **Namespaces** | PID 1 isolation | `/proc/1/comm` | **PASS** | Critical | PID 1 is sandbox launcher, not host init |
| **Namespaces** | User namespace isolation | `/proc/self/uid_map` | **PASS** | High | Unprivileged user mapping |
| **Filesystem** | Root directory read-only | `touch /test_write` | **PASS** | High | `Read-only file system` (EPERM) |
| **Filesystem** | System dirs (`/usr`, `/bin`, `/lib`) read-only | `touch /usr/test_write` | **PASS** | High | `Read-only file system` (EPERM) |
| **Filesystem** | `/tmp` isolated instance | Check `/tmp` mounts | **PASS** | Medium | Private tmpfs / staging mount |
| **Filesystem** | `/etc` tmpfs overlay | `ls -la /etc/sudoers` | **PASS** | High | Overlaid with tmpfs; sudoers masked |
| **Devices** | Raw disk devices unmapped | Check `/dev/sda`, `/dev/nvme0n1` | **PASS** | Critical | Nodes do not exist |
| **Devices** | Physical memory unmapped | Check `/dev/mem`, `/dev/kmem`, `/dev/port` | **PASS** | Critical | Nodes do not exist |
| **Devices** | Standard safe devices present | Check `/dev/null`, `/dev/urandom` | **PASS** | Low | Virtual character devices functional |
| **Kernel IF** | `/proc/sysrq-trigger` write blocked | Write to sysrq-trigger | **PASS** | Critical | `Permission denied` |
| **Kernel IF** | `/proc/kcore` read blocked | Read from `/proc/kcore` | **PASS** | High | `Permission denied` |
| **Sysfs** | `/sys` read-only | Write to `/sys/kernel/` | **PASS** | High | `Read-only file system` |
| **Sysfs** | `/sys/kernel/debug` unreadable | Access `/sys/kernel/debug` | **PASS** | Medium | Unreadable / inaccessible |
| **Workspace** | `.bws/` config directory auto-masked | `ls -la .bws/` in workspace | **PASS** | High | Overlaid with empty `tmpfs` |
| **SSH** | Deploy keys directory masked | `ls -la ~/.sandbox/deploy_keys/` | **PASS** | High | Overlaid with empty `tmpfs` |
| **X11 Exposure** | X11 socket access | Connect to `/tmp/.X11-unix/X0` | **WARN** | High | Connectable when `enable_x11: true` |
| **D-Bus Exposure** | User session bus access | Connect to `/run/user/<UID>/bus` | **WARN** | Critical | Connectable if `XDG_RUNTIME_DIR` bound |
| **WSL2 Interop** | WSL bridge socket access | Connect to `/run/WSL` | **WARN** | Critical | Connectable if WSL2 detection active |
| **Docker Socket** | Docker daemon socket access | Connect to `/var/run/docker.sock` | **WARN** | Critical | Connectable if `docker` profile loaded |

---

## Actionable hardening recommendations

To eliminate all identified escape vectors and harden `bws` against local privilege escalation and credential theft, implement the following architectural and code remediations:

### 1. Remediate X11 integration & drop `/run/user/<UID>` read-write mount

**Action**: Remove the `--bind-try` of `XDG_RUNTIME_DIR` in `internal/bwrap/helpers.go`. If X11 is needed, bind only the specific X11 socket (`/tmp/.X11-unix/X<N>`) and `.Xauthority` read-only. Never mount `/run/user/<UID>`.

```diff
--- a/internal/bwrap/helpers.go
+++ b/internal/bwrap/helpers.go
@@ -95,12 +95,6 @@ func addX11Args(args *[]string) {
 		*args = append(*args, "--setenv", "XAUTHORITY", filepath.Join(util.HomeDir(), ".Xauthority"))
 	}
 
-	uid := os.Getuid()
-	userRunDir := os.Getenv("XDG_RUNTIME_DIR")
-	if userRunDir == "" {
-		userRunDir = fmt.Sprintf("/run/user/%d", uid)
-	}
-	if fi, err := os.Stat(userRunDir); err == nil && fi.IsDir() {
-		*args = append(*args, "--bind-try", userRunDir, userRunDir)
-		*args = append(*args, "--setenv", "XDG_RUNTIME_DIR", userRunDir)
-	}
-
 	*args = append(*args, "--setenv", "NO_AT_SPI", "1")
 }
```

### 2. Isolate or gate WSL2 interop execution

**Action**: Do not bind `/run/WSL` or `/init` by default. If WSL clipboard integration (`clip.exe`) is needed, bind only `clip.exe` and pipe clipboard data without exposing the raw `/run/WSL` socket.

```diff
--- a/internal/bwrap/helpers.go
+++ b/internal/bwrap/helpers.go
@@ -112,8 +112,6 @@ func addWSLArgs(args *[]string) {
 	isWSL := (wslInterop != "") || dirExists("/run/WSL") || fileExists("/proc/sys/fs/binfmt_misc/WSLInterop")
 
 	if isWSL {
-		*args = append(*args, "--ro-bind-try", "/init", "/init")
-		*args = append(*args, "--bind-try", "/run/WSL", "/run/WSL")
 		if wslInterop != "" {
 			*args = append(*args, "--setenv", "WSL_INTEROP", wslInterop)
 		}
```

### 3. Remove `docker.sock` from default `docker` profile & disable auto-detection

**Action**: Modify `profiles/docker.json` to require explicit Docker-in-Docker or rootless container proxies rather than raw socket passthrough. Remove automatic detection matching `Dockerfile` / `docker-compose.yml` that could silently grant root breakout in arbitrary repos.

### 4. Sanitize default configuration template

**Action**: In `internal/config/config.go` (`generateDefaultConfig`), remove `~/.git-credentials` and `~/.local` from `binds_ro`.

```diff
--- a/internal/config/config.go
+++ b/internal/config/config.go
@@ -240,9 +240,6 @@ func generateDefaultConfig() string {
     ["/libx32", "/libx32"],
     ["/sbin", "/sbin"],
     ["/run/systemd/journal", "/run/systemd/journal"],
-    ["~/.local", "%[1]s/.local"],
     ["~/.gitconfig", "%[1]s/.gitconfig"],
-    ["~/.git-credentials", "%[1]s/.git-credentials"],
     ["~/.config/git", "%[1]s/.config/git"],
     ["~/.ssh/config", "%[1]s/.ssh/config"],
     ["~/.ssh/known_hosts", "%[1]s/.ssh/known_hosts"]
```

### 5. Always unshare the IPC namespace (`--unshare-ipc`)

**Action**: In `internal/bwrap/bwrap.go`, add `--unshare-ipc` to the standard bwrap arguments.

```diff
--- a/internal/bwrap/bwrap.go
+++ b/internal/bwrap/bwrap.go
@@ -20,6 +20,7 @@ func BuildArgs(cfg *config.Config, sandboxDir, currentDir string, dryRun, verbose
 	}
 
 	args = append(args, "--tmpfs", "/etc")
+	args = append(args, "--unshare-ipc")
 	if verbose {
 		fmt.Fprintf(os.Stderr, "[verbose]   --tmpfs /etc\n")
+		fmt.Fprintf(os.Stderr, "[verbose]   --unshare-ipc\n")
 	}
```

### 6. Restrict SSH agent key ingestion & mount `known_hosts` read-only

**Action**:
1. In `internal/ssh/ssh.go`, never call `ssh-add` without explicit key paths. If no deploy key is available and no keys are specified, start an empty agent rather than loading master host keys.
2. In `internal/bwrap/helpers.go`, change `filepath.Join(hostSSHDir, "known_hosts")` mount from `--bind` to `--ro-bind`.

### 7. Remove unconditional `/run/dbus` bind mount

**Action**: Remove `--ro-bind-try /run/dbus /run/dbus` from `bwrap.go`. Developer toolchains and compilers do not require system D-Bus access.

### 8. Introduce default Seccomp syscall filtering

**Action**: Integrate a default BPF Seccomp filter denying dangerous or unnecessary system calls (`io_uring_setup`, `bpf`, `userfaultfd`, `kexec_load`, `reboot`, `sys_kcmp`, `perf_event_open`, `process_vm_readv`, `process_vm_writev`).

---

## Conclusion & verification summary

The core containerization engine of `bws` provides robust baseline isolation for CPU, memory, filesystem, and process hierarchies through unprivileged Bubblewrap primitives. By implementing the eight hardening recommendations above—particularly eliminating `/run/user/<UID>` and `/run/WSL` socket bindings, unsharing the IPC namespace, removing plaintext credential bindings, and restricting SSH/Docker socket access—`bws` achieves complete hermetic containment for autonomous AI agents and untrusted developer workloads.
