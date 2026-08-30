package bwrap

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bws/internal/config"
	"bws/internal/util"
)

func TestBuildArgsUnshareIPC(t *testing.T) {
	cfg := &config.Config{}
	sandboxDir := t.TempDir()
	currentDir := t.TempDir()

	args := BuildArgs(cfg, sandboxDir, currentDir, true, false)

	foundUnshareIPC := false
	for _, arg := range args {
		if arg == "--unshare-ipc" {
			foundUnshareIPC = true
			break
		}
	}
	if !foundUnshareIPC {
		t.Errorf("expected --unshare-ipc in BuildArgs output, got: %v", args)
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

func boolPtr(b bool) *bool {
	return &b
}
