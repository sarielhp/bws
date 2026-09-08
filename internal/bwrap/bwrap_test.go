package bwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/util"
)

func TestBuildArgsDefaultIsolationFlags(t *testing.T) {
	cfg := &config.Config{}
	sandboxDir := t.TempDir()
	currentDir := t.TempDir()

	args := BuildArgs(cfg, sandboxDir, currentDir, true, false)

	expectedFlags := []string{"--unshare-ipc", "--unshare-pid", "--new-session"}
	for _, flag := range expectedFlags {
		found := false
		for _, arg := range args {
			if arg == flag {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s in BuildArgs output, got: %v", flag, args)
		}
	}
}

func TestAddX11ArgsTightened(t *testing.T) {
	t.Setenv("DISPLAY", ":0")
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1000")

	var args []string
	addX11Args(&args)

	for i := 0; i < len(args); i++ {
		if args[i] == "--bind-try" && i+1 < len(args) && strings.Contains(args[i+1], "/run/user") {
			t.Errorf("found prohibited /run/user bind in addX11Args: %s", args[i+1])
		}
		if args[i] == "--setenv" && i+1 < len(args) && args[i+1] == "XDG_RUNTIME_DIR" {
			t.Errorf("found prohibited XDG_RUNTIME_DIR setenv in addX11Args")
		}
	}

	foundDisplay := false
	foundNoAtSpi := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--setenv" && args[i+1] == "DISPLAY" {
			foundDisplay = true
		}
		if args[i] == "--setenv" && args[i+1] == "NO_AT_SPI" {
			foundNoAtSpi = true
		}
	}
	if !foundDisplay {
		t.Error("expected DISPLAY setenv in addX11Args")
	}
	if !foundNoAtSpi {
		t.Error("expected NO_AT_SPI setenv in addX11Args")
	}
}

func TestSSHKnownHostsReadOnly(t *testing.T) {
	home := util.HomeDir()
	sshDir := filepath.Join(home, ".ssh")
	knownHosts := filepath.Join(sshDir, "known_hosts")
	if fi, err := os.Stat(knownHosts); err == nil && !fi.IsDir() {
		cfg := &config.Config{
			Features: &config.FeaturesConfig{
				EnableSSH: boolPtr(true),
			},
		}
		args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

		for i := 0; i < len(args)-2; i++ {
			if strings.HasSuffix(args[i+1], "known_hosts") {
				if args[i] == "--bind" {
					t.Errorf("known_hosts must not be mounted with --bind (RW), found at index %d", i)
				}
			}
		}
	}
}

func TestAddDBusArgs(t *testing.T) {
	fakeBus := filepath.Join(t.TempDir(), "bus")
	if err := os.WriteFile(fakeBus, []byte("fake"), 0600); err != nil {
		t.Fatal(err)
	}

	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeBus)
	t.Setenv("XDG_RUNTIME_DIR", "/run/user/1234")

	var args []string
	cfg := &config.Config{
		Features: &config.FeaturesConfig{
			EnableDBus: boolPtr(true),
		},
	}
	addDBusArgs(&args, cfg, true)

	foundBind := false
	foundDBusEnv := false
	foundXDGEnv := false

	for i := 0; i < len(args)-2; i++ {
		if args[i] == "--bind" && args[i+1] == "<filtered-dbus-proxy>" && args[i+2] == "/run/user/1234/bus" {
			foundBind = true
		}
	}
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--setenv" && args[i+1] == "DBUS_SESSION_BUS_ADDRESS" {
			foundDBusEnv = true
		}
		if args[i] == "--setenv" && args[i+1] == "XDG_RUNTIME_DIR" {
			foundXDGEnv = true
		}
	}

	if !foundBind {
		t.Errorf("expected --bind <filtered-dbus-proxy> /run/user/1234/bus in addDBusArgs, got %v", args)
	}
	if !foundDBusEnv {
		t.Error("expected DBUS_SESSION_BUS_ADDRESS setenv in addDBusArgs")
	}
	if !foundXDGEnv {
		t.Error("expected XDG_RUNTIME_DIR setenv in addDBusArgs")
	}
}

func TestDBusDisabled(t *testing.T) {
	fakeBus := filepath.Join(t.TempDir(), "bus")
	if err := os.WriteFile(fakeBus, []byte("fake"), 0600); err != nil {
		// explicitly ignored in test
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeBus)

	f := false
	cfg := &config.Config{
		Features: &config.FeaturesConfig{
			EnableDBus: &f,
		},
	}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--setenv" && args[i+1] == "DBUS_SESSION_BUS_ADDRESS" {
			t.Error("DBUS_SESSION_BUS_ADDRESS should not be set when enable_dbus is false")
		}
	}
}

