# `bws` capability profiles catalog

This directory contains the declarative **capability profiles** for `bws` (Bubblewrap Sandbox).
Profiles modularize bind mounts, environment variables, path additions, security masks, and verification smoke tests.

---

## Table of contents

* [Overview & resolution](#overview--resolution)
* [Meta-profiles & developer stacks](#1-meta-profiles--developer-stacks)
* [Language & compiler toolchains](#2-language--compiler-toolchains)
* [AI agent & coding assistant profiles](#3-ai-agent--coding-assistant-profiles)
* [Security & hardening profiles](#4-security--hardening-profiles)
* [Developer tools & utilities](#5-developer-tools--utilities)
* [Profile JSON schema specification](#profile-json-schema-specification)
* [Authoring & testing profiles](#authoring--testing-profiles)

---

## Overview & resolution

When a profile is activated (via `"profiles": ["go-dev"]` in `~/.config/bws/config.jsonc` or `.bws/config.jsonc`), `bws`:
1. **Topologically resolves dependencies** declared in `requires`.
2. **Merges read-write and read-only bind mounts** without duplicate entries.
3. **Prepends PATH entries** to ensure tools are immediately executable.
4. **Applies path masks** (`tmpfs` / `/dev/null`) to block unauthorized host tools.
5. **Extracts safe pass-through environment variables** (`pass_env`).

---

### Security & isolation profiles

`bws` provides dedicated security profiles to isolate sensitive host data:

| Profile | Protects | Targets Blocked |
| :--- | :--- | :--- |
| **`no-sudo`** | Root privilege escalation | `sudo`, `su`, `pkexec`, `doas`, `/etc/sudoers` |
| **`no-ssh`** | SSH host credentials | `~/.ssh`, `/etc/ssh/ssh_config` |
| **`no-browser`** | Web sessions & passwords | Firefox, Chrome, Chromium, Brave, Edge |
| **`no-email`** | Local email stores | Thunderbird, Evolution, Mutt, Maildir |
| **`no-chat`** | Messaging databases | Discord, Slack, Signal, Telegram |
| **`no-secrets`** | Cloud provider keys | `~/.aws`, `~/.azure`, `~/.config/gcloud`, `~/.gnupg` |
| **`no-history`**| Command line logs | `.bash_history`, `.zsh_history`, `.python_history` |
| **`offline`** | Network isolation | Blocks outbound internet and isolates loopback (host `127.0.0.1` unreachable) |
| **`secure-agent`**| All of the above combined | Hardened environment for autonomous AI agents |

---

## 1. Meta-profiles & developer stacks

Meta-profiles aggregate individual tools into unified, full-stack developer environments:

### `editor`
**Description**: Default text editor (override in `~/.config/bws/profiles/editor.json` to choose `emacs`, `neovim`, or `vim`).

**Requires**: `emacs`

*Example `~/.config/bws/profiles/editor.json` to use Neovim:*
```json
{
  "name": "editor",
  "description": "Default text editor (Neovim)",
  "requires": ["neovim"],
  "env": {
    "EDITOR": "nvim",
    "VISUAL": "nvim"
  }
}
```

### `go-dev`
**Description**: Full Go developer environment (Go toolchain, Gopls LSP, Git, GitHub CLI, Editor)

**Requires**: `go`, `gopls`, `git`, `gh`, `editor`

### `latex-dev`
**Description**: Complete LaTeX authoring environment (alias for latex-use)

**Requires**: `latex-use`

### `latex-use`
**Description**: Complete LaTeX authoring environment (TeX Live, Pandoc, Editor)

**Requires**: `latex`, `pandoc`, `editor`

### `python-dev`
**Description**: Full Python developer environment (Python3, UV, Git, GitHub CLI, Editor)

**Requires**: `python`, `uv`, `git`, `gh`, `editor`

### `rust-dev`
**Description**: Full Rust developer environment (Rustc, Cargo, Git, GitHub CLI, Editor)

**Requires**: `rust`, `git`, `gh`, `editor`

---

## 2. Language & compiler toolchains

### `go`
**Description**: Go programming language toolchain and module cache

**PATH additions**: `@@HOME@@/.go/bin`

**Pass-through variables**: `GOPROXY`, `GONOSUMDB`, `GOFLAGS`

**Environment**:
- `GOPATH=@@HOME@@/.go`

**Read-write binds**:
- `~/.cache/go-build -> @@HOME@@/.cache/go-build`
- `~/.config/go -> @@HOME@@/.config/go`
- `~/.go -> @@HOME@@/.go`

**Verification tests**:
- `Go binary version`: `go version`
- `Go compilation & run`: `bash -c echo "package main; import (\"fmt\"); func main(){ fmt.Println(\"Go OK\") }" > /tmp/bw_hello.go && go run /tmp/bw_hello.go && rm -f /tmp/bw_hello.go`

### `gopls`
**Description**: Official Go Language Server (LSP) daemon

**Requires**: `go`

**PATH additions**: `@@HOME@@/.go/bin`, `@@HOME@@/bin`, `@@HOME@@/.local/bin`

**Read-write binds**:
- `~/.go -> @@HOME@@/.go`
- `~/.cache/gopls -> @@HOME@@/.cache/gopls`

**Verification tests**:
- `Go language server version`: `gopls version`

### `latex`
**Description**: TeX Live and LaTeX typesetting environment

**Read-write binds**:
- `~/.texlive2026/texmf-var -> @@HOME@@/.texlive2026/texmf-var`
- `~/.texlive2026/texmf-config -> @@HOME@@/.texlive2026/texmf-config`
- `~/.local/share/fonts -> @@HOME@@/.local/share/fonts`
- `~/.cache/fontconfig -> @@HOME@@/.cache/fontconfig`

**Read-only binds**:
- `/var/lib/texmf -> /var/lib/texmf`
- `/var/cache/fontconfig -> /var/cache/fontconfig`

**Verification tests**:
- `pdfLaTeX binary`: `pdflatex --version`
- `pdfLaTeX compilation`: `pdflatex -interaction=batchmode -output-directory=/tmp \documentclass{article}\begin{document}Hello\end{document}`
- `XeLaTeX binary`: `xelatex --version`
- `LuaLaTeX binary`: `lualatex --version`
- `Biber binary`: `biber --version`
- `BibTeX binary`: `bibtex --version`
- `Latexmk binary`: `latexmk --version`

### `node`
**Description**: Node.js JavaScript runtime, npm, pnpm, and yarn caches

**Read-write binds**:
- `~/.npm -> @@HOME@@/.npm`
- `~/.cache/yarn -> @@HOME@@/.cache/yarn`
- `~/.local/share/pnpm -> @@HOME@@/.local/share/pnpm`

**Verification tests**:
- `Node.js version`: `node --version`
- `Node.js evaluation`: `node -e console.log("Node OK:", process.version)`

### `pandoc`
**Description**: Universal markup document converter

**Verification tests**:
- `Pandoc version`: `pandoc --version`
- `Pandoc markdown conversion`: `pandoc -f markdown -t html -o /dev/null`

### `python`
**Description**: Python runtime, pip, and uv package cache

**Pass-through variables**: `PYTHONUNBUFFERED`, `PYTHONDONTWRITEBYTECODE`

**Read-write binds**:
- `~/.cache/pip -> @@HOME@@/.cache/pip`
- `~/.cache/uv -> @@HOME@@/.cache/uv`
- `~/.cache/pypoetry -> @@HOME@@/.cache/pypoetry`

**Verification tests**:
- `Python3 version`: `python3 --version`
- `Python3 execution`: `python3 -c import sys; print("Python OK: " + sys.version.split()[0])`

### `quarto`
**Description**: Quarto scientific and technical publishing system

**Verification tests**:
- `Quarto binary version`: `quarto --version`
- `Quarto check`: `quarto check`

### `rust`
**Description**: Rust language compiler and Cargo package manager

**PATH additions**: `@@HOME@@/.cargo/bin`

**Pass-through variables**: `RUST_BACKTRACE`, `RUST_LOG`

**Environment**:
- `CARGO_HOME=@@HOME@@/.cargo`
- `RUSTUP_HOME=@@HOME@@/.rustup`

**Read-write binds**:
- `~/.cargo -> @@HOME@@/.cargo`

**Read-only binds**:
- `~/.rustup -> @@HOME@@/.rustup`

**Verification tests**:
- `Rustc version`: `rustc --version`
- `Cargo version`: `cargo --version`

### `uv`
**Description**: Fast Python package and project manager written in Rust

**Requires**: `python`

**PATH additions**: `@@HOME@@/.local/bin`, `@@HOME@@/.cargo/bin`

**Read-write binds**:
- `~/.cache/uv -> @@HOME@@/.cache/uv`

**Verification tests**:
- `uv binary version`: `uv --version`

---

## 3. AI agent & coding assistant profiles

### `ai`
**Description**: AI Coding Assistant stack (Antigravity CLI, OpenCode, oc switcher, Claude Code)

**Requires**: `opencode`, `oc`, `agy`, `claude`, `no-sudo`

### `antigravity`
**Description**: Google Antigravity CLI and coding agent framework

**Read-write binds**:
- `~/.gemini -> @@HOME@@/.gemini`
- `~/.config/antigravity -> @@HOME@@/.config/antigravity`

**Verification tests**:
- `Antigravity CLI version`: `agy --version`

### `claude`
**Description**: Anthropic Claude Code CLI assistant, runtime, and caches

**Aliases**: `claude-code`, `anthropic`

**PATH additions**: `@@HOME@@/.local/bin`, `@@HOME@@/bin`, `@@HOME@@/.local/share/claude/versions`

**Pass-through variables**: `ANTHROPIC_API_KEY`, `ANTHROPIC_AUTH_TOKEN`, `ANTHROPIC_BASE_URL`, `ANTHROPIC_MODEL`, `ANTHROPIC_SMALL_MODEL`, `ANTHROPIC_*`, `CLAUDE_*`

**Read-write binds**:
- `~/.claude -> @@HOME@@/.claude`
- `~/.claude.json -> @@HOME@@/.claude.json`
- `~/.config/claude -> @@HOME@@/.config/claude`
- `~/.local/share/claude -> @@HOME@@/.local/share/claude`
- `~/.local/state/claude -> @@HOME@@/.local/state/claude`
- `~/.cache/claude -> @@HOME@@/.cache/claude`

**Verification tests**:
- `Claude Code version`: `claude --version`
- `Claude Code doctor check`: `claude doctor`

### `oc`
**Description**: OpenCode profile switcher CLI and configuration

**Requires**: `opencode`

**Read-write binds**:
- `~/.config/open-switcher -> @@HOME@@/.config/open-switcher`

**Verification tests**:
- `oc binary version`: `oc --version`

### `opencode`
**Description**: OpenCode AI coding assistant runtime and caches

**Read-write binds**:
- `~/.opencode -> @@HOME@@/.opencode`
- `~/.config/opencode -> @@HOME@@/.config/opencode`
- `~/.config/opencode-switcher -> @@HOME@@/.config/opencode-switcher`
- `~/.local/share/opencode -> @@HOME@@/.local/share/opencode`
- `~/.local/state/opencode -> @@HOME@@/.local/state/opencode`
- `~/.cache/opencode -> @@HOME@@/.cache/opencode`

**Verification tests**:
- `OpenCode version`: `bash -c opencode --version`
- `OpenCode paths inspection`: `bash -c opencode debug paths`

### `secure-agent`
**Description**: Hardened AI agent environment with complete isolation from host secrets, browser data, and history

**Requires**: `ai`, `no-sudo`, `no-ssh`, `no-browser`, `no-email`, `no-secrets`, `no-history`

---

## 4. Security & hardening profiles

Hardening profiles implement zero-trust path masking via `/dev/null` overlays and ephemeral `tmpfs` mounts:

### `mask-sudo`
**Description**: Mask privilege escalation and superuser binaries (alias for no-sudo)

**Requires**: `no-sudo`

### `no-browser`
**Description**: Mask web browser profiles, saved passwords, and session cookies

**Masked / blocked paths**:
- ⊘ `~/.mozilla`
- ⊘ `~/.cache/mozilla`
- ⊘ `~/.config/google-chrome`
- ⊘ `~/.cache/google-chrome`
- ⊘ `~/.config/chromium`
- ⊘ `~/.cache/chromium`
- ⊘ `~/.config/BraveSoftware`
- ⊘ `~/.config/microsoft-edge`

**Verification tests**:
- `mozilla profile is masked`: `bash -c ! test -e ~/.mozilla/firefox/profiles.ini`

### `no-chat`
**Description**: Mask messaging apps and communication databases

**Masked / blocked paths**:
- ⊘ `~/.config/discord`
- ⊘ `~/.config/Slack`
- ⊘ `~/.config/Signal`
- ⊘ `~/.local/share/TelegramDesktop`
- ⊘ `~/.config/Element`

**Verification tests**:
- `chat directories are masked`: `bash -c ! test -s ~/.config/Slack && ! test -s ~/.config/discord`

### `no-email`
**Description**: Mask desktop email clients, offline mailboxes, and mail credentials

**Masked / blocked paths**:
- ⊘ `~/.thunderbird`
- ⊘ `~/.cache/thunderbird`
- ⊘ `~/.config/evolution`
- ⊘ `~/.local/share/evolution`
- ⊘ `~/.mutt`
- ⊘ `~/.muttrc`
- ⊘ `~/.config/aerc`
- ⊘ `~/Maildir`
- ⊘ `~/Mail`
- ⊘ `~/.mail`

**Verification tests**:
- `thunderbird mailbox is masked`: `bash -c ! test -e ~/.thunderbird/profiles.ini`

### `no-history`
**Description**: Mask shell and REPL command history files containing potential secrets (enabled by default)

**Masked / blocked paths**:
- ⊘ `~/.bash_history`
- ⊘ `~/.zsh_history`, `~/.zhistory`, `~/.histfile`
- ⊘ `~/.sh_history`, `~/.ash_history`, `~/.history`
- ⊘ `~/.fish_history`, `~/.local/share/fish/fish_history`, `~/.config/fish/fish_history`
- ⊘ `~/.local/state/bash/history`, `~/.local/share/zsh/history`
- ⊘ `~/.config/nushell/history.txt`, `~/.local/share/nushell/history.txt`
- ⊘ `~/.local/share/powershell/PSReadLine/ConsoleHost_history.txt`
- ⊘ `~/.python_history`, `~/.local/state/python/history`
- ⊘ `~/.node_repl_history`
- ⊘ `~/.irb_history`, `~/.pry_history`
- ⊘ `~/.psql_history`, `~/.mysql_history`, `~/.sqlite_history`, `~/.rediscli_history`, `~/.dbshell`
- ⊘ `~/.julia_history`, `~/.Rhistory`, `~/.php_history`
- ⊘ `~/.ghci_history`, `~/.erlang_history`, `~/.iex_history`, `~/.lua_history`
- ⊘ `~/.lesshst`, `~/.local/state/lesshst`, `~/.nano_history`, `~/.viminfo`

**Verification tests**:
- `shell history is masked`: `bash -c ! test -s ~/.bash_history && ! test -s ~/.zsh_history`

### `no-secrets`
**Description**: Mask cloud provider credentials, GPG keyrings, and password stores

**Masked / blocked paths**:
- ⊘ `~/.aws`
- ⊘ `~/.azure`
- ⊘ `~/.config/gcloud`
- ⊘ `~/.password-store`
- ⊘ `~/.gnupg`
- ⊘ `~/.vault-token`

**Verification tests**:
- `cloud and gpg secrets are masked`: `bash -c ! test -e ~/.aws/credentials && ! test -e ~/.gnupg/secring.gpg`

### `no-ssh`
**Description**: Block all SSH access, configuration, and host keys

**Masked / blocked paths**:
- ⊘ `~/.ssh`
- ⊘ `@@HOME@@/.ssh`
- ⊘ `/etc/ssh/ssh_config`
- ⊘ `/etc/ssh/ssh_config.d`

**Verification tests**:
- `ssh config is masked and empty`: `bash -c ! test -s ~/.ssh/config && ! test -s ~/.ssh/id_rsa`

### `no-sudo`
**Description**: Mask privilege escalation binaries, superuser tools, and sudoers configuration

**Masked / blocked paths**:
- ⊘ `/usr/bin/sudo`
- ⊘ `/bin/sudo`
- ⊘ `/usr/bin/su`
- ⊘ `/bin/su`
- ⊘ `/usr/bin/pkexec`
- ⊘ `/bin/pkexec`
- ⊘ `/usr/bin/doas`
- ⊘ `/bin/doas`
- ⊘ `/usr/bin/gpasswd`
- ⊘ `/bin/gpasswd`
- ⊘ `/usr/bin/newgrp`
- ⊘ `/bin/newgrp`
- ⊘ `/etc/sudoers`
- ⊘ `/etc/sudoers.d`

**Verification tests**:
- `sudo binary is blocked`: `bash -c ! sudo 2>/dev/null`
- `su binary is blocked`: `bash -c ! su 2>/dev/null`
- `pkexec binary is blocked`: `bash -c ! pkexec 2>/dev/null`
- `doas binary is blocked`: `bash -c ! doas 2>/dev/null`
- `gpasswd binary is blocked`: `bash -c ! gpasswd 2>/dev/null`
- `newgrp binary is blocked`: `bash -c ! newgrp 2>/dev/null`
- `sudoers file is masked`: `bash -c ! test -s /etc/sudoers`

---

### `offline`
**Description**: Completely isolate network namespace (blocks internet and host 127.0.0.1 services)

**Aliases**: `no-net`

**Unshares**: Network namespace (`CLONE_NEWNET`)

**Verification tests**:
- `Outbound network connection is blocked`: `bash -c '! curl -s --connect-timeout 1 https://google.com >/dev/null 2>&1'`
- `Private loopback is functional`: `bash -c 'ping -c 1 127.0.0.1 >/dev/null 2>&1 || true'`

---

## 5. Developer tools & utilities

### `docker`
**Description**: Docker container CLI and daemon socket integration

**Read-write binds**:
- `~/.docker -> @@HOME@@/.docker`
- `/var/run/docker.sock -> /var/run/docker.sock`
- `/run/user/1000/docker.sock -> /run/user/1000/docker.sock`

**Verification tests**:
- `Docker CLI version`: `docker --version`
- `Docker daemon connectivity`: `docker info`

### `emacs`
**Description**: GNU Emacs extensible text editor and Lisp environment

**Read-write binds**:
- `~/.config/emacs -> @@HOME@@/.config/emacs`
- `~/.emacs.d -> @@HOME@@/.emacs.d`
- `~/.cache/emacs -> @@HOME@@/.cache/emacs`
- `~/.doom.d -> @@HOME@@/.doom.d`
- `~/.config/doom -> @@HOME@@/.config/doom`

**Read-only binds**:
- `~/.emacs -> @@HOME@@/.emacs`

**Verification tests**:
- `Emacs version`: `emacs --version`
- `Emacs batch Lisp evaluation`: `emacs --batch --eval (message "Emacs Lisp OK: %s" emacs-version)`

### `gh`
**Description**: GitHub official command-line interface

**Read-write binds**:
- `~/.config/gh -> @@HOME@@/.config/gh`
- `~/.local/share/gh -> @@HOME@@/.local/share/gh`

**Verification tests**:
- `GitHub CLI version`: `gh --version`

### `git`
**Description**: Git distributed version control system

**Read-only binds**:
- `~/.gitconfig -> @@HOME@@/.gitconfig`
- `~/.git-credentials -> @@HOME@@/.git-credentials`

**Verification tests**:
- `Git binary version`: `git --version`

### `jq`
**Description**: Lightweight and flexible command-line JSON processor

**Verification tests**:
- `jq version`: `jq --version`
- `jq filter evaluation`: `jq -n {"status":"ok"} | .status`

### `neovim`
**Description**: Vim-fork focused on extensibility and usability

**Read-write binds**:
- `~/.config/nvim -> @@HOME@@/.config/nvim`
- `~/.local/share/nvim -> @@HOME@@/.local/share/nvim`
- `~/.local/state/nvim -> @@HOME@@/.local/state/nvim`
- `~/.cache/nvim -> @@HOME@@/.cache/nvim`

**Verification tests**:
- `Neovim version`: `nvim --version`

### `tmux`
**Description**: Terminal multiplexer configuration and sockets

**Read-write binds**:
- `~/.tmux.conf -> @@HOME@@/.tmux.conf`

**Verification tests**:
- `tmux version`: `tmux -V`

---

## Profile JSON schema specification

A valid `bws` profile JSON file accepts the following fields:

```jsonc
{
  "name": "profile-name",
  "description": "Human readable description",
  "requires": ["dependency-profile-1", "dependency-profile-2"],
  "path": ["@@HOME@@/bin", "@@HOME@@/.local/bin"],
  "pass_env": ["SAFE_VAR_1", "SAFE_VAR_2"],
  "env": {
    "KEY": "VALUE",
    "ALIAS": "$KEY"
  },
  "binds_rw": [
    ["~/.cache/mytool", "@@HOME@@/.cache/mytool"]
  ],
  "binds_ro": [
    ["~/.config/mytool", "@@HOME@@/.config/mytool"]
  ],
  "mask": [
    "/usr/bin/unwanted_binary",
    "~/.sensitive_dir"
  ],
  "detect": {
    "files": ["mytool.config", ".mytoolrc"]
  },
  "tests": [
    {
      "name": "Verify tool version",
      "cmd": ["mytool", "--version"],
      "type": "version"
    }
  ]
}
```

---

## Authoring & testing profiles

### 1. Generate from Homebrew & Firejail
```bash
bws profile new <name>
```

### 2. Test in sandbox
```bash
bws test <name>
```

### 3. Inspect plan
```bash
bws profile show <name>
```

