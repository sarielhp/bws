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
	if !strings.Contains(outStr, "✓ Sandbox configuration already covers all required access. No changes needed.") {
		t.Errorf("expected success message in output:\n%s", outStr)
	}
	if strings.Contains(outStr, "Detected Features:") {
		t.Errorf("expected 'Detected Features:' to be omitted when delta is empty and verbose=false, got:\n%s", outStr)
	}
	if strings.Contains(outStr, "Discovered Bind Mounts:") {
		t.Errorf("expected 'Discovered Bind Mounts:' to be omitted when delta is empty and verbose=false, got:\n%s", outStr)
	}
}

func TestLearnDryRunVerbose(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tempDir := t.TempDir()
	cmd := exec.Command(bwPath, "learn", "-v", "-n", "--", "echo", "hello learn")
	cmd.Dir = tempDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws learn -v -n failed: %v\n%s", err, string(output))
	}

	outStr := string(output)
	if !strings.Contains(outStr, "Learning command:") || !strings.Contains(outStr, "Trace analysis complete") {
		t.Errorf("expected trace output header in output:\n%s", outStr)
	}
	if !strings.Contains(outStr, "Detected Features:") {
		t.Errorf("expected 'Detected Features:' in verbose output:\n%s", outStr)
	}
	if !strings.Contains(outStr, "Discovered Bind Mounts:") {
		t.Errorf("expected 'Discovered Bind Mounts:' in verbose output:\n%s", outStr)
	}
	if !strings.Contains(outStr, "✓ Sandbox configuration already covers all required access. No changes needed.") {
		t.Errorf("expected completion message in verbose output:\n%s", outStr)
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

func TestLearnIdempotentSubsequentRuns(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tempDir := t.TempDir()

	// Initial run
	cmd1 := exec.Command(bwPath, "learn", "--", "echo", "idempotent test")
	cmd1.Dir = tempDir
	output1, err := cmd1.CombinedOutput()
	if err != nil {
		t.Fatalf("initial bws learn failed: %v\n%s", err, string(output1))
	}
	out1 := string(output1)
	if !strings.Contains(out1, "✓ Sandbox configuration already covers all required access. No changes needed.") {
		t.Errorf("expected initial run to report no changes needed, got:\n%s", out1)
	}

	// Subsequent run should be quiet and idempotent
	cmd2 := exec.Command(bwPath, "learn", "--", "echo", "idempotent test")
	cmd2.Dir = tempDir
	output2, err := cmd2.CombinedOutput()
	if err != nil {
		t.Fatalf("subsequent bws learn failed: %v\n%s", err, string(output2))
	}
	out2 := string(output2)
	if !strings.Contains(out2, "✓ Sandbox configuration already covers all required access. No changes needed.") {
		t.Errorf("expected subsequent run to be quiet, got:\n%s", out2)
	}
	if strings.Contains(out2, "Detected Features:") {
		t.Errorf("subsequent run should omit detected features, got:\n%s", out2)
	}
	if strings.Contains(out2, "Discovered Bind Mounts:") {
		t.Errorf("subsequent run should omit bind mounts, got:\n%s", out2)
	}
}
