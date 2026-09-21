package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
)

func TestHandleProfileAddUnknownFailsClosed(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(tmpDir)

	localPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if err := config.CreateDefault(localPath); err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}

	err := HandleProfileAdd([]string{"flogi_nonexistent_profile"}, false, true, false)
	if err == nil {
		t.Fatalf("expected error adding non-existent profile without -c, got nil")
	}
	if !strings.Contains(err.Error(), "not found in catalog") {
		t.Errorf("expected error message to mention 'not found in catalog', got: %v", err)
	}

	cfg, err := config.LoadFile(localPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	for _, p := range cfg.Profiles {
		if p == "flogi_nonexistent_profile" {
			t.Errorf("flogi_nonexistent_profile was added to config despite failure!")
		}
	}
}

func TestHandleProfileAddExistingProfile(t *testing.T) {
	tmpDir := t.TempDir()
	origWd, _ := os.Getwd()
	defer os.Chdir(origWd)
	_ = os.Chdir(tmpDir)

	localPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if err := config.CreateDefault(localPath); err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}

	err := HandleProfileAdd([]string{"git"}, false, true, false)
	if err != nil {
		t.Fatalf("expected nil error adding existing profile, got: %v", err)
	}

	cfg, err := config.LoadFile(localPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	found := false
	for _, p := range cfg.Profiles {
		if p == "git" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'git' to be in cfg.Profiles, got: %v", cfg.Profiles)
	}
}
