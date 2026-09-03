package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBinAddListRm(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()

	// Create a dummy executable script
	scriptPath := filepath.Join(tmpDir, "my-tool.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\necho hello\n"), 0755); err != nil {
		t.Fatalf("failed to create dummy script: %v", err)
	}

	// 1. Add locally
	cmd := exec.Command(bwPath, "bin", "add", scriptPath, "-l")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws bin add failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Added executable") {
		t.Errorf("expected 'Added executable' in output, got: %s", string(out))
	}

	// 2. Duplicate add should report already configured
	cmd = exec.Command(bwPath, "bin", "add", scriptPath, "-l")
	cmd.Dir = tmpDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("duplicate bws bin add failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "already configured") {
		t.Errorf("expected 'already configured' in output, got: %s", string(out))
	}

	// 3. List
	cmd = exec.Command(bwPath, "bin", "list")
	cmd.Dir = tmpDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws bin list failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "my-tool.sh") {
		t.Errorf("expected 'my-tool.sh' in bws bin list output, got: %s", string(out))
	}

	// 4. Remove by name
	cmd = exec.Command(bwPath, "bin", "rm", "my-tool.sh", "-l")
	cmd.Dir = tmpDir
	out, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws bin rm failed: %v\n%s", err, string(out))
	}
	if !strings.Contains(string(out), "Removed binary") {
		t.Errorf("expected 'Removed binary' in output, got: %s", string(out))
	}

	// 5. Remove again should fail with not found
	cmd = exec.Command(bwPath, "bin", "rm", "my-tool.sh", "-l")
	cmd.Dir = tmpDir
	out, _ = cmd.CombinedOutput()
	if !strings.Contains(string(out), "not found") {
		t.Errorf("expected 'not found' on duplicate remove, got: %s", string(out))
	}
}
