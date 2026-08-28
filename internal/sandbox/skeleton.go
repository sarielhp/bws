package sandbox

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

const DefaultTmuxConf = `# Bubblewrap Sandbox tmux configuration
set -g status-left-length 20
set -g status-left "#[fg=white,bg=purple,bold] BUBBLE #[default] "
set -g status-style bg=colour234,fg=colour137

# WSL clipboard integration via clip.exe
if-shell 'test -n "$WSL_INTEROP" -a -x "/mnt/c/Windows/System32/clip.exe"' {
  bind-key -T copy-mode-vi MouseDragEnd1Pane send-keys -X copy-pipe-and-cancel "/mnt/c/Windows/System32/clip.exe"
  bind-key -T copy-mode    MouseDragEnd1Pane send-keys -X copy-pipe-and-cancel "/mnt/c/Windows/System32/clip.exe"
}
`

const DefaultBashrc = `# Bubblewrap Sandbox .bashrc
# Source global definitions if available
if [ -f /etc/bashrc ]; then
  . /etc/bashrc
fi

# Standard aliases
alias ls='ls --color=auto'
alias ll='ls -alF'
alias la='ls -A'
alias l='ls -CF'
alias grep='grep --color=auto'

`

const DefaultProfile = `# Bubblewrap Sandbox .profile
if [ -n "$BASH_VERSION" ]; then
  if [ -f "$HOME/.bashrc" ]; then
    . "$HOME/.bashrc"
  fi
fi
`

func GlobalSkeletonDir() string {
	return filepath.Join(config.ConfigDir(), "skeleton")
}

func LocalSkeletonDir(cwd string) string {
	return filepath.Join(cwd, ".bw", "skeleton")
}

func EnsureGlobalSkeleton() error {
	dir := GlobalSkeletonDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating skeleton directory: %w", err)
	}

	files := []struct {
		name    string
		content string
	}{
		{".tmux.conf", DefaultTmuxConf},
		{".bashrc", DefaultBashrc},
		{".profile", DefaultProfile},
	}

	for _, f := range files {
		target := filepath.Join(dir, f.name)
		if _, err := os.Stat(target); os.IsNotExist(err) {
			if err := os.WriteFile(target, []byte(f.content), 0644); err != nil {
				return fmt.Errorf("writing skeleton file %s: %w", target, err)
			}
		}
	}
	return nil
}

