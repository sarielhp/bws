package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestInitDevDryRun(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module test"), 0644)

	cmd := exec.Command(bwPath, "init-dev", "-n", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bw init-dev -n failed: %v\n%s", err, string(output))
	}

	outStr := string(output)
	if !containsStr(outStr, `"GOPATH": "@@HOME@@/.go"`) {
		t.Errorf("expected GOPATH in dry run output, got:\n%s", outStr)
	}
	if !containsStr(outStr, `"~/.gemini"`) {
		t.Errorf("expected ~/.gemini in dry run output, got:\n%s", outStr)
	}

	// Make sure .bw.jsonc was NOT written
	if _, err := os.Stat(filepath.Join(tmpDir, ".bw.jsonc")); err == nil {
		t.Error(".bw.jsonc should not exist after dry run")
	}
}

func TestInitDevWriteAndBackup(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "pyproject.toml"), []byte("[project]"), 0644)

	// First init
	cmd := exec.Command(bwPath, "init-dev", tmpDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bw init-dev failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Initialized development sandbox configuration") {
		t.Errorf("expected success output, got: %s", string(output))
	}

	configPath := filepath.Join(tmpDir, ".bw", "config.jsonc")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf(".bw/config.jsonc was not created: %v", err)
	}

	// Second init (should backup old)
	cmd = exec.Command(bwPath, "init-dev", tmpDir)
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("second bw init-dev failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Backed up existing configuration") {
		t.Errorf("expected backup notice, got: %s", string(output))
	}

	backupPath := configPath + ".bak"
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup file was not created: %v", err)
	}
}
