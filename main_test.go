package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"bws/internal/config"
	"bws/internal/util"
)

var bwPath string

func init() {
	bwPath, _ = os.Getwd()
	bwPath = filepath.Join(bwPath, "bw")
}

func ensureGlobalConfig(t *testing.T) {
	t.Helper()
	globalPath := config.GlobalPath()
	if _, err := os.Stat(globalPath); os.IsNotExist(err) {
		config.CreateDefault(globalPath)
		examplePath := filepath.Join(filepath.Dir(globalPath), "example-config.jsonc")
		config.CreateExampleConfig(examplePath)
	}
}

func TestBuildAndConf(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	cmd := exec.Command(bwPath, "conf", "info", "-f")
	cmd.Dir = t.TempDir()
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws conf info -f failed: %v\n%s", err, string(output))
	}

	if !containsStr(string(output), "Bubblewrap Sandbox Configuration Information") {
		t.Error("expected info header in output")
	}
	if !containsStr(string(output), "Global Configuration File") {
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
		if !containsStr(string(output), "Usage of bws:") {
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
	if !containsStr(string(output), "Usage of bw") {
		t.Errorf("expected usage string, got %q", string(output))
	}
}

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

	cmd := exec.Command(bwPath, "conf", "info", "-f")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws conf info -f in dir with >1000 files should succeed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Bubblewrap Sandbox Configuration Information") {
		t.Error("expected info header in output")
	}
}

func TestConfWhere(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	cmd := exec.Command(bwPath, "conf", "where")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws conf where failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Global:") {
		t.Errorf("expected Global: in output, got %q", string(output))
	}
	if !containsStr(string(output), "Local:") {
		t.Errorf("expected Local: in output, got %q", string(output))
	}
}

func TestConfPathUsage(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	cmd := exec.Command(bwPath, "conf", "path")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws conf path failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Usage:") || !containsStr(string(output), "add") {
		t.Errorf("expected usage output, got %q", string(output))
	}
}

func TestConfPathList(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	cmd := exec.Command(bwPath, "conf", "path", "list")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws conf path list failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "[g]") {
		t.Errorf("expected [g] in output, got %q", string(output))
	}
}

func TestConfShowGlobal(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	cmd := exec.Command(bwPath, "conf", "show", "-g")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws conf show -g failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "share_net") {
		t.Errorf("expected config content, got %q", string(output))
	}
}

func TestConfShowLocalMissing(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	cmd := exec.Command(bwPath, "conf", "show", "-l")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error when local config doesn't exist")
	}
	if !containsStr(string(output), "not found") {
		t.Errorf("expected not found message, got %q", string(output))
	}
}

func TestCCopyRequiresGL(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	home := util.HomeDir()
	prog := filepath.Join(home, "test-ccopy-gl")
	os.WriteFile(prog, []byte("test"), 0644)
	defer os.Remove(prog)

	cmd := exec.Command(bwPath, "ccopy", "add", prog)
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Error("expected error when -g/-l not specified")
	}
	if !containsStr(string(output), "-g") || !containsStr(string(output), "-l") {
		t.Errorf("expected -g/-l error message, got %q", string(output))
	}
}

func TestCCopyAddListDel(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	home := util.HomeDir()
	prog := filepath.Join(home, "test-ccopy-addlist")
	os.WriteFile(prog, []byte("test"), 0644)
	defer os.Remove(prog)

	// Add via local config
	cmd := exec.Command(bwPath, "ccopy", "add", prog, "-l")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws ccopy add failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Added") {
		t.Errorf("expected add confirmation, got %q", string(output))
	}

	// List should show it
	cmd = exec.Command(bwPath, "ccopy", "list")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws ccopy list failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), prog) {
		t.Errorf("expected prog in list, got %q", string(output))
	}

	// Delete it
	cmd = exec.Command(bwPath, "ccopy", "del", prog, "-l")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws ccopy del failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Removed") {
		t.Errorf("expected del confirmation, got %q", string(output))
	}

	// Cleanup local config
	os.Remove(filepath.Join(tmpDir, ".bws/config.jsonc"))
}

func TestCBindAddListDel(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}

	tmpDir := t.TempDir()
	bindPath := "/tmp/bws-test-bind-addlist"
	os.MkdirAll(bindPath, 0755)
	defer os.RemoveAll(bindPath)

	// Add via local config
	cmd := exec.Command(bwPath, "cbind", "add", bindPath, "-l")
	cmd.Dir = tmpDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws cbind add failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Added") {
		t.Errorf("expected add confirmation, got %q", string(output))
	}

	// List should show it
	cmd = exec.Command(bwPath, "cbind", "list")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws cbind list failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), bindPath) {
		t.Errorf("expected bindPath in local list, got %q", string(output))
	}

	// Delete it
	cmd = exec.Command(bwPath, "cbind", "del", bindPath, "-l")
	cmd.Dir = tmpDir
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws cbind del failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Removed") {
		t.Errorf("expected del confirmation, got %q", string(output))
	}

	// Cleanup local config
	os.Remove(filepath.Join(tmpDir, ".bws/config.jsonc"))
}

func TestConfInitGlobal(t *testing.T) {
	if _, err := os.Stat(bwPath); os.IsNotExist(err) {
		t.Skip("binary not built, skipping")
	}
	ensureGlobalConfig(t)

	cmd := exec.Command(bwPath, "conf", "init", "-g")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bws conf init -g failed: %v\n%s", err, string(output))
	}
	if !containsStr(string(output), "Created") {
		t.Errorf("expected creation confirmation, got %q", string(output))
	}
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
