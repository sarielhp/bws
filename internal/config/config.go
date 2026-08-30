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
	EnableProxy       *bool    `json:"enable_proxy,omitempty"`
	NoNet             *bool    `json:"no_net,omitempty"`
	UnshareNet        *bool    `json:"unshare_net,omitempty"`
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
	Features        *FeaturesConfig   `json:"features"`
	Env             map[string]string `json:"env"`
	PassEnv         []string          `json:"pass_env,omitempty"`
	Path            []string          `json:"path"`
	BindsRW         []BindEntry       `json:"binds_rw"`
	BindsRO         []BindEntry       `json:"binds_ro"`
	Profiles        []string          `json:"profiles,omitempty"`
	Mask            []string          `json:"mask,omitempty"`
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
	return filepath.Join(home, ".config", "bws")
}

func GlobalPath() string {
	return filepath.Join(ConfigDir(), "config.jsonc")
}

func LocalPath() string {
	cwd, _ := os.Getwd()
	return FindLocalPath(cwd)
}

func FindWorkspaceRoot(startDir string) (rootDir string, configPath string) {
	home, _ := os.UserHomeDir()
	homeReal, _ := filepath.EvalSymlinks(home)

	dir := filepath.Clean(startDir)
	for {
		candidates := []string{
			filepath.Join(dir, ".bws", "config.jsonc"),
			filepath.Join(dir, ".bws", "config.json"),
			filepath.Join(dir, ".bws.jsonc"),
		}
		for _, p := range candidates {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return dir, p
			}
		}
		dirReal, _ := filepath.EvalSymlinks(dir)
		if dir == "/" || dir == "." || dirReal == homeReal || dir == home {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return startDir, filepath.Join(startDir, ".bws", "config.jsonc")
}

func FindLocalPath(cwd string) string {
	_, cfgPath := FindWorkspaceRoot(cwd)
	return cfgPath
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
  "profiles": [
    "editor"
  ],
  // Features are enabled by default. Uncomment to disable:
  // "features": {
  //   "enable_ssh": false,
  //   "enable_x11": false,
  //   "enable_wsl": false
  // },
  "pass_env": [
    "USER",
    "LOGNAME",
    "SHELL",
    "TERM",
    "LANG",
    "LC_ALL"
  ],
  "env": {
    "HOME": %[1]q,
    "EDITOR": "emacs -nw",
    "VISUAL": "$EDITOR"
  },
  "path": [
    "%[1]s/bin",
    "%[1]s/.local/bin",
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
    "enable_etc_auto_bind": false
  },

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
