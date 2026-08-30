package config

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestGenerateDefaultConfigNoGitCredentials(t *testing.T) {
	content := generateDefaultConfig()
	if strings.Contains(content, ".git-credentials") {
		t.Error("generateDefaultConfig should not include .git-credentials in default configuration")
	}

	tmpDir := t.TempDir()
	confPath := filepath.Join(tmpDir, "config.jsonc")
	if err := CreateDefault(confPath); err != nil {
		t.Fatalf("CreateDefault failed: %v", err)
	}

	loaded, err := LoadFile(confPath)
	if err != nil {
		t.Fatalf("LoadFile on default config failed: %v", err)
	}

	for _, bind := range loaded.BindsRO {
		if strings.Contains(bind.Host, ".git-credentials") || strings.Contains(bind.Sandbox, ".git-credentials") {
			t.Errorf("found .git-credentials in loaded default config: %+v", bind)
		}
	}
}
