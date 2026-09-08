package cli

import (
	"bytes"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bws/internal/config"
)

func TestCheckKernelUserns(t *testing.T) {
	// Test success
	mockPassProber := func() (string, error) {
		return "", nil
	}
	resPass := CheckKernelUserns(mockPassProber)
	if resPass.Status != StatusPass {
		t.Errorf("expected StatusPass, got %v", resPass.Status)
	}

	// Test failure
	mockFailProber := func() (string, error) {
		return "Creating new namespace failed", errors.New("exit status 1")
	}
	resFail := CheckKernelUserns(mockFailProber)
	if resFail.Status != StatusFail {
		t.Errorf("expected StatusFail, got %v", resFail.Status)
	}
	if len(resFail.Advice) == 0 {
		t.Error("expected actionable sysctl advice on failure")
	}
	adviceJoined := strings.Join(resFail.Advice, " ")
	if !strings.Contains(adviceJoined, "unprivileged_userns_clone") {
		t.Errorf("expected unprivileged_userns_clone advice, got %s", adviceJoined)
	}
}

func TestCheckCriticalTools(t *testing.T) {
	// All tools exist
	mockLookupAll := func(name string) bool {
		return true
	}
	res := CheckCriticalTools(mockLookupAll)
	if res.Status != StatusPass {
		t.Errorf("expected StatusPass, got %v (%s)", res.Status, res.Message)
	}

	// Missing tmux
	mockLookupMissing := func(name string) bool {
		return name != "tmux"
	}
	res = CheckCriticalTools(mockLookupMissing)
	if res.Status != StatusFail {
		t.Errorf("expected StatusFail when tmux is missing, got %v", res.Status)
	}
	if !strings.Contains(res.Message, "tmux") {
		t.Errorf("expected missing tool in message, got %s", res.Message)
	}
}

func TestCheckConfigFilesAndProfiles(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Valid local config with valid embedded profile
	bwsDir := filepath.Join(tmpDir, ".bws")
	os.MkdirAll(bwsDir, 0755)
	validLocalJSON := `{"profiles": ["editor"]}`
	os.WriteFile(filepath.Join(bwsDir, "config.jsonc"), []byte(validLocalJSON), 0644)

	res := CheckConfigFilesAndProfiles(tmpDir)
	if res.Status != StatusPass {
		t.Errorf("expected StatusPass with valid config, got %v (%s)", res.Status, res.Message)
	}

	// 2. Syntax error in local config
	os.WriteFile(filepath.Join(bwsDir, "config.jsonc"), []byte(`{ invalid json`), 0644)
	res = CheckConfigFilesAndProfiles(tmpDir)
	if res.Status != StatusFail {
		t.Errorf("expected StatusFail with invalid syntax, got %v", res.Status)
	}

	// 3. Unknown profile declared
	os.WriteFile(filepath.Join(bwsDir, "config.jsonc"), []byte(`{"profiles": ["nonexistent-xyz-profile"]}`), 0644)
	res = CheckConfigFilesAndProfiles(tmpDir)
	if res.Status != StatusFail {
		t.Errorf("expected StatusFail with unknown profile, got %v", res.Status)
	}
	if !strings.Contains(res.Message, "nonexistent-xyz-profile") {
		t.Errorf("expected profile name in error message, got %s", res.Message)
	}
}

func TestCheckDeadBindMounts(t *testing.T) {
	tmpDir := t.TempDir()
	existingPath := filepath.Join(tmpDir, "exists")
	os.MkdirAll(existingPath, 0755)
	nonExistentPath := filepath.Join(tmpDir, "does-not-exist")

	cfg := &config.Config{
		BindsRW: []config.BindEntry{
			{Host: existingPath, Sandbox: "/tmp/a"},
			{Host: nonExistentPath, Sandbox: "/tmp/b"},
		},
	}

	res := CheckDeadBindMounts(cfg, tmpDir)
	if res.Status != StatusWarn {
		t.Errorf("expected StatusWarn for missing bind mount, got %v", res.Status)
	}
	if len(res.Advice) == 0 || !strings.Contains(res.Advice[0], "bws mount rm") {
		t.Errorf("expected advice with 'bws mount rm', got %v", res.Advice)
	}

	// Now with all existing binds
	cfgValid := &config.Config{
		BindsRW: []config.BindEntry{
			{Host: existingPath, Sandbox: "/tmp/a"},
		},
	}
	resValid := CheckDeadBindMounts(cfgValid, tmpDir)
	if resValid.Status != StatusPass {
		t.Errorf("expected StatusPass for all existing binds, got %v", resValid.Status)
	}
}

func TestCheckWorkspaceSymlinks(t *testing.T) {
	wsDir := t.TempDir()
	outsideDir := t.TempDir()

	// 1. Symlink pointing to target inside workspace
	insideTarget := filepath.Join(wsDir, "inside_target.txt")
	os.WriteFile(insideTarget, []byte("data"), 0644)
	os.Symlink(insideTarget, filepath.Join(wsDir, "link_inside"))

	cfg := &config.Config{}
	res := CheckWorkspaceSymlinks(cfg, wsDir)
	if res.Status != StatusPass {
		t.Errorf("expected StatusPass for internal symlink, got %v (%s)", res.Status, res.Message)
	}

	// 2. Symlink pointing outside mounted set
	outsideTarget := filepath.Join(outsideDir, "external.txt")
	os.WriteFile(outsideTarget, []byte("secret"), 0644)
	linkOutside := filepath.Join(wsDir, "link_outside")
	os.Symlink(outsideTarget, linkOutside)

	res = CheckWorkspaceSymlinks(cfg, wsDir)
	if res.Status != StatusWarn {
		t.Errorf("expected StatusWarn for outside symlink, got %v", res.Status)
	}
	if len(res.Advice) == 0 || !strings.Contains(res.Advice[0], "bws mount add") {
		t.Errorf("expected advice with 'bws mount add', got %v", res.Advice)
	}
}

