package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
)

func TestPromptAutoInit(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"y\n", true},
		{"Y\n", true},
		{"yes\n", true},
		{"YES\n", true},
		{"\n", true},
		{"  y  \n", true},
		{"n\n", false},
		{"N\n", false},
		{"no\n", false},
		{"NO\n", false},
		{"anything\n", false},
		{"", false},
	}

	for _, tt := range tests {
		var out bytes.Buffer
		r := strings.NewReader(tt.input)
		ans, err := PromptAutoInit(r, &out)
		if err != nil && tt.input != "" {
			t.Fatalf("unexpected error for input %q: %v", tt.input, err)
		}
		if ans != tt.expected {
			t.Errorf("PromptAutoInit(%q) = %v; want %v", tt.input, ans, tt.expected)
		}
		if !strings.Contains(out.String(), "Auto-configure workspace now?") {
			t.Errorf("expected prompt in output, got %q", out.String())
		}
	}
}

func TestAutoConfigureWorkspaceEmptyDir(t *testing.T) {
	tmpDir := t.TempDir()
	configPath, summary, err := AutoConfigureWorkspace(tmpDir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if configPath != "" || summary != "" {
		t.Errorf("expected empty result for empty dir, got path=%q summary=%q", configPath, summary)
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".bws")); err == nil {
		t.Errorf(".bws directory should not be created for empty dir")
	}
}

func TestAutoConfigureWorkspaceGoProject(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module mytest\n"), 0644); err != nil {
		t.Fatalf("failed to create go.mod: %v", err)
	}

	configPath, summary, err := AutoConfigureWorkspace(tmpDir, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "Go" {
		t.Errorf("expected summary 'Go', got %q", summary)
	}
	expectedPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if configPath != expectedPath {
		t.Errorf("expected configPath %q, got %q", expectedPath, configPath)
	}

	rawContent, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read raw config file: %v", err)
	}
	if !strings.Contains(string(rawContent), "@@HOME@@/.go") {
		t.Errorf("expected raw config file to contain @@HOME@@/.go, got:\n%s", string(rawContent))
	}

	loaded, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("failed to load generated config: %v", err)
	}
	if loaded.Features == nil || loaded.Features.EnableSSH == nil || !*loaded.Features.EnableSSH {
		t.Errorf("expected SSH enabled by default in Go project config")
	}
	if !strings.HasSuffix(loaded.Env["GOPATH"], "/.go") {
		t.Errorf("expected GOPATH ending with /.go, got %q", loaded.Env["GOPATH"])
	}
}

func TestAutoConfigureWorkspaceNoSSH(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte("[project]\n"), 0644); err != nil {
		t.Fatalf("failed to create pyproject.toml: %v", err)
	}

	configPath, summary, err := AutoConfigureWorkspace(tmpDir, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary != "Python/UV" {
		t.Errorf("expected summary 'Python/UV', got %q", summary)
	}

	loaded, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("failed to load generated config: %v", err)
	}
	if loaded.Features == nil || loaded.Features.EnableSSH == nil || *loaded.Features.EnableSSH {
		t.Errorf("expected SSH disabled when noSSH is true")
	}
}
