package config

import (
	"strings"
	"testing"
)

func TestGenerateDevConfigJSON(t *testing.T) {
	tests := []struct {
		name         string
		opts         InitDevOptions
		expectSubstr []string
		forbidSubstr []string
	}{
		{
			name: "Go dev config",
			opts: InitDevOptions{
				Features: ProjectFeatures{
					HasGo:     true,
					EnableSSH: true,
				},
			},
			expectSubstr: []string{
				`"enable_ssh": true`,
				`"GOPATH": "@@HOME@@/.go"`,
				`"GIT_SSH_COMMAND": "ssh -F @@HOME@@/.ssh/config"`,
				`"~/.cache/go-build"`,
				`"~/.go"`,
				`"~/.gemini"`,
				`"~/bin"`,
				`"~/.local"`,
			},
			forbidSubstr: []string{
				`"~/.cache/uv"`,
				`"~/.cargo"`,
			},
		},
		{
			name: "Python preset without SSH",
			opts: InitDevOptions{
				NoSSH:  true,
				Preset: "python",
			},
			expectSubstr: []string{
				`"enable_ssh": false`,
				`"~/.cache/uv"`,
			},
			forbidSubstr: []string{
				`"GOPATH"`,
				`"GIT_SSH_COMMAND"`,
			},
		},
		{
			name: "OpenCode forced flag",
			opts: InitDevOptions{
				OpenCode: true,
			},
			expectSubstr: []string{
				`"~/.config/opencode"`,
				`"~/.config/opencode-switcher"`,
			},
		},
		{
			name: "LaTeX preset config",
			opts: InitDevOptions{
				Preset: "latex",
			},
			expectSubstr: []string{
				`"~/.texlive2026/texmf-var"`,
				`"~/.texlive2026/texmf-config"`,
				`"~/.local/share/fonts"`,
				`"~/.cache/fontconfig"`,
				`"/var/lib/texmf"`,
				`"/var/cache/fontconfig"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			jsonStr, err := GenerateDevConfigJSON(tt.opts)
			if err != nil {
				t.Fatalf("GenerateDevConfigJSON failed: %v", err)
			}

			for _, s := range tt.expectSubstr {
				if !strings.Contains(jsonStr, s) {
					t.Errorf("expected generated json to contain %q, got:\n%s", s, jsonStr)
				}
			}

			for _, s := range tt.forbidSubstr {
				if strings.Contains(jsonStr, s) {
					t.Errorf("expected generated json NOT to contain %q, got:\n%s", s, jsonStr)
				}
			}
		})
	}
}
