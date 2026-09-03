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

	cfg, err := config.LoadFile(configPath)
	if err != nil {
		t.Fatalf("failed to load created config: %v", err)
	}
	if !strings.HasSuffix(cfg.Env["GOPATH"], "/.go") {
		t.Errorf("expected GOPATH ending with /.go in loaded config, got %q", cfg.Env["GOPATH"])
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

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module testpkg\n"), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	var stderr bytes.Buffer
	cmd := exec.Command(bwPath, "--no-init", "--no-ssh", "--", "true")
	cmd.Dir = tmpDir
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bws failed: %v, stderr: %s", err, stderr.String())
	}

	if containsStr(stderr.String(), "Auto-configured") {
		t.Errorf("expected --no-init to suppress auto-configuration, got: %s", stderr.String())
	}

	configPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if _, err := os.Stat(configPath); err == nil {
		t.Errorf("expected .bws/config.jsonc NOT to exist when --no-init passed")
	}
}

func TestAutoInitNeverConfigSkips(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	customHome := setupGlobalConfigWithAutoInit(t, "never")

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "Cargo.toml"), []byte("[package]\nname = \"test\"\n"), 0644); err != nil {
		t.Fatalf("failed to write Cargo.toml: %v", err)
	}

	var stderr bytes.Buffer
	cmd := exec.Command(bwPath, "--no-ssh", "--", "true")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "HOME="+customHome)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bws failed: %v, stderr: %s", err, stderr.String())
	}

	if containsStr(stderr.String(), "Auto-configured") {
		t.Errorf("expected auto_init 'never' to suppress auto-configuration, got: %s", stderr.String())
	}

	configPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if _, err := os.Stat(configPath); err == nil {
		t.Errorf("expected .bws/config.jsonc NOT to exist when auto_init is never")
	}
}

func TestAutoInitForceFlagSkips(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module testpkg\n"), 0644); err != nil {
		t.Fatalf("failed to write go.mod: %v", err)
	}

	var stderr bytes.Buffer
	cmd := exec.Command(bwPath, "-f", "--no-ssh", "--", "true")
	cmd.Dir = tmpDir
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bws failed: %v, stderr: %s", err, stderr.String())
	}

	if containsStr(stderr.String(), "Auto-configured") {
		t.Errorf("expected -f to suppress auto-configuration, got: %s", stderr.String())
	}

	configPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if _, err := os.Stat(configPath); err == nil {
		t.Errorf("expected .bws/config.jsonc NOT to exist when -f passed")
	}
}

func TestAutoInitPromptNonTTYSkips(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	customHome := setupGlobalConfigWithAutoInit(t, "prompt")

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte("{\"name\": \"test\"}\n"), 0644); err != nil {
		t.Fatalf("failed to write package.json: %v", err)
	}

	var stderr bytes.Buffer
	// In exec.Command without pseudo-terminal, stdin is not a TTY
	cmd := exec.Command(bwPath, "--no-ssh", "--", "true")
	cmd.Dir = tmpDir
	cmd.Env = append(os.Environ(), "HOME="+customHome)
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("bws failed: %v, stderr: %s", err, stderr.String())
	}

	if containsStr(stderr.String(), "Auto-configured") {
		t.Errorf("expected auto_init 'prompt' on non-TTY to skip auto-configuration, got: %s", stderr.String())
	}

	configPath := filepath.Join(tmpDir, ".bws", "config.jsonc")
	if _, err := os.Stat(configPath); err == nil {
		t.Errorf("expected .bws/config.jsonc NOT to exist when prompt on non-TTY")
	}
}
