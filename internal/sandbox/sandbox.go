package sandbox

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"bws/internal/config"
	"bws/internal/util"
)

type CopyRecord struct {
	Src    string
	Dest   string
	Copied bool
}

var CopiedFiles []CopyRecord

func Prepare(cfg *config.Config, sandboxDir string) {
	os.MkdirAll(filepath.Join(sandboxDir, "bin"), 0755)
	os.MkdirAll(filepath.Join(sandboxDir, "etc"), 0755)
	os.MkdirAll(filepath.Join(sandboxDir, ".pi", "agent"), 0755)

	processCopyPaths(cfg.Copy, sandboxDir)

	modelsSrc := "~/info/llm/models.json"
	if cfg.ModelsJSONPath != "" {
		modelsSrc = cfg.ModelsJSONPath
	}
	modelsSrc = util.ExpandHome(modelsSrc)
	if util.CopyIfNewer(modelsSrc, filepath.Join(sandboxDir, ".pi", "agent", "models.json")) {
		CopiedFiles = append(CopiedFiles, CopyRecord{Src: modelsSrc, Dest: filepath.Join(sandboxDir, ".pi", "agent", "models.json"), Copied: true})
	}

	if data, err := os.ReadFile("/etc/resolv.conf"); err == nil {
		dest := filepath.Join(sandboxDir, "etc", "resolv.conf")
		copied := false
		if existing, err := os.ReadFile(dest); err != nil || string(existing) != string(data) {
			os.WriteFile(dest, data, 0644)
			copied = true
		}
		CopiedFiles = append(CopiedFiles, CopyRecord{Src: "/etc/resolv.conf", Dest: dest, Copied: copied})
	}

	enableOMP := true
	if cfg.Features != nil && cfg.Features.EnableOhMyPosh != nil {
		enableOMP = *cfg.Features.EnableOhMyPosh
	}

	if enableOMP {
		setupOhMyPosh(cfg, sandboxDir)
	}

	setupShellConfig(cfg, sandboxDir)
	setupTmuxConfig(sandboxDir)
	setupCdtoday(cfg, sandboxDir)
}

func processCopyPaths(copyList []string, sandboxDir string) {
	home := util.HomeDir()
	for _, pathStr := range copyList {
		expanded := util.ExpandHome(pathStr)
		if expanded == home || strings.HasPrefix(expanded, home+"/.sandbox") {
			fmt.Fprintf(os.Stderr, "Warning: Refusing to copy home directory or sandbox backing store: %s\n", expanded)
			continue
		}
		if !strings.HasPrefix(expanded, home+"/") {
			fmt.Fprintf(os.Stderr, "Warning: Cannot copy path outside home directory: %s\n", expanded)
			continue
		}
		relPath := strings.TrimPrefix(expanded, home+"/")
		sandboxDest := filepath.Join(sandboxDir, relPath)

		if fi, err := os.Stat(expanded); err == nil {
			copied := false
			if fi.IsDir() {
				os.RemoveAll(sandboxDest)
				os.MkdirAll(filepath.Dir(sandboxDest), 0755)
				copyDir(expanded, sandboxDest)
				copied = true
			} else {
				copied = util.CopyIfNewer(expanded, sandboxDest)
			}
			CopiedFiles = append(CopiedFiles, CopyRecord{Src: expanded, Dest: sandboxDest, Copied: copied})
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Path to copy does not exist: %s\n", expanded)
		}
	}
}

func copyDir(src, dest string) {
	filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			os.MkdirAll(target, 0755)
		} else {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			os.WriteFile(target, data, 0644)
		}
		return nil
	})
}

