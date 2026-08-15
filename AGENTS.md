# bw — Bubblewrap Sandbox Launcher

## Project

A Ruby script (`bw`) that wraps `bwrap` to launch a secure sandbox with persistent home (`~/.sandbox/pi_generic/`), configurable mounts, env vars, SSH forwarding, X11 support, and shell theming (oh-my-posh).

## Key Files

| File | Purpose |
|---|---|
| `bw` | Main script (Ruby, ~1280 lines) |
| `~/.config/bw/config.jsonc` | Global config (JSONC with `//` comments) |
| `.bw.jsonc` | Per-directory config override |
| `issues_to_fix.md` | WSL clipboard issue documentation |
| `CHANGELOG.md` | Change history |
| `GEMINI.md` | Architecture notes |

## Syntax & Validation

```bash
ruby -c bw                                          # Check Ruby syntax
ruby -rjson -e 'JSON.parse(File.read("...").gsub(/^\s*(\/\/|#).*$/,""))'  # Check JSONC validity
```

## Testing

```bash
bw                                                  # Launch interactive tmux session
bw bash                                             # Run bash inside sandbox
bw pdflatex --version                               # Test pdflatex
bw lualatex --version                               # Test lualatex
bw --info                                           # Show all bindings and config
bw -test opencode                                   # Test opencode loads
bw -test quarto                                     # Test quarto loads
bw -test uv                                         # Test uv and uvx load
```

## Config Structure

```jsonc
{
  "system":     { "share_net", "clearenv", "unshare_uts", "hostname" },
  "features":   { "enable_ssh", "enable_x11", "enable_etc_auto_bind" },
  "env":        { "HOME", "TERM", "LANG", "LC_ALL", "USER", "LOGNAME", "SHELL" },
  "path":       [ "/home/sariel/bin", "/home/sariel/.local/bin", ... ],
  "binds_rw":   [ ["host_path", "sandbox_path"], ... ],
  "binds_ro":   [ ["host_path", "sandbox_path"], ... ],
  "copy":       [ "/absolute/path/to/program", ... ]
}
```

## LaTeX/Font Bindings (added 2026-08-10)

| Path | Type | Purpose |
|---|---|---|
| `/var/lib/texmf` | ro | Format files (pdflatex.fmt, lualatex.fmt), font maps, ls-R |
| `/var/cache/fontconfig` | ro | Fontconfig cache (performance) |
| `~/.texlive2026/texmf-var` | rw | luaotfload font cache (writes) |
| `~/.local/share/fonts` | rw | User-installed fonts |
| `~/.cache/fontconfig` | rw | Fontconfig cache (writes) |

## Python / UV Bindings (added 2026-08-14)

| Path | Type | Purpose |
|---|---|---|
| `~/.local/bin` | rw | Binaries including `uv`, `uvx`, and installed tools |
| `~/.local/share/uv` | rw | UV python versions and tools |
| `~/.cache/uv` | rw | UV package/wheel cache and uvx environments |
| `~/.config/uv` | rw | UV user configuration |

## Key Behaviors

- `--tmpfs /etc` then overlays `/etc/*` (except resolv.conf) via `enable_etc_auto_bind`
- Unique `/tmp/bw/sandbox_XXXXXX` per instance, cleaned up automatically
- Blocks running from `~/` or `~/bin/` or dirs with >100 files (override with `-f`)
- Copy management: `bw copy add <path>`, `bw copy list`, `bw copy del <path>`

## Common Issues

1. **AppArmor / userns**: Run `bw` once; it detects and offers to fix via `sudo`
2. **WSL clipboard**: See `issues_to_fix.md` — needs `/init` + `/run/WSL` binds + `WSL_INTEROP` env
3. **Config changes**: Edit `~/.config/bw/config.jsonc` directly (no restart needed)

## Subcommands

- `bw copy add <absolute-path>` — add program to copy list
- `bw copy list` — list copied programs
- `bw copy del <absolute-path>` — remove from copy list
- `bw -test opencode` — verify opencode loads
- `bw -test quarto` — verify quarto loads
- `bw -test uv` — verify uv and uvx load
- `bw --info` — print all bindings and exit