# D-Bus integration, security analysis, and remediation

## Overview

When running Google Antigravity (`agy`) inside a `bws` sandbox, `agy` requires access to saved OAuth authentication credentials. On Linux desktop environments, `agy` relies on `github.com/zalando/go-keyring`, which connects to the Secret Service daemon (`gnome-keyring-daemon` or `ksecretsservice`) over the user session D-Bus.

Without session D-Bus access, `agy` fails to find saved tokens and prompts for interactive browser authentication.

---

## Root cause analysis

1. **Credential storage mechanism**:
   - `agy` stores and retrieves OAuth tokens via the FreeDesktop Secret Service API.
   - On Linux, this communication takes place over the user session bus located at `/run/user/<UID>/bus` (or `$DBUS_SESSION_BUS_ADDRESS`).

2. **Sandbox isolation behavior**:
   - By default, `bws` isolates IPC and does not mount the host user's runtime directory (`/run/user/<UID>`).
   - When the session bus socket is unavailable inside the container, Secret Service lookups fail immediately, causing `agy` to enter the OAuth login flow.

---

## Security model and escape vector analysis

Binding the raw host session D-Bus socket (`/run/user/<UID>/bus`) directly into the sandbox introduces critical security risks that violate the `bws` isolation perimeter:

### 1. Host execution escape via `systemd --user`

- The user session bus connects directly to the host's `systemd --user` daemon (`org.freedesktop.systemd1`).
- Any process inside the sandbox can invoke the D-Bus method `org.freedesktop.systemd1.Manager.StartTransientUnit`.
- This instructs the host `systemd --user` instance (running outside the Bubblewrap sandbox in the host mount, user, and PID namespaces) to execute arbitrary binaries with the host user's full privileges.
- This represents a complete container breakout.

### 2. Full credential dumping via `org.freedesktop.secrets`

- Any sandboxed script, dependency build hook, or untrusted package can query the Secret Service interface and dump all passwords, personal access tokens, and stored keys in the user's GNOME Keyring or KWallet.
- This bypasses the protections provided by the `no-secrets` profile.

### 3. Desktop session interception

- The session bus exposes interfaces for notification servers, clipboard managers, application launchers (`org.freedesktop.Application`), and screen inhibition.

---

## Current status in `bws`

The following changes were implemented to enable `agy` execution:

1. **Configuration**: Added `EnableDBus` (`features.enable_dbus`) to `FeaturesConfig` in `internal/config/config.go`, with alias support in `internal/config/kv.go` and merge handling in `internal/config/merge.go`.
2. **Mount logic**: Implemented `addDBusArgs` in `internal/bwrap/helpers.go` to discover the session socket (`$DBUS_SESSION_BUS_ADDRESS` or `/run/user/<UID>/bus`), bind it, and set `DBUS_SESSION_BUS_ADDRESS` and `XDG_RUNTIME_DIR`.
3. **Profiles**: Updated `profiles/agy.json` and `internal/profile/embedded_profiles/agy.json` to pass `DBUS_SESSION_BUS_ADDRESS` and `XDG_RUNTIME_DIR`, and mount `~/.config/antigravity-usage`.

---

## Remediation strategies

To support tools requiring Secret Service access without exposing dangerous host D-Bus interfaces, the following approaches are available:

### Strategy A: Filtered D-Bus proxy via `xdg-dbus-proxy` (Recommended)

Use `xdg-dbus-proxy` to create an isolated, filtered proxy socket:

```bash
xdg-dbus-proxy unix:path=/run/user/$UID/bus /tmp/bws/dbus-proxy-$ID \
  --filter \
  --talk=org.freedesktop.secrets
```

**Benefits**:
- Permits communication only with `org.freedesktop.secrets`.
- Explicitly blocks `org.freedesktop.systemd1` and other dangerous system interfaces.
- Prevents transient unit execution and sandbox escapes.

### Strategy B: Strict opt-in feature flag

- Set `enable_dbus: false` by default in default configuration templates and base profiles.
- Require users or profiles to explicitly opt into raw D-Bus socket access when necessary, documenting the associated escape risks similarly to the `docker` profile.

### Strategy C: Environment-based credentials

Configure `agy` to authenticate via environment variables instead of session D-Bus:
- `GEMINI_API_KEY`
- `GOOGLE_API_KEY`
- `GOOGLE_APPLICATION_CREDENTIALS` (Service account JSON key file)

These variables are passed through `pass_env` in `profiles/agy.json` and do not require D-Bus access.
