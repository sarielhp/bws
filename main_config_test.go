package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCLIConfigSetGetUnset(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	cmdInit := exec.Command(bwPath, "init", "-f", tmpDir)
	if out, err := cmdInit.CombinedOutput(); err != nil {
		t.Fatalf("init failed: %v\n%s", err, string(out))
	}

	confPath := filepath.Join(tmpDir, ".bws/config.jsonc")

	// Set enable_proxy true
	cmdSet := exec.Command(bwPath, "config", "set", "enable_proxy", "true")
	cmdSet.Dir = tmpDir
	if out, err := cmdSet.CombinedOutput(); err != nil {
		t.Fatalf("config set failed: %v\n%s", err, string(out))
	}

	// Get enable_proxy
	cmdGet := exec.Command(bwPath, "config", "get", "enable_proxy")
	cmdGet.Dir = tmpDir
	outGet, err := cmdGet.CombinedOutput()
	if err != nil {
		t.Fatalf("config get failed: %v\n%s", err, string(outGet))
	}
	if !strings.Contains(string(outGet), "true") {
		t.Errorf("expected 'true', got %q", string(outGet))
	}

	// Unset enable_proxy
	cmdUnset := exec.Command(bwPath, "config", "unset", "enable_proxy")
	cmdUnset.Dir = tmpDir
	if out, err := cmdUnset.CombinedOutput(); err != nil {
		t.Fatalf("config unset failed: %v\n%s", err, string(out))
	}

	data, _ := os.ReadFile(confPath)
	if strings.Contains(string(data), "enable_proxy") {
		t.Errorf("expected enable_proxy to be removed from %s, got:\n%s", confPath, string(data))
	}
}
