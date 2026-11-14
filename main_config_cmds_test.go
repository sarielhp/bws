package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	expectGlobalCmd(t, "[g]", "path", "list")
}
func TestConfShowGlobal(t *testing.T) {
	expectGlobalCmd(t, "share_net", "config", "show", "-g")
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
	expectGlobalCmd(t, "Created", "config", "reset", "-g")
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

// expectGlobalCmd runs bws with args against an existing global config and
// checks that it succeeds and prints want.
func expectGlobalCmd(t *testing.T, want string, args ...string) {
	t.Helper()
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)
	output, err := exec.Command(bwPath, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("bws %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
	if !containsStr(string(output), want) {
		t.Errorf("bws %s: expected %q in output, got %q", strings.Join(args, " "), want, output)
	}
}
