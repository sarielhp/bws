package main

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestMountAddRelativePinhole(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	notesDir := filepath.Join(tmpDir, "notes")
	workDir := filepath.Join(notesDir, "06_verify")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	prefixFile := filepath.Join(notesDir, "prefix.tex")
	if err := os.WriteFile(prefixFile, []byte("prefix"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(bwPath, "mount", "add", "../prefix.tex", "--ro")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws mount add ../prefix.tex --ro failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Added read-only bind mount '../prefix.tex'") {
		t.Errorf("expected success message, got: %q", string(output))
	}

	localConf := filepath.Join(workDir, ".bws", "config.jsonc")
	data, err := os.ReadFile(localConf)
	if err != nil {
		t.Fatalf("reading local config failed: %v", err)
	}
	if !containsStr(string(data), "../prefix.tex") {
		t.Errorf("expected ../prefix.tex in local config, got: %s", string(data))
	}
}
