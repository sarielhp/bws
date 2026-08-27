package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const HomeToken = "@@HOME@@"

type SystemConfig struct {
	ShareNet   *bool   `json:"share_net"`
	Clearenv   *bool   `json:"clearenv"`
	UnshareUTS *bool   `json:"unshare_uts"`
	Hostname   *string `json:"hostname"`
}

type FeaturesConfig struct {
	EnableSSH         *bool    `json:"enable_ssh"`
	SSHKeys           []string `json:"ssh_keys"`
	AutoRepoDeployKey *bool    `json:"auto_repo_deploy_key"`
	EnableX11         *bool    `json:"enable_x11"`
	EnableWSL         *bool    `json:"enable_wsl"`
	EnableEtcAutoBind *bool    `json:"enable_etc_auto_bind"`
	EnableOhMyPosh    *bool    `json:"enable_oh_my_posh"`
}

type OhMyPoshConfig struct {
	ThemePath *string `json:"theme_path"`
}

type BindEntry struct {
	Host    string
	Sandbox string
}

func (b *BindEntry) UnmarshalJSON(data []byte) error {
	var s string
	if err := json.Unmarshal(data, &s); err == nil {
		b.Host = s
		return nil
	}
	var pair []string
	if err := json.Unmarshal(data, &pair); err != nil {
		return fmt.Errorf("bind entry must be a string or [host, sandbox] pair: %w", err)
	}
	if len(pair) >= 1 {
		b.Host = pair[0]
	}
	if len(pair) >= 2 {
		b.Sandbox = pair[1]
	}
	return nil
}

type Config struct {
	System          *SystemConfig     `json:"system"`
	SandboxPath     string            `json:"sandbox_path"`
	ModelsJSONPath  string            `json:"models_json_path"`
	TmuxSessionName string            `json:"tmux_session_name"`
	MaxFileCount    int               `json:"max_file_count"`
	Cdtoday         string            `json:"cdtoday"`
	Features        *FeaturesConfig   `json:"features"`
	OhMyPosh        *OhMyPoshConfig   `json:"oh_my_posh"`
	Env             map[string]string `json:"env"`
	Path            []string          `json:"path"`
	BindsRW         []BindEntry       `json:"binds_rw"`
	BindsRO         []BindEntry       `json:"binds_ro"`
	Profiles        []string          `json:"profiles,omitempty"`
	Copy            []string          `json:"copy"`
}

func replaceHomeToken(obj interface{}, home string) interface{} {
	switch v := obj.(type) {
	case map[string]interface{}:
		for k, val := range v {
			v[k] = replaceHomeToken(val, home)
		}
		return v
	case []interface{}:
		for i, val := range v {
			v[i] = replaceHomeToken(val, home)
		}
		return v
	case string:
		return strings.ReplaceAll(v, HomeToken, home)
	default:
		return v
	}
}