func TestDBusDisabledByDefault(t *testing.T) {
	fakeBus := filepath.Join(t.TempDir(), "bus")
	if err := os.WriteFile(fakeBus, []byte("fake"), 0600); err != nil {
		// explicitly ignored in test
	}
	t.Setenv("DBUS_SESSION_BUS_ADDRESS", "unix:path="+fakeBus)

	cfg := &config.Config{}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--setenv" && args[i+1] == "DBUS_SESSION_BUS_ADDRESS" {
			t.Error("DBUS_SESSION_BUS_ADDRESS should not be set by default")
		}
	}
}

func TestNoUnconditionalSystemDBus(t *testing.T) {
	cfg := &config.Config{}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	for i := 0; i < len(args)-1; i++ {
		if args[i] == "/run/dbus" {
			t.Errorf("unconditional /run/dbus mount must not be present, found at arg index %d", i)
		}
	}
}

func boolPtr(b bool) *bool {
	return &b
}

func TestBuildArgsRelativePinholeMount(t *testing.T) {
	tmpDir := t.TempDir()
	notesDir := filepath.Join(tmpDir, "notes")
	workDir := filepath.Join(notesDir, "06_verify")
	if err := os.MkdirAll(workDir, 0755); err != nil {
		t.Fatal(err)
	}

	prefixFile := filepath.Join(notesDir, "prefix.tex")
	if err := os.WriteFile(prefixFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		BindsRO: []config.BindEntry{
			{Host: "../prefix.tex"},
		},
	}

	sandboxDir := t.TempDir()
	args := BuildArgs(cfg, sandboxDir, workDir, true, false)

	foundPinhole := false
	for i := 0; i < len(args)-2; i++ {
		if args[i] == "--ro-bind" && args[i+1] == prefixFile && args[i+2] == prefixFile {
			foundPinhole = true
			break
		}
	}
	if !foundPinhole {
		t.Errorf("expected relative pinhole mount %s -> %s in args: %v", prefixFile, prefixFile, args)
	}
}

func TestDefaultHistoryMasking(t *testing.T) {
	home := util.HomeDir()
	bashHistory := filepath.Join(home, ".bash_history")

	if fi, err := os.Stat(bashHistory); err == nil && !fi.IsDir() {
		cfg := &config.Config{}
		args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

		foundMask := false
		for i := 0; i < len(args)-2; i++ {
			if args[i] == "--ro-bind-try" && args[i+1] == "/dev/null" && args[i+2] == bashHistory {
				foundMask = true
				break
			}
		}
		if !foundMask {
			t.Errorf("expected %s to be masked with /dev/null by default", bashHistory)
		}

		f := false
		cfgDisabled := &config.Config{
			Features: &config.FeaturesConfig{
				MaskHistory: &f,
			},
		}
		argsDisabled := BuildArgs(cfgDisabled, t.TempDir(), t.TempDir(), true, false)
		for i := 0; i < len(argsDisabled)-2; i++ {
			if argsDisabled[i] == "--ro-bind-try" && argsDisabled[i+1] == "/dev/null" && argsDisabled[i+2] == bashHistory {
				t.Errorf("%s should not be masked when mask_history is false", bashHistory)
			}
		}
	}
}

func TestNoDuplicateHistoryMasks(t *testing.T) {
	home := util.HomeDir()
	bashHistory := filepath.Join(home, ".bash_history")

	if fi, err := os.Stat(bashHistory); err == nil && !fi.IsDir() {
		cfg := &config.Config{
			Mask: []string{"~/.bash_history"},
		}
		args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

		count := 0
		for i := 0; i < len(args)-2; i++ {
			if args[i] == "--ro-bind-try" && args[i+1] == "/dev/null" && args[i+2] == bashHistory {
				count++
			}
		}
		if count > 1 {
			t.Errorf("expected exactly 1 mask for %s, got %d", bashHistory, count)
		}
	}
}

func TestBlockGHMasksDefault(t *testing.T) {
	cfg := &config.Config{}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	home := util.HomeDir()
	ghBin := "/usr/bin/gh"
	if fi, err := os.Stat(ghBin); err == nil && !fi.IsDir() {
		found := false
		for i := 0; i < len(args)-2; i++ {
			if args[i] == "--ro-bind-try" && args[i+1] == "/dev/null" && args[i+2] == ghBin {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s to be masked with /dev/null by default in args: %v", ghBin, args)
		}
	}

	ghConfig := filepath.Join(home, ".config", "gh")
	if fi, err := os.Stat(ghConfig); err == nil && fi.IsDir() {
		found := false
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--tmpfs" && args[i+1] == ghConfig {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s to be masked with --tmpfs by default in args: %v", ghConfig, args)
		}
	}

	gitCreds := filepath.Join(home, ".git-credentials")
	if fi, err := os.Stat(gitCreds); err == nil && !fi.IsDir() {
		found := false
		for i := 0; i < len(args)-2; i++ {
			if args[i] == "--ro-bind-try" && args[i+1] == "/dev/null" && args[i+2] == gitCreds {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected %s to be masked with /dev/null by default in args: %v", gitCreds, args)
		}
	}
}

