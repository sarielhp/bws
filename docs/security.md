# Security & path masking engine

`bws` is designed for unprivileged, hermetic developer environments and zero-trust autonomous AI coding agent execution.

---

## Table of contents

* [Threat model & security guarantees](#threat-model--security-guarantees)
* [Path masking mechanics](#path-masking-mechanics)
* [Built-in security & hardening profiles](#built-in-security--hardening-profiles)
* [Automatic `.bws/` workspace protection](#automatic-bws-workspace-protection)

---

## Threat model & security guarantees

1. **No root permissions**: Bubblewrap runs strictly in unprivileged Linux user namespaces (`CLONE_NEWUSER`).
2. **Hermetic `$HOME`**: The host user's personal home directory is completely unmapped.
3. **Environment sanitization**: Host environment variables are not leaked unless explicitly listed in `pass_env`.
4. **Child process termination**: All spawned sandbox processes terminate automatically when `bws` exits (`--die-with-parent`).

---

## Path masking mechanics

`bws` implements zero-trust **path masking** using two non-destructive overlay primitives:

* **Directories (`--tmpfs <path>`)**: Overlays an empty, ephemeral in-memory `tmpfs` over the directory. Any reads see an empty folder; any writes exist only in memory and disappear on session exit.
* **Files & binaries (`--ro-bind-try /dev/null <path>`)**: Overlays `/dev/null` over the file. Any execution or read attempts fail immediately (0-byte file, unexecutable).

---

## Built-in security & hardening profiles

| Profile | Target Paths | Protection Provided |
| :--- | :--- | :--- |
| **`no-sudo`** | `sudo`, `su`, `pkexec`, `doas`, `gpasswd`, `newgrp`, `/etc/sudoers` | Prevents privilege escalation and superuser execution |
| **`no-ssh`** | `~/.ssh`, `/etc/ssh/ssh_config` | Blocks access to host private keys and SSH configuration |
| **`no-browser`** | Firefox, Chrome, Chromium, Brave, Edge profiles | Protects saved passwords, web sessions, and browser cookies |
| **`no-email`** | Thunderbird, Evolution, Mutt, Maildir | Shields desktop mailboxes and email credentials |
| **`no-chat`** | Discord, Slack, Signal, Telegram, Element | Protects messaging databases and tokens |
| **`no-secrets`** | `~/.aws`, `~/.azure`, `~/.config/gcloud`, `~/.password-store`, `~/.gnupg` | Shields cloud provider credentials and GPG keys |
| **`no-history`**| `.bash_history`, `.zsh_history`, `.python_history`, `.psql_history` | Prevents scanning command histories for leaked tokens |
| **`secure-agent`**| All of the above combined + `ai` coding assistant stack | Zero-trust developer sandbox for autonomous coding agents |

---

## Automatic `.bws/` workspace protection

When `bws` launches inside a workspace, the local `.bws/` directory and `.bws.jsonc` file are **automatically masked by default** inside the sandbox:
* Code running inside the bubble cannot inspect host sandbox configuration.
* Untrusted build scripts or autonomous agents cannot modify `.bws/config.jsonc` to weaken sandbox rules on future host invocations.
