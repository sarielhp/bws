package main

import (
	"os"
	"os/exec"
	"testing"
)

func TestConfWhere(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	cmd := exec.Command(bwPath, "config", "where")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws config where failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Global:") {
		t.Errorf("expected Global: in output, got %q", string(output))
	}
	if !containsStr(string(output), "Local:") {
		t.Errorf("expected Local: in output, got %q", string(output))
	}
}
func TestPathUsage(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	cmd := exec.Command(bwPath, "path", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws path --help failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Usage:") || !containsStr(string(output), "add") {
		t.Errorf("expected usage output, got %q", string(output))
	}
}
func TestPathList(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	cmd := exec.Command(bwPath, "path", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws path list failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "[g]") {
		t.Errorf("expected [g] in output, got %q", string(output))
	}
}
func TestConfShowGlobal(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	cmd := exec.Command(bwPath, "config", "show", "-g")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws config show -g failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "share_net") {
		t.Errorf("expected config content, got %q", string(output))
	}
}
func TestConfShowLocalMissing(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	cmd := exec.Command(bwPath, "config", "show", "-l")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error when local config doesn't exist")
	}
	if !containsStr(string(output), "not found") {
		t.Errorf("expected not found message, got %q", string(output))
	}
}
func TestConfigResetGlobal(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	cmd := exec.Command(bwPath, "config", "reset", "-g")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws config reset -g failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Created") {
		t.Errorf("expected creation confirmation, got %q", string(output))
	}
}
