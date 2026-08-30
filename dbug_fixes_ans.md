# D-Bus security remediation and sandbox hardening report

## Executive summary

When running tools that rely on the FreeDesktop Secret Service API (such as Google Antigravity `agy` via `go-keyring`), access to the session D-Bus is required to retrieve stored credentials. However, mounting the host session bus (`/run/user/<UID>/bus`) directly into the sandbox exposes the host's `systemd --user` manager (`org.freedesktop.systemd1`), allowing sandboxed processes to trigger arbitrary host code execution via transient units (`StartTransientUnit`).

To remediate this escape vector while enabling credential access for `agy` and other AI coding assistants, `bws` now integrates an ephemeral, filtered D-Bus proxy via `xdg-dbus-proxy`.

---

## Threat model and breakout analysis

Directly binding the host user D-Bus socket introduces three critical vulnerabilities:

1. **Host execution escape (`org.freedesktop.systemd1`)**:
   - Sandboxed code can issue RPC calls to `org.freedesktop.systemd1.Manager.StartTransientUnit`.
   - The host systemd daemon executes the requested binary outside the Bubblewrap mount, user, and PID namespaces with full host user permissions.
2. **Indiscriminate credential exposure (`org.freedesktop.secrets`)**:
   - Binding the raw socket without filtering allows access to all stored passwords and keys in GNOME Keyring or KWallet without restriction.
3. **Session snooping and IPC interference**:
   - Access to notification servers, clipboard managers, and desktop application launchers over the shared session bus.

---

## Architectural solution: Filtered D-Bus proxy

### 1. Ephemeral proxy lifecycle (`internal/dbus`)

The new `internal/dbus` package manages the lifecycle of `xdg-dbus-proxy`:

- **Discovery**: Automatically detects the host session D-Bus socket via `$DBUS_SESSION_BUS_ADDRESS` or `/run/user/<UID>/bus`.
- **Filtering**: Launches `xdg-dbus-proxy` pointing to the host bus with `--filter` and explicit `--talk` policies:
  ```bash
  xdg-dbus-proxy unix:path=/run/user/<UID>/bus /tmp/bws/dbus_proxy_<ID>/bus \
    --filter \
    --talk=org.freedesktop.secrets
  ```
- **Socket mount**: Binds the filtered proxy socket into the sandbox destination path (`/run/user/<UID>/bus`), setting `DBUS_SESSION_BUS_ADDRESS` and `XDG_RUNTIME_DIR`.
- **Cleanup**: Ensures the proxy daemon and temporary directory are terminated and unlinked on exit or signal interruption (`SIGINT`, `SIGTERM`, `SIGHUP`).

### 2. Profile and configuration integration

- **Profile features**: Added `features` support to `Profile` and `ResolvedProfile` in `internal/profile/profile.go`. The `agy` profile (`profiles/agy.json`) enables `enable_proxy` and `enable_dbus`.
- **Configuration merge**: Merges `features.enable_dbus`, `features.dbus_talk`, and `features.allow_raw_dbus` across global, local, and profile configurations (`internal/config/merge.go`).
- **CLI flags**:
  - `--dbus`: Enable filtered D-Bus proxy access.
  - `--no-dbus`: Explicitly suppress D-Bus proxy access.

### 3. Fail-safe security defaults

- If `enable_dbus` is active but `xdg-dbus-proxy` is not installed on the host, D-Bus access is disabled by default with a warning to prevent accidental container escapes.
- Users may explicitly opt into raw socket binding only by setting `features.allow_raw_dbus: true`, which emits an explicit security alert.

---

## Verification and test suite

All verification suites pass cleanly:

1. **Unit tests (`internal/dbus`)**:
   - Session address and socket path extraction.
   - Sandbox destination path formatting.
   - Proxy configuration and talk policy generation.
   - Graceful handling when socket is absent.
2. **Integration tests (`main_dbus_test.go`)**:
   - `TestCLIDBusFlagsAndPlan`: Verifies default D-Bus suppression, `--dbus` plan resolution, and `--no-dbus` override behavior.
   - `TestCLIAgyProfileDBus`: Verifies that initializing a workspace with the `agy` profile enables D-Bus proxy binding.
3. **Static analysis and audit**:
   - `go vet ./...` completed with zero warnings.
   - `tools/verify_build.sh` completed successfully (`go fmt`, `go vet`, `go test ./...`, `go build`).
   - `tools/audit_lines.sh` confirmed all Go source files adhere to the 500-line limit.
