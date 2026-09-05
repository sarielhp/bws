package config

import (
	_ "embed"
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
	EnableDBus        *bool    `json:"enable_dbus"`
	DBusTalk          []string `json:"dbus_talk,omitempty"`
	AllowRawDBus      *bool    `json:"allow_raw_dbus,omitempty"`
	EnableWSL         *bool    `json:"enable_wsl"`
	EnableEtcAutoBind *bool    `json:"enable_etc_auto_bind"`
	EnableProxy       *bool    `json:"enable_proxy,omitempty"`
	NoNet             *bool    `json:"no_net,omitempty"`
	UnshareNet        *bool    `json:"unshare_net,omitempty"`
	AutoInit          string   `json:"auto_init,omitempty"`
	MaskHistory       *bool    `json:"mask_history,omitempty"`
}

func (f *FeaturesConfig) UnmarshalJSON(data []byte) error {
	type Alias FeaturesConfig
	aux := struct {
		AutoInitRaw json.RawMessage `json:"auto_init,omitempty"`
		*Alias
	}{
		Alias: (*Alias)(f),
	}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	if len(aux.AutoInitRaw) > 0 && string(aux.AutoInitRaw) != "null" {
		var s string
		if err := json.Unmarshal(aux.AutoInitRaw, &s); err == nil {
			f.AutoInit = s
			return nil
		}
		var b bool
		if err := json.Unmarshal(aux.AutoInitRaw, &b); err == nil {
			if b {
				f.AutoInit = "always"
			} else {
				f.AutoInit = "never"
			}
			return nil
		}
		return fmt.Errorf("auto_init must be a string or boolean: %s", string(aux.AutoInitRaw))
	}
	return nil
}

// AutoInitMode returns the resolved auto_init mode ("always", "prompt", "never").
// Defaults to "always" when unset or empty.
func AutoInitMode(cfg *Config) string {
	if cfg == nil || cfg.Features == nil {
		return "always"
	}
	return AutoInitModeFromFeatures(cfg.Features)
}

// AutoInitModeFromFeatures returns the resolved auto_init mode from FeaturesConfig.
// Defaults to "always" when unset or empty.
func AutoInitModeFromFeatures(f *FeaturesConfig) string {
	if f == nil || strings.TrimSpace(f.AutoInit) == "" {
		return "always"
	}
	mode := strings.ToLower(strings.TrimSpace(f.AutoInit))
	switch mode {
	case "never", "off", "false":
		return "never"
	case "prompt", "ask":
		return "prompt"
	case "always", "on", "true":
		return "always"
	default:
		return mode
	}
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

//go:embed assets/default_config.jsonc
var DefaultConfigTemplate string

//go:embed assets/example_config.jsonc
var ExampleConfigContent string

func generateDefaultConfig() string {
	h := os.Getenv("HOME")
	if h == "" {
		h = "/home/" + os.Getenv("USER")
	}
	return strings.ReplaceAll(DefaultConfigTemplate, HomeToken, h)
}
