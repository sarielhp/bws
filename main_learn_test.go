package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestLearnHelp(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	cmd := exec.Command(bwPath, "learn", "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws learn --help failed: %v\n%s", err, string(output))
	}
	outStr := string(output)
	if !strings.Contains(outStr, "Learn required mounts") {
		t.Errorf("expected command description in learn --help, got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "--dry-run") || !strings.Contains(outStr, "--profile") {
		t.Errorf("expected flags in learn --help, got:\n%s", outStr)
	}

	// Trace and Record should no longer exist as dedicated commands
	app := buildApp()
	if app.LookupCommand("trace") != nil {
		t.Errorf("expected 'trace' command to be completely removed from app command tree")
	}
	if app.LookupCommand("record") != nil {
		t.Errorf("expected 'record' alias to be completely removed from app command tree")
	}
	if app.LookupCommand("learn") == nil {
		t.Errorf("expected 'learn' command to exist in app command tree")
	}
}

func TestLearnDryRun(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tempDir := t.TempDir()
	cmd := exec.Command(bwPath, "learn", "-n", "--", "echo", "hello learn")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws learn -n failed: %v\n%s", err, string(output))
	}

	outStr := string(output)
	if !strings.Contains(outStr, "Learning command:") || !strings.Contains(outStr, "Trace analysis complete") {
		t.Errorf("expected trace output header in output:\n%s", outStr)
	}
	if !strings.Contains(outStr, "Detected Features:") {
		t.Errorf("expected 'Detected Features:' in output:\n%s", outStr)
	}
}

func TestLearnFlagPassThroughWithoutDashDash(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tempDir := t.TempDir()
	// Run child command with flags (e.g. sh -c "echo passthrough") without explicit --
	cmd := exec.Command(bwPath, "learn", "-n", "sh", "-c", "echo passthrough")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws learn -n sh -c failed: %v\n%s", err, string(output))
	}

	outStr := string(output)
	if !strings.Contains(outStr, "Learning command: sh -c echo passthrough") {
		t.Errorf("expected command pass-through in output:\n%s", outStr)
	}
}

func TestLearnProfileGeneration(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tempDir := t.TempDir()
	profName := "sample-tool"
	cmd := exec.Command(bwPath, "learn", "-p", profName, "-l", "--", "echo", "profile test")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws learn -p failed: %v\n%s", err, string(output))
	}

	expectedProfilePath := filepath.Join(tempDir, ".bws", "profiles", profName+".json")
	if _, err := os.Stat(expectedProfilePath); os.IsNotExist(err) {
		t.Fatalf("expected profile file to be created at %s, but not found", expectedProfilePath)
	}
}

func TestLearnNoArgsShowsUsageWithExamples(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	cmd := exec.Command(bwPath, "learn")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws learn (no args) failed with error: %v\n%s", err, string(output))
	}
	outStr := string(output)
	if !strings.Contains(outStr, "Usage:") || !strings.Contains(outStr, "bws learn") {
		t.Errorf("expected usage message in bws learn (no args), got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "Learn required mounts") {
		t.Errorf("expected command description in bws learn (no args), got:\n%s", outStr)
	}
	if !strings.Contains(outStr, "Examples:") || !strings.Contains(outStr, "bws learn") {
		t.Errorf("expected examples in bws learn (no args), got:\n%s", outStr)
	}
}