func TestBlockGHDisabled(t *testing.T) {
	f := false
	cfg := &config.Config{
		Features: &config.FeaturesConfig{
			BlockGH: &f,
		},
	}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	home := util.HomeDir()
	ghBin := "/usr/bin/gh"
	for i := 0; i < len(args)-2; i++ {
		if args[i] == "--ro-bind-try" && args[i+1] == "/dev/null" && args[i+2] == ghBin {
			t.Errorf("%s should not be masked when block_gh is false", ghBin)
		}
	}

	ghConfig := filepath.Join(home, ".config", "gh")
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--tmpfs" && args[i+1] == ghConfig {
			t.Errorf("%s should not be masked when block_gh is false", ghConfig)
		}
	}

	gitCreds := filepath.Join(home, ".git-credentials")
	for i := 0; i < len(args)-2; i++ {
		if args[i] == "--ro-bind-try" && args[i+1] == "/dev/null" && args[i+2] == gitCreds {
			t.Errorf("%s should not be masked when block_gh is false", gitCreds)
		}
	}
}

func TestScrubForgeTokensFromPassEnv(t *testing.T) {
	t.Setenv("GH_TOKEN", "secret-gh")
	t.Setenv("GITHUB_TOKEN", "secret-github")
	t.Setenv("GH_ENTERPRISE_TOKEN", "secret-gh-ent")
	t.Setenv("GITHUB_ENTERPRISE_TOKEN", "secret-github-ent")
	t.Setenv("SAFE_VAR", "safe-value")

	cfg := &config.Config{
		PassEnv: []string{"GH_*", "GITHUB_*", "SAFE_VAR"},
	}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	foundSafe := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--setenv" && args[i+1] == "SAFE_VAR" {
			foundSafe = true
		}
		if args[i] == "--setenv" {
			for _, blocked := range blockedForgeTokens {
				if args[i+1] == blocked {
					t.Errorf("blocked token %s was set via pass_env wildcard: %v", blocked, args)
				}
			}
		}
	}
	if !foundSafe {
		t.Error("expected SAFE_VAR to be set via pass_env")
	}
}

func TestExplicitEnvAllowsForgeToken(t *testing.T) {
	cfg := &config.Config{
		Env: map[string]string{
			"GH_TOKEN": "explicitly-allowed",
		},
	}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	found := false
	for i := 0; i < len(args)-2; i++ {
		if args[i] == "--setenv" && args[i+1] == "GH_TOKEN" && args[i+2] == "explicitly-allowed" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected GH_TOKEN to be set when explicitly declared in config.Env")
	}
}

func TestClearenvFalseUnsetsForgeTokens(t *testing.T) {
	f := false
	cfg := &config.Config{
		System: &config.SystemConfig{
			Clearenv: &f,
		},
	}
	args := BuildArgs(cfg, t.TempDir(), t.TempDir(), true, false)

	for _, blocked := range blockedForgeTokens {
		foundUnset := false
		for i := 0; i < len(args)-1; i++ {
			if args[i] == "--unsetenv" && args[i+1] == blocked {
				foundUnset = true
				break
			}
		}
		if !foundUnset {
			t.Errorf("expected --unsetenv %s when clearenv is false", blocked)
		}
	}
}

func TestBuildArgsMountsWorkspaceRootWhenInSubdirectory(t *testing.T) {
	wsRoot := t.TempDir()
	bwsDir := filepath.Join(wsRoot, ".bws")
	if err := os.MkdirAll(bwsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bwsDir, "config.jsonc"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	subDir := filepath.Join(wsRoot, "sub", "chapter")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	sandboxDir := t.TempDir()
	args := BuildArgs(cfg, sandboxDir, subDir, true, false)

	foundWsRoot := false
	foundSubDir := false
	foundChdir := false
	for i := 0; i < len(args)-1; i++ {
		if args[i] == "--bind" && i+2 < len(args) && args[i+1] == wsRoot && args[i+2] == wsRoot {
			foundWsRoot = true
		}
		if args[i] == "--bind" && i+2 < len(args) && args[i+1] == subDir && args[i+2] == subDir {
			foundSubDir = true
		}
		if args[i] == "--chdir" && args[i+1] == subDir {
			foundChdir = true
		}
	}

	if !foundWsRoot {
		t.Errorf("expected workspace root %s to be bound, got args: %v", wsRoot, args)
	}
	if !foundSubDir {
		t.Errorf("expected sub directory %s to be bound, got args: %v", subDir, args)
	}
	if !foundChdir {
		t.Errorf("expected chdir to %s, got args: %v", subDir, args)
	}
}
