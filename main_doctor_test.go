package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sarielhp/clihelp"
)

func TestDoctorIntegrationExecution(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	cmd := exec.Command(bwPath, "doctor")
	cmd.Dir = t.TempDir()
	out, err := cmd.CombinedOutput()
	output := string(out)

	// Check header and all 7 diagnostics checks are present in output
	expectedSubstrings := []string{
		"Bubblewrap Sandbox Diagnostics (bws doctor)",
		"Kernel & Bubblewrap User Namespaces",
		"Critical Host Tools",
		"Configuration Files & Profiles",
		"Stale / Dead Bind Mounts",
		"Workspace Symlink Boundary Audit",
		"Mount Masking Conflicts",
		"SSH Agent Socket Health",
		"Result:",
	}
	for _, sub := range expectedSubstrings {
		if !strings.Contains(output, sub) {
			t.Errorf("expected doctor output to contain %q, but got:\n%s", sub, output)
		}
	}

	// If there were failed checks, exit code should be non-zero; otherwise zero
	if strings.Contains(output, "0 failed") {
		if err != nil {
			t.Errorf("expected exit code 0 when 0 failed, but got error: %v\n%s", err, output)
		}
	} else {
		if err == nil {
			t.Errorf("expected non-zero exit code when checks failed, but got nil error\n%s", output)
		}
	}
}

func TestDoctorExitCodeOnConfigFailure(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	tmpDir := t.TempDir()
	bwsDir := filepath.Join(tmpDir, ".bws")
	if err := os.MkdirAll(bwsDir, 0755); err != nil {
		t.Fatalf("failed to create .bws dir: %v", err)
	}
	// Write malformed JSONC to trigger Check 3 failure
	if err := os.WriteFile(filepath.Join(bwsDir, "config.jsonc"), []byte(`{ broken json`), 0644); err != nil {
		t.Fatalf("failed to write malformed config: %v", err)
	}

	cmd := exec.Command(bwPath, "doctor")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	output := string(out)

	if err == nil {
		t.Fatalf("expected non-zero exit code for doctor when config has syntax error, got success\n%s", output)
	}
	if !strings.Contains(output, "[FAIL]") {
		t.Errorf("expected [FAIL] in doctor output, got:\n%s", output)
	}
	if !strings.Contains(output, "Configuration Files & Profiles") {
		t.Errorf("expected check name in output, got:\n%s", output)
	}
}

func TestDoctorClihelpAuditAndRouting(t *testing.T) {
	app := buildApp()
	if err := clihelp.Audit(app); err != nil {
		t.Fatalf("clihelp.Audit failed with doctor command: %v", err)
	}

	// Verify doctor rejects unexpected positional arguments
	res := clihelp.TestExecute(app, []string{"doctor", "unexpected-arg"})
	if res.Error == nil {
		t.Error("expected error when doctor is passed unexpected positional argument")
	}
}
