package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestTraceHelpAndAliases(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	for _, cmdName := range []string{"trace", "record", "learn"} {
		cmd := exec.Command(bwPath, cmdName, "--help")
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bws %s --help failed: %v\n%s", cmdName, err, string(output))
		}
		outStr := string(output)
		if !strings.Contains(outStr, "Trace a command dynamically") {
			t.Errorf("expected command description in %s --help, got:\n%s", cmdName, outStr)
		}
		if !strings.Contains(outStr, "--dry-run") || !strings.Contains(outStr, "--write") || !strings.Contains(outStr, "--profile") {
			t.Errorf("expected flags in %s --help, got:\n%s", cmdName, outStr)
		}
	}
}

func TestTraceDryRun(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tempDir := t.TempDir()
	cmd := exec.Command(bwPath, "trace", "-n", "--", "echo", "hello trace")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws trace -n failed: %v\n%s", err, string(output))
	}

	outStr := string(output)
	if !strings.Contains(outStr, "Tracing command:") || !strings.Contains(outStr, "Trace analysis complete") {
		t.Errorf("expected trace output header in output:\n%s", outStr)
	}
	if !strings.Contains(outStr, "Detected Features:") {
		t.Errorf("expected 'Detected Features:' in output:\n%s", outStr)
	}
}

func TestTraceProfileGeneration(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tempDir := t.TempDir()
	profName := "sample-tool"
	cmd := exec.Command(bwPath, "trace", "-p", profName, "-l", "--", "echo", "profile test")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws trace -p failed: %v\n%s", err, string(output))
	}

	expectedProfilePath := filepath.Join(tempDir, ".bws", "profiles", profName+".json")
	if _, err := os.Stat(expectedProfilePath); os.IsNotExist(err) {
		t.Fatalf("expected profile file to be created at %s, but not found", expectedProfilePath)
	}
}
