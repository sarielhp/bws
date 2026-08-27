package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// InitDevOptions contains settings used to generate the development sandbox configuration.
type InitDevOptions struct {
	Features     ProjectFeatures
	TargetDir    string
	Force        bool
	DryRun       bool
	NoSSH        bool
	OpenCode     bool
	Preset       string
	Profiles     []string
	ExtraBindsRW [][]string
	ExtraBindsRO [][]string
	ExtraPath    []string
	ExtraEnv     map[string]string
}

// ApplyPreset applies a preset to the features if specified.
func (opts *InitDevOptions) ApplyPreset() error {
	if opts.Preset == "" {
		return nil
	}

	preset := strings.ToLower(opts.Preset)
	switch preset {
	case "go":
		opts.Features.HasGo = true
	case "python", "py", "uv":
		opts.Features.HasPython = true
	case "rust":
		opts.Features.HasRust = true
	case "node", "js", "ts":
		opts.Features.HasNode = true
	case "latex", "tex":
		opts.Features.HasLatex = true
	case "opencode", "oc", "agent":
		opts.Features.HasOpenCode = true
	case "all":
		opts.Features.HasGo = true
		opts.Features.HasPython = true
		opts.Features.HasRust = true
		opts.Features.HasNode = true
		opts.Features.HasLatex = true
		opts.Features.HasOpenCode = true
	default:
		return fmt.Errorf("unknown preset %q (available: go, python, rust, node, latex, agent, all)", opts.Preset)
	}
	return nil
}

