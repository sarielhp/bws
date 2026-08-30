# `bws` project roadmap & to-do list

---

## 1. AI agent & CLI tool capability profiles

Add first-class capability profiles to `profiles/` with appropriate bind mounts, cache persistence, environment pass-through, and automated smoke test definitions (`bws test <profile>`):

* [ ] **Anthropic Claude Code (`claude`)**
  - Path: `~/.claude/`, `~/.claude.json`, `~/.config/claude/`
  - Cache: `~/.cache/claude/`
  - Smoke test: `claude --version`
* [ ] **Aider AI pair programmer (`aider`)**
  - Path: `~/.aider/`, `~/.aider.conf.yml`
  - Cache: `~/.aider.tags.cache.v3`, `~/.cache/aider/` (repository tree-sitter tag caches)
  - Smoke test: `aider --version`
* [ ] **Simon Willison's LLM CLI (`llm`)**
  - Path: `~/.config/io.datasette.llm/` (config, plugin directories, prompt history SQLite databases)
  - Environment: `OPENAI_API_KEY`, `ANTHROPIC_API_KEY`, `GEMINI_API_KEY` pass-through options
  - Smoke test: `llm --version`
* [ ] **Shell-GPT (`sgpt`)**
  - Path: `~/.config/shell_gpt/` (history and cache)
  - Smoke test: `sgpt --version`
* [ ] **GitHub Copilot CLI (`copilot` / `gh copilot`)**
  - Path: `~/.config/github-copilot/`, `~/.config/gh/`
* [ ] **Update composite profiles**:
  - Update `profiles/ai.json` and `profiles/secure-agent.json` to include newly added AI tools.

---

## 2. Security hardening & sandbox refinements

Follow-up items from the security and penetration audit ([`reviews/sandbox_escape_audit.md`](reviews/sandbox_escape_audit.md)):

* [ ] **Syscall filtering (Seccomp BPF)**:
  - Investigate attaching a minimal unprivileged BPF filter to restrict potentially exploitable kernel interfaces (`io_uring`, unprivileged `bpf`, `userfaultfd`).
* [ ] **WSL2 interop isolation**:
  - Refine `/run/WSL` handling on WSL2 to prevent calling host Windows binaries unless explicitly enabled (`enable_wsl: true`).
* [ ] **Network loopback fine-grained control**:
  - Support blocking host `127.0.0.1` services while keeping outbound internet access (or vice versa) via enhanced proxy routing rules.

---

## 3. Git workflow (`bws gw`) enhancements

* [ ] **Untracked dependency cache sharing**:
  - Add optional `--share-cache` flag to symlink or bind-mount untracked dependencies (such as `node_modules`, `.venv`, `target/`) into the disposable clone to avoid re-downloading packages in clean workspaces.
* [ ] **Interactive conflict resolution helper**:
  - Provide a guided 3-way merge resolver if merging or squash-merging an agent branch encounters conflicts.

---

## 4. Documentation & release announcements

* [ ] **Release v0.4.0** with full agent workflow suite (`bws gw`, `bws gw list`, `bws gw prune`) and hardened IPC/X11 isolation.
* [ ] **Draft Show HN announcement**:
  - Prepare a concise, technical writeup highlighting unprivileged Bubblewrap sandboxing for autonomous AI coding agents without Docker.
* [ ] **Share on Reddit**:
  - `r/golang`, `r/commandline`, `r/linux`, `r/LocalLLaMA`.
