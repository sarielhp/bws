package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/policy"
	"bws/internal/testutil"
)

func setupGlobalConfigWithAutoInit(t *testing.T, autoInit string) string {
	t.Helper()
	customHome := t.TempDir()
	globalConfigPath := filepath.Join(customHome, ".config", "bws", "config.jsonc")
	if err := config.CreateDefault(globalConfigPath); err != nil {
		t.Fatalf("failed to create default config: %v", err)
	}
	content, err := os.ReadFile(globalConfigPath)
	if err != nil {
		t.Fatalf("failed to read default config: %v", err)
	}
	snippet := fmt.Sprintf(`"features": {"auto_init": %q}, "system": {`, autoInit)
	replaced := strings.Replace(string(content), `"system": {`, snippet, 1)
	if err := os.WriteFile(globalConfigPath, []byte(replaced), 0644); err != nil {
		t.Fatalf("failed to write updated global config: %v", err)
	}
	return customHome
}

func TestAutoInitAlwaysCreatesAndLoads(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module testpkg\n"), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	var stdout, stderr bytes.Buffer
	cmd := exec.Command(bwPath, "--no-ssh", "--", "true")
	cmd.Dir = tmpDir
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bws failed: %v, stderr: %s", err, stderr.String())
	}

	errStr := stderr.String()
	if !containsStr(errStr, "[bws] Auto-configured .bws/config.jsonc (Go detected)") {
		t.Errorf("expected auto-config notice in stderr, got: %s", errStr)
	}

	configPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if _, err := os.Stat(configPath); err != nil {
		t.Fatalf("expected .bws/config.jsonc to be created, stat err: %v", err)
	}

	resolved, err := policy.Load(tmpDir)
	if err != nil {
		t.Fatalf("failed to load created config: %v", err)
	}
	if !strings.HasSuffix(resolved.Config.Env["GOPATH"], "/.go") {
		t.Errorf("expected resolved GOPATH ending with /.go, got %q", resolved.Config.Env["GOPATH"])
	}
}

func TestAutoInitEmptyDirNoConfigCreated(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "notes.txt"), []byte("just some text\n"), 0644); err != nil {
		t.Fatalf("failed to write notes.txt: %v", err)
	}

	var stderr bytes.Buffer
	cmd := exec.Command(bwPath, "--no-ssh", "--", "true")
	cmd.Dir = tmpDir
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bws failed: %v, stderr: %s", err, stderr.String())
	}

	if containsStr(stderr.String(), "Auto-configured") {
		t.Errorf("expected no auto-config in empty/text dir, got: %s", stderr.String())
	}

	configPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if _, err := os.Stat(configPath); err == nil {
		t.Errorf("expected .bws/config.jsonc NOT to exist in empty dir")
	}
}

func TestAutoInitNoInitFlagSkips(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)
	expectNoAutoInit(t, "", "go.mod", "module testpkg\n", "--no-init", "--no-init", "--no-ssh", "--", "true")
}

func TestAutoInitNeverConfigSkips(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	home := setupGlobalConfigWithAutoInit(t, "never")
	expectNoAutoInit(t, home, "Cargo.toml", "[package]\nname = \"test\"\n", "auto_init 'never'", "--no-ssh", "--", "true")
}

func TestAutoInitForceFlagSkips(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)
	expectNoAutoInit(t, "", "go.mod", "module testpkg\n", "-f", "-f", "--no-ssh", "--", "true")
}

// TestAutoInitPromptNonTTYSkips relies on exec.Command giving bws a stdin
// that is not a terminal.
func TestAutoInitPromptNonTTYSkips(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	home := setupGlobalConfigWithAutoInit(t, "prompt")
	expectNoAutoInit(t, home, "package.json", "{\"name\": \"test\"}\n", "auto_init 'prompt' on a non-TTY", "--no-ssh", "--", "true")
}

// expectNoAutoInit runs bws with args in a new workspace holding the project
// file marker and checks that it neither auto-configures nor writes
// .bws/config.jsonc; why names the cause in failures. An empty home keeps the
// test's HOME.
func expectNoAutoInit(t *testing.T, home, marker, contents, why string, args ...string) {
	t.Helper()
	tmpDir := t.TempDir()
	testutil.WriteFile(t, filepath.Join(tmpDir, marker), contents)

	var stderr bytes.Buffer
	cmd := exec.Command(bwPath, args...)
	cmd.Dir = tmpDir
	if home != "" {
		cmd.Env = append(os.Environ(), "HOME="+home)
	}
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bws failed: %v, stderr: %s", err, stderr.String())
	}
	if containsStr(stderr.String(), "Auto-configured") {
		t.Errorf("expected %s to suppress auto-configuration, got: %s", why, stderr.String())
	}
	if _, err := os.Stat(filepath.Join(tmpDir, ".bws", "config.jsonc")); err == nil {
		t.Errorf("expected .bws/config.jsonc NOT to exist with %s", why)
	}
}