func setupOhMyPosh(cfg *config.Config, sandboxDir string) {
	themeSource := filepath.Join(util.HomeDir(), ".config", "bw", "theme.omp.json")
	if cfg.OhMyPosh != nil && cfg.OhMyPosh.ThemePath != nil {
		themeSource = util.ExpandHome(*cfg.OhMyPosh.ThemePath)
	}

	if _, err := os.Stat(themeSource); os.IsNotExist(err) {
		if themeSource == filepath.Join(util.HomeDir(), ".config", "bw", "theme.omp.json") {
			os.MkdirAll(filepath.Dir(themeSource), 0755)
			os.WriteFile(themeSource, []byte(config.DefaultThemeJSON), 0644)
		}
	}

	if fi, err := os.Stat(themeSource); err == nil && !fi.IsDir() {
		themeDest := filepath.Join(sandboxDir, ".mytheme.omp.json")
		util.CopyIfNewer(themeSource, themeDest)

		ompBin := filepath.Join(util.HomeDir(), "bin", "oh-my-posh")
		if fi, err := os.Stat(ompBin); err == nil && !fi.IsDir() {
			util.CopyIfNewer(ompBin, filepath.Join(sandboxDir, "bin", "oh-my-posh"))
		}

		sandboxTheme := filepath.Join(util.HomeDir(), ".mytheme.omp.json")
		injectOhMyPosh(filepath.Join(sandboxDir, ".bashrc"), "bash", sandboxTheme)
		if _, err := os.Stat(filepath.Join(sandboxDir, ".zshrc")); err == nil {
			injectOhMyPosh(filepath.Join(sandboxDir, ".zshrc"), "zsh", sandboxTheme)
		}
	}
}

func ensureFileExists(path string) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		os.MkdirAll(filepath.Dir(path), 0755)
		os.WriteFile(path, nil, 0644)
	}
}

func injectOhMyPosh(targetFile, shellType, themePath string) {
	home := util.HomeDir()
	ensureFileExists(targetFile)

	data, _ := os.ReadFile(targetFile)
	content := string(data)

	re := regexp.MustCompile(`(?ms)# Initialize oh-my-posh prompt theme engine\n.*?\nfi\n?`)
	content = re.ReplaceAllString(content, "")

	block := fmt.Sprintf(`if command -v oh-my-posh >/dev/null 2>&1; then
  eval "$(oh-my-posh init %s --config %s)"
elif [ -x "%s/bin/oh-my-posh" ]; then
  eval "$(%s/bin/oh-my-posh init %s --config %s)"
fi
`, shellType, themePath, home, home, shellType, themePath)

	if !strings.Contains(content, "oh-my-posh init") {
		content += "\n# Initialize oh-my-posh prompt theme engine\n" + block
		os.WriteFile(targetFile, []byte(content), 0644)
	}
}

func setupShellConfig(cfg *config.Config, sandboxDir string) {
	sandboxPaths := cfg.Path
	editor := "emacs -nw"
	visual := "emacs -nw"
	if cfg.Env != nil {
		if e, ok := cfg.Env["EDITOR"]; ok && e != "" {
			editor = e
		}
		if v, ok := cfg.Env["VISUAL"]; ok && v != "" {
			visual = v
		}
	}

	pathLine := ""
	if len(sandboxPaths) > 0 {
		resolved := make([]string, 0, len(sandboxPaths))
		for _, p := range sandboxPaths {
			if strings.HasPrefix(p, "~") {
				resolved = append(resolved, util.ExpandHome(p))
			} else {
				resolved = append(resolved, p)
			}
		}
		pathLine = fmt.Sprintf("export PATH=\"%s:$PATH\"", strings.Join(resolved, ":"))
	}
	editorBlock := fmt.Sprintf("export EDITOR=\"%s\"\nexport VISUAL=\"%s\"", editor, visual)

	for _, rcName := range []string{".bashrc", ".zshrc"} {
		rcPath := filepath.Join(sandboxDir, rcName)
		if rcName != ".bashrc" {
			if _, err := os.Stat(rcPath); os.IsNotExist(err) {
				continue
			}
		}
		ensureFileExists(rcPath)

		data, _ := os.ReadFile(rcPath)
		text := string(data)
		original := text

		if pathLine != "" {
			re := regexp.MustCompile(`(?m)# Add sandbox-specific binary paths\nexport PATH="[^"]*"\n?`)
			if re.MatchString(text) {
				text = re.ReplaceAllString(text, "# Add sandbox-specific binary paths\n"+pathLine+"\n")
			} else {
				text += "\n# Add sandbox-specific binary paths\n" + pathLine + "\n"
			}
		}

		re := regexp.MustCompile(`(?m)# sandbox default editor\nexport EDITOR="[^"]*"\nexport VISUAL="[^"]*"\n?`)
		if re.MatchString(text) {
			text = re.ReplaceAllString(text, "# sandbox default editor\n"+editorBlock+"\n")
		} else {
			text += "\n# sandbox default editor\n" + editorBlock + "\n"
		}

		if text != original {
			os.WriteFile(rcPath, []byte(text), 0644)
		}
	}

	for _, profileName := range []string{".bash_profile", ".profile"} {
		profilePath := filepath.Join(sandboxDir, profileName)
		ensureFileExists(profilePath)
		data, _ := os.ReadFile(profilePath)
		content := string(data)
		if !strings.Contains(content, ".bashrc") {
			content += "\n# Include .bashrc if it exists\nif [ -f \"$HOME/.bashrc\" ]; then\n  . \"$HOME/.bashrc\"\nfi\n"
			os.WriteFile(profilePath, []byte(content), 0644)
		}
	}
}