func LoadFile(path string) (*Config, error) {
	standardized, err := LoadFileWithHuJSON(path)
	if err != nil {
		return nil, err
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(standardized, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	home, _ := os.UserHomeDir()
	raw = replaceHomeToken(raw, home).(map[string]interface{})
	cfgBytes, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(cfgBytes, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "bw")
}

func GlobalPath() string {
	return filepath.Join(ConfigDir(), "config.jsonc")
}

func LocalPath() string {
	cwd, _ := os.Getwd()
	return FindLocalPath(cwd)
}

func FindLocalPath(cwd string) string {
	p1 := filepath.Join(cwd, ".bw", "config.jsonc")
	if fi, err := os.Stat(p1); err == nil && !fi.IsDir() {
		return p1
	}
	p2 := filepath.Join(cwd, ".bw", "config.json")
	if fi, err := os.Stat(p2); err == nil && !fi.IsDir() {
		return p2
	}
	p3 := filepath.Join(cwd, ".bw.jsonc")
	if fi, err := os.Stat(p3); err == nil && !fi.IsDir() {
		return p3
	}
	return p1
}

func CreateDefault(path string) error {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	content := generateDefaultConfig()
	return os.WriteFile(path, []byte(content), 0644)
}

func CreateExampleConfig(path string) error {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	return os.WriteFile(path, []byte(ExampleConfigContent), 0644)
}

func CreateDefaultTheme(path string) error {
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	return os.WriteFile(path, []byte(DefaultThemeJSON), 0644)
}

func generateDefaultConfig() string {
	h := os.Getenv("HOME")
	if h == "" {
		h = "/home/" + os.Getenv("USER")
	}
	user := os.Getenv("USER")
	if user == "" {
		user = "user"
	}
	logname := os.Getenv("LOGNAME")
	if logname == "" {
		logname = user
	}
	return fmt.Sprintf(`// Bubblewrap Sandbox Launcher Configuration File
{
  "system": {
    "share_net": true,
    "clearenv": true,
    "unshare_uts": true,
    "hostname": "bubble"
  },
  "sandbox_path": "",
  "models_json_path": "",
  "tmux_session_name": "bwrap-dev",
  "max_file_count": 1000,
  "cdtoday": "info/misc",
  "profiles": [
    "editor"
  ],
  "features": {
    "enable_ssh": true,
    "ssh_keys": [],
    "auto_repo_deploy_key": true,
    "enable_x11": true,
    "enable_wsl": true,
    "enable_etc_auto_bind": true,
    "enable_oh_my_posh": true
  },
  "oh_my_posh": {
    "theme_path": "~/.config/bw/theme.omp.json"
  },
  "env": {
    "HOME": %[1]q,
    "TERM": "xterm-256color",
    "LANG": "C.UTF-8",
    "LC_ALL": "C.UTF-8",
    "USER": %[2]q,
    "LOGNAME": %[3]q,
    "SHELL": "/bin/bash",
    "EDITOR": "emacs -nw",
    "VISUAL": "emacs -nw",
    "OPENROUTER_API_KEY": ""
  },
  "path": [
    "%[1]s/bin",
    "%[1]s/.local/bin",
    "%[1]s/.opencode/bin",
    "%[1]s/.cargo/bin",
    "/usr/local/sbin",
    "/usr/local/bin",
    "/usr/sbin",
    "/usr/bin",
    "/sbin",
    "/bin"
  ],
  "binds_rw": [
    ["~/.opencode", "%[1]s/.opencode"],
    ["~/.terminfo", "%[1]s/.terminfo"],
    ["~/.local/share/terminfo", "%[1]s/.local/share/terminfo"]
  ],
  "binds_ro": [
    ["/bin", "/bin"],
    ["/usr", "/usr"],
    ["/lib", "/lib"],
    ["/lib64", "/lib64"],
    ["/lib32", "/lib32"],
    ["/libx32", "/libx32"],
    ["/sbin", "/sbin"],
    ["/run/systemd/journal", "/run/systemd/journal"],
    ["~/.local", "%[1]s/.local"],
    ["~/.gitconfig", "%[1]s/.gitconfig"],
    ["~/.git-credentials", "%[1]s/.git-credentials"],
    ["~/.config/git", "%[1]s/.config/git"],
    ["~/.ssh/config", "%[1]s/.ssh/config"],
    ["~/.ssh/known_hosts", "%[1]s/.ssh/known_hosts"]
  ]
}
`, h, user, logname)
}

const ExampleConfigContent = `// Bubblewrap Sandbox Launcher Configuration File (Blank Template Example)
//
// Use this file as a template to customize your sandbox configuration.
// Copy this to config.jsonc and modify as needed.

{
  "system": {
    "share_net": true,
    "clearenv": true,
    "unshare_uts": true,
    "hostname": "bubble"
  },

  "features": {
    "enable_ssh": false,
    "enable_x11": false,
    "enable_wsl": true,
    "enable_etc_auto_bind": false,
    "enable_oh_my_posh": false
  },

  "oh_my_posh": {
    "theme_path": "~/.config/bw/theme.omp.json"
  },

  "cdtoday": "info/misc",

  "env": {
    "HOME": "@@HOME@@",
    "TERM": "xterm-256color",
    "LANG": "C.UTF-8",
    "EDITOR": "emacs -nw",
    "VISUAL": "emacs -nw",
    "OPENROUTER_API_KEY": ""
  },

  "path": [
    "@@HOME@@/bin",
    "@@HOME@@/.local/bin",
    "/usr/local/bin",
    "/usr/bin",
    "/bin"
  ],

  "binds_rw": [
    // ["/path/to/host/dir", "/path/to/sandbox/dir"]
  ],

  "binds_ro": [
    // ["/path/to/host/file_or_dir", "/path/to/sandbox/file_or_dir"]
  ]
}
`

const DefaultThemeJSON = `{
  "version": 4,
  "final_space": true,
  "blocks": [
    {
      "type": "prompt",
      "alignment": "left",
      "newline": true,
      "segments": [
        {
          "type": "text",
          "style": "diamond",
          "foreground": "#ffffff",
          "background": "#8a2be2",
          "leading_diamond": "\u{e0b6}",
          "trailing_diamond": "\u{e0b0}",
          "template": " \u{2b22} BUBBLE "
        },
        {
          "type": "session",
          "style": "powerline",
          "powerline_symbol": "\u{e0b0}",
          "foreground": "#ffffff",
          "background": "#3a3a3a",
          "template": " {{ .UserName }}@{{ .HostName }} "
        },
        {
          "type": "path",
          "style": "powerline",
          "powerline_symbol": "\u{e0b0}",
          "foreground": "#ffffff",
          "background": "#1c2e4a",
          "template": " {{ .Path }} ",
          "properties": {
            "style": "folder"
          }
        },
        {
          "type": "git",
          "style": "powerline",
          "powerline_symbol": "\u{e0b0}",
          "foreground": "#101010",
          "background": "#2e9599",
          "template": " {{ .HEAD }}{{ if .BranchStatus }} {{ .BranchStatus }}{{ end }} "
        }
      ]
    }
  ]
}
`
