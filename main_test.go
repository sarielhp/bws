package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"bws/internal/config"
	"bws/internal/util"

	"github.com/sarielhp/clihelp"
)

var bwPath string

func init() {
	cwd, _ := os.Getwd()
	if _, err := os.Stat(filepath.Join(cwd, "bws")); err == nil {
		bwPath = filepath.Join(cwd, "bws")
	} else {
		bwPath = filepath.Join(cwd, "bw")
	}
}

func ensureGlobalConfig(t *testing.T) {
	t.Helper()
	globalPath := config.GlobalPath()
	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		if err := config.CreateDefault(globalPath); err != nil {
			tmpHome := t.TempDir()
			t.Setenv("HOME", tmpHome)
			globalPath = config.GlobalPath()
			_ = config.CreateDefault(globalPath)
		}
		examplePath := filepath.Join(filepath.Dir(globalPath), "example-config.jsonc")
		_ = config.CreateExampleConfig(examplePath)
	}
}

func TestBuildAndConf(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	// Short status test
	cmd := exec.Command(bwPath, "status")
	cmd.Dir = t.TempDir()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws status failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "bws status all") {
		t.Error("expected tip mentioning 'bws status all' in short status output")
	}

	// Full status all test
	cmd = exec.Command(bwPath, "status", "all", "-f")
	cmd.Dir = t.TempDir()
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws status all -f failed: %v\n%s", err, string(output))
	}

	if !containsStr(string(output), "Bubblewrap sandbox configuration information") {
		t.Error("expected info header in output")
	}
	if !containsStr(string(output), "Global configuration file") {
		t.Error("expected global config path in output")
	}
}

func TestBuildAndVersion(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	cmd := exec.Command(bwPath, "--version")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bw --version failed: %v", err)
	}
	if !containsStr(string(output), "bws ") || !containsStr(string(output), ".") {
		t.Errorf("expected version string like 'bws X.Y.Z', got %q", string(output))
	}
}

func TestHelp(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	for _, flag := range []string{"help", "-h", "--help", "-help", "--h", "-?", "-H"} {
		cmd := exec.Command(bwPath, flag)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("bws %s failed: %v\n%s", flag, err, string(output))
		}
		if !containsStr(string(output), "Usage:") || !containsStr(string(output), "bws") {
			t.Errorf("expected usage header for flag %q, got %q", flag, string(output))
		}
	}
}

func TestBuildAndHelp(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	cmd := exec.Command(bwPath, "--help")
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("bw --help failed: %v", err)
	}
	if !containsStr(string(output), "Usage:") || !containsStr(string(output), "bws") {
		t.Errorf("expected usage string, got %q", string(output))
	}
}

func TestAppAudit(t *testing.T) {
	app := buildApp()
	if err := clihelp.Audit(app); err != nil {
		t.Fatalf("clihelp.Audit failed: %v", err)
	}
}

func TestClihelpAbbrevAndCompletion(t *testing.T) {
	ensureGlobalConfig(t)
	app := buildApp()

	// Test shell completion generator
	res := clihelp.TestExecute(app, []string{"config", "completion", "bash"})
	res.AssertNoError(t)
	res.AssertStdoutContains(t, "_bws_complete")

	// Test MutuallyExclusive validation (-g and -l together)
	res = clihelp.TestExecute(app, []string{"config", "reset", "-g", "-l"})
	if res.Error == nil {
		t.Error("expected mutually exclusive error when both -g and -l are passed")
	}

	// Test Enum validation on init --preset
	res = clihelp.TestExecute(app, []string{"init", "--preset", "invalidstack"})
	if res.Error == nil {
		t.Error("expected error for invalid --preset enum value")
	}

	// Test status and plan commands
	res = clihelp.TestExecute(app, []string{"status"})
	res.AssertNoError(t)
	res = clihelp.TestExecute(app, []string{"plan"})
	res.AssertNoError(t)

	// Test top-level add / rm command routing
	res = clihelp.TestExecute(app, []string{"add"})
	if res.Error == nil {
		t.Error("expected error when add is missing name argument")
	}
	res = clihelp.TestExecute(app, []string{"rm"})
	if res.Error == nil {
		t.Error("expected error when rm is missing name argument")
	}

	// Test profile add / del command routing
	res = clihelp.TestExecute(app, []string{"profile", "add"})
	if res.Error == nil {
		t.Error("expected error when profile add is missing name argument")
	}
}

func TestCopyAddListDel(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	home := util.HomeDir()
	prog := filepath.Join(home, "test-copy-addlist")
	os.WriteFile(prog, []byte("test"), 0644)
	defer os.Remove(prog)

	// Add via local config
	cmd := exec.Command(bwPath, "copy", "add", prog, "-l")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws copy add failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Added") {
		t.Errorf("expected add confirmation, got %q", string(output))
	}

	// List should show it
	cmd = exec.Command(bwPath, "copy", "list")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws copy list failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), prog) {
		t.Errorf("expected prog in list, got %q", string(output))
	}

	// Delete it
	cmd = exec.Command(bwPath, "copy", "del", prog, "-l")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws copy del failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Removed") {
		t.Errorf("expected del confirmation, got %q", string(output))
	}

	// Cleanup local config
	os.Remove(filepath.Join(tmpDir, ".bws/config.jsonc"))
}

func TestMountAddListDel(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	bindPath := "/tmp/bws-test-bind-addlist"
	os.MkdirAll(bindPath, 0755)
	defer os.RemoveAll(bindPath)

	// Add via local config
	cmd := exec.Command(bwPath, "mount", "add", bindPath, "-l")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws mount add failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Added") {
		t.Errorf("expected add confirmation, got %q", string(output))
	}

	// List should show it
	cmd = exec.Command(bwPath, "mount", "list")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws mount list failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), bindPath) {
		t.Errorf("expected bindPath in local list, got %q", string(output))
	}

	// Delete it
	cmd = exec.Command(bwPath, "mount", "del", bindPath, "-l")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws mount del failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Removed") {
		t.Errorf("expected del confirmation, got %q", string(output))
	}

	// Cleanup local config
	os.Remove(filepath.Join(tmpDir, ".bws/config.jsonc"))
}

func containsStr(s, substr string) bool {
	return len(s) >= len(substr) && searchStr(s, substr)
}

func searchStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(buf[pos:])
}