func setupTmuxConfig(sandboxDir string) {
	tmuxConf := filepath.Join(sandboxDir, ".tmux.conf")
	hostTmux := filepath.Join(util.HomeDir(), ".tmux.conf")
	needsBubble := false

	if fi, err := os.Stat(hostTmux); err == nil && !fi.IsDir() {
		copied := false
		if dfi, err := os.Stat(tmuxConf); os.IsNotExist(err) || fi.ModTime().After(dfi.ModTime()) {
			data, _ := os.ReadFile(hostTmux)
			os.WriteFile(tmuxConf, data, 0644)
			needsBubble = true
			copied = true
		}
		CopiedFiles = append(CopiedFiles, CopyRecord{Src: hostTmux, Dest: tmuxConf, Copied: copied})
	} else if _, err := os.Stat(tmuxConf); os.IsNotExist(err) {
		ensureFileExists(tmuxConf)
		needsBubble = true
		CopiedFiles = append(CopiedFiles, CopyRecord{Src: "<empty>", Dest: tmuxConf, Copied: true})
	}

	data, _ := os.ReadFile(tmuxConf)
	content := string(data)

	if needsBubble || !strings.Contains(content, "BUBBLE") {
		content += "\n# Sandbox indicator in status bar\nset -g status-left-length 20\nset -g status-left \"#[fg=white,bg=purple,bold] BUBBLE #[default] \"\nset -g status-style bg=colour234,fg=colour137\n"
	}

	if !strings.Contains(content, "clip.exe") {
		content += "\n# WSL clipboard integration via clip.exe\nif-shell 'test -n \"$WSL_INTEROP\" -a -x \"/mnt/c/Windows/System32/clip.exe\"' {\n  bind-key -T copy-mode-vi MouseDragEnd1Pane send-keys -X copy-pipe-and-cancel \"/mnt/c/Windows/System32/clip.exe\"\n  bind-key -T copy-mode    MouseDragEnd1Pane send-keys -X copy-pipe-and-cancel \"/mnt/c/Windows/System32/clip.exe\"\n}\n"
	}

	os.WriteFile(tmuxConf, []byte(content), 0644)
}

func setupCdtoday(cfg *config.Config, sandboxDir string) {
	base := "info/misc"
	if cfg.Cdtoday != "" {
		base = strings.TrimSuffix(cfg.Cdtoday, "/")
	}

	injectCdtoday(filepath.Join(sandboxDir, ".bashrc"), base)
	if _, err := os.Stat(filepath.Join(sandboxDir, ".zshrc")); err == nil {
		injectCdtoday(filepath.Join(sandboxDir, ".zshrc"), base)
	}
}

func injectCdtoday(targetFile, base string) {
	ensureFileExists(targetFile)

	data, _ := os.ReadFile(targetFile)
	content := string(data)

	re := regexp.MustCompile(`(?m)^cdtoday \(\) \{\n(?:.*\n)*?\}\n?`)
	content = re.ReplaceAllString(content, "")

	block := fmt.Sprintf(`cdtoday () {
    BS=%s/$(date +%%02y/%%02m/%%02d)/
    DR=~/$BS
    mkdir -p $DR
    cd $DR
    echo cd '~/'$BS
}
`, base)

	if !strings.Contains(content, "cdtoday ()") {
		content += "\n# Function to navigate to today's " + base + " directory\n" + block
		os.WriteFile(targetFile, []byte(content), 0644)
	}
}
