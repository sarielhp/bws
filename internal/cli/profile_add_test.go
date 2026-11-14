package cli

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"bws/internal/config"
)

func TestHandleProfileAddUnknownFailsClosed(t *testing.T) {
	localPath := profileAddWorkspace(t)
	expectProfileAddFails(t, localPath, "flogi_nonexistent_profile", false, "not found in catalog")
}

func TestHandleProfileAddExistingProfile(t *testing.T) {
	localPath := profileAddWorkspace(t)

	err := HandleProfileAdd([]string{"git"}, false, true, false, false)
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

func TestHandleProfileAddUninstalledToolFails(t *testing.T) {
	localPath := profileAddWorkspace(t)
	// Creating a non-existent tool with -c should fail because the tool is not installed on host
	expectProfileAddFails(t, localPath, "mongishogi_nonexistent", true, "not installed on host system")
}

func TestHandleProfileAddPolicyProfileSucceeds(t *testing.T) {
	profileAddWorkspace(t)

	// no-net is a policy profile, does not need an installed binary
	err := HandleProfileAdd([]string{"no-net"}, false, true, false, false)
	if err != nil {
		t.Fatalf("expected nil error adding policy profile no-net, got: %v", err)
	}
}

// profileAddWorkspace makes a temporary workspace with a default local config,
// changes into it for the rest of the test, and returns the config path.
func profileAddWorkspace(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)
	localPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if err := config.CreateDefault(localPath); err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}
	return localPath
}

// expectProfileAddFails checks that adding name (creating it when create is
// set) fails with an error containing want and leaves localPath without it.
func expectProfileAddFails(t *testing.T, localPath, name string, create bool, want string) {
	t.Helper()
	err := HandleProfileAdd([]string{name}, false, true, create, false)
	if err == nil {
		t.Fatalf("expected error adding %s, got nil", name)
	}
	if !strings.Contains(err.Error(), want) {
		t.Errorf("expected error mentioning %q, got: %v", want, err)
	}
	cfg, err := config.LoadFile(localPath)
	if err != nil {
		t.Fatalf("failed to read config: %v", err)
	}
	if slices.Contains(cfg.Profiles, name) {
		t.Errorf("%s was added to config despite failure", name)
	}
}