func TestCheckMountMaskingConflicts(t *testing.T) {
	tmpDir := t.TempDir()

	// Bind target matches a masked directory
	cfg := &config.Config{
		Mask: []string{"/tmp/masked_dir"},
		BindsRW: []config.BindEntry{
			{Host: "/tmp/some_host", Sandbox: "/tmp/masked_dir/subfile"},
		},
	}

	res := CheckMountMaskingConflicts(cfg, tmpDir)
	if res.Status != StatusWarn {
		t.Errorf("expected StatusWarn for shadowed bind mount, got %v", res.Status)
	}
	if len(res.Details) == 0 || !strings.Contains(res.Details[0], "shadowed by tmpfs mask") {
		t.Errorf("expected conflict details in result, got %v", res.Details)
	}

	// No conflicts
	cfgClean := &config.Config{
		Mask: []string{"/tmp/other_dir"},
		BindsRW: []config.BindEntry{
			{Host: "/tmp/some_host", Sandbox: "/tmp/unmasked_dir"},
		},
	}
	resClean := CheckMountMaskingConflicts(cfgClean, tmpDir)
	if resClean.Status != StatusPass {
		t.Errorf("expected StatusPass when no masking conflicts, got %v", resClean.Status)
	}
}

func TestCheckSSHAgent(t *testing.T) {
	// 1. SSH disabled in config
	falseVal := false
	cfgDisabled := &config.Config{
		Features: &config.FeaturesConfig{
			EnableSSH: &falseVal,
		},
	}
	res := CheckSSHAgent(cfgDisabled, nil)
	if res.Status != StatusPass {
		t.Errorf("expected StatusPass when SSH is disabled, got %v", res.Status)
	}

	// 2. SSH enabled, SSH_AUTH_SOCK unset
	trueVal := true
	cfgEnabled := &config.Config{
		Features: &config.FeaturesConfig{
			EnableSSH: &trueVal,
		},
	}
	t.Setenv("SSH_AUTH_SOCK", "")
	res = CheckSSHAgent(cfgEnabled, nil)
	if res.Status != StatusWarn {
		t.Errorf("expected StatusWarn when SSH_AUTH_SOCK is unset, got %v", res.Status)
	}

	// 3. SSH enabled, socket file exists and responsive
	sockDir := t.TempDir()
	sockPath := filepath.Join(sockDir, "test_agent.sock")
	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatalf("failed to create unix socket listener: %v", err)
	}
	defer ln.Close()

	t.Setenv("SSH_AUTH_SOCK", sockPath)
	res = CheckSSHAgent(cfgEnabled, nil)
	if res.Status != StatusPass {
		t.Errorf("expected StatusPass for active responsive socket, got %v (%s)", res.Status, res.Message)
	}

	// 4. SSH enabled, socket unresponsive (mock dialer returns error)
	mockFailDial := func(network, address string, timeout time.Duration) (net.Conn, error) {
		return nil, errors.New("connection refused")
	}
	res = CheckSSHAgent(cfgEnabled, mockFailDial)
	if res.Status != StatusWarn {
		t.Errorf("expected StatusWarn for unresponsive socket, got %v", res.Status)
	}
}

func TestRenderDoctorReport(t *testing.T) {
	results := []CheckResult{
		{Name: "Check 1", Status: StatusPass, Message: "All good."},
		{Name: "Check 2", Status: StatusWarn, Message: "Warning here.", Advice: []string{"Do something"}},
		{Name: "Check 3", Status: StatusFail, Message: "Failure here.", Details: []string{"Extra info"}},
	}

	// Plain formatting (colors disabled)
	var buf bytes.Buffer
	colorsPlain := initDoctorColors(false)
	failed, warned, passed := RenderDoctorReport(&buf, results, colorsPlain)

	if failed != 1 || warned != 1 || passed != 1 {
		t.Errorf("expected 1 failed, 1 warned, 1 passed, got (%d, %d, %d)", failed, warned, passed)
	}
	output := buf.String()
	if !strings.Contains(output, "Result: 1 failed, 1 warning, 1 passed.") {
		t.Errorf("expected summary line in output, got:\n%s", output)
	}
	if !strings.Contains(output, "[OK]") || !strings.Contains(output, "[WARN]") || !strings.Contains(output, "[FAIL]") {
		t.Errorf("expected status tags in output, got:\n%s", output)
	}
	if strings.Contains(output, "\x1b[") {
		t.Errorf("expected no ANSI color codes when color is disabled, got:\n%s", output)
	}

	// Colored formatting
	var colorBuf bytes.Buffer
	colorsColored := initDoctorColors(true)
	RenderDoctorReport(&colorBuf, results, colorsColored)
	colorOutput := colorBuf.String()
	if !strings.Contains(colorOutput, "\x1b[") {
		t.Error("expected ANSI escape codes when color is enabled")
	}
}
