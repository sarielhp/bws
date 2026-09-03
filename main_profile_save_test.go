package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestProfileSave(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	wsDir := t.TempDir()
	bwsDir := filepath.Join(wsDir, ".bws")
	if err := os.MkdirAll(bwsDir, 0755); err != nil {
		t.Fatalf("failed to create .bws dir: %v", err)
	}

	// Create dummy local config.jsonc
	dummyConfig := `{
  "profiles": ["editor"],
  "path": ["/home/sariel/bin", "~/custom/bin"],
  "binds_ro": [
    ["/bin", "/bin"],
    ["/home/sariel/.gitconfig", "/home/sariel/.gitconfig"]
  ],
  "env": {
    "CUSTOM_VAR": "val"
  }
}
`
	if err := os.WriteFile(filepath.Join(bwsDir, "config.jsonc"), []byte(dummyConfig), 0644); err != nil {
		t.Fatalf("failed to write dummy config: %v", err)
	}

	// 1. Test saving locally
	cmd := exec.Command(bwPath, "profile", "save", "snap-local", "-l", "-d", "My local test snapshot")
	cmd.Dir = wsDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws profile save -l failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Saved environment snapshot") {
		t.Errorf("expected success message, got: %s", string(out))
	}

	localProfilePath := filepath.Join(bwsDir, "profiles", "snap-local.json")
	data, err := os.ReadFile(localProfilePath)
	if err != nil {
		t.Fatalf("failed to read created profile: %v", err)
	}

	var p map[string]interface{}
	if err := json.Unmarshal(data, &p); err != nil {
		t.Fatalf("created profile is not valid JSON: %v", err)
	}
	if p["name"] != "snap-local" {
		t.Errorf("expected name 'snap-local', got: %v", p["name"])
	}
	if p["description"] != "My local test snapshot" {
		t.Errorf("expected custom description, got: %v", p["description"])
	}

	// 2. Test conflict without -f
	cmd = exec.Command(bwPath, "profile", "save", "snap-local", "-l")
	cmd.Dir = wsDir
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "already exists") {
		t.Errorf("expected conflict error, got: %s", string(out))
	}

	// 3. Test overwrite with -f
	cmd = exec.Command(bwPath, "profile", "save", "snap-local", "-l", "-f", "-d", "Updated snapshot")
	cmd.Dir = wsDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws profile save -f failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Saved environment snapshot") {
		t.Errorf("expected success message with -f, got: %s", string(out))
	}

	// 4. Test profile show
	cmd = exec.Command(bwPath, "profile", "show", "snap-local")
	cmd.Dir = wsDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws profile show snap-local failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "snap-local") {
		t.Errorf("expected profile name in show output, got: %s", string(out))
	}

	// 5. Test global save
	cmd = exec.Command(bwPath, "profile", "save", "snap-global", "-g")
	cmd.Dir = wsDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws profile save -g failed: %v\n%s", err, string(out))
	}
	globalProfilePath := filepath.Join(tmpHome, ".config", "bws", "profiles", "snap-global.json")
	if _, err := os.Stat(globalProfilePath); err != nil {
		t.Fatalf("expected global profile at %s: %v", globalProfilePath, err)
	}
}
