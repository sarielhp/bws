package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestTmuxSettingsInsideSandbox(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("bws binary not built, skipping")
	}
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed, skipping")
	}

	cmd := exec.Command(bwPath, "run", "--", "bash", "-c",
		"tmux -u new-session -d -s test-sess && tmux show -g -t test-sess mouse && tmux kill-session -t test-sess")
	cmd.Dir = t.TempDir()

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws run tmux test failed: %v\n%s", err, string(output))
	}

	if !strings.Contains(string(output), "mouse on") {
		t.Errorf("expected 'mouse on' inside sandbox tmux, got: %q", string(output))
	}
}