// GenerateDevConfigJSON generates the formatted JSONC string for the local development sandbox.
func GenerateDevConfigJSON(opts InitDevOptions) (string, error) {
	if err := opts.ApplyPreset(); err != nil {
		return "", err
	}

	if opts.OpenCode {
		opts.Features.HasOpenCode = true
	}
	if opts.NoSSH {
		opts.Features.EnableSSH = false
	}

	envMap := make(map[string]string)
	if opts.Features.EnableSSH {
		envMap["GIT_SSH_COMMAND"] = fmt.Sprintf("ssh -F %s/.ssh/config", HomeToken)
	}
	if opts.Features.HasGo {
		envMap["GOPATH"] = fmt.Sprintf("%s/.go", HomeToken)
	}

	rwBinds := [][]string{
		{"~/.gemini", fmt.Sprintf("%s/.gemini", HomeToken)},
	}

	if opts.Features.HasGo {
		rwBinds = append(rwBinds,
			[]string{"~/.cache/go-build", fmt.Sprintf("%s/.cache/go-build", HomeToken)},
			[]string{"~/.config/go", fmt.Sprintf("%s/.config/go", HomeToken)},
			[]string{"~/.go", fmt.Sprintf("%s/.go", HomeToken)},
		)
	}

	if opts.Features.HasPython {
		rwBinds = append(rwBinds,
			[]string{"~/.cache/uv", fmt.Sprintf("%s/.cache/uv", HomeToken)},
		)
	}

	if opts.Features.HasRust {
		rwBinds = append(rwBinds,
			[]string{"~/.cargo", fmt.Sprintf("%s/.cargo", HomeToken)},
		)
	}

	if opts.Features.HasNode {
		rwBinds = append(rwBinds,
			[]string{"~/.npm", fmt.Sprintf("%s/.npm", HomeToken)},
			[]string{"~/.cache/yarn", fmt.Sprintf("%s/.cache/yarn", HomeToken)},
		)
	}

	if opts.Features.HasLatex {
		rwBinds = append(rwBinds,
			[]string{"~/.texlive2026/texmf-var", fmt.Sprintf("%s/.texlive2026/texmf-var", HomeToken)},
			[]string{"~/.texlive2026/texmf-config", fmt.Sprintf("%s/.texlive2026/texmf-config", HomeToken)},
			[]string{"~/.local/share/fonts", fmt.Sprintf("%s/.local/share/fonts", HomeToken)},
			[]string{"~/.cache/fontconfig", fmt.Sprintf("%s/.cache/fontconfig", HomeToken)},
		)
	}

	if opts.Features.HasOpenCode {
		rwBinds = append(rwBinds,
			[]string{"~/.opencode", fmt.Sprintf("%s/.opencode", HomeToken)},
			[]string{"~/.config/opencode", fmt.Sprintf("%s/.config/opencode", HomeToken)},
			[]string{"~/.config/opencode-switcher", fmt.Sprintf("%s/.config/opencode-switcher", HomeToken)},
			[]string{"~/.local/share/opencode", fmt.Sprintf("%s/.local/share/opencode", HomeToken)},
			[]string{"~/.local/state/opencode", fmt.Sprintf("%s/.local/state/opencode", HomeToken)},
			[]string{"~/.cache/opencode", fmt.Sprintf("%s/.cache/opencode", HomeToken)},
		)
	}

	roBinds := [][]string{
		{"~/bin", fmt.Sprintf("%s/bin", HomeToken)},
		{"~/.local", fmt.Sprintf("%s/.local", HomeToken)},
		{"~/.gitconfig", fmt.Sprintf("%s/.gitconfig", HomeToken)},
		{"~/.git-credentials", fmt.Sprintf("%s/.git-credentials", HomeToken)},
		{"~/.ssh/config", fmt.Sprintf("%s/.ssh/config", HomeToken)},
		{"~/.ssh/known_hosts", fmt.Sprintf("%s/.ssh/known_hosts", HomeToken)},
	}

	if opts.Features.HasRust {
		roBinds = append(roBinds,
			[]string{"~/.rustup", fmt.Sprintf("%s/.rustup", HomeToken)},
		)
	}

	if opts.Features.HasLatex {
		roBinds = append(roBinds,
			[]string{"/var/lib/texmf", "/var/lib/texmf"},
			[]string{"/var/cache/fontconfig", "/var/cache/fontconfig"},
		)
	}

	for k, v := range opts.ExtraEnv {
		envMap[k] = v
	}

	seenRW := make(map[string]bool)
	var finalRW [][]string
	for _, b := range append(rwBinds, opts.ExtraBindsRW...) {
		key := strings.Join(b, "->")
		if !seenRW[key] {
			seenRW[key] = true
			finalRW = append(finalRW, b)
		}
	}

	seenRO := make(map[string]bool)
	var finalRO [][]string
	for _, b := range append(roBinds, opts.ExtraBindsRO...) {
		key := strings.Join(b, "->")
		if !seenRO[key] {
			seenRO[key] = true
			finalRO = append(finalRO, b)
		}
	}

	type localFeatures struct {
		EnableSSH bool `json:"enable_ssh"`
	}

	type localDevConfig struct {
		Profiles []string          `json:"profiles,omitempty"`
		Features localFeatures     `json:"features"`
		Env      map[string]string `json:"env,omitempty"`
		Path     []string          `json:"path,omitempty"`
		BindsRW  [][]string        `json:"binds_rw"`
		BindsRO  [][]string        `json:"binds_ro"`
	}

	cfg := localDevConfig{
		Profiles: opts.Profiles,
		Features: localFeatures{
			EnableSSH: opts.Features.EnableSSH,
		},
		Env:     envMap,
		Path:    opts.ExtraPath,
		BindsRW: finalRW,
		BindsRO: finalRO,
	}

	var buf bytes.Buffer
	buf.WriteString("// Bubblewrap Development Sandbox Configuration (.bws/config.jsonc)\n")
	buf.WriteString("// Automatically generated by 'bws init-dev'\n\n")

	encoder := json.NewEncoder(&buf)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(cfg); err != nil {
		return "", fmt.Errorf("encoding config JSON: %w", err)
	}

	return buf.String(), nil
}
