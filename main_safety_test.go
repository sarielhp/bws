package main

import (
	"bws/internal/util"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestSafetyRootDir(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	cmd := exec.Command(bwPath)
	cmd.Dir = "/"
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error when running from /")
	}
	if !containsStr(string(output), "running the sandbox from / is blocked") {
		t.Errorf("expected root directory blocked message, got %q", string(output))
	}
}
func TestSafetyHomeDir(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	home := util.HomeDir()
	cmd := exec.Command(bwPath)
	cmd.Dir = home
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error when running from home directory")
	}
	if !containsStr(string(output), "running the sandbox from ~/ is blocked") {
		t.Errorf("expected home directory blocked message, got %q", string(output))
	}
}
func TestSafetyHomeBinDir(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	homeBin := filepath.Join(util.HomeDir(), "bin")
	if _, err := os.Stat(homeBin); os.IsNotExist(err) {
		t.Skip("~/bin does not exist, skipping")
	}

	cmd := exec.Command(bwPath)
	cmd.Dir = homeBin
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error when running from ~/bin")
	}
	if !containsStr(string(output), "running the sandbox from ~/bin/ is blocked") {
		t.Errorf("expected ~/bin blocked message, got %q", string(output))
	}
}
func TestSafetyFileCountLimit(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	tmpDir := t.TempDir()
	for i := 0; i < 1001; i++ {
		os.WriteFile(filepath.Join(tmpDir, "file_"+itoa(i)), []byte("x"), 0644)
	}

	cmd := exec.Command(bwPath)
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error when directory has >1000 files")
	}
	if !containsStr(string(output), "more than") || !containsStr(string(output), "files") {
		t.Errorf("expected file count limit message, got %q", string(output))
	}
}
func TestSafetyFileCountForceFlag(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	tmpDir := t.TempDir()
	for i := 0; i < 1001; i++ {
		os.WriteFile(filepath.Join(tmpDir, "file_"+itoa(i)), []byte("x"), 0644)
	}

	cmd := exec.Command(bwPath, "status", "all", "-f")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws status all -f in dir with >1000 files should succeed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Bubblewrap sandbox configuration information") {
		t.Error("expected info header in output")
	}
}