func StageHome(cfg *config.Config, currentDir string) (string, func(), error) {
	if err := EnsureGlobalSkeleton(); err != nil {
		return "", nil, err
	}

	os.MkdirAll("/tmp/bws", 0755)
	stageDir, err := os.MkdirTemp("/tmp/bws", "stage_")
	if err != nil {
		return "", nil, fmt.Errorf("creating session stage directory: %w", err)
	}

	cleanup := func() {
		os.RemoveAll(stageDir)
	}

	// 1. Copy global skeleton
	globalSkel := GlobalSkeletonDir()
	if err := copyDirectoryContents(globalSkel, stageDir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("copying global skeleton: %w", err)
	}

	for _, rcName := range []string{".bashrc", ".zshrc"} {
		stagedRC := filepath.Join(stageDir, rcName)
		localSkel := LocalSkeletonDir(currentDir)
		localRCExists := false
		if fi, err := os.Stat(filepath.Join(localSkel, rcName)); err == nil && !fi.IsDir() {
			localRCExists = true
		}

		if fi, err := os.Stat(stagedRC); err == nil && !fi.IsDir() && !localRCExists {
			data, err := os.ReadFile(stagedRC)
			if err == nil {
				comment := fmt.Sprintf("# --- Copied from global skeleton (%s) ---\n", filepath.Join(globalSkel, rcName))
				os.WriteFile(stagedRC, append([]byte(comment), data...), 0644)
			}
		}
	}

	// 2. Overlay project skeleton if present
	localSkel := LocalSkeletonDir(currentDir)
	if fi, err := os.Stat(localSkel); err == nil && fi.IsDir() {
		if err := copyDirectoryContents(localSkel, stageDir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("copying local skeleton: %w", err)
		}
		for _, rcName := range []string{".bashrc", ".zshrc"} {
			stagedRC := filepath.Join(stageDir, rcName)
			if fi, err := os.Stat(filepath.Join(localSkel, rcName)); err == nil && !fi.IsDir() {
				data, err := os.ReadFile(stagedRC)
				if err == nil {
					comment := fmt.Sprintf("# --- Copied from local skeleton (%s) ---\n", filepath.Join(localSkel, rcName))
					os.WriteFile(stagedRC, append([]byte(comment), data...), 0644)
				}
			}
		}
	}

	// 3. Inject dynamic appendices into staged .bashrc
	stagedBashrc := filepath.Join(stageDir, ".bashrc")
	if err := appendDynamicConfig(cfg, stagedBashrc); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("appending dynamic shell config: %w", err)
	}

	// 4. Pre-create mountpoint paths inside staged home
	home := util.HomeDir()
	allEntries := append(append([]config.BindEntry{}, cfg.BindsRW...), cfg.BindsRO...)
	for _, b := range allEntries {
		dest := b.Sandbox
		if dest == "" {
			dest = b.Host
		}
		dest = strings.ReplaceAll(dest, config.HomeToken, home)
		dest = util.ExpandHome(dest)
		if strings.HasPrefix(dest, home+"/") {
			rel := strings.TrimPrefix(dest, home+"/")
			target := filepath.Join(stageDir, rel)
			host := util.ExpandHome(b.Host)
			if fi, err := os.Stat(host); err == nil {
				if fi.IsDir() {
					os.MkdirAll(target, 0755)
				} else {
					os.MkdirAll(filepath.Dir(target), 0755)
					if _, err := os.Stat(target); os.IsNotExist(err) {
						os.WriteFile(target, []byte{}, 0644)
					}
				}
			}
		}
	}
	for _, m := range cfg.Mask {
		expanded := util.ExpandHome(m)
		expanded = strings.ReplaceAll(expanded, config.HomeToken, home)
		if strings.HasPrefix(expanded, home+"/") {
			rel := strings.TrimPrefix(expanded, home+"/")
			target := filepath.Join(stageDir, rel)
			if fi, err := os.Stat(expanded); err == nil {
				if fi.IsDir() {
					os.MkdirAll(target, 0755)
				} else {
					os.MkdirAll(filepath.Dir(target), 0755)
					if _, err := os.Stat(target); os.IsNotExist(err) {
						os.WriteFile(target, []byte{}, 0644)
					}
				}
			}
		}
	}

	return stageDir, cleanup, nil
}

func copyDirectoryContents(srcDir, destDir string) error {
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(srcDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(destPath, 0755); err != nil {
				return err
			}
			if err := copyDirectoryContents(srcPath, destPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, destPath); err != nil {
				return err
			}
		}
	}
	return nil
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

func appendDynamicConfig(cfg *config.Config, bashrcPath string) error {
	f, err := os.OpenFile(bashrcPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	var sb strings.Builder
	sb.WriteString("\n# --- Dynamic Sandbox Appendices (Generated by bws from configuration file) ---\n")

	if len(cfg.Path) > 0 {
		resolved := make([]string, 0, len(cfg.Path))
		for _, p := range cfg.Path {
			resolvedP := strings.ReplaceAll(p, config.HomeToken, util.HomeDir())
			resolvedP = util.ExpandHome(resolvedP)
			resolved = append(resolved, resolvedP)
		}
		sb.WriteString("\n# Add sandbox-specific binary paths (Generated by bws from configuration file 'path' entry)\n")
		sb.WriteString(fmt.Sprintf("export PATH=\"%s:$PATH\"\n", strings.Join(resolved, ":")))
	}

	if cfg.Env != nil {
		hasEnv := false
		var envSB strings.Builder
		if editor, ok := cfg.Env["EDITOR"]; ok && editor != "" {
			envSB.WriteString(fmt.Sprintf("export EDITOR=%q\n", editor))
			hasEnv = true
		}
		if visual, ok := cfg.Env["VISUAL"]; ok && visual != "" {
			envSB.WriteString(fmt.Sprintf("export VISUAL=%q\n", visual))
			hasEnv = true
		}
		if gopath, ok := cfg.Env["GOPATH"]; ok && gopath != "" {
			envSB.WriteString(fmt.Sprintf("export GOPATH=%q\n", gopath))
			hasEnv = true
		}
		if hasEnv {
			sb.WriteString("\n# sandbox default environment variables (Generated by bws from configuration file 'env' entry)\n")
			sb.WriteString(envSB.String())
		}
	}

	_, err = f.WriteString(sb.String())
	return err
}
