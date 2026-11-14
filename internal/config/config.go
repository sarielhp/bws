package config

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"bws/internal/util"

	"github.com/tailscale/hujson"
)

// HomeToken in a path stands for the user's home directory.
const HomeToken = "@@HOME@@"

// SystemConfig holds the namespace and process settings of the sandbox.
type SystemConfig struct {
	ShareNet   *bool   `json:"share_net"`
	Clearenv   *bool   `json:"clearenv"`
	UnshareUTS *bool   `json:"unshare_uts"`
	Hostname   *string `json:"hostname"`
	NewSession *bool   `json:"new_session,omitempty"`
}

// FeaturesConfig switches optional sandbox features. A nil pointer means the
// feature's default applies.
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
	BlockGH           *bool    `json:"block_gh,omitempty"`
	// ProtectGit mounts the workspace Git configuration and hooks read-only.
	ProtectGit *bool `json:"protect_git,omitempty"`
	// DeployKeyRepo pins the owner/repo used for automatic deploy keys.
	DeployKeyRepo string `json:"deploy_key_repo,omitempty"`
}

// UnmarshalJSON accepts auto_init as either a string mode or the older boolean
// form.
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

// BindEntry is a bind mount from Host to Sandbox; an empty Sandbox means the
// same path inside the sandbox.
type BindEntry struct {
	Host    string
	Sandbox string
}

// UnmarshalJSON accepts a bind as a host path string or a [host, sandbox] pair.
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

// Config is a bws configuration file: global, workspace-local, or their merge.
type Config struct {
	Stack            string                     `json:"stack,omitempty"`
	ReviewedStack    *ProfileApproval           `json:"reviewed_stack,omitempty"`
	ReviewedProfiles map[string]ProfileApproval `json:"reviewed_profiles,omitempty"`
	System           *SystemConfig              `json:"system"`
	SandboxPath      string                     `json:"sandbox_path"`
	ModelsJSONPath   string                     `json:"models_json_path"`
	TmuxSessionName  string                     `json:"tmux_session_name"`
	MaxFileCount     *int                       `json:"max_file_count,omitempty"`
	NoFileLimit      bool                       `json:"no_file_limit,omitempty"`
	Features         *FeaturesConfig            `json:"features"`
	Env              map[string]string          `json:"env"`
	PassEnv          []string                   `json:"pass_env,omitempty"`
	Path             []string                   `json:"path"`
	BindsRW          []BindEntry                `json:"binds_rw"`
	BindsRO          []BindEntry                `json:"binds_ro"`
	// RejectedBinds lists local read-write binds dropped by Merge; see RestrictRW.
	RejectedBinds []string `json:"-"`
	Profiles      []string `json:"profiles,omitempty"`
	// ActiveProfiles lists every profile resolved from Profiles, set on the
	// host by policy.ApplyProfiles; it is never read from a config file.
	ActiveProfiles []string `json:"-"`
	Mask           []string `json:"mask,omitempty"`
	Copy           []string `json:"copy"`
}

// EffectiveMaxFileCount returns the workspace file count limit.
// A return value < 0 indicates file count safety checking is disabled/suppressed.
func (c *Config) EffectiveMaxFileCount() int {
	if c == nil {
		return 1000
	}
	if c.NoFileLimit {
		return -1
	}
	if c.MaxFileCount == nil {
		return 1000
	}
	if *c.MaxFileCount <= 0 {
		return -1
	}
	return *c.MaxFileCount
}

// ProfileApproval records reviewed profile identity without importing the resolver.
type ProfileApproval struct {
	Source string `json:"source"`
	SHA256 string `json:"sha256"`
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

// LoadFile reads and parses the JSONC configuration at path.
func LoadFile(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data, path)
}

// Parse loads JSONC configuration without reading or writing files.
func Parse(data []byte, path string) (*Config, error) {
	standardized, err := hujson.Standardize(bytes.Clone(data))
	if err != nil {
		return nil, fmt.Errorf("invalid JSONC in %s: %w", path, err)
	}
	var raw map[string]interface{}
	if err = json.Unmarshal(standardized, &raw); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", path, err)
	}
	home := util.HomeDir()
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

// Dir returns the bws configuration directory, ~/.config/bws.
func Dir() string {
	home := util.HomeDir()
	return filepath.Join(home, ".config", "bws")
}

// GlobalPath returns the global configuration file, ~/.config/bws/config.jsonc.
func GlobalPath() string {
	return filepath.Join(Dir(), "config.jsonc")
}

// LocalPath returns the workspace configuration file for the current directory.
func LocalPath() string {
	cwd, _ := os.Getwd()
	return FindLocalPath(cwd)
}

// FindWorkspaceRoot walks up from startDir to the nearest directory with a
// .bws/config.jsonc, .bws/config.json or .bws.jsonc, stopping before /, the
// home directory and ~/bin. Without one it returns startDir and the
// .bws/config.jsonc path there.
func FindWorkspaceRoot(startDir string) (rootDir string, configPath string) {
	home := util.HomeDir()
	homeReal, _ := filepath.EvalSymlinks(home)

	dir := filepath.Clean(startDir)
	for {
		dirReal, _ := filepath.EvalSymlinks(dir)
		binReal, _ := filepath.EvalSymlinks(filepath.Join(home, "bin"))
		if dir == "/" || dir == "." || dirReal == homeReal || dir == home || (binReal != "" && dirReal == binReal) {
			break
		}
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
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return startDir, filepath.Join(startDir, ".bws", "config.jsonc")
}

// FindLocalPath returns the workspace configuration path that applies to cwd.
func FindLocalPath(cwd string) string {
	_, cfgPath := FindWorkspaceRoot(cwd)
	return cfgPath
}

// CreateDefault writes the default configuration to path and records it as trusted.
func CreateDefault(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	content := generateDefaultConfig()
	return WriteTrustedFile(path, []byte(content))
}

// CreateExampleConfig writes the annotated example configuration to path.
func CreateExampleConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(ExampleConfigContent), 0644)
}

// DefaultConfigTemplate is the default global configuration.
//
//go:embed assets/default_config.jsonc
var DefaultConfigTemplate string

// ExampleConfigContent is the annotated example configuration.
//
//go:embed assets/example_config.jsonc
var ExampleConfigContent string

func generateDefaultConfig() string {
	// The token sits inside JSON strings, so substitute the escaped form.
	quoted, _ := json.Marshal(util.HomeDir())
	return strings.ReplaceAll(DefaultConfigTemplate, HomeToken, string(quoted[1:len(quoted)-1]))
}

// BlockGH returns true if gh binary and forge credentials should be blocked/masked.
// Defaults to true unless explicitly disabled in features (block_gh: false).
func BlockGH(cfg *Config) bool {
	return FeatureEnabledDefault(cfg, func(f *FeaturesConfig) *bool { return f.BlockGH }, true)
}

// ProtectGit reports whether workspace Git configuration and hooks are mounted
// read-only. Defaults to true unless features.protect_git is false.
func ProtectGit(cfg *Config) bool {
	return FeatureEnabledDefault(cfg, func(f *FeaturesConfig) *bool { return f.ProtectGit }, true)
}
